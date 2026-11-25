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
