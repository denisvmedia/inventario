package apiserver_test

import (
	"bytes"
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

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())

	req, err := http.NewRequest("GET", "/api/v1/commodities", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedCommodities))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedCommodities[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].name"), expectedCommodities[0].Name)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].short_name"), expectedCommodities[0].ShortName)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].type"), string(expectedCommodities[0].Type))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].area_id"), expectedCommodities[0].AreaID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].count"), float64(expectedCommodities[0].Count))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].original_price"), expectedCommodities[0].OriginalPrice.String())
	c.Assert(body, checkers.JSONPathEquals("$.data[0].original_price_currency"), string(expectedCommodities[0].OriginalPriceCurrency))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].converted_original_price"), expectedCommodities[0].ConvertedOriginalPrice.String())
	c.Assert(body, checkers.JSONPathEquals("$.data[0].current_price"), expectedCommodities[0].CurrentPrice.String())
	c.Assert(body, checkers.JSONPathEquals("$.data[0].serial_number"), expectedCommodities[0].SerialNumber)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].extra_serial_numbers"), expectedCommodities[0].ExtraSerialNumbers)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].part_numbers"), expectedCommodities[0].PartNumbers)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].tags"), expectedCommodities[0].Tags)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].image_ids"), expectedCommodities[0].ImageIDs)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].manual_ids"), expectedCommodities[0].ManualIDs)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].invoice_ids"), expectedCommodities[0].InvoiceIDs)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].status"), string(expectedCommodities[0].Status))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].purchase_date"), expectedCommodities[0].PurchaseDate)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].registered_date"), expectedCommodities[0].RegisteredDate)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].last_modified_date"), expectedCommodities[0].LastModifiedDate)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].comments"), expectedCommodities[0].Comments)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].draft"), expectedCommodities[0].Draft)
}

func TestCommodityGet(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.type"), "commodities")
	c.Assert(body, checkers.JSONPathEquals("$.id"), commodity.ID)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), commodity.Name)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.short_name"), commodity.ShortName)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.urls"), []any{})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.type"), string(commodity.Type))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.area_id"), commodity.AreaID)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.count"), float64(commodity.Count))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.original_price"), commodity.OriginalPrice.String())
	c.Assert(body, checkers.JSONPathEquals("$.attributes.original_price_currency"), string(commodity.OriginalPriceCurrency))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.converted_original_price"), commodity.ConvertedOriginalPrice.String())
	c.Assert(body, checkers.JSONPathEquals("$.attributes.current_price"), commodity.CurrentPrice.String())
	c.Assert(body, checkers.JSONPathEquals("$.attributes.serial_number"), commodity.SerialNumber)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.extra_serial_numbers"), commodity.ExtraSerialNumbers)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.part_numbers"), commodity.PartNumbers)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.tags"), commodity.Tags)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.image_ids"), commodity.ImageIDs)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.manual_ids"), commodity.ManualIDs)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.invoice_ids"), commodity.InvoiceIDs)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.status"), string(commodity.Status))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.purchase_date"), commodity.PurchaseDate)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.registered_date"), commodity.RegisteredDate)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.last_modified_date"), commodity.LastModifiedDate)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.comments"), commodity.Comments)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.draft"), commodity.Draft)
}

