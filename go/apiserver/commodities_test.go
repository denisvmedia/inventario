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

func TestCommoditiesList(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	expectedCommodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/commodities", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedCommodities))
	c.Check(body, checkers.JSONPathEquals("$.data[0].id"), expectedCommodities[0].ID)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.name"), expectedCommodities[0].Name)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.short_name"), expectedCommodities[0].ShortName)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.type"), string(expectedCommodities[0].Type))
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.area_id"), expectedCommodities[0].AreaID)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.count"), float64(expectedCommodities[0].Count))
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.original_price"), expectedCommodities[0].OriginalPrice.String())
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.original_price_currency"), string(expectedCommodities[0].OriginalPriceCurrency))
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.converted_original_price"), expectedCommodities[0].ConvertedOriginalPrice.String())
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.current_price"), expectedCommodities[0].CurrentPrice.String())
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.serial_number"), expectedCommodities[0].SerialNumber)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.extra_serial_numbers"), expectedCommodities[0].ExtraSerialNumbers)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.part_numbers"), expectedCommodities[0].PartNumbers)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.tags"), expectedCommodities[0].Tags)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.status"), string(expectedCommodities[0].Status))
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.purchase_date"), expectedCommodities[0].PurchaseDate)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.registered_date"), expectedCommodities[0].RegisteredDate)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.last_modified_date"), expectedCommodities[0].LastModifiedDate)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.comments"), expectedCommodities[0].Comments)
	c.Check(body, checkers.JSONPathEquals("$.data[0].attributes.draft"), expectedCommodities[0].Draft)
}

func TestCommodityGet(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	expectedCommodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	req, err := http.NewRequest("GET", "/api/v1/g/"+testGroup.Slug+"/commodities/"+commodity.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Check(body, checkers.JSONPathEquals("$.data.type"), "commodities")
	c.Check(body, checkers.JSONPathEquals("$.data.id"), commodity.ID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.name"), commodity.Name)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.short_name"), commodity.ShortName)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.urls"), nil)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.type"), string(commodity.Type))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.area_id"), commodity.AreaID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.count"), float64(commodity.Count))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), commodity.OriginalPrice.String())
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price_currency"), string(commodity.OriginalPriceCurrency))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.converted_original_price"), commodity.ConvertedOriginalPrice.String())
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), commodity.CurrentPrice.String())
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.serial_number"), commodity.SerialNumber)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.extra_serial_numbers"), commodity.ExtraSerialNumbers)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.part_numbers"), commodity.PartNumbers)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.tags"), commodity.Tags)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.status"), string(commodity.Status))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.purchase_date"), commodity.PurchaseDate)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.registered_date"), commodity.RegisteredDate)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.last_modified_date"), commodity.LastModifiedDate)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.comments"), commodity.Comments)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.draft"), commodity.Draft)
}

func TestCommodityCreate(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	expectedAreas := must.Must(registrySet.AreaRegistry.List(context.Background()))
	area := expectedAreas[1]

	urls := []*models.URL{
		must.Must(models.URLParse("https://example.com")),
		must.Must(models.URLParse("https://example.com/2")),
	}

	obj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			Type: "commodities",
			Attributes: &models.Commodity{
				Name:                   "New Commodity in Area 2",
				ShortName:              "NewCom2",
				AreaID:                 area.ID,
				Type:                   models.CommodityTypeElectronics,
				OriginalPrice:          must.Must(decimal.NewFromString("1000.00")),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: must.Must(decimal.NewFromString("0")), // to pass the validation
				CurrentPrice:           must.Must(decimal.NewFromString("800.00")),
				SerialNumber:           "SN123456",
				ExtraSerialNumbers:     []string{"SN654321"},
				PartNumbers:            []string{"P123", "P456"},
				Tags:                   []string{"tag1", "tag2"},
				URLs:                   urls,
				Count:                  1,
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Comments:               "New commodity comments",
				Draft:                  false,
			},
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/commodities", buf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	body := rr.Body.Bytes()
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("Body: %s", body))

	c.Check(body, checkers.JSONPathEquals("$.data.type"), "commodities")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.name"), "New Commodity in Area 2")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.short_name"), "NewCom2")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.urls"), []any{"https://example.com", "https://example.com/2"})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.type"), string(models.CommodityTypeElectronics))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.area_id"), area.ID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.count"), float64(1))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), "1000")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price_currency"), "USD")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.converted_original_price"), "0")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), "800")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.serial_number"), "SN123456")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.extra_serial_numbers"), []any{"SN654321"})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.part_numbers"), []any{"P123", "P456"})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.tags"), []any{"tag1", "tag2"})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.status"), string(models.CommodityStatusInUse))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.purchase_date"), "2023-01-01")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.registered_date"), "2023-01-02")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.last_modified_date"), "2023-01-03")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.comments"), "New commodity comments")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.draft"), false)
}

