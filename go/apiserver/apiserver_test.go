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

// createTestUserContext creates a user context for testing with the given user ID
func createTestUserContext(userID string) context.Context {
	return appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: userID},
		},
	})
}

// getRegistrySetFromParams creates a user-aware registry set from params and user ID
func getRegistrySetFromParams(params apiserver.Params, userID string) *registry.Set {
	ctx := createTestUserContext(userID)
	return must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
}

func newUserRegistryWithUser() (registry.UserRegistry, *models.User) {
	var userRegistry = memory.NewUserRegistry()

	// Create a test user for authentication
	testUser := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			// ID will be generated server-side for security
			TenantID: "test-tenant-id",
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}
	must.Assert(testUser.SetPassword("password123"))
	createdUser := must.Must(userRegistry.Create(context.Background(), testUser))

	return userRegistry, createdUser
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

// addTestUserAuthHeader adds authentication header for the given user
func addTestUserAuthHeader(req *http.Request, userID string) {
	addAuthHeader(req, userID, models.UserRoleUser)
}

func populateLocationTestData(locationRegistry registry.LocationRegistry) {
	must.Must(locationRegistry.Create(context.Background(), models.Location{
		Name:    "Location 1",
		Address: "Address 1",
	}))

	must.Must(locationRegistry.Create(context.Background(), models.Location{
		Name:    "Location 2",
		Address: "Address 2",
	}))
}

func populateAreaTestData(areaRegistry registry.AreaRegistry, locationRegistry registry.LocationRegistry) {
	locations := must.Must(locationRegistry.List(context.Background()))

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
}

func populateSettingsTestData(settingsRegistry registry.SettingsRegistry) {
	must.Assert(settingsRegistry.Patch(context.Background(), "system.main_currency", "USD"))
}

func populateCommodityTestData(commodityRegistry registry.CommodityRegistry, areaRegistry registry.AreaRegistry) {
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
}

func populateImageTestData(imageRegistry registry.ImageRegistry, commodityRegistry registry.CommodityRegistry) {
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
}

func populateInvoiceTestData(invoiceRegistry registry.InvoiceRegistry, commodityRegistry registry.CommodityRegistry) {
	commodities := must.Must(commodityRegistry.List(context.Background()))

	b := must.Must(blob.OpenBucket(context.Background(), uploadLocation))
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
}

func populateManualTestData(manualRegistry registry.ManualRegistry, commodityRegistry registry.CommodityRegistry) {
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

func newParams() (apiserver.Params, *models.User) {
	var params apiserver.Params

	// Create user registry first to get the user ID
	userRegistry, testUser := newUserRegistryWithUser()

	// Create factory set
	params.FactorySet = memory.NewFactorySet()
	params.FactorySet.UserRegistry = userRegistry

	// Create user context and get user-aware registry set
	ctx := createTestUserContext(testUser.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))

	// Populate test data
	populateLocationTestData(registrySet.LocationRegistry)
	populateAreaTestData(registrySet.AreaRegistry, registrySet.LocationRegistry)
	populateSettingsTestData(registrySet.SettingsRegistry)
	populateCommodityTestData(registrySet.CommodityRegistry, registrySet.AreaRegistry)
	populateImageTestData(registrySet.ImageRegistry, registrySet.CommodityRegistry)
	populateInvoiceTestData(registrySet.InvoiceRegistry, registrySet.CommodityRegistry)
	populateManualTestData(registrySet.ManualRegistry, registrySet.CommodityRegistry)

	params.UploadLocation = uploadLocation
	params.JWTSecret = testJWTSecret

	// Create EntityService
	params.EntityService = services.NewEntityService(params.FactorySet, params.UploadLocation)

	// Populate FileRegistry with test data using the same instance
	populateFileRegistryWithTestData(registrySet.FileRegistry, registrySet.CommodityRegistry)
	return params, testUser
}

func newParamsAreaRegistryOnly() (apiserver.Params, *models.User) {
	var params apiserver.Params

	// Create user registry first to get the user ID
	userRegistry, testUser := newUserRegistryWithUser()

	// Create factory set
	params.FactorySet = memory.NewFactorySet()
	params.FactorySet.UserRegistry = userRegistry

	// Create user context and get user-aware registry set
	ctx := createTestUserContext(testUser.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))

	// Populate minimal test data
	populateLocationTestData(registrySet.LocationRegistry)
	populateAreaTestData(registrySet.AreaRegistry, registrySet.LocationRegistry)

	params.UploadLocation = uploadLocation
	params.JWTSecret = testJWTSecret
	return params, testUser
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
