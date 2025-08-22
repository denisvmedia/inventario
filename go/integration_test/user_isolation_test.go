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

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	registrySet, err := registrySetFunc(registry.Config(dsn))
	if err != nil {
		t.Fatalf("Failed to create registry set: %v", err)
	}

	return registrySet, func() {
		cleanup()
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
		Name:     "Test User " + email,
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

// withUserContext creates a context with user ID set
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
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Test: User2 cannot see User1's commodity in list
	commodities2, err := registrySet.CommodityRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities2), qt.Equals, 0)

	// Test: User1 can see their own commodity
	commodities1, err := registrySet.CommodityRegistry.ListWithUser(ctx1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities1), qt.Equals, 1)
	c.Assert(commodities1[0].ID, qt.Equals, created1.ID)

	// Test: User2 creates their own commodity
	commodity2 := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "commodity-user2"},
			TenantID: "test-tenant-id",
			UserID:   user2.ID,
		},
		Name:                   "User2 Commodity",
		ShortName:              "UC2",
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

	created2, err := registrySet.CommodityRegistry.CreateWithUser(ctx2, commodity2)
	c.Assert(err, qt.IsNil)
	c.Assert(created2, qt.IsNotNil)
	c.Assert(created2.GetUserID(), qt.Equals, user2.ID)

	// Test: Each user can only see their own commodities
	commodities1Final, err := registrySet.CommodityRegistry.ListWithUser(ctx1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities1Final), qt.Equals, 1)
	c.Assert(commodities1Final[0].ID, qt.Equals, created1.ID)

	commodities2Final, err := registrySet.CommodityRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities2Final), qt.Equals, 1)
	c.Assert(commodities2Final[0].ID, qt.Equals, created2.ID)
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
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

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

// TestUserIsolation_Areas tests that users cannot access each other's areas
func TestUserIsolation_Areas(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Setup: Create two users
	user1 := createTestUser(c, registrySet, "user1@example.com")
	user2 := createTestUser(c, registrySet, "user2@example.com")

	// Setup: Create locations for each user first
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
	createdLocation1, err := registrySet.LocationRegistry.CreateWithUser(ctx1, location1)
	c.Assert(err, qt.IsNil)

	ctx2 := withUserContext(context.Background(), user2.ID)
	location2 := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "location-user2"},
			TenantID: "test-tenant-id",
			UserID:   user2.ID,
		},
		Name:    "User2 Location",
		Address: "456 User2 Street",
	}
	_, err = registrySet.LocationRegistry.CreateWithUser(ctx2, location2)
	c.Assert(err, qt.IsNil)

	// Test: User1 creates an area
	area1 := models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "area-user1"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name:       "User1 Area",
		LocationID: createdLocation1.ID,
	}

	createdArea1, err := registrySet.AreaRegistry.CreateWithUser(ctx1, area1)
	c.Assert(err, qt.IsNil)
	c.Assert(createdArea1, qt.IsNotNil)
	c.Assert(createdArea1.GetUserID(), qt.Equals, user1.ID)

	// Test: User2 cannot access User1's area
	_, err = registrySet.AreaRegistry.GetWithUser(ctx2, createdArea1.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Test: User2 cannot see User1's area in list
	areas2, err := registrySet.AreaRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas2), qt.Equals, 0)

	// Test: User1 can see their own area
	areas1, err := registrySet.AreaRegistry.ListWithUser(ctx1)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas1), qt.Equals, 1)
	c.Assert(areas1[0].ID, qt.Equals, createdArea1.ID)
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
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

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
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

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

