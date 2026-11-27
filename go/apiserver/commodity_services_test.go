package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
)

// serviceAPI creates a single item (services are per-instance; the seeded
// items are bundles) and returns its services URL and a POST helper.
func serviceAPI(c *qt.C) (servicesURL string, post func(url, body string) *httptest.ResponseRecorder) {
	c.Helper()
	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	area := must.Must(registrySet.AreaRegistry.List(context.Background()))[0]
	commodity := must.Must(registrySet.CommodityRegistry.Create(context.Background(), models.Commodity{
		Name:                  "Phone",
		ShortName:             "phone",
		AreaID:                new(area.ID),
		Status:                models.CommodityStatusInUse,
		Type:                  models.CommodityTypeElectronics,
		Count:                 1,
		OriginalPrice:         new(must.Must(decimal.NewFromString("100.00"))),
		OriginalPriceCurrency: models.Currency("USD"),
	}))
	handler := apiserver.APIServer(params, &mockRestoreWorker{hasRunningRestores: false})
	servicesURL = "/api/v1/g/" + testGroup.Slug + "/commodities/" + commodity.ID + "/services"

	post = func(url, body string) *httptest.ResponseRecorder {
		req := must.Must(http.NewRequest("POST", url, bytes.NewBufferString(body)))
		req.Header.Set("Content-Type", "application/vnd.api+json")
		addTestUserAuthHeader(req, testUser.ID)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		return rr
	}
	return servicesURL, post
}

// A repair done for free, under warranty for example, records a cost of 0
// with its currency on both write paths that take a cost.
func TestCommodityServices_CostOfZeroIsRecorded(t *testing.T) {
	c := qt.New(t)

	servicesURL, post := serviceAPI(c)
	decode := func(rr *httptest.ResponseRecorder) models.CommodityService {
		var resp jsonapi.CommodityServiceResponse
		c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
		return resp.Data.Attributes
	}

	rr := post(servicesURL, `{"data":{"type":"commodity_services","attributes":{`+
		`"provider_name":"Repair Shop","sent_at":"2026-05-01","cost_amount":"0","cost_currency":"EUR"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	created := decode(rr)
	c.Assert(created.CostAmount.IsZero(), qt.IsTrue)
	c.Assert(created.CostCurrency, qt.Equals, "EUR")
	c.Assert(created.HasCost(), qt.IsTrue)

	rr = post(servicesURL+"/"+created.ID+"/return", `{"data":{"type":"commodity_services","attributes":{`+
		`"cost_amount":"0","cost_currency":"USD"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	returned := decode(rr)
	c.Assert(returned.CostAmount.IsZero(), qt.IsTrue)
	c.Assert(returned.CostCurrency, qt.Equals, "USD")
}

// A currency sent on return without an amount is refused: on a row with no
// cost it would otherwise record a cost of 0 the caller never stated.
func TestCommodityServices_ReturnRefusesCurrencyWithoutAmount(t *testing.T) {
	c := qt.New(t)

	servicesURL, post := serviceAPI(c)

	rr := post(servicesURL, `{"data":{"type":"commodity_services","attributes":{`+
		`"provider_name":"Repair Shop","sent_at":"2026-05-01"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var resp jsonapi.CommodityServiceResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)

	rr = post(servicesURL+"/"+resp.Data.ID+"/return", `{"data":{"type":"commodity_services","attributes":{`+
		`"cost_currency":"EUR"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(rr.Body.String(), qt.Contains, "cost_currency")
}
