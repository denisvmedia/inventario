package apiserver_test

import (
	"context"
	"fmt"
	"net/textproto"
	"strings"

	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"
	_ "gocloud.dev/blob/fileblob"

	"github.com/denisvmedia/inventario/apiserver"
	_ "github.com/denisvmedia/inventario/internal/fileblob"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

const uploadLocation = "afile://uploads?memfs=1&create_dir=1"

func newLocationRegistry() registry.LocationRegistry {
	var locationsRegistry = memory.NewLocationRegistry()

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
	var areaRegistry = memory.NewAreaRegistry(locationRegistry)

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
	var commodityRegistry = memory.NewCommodityRegistry(areaRegistry)

	areas := must.Must(areaRegistry.List())

	must.Must(commodityRegistry.Create(models.Commodity{
		Name:          "Commodity 1",
		ShortName:     "C1",
		AreaID:        areas[0].ID,
		Type:          models.CommodityTypeFurniture,
		Status:        models.CommodityStatusInUse,
		Count:         10,
		OriginalPrice: must.Must(decimal.NewFromString("2000.00")),
	}))

	must.Must(commodityRegistry.Create(models.Commodity{
		Name:          "Commodity 2",
		ShortName:     "C2",
		AreaID:        areas[0].ID,
		Status:        models.CommodityStatusInUse,
		Type:          models.CommodityTypeElectronics,
		Count:         5,
		OriginalPrice: must.Must(decimal.NewFromString("1500.00")),
	}))

	return commodityRegistry
}

func newImageRegistry(commodityRegistry registry.CommodityRegistry) registry.ImageRegistry {
	var imageRegistry = memory.NewImageRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List())

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "image1.jpg", []byte("image1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(imageRegistry.Create(models.Image{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:     "image1.jpg",
			Ext:      ".jpg",
			MIMEType: "image/jpeg",
		},
	}))

	b = must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err = b.WriteAll(context.TODO(), "image2.jpg", []byte("image2"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(imageRegistry.Create(models.Image{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:     "image2.jpg",
			Ext:      ".jpg",
			MIMEType: "image/jpeg",
		},
	}))

	return imageRegistry
}

func newInvoiceRegistry(commodityRegistry registry.CommodityRegistry) registry.InvoiceRegistry {
	var invoiceRegistry = memory.NewInvoiceRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List())

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "invoice1.pdf", []byte("invoice1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(invoiceRegistry.Create(models.Invoice{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:     "invoice1.pdf",
			Ext:      ".pdf",
			MIMEType: "application/pdf",
		},
	}))

	b = must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err = b.WriteAll(context.TODO(), "invoice2.pdf", []byte("invoice2"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(invoiceRegistry.Create(models.Invoice{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:     "invoice2.pdf",
			Ext:      ".pdf",
			MIMEType: "application/pdf",
		},
	}))

	return invoiceRegistry
}

func newManualRegistry(commodityRegistry registry.CommodityRegistry) registry.ManualRegistry {
	var manualRegistry = memory.NewManualRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List())

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "manual1.pdf", []byte("manual1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(manualRegistry.Create(models.Manual{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:     "manual1.pdf",
			Ext:      ".pdf",
			MIMEType: "application/pdf",
		},
	}))

	b = must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err = b.WriteAll(context.TODO(), "manual2.pdf", []byte("manual2"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(manualRegistry.Create(models.Manual{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:     "manual2.pdf",
			Ext:      ".pdf",
			MIMEType: "application/pdf",
		},
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
	params.UploadLocation = uploadLocation
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