// TestDataIsolation_AllEntities tests user isolation across all entity types
func TestDataIsolation_AllEntities(t *testing.T) {
	tests := []struct {
		name     string
		entity   string
		testFunc func(*qt.C, *registry.Set, *models.User, *models.User)
	}{
		{"Commodities", "commodity", testCommodityIsolation},
		{"Locations", "location", testLocationIsolation},
		{"Areas", "area", testAreaIsolation},
		{"Files", "file", testFileIsolation},
		{"Exports", "export", testExportIsolation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			registrySet, cleanup := setupTestDatabase(t)
			defer cleanup()

			// Create two users for each test
			user1 := createTestUser(c, registrySet, "user1@"+tt.entity+".com")
			user2 := createTestUser(c, registrySet, "user2@"+tt.entity+".com")

			// Run the specific entity isolation test
			tt.testFunc(c, registrySet, user1, user2)
		})
	}
}

// Helper functions for entity isolation testing
func testCommodityIsolation(c *qt.C, registrySet *registry.Set, user1, user2 *models.User) {
	ctx1 := withUserContext(context.Background(), user1.ID)
	ctx2 := withUserContext(context.Background(), user2.ID)

	// User1 creates commodity
	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-commodity"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name: "Test Commodity",
	}
	created, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity)
	c.Assert(err, qt.IsNil)

	// User2 cannot access it
	_, err = registrySet.CommodityRegistry.GetWithUser(ctx2, created.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see it in list
	list, err := registrySet.CommodityRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(list), qt.Equals, 0)
}

func testLocationIsolation(c *qt.C, registrySet *registry.Set, user1, user2 *models.User) {
	ctx1 := withUserContext(context.Background(), user1.ID)
	ctx2 := withUserContext(context.Background(), user2.ID)

	// User1 creates location
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-location"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name: "Test Location",
	}
	created, err := registrySet.LocationRegistry.CreateWithUser(ctx1, location)
	c.Assert(err, qt.IsNil)

	// User2 cannot access it
	_, err = registrySet.LocationRegistry.GetWithUser(ctx2, created.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see it in list
	list, err := registrySet.LocationRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(list), qt.Equals, 0)
}

func testAreaIsolation(c *qt.C, registrySet *registry.Set, user1, user2 *models.User) {
	ctx1 := withUserContext(context.Background(), user1.ID)
	ctx2 := withUserContext(context.Background(), user2.ID)

	// First create a location for user1
	location := models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-location-for-area"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name: "Test Location for Area",
	}
	createdLocation, err := registrySet.LocationRegistry.CreateWithUser(ctx1, location)
	c.Assert(err, qt.IsNil)

	// User1 creates area
	area := models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-area"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name:       "Test Area",
		LocationID: createdLocation.ID,
	}
	created, err := registrySet.AreaRegistry.CreateWithUser(ctx1, area)
	c.Assert(err, qt.IsNil)

	// User2 cannot access it
	_, err = registrySet.AreaRegistry.GetWithUser(ctx2, created.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see it in list
	list, err := registrySet.AreaRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(list), qt.Equals, 0)
}

func testFileIsolation(c *qt.C, registrySet *registry.Set, user1, user2 *models.User) {
	ctx1 := withUserContext(context.Background(), user1.ID)
	ctx2 := withUserContext(context.Background(), user2.ID)

	// User1 creates file
	file := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-file"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Title:       "Test File",
		Description: "A test file",
		Type:        models.FileTypeDocument,
		File: &models.File{
			OriginalPath: "/uploads/test-file.txt",
			Path:         "test-file",
			Ext:          ".txt",
			MIMEType:     "text/plain",
		},
	}
	created, err := registrySet.FileRegistry.CreateWithUser(ctx1, file)
	c.Assert(err, qt.IsNil)

	// User2 cannot access it
	_, err = registrySet.FileRegistry.GetWithUser(ctx2, created.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see it in list
	list, err := registrySet.FileRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(list), qt.Equals, 0)
}

