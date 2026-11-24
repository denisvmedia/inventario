package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
)

// TestCommodityCreate_AssignsAssetNumber checks that the API hands out the
// next asset number of the group and ignores one sent in the payload.
func TestCommodityCreate_AssignsAssetNumber(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	top := 0
	for _, existing := range must.Must(registrySet.CommodityRegistry.List(context.Background())) {
		if existing.AssetNumber != nil {
			top = max(top, *existing.AssetNumber)
		}
	}

	obj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			Type: "commodities",
			Attributes: &models.Commodity{
				Name:                   "Labelled Box",
				ShortName:              "Box",
				Type:                   models.CommodityTypeOther,
				OriginalPrice:          decimal.RequireFromString("10.00"),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: decimal.Zero,
				CurrentPrice:           decimal.RequireFromString("10.00"),
				Count:                  1,
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				AssetNumber:            new(999),
			},
		},
	}
	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/commodities", bytes.NewReader(must.Must(json.Marshal(obj))))
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

	body := rr.Body.Bytes()
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("Body: %s", body))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.asset_number"), float64(top+1))
}

// TestCommoditySearch_ByAssetID checks that both the items list `q` and the
// global search find a commodity by its asset ID.
func TestCommoditySearch_ByAssetID(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	commodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	c.Assert(commodities, qt.Not(qt.HasLen), 0)
	target := commodities[0]
	c.Assert(target.AssetNumber, qt.IsNotNil)
	assetID := models.FormatAssetID(*target.AssetNumber)

	// The list wraps each row in JSON:API attributes; /search returns the
	// models flat.
	for _, tc := range []struct {
		path, numberPath string
	}{
		{"/api/v1/g/" + testGroup.Slug + "/commodities?q=" + url.QueryEscape(assetID), "$.data[0].attributes.asset_number"},
		{"/api/v1/g/" + testGroup.Slug + "/search?type=commodities&q=" + url.QueryEscape(assetID), "$.data[0].asset_number"},
	} {
		c.Run(tc.path, func(c *qt.C) {
			req, err := http.NewRequest("GET", tc.path, nil)
			c.Assert(err, qt.IsNil)
			addTestUserAuthHeader(req, testUser.ID)
			rr := httptest.NewRecorder()
			apiserver.APIServer(params, &mockRestoreWorker{}).ServeHTTP(rr, req)

			body := rr.Body.Bytes()
			c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("Body: %s", body))
			c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), 1)
			c.Check(body, checkers.JSONPathEquals("$.data[0].id"), target.ID)
			c.Check(body, checkers.JSONPathEquals(tc.numberPath), float64(*target.AssetNumber))
		})
	}
}
