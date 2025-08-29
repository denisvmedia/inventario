package apiserver_test

import (
	"context"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"
	"time"

	"github.com/go-extras/go-kit/must"
	"github.com/golang-jwt/jwt/v5"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/appctx"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register fileblob driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

const uploadLocation = "file://uploads?memfs=1&create_dir=1"

// Test JWT secret for authentication
var testJWTSecret = []byte("test-jwt-secret-32-bytes-minimum-length")

func newLocationRegistry() registry.LocationRegistry {
	var locationsRegistry registry.LocationRegistry = memory.NewLocationRegistry()
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	locationsRegistry = must.Must(locationsRegistry.WithCurrentUser(ctx))

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
	var areaRegistry registry.AreaRegistry = memory.NewAreaRegistry(locationRegistry.(*memory.LocationRegistry))
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	areaRegistry = must.Must(areaRegistry.WithCurrentUser(ctx))
	locations := must.Must(must.Must(locationRegistry.WithCurrentUser(ctx)).List(context.Background()))

	must.Must(areaRegistry.Create(context.Background(), models.Area{
		TenantAwareEntityID: models.WithTenantAwareEntityID("1", "default-tenant"),
		Name:                "Area 1",
		LocationID:          locations[0].ID,
	}))

	must.Must(areaRegistry.Create(context.Background(), models.Area{
		TenantAwareEntityID: models.WithTenantAwareEntityID("2", "default-tenant"),
		Name:                "Area 2",
		LocationID:          locations[0].ID,
	}))

	return areaRegistry
}

func newCommodityRegistry(areaRegistry registry.AreaRegistry) registry.CommodityRegistry {
	var commodityRegistry registry.CommodityRegistry = memory.NewCommodityRegistry(areaRegistry.(*memory.AreaRegistry))
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	commodityRegistry = must.Must(commodityRegistry.WithCurrentUser(ctx))

	areaRegistry = must.Must(areaRegistry.WithCurrentUser(ctx))
	commodityRegistry = must.Must(commodityRegistry.WithCurrentUser(ctx))

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
	var imageRegistry = memory.NewImageRegistry(commodityRegistry.(*memory.CommodityRegistry))

	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	commodities := must.Must(commodityRegistry.List(ctx))
	imgReg := must.Must(imageRegistry.WithCurrentUser(ctx))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "image1.jpg", []byte("image1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(imgReg.Create(ctx, models.Image{
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

	must.Must(imgReg.Create(ctx, models.Image{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "image2",     // Without extension
			OriginalPath: "image2.jpg", // This is the actual file name in storage
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	return imgReg
}

func newInvoiceRegistry(commodityRegistry registry.CommodityRegistry) registry.InvoiceRegistry {
	var invoiceRegistry = memory.NewInvoiceRegistry(commodityRegistry.(*memory.CommodityRegistry))

	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	commodities := must.Must(commodityRegistry.List(ctx))
	invReg := must.Must(invoiceRegistry.WithCurrentUser(ctx))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "invoice1.pdf", []byte("invoice1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(invReg.Create(ctx, models.Invoice{
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

	must.Must(invReg.Create(ctx, models.Invoice{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "invoice2",     // Without extension
			OriginalPath: "invoice2.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	return invReg
}

func newManualRegistry(commodityRegistry registry.CommodityRegistry) registry.ManualRegistry {
	var manualRegistry = memory.NewManualRegistry(commodityRegistry.(*memory.CommodityRegistry))

	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	commodities := must.Must(commodityRegistry.List(ctx))
	manReg := must.Must(manualRegistry.WithCurrentUser(ctx))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "manual1.pdf", []byte("manual1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(manReg.Create(ctx, models.Manual{
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

	must.Must(manReg.Create(ctx, models.Manual{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "manual2",     // Without extension
			OriginalPath: "manual2.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	return manReg
}

func newSettingsRegistry() registry.SettingsRegistry {
	var settingsRegistry registry.SettingsRegistry = memory.NewSettingsRegistry()
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	settingsRegistry = must.Must(settingsRegistry.WithCurrentUser(ctx))

	must.Assert(settingsRegistry.Patch(context.Background(), "system.main_currency", "USD"))

	return settingsRegistry
}

func newUserRegistry() registry.UserRegistry {
	var userRegistry = memory.NewUserRegistry()

	// Create a test user for authentication
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-user-id"},
			TenantID: "test-tenant-id",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	must.Assert(testUser.SetPassword("password123"))
	must.Must(userRegistry.Create(context.Background(), testUser))

	return userRegistry
}

// createTestJWTToken creates a JWT token for testing
func createTestJWTToken(userID string, role models.UserRole) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"role":    string(role),
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(testJWTSecret)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test JWT token: %v", err))
	}

	return tokenString
}

// addAuthHeader adds JWT authentication header to a request
func addAuthHeader(req *http.Request, userID string, role models.UserRole) {
	token := createTestJWTToken(userID, role)
	req.Header.Set("Authorization", "Bearer "+token)
}

// addTestUserAuthHeader adds authentication header for the default test user
func addTestUserAuthHeader(req *http.Request) {
	addAuthHeader(req, "test-user-id", models.UserRoleUser)
}

func populateFileRegistryWithTestData(fileRegistry registry.FileRegistry, commodityRegistry registry.CommodityRegistry) {
	commodities := must.Must(commodityRegistry.List(context.Background()))
	if len(commodities) == 0 {
		return
	}

	now := time.Now()

	// Create file entities for images
	must.Must(fileRegistry.Create(context.Background(), models.FileEntity{
		Title:            "image1",
		Description:      "Test image 1",
		Type:             models.FileTypeImage,
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodities[0].ID,
		LinkedEntityMeta: "images",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "image1",
			OriginalPath: "image1.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	must.Must(fileRegistry.Create(context.Background(), models.FileEntity{
		Title:            "image2",
		Description:      "Test image 2",
		Type:             models.FileTypeImage,
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodities[0].ID,
		LinkedEntityMeta: "images",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "image2",
			OriginalPath: "image2.jpg",
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))

	// Create file entities for invoices
	must.Must(fileRegistry.Create(context.Background(), models.FileEntity{
		Title:            "invoice1",
		Description:      "Test invoice 1",
		Type:             models.FileTypeDocument,
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodities[0].ID,
		LinkedEntityMeta: "invoices",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "invoice1",
			OriginalPath: "invoice1.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	must.Must(fileRegistry.Create(context.Background(), models.FileEntity{
		Title:            "invoice2",
		Description:      "Test invoice 2",
		Type:             models.FileTypeDocument,
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodities[0].ID,
		LinkedEntityMeta: "invoices",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "invoice2",
			OriginalPath: "invoice2.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	// Create file entities for manuals
	must.Must(fileRegistry.Create(context.Background(), models.FileEntity{
		Title:            "manual1",
		Description:      "Test manual 1",
		Type:             models.FileTypeDocument,
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodities[0].ID,
		LinkedEntityMeta: "manuals",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "manual1",
			OriginalPath: "manual1.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	must.Must(fileRegistry.Create(context.Background(), models.FileEntity{
		Title:            "manual2",
		Description:      "Test manual 2",
		Type:             models.FileTypeDocument,
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   commodities[0].ID,
		LinkedEntityMeta: "manuals",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         "manual2",
			OriginalPath: "manual2.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))
}

func newParams() apiserver.Params {
	var params apiserver.Params
	params.RegistrySet = &registry.Set{}
	params.RegistrySet.LocationRegistry = newLocationRegistry()
	params.RegistrySet.AreaRegistry = newAreaRegistry(params.RegistrySet.LocationRegistry)
	params.RegistrySet.SettingsRegistry = newSettingsRegistry()
	params.RegistrySet.UserRegistry = newUserRegistry()

	// Create FileRegistry and populate it with test data first
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	})
	params.RegistrySet.FileRegistry = must.Must(memory.NewFileRegistry().WithCurrentUser(ctx))

	// Create CommodityRegistry
	params.RegistrySet.CommodityRegistry = newCommodityRegistry(params.RegistrySet.AreaRegistry)
	params.RegistrySet.ImageRegistry = newImageRegistry(params.RegistrySet.CommodityRegistry)
	params.RegistrySet.InvoiceRegistry = newInvoiceRegistry(params.RegistrySet.CommodityRegistry)
	params.RegistrySet.ManualRegistry = newManualRegistry(params.RegistrySet.CommodityRegistry)

	params.UploadLocation = uploadLocation
	params.JWTSecret = testJWTSecret

	// Create EntityService
	params.EntityService = services.NewEntityService(params.RegistrySet, params.UploadLocation)

	// Populate FileRegistry with test data using the same instance
	populateFileRegistryWithTestData(params.RegistrySet.FileRegistry, params.RegistrySet.CommodityRegistry)
	return params
}

func newParamsAreaRegistryOnly() apiserver.Params {
	var params apiserver.Params
	params.RegistrySet = &registry.Set{}
	params.RegistrySet.LocationRegistry = newLocationRegistry()
	params.RegistrySet.AreaRegistry = newAreaRegistry(params.RegistrySet.LocationRegistry)
	params.RegistrySet.UserRegistry = newUserRegistry()
	params.UploadLocation = uploadLocation
	params.JWTSecret = testJWTSecret
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