func TestCommodityCreate(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedAreas := must.Must(params.AreaRegistry.List())
	area := expectedAreas[1]

	obj := &jsonapi.CommodityRequest{
		Data: &models.Commodity{
			Name:                   "New Commodity in Area 2",
			AreaID:                 area.ID,
			Type:                   models.CommodityTypeElectronics,
			OriginalPrice:          must.Must(decimal.NewFromString("1000.00")),
			OriginalPriceCurrency:  models.Currency("USD"),
			ConvertedOriginalPrice: must.Must(decimal.NewFromString("1200.00")),
			CurrentPrice:           must.Must(decimal.NewFromString("800.00")),
			SerialNumber:           "SN123456",
			ExtraSerialNumbers:     []string{"SN654321"},
			PartNumbers:            []string{"P123", "P456"},
			Tags:                   []string{"tag1", "tag2"},
			ImageIDs:               []string{"img1", "img2"},
			ManualIDs:              []string{"man1", "man2"},
			URLs: []*models.URL{
				must.Must(models.URLParse("https://example.com")),
				must.Must(models.URLParse("https://example.com/2")),
			},
			InvoiceIDs:       []string{"inv1"},
			Status:           models.CommodityStatusInUse,
			PurchaseDate:     "2023-01-01",
			RegisteredDate:   "2023-01-02",
			LastModifiedDate: "2023-01-03",
			Comments:         "New commodity comments",
			Draft:            false,
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("POST", "/api/v1/commodities", buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusCreated)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.type"), "commodities")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), "New Commodity in Area 2")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.short_name"), "")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.urls"), []any{
		"https://example.com",
		"https://example.com/2",
	})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.type"), string(models.CommodityTypeElectronics))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.area_id"), area.ID)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.count"), float64(0))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.original_price"), "1000")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.original_price_currency"), "USD")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.converted_original_price"), "1200")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.current_price"), "800")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.serial_number"), "SN123456")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.extra_serial_numbers"), []any{"SN654321"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.part_numbers"), []any{"P123", "P456"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.tags"), []any{"tag1", "tag2"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.image_ids"), []any{"img1", "img2"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.manual_ids"), []any{"man1", "man2"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.invoice_ids"), []any{"inv1"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.status"), string(models.CommodityStatusInUse))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.purchase_date"), "2023-01-01")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.registered_date"), "2023-01-02")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.last_modified_date"), "2023-01-03")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.comments"), "New commodity comments")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.draft"), false)
}

func TestCommodityUpdate(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	obj := &jsonapi.CommodityRequest{
		Data: &models.Commodity{
			ID:                     commodity.ID,
			Name:                   "Updated Commodity",
			ShortName:              "UC",
			AreaID:                 commodity.AreaID,
			Type:                   models.CommodityTypeFurniture,
			Count:                  10,
			OriginalPrice:          must.Must(decimal.NewFromString("2000.00")),
			OriginalPriceCurrency:  models.Currency("USD"),
			ConvertedOriginalPrice: must.Must(decimal.NewFromString("2400.00")),
			CurrentPrice:           must.Must(decimal.NewFromString("1800.00")),
			SerialNumber:           "SN654321",
			ExtraSerialNumbers:     []string{"SN123456"},
			PartNumbers:            []string{"P789"},
			Tags:                   []string{"tag1", "tag3"},
			ImageIDs:               []string{"img1", "img3"},
			ManualIDs:              []string{"man1", "man3"},
			InvoiceIDs:             []string{"inv2"},
			Status:                 models.CommodityStatusInUse,
			PurchaseDate:           "2022-01-01",
			RegisteredDate:         "2022-01-02",
			LastModifiedDate:       "2022-01-03",
			Comments:               "Updated commodity comments",
			Draft:                  false,
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/commodities/"+commodity.ID, buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathEquals("$.type"), "commodities")
	c.Assert(body, checkers.JSONPathEquals("$.id"), commodity.ID)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.name"), "Updated Commodity")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.short_name"), "UC")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.urls"), []any{})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.type"), string(models.CommodityTypeFurniture))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.area_id"), commodity.AreaID)
	c.Assert(body, checkers.JSONPathEquals("$.attributes.count"), float64(commodity.Count))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.original_price"), "2000")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.original_price_currency"), "USD")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.converted_original_price"), "2400")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.current_price"), "1800")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.serial_number"), "SN654321")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.extra_serial_numbers"), []any{"SN123456"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.part_numbers"), []any{"P789"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.tags"), []any{"tag1", "tag3"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.image_ids"), []any{"img1", "img3"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.manual_ids"), []any{"man1", "man3"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.invoice_ids"), []any{"inv2"})
	c.Assert(body, checkers.JSONPathEquals("$.attributes.status"), string(models.CommodityStatusInUse))
	c.Assert(body, checkers.JSONPathEquals("$.attributes.purchase_date"), "2022-01-01")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.registered_date"), "2022-01-02")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.last_modified_date"), "2022-01-03")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.comments"), "Updated commodity comments")
	c.Assert(body, checkers.JSONPathEquals("$.attributes.draft"), false)
}

