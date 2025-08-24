package integration_test

import (
	"context"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TestUserIsolation_ComprehensiveScenarios tests complex real-world scenarios
func TestUserIsolation_ComprehensiveScenarios(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	// Create multiple users for complex scenarios
	user1 := createTestUser(c, registrySet, "user1@comprehensive.com")
	user2 := createTestUser(c, registrySet, "user2@comprehensive.com")
	user3 := createTestUser(c, registrySet, "user3@comprehensive.com")

	ctx1 := registry.WithUserContext(context.Background(), user1.ID)
	ctx2 := registry.WithUserContext(context.Background(), user2.ID)
	ctx3 := registry.WithUserContext(context.Background(), user3.ID)

	t.Run("Complex Entity Relationships", func(t *testing.T) {
		c := qt.New(t)

		// User1 creates location -> area -> commodity chain
		location1 := models.Location{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-location-1"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name:    "User1 Warehouse",
			Address: "123 User1 Street",
		}
		userAwareLocationRegistry1, err := registrySet.LocationRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		createdLocation1, err := userAwareLocationRegistry1.Create(ctx1, location1)
		c.Assert(err, qt.IsNil)

		area1 := models.Area{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-area-1"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name:       "User1 Storage Area",
			LocationID: createdLocation1.ID,
		}
		userAwareAreaRegistry1, err := registrySet.AreaRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		createdArea1, err := userAwareAreaRegistry1.Create(ctx1, area1)
		if err != nil {
			t.Fatalf("Failed to create area: %v", err)
		}

		commodity1 := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-commodity-1"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name:                   "User1 Product",
			ShortName:              "UP1",
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
			AreaID:                 createdArea1.ID,
		}
		userAwareCommodityRegistry1, err := registrySet.CommodityRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		_, err = userAwareCommodityRegistry1.Create(ctx1, commodity1)
		if err != nil {
			t.Fatalf("Failed to create commodity: %v", err)
		}

		// User2 tries to access User1's entities through relationships
		// Should not be able to access location
		userAwareLocationRegistry2, err := registrySet.LocationRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		_, err = userAwareLocationRegistry2.Get(ctx2, createdLocation1.ID)
		if err == nil {
			t.Error("Expected error when user2 tries to access user1's location")
		}

		// Should not be able to access area
		userAwareAreaRegistry2, err := registrySet.AreaRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		_, err = userAwareAreaRegistry2.Get(ctx2, createdArea1.ID)
		if err == nil {
			t.Error("Expected error when user2 tries to access user1's area")
		}

		// User2 creates their own entities with similar names
		location2 := models.Location{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-location-2"},
				TenantID: "test-tenant-id",
				UserID:   user2.ID,
			},
			Name:    "User1 Warehouse", // Same name as User1's location
			Address: "456 User2 Street",
		}
		_, err = userAwareLocationRegistry2.Create(ctx2, location2)
		if err != nil {
			t.Fatalf("Failed to create location for user2: %v", err)
		}

		// Verify users can only see their own entities despite same names
		locations1, err := userAwareLocationRegistry1.List(ctx1)
		if err != nil {
			t.Fatalf("Failed to list locations for user1: %v", err)
		}
		if len(locations1) != 1 {
			t.Errorf("Expected 1 location for user1, got %d", len(locations1))
		}
		if locations1[0].ID != createdLocation1.ID {
			t.Errorf("Expected location ID %s, got %s", createdLocation1.ID, locations1[0].ID)
		}

		locations2, err := userAwareLocationRegistry2.List(ctx2)
		if err != nil {
			t.Fatalf("Failed to list locations for user2: %v", err)
		}
		if len(locations2) != 1 {
			t.Errorf("Expected 1 location for user2, got %d", len(locations2))
		}
	})

	t.Run("Cross-User Update Attempts", func(t *testing.T) {
		// User1 creates entities
		commodity1 := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "update-test-commodity"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name:                   "Original Name",
			ShortName:              "ON",
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
		userAwareCommodityRegistry1, err := registrySet.CommodityRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		created1, err := userAwareCommodityRegistry1.Create(ctx1, commodity1)
		if err != nil {
			t.Fatalf("Failed to create commodity: %v", err)
		}

		// User2 tries to update User1's commodity
		created1.Name = "Hacked Name"
		created1.ShortName = "HN"
		userAwareCommodityRegistry2, err := registrySet.CommodityRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		_, err = userAwareCommodityRegistry2.Update(ctx2, *created1)
		if err == nil {
			t.Error("Expected error when user2 tries to update user1's commodity")
		}

		// User3 also tries to update User1's commodity
		created1.Name = "Another Hack"
		userAwareCommodityRegistry3, err := registrySet.CommodityRegistry.WithCurrentUser(ctx3)
		c.Assert(err, qt.IsNil)
		_, err = userAwareCommodityRegistry3.Update(ctx3, *created1)
		if err == nil {
			t.Error("Expected error when user3 tries to update user1's commodity")
		}

		// Verify the commodity remains unchanged
		retrieved, err := userAwareCommodityRegistry1.Get(ctx1, created1.ID)
		if err != nil {
			t.Fatalf("Failed to retrieve commodity: %v", err)
		}
		if retrieved.Name != "Original Name" {
			t.Errorf("Expected name 'Original Name', got %s", retrieved.Name)
		}
		if retrieved.ShortName != "ON" {
			t.Errorf("Expected short name 'ON', got %s", retrieved.ShortName)
		}
	})

	t.Run("Bulk Operations Isolation", func(t *testing.T) {
		// Each user creates multiple entities
		for userIndex, ctx := range []context.Context{ctx1, ctx2, ctx3} {
			for i := 0; i < 10; i++ {
				commodity := models.Commodity{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: fmt.Sprintf("bulk-commodity-user%d-%d", userIndex+1, i)},
						TenantID: "test-tenant-id",
						UserID:   []*models.User{user1, user2, user3}[userIndex].ID,
					},
					Name:                   fmt.Sprintf("User%d Commodity %d", userIndex+1, i),
					ShortName:              fmt.Sprintf("U%dC%d", userIndex+1, i),
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
				userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(ctx)
				c.Assert(err, qt.IsNil)
				_, err = userAwareCommodityRegistry.Create(ctx, commodity)
				if err != nil {
					t.Fatalf("Failed to create commodity for user %d: %v", userIndex+1, err)
				}
			}
		}

		// Verify each user can only see their own entities
		userAwareCommodityRegistry1, err := registrySet.CommodityRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		commodities1, err := userAwareCommodityRegistry1.List(ctx1)
		if err != nil {
			t.Fatalf("Failed to list commodities for user1: %v", err)
		}
		if len(commodities1) != 10 {
			t.Errorf("Expected 10 commodities for user1, got %d", len(commodities1))
		}
		for _, commodity := range commodities1 {
			if commodity.GetUserID() != user1.ID {
				t.Errorf("Expected user ID %s, got %s", user1.ID, commodity.GetUserID())
			}
		}

		userAwareCommodityRegistry2, err := registrySet.CommodityRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		commodities2, err := userAwareCommodityRegistry2.List(ctx2)
		if err != nil {
			t.Fatalf("Failed to list commodities for user2: %v", err)
		}
		if len(commodities2) != 10 {
			t.Errorf("Expected 10 commodities for user2, got %d", len(commodities2))
		}
		for _, commodity := range commodities2 {
			if commodity.GetUserID() != user2.ID {
				t.Errorf("Expected user ID %s, got %s", user2.ID, commodity.GetUserID())
			}
		}

		userAwareCommodityRegistry3, err := registrySet.CommodityRegistry.WithCurrentUser(ctx3)
		c.Assert(err, qt.IsNil)
		commodities3, err := userAwareCommodityRegistry3.List(ctx3)
		if err != nil {
			t.Fatalf("Failed to list commodities for user3: %v", err)
		}
		if len(commodities3) != 10 {
			t.Errorf("Expected 10 commodities for user3, got %d", len(commodities3))
		}
		for _, commodity := range commodities3 {
			if commodity.GetUserID() != user3.ID {
				t.Errorf("Expected user ID %s, got %s", user3.ID, commodity.GetUserID())
			}
		}
	})
}

