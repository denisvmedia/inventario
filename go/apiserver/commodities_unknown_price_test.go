package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
)

// commodityAttrs is a reviewed item's request attributes with nothing known
// about its purchase; each test adds the prices it is about.
func commodityAttrs(areaID string, extra map[string]any) map[string]any {
	attrs := map[string]any{
		"name":                    "Grandfather clock",
		"short_name":              "Clock",
		"type":                    "furniture",
		"area_id":                 areaID,
		"count":                   1,
		"status":                  "in_use",
		"original_price_currency": "USD",
		"draft":                   false,
	}
	maps.Copy(attrs, extra)
	return attrs
}

func serve(handler http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}

func TestCommodityCreate_UnknownPurchaseInfo(t *testing.T) {
	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	area := must.Must(registrySet.AreaRegistry.List(context.Background()))[0]
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	base := "/api/v1/g/" + testGroup.Slug + "/commodities"

	send := func(c *qt.C, method, path, id string, attrs map[string]any) *httptest.ResponseRecorder {
		data := map[string]any{"type": "commodities", "attributes": attrs}
		if id != "" {
			data["id"] = id
			attrs["id"] = id
		}
		body := must.Must(json.Marshal(map[string]any{"data": data}))
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		addTestUserAuthHeader(req, testUser.ID)
		return serve(handler, req)
	}

	t.Run("gift with unknown date and price", func(t *testing.T) {
		c := qt.New(t)
		rr := send(c, http.MethodPost, base, "", commodityAttrs(area.ID, map[string]any{
			"original_price": nil,
			"current_price":  nil,
		}))
		body := rr.Body.Bytes()
		c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("Body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.converted_original_price"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.purchase_date"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.draft"), false)
		var created struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		c.Assert(json.Unmarshal(body, &created), qt.IsNil)
		id := created.Data.ID

		// Learning later that it was free makes the price a known zero.
		rr = send(c, http.MethodPut, base+"/"+id, id, commodityAttrs(area.ID, map[string]any{
			"original_price": "0",
		}))
		body = rr.Body.Bytes()
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("Body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), "0")
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), nil)
	})

	t.Run("only a current value", func(t *testing.T) {
		c := qt.New(t)
		rr := send(c, http.MethodPost, base, "", commodityAttrs(area.ID, map[string]any{
			"current_price": "1200",
		}))
		body := rr.Body.Bytes()
		c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("Body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), "1200")
	})

	t.Run("foreign-currency known price", func(t *testing.T) {
		c := qt.New(t)
		rr := send(c, http.MethodPost, base, "", commodityAttrs(area.ID, map[string]any{
			"original_price":           "100",
			"original_price_currency":  "EUR",
			"converted_original_price": "108.5",
			"purchase_date":            "2024-05-01",
		}))
		body := rr.Body.Bytes()
		c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("Body: %s", body))
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), "100")
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.converted_original_price"), "108.5")
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), nil)
	})

	t.Run("foreign-currency known price without a group-currency amount", func(t *testing.T) {
		c := qt.New(t)
		rr := send(c, http.MethodPost, base, "", commodityAttrs(area.ID, map[string]any{
			"original_price":          "100",
			"original_price_currency": "EUR",
		}))
		c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("Body: %s", rr.Body.String()))
	})
}