func TestCommodityDelete(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
}

func TestCommodityDelete_MissingCommodity(t *testing.T) {
	c := qt.New(t)

	params := newParams()

	missingAreaID := "missing-area-id"
	missingCommodityID := "missing-commodity-id"

	req, err := http.NewRequest("DELETE", "/api/v1/areas/"+missingAreaID+"/commodities/"+missingCommodityID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityUpdate_WrongIDInRequestBody(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	wrongAreaID := "wrong-area-id"
	wrongCommodityID := "wrong-commodity-id"

	obj := &jsonapi.CommodityRequest{
		Data: &models.Commodity{
			ID:                     wrongCommodityID, // Using a different ID in the update request
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
			ImageIDs:               []string{"img1", "img3"},
			ManualIDs:              []string{"man1", "man3"},
			InvoiceIDs:             []string{"inv2"},
			Status:                 models.CommodityStatusInUse,
			PurchaseDate:           "2022-01-01",
			RegisteredDate:         "2022-01-02",
			LastModifiedDate:       "2022-01-03",
			Comments:               "Updated commodity comments",
			Draft:                  false,
		},
	}
	data := must.Must(json.Marshal(obj))
	buf := bytes.NewReader(data)

	req, err := http.NewRequest("PUT", "/api/v1/commodities/"+commodity.ID, buf)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestCommodityListImages(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	imageIDs := params.CommodityRegistry.GetImages(commodity.ID)
	expectedImages := make([]*models.Image, 0, len(imageIDs))
	for _, id := range imageIDs {
		expectedImages = append(expectedImages, must.Must(params.ImageRegistry.Get(id)))
	}

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/images", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedImages))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedImages[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].path"), expectedImages[0].Path)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].commodity_id"), expectedImages[0].CommodityID)
}

func TestCommodityListInvoices(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	invoiceIDs := params.CommodityRegistry.GetInvoices(commodity.ID)
	expectedInvoices := make([]*models.Invoice, 0, len(invoiceIDs))
	for _, id := range invoiceIDs {
		expectedInvoices = append(expectedInvoices, must.Must(params.InvoiceRegistry.Get(id)))
	}

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/invoices", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedInvoices))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedInvoices[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].path"), expectedInvoices[0].Path)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].commodity_id"), expectedInvoices[0].CommodityID)
}

func TestCommodityListManuals(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]

	manualIDs := params.CommodityRegistry.GetManuals(commodity.ID)
	expectedManuals := make([]*models.Manual, 0, len(manualIDs))
	for _, id := range manualIDs {
		expectedManuals = append(expectedManuals, must.Must(params.ManualRegistry.Get(id)))
	}

	req, err := http.NewRequest("GET", "/api/v1/commodities/"+commodity.ID+"/manuals", nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	body := rr.Body.Bytes()

	c.Assert(body, checkers.JSONPathMatches("$.data", qt.HasLen), len(expectedManuals))
	c.Assert(body, checkers.JSONPathEquals("$.data[0].id"), expectedManuals[0].ID)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].path"), expectedManuals[0].Path)
	c.Assert(body, checkers.JSONPathEquals("$.data[0].commodity_id"), expectedManuals[0].CommodityID)
}

func TestCommodityDeleteImage(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]
	imageID := "image-id-to-delete"

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID+"/images/"+imageID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityDeleteInvoice(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]
	invoiceID := "invoice-id-to-delete"

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID+"/invoices/"+invoiceID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestCommodityDeleteManual(t *testing.T) {
	c := qt.New(t)

	params := newParams()
	expectedCommodities := must.Must(params.CommodityRegistry.List())
	commodity := expectedCommodities[0]
	manualID := "manual-id-to-delete"

	req, err := http.NewRequest("DELETE", "/api/v1/commodities/"+commodity.ID+"/manuals/"+manualID, nil)
	c.Assert(err, qt.IsNil)

	rr := httptest.NewRecorder()

	handler := apiserver.APIServer(params)
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}