// TestUserIsolation_EdgeCases tests edge cases and boundary conditions
func TestUserIsolation_EdgeCases(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	user := createTestUser(c, registrySet, "edge@example.com")
	_ = registry.WithUserContext(context.Background(), user.ID)

	t.Run("Empty User Context", func(t *testing.T) {
		emptyCtx := context.Background()

		userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(emptyCtx)
		if err == nil {
			t.Error("Expected error when creating user-aware registry with empty context")
			return
		}
		// Since WithCurrentUser failed, we can't proceed with List
		_ = userAwareCommodityRegistry
	})

	t.Run("Non-existent User ID", func(t *testing.T) {
		nonExistentCtx := registry.WithUserContext(context.Background(), "non-existent-user-id")

		// Should not crash, but should return empty results
		userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(nonExistentCtx)
		c.Assert(err, qt.IsNil)
		commodities, err := userAwareCommodityRegistry.List(nonExistentCtx)
		if err != nil {
			t.Fatalf("Unexpected error for non-existent user: %v", err)
		}
		if len(commodities) != 0 {
			t.Errorf("Expected 0 commodities for non-existent user, got %d", len(commodities))
		}
	})

	t.Run("Very Long User ID", func(t *testing.T) {
		longUserID := string(make([]byte, 10000))
		longCtx := registry.WithUserContext(context.Background(), longUserID)

		// Should handle gracefully
		userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(longCtx)
		c.Assert(err, qt.IsNil)
		commodities, err := userAwareCommodityRegistry.List(longCtx)
		if err != nil {
			t.Fatalf("Unexpected error for long user ID: %v", err)
		}
		if len(commodities) != 0 {
			t.Errorf("Expected 0 commodities for long user ID, got %d", len(commodities))
		}
	})
}
