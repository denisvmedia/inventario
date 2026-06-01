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

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

// Issue #1554: locks the four illegal write paths surfaced as 422.
//
//   1. POST /commodities — Count > 1 with warranty fields rejected.
//   2. POST /commodities/{id}/loans — bundle commodity rejected.
//   3. POST /commodities/{id}/services — bundle commodity rejected.
//   4. PUT /commodities/{id} — 1 → >1 bump with open per-instance state
//      rejected with multi-error.

func TestCommodity_Issue1554_CreateRejectsBundleWithWarranty(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	areas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := areas[0]

	expires := models.ToPDate("2027-01-01")
	obj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			Type: "commodities",
			Attributes: &models.Commodity{
				Name:                   "12 light bulbs",
				ShortName:              "bulbs",
				AreaID:                 new(area.ID),
				Type:                   models.CommodityTypeOther,
				Status:                 models.CommodityStatusInUse,
				Count:                  12,
				OriginalPrice:          must.Must(decimal.NewFromString("20.00")),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: must.Must(decimal.NewFromString("0")),
				CurrentPrice:           must.Must(decimal.NewFromString("18.00")),
				PurchaseDate:           models.ToPDate("2026-01-01"),
				WarrantyExpiresAt:      expires,
			},
		},
	}
	data := must.Must(json.Marshal(obj))
	req := must.Must(http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/commodities", bytes.NewReader(data)))
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	apiserver.APIServer(params, mockRestoreWorker).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
}

func TestCommodity_Issue1554_LoanRejectsBundle(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	all := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	// The fixture seeds Commodity 1 with Count=10 (a bundle) — perfect
	// target for the rejection path. Sanity-check rather than rely on
	// fixture order.
	var bundle *models.Commodity
	for _, c := range all {
		if c.Count > 1 {
			bundle = c
			break
		}
	}
	c.Assert(bundle, qt.IsNotNil, qt.Commentf("expected fixture to seed at least one Count>1 commodity"))

	body := `{"data":{"type":"commodity_loans","attributes":{"borrower_name":"Alice","lent_at":"2026-05-01"}}}`
	req := must.Must(http.NewRequest("POST",
		"/api/v1/g/"+testGroup.Slug+"/commodities/"+bundle.ID+"/loans",
		bytes.NewBufferString(body)))
	req.Header.Set("Content-Type", "application/vnd.api+json")
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	apiserver.APIServer(params, mockRestoreWorker).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
}

func TestCommodity_Issue1554_ServiceRejectsBundle(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	all := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	var bundle *models.Commodity
	for _, c := range all {
		if c.Count > 1 {
			bundle = c
			break
		}
	}
	c.Assert(bundle, qt.IsNotNil)

	body := `{"data":{"type":"commodity_services","attributes":{"provider_name":"Repair Shop","sent_at":"2026-05-01"}}}`
	req := must.Must(http.NewRequest("POST",
		"/api/v1/g/"+testGroup.Slug+"/commodities/"+bundle.ID+"/services",
		bytes.NewBufferString(body)))
	req.Header.Set("Content-Type", "application/vnd.api+json")
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	apiserver.APIServer(params, mockRestoreWorker).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
}

func TestCommodity_Issue1554_BumpRejectsWithOpenLoan(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	areas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := areas[0]

	// Seed a fresh Count=1 commodity so the bump check has a clean
	// starting state independent of fixture data.
	commodity := must.Must(registrySet.CommodityRegistry.Create(context.Background(), models.Commodity{
		Name:                  "Single laptop",
		ShortName:             "laptop",
		AreaID:                new(area.ID),
		Type:                  models.CommodityTypeElectronics,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPrice:         must.Must(decimal.NewFromString("1000.00")),
		OriginalPriceCurrency: models.Currency("USD"),
	}))

	// Open a loan on the Count=1 row — perfectly legal today.
	loanBody := `{"data":{"type":"commodity_loans","attributes":{"borrower_name":"Alice","lent_at":"2026-05-01"}}}`
	loanReq := must.Must(http.NewRequest("POST",
		"/api/v1/g/"+testGroup.Slug+"/commodities/"+commodity.ID+"/loans",
		bytes.NewBufferString(loanBody)))
	loanReq.Header.Set("Content-Type", "application/vnd.api+json")
	addTestUserAuthHeader(loanReq, testUser.ID)
	loanRR := httptest.NewRecorder()
	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	apiserver.APIServer(params, mockRestoreWorker).ServeHTTP(loanRR, loanReq)
	c.Assert(loanRR.Code, qt.Equals, http.StatusCreated, qt.Commentf("loan body=%s", loanRR.Body.String()))

	// Now try to bump quantity from 1 to 5 — must fail 422 with a
	// multi-error envelope listing the loan as a blocker.
	updateObj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			ID:   commodity.ID,
			Type: "commodities",
			Attributes: models.WithID(commodity.ID, &models.Commodity{
				Name:                   "Single laptop",
				ShortName:              "laptop",
				AreaID:                 new(area.ID),
				Type:                   models.CommodityTypeElectronics,
				Status:                 models.CommodityStatusInUse,
				Count:                  5,
				OriginalPrice:          must.Must(decimal.NewFromString("1000.00")),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: must.Must(decimal.NewFromString("0")),
				CurrentPrice:           must.Must(decimal.NewFromString("900.00")),
				PurchaseDate:           models.ToPDate("2026-01-01"),
			}),
		},
	}
	data := must.Must(json.Marshal(updateObj))
	req := must.Must(http.NewRequest("PUT",
		"/api/v1/g/"+testGroup.Slug+"/commodities/"+commodity.ID,
		bytes.NewReader(data)))
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()
	apiserver.APIServer(params, mockRestoreWorker).ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	body := rr.Body.Bytes()
	// The multi-error payload should include at least the loan
	// blocker. The error envelope's first entry has its own
	// `error.kind` and `error.source.pointer` fields keyed off the
	// quantityBumpBlockerError shape.
	c.Check(body, checkers.JSONPathEquals("$.errors[0].error.kind"), "loan")
	c.Check(body, checkers.JSONPathEquals("$.errors[0].error.source.pointer"), "/data/attributes/count")
}
