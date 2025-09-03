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

	params, testUser := newParams()
	registrySet := getRegistrySetFromParams(params, testUser.ID)
	expectedCommodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))

	req, err := http.NewRequest("GET", "/api/v1/commodities", nil)
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

	params, testUser := newParams()
	registrySet := getRegistrySetFromParams(params, testUser.ID)
	expectedCommodities := must.Must(registrySet.CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]
	expectedImages := sliceToSliceOfAny(getCommodityMeta(c, params, testUser.ID).Images)
	expectedInvoices := sliceToSliceOfAny(getCommodityMeta(c, params, testUser.ID).Invoices)
	expectedManuals := sliceToSliceOfAny(getCommodityMeta(c, params, testUser.ID).Manuals)

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID, nil)
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
	c.Check(body, checkers.JSONPathEquals("$.data.meta.images"), expectedImages)
	c.Check(body, checkers.JSONPathEquals("$.data.meta.invoices"), expectedInvoices)
	c.Check(body, checkers.JSONPathEquals("$.data.meta.manuals"), expectedManuals)
}

func TestCommodityCreate(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	registrySet := getRegistrySetFromParams(params, testUser.ID)
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

	req, err := http.NewRequest("POST", "/api/v1/commodities", buf)
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

	params, testUser := newParams()
	registrySet := getRegistrySetFromParams(params, testUser.ID)
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

	req, err := http.NewRequest("PUT", "/api/v1/commodities/"+commodity.ID, buf)
	c.Check(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)

	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	body := rr.Body.Bytes()
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	expectedImages := sliceToSliceOfAny(getCommodityMeta(c, params, testUser.ID).Images)
	expectedInvoices := sliceToSliceOfAny(getCommodityMeta(c, params, testUser.ID).Invoices)
	expectedManuals := sliceToSliceOfAny(getCommodityMeta(c, params, testUser.ID).Manuals)

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
	c.Check(body, checkers.JSONPathEquals("$.data.meta.images"), expectedImages)
	c.Check(body, checkers.JSONPathEquals("$.data.meta.invoices"), expectedInvoices)
	c.Check(body, checkers.JSONPathEquals("$.data.meta.manuals"), expectedManuals)
}

func TestCommodityDelete(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Check(rr.Code, qt.Equals, http.StatusNoContent)
}

func TestCommodityDelete_MissingCommodity(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()

	missingAreaID := "missing-area-id"
	missingCommodityID := "missing-commodity-id"

	req, err := http.NewRequest("DELETE", "/api/v1/areas/"+missingAreaID+"/commodities/"+missingCommodityID, nil)
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

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(context.Background()))
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

	req, err := http.NewRequest("PUT", "/api/v1/commodities/"+commodity.ID, buf)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestCommodityListImages(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	// Get file entities linked to this commodity with "images" meta
	files, err := getRegistrySetFromParams(params, testUser.ID).FileRegistry.ListByLinkedEntityAndMeta(context.Background(), "commodity", commodity.ID, "images")
	c.Assert(err, qt.IsNil)

	// Convert file entities to legacy image format for comparison
	expectedImages := make([]*models.Image, 0, len(files))
	for _, file := range files {
		image := &models.Image{
			TenantAwareEntityID: models.WithTenantAwareEntityID(file.ID, "default-tenant"),
			CommodityID:         commodity.ID,
			File:                file.File,
		}
		expectedImages = append(expectedImages, image)
	}

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/images", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedImages))
	c.Check(body, checkers.JSONPathEquals("$.data[0].id"), expectedImages[0].ID)
	c.Check(body, checkers.JSONPathEquals("$.data[0].path"), expectedImages[0].Path)
	c.Check(body, checkers.JSONPathEquals("$.data[0].ext"), expectedImages[0].Ext)
	c.Check(body, checkers.JSONPathEquals("$.data[0].commodity_id"), expectedImages[0].CommodityID)
}

func TestCommodityListInvoices(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	// Get file entities linked to this commodity with "invoices" meta
	files, err := getRegistrySetFromParams(params, testUser.ID).FileRegistry.ListByLinkedEntityAndMeta(context.Background(), "commodity", commodity.ID, "invoices")
	c.Assert(err, qt.IsNil)

	// Convert file entities to legacy invoice format for comparison
	expectedInvoices := make([]*models.Invoice, 0, len(files))
	for _, file := range files {
		invoice := &models.Invoice{
			TenantAwareEntityID: models.WithTenantAwareEntityID(file.ID, "default-tenant"),
			CommodityID:         commodity.ID,
			File:                file.File,
		}
		expectedInvoices = append(expectedInvoices, invoice)
	}

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/invoices", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedInvoices))
	c.Check(body, checkers.JSONPathEquals("$.data[0].id"), expectedInvoices[0].ID)
	c.Check(body, checkers.JSONPathEquals("$.data[0].path"), expectedInvoices[0].Path)
	c.Check(body, checkers.JSONPathEquals("$.data[0].ext"), expectedInvoices[0].Ext)
	c.Check(body, checkers.JSONPathEquals("$.data[0].commodity_id"), expectedInvoices[0].CommodityID)
}