func TestCommodityUpdate(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	expectedCommodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	obj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			ID:   commodity.ID,
			Type: "commodities",
			Attributes: models.WithID(commodity.ID, &models.Commodity{
				Name:                   "Updated Commodity",
				ShortName:              "UC",
				AreaID:                 commodity.AreaID,
				Type:                   models.CommodityTypeFurniture,
				Count:                  10,
				OriginalPrice:          must.Must(decimal.NewFromString("2000.00")),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: must.Must(decimal.NewFromString("0")), // to pass the validation
				CurrentPrice:           must.Must(decimal.NewFromString("1800.00")),
				SerialNumber:           "SN654321",
				ExtraSerialNumbers:     []string{"SN123456"},
				PartNumbers:            []string{"P789"},
				Tags:                   []string{"tag1", "tag3"},
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2022-01-01"),
				RegisteredDate:         models.ToPDate("2022-01-02"),
				LastModifiedDate:       models.ToPDate("2022-01-03"),
				Comments:               "Updated commodity comments",
				Draft:                  false,
			}),
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/commodities/"+commodity.ID, buf)
	c.Check(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	body := rr.Body.Bytes()
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	c.Check(body, checkers.JSONPathEquals("$.data.type"), "commodities")
	c.Check(body, checkers.JSONPathEquals("$.data.id"), commodity.ID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.name"), "Updated Commodity")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.short_name"), "UC")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.urls"), nil)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.type"), string(models.CommodityTypeFurniture))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.area_id"), commodity.AreaID)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.count"), float64(commodity.Count))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price"), "2000")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.original_price_currency"), "USD")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.converted_original_price"), "0")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.current_price"), "1800")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.serial_number"), "SN654321")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.extra_serial_numbers"), []any{"SN123456"})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.part_numbers"), []any{"P789"})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.tags"), []any{"tag1", "tag3"})
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.status"), string(models.CommodityStatusInUse))
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.purchase_date"), "2022-01-01")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.registered_date"), "2022-01-02")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.last_modified_date"), "2022-01-03")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.comments"), "Updated commodity comments")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.draft"), false)
}

func TestCommodityDelete(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser).CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/commodities/"+commodity.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Check(rr.Code, qt.Equals, http.StatusNoContent)
}

func TestCommodityBulkDelete(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	all := must.Must(getRegistrySetFromParams(params, testUser).CommodityRegistry.List(context.Background()))
	c.Assert(len(all) >= 1, qt.IsTrue, qt.Commentf("Expected at least one commodity in fixture"))
	targetID := all[0].ID

	body := `{"data":{"type":"commodities","attributes":{"ids":["` + targetID + `","does-not-exist"]}}}`
	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/commodities/bulk-delete", bytes.NewBufferString(body))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	resp := rr.Body.String()
	c.Check(resp, checkers.JSONPathEquals("$.data.attributes.succeeded[0]"), targetID)
	c.Check(resp, checkers.JSONPathEquals("$.data.attributes.failed[0].id"), "does-not-exist")
}

func TestCommodityBulkMove(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	registrySet := getRegistrySetFromParams(params, testUser)
	all := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	c.Assert(len(all) >= 1, qt.IsTrue)
	target := all[0]

	// Create a fresh destination area in the same location so the move is valid.
	destArea, err := registrySet.AreaRegistry.Create(context.Background(), models.Area{
		Name:       "Bulk Move Destination",
		LocationID: must.Must(registrySet.AreaRegistry.Get(context.Background(), target.AreaID)).LocationID,
	})
	c.Assert(err, qt.IsNil)

	body := `{"data":{"type":"commodities","attributes":{"ids":["` + target.ID + `"],"area_id":"` + destArea.ID + `"}}}`
	req, err := http.NewRequest("POST", "/api/v1/g/"+testGroup.Slug+"/commodities/bulk-move", bytes.NewBufferString(body))
	c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded[0]"), target.ID)

	updated := must.Must(registrySet.CommodityRegistry.Get(context.Background(), target.ID))
	c.Check(updated.AreaID, qt.Equals, destArea.ID)
}

func TestCommodityDelete_MissingCommodity(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()

	missingAreaID := "missing-area-id"
	missingCommodityID := "missing-commodity-id"

	req, err := http.NewRequest("DELETE", "/api/v1/g/"+testGroup.Slug+"/areas/"+missingAreaID+"/commodities/"+missingCommodityID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityUpdate_WrongIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	params, testUser, testGroup := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser).CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	wrongAreaID := "wrong-area-id"
	wrongCommodityID := "wrong-commodity-id"

	obj := &jsonapi.CommodityRequest{
		Data: &jsonapi.CommodityData{
			ID:   wrongCommodityID,
			Type: "commodities",
			Attributes: models.WithID(wrongCommodityID, &models.Commodity{
				Name:                   "Updated Commodity",
				ShortName:              "UC",
				AreaID:                 wrongAreaID,
				Type:                   models.CommodityTypeFurniture,
				OriginalPrice:          must.Must(decimal.NewFromString("2000.00")),
				OriginalPriceCurrency:  models.Currency("USD"),
				ConvertedOriginalPrice: must.Must(decimal.NewFromString("2400.00")),
				CurrentPrice:           must.Must(decimal.NewFromString("1800.00")),
				SerialNumber:           "SN654321",
				ExtraSerialNumbers:     []string{"SN123456"},
				PartNumbers:            []string{"P789"},
				Tags:                   []string{"tag1", "tag3"},
				Status:                 models.CommodityStatusInUse,
				PurchaseDate:           models.ToPDate("2023-01-01"),
				RegisteredDate:         models.ToPDate("2023-01-02"),
				LastModifiedDate:       models.ToPDate("2023-01-03"),
				Comments:               "Updated commodity comments",
				Draft:                  false,
			}),
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/g/"+testGroup.Slug+"/commodities/"+commodity.ID, buf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

// Legacy `/commodities/{id}/{images,invoices,manuals}*` route tests
// (formerly `TestCommodity{ListImages,ListInvoices,ListManuals,
// DeleteImage,DeleteInvoice,DeleteManual}`, `TestDownload{Image,Invoice,
// Manual}*`, `TestGet{Image,Invoice,Manuals}Data*`) were removed under
// #1421 alongside the routes themselves. The unified `/files` surface
// covers the same reads via `?linked_entity_type=commodity&linked_entity_id=…`
// and is exercised by the file-registry + apiserver/files tests.
