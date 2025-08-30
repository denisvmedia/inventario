package integration_test

import (
	"context"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

// TestUserIsolation_ComprehensiveScenarios tests complex real-world scenarios
func TestUserIsolation_ComprehensiveScenarios(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	c.Run("Complex Entity Relationships", func(c *qt.C) {
		// Create multiple users for complex scenarios
		user1 := createTestUser(c, registrySet, "user1t1@comprehensive.com")
		user2 := createTestUser(c, registrySet, "user2t1@comprehensive.com")

		ctx1 := appctx.WithUser(context.Background(), user1)
		ctx2 := appctx.WithUser(context.Background(), user2)

		// Create shared location and area for user1 that can be used across tests
		location1 := models.Location{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-location-1t1"},
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
		c.Assert(err, qt.IsNil)

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
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create commodity: %v", err))

		// User2 tries to access User1's entities through relationships
		// Should not be able to access location
		userAwareLocationRegistry2, err := registrySet.LocationRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		_, err = userAwareLocationRegistry2.Get(ctx2, createdLocation1.ID)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user2 tries to access user1's location"))

		// Should not be able to access area
		userAwareAreaRegistry2, err := registrySet.AreaRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		_, err = userAwareAreaRegistry2.Get(ctx2, createdArea1.ID)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user2 tries to access user1's area"))

		// User2 creates their own entities with similar names
		location2 := models.Location{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-location-2t1"},
				TenantID: "test-tenant-id",
				UserID:   user2.ID,
			},
			Name:    "User1 Warehouse", // Same name as User1's location
			Address: "456 User2 Street",
		}
		_, err = userAwareLocationRegistry2.Create(ctx2, location2)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create location for user2: %v", err))

		// Verify users can only see their own entities despite same names
		locations1, err := userAwareLocationRegistry1.List(ctx1)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to list locations for user1: %v", err))
		c.Assert(locations1, qt.HasLen, 1, qt.Commentf("Expected 1 location for user1, got %d", len(locations1)))
		c.Assert(locations1[0].ID, qt.Equals, createdLocation1.ID, qt.Commentf("Expected location ID %s, got %s", createdLocation1.ID, locations1[0].ID))

		locations2, err := userAwareLocationRegistry2.List(ctx2)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to list locations for user2: %v", err))
		c.Assert(locations2, qt.HasLen, 1, qt.Commentf("Expected 1 location for user2, got %d", len(locations2)))
	})

	c.Run("Cross-User Update Attempts", func(c *qt.C) {
		// Create multiple users for complex scenarios
		user1 := createTestUser(c, registrySet, "user1t2@comprehensive.com")
		user2 := createTestUser(c, registrySet, "user2t2@comprehensive.com")
		user3 := createTestUser(c, registrySet, "user3t2@comprehensive.com")

		ctx1 := appctx.WithUser(context.Background(), user1)
		ctx2 := appctx.WithUser(context.Background(), user2)
		ctx3 := appctx.WithUser(context.Background(), user3)

		// Create shared location and area for user1 that can be used across tests
		location1 := models.Location{
			Name:    "User1 Warehouse",
			Address: "123 User1 Street",
			// Note: ID will be generated server-side for security
		}
		userAwareLocationRegistry1, err := registrySet.LocationRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		createdLocation1, err := userAwareLocationRegistry1.Create(ctx1, location1)
		c.Assert(err, qt.IsNil)

		area1 := models.Area{
			Name:       "User1 Storage Area",
			LocationID: createdLocation1.ID,
			// Note: ID will be generated server-side for security
		}
		userAwareAreaRegistry1, err := registrySet.AreaRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		createdArea1, err := userAwareAreaRegistry1.Create(ctx1, area1)
		c.Assert(err, qt.IsNil)

		// User1 creates entities
		commodity1 := models.Commodity{
			// Note: ID will be generated server-side for security
			Name:                   "Original Name",
			ShortName:              "ON",
			AreaID:                 createdArea1.ID,
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
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to create commodity: %v", err))

		// User2 tries to update User1's commodity
		created1.Name = "Hacked Name"
		created1.ShortName = "HN"
		userAwareCommodityRegistry2, err := registrySet.CommodityRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		_, err = userAwareCommodityRegistry2.Update(ctx2, *created1)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user2 tries to update user1's commodity"))

		// User3 also tries to update User1's commodity
		created1.Name = "Another Hack"
		userAwareCommodityRegistry3, err := registrySet.CommodityRegistry.WithCurrentUser(ctx3)
		c.Assert(err, qt.IsNil)
		_, err = userAwareCommodityRegistry3.Update(ctx3, *created1)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user3 tries to update user1's commodity"))

		// Verify the commodity remains unchanged
		retrieved, err := userAwareCommodityRegistry1.Get(ctx1, created1.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve commodity: %v", err))
		c.Assert(retrieved, qt.IsNotNil, qt.Commentf("Expected commodity to be retrieved"))
		c.Assert(retrieved.Name, qt.Equals, "Original Name", qt.Commentf("Expected name to remain unchanged"))
		c.Assert(retrieved.ShortName, qt.Equals, "ON", qt.Commentf("Expected short name to remain unchanged"))
	})

	c.Run("Bulk Operations Isolation", func(c *qt.C) {
		// Create multiple users for complex scenarios
		user1 := createTestUser(c, registrySet, "user1t3@comprehensive.com")
		user2 := createTestUser(c, registrySet, "user2t3@comprehensive.com")
		user3 := createTestUser(c, registrySet, "user3t3@comprehensive.com")

		ctx1 := appctx.WithUser(context.Background(), user1)
		ctx2 := appctx.WithUser(context.Background(), user2)
		ctx3 := appctx.WithUser(context.Background(), user3)

		// Create shared location and area for user1 that can be used across tests
		location1 := models.Location{
			Name:    "User1 Warehouse",
			Address: "123 User1 Street",
			// Note: ID will be generated server-side for security
		}
		userAwareLocationRegistry1, err := registrySet.LocationRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		createdLocation1, err := userAwareLocationRegistry1.Create(ctx1, location1)
		c.Assert(err, qt.IsNil)

		area1 := models.Area{
			Name:       "User1 Storage Area",
			LocationID: createdLocation1.ID,
			// Note: ID will be generated server-side for security
		}
		userAwareAreaRegistry1, err := registrySet.AreaRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		createdArea1, err := userAwareAreaRegistry1.Create(ctx1, area1)
		c.Assert(err, qt.IsNil)

		// Create areas for each user first (reuse existing area for user1)
		areas := []*models.Area{createdArea1} // User1 already has an area

		// Create areas for user2 and user3
		for userIndex := 1; userIndex < 3; userIndex++ {
			user := []*models.User{user1, user2, user3}[userIndex]
			ctx := []context.Context{ctx1, ctx2, ctx3}[userIndex]

			// Create location first
			location := models.Location{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: fmt.Sprintf("bulk-location-user%d", userIndex+1)},
					TenantID: "test-tenant-id",
					UserID:   user.ID,
				},
				Name:    fmt.Sprintf("User%d Bulk Location", userIndex+1),
				Address: fmt.Sprintf("123 User%d Bulk Street", userIndex+1),
			}
			userAwareLocationRegistry, err := registrySet.LocationRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)
			createdLocation, err := userAwareLocationRegistry.Create(ctx, location)
			c.Assert(err, qt.IsNil)

			// Create area
			area := models.Area{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: fmt.Sprintf("bulk-area-user%d", userIndex+1)},
					TenantID: "test-tenant-id",
					UserID:   user.ID,
				},
				Name:       fmt.Sprintf("User%d Bulk Area", userIndex+1),
				LocationID: createdLocation.ID,
			}
			userAwareAreaRegistry, err := registrySet.AreaRegistry.WithCurrentUser(ctx)
			c.Assert(err, qt.IsNil)
			createdArea, err := userAwareAreaRegistry.Create(ctx, area)
			c.Assert(err, qt.IsNil)
			areas = append(areas, createdArea)
		}

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
					AreaID:                 areas[userIndex].ID,
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
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to create user-aware registry for user %d: %v", userIndex+1, err))
				_, err = userAwareCommodityRegistry.Create(ctx, commodity)
				c.Assert(err, qt.IsNil, qt.Commentf("Failed to create commodity for user %d: %v", userIndex+1, err))
			}
		}

		// Verify each user can only see their own entities
		userAwareCommodityRegistry1, err := registrySet.CommodityRegistry.WithCurrentUser(ctx1)
		c.Assert(err, qt.IsNil)
		commodities1, err := userAwareCommodityRegistry1.List(ctx1)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to list commodities for user1: %v", err))
		c.Assert(commodities1, qt.HasLen, 10, qt.Commentf("Expected 10 commodities for user1, got %d", len(commodities1)))
		for _, commodity := range commodities1 {
			c.Assert(commodity.GetUserID(), qt.Equals, user1.ID, qt.Commentf("Expected user ID %s, got %s", user1.ID, commodity.GetUserID()))
		}

		userAwareCommodityRegistry2, err := registrySet.CommodityRegistry.WithCurrentUser(ctx2)
		c.Assert(err, qt.IsNil)
		commodities2, err := userAwareCommodityRegistry2.List(ctx2)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to list commodities for user2: %v", err))
		c.Assert(commodities2, qt.HasLen, 10, qt.Commentf("Expected 10 commodities for user2, got %d", len(commodities2)))
		for _, commodity := range commodities2 {
			c.Assert(commodity.GetUserID(), qt.Equals, user2.ID, qt.Commentf("Expected user ID %s, got %s", user2.ID, commodity.GetUserID()))
		}

		userAwareCommodityRegistry3, err := registrySet.CommodityRegistry.WithCurrentUser(ctx3)
		c.Assert(err, qt.IsNil)
		commodities3, err := userAwareCommodityRegistry3.List(ctx3)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to list commodities for user3: %v", err))
		c.Assert(commodities3, qt.HasLen, 10, qt.Commentf("Expected 10 commodities for user3, got %d", len(commodities3)))
		for _, commodity := range commodities3 {
			c.Assert(commodity.GetUserID(), qt.Equals, user3.ID, qt.Commentf("Expected user ID %s, got %s", user3.ID, commodity.GetUserID()))
		}
	})
}