func testExportIsolation(c *qt.C, registrySet *registry.Set, user1, user2 *models.User) {
	ctx1 := withUserContext(context.Background(), user1.ID)
	ctx2 := withUserContext(context.Background(), user2.ID)

	// User1 creates export
	export := models.Export{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "test-export"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Type:        models.ExportTypeFullDatabase,
		Description: "Test Export",
		Status:      models.ExportStatusPending,
	}
	created, err := registrySet.ExportRegistry.CreateWithUser(ctx1, export)
	c.Assert(err, qt.IsNil)

	// User2 cannot access it
	_, err = registrySet.ExportRegistry.GetWithUser(ctx2, created.ID)
	c.Assert(err, qt.IsNotNil)

	// User2 cannot see it in list
	list, err := registrySet.ExportRegistry.ListWithUser(ctx2)
	c.Assert(err, qt.IsNil)
	c.Assert(len(list), qt.Equals, 0)
}

// TestSecurityBoundaries tests security edge cases and malicious attempts
func TestSecurityBoundaries(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Test: SQL injection attempts in user ID
	t.Run("SQL injection in user ID", func(t *testing.T) {
		maliciousUserID := "'; DROP TABLE commodities; --"
		ctx := withUserContext(context.Background(), maliciousUserID)

		_, err := registrySet.CommodityRegistry.ListWithUser(ctx)
		c.Assert(err, qt.IsNotNil) // Should fail safely
	})

	// Test: Empty user context
	t.Run("empty user context", func(t *testing.T) {
		emptyCtx := context.Background()
		_, err := registrySet.CommodityRegistry.ListWithUser(emptyCtx)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*user context required.*")
	})

	// Test: Invalid user ID format
	t.Run("invalid user ID format", func(t *testing.T) {
		invalidUserID := "invalid-user-id-format-123456789"
		ctx := withUserContext(context.Background(), invalidUserID)

		// Should not crash, but should return empty results
		commodities, err := registrySet.CommodityRegistry.ListWithUser(ctx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities), qt.Equals, 0)
	})

	// Test: Null user ID
	t.Run("null user ID", func(t *testing.T) {
		ctx := withUserContext(context.Background(), "")
		_, err := registrySet.CommodityRegistry.ListWithUser(ctx)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*user context required.*")
	})
}

// TestUserIsolation_UpdateOperations tests that users cannot update each other's data
func TestUserIsolation_UpdateOperations(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Setup: Create two users
	user1 := createTestUser(c, registrySet, "user1@example.com")
	user2 := createTestUser(c, registrySet, "user2@example.com")

	// User1 creates a commodity
	ctx1 := withUserContext(context.Background(), user1.ID)
	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "commodity-update-test"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name:                   "Original Commodity",
		ShortName:              "OC",
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

	created, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity)
	c.Assert(err, qt.IsNil)

	// User2 tries to update User1's commodity
	ctx2 := withUserContext(context.Background(), user2.ID)
	created.Name = "Modified by User2"
	created.ShortName = "MU2"

	_, err = registrySet.CommodityRegistry.UpdateWithUser(ctx2, *created)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify the commodity was not modified
	original, err := registrySet.CommodityRegistry.GetWithUser(ctx1, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(original.Name, qt.Equals, "Original Commodity")
	c.Assert(original.ShortName, qt.Equals, "OC")
}

// TestUserIsolation_DeleteOperations tests that users cannot delete each other's data
func TestUserIsolation_DeleteOperations(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Setup: Create two users
	user1 := createTestUser(c, registrySet, "user1@example.com")
	user2 := createTestUser(c, registrySet, "user2@example.com")

	// User1 creates a commodity
	ctx1 := withUserContext(context.Background(), user1.ID)
	commodity := models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "commodity-delete-test"},
			TenantID: "test-tenant-id",
			UserID:   user1.ID,
		},
		Name: "Commodity to Delete",
	}

	created, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity)
	c.Assert(err, qt.IsNil)

	// User2 tries to delete User1's commodity
	ctx2 := withUserContext(context.Background(), user2.ID)
	err = registrySet.CommodityRegistry.DeleteWithUser(ctx2, created.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorMatches, ".*not found.*")

	// Verify the commodity still exists for User1
	_, err = registrySet.CommodityRegistry.GetWithUser(ctx1, created.ID)
	c.Assert(err, qt.IsNil)
}
