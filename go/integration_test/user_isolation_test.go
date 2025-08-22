package integration_test

import (
	"context"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// setupTestDatabase creates a test database connection and returns cleanup function
func setupTestDatabase(t *testing.T) (*registry.Set, func()) {
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("POSTGRES_TEST_DSN environment variable not set")
		return nil, nil
	}

	registrySetFunc, cleanupFunc := postgres.NewPostgresRegistrySet()
	registrySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		t.Fatalf("Failed to create registry set: %v", err)
	}

	return registrySet, func() {
		cleanupFunc()
	}
}

// createTestUser creates a test user with the given email and returns the created user
func createTestUser(c *qt.C, registrySet *registry.Set, email string) *models.User {
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-" + email},
			TenantID: "test-tenant-id",
		},
		Email:    email,
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}

	err := user.SetPassword("testpassword123")
	c.Assert(err, qt.IsNil)

	created, err := registrySet.UserRegistry.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)

	return created
}

// withUserContext creates a context with the given user ID
func withUserContext(ctx context.Context, userID string) context.Context {
	return registry.WithUserContext(ctx, userID)
}

// TestUserIsolation_Commodities tests that users cannot access each other's commodities
func TestUserIsolation_Commodities(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Setup: Create two users
	user1 := createTestUser(c, registrySet, "user1@example.com")
	user2 := createTestUser(c, registrySet, "user2@example.com")

	// Test: User1 creates a commodity
	ctx1 := withUserContext(context.Background(), user1.ID)
	commodity1 := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "commodity-user1"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name:                   "User1 Commodity",
		ShortName:              "UC1",
		Type:                   models.CommodityTypeElectronics,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		CurrentPrice:           decimal.NewFromFloat(90.00),
		Status:                 models.CommodityStatusInUse,
		PurchaseDate:           models.ToPDate("2023-01-01"),
		RegisteredDate:         models.ToPDate("2023-01-02"),
		LastModifiedDate:       models.ToPDate("2023-01-03"),
		Draft:                  false,
	}

	created1, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity1)
	c.Assert(err, qt.IsNil)
	c.Assert(created1, qt.IsNotNil)
	c.Assert(created1.GetUserID(), qt.Equals, user1.ID)

	// Test: User2 cannot access User1's commodity
	ctx2 := withUserContext(context.Background(), user2.ID)
	_, err = registrySet.CommodityRegistry.GetWithUser(ctx2, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// Test: User2 cannot see User1's commodity in list
	commodities2, err := registrySet.CommodityRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities2), qt.Equals, 0)

	// Test: User1 can see their own commodity
	commodities1, err := registrySet.CommodityRegistry.ListWithUser(ctx1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities1), qt.Equals, 1)
	c.Assert(commodities1[0].ID, qt.Equals, created1.ID)
}

// TestUserIsolation_Locations tests that users cannot access each other's locations
func TestUserIsolation_Locations(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Setup: Create two users
	user1 := createTestUser(c, registrySet, "user1@example.com")
	user2 := createTestUser(c, registrySet, "user2@example.com")

	// Test: User1 creates a location
	ctx1 := withUserContext(context.Background(), user1.ID)
	location1 := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "location-user1"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name:    "User1 Location",
		Address: "123 User1 Street",
	}

	created1, err := registrySet.LocationRegistry.CreateWithUser(ctx1, location1)
	c.Assert(err, qt.IsNil)
	c.Assert(created1, qt.IsNotNil)
	c.Assert(created1.GetUserID(), qt.Equals, user1.ID)

	// Test: User2 cannot access User1's location
	ctx2 := withUserContext(context.Background(), user2.ID)
	_, err = registrySet.LocationRegistry.GetWithUser(ctx2, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// Test: User2 cannot see User1's location in list
	locations2, err := registrySet.LocationRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations2), qt.Equals, 0)

	// Test: User1 can see their own location
	locations1, err := registrySet.LocationRegistry.ListWithUser(ctx1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(locations1), qt.Equals, 1)
	c.Assert(locations1[0].ID, qt.Equals, created1.ID)
}

// TestUserIsolation_Files tests that users cannot access each other's files
func TestUserIsolation_Files(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Setup: Create two users
	user1 := createTestUser(c, registrySet, "user1@example.com")
	user2 := createTestUser(c, registrySet, "user2@example.com")

	// Test: User1 creates a file
	ctx1 := withUserContext(context.Background(), user1.ID)
	file1 := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "file-user1"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Title:       "User1 File",
		Description: "A file created by user1",
		Type:        models.FileTypeDocument,
		File: &models.File{
			OriginalPath: "/uploads/user1-file.txt",
			Path:         "user1-file",
			Ext:          ".txt",
			MIMEType:     "text/plain",
		},
	}

	created1, err := registrySet.FileRegistry.CreateWithUser(ctx1, file1)
	c.Assert(err, qt.IsNil)
	c.Assert(created1, qt.IsNotNil)
	c.Assert(created1.GetUserID(), qt.Equals, user1.ID)

	// Test: User2 cannot access User1's file
	ctx2 := withUserContext(context.Background(), user2.ID)
	_, err = registrySet.FileRegistry.GetWithUser(ctx2, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// Test: User2 cannot see User1's file in list
	files2, err := registrySet.FileRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(files2), qt.Equals, 0)

	// Test: User1 can see their own file
	files1, err := registrySet.FileRegistry.ListWithUser(ctx1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(files1), qt.Equals, 1)
	c.Assert(files1[0].ID, qt.Equals, created1.ID)
}

// TestUserIsolation_Exports tests that users cannot access each other's exports
func TestUserIsolation_Exports(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Setup: Create two users
	user1 := createTestUser(c, registrySet, "user1@example.com")
	user2 := createTestUser(c, registrySet, "user2@example.com")

	// Test: User1 creates an export
	ctx1 := withUserContext(context.Background(), user1.ID)
	export1 := models.Export{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "export-user1"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Type:        models.ExportTypeFullDatabase,
		Description: "An export created by user1",
		Status:      models.ExportStatusPending,
	}

	created1, err := registrySet.ExportRegistry.CreateWithUser(ctx1, export1)
	c.Assert(err, qt.IsNil)
	c.Assert(created1, qt.IsNotNil)
	c.Assert(created1.GetUserID(), qt.Equals, user1.ID)

	// Test: User2 cannot access User1's export
	ctx2 := withUserContext(context.Background(), user2.ID)
	_, err = registrySet.ExportRegistry.GetWithUser(ctx2, created1.ID)
	c.Assert(err, qt.IsNotNil)

	// Test: User2 cannot see User1's export in list
	exports2, err := registrySet.ExportRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports2), qt.Equals, 0)

	// Test: User1 can see their own export
	exports1, err := registrySet.ExportRegistry.ListWithUser(ctx1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports1), qt.Equals, 1)
	c.Assert(exports1[0].ID, qt.Equals, created1.ID)
}
