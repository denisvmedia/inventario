package apiserver_test

import (
	"fmt"
	"net/textproto"
	"strings"

	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func newLocationRegistry() registry.LocationRegistry {
	var locationsRegistry = registry.NewMemoryLocationRegistry()

	must.Must(locationsRegistry.Create(models.Location{
		Name:    "Location 1",
		Address: "Address 1",
	}))

	must.Must(locationsRegistry.Create(models.Location{
		Name:    "Location 2",
		Address: "Address 2",
	}))

	return locationsRegistry
}

func newAreaRegistry(locationRegistry registry.LocationRegistry) registry.AreaRegistry {
	var areaRegistry = registry.NewMemoryAreaRegistry(locationRegistry)

	locations := must.Must(locationRegistry.List())

	must.Must(areaRegistry.Create(models.Area{
		ID:         "1",
		Name:       "Area 1",
		LocationID: locations[0].ID,
	}))

	must.Must(areaRegistry.Create(models.Area{
		ID:         "2",
		Name:       "Area 2",
		LocationID: locations[0].ID,
	}))

	return areaRegistry
}

func newCommodityRegistry(areaRegistry registry.AreaRegistry) registry.CommodityRegistry {
	var commodityRegistry = registry.NewMemoryCommodityRegistry(areaRegistry)

	areas := must.Must(areaRegistry.List())

	must.Must(commodityRegistry.Create(models.Commodity{
		ID:            "1",
		Name:          "Commodity 1",
		ShortName:     "C1",
		AreaID:        areas[0].ID,
		Type:          models.CommodityTypeFurniture,
		Count:         10,
		OriginalPrice: must.Must(decimal.NewFromString("2000.00")),
	}))

	must.Must(commodityRegistry.Create(models.Commodity{
		ID:            "2",
		Name:          "Commodity 2",
		ShortName:     "C2",
		AreaID:        areas[0].ID,
		Type:          models.CommodityTypeElectronics,
		Count:         5,
		OriginalPrice: must.Must(decimal.NewFromString("1500.00")),
	}))

	return commodityRegistry
}

func newImageRegistry(commodityRegistry registry.CommodityRegistry) registry.ImageRegistry {
	var imageRegistry = registry.NewMemoryImageRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List())

	must.Must(imageRegistry.Create(models.Image{
		ID:          "img1",
		Path:        "/path/to/image1.jpg",
		CommodityID: commodities[0].ID,
	}))

	must.Must(imageRegistry.Create(models.Image{
		ID:          "img2",
		Path:        "/path/to/image2.jpg",
		CommodityID: commodities[0].ID,
	}))

	return imageRegistry
}

func newInvoiceRegistry(commodityRegistry registry.CommodityRegistry) registry.InvoiceRegistry {
	var invoiceRegistry = registry.NewMemoryInvoiceRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List())

	must.Must(invoiceRegistry.Create(models.Invoice{
		ID:          "inv1",
		Path:        "path/to/invoice1.pdf",
		CommodityID: commodities[0].ID,
	}))

	must.Must(invoiceRegistry.Create(models.Invoice{
		ID:          "inv2",
		Path:        "path/to/invoice2.pdf",
		CommodityID: commodities[0].ID,
	}))

	return invoiceRegistry
}

func newManualRegistry(commodityRegistry registry.CommodityRegistry) registry.ManualRegistry {
	var manualRegistry = registry.NewMemoryManualRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List())

	must.Must(manualRegistry.Create(models.Manual{
		ID:          "man1",
		Path:        "/path/to/manual1.pdf",
		CommodityID: commodities[0].ID,
	}))

	must.Must(manualRegistry.Create(models.Manual{
		ID:          "man2",
		Path:        "/path/to/manual2.pdf",
		CommodityID: commodities[0].ID,
	}))

	return manualRegistry
}

func newParams() apiserver.Params {
	var params apiserver.Params
	params.LocationRegistry = newLocationRegistry()
	params.AreaRegistry = newAreaRegistry(params.LocationRegistry)
	params.CommodityRegistry = newCommodityRegistry(params.AreaRegistry)
	params.ImageRegistry = newImageRegistry(params.CommodityRegistry)
	params.InvoiceRegistry = newInvoiceRegistry(params.CommodityRegistry)
	params.ManualRegistry = newManualRegistry(params.CommodityRegistry)
	params.UploadLocation = "mem://uploads"
	return params
}

// src: mime/multipart/writer.go
var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

// src: mime/multipart/writer.go
func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

// original code: mime/multipart/writer.go
// CreateFormFileMIME creates a new form-data header with the provided field name,
// file name and content type.
func CreateFormFileMIME(fieldname, filename, contentType string) textproto.MIMEHeader {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition",
		fmt.Sprintf(`form-data; name="%s"; filename="%s"`,
			escapeQuotes(fieldname), escapeQuotes(filename)))
	h.Set("Content-Type", contentType)
	return h
}

func sliceToSliceOfAny[T any](v []T) (result []any) {
	for _, item := range v {
		result = append(result, item)
	}
	return result
}