// TestUserIsolation_EdgeCases tests edge cases and boundary conditions
func TestUserIsolation_EdgeCases(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	c.Run("Empty User Context", func(c *qt.C) {
		emptyCtx := context.Background()

		userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(emptyCtx)
		if err == nil {
			c.Error("Expected error when creating user-aware registry with empty context")
			return
		}
		// Since WithCurrentUser failed, we can't proceed with List
		_ = userAwareCommodityRegistry
	})

	c.Run("Non-existent User ID", func(c *qt.C) {
		nonExistentCtx := appctx.WithUser(context.Background(), &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "non-existent-id"},
				TenantID: "test-tenant-id",
			},
		})

		// Should not crash, but should return empty results
		userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(nonExistentCtx)
		c.Assert(err, qt.IsNil)
		commodities, err := userAwareCommodityRegistry.List(nonExistentCtx)
		if err != nil {
			c.Fatalf("Unexpected error for non-existent user: %v", err)
		}
		if len(commodities) != 0 {
			c.Errorf("Expected 0 commodities for non-existent user, got %d", len(commodities))
		}
	})

	c.Run("Very Long User ID", func(c *qt.C) {
		longUserID := string(make([]byte, 10000))
		longCtx := appctx.WithUser(context.Background(), &models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: longUserID},
				TenantID: "test-tenant-id",
			},
		})

		// Should handle gracefully
		userAwareCommodityRegistry, err := registrySet.CommodityRegistry.WithCurrentUser(longCtx)
		c.Assert(err, qt.IsNil)
		_, err = userAwareCommodityRegistry.List(longCtx)
		if err == nil {
			c.Fatalf("Unexpected no error for long user ID")
		}
	})
}