func TestCommodityListManuals(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(context.Background()))
	commodity := expectedCommodities[0]

	// Get file entities linked to this commodity with "manuals" meta
	files, err := getRegistrySetFromParams(params, testUser.ID).FileRegistry.ListByLinkedEntityAndMeta(context.Background(), "commodity", commodity.ID, "manuals")
	c.Assert(err, qt.IsNil)

	// Convert file entities to legacy manual format for comparison
	expectedManuals := make([]*models.Manual, 0, len(files))
	for _, file := range files {
		manual := &models.Manual{
			TenantAwareEntityID: models.WithTenantAwareEntityID(file.ID, "default-tenant"),
			CommodityID:         commodity.ID,
			File:                file.File,
		}
		expectedManuals = append(expectedManuals, manual)
	}

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/manuals", nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Check(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedManuals))
	c.Check(body, checkers.JSONPathEquals("$.data[0].id"), expectedManuals[0].ID)
	c.Check(body, checkers.JSONPathEquals("$.data[0].path"), expectedManuals[0].Path)
	c.Check(body, checkers.JSONPathEquals("$.data[0].ext"), expectedManuals[0].Ext)
	c.Check(body, checkers.JSONPathEquals("$.data[0].commodity_id"), expectedManuals[0].CommodityID)
}

