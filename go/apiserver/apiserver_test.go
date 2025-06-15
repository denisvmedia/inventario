package apiserver_test

import (
	"context"
	"fmt"
	"net/textproto"
	"strings"

	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

const uploadLocation = "file://uploads?memfs=1&create_dir=1"

func newLocationRegistry() registry.LocationRegistry {
	var locationsRegistry = memory.NewLocationRegistry()

	must.Must(locationsRegistry.Create(context.Background(), models.Location{
		Name:    "Location 1",
		Address: "Address 1",
	}))

	must.Must(locationsRegistry.Create(context.Background(), models.Location{
		Name:    "Location 2",
		Address: "Address 2",
	}))

	return locationsRegistry
}

func newAreaRegistry(locationRegistry registry.LocationRegistry) registry.AreaRegistry {
	var areaRegistry = memory.NewAreaRegistry(locationRegistry)

	locations := must.Must(locationRegistry.List(context.Background()))

	must.Must(areaRegistry.Create(context.Background(), models.Area{
		EntityID:   models.EntityID{ID: "1"},
		Name:       "Area 1",
		LocationID: locations[0].ID,
	}))

	must.Must(areaRegistry.Create(context.Background(), models.Area{
		EntityID:   models.EntityID{ID: "2"},
		Name:       "Area 2",
		LocationID: locations[0].ID,
	}))

	return areaRegistry
}

func newCommodityRegistry(areaRegistry registry.AreaRegistry) registry.CommodityRegistry {
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)

	areas := must.Must(areaRegistry.List(context.Background()))

	must.Must(commodityRegistry.Create(context.Background(), models.Commodity{
		Name:                  "Commodity 1",
		ShortName:             "C1",
		AreaID:                areas[0].ID,
		Type:                  models.CommodityTypeFurniture,
		Status:                models.CommodityStatusInUse,
		Count:                 10,
		OriginalPrice:         must.Must(decimal.NewFromString("2000.00")),
		OriginalPriceCurrency: models.Currency("USD"),
	}))

	must.Must(commodityRegistry.Create(context.Background(), models.Commodity{
		Name:                  "Commodity 2",
		ShortName:             "C2",
		AreaID:                areas[0].ID,
		Status:                models.CommodityStatusInUse,
		Type:                  models.CommodityTypeElectronics,
		Count:                 5,
		OriginalPrice:         must.Must(decimal.NewFromString("1500.00")),
		OriginalPriceCurrency: models.Currency("USD"),
	}))

	return commodityRegistry
}

func newImageRegistry(commodityRegistry registry.CommodityRegistry) registry.ImageRegistry {
	var imageRegistry = memory.NewImageRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List(context.Background()))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "image1.jpg", []byte("image1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(imageRegistry.Create(context.Background(), models.Image{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "image1",     // Without extension
			OriginalPath: "image1.jpg", // This is the actual file name in storage
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	b = must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err = b.WriteAll(context.TODO(), "image2.jpg", []byte("image2"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(imageRegistry.Create(context.Background(), models.Image{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "image2",     // Without extension
			OriginalPath: "image2.jpg", // This is the actual file name in storage
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	return imageRegistry
}

func newInvoiceRegistry(commodityRegistry registry.CommodityRegistry) registry.InvoiceRegistry {
	var invoiceRegistry = memory.NewInvoiceRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List(context.Background()))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "invoice1.pdf", []byte("invoice1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(invoiceRegistry.Create(context.Background(), models.Invoice{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "invoice1",     // Without extension
			OriginalPath: "invoice1.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	b = must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err = b.WriteAll(context.TODO(), "invoice2.pdf", []byte("invoice2"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(invoiceRegistry.Create(context.Background(), models.Invoice{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "invoice2",     // Without extension
			OriginalPath: "invoice2.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	return invoiceRegistry
}

func newManualRegistry(commodityRegistry registry.CommodityRegistry) registry.ManualRegistry {
	var manualRegistry = memory.NewManualRegistry(commodityRegistry)

	commodities := must.Must(commodityRegistry.List(context.Background()))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "manual1.pdf", []byte("manual1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(manualRegistry.Create(context.Background(), models.Manual{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "manual1",     // Without extension
			OriginalPath: "manual1.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	b = must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err = b.WriteAll(context.TODO(), "manual2.pdf", []byte("manual2"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(manualRegistry.Create(context.Background(), models.Manual{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "manual2",     // Without extension
			OriginalPath: "manual2.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	return manualRegistry
}

func newSettingsRegistry() registry.SettingsRegistry {
	var settingsRegistry = memory.NewSettingsRegistry()

	must.Assert(settingsRegistry.Patch(context.Background(), "system.main_currency", "USD"))

	return settingsRegistry
}

func newParams() apiserver.Params {
	var params apiserver.Params
	params.RegistrySet = &registry.Set{}
	params.RegistrySet.LocationRegistry = newLocationRegistry()
	params.RegistrySet.AreaRegistry = newAreaRegistry(params.RegistrySet.LocationRegistry)
	params.RegistrySet.SettingsRegistry = newSettingsRegistry()
	params.RegistrySet.CommodityRegistry = newCommodityRegistry(params.RegistrySet.AreaRegistry)
	params.RegistrySet.ImageRegistry = newImageRegistry(params.RegistrySet.CommodityRegistry)
	params.RegistrySet.InvoiceRegistry = newInvoiceRegistry(params.RegistrySet.CommodityRegistry)
	params.RegistrySet.ManualRegistry = newManualRegistry(params.RegistrySet.CommodityRegistry)
	params.UploadLocation = uploadLocation
	return params
}

func newParamsAreaRegistryOnly() apiserver.Params {
	var params apiserver.Params
	params.RegistrySet = &registry.Set{}
	params.RegistrySet.LocationRegistry = newLocationRegistry()
	params.RegistrySet.AreaRegistry = newAreaRegistry(params.RegistrySet.LocationRegistry)
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
