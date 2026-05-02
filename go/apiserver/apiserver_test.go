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

// createTestUserContext creates a user context for testing with the given user ID and tenant ID.
func createTestUserContext(userID, tenantID string) context.Context {
	return appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			EntityID: models.EntityID{ID: userID},
		},
	})
}

// createTestUserContextWithGroup creates a context with both user and group set.
// The synthetic group is stamped with MainCurrency=USD so tests that rely on
// commodity validation (which pulls the main currency off the group in context)
// don't trip "main currency not set".
func createTestUserContextWithGroup(userID, tenantID, groupID string) context.Context {
	ctx := createTestUserContext(userID, tenantID)
	return appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: groupID}, TenantID: tenantID},
		MainCurrency:        models.Currency("USD"),
	})
}

// createTestGroupForUser creates a default group and membership for a test
// user. The group defaults to USD so commodity-validation code paths don't
// trip "main currency not set"; tests that want a different currency should
// call LocationGroupRegistry.Update themselves.
func createTestGroupForUser(fs *registry.FactorySet, tenantID, userID string) *models.LocationGroup {
	slug := must.Must(models.GenerateGroupSlug())
	group := must.Must(fs.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Name:                "Test Group",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           userID,
		MainCurrency:        models.Currency("USD"),
	}))
	must.Must(fs.GroupMembershipRegistry.Create(context.Background(), models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		GroupID:             group.ID,
		MemberUserID:        userID,
		Role:                models.GroupRoleAdmin,
	}))
	return group
}

// getRegistrySetFromParams creates a user+group-aware registry set from params using the supplied user.
// The group is resolved via the user's memberships (GroupMembershipRegistry.ListByUser) rather
// than ListByTenant()[0], so tests that happen to run against a tenant with multiple groups
// still see a group the user actually belongs to — matching the invariant enforced by the
// GroupSlugResolverMiddleware in production.
func getRegistrySetFromParams(params apiserver.Params, user *models.User) *registry.Set {
	ctx := createTestUserContext(user.ID, user.TenantID)
	memberships, err := params.FactorySet.GroupMembershipRegistry.ListByUser(context.Background(), user.TenantID, user.ID)
	if err == nil && len(memberships) > 0 {
		if group, gerr := params.FactorySet.LocationGroupRegistry.Get(context.Background(), memberships[0].GroupID); gerr == nil {
			ctx = appctx.WithGroup(ctx, group)
		}
	}
	return must.Must(params.FactorySet.CreateUserRegistrySet(ctx))
}

// createTestJWTToken creates a JWT token for testing
func createTestJWTToken(userID string) string {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(testJWTSecret)
	if err != nil {
		panic(fmt.Sprintf("Failed to create test JWT token: %v", err))
	}

	return tokenString
}

// addAuthHeader adds JWT authentication header to a request
func addAuthHeader(req *http.Request, userID string) {
	token := createTestJWTToken(userID)
	req.Header.Set("Authorization", "Bearer "+token)
}

// addTestUserAuthHeader adds authentication header for the given user
func addTestUserAuthHeader(req *http.Request, userID string) {
	addAuthHeader(req, userID)
}

func populateLocationTestData(ctx context.Context, locationRegistry registry.LocationRegistry) {
	must.Must(locationRegistry.Create(ctx, models.Location{
		Name:    "Location 1",
		Address: "Address 1",
	}))

	must.Must(locationRegistry.Create(ctx, models.Location{
		Name:    "Location 2",
		Address: "Address 2",
	}))
}

func populateAreaTestData(ctx context.Context, areaRegistry registry.AreaRegistry, locationRegistry registry.LocationRegistry) {
	locations := must.Must(locationRegistry.List(ctx))

	must.Must(areaRegistry.Create(ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{EntityID: models.EntityID{ID: "1"}, TenantID: "default-tenant"},
		Name:                     "Area 1",
		LocationID:               locations[0].ID,
	}))

	must.Must(areaRegistry.Create(ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{EntityID: models.EntityID{ID: "2"}, TenantID: "default-tenant"},
		Name:                     "Area 2",
		LocationID:               locations[0].ID,
	}))
}