func TestCommodityDeleteImage(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	imageID := "image-id-to-delete"

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID+"/images/"+imageID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityDeleteInvoice(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	invoiceID := "invoice-id-to-delete"

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID+"/invoices/"+invoiceID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityDeleteManual(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	manualID := "manual-id-to-delete"

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID+"/manuals/"+manualID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestDownloadImage(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	expectedImage := must.Must(getRegistrySetFromParams(params, testUser.ID).ImageRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	imageID := expectedImage[0].ID
	imageExt := expectedImage[0].File.Ext

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/images/"+imageID+"."+imageExt, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "image/jpeg")
	c.Assert(rr.Body.Bytes(), qt.DeepEquals, []byte("image1"))
}

func TestDownloadImage_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	imageID := "image-id"
	imageExt := "png"

	req, err := http.NewRequest("GET", "/api/v1/commodities/non-existent/images/"+imageID+"."+imageExt, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestDownloadInvoice(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	expectedInvoices := must.Must(getRegistrySetFromParams(params, testUser.ID).InvoiceRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	invoiceID := expectedInvoices[0].ID
	invoiceExt := expectedInvoices[0].File.Ext

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/invoices/"+invoiceID+"."+invoiceExt, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "application/pdf")
	c.Assert(rr.Body.Bytes(), qt.DeepEquals, []byte("invoice1"))
}

func TestDownloadInvoice_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	invoiceID := "invoice-id"
	invoiceExt := "pdf"

	req, err := http.NewRequest("GET", "/api/v1/commodities/non-existent/invoices/"+invoiceID+"."+invoiceExt, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestDownloadManual(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	expectedManuals := must.Must(getRegistrySetFromParams(params, testUser.ID).ManualRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	manualID := expectedManuals[0].ID
	manualExt := expectedManuals[0].File.Ext

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/manuals/"+manualID+"."+manualExt, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Header().Get("Content-Type"), qt.Equals, "application/pdf")
	c.Assert(rr.Body.Bytes(), qt.DeepEquals, []byte("manual1"))
}

func TestDownloadManual_CommodityNotFound(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	manualID := "manual-id"
	manualExt := "pdf"

	req, err := http.NewRequest("GET", "/api/v1/commodities/non-existent/manuals/"+manualID+"."+manualExt, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestGetImageData(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]

	// Get file entities linked to this commodity with "images" meta
	files, err := getRegistrySetFromParams(params, testUser.ID).FileRegistry.ListByLinkedEntityAndMeta(c.Context(), "commodity", commodity.ID, "images")
	c.Assert(err, qt.IsNil)
	c.Assert(len(files), qt.Not(qt.Equals), 0)
	imageID := files[0].ID

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/images/"+imageID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	// Get the expected file entity
	expectedFile := must.Must(getRegistrySetFromParams(params, testUser.ID).FileRegistry.Get(c.Context(), imageID))

	c.Check(body, checkers.JSONPathEquals("$.type"), "images")
	c.Check(body, checkers.JSONPathEquals("$.id"), expectedFile.ID)
	c.Check(body, checkers.JSONPathEquals("$.attributes.path"), expectedFile.File.Path)
	c.Check(body, checkers.JSONPathEquals("$.attributes.ext"), expectedFile.File.Ext)
	c.Check(body, checkers.JSONPathEquals("$.attributes.commodity_id"), commodity.ID)
}

func TestGetImageData_ImageNotFound(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	nonExistentImageID := "non-existent-image-id"

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/images/"+nonExistentImageID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestGetInvoiceData(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]

	// Get file entities linked to this commodity with "invoices" meta
	files, err := getRegistrySetFromParams(params, testUser.ID).FileRegistry.ListByLinkedEntityAndMeta(c.Context(), "commodity", commodity.ID, "invoices")
	c.Assert(err, qt.IsNil)
	c.Assert(len(files), qt.Not(qt.Equals), 0)
	invoiceID := files[0].ID

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/invoices/"+invoiceID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	// Get the expected file entity
	expectedFile := must.Must(getRegistrySetFromParams(params, testUser.ID).FileRegistry.Get(c.Context(), invoiceID))

	c.Check(body, checkers.JSONPathEquals("$.type"), "invoices")
	c.Check(body, checkers.JSONPathEquals("$.id"), expectedFile.ID)
	c.Check(body, checkers.JSONPathEquals("$.attributes.path"), expectedFile.File.Path)
	c.Check(body, checkers.JSONPathEquals("$.attributes.ext"), expectedFile.File.Ext)
	c.Check(body, checkers.JSONPathEquals("$.attributes.commodity_id"), commodity.ID)
}

func TestGetInvoiceData_InvoiceNotFound(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	nonExistentInvoiceID := "non-existent-invoice-id"

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/invoices/"+nonExistentInvoiceID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestGetManualsData(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]

	// Get file entities linked to this commodity with "manuals" meta
	files, err := getRegistrySetFromParams(params, testUser.ID).FileRegistry.ListByLinkedEntityAndMeta(c.Context(), "commodity", commodity.ID, "manuals")
	c.Assert(err, qt.IsNil)
	c.Assert(len(files), qt.Not(qt.Equals), 0)
	manualID := files[0].ID

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/manuals/"+manualID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	// Get the expected file entity
	expectedFile := must.Must(getRegistrySetFromParams(params, testUser.ID).FileRegistry.Get(c.Context(), manualID))

	c.Check(body, checkers.JSONPathEquals("$.type"), "manuals")
	c.Check(body, checkers.JSONPathEquals("$.id"), expectedFile.ID)
	c.Check(body, checkers.JSONPathEquals("$.attributes.path"), expectedFile.File.Path)
	c.Check(body, checkers.JSONPathEquals("$.attributes.ext"), expectedFile.File.Ext)
	c.Check(body, checkers.JSONPathEquals("$.attributes.commodity_id"), commodity.ID)
}

func TestGetManualsData_ManualNotFound(t *testing.T) {
	c := qt.New(t)

	params, testUser := newParams()
	expectedCommodities := must.Must(getRegistrySetFromParams(params, testUser.ID).CommodityRegistry.List(c.Context()))
	commodity := expectedCommodities[0]
	nonExistentManualID := "non-existent-manual-id"

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/manuals/"+nonExistentManualID, nil)
	c.Assert(err, qt.IsNil)
	addTestUserAuthHeader(req, testUser.ID)
	rr := httptest.NewRecorder()

	mockRestoreWorker := &mockRestoreWorker{hasRunningRestores: false}
	handler := apiserver.APIServer(params, mockRestoreWorker)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func getCommodityMeta(c *qt.C, params apiserver.Params, userID string) *jsonapi.CommodityMeta {
	registrySet := getRegistrySetFromParams(params, userID)
	expectedImages, err := registrySet.ImageRegistry.List(c.Context())
	c.Assert(err, qt.IsNil)
	images := make([]string, 0, len(expectedImages))
	for _, image := range expectedImages {
		images = append(images, image.ID)
	}

	expectedInvoices, err := registrySet.InvoiceRegistry.List(c.Context())
	c.Assert(err, qt.IsNil)
	invoices := make([]string, 0, len(expectedInvoices))
	for _, invoice := range expectedInvoices {
		invoices = append(invoices, invoice.ID)
	}

	expectedManuals, err := registrySet.ManualRegistry.List(c.Context())
	c.Assert(err, qt.IsNil)
	manuals := make([]string, 0, len(expectedManuals))
	for _, manual := range expectedManuals {
		manuals = append(manuals, manual.ID)
	}

	return &jsonapi.CommodityMeta{
		Images:   images,
		Invoices: invoices,
		Manuals:  manuals,
	}
}