func TestCommodityUpdate_UnknownPrices(t *testing.T) {
	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	area := must.Must(registrySet.AreaRegistry.List(context.Background()))[0]
	handler := apiserver.APIServer(params, &mockRestoreWorker{})
	base := "/api/v1/g/" + testGroup.Slug + "/commodities"

	do := func(c *qt.C, method, path string, body []byte) []byte {
		c.Helper()
		req := httptest.NewRequest(method, path, bytes.NewReader(body))
		addTestUserAuthHeader(req, testUser.ID)
		rr := serve(handler, req)
		c.Assert(rr.Code < 300, qt.IsTrue, qt.Commentf("%s %s: %d %s", method, path, rr.Code, rr.Body.String()))
		return rr.Body.Bytes()
	}
	write := func(c *qt.C, method, path, id string, attrs map[string]any) []byte {
		c.Helper()
		data := map[string]any{"type": "commodities", "attributes": attrs}
		if id != "" {
			data["id"] = id
			attrs["id"] = id
		}
		return do(c, method, path, must.Must(json.Marshal(map[string]any{"data": data})))
	}
	create := func(c *qt.C, attrs map[string]any) string {
		c.Helper()
		var created struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		c.Assert(json.Unmarshal(write(c, http.MethodPost, base, "", attrs), &created), qt.IsNil)
		return created.Data.ID
	}
	type valueCounts struct {
		Data struct {
			Attributes struct {
				ValuedCount   int `json:"valued_count"`
				UnvaluedCount int `json:"unvalued_count"`
			} `json:"attributes"`
		} `json:"data"`
	}
	counts := func(c *qt.C) (valued, unvalued int) {
		c.Helper()
		var v valueCounts
		c.Assert(json.Unmarshal(do(c, http.MethodGet, base+"/values", nil), &v), qt.IsNil)
		return v.Data.Attributes.ValuedCount, v.Data.Attributes.UnvaluedCount
	}

	t.Run("a known price set to null is cleared and recorded", func(t *testing.T) {
		c := qt.New(t)
		id := create(c, commodityAttrs(area.ID, map[string]any{
			"original_price": "250",
			"current_price":  "200",
		}))

		body := write(c, http.MethodPut, base+"/"+id, id, commodityAttrs(area.ID, map[string]any{
			"original_price": "250",
			"current_price":  nil,
		}))
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), nil)

		body = do(c, http.MethodGet, base+"/"+id, nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), "250")

		body = do(c, http.MethodGet, base+"/"+id+"/events?kind=price_changed", nil)
		c.Check(body, checkers.JSONPathEquals("$.data[0].kind"), "price_changed")
		c.Check(body, checkers.JSONPathEquals("$.data[0].before.current_price"), "200")
		c.Check(body, checkers.JSONPathEquals("$.data[0].after.current_price"), nil)
	})

	t.Run("a price left out of a PUT is stored as unknown", func(t *testing.T) {
		c := qt.New(t)
		id := create(c, commodityAttrs(area.ID, map[string]any{
			"original_price": "80",
			"current_price":  "60",
		}))

		// PUT replaces the item: a price the body does not carry is unknown.
		write(c, http.MethodPut, base+"/"+id, id, commodityAttrs(area.ID, map[string]any{
			"original_price": "80",
		}))
		body := do(c, http.MethodGet, base+"/"+id, nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), "80")
	})

	t.Run("a zero survives a read as a known zero", func(t *testing.T) {
		c := qt.New(t)
		id := create(c, commodityAttrs(area.ID, nil))

		write(c, http.MethodPut, base+"/"+id, id, commodityAttrs(area.ID, map[string]any{
			"original_price": "0",
			"current_price":  "0",
		}))
		body := do(c, http.MethodGet, base+"/"+id, nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), "0")
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), "0")
	})

	t.Run("a converted zero on a group-currency item stays unknown", func(t *testing.T) {
		c := qt.New(t)
		valuedBefore, unvaluedBefore := counts(c)

		// What clients written before unknown prices existed send for an
		// item whose price was not entered.
		id := create(c, commodityAttrs(area.ID, map[string]any{
			"converted_original_price": 0,
		}))
		body := do(c, http.MethodGet, base+"/"+id, nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.converted_original_price"), nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), nil)

		write(c, http.MethodPut, base+"/"+id, id, commodityAttrs(area.ID, map[string]any{
			"converted_original_price": 0,
		}))
		body = do(c, http.MethodGet, base+"/"+id, nil)
		c.Check(body, checkers.JSONPathEquals("$.data.attributes.converted_original_price"), nil)

		valued, unvalued := counts(c)
		c.Check(valued, qt.Equals, valuedBefore)
		c.Check(unvalued, qt.Equals, unvaluedBefore+1)
	})
}