// populateSettingsTestData used to seed system.main_currency into user
// settings. Main currency now lives on the location group (stamped USD by
// createTestGroupForUser), so this is a no-op retained purely to keep the
// call-site story legible for readers of the test setup.
func populateSettingsTestData(_ context.Context, _ registry.SettingsRegistry) {}

func populateCommodityTestData(ctx context.Context, commodityRegistry registry.CommodityRegistry, areaRegistry registry.AreaRegistry) {
	areas := must.Must(areaRegistry.List(ctx))

	must.Must(commodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Commodity 1",
		ShortName:             "C1",
		AreaID:                areas[0].ID,
		Type:                  models.CommodityTypeFurniture,
		Status:                models.CommodityStatusInUse,
		Count:                 10,
		OriginalPrice:         must.Must(decimal.NewFromString("2000.00")),
		OriginalPriceCurrency: models.Currency("USD"),
	}))

	must.Must(commodityRegistry.Create(ctx, models.Commodity{
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

func populateImageTestData(ctx context.Context, imageRegistry registry.ImageRegistry, commodityRegistry registry.CommodityRegistry) {
	commodities := must.Must(commodityRegistry.List(ctx))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "image1.jpg", []byte("image1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(imageRegistry.Create(ctx, models.Image{
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

	must.Must(imageRegistry.Create(ctx, models.Image{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "image2",     // Without extension
			OriginalPath: "image2.jpg", // This is the actual file name in storage
			Ext:          ".jpg",
			MIMEType:     "image/jpeg",
		},
	}))
}

func populateInvoiceTestData(ctx context.Context, invoiceRegistry registry.InvoiceRegistry, commodityRegistry registry.CommodityRegistry) {
	commodities := must.Must(commodityRegistry.List(ctx))

	b := must.Must(blob.OpenBucket(context.Background(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "invoice1.pdf", []byte("invoice1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(invoiceRegistry.Create(ctx, models.Invoice{
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

	must.Must(invoiceRegistry.Create(ctx, models.Invoice{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "invoice2",     // Without extension
			OriginalPath: "invoice2.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))
}

func populateManualTestData(ctx context.Context, manualRegistry registry.ManualRegistry, commodityRegistry registry.CommodityRegistry) {
	commodities := must.Must(commodityRegistry.List(ctx))

	b := must.Must(blob.OpenBucket(context.TODO(), uploadLocation))
	defer b.Close()
	err := b.WriteAll(context.TODO(), "manual1.pdf", []byte("manual1"), nil)
	if err != nil {
		panic(err)
	}

	must.Must(manualRegistry.Create(ctx, models.Manual{
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

	must.Must(manualRegistry.Create(ctx, models.Manual{
		CommodityID: commodities[0].ID,
		File: &models.File{
			Path:         "manual2",     // Without extension
			OriginalPath: "manual2.pdf", // This is the actual file name in storage
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))
}

func populateFileRegistryWithTestData(ctx context.Context, fileRegistry registry.FileRegistry, commodityRegistry registry.CommodityRegistry) {
	commodities := must.Must(commodityRegistry.List(ctx))
	if len(commodities) == 0 {
		return
	}

	now := time.Now()

	// Create file entities for images
	must.Must(fileRegistry.Create(ctx, models.FileEntity{
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

	must.Must(fileRegistry.Create(ctx, models.FileEntity{
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
	must.Must(fileRegistry.Create(ctx, models.FileEntity{
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

	must.Must(fileRegistry.Create(ctx, models.FileEntity{
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
	must.Must(fileRegistry.Create(ctx, models.FileEntity{
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

	must.Must(fileRegistry.Create(ctx, models.FileEntity{
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

func newParams() (apiserver.Params, *models.User, *models.LocationGroup) {
	var params apiserver.Params
	params.FactorySet = memory.NewFactorySet()

	// Create default tenant first so we have the server-generated ID.
	createdTenant := must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:      "Test Organization",
		Slug:      "test-org",
		Status:    models.TenantStatusActive,
		IsDefault: true,
	}))

	// Create test user scoped to the generated tenant ID.
	testUserTemplate := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: createdTenant.ID,
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	must.Assert(testUserTemplate.SetPassword("password123"))
	testUser := must.Must(params.FactorySet.UserRegistry.Create(context.Background(), testUserTemplate))

	// Create a default group for the test user
	testGroup := createTestGroupForUser(params.FactorySet, testUser.TenantID, testUser.ID)

	// Create user + group context and get user-aware registry set
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))

	// Populate test data
	populateLocationTestData(ctx, registrySet.LocationRegistry)
	populateAreaTestData(ctx, registrySet.AreaRegistry, registrySet.LocationRegistry)
	populateSettingsTestData(ctx, registrySet.SettingsRegistry)
	populateCommodityTestData(ctx, registrySet.CommodityRegistry, registrySet.AreaRegistry)
	populateImageTestData(ctx, registrySet.ImageRegistry, registrySet.CommodityRegistry)
	populateInvoiceTestData(ctx, registrySet.InvoiceRegistry, registrySet.CommodityRegistry)
	populateManualTestData(ctx, registrySet.ManualRegistry, registrySet.CommodityRegistry)

	params.UploadLocation = uploadLocation
	params.JWTSecret = testJWTSecret

	// Create EntityService
	params.EntityService = services.NewEntityService(params.FactorySet, params.UploadLocation)

	// Populate FileRegistry with test data using the same instance
	populateFileRegistryWithTestData(ctx, registrySet.FileRegistry, registrySet.CommodityRegistry)
	return params, testUser, testGroup
}

func newParamsAreaRegistryOnly() (apiserver.Params, *models.User, *models.LocationGroup) {
	var params apiserver.Params
	params.FactorySet = memory.NewFactorySet()

	// Create default tenant first so we have the server-generated ID.
	createdTenant := must.Must(params.FactorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:      "Test Organization",
		Slug:      "test-org",
		Status:    models.TenantStatusActive,
		IsDefault: true,
	}))

	// Create test user scoped to the generated tenant ID.
	testUserTemplate := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: createdTenant.ID,
		},
		Email:    "test@example.com",
		Name:     "Test User",
		IsActive: true,
	}
	must.Assert(testUserTemplate.SetPassword("password123"))
	testUser := must.Must(params.FactorySet.UserRegistry.Create(context.Background(), testUserTemplate))

	// Create a default group for the test user
	testGroup := createTestGroupForUser(params.FactorySet, testUser.TenantID, testUser.ID)

	// Create user + group context and get user-aware registry set
	ctx := createTestUserContextWithGroup(testUser.ID, testUser.TenantID, testGroup.ID)
	registrySet := must.Must(params.FactorySet.CreateUserRegistrySet(ctx))

	// Populate minimal test data
	populateLocationTestData(ctx, registrySet.LocationRegistry)
	populateAreaTestData(ctx, registrySet.AreaRegistry, registrySet.LocationRegistry)

	params.UploadLocation = uploadLocation
	params.JWTSecret = testJWTSecret
	return params, testUser, testGroup
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

// populateLocationFileTestData was the seed helper for the legacy
// `/locations/{id}/{images,files}*` route tests; both routes and tests
// were removed under #1421 (this commit). The unified `/files` surface
// covers the same reads via `?linked_entity_type=location&linked_entity_id=…`.

func sliceToSliceOfAny[T any](v []T) (result []any) {
	for _, item := range v {
		result = append(result, item)
	}
	return result
}
