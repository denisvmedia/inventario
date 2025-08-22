package integration_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
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
		createdLocation1, err := registrySet.LocationRegistry.CreateWithUser(ctx1, location1)
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
		createdArea1, err := registrySet.AreaRegistry.CreateWithUser(ctx1, area1)
		c.Assert(err, qt.IsNil)

		commodity1 := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-commodity-1"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name:   "User1 Product",
			AreaID: &createdArea1.ID,
		}
		createdCommodity1, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity1)
		c.Assert(err, qt.IsNil)

		// User2 tries to access User1's entities through relationships
		// Should not be able to access location
		_, err = registrySet.LocationRegistry.GetWithUser(ctx2, createdLocation1.ID)
		c.Assert(err, qt.IsNotNil)

		// Should not be able to access area
		_, err = registrySet.AreaRegistry.GetWithUser(ctx2, createdArea1.ID)
		c.Assert(err, qt.IsNotNil)

		// Should not be able to access commodity
		_, err = registrySet.CommodityRegistry.GetWithUser(ctx2, createdCommodity1.ID)
		c.Assert(err, qt.IsNotNil)

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
		createdLocation2, err := registrySet.LocationRegistry.CreateWithUser(ctx2, location2)
		c.Assert(err, qt.IsNil)

		// Verify users can only see their own entities despite same names
		locations1, err := registrySet.LocationRegistry.ListWithUser(ctx1)
		c.Assert(err, qt.IsNil)
		c.Assert(len(locations1), qt.Equals, 1)
		c.Assert(locations1[0].ID, qt.Equals, createdLocation1.ID)

		locations2, err := registrySet.LocationRegistry.ListWithUser(ctx2)
		c.Assert(err, qt.IsNil)
		c.Assert(len(locations2), qt.Equals, 1)
		c.Assert(locations2[0].ID, qt.Equals, createdLocation2.ID)
	})

	t.Run("File Associations and Isolation", func(t *testing.T) {
		c := qt.New(t)

		// User1 creates a commodity with associated files
		commodity := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-commodity-with-files"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name: "Commodity with Files",
		}
		createdCommodity, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity)
		c.Assert(err, qt.IsNil)

		// Create files associated with the commodity
		file1 := models.File{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-file-1"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			OriginalPath: "/uploads/user1-file1.jpg",
			Path:         "user1-file1",
			Ext:          "jpg",
			Type:         "image/jpeg",
			Size:         1024,
			EntityType:   "commodity",
			EntityID:     createdCommodity.ID,
		}
		createdFile1, err := registrySet.FileRegistry.CreateWithUser(ctx1, file1)
		c.Assert(err, qt.IsNil)

		// User2 should not be able to access User1's files
		_, err = registrySet.FileRegistry.GetWithUser(ctx2, createdFile1.ID)
		c.Assert(err, qt.IsNotNil)

		// User2 should not see User1's files in list
		files2, err := registrySet.FileRegistry.ListWithUser(ctx2)
		c.Assert(err, qt.IsNil)
		c.Assert(len(files2), qt.Equals, 0)

		// User1 can see their own files
		files1, err := registrySet.FileRegistry.ListWithUser(ctx1)
		c.Assert(err, qt.IsNil)
		c.Assert(len(files1), qt.Equals, 1)
		c.Assert(files1[0].ID, qt.Equals, createdFile1.ID)
	})

	t.Run("Export Data Isolation", func(t *testing.T) {
		c := qt.New(t)

		// User1 creates commodities and exports
		for i := 0; i < 5; i++ {
			commodity := models.Commodity{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: fmt.Sprintf("export-commodity-%d", i)},
					TenantID: "test-tenant-id",
					UserID:   user1.ID,
				},
				Name: fmt.Sprintf("Export Test Commodity %d", i),
			}
			_, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity)
			c.Assert(err, qt.IsNil)
		}

		// User1 creates an export
		export1 := models.Export{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "comp-export-1"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name:        "User1 Data Export",
			Description: "Export of User1's data",
			Status:      models.ExportStatusCompleted,
		}
		createdExport1, err := registrySet.ExportRegistry.CreateWithUser(ctx1, export1)
		c.Assert(err, qt.IsNil)

		// User2 should not see User1's export
		_, err = registrySet.ExportRegistry.GetWithUser(ctx2, createdExport1.ID)
		c.Assert(err, qt.IsNotNil)

		exports2, err := registrySet.ExportRegistry.ListWithUser(ctx2)
		c.Assert(err, qt.IsNil)
		c.Assert(len(exports2), qt.Equals, 0)

		// User1 can see their own export
		exports1, err := registrySet.ExportRegistry.ListWithUser(ctx1)
		c.Assert(err, qt.IsNil)
		c.Assert(len(exports1), qt.Equals, 1)
		c.Assert(exports1[0].ID, qt.Equals, createdExport1.ID)
	})

	t.Run("Cross-User Update Attempts", func(t *testing.T) {
		c := qt.New(t)

		// User1 creates entities
		commodity1 := models.Commodity{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: "update-test-commodity"},
				TenantID: "test-tenant-id",
				UserID:   user1.ID,
			},
			Name:        "Original Name",
			Description: "Original Description",
		}
		created1, err := registrySet.CommodityRegistry.CreateWithUser(ctx1, commodity1)
		c.Assert(err, qt.IsNil)

		// User2 tries to update User1's commodity
		created1.Name = "Hacked Name"
		created1.Description = "Hacked Description"
		_, err = registrySet.CommodityRegistry.UpdateWithUser(ctx2, *created1)
		c.Assert(err, qt.IsNotNil)

		// User3 also tries to update User1's commodity
		created1.Name = "Another Hack"
		_, err = registrySet.CommodityRegistry.UpdateWithUser(ctx3, *created1)
		c.Assert(err, qt.IsNotNil)

		// Verify the commodity remains unchanged
		retrieved, err := registrySet.CommodityRegistry.GetWithUser(ctx1, created1.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(retrieved.Name, qt.Equals, "Original Name")
		c.Assert(retrieved.Description, qt.Equals, "Original Description")
	})

	t.Run("Bulk Operations Isolation", func(t *testing.T) {
		c := qt.New(t)

		// Each user creates multiple entities
		for userIndex, ctx := range []context.Context{ctx1, ctx2, ctx3} {
			for i := 0; i < 10; i++ {
				commodity := models.Commodity{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: fmt.Sprintf("bulk-commodity-user%d-%d", userIndex+1, i)},
						TenantID: "test-tenant-id",
						UserID:   []*models.User{user1, user2, user3}[userIndex].ID,
					},
					Name: fmt.Sprintf("User%d Commodity %d", userIndex+1, i),
				}
				_, err := registrySet.CommodityRegistry.CreateWithUser(ctx, commodity)
				c.Assert(err, qt.IsNil)
			}
		}

		// Verify each user can only see their own entities
		commodities1, err := registrySet.CommodityRegistry.ListWithUser(ctx1)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities1), qt.Equals, 10)
		for _, commodity := range commodities1 {
			c.Assert(commodity.GetUserID(), qt.Equals, user1.ID)
		}

		commodities2, err := registrySet.CommodityRegistry.ListWithUser(ctx2)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities2), qt.Equals, 10)
		for _, commodity := range commodities2 {
			c.Assert(commodity.GetUserID(), qt.Equals, user2.ID)
		}

		commodities3, err := registrySet.CommodityRegistry.ListWithUser(ctx3)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities3), qt.Equals, 10)
		for _, commodity := range commodities3 {
			c.Assert(commodity.GetUserID(), qt.Equals, user3.ID)
		}
	})
}

// TestUserIsolation_EdgeCases tests edge cases and boundary conditions
func TestUserIsolation_EdgeCases(t *testing.T) {
	c := qt.New(t)
	registrySet, cleanup := setupTestDatabase(t)
	defer cleanup()

	user := createTestUser(c, registrySet, "edge@example.com")
	ctx := registry.WithUserContext(context.Background(), user.ID)

	t.Run("Empty User Context", func(t *testing.T) {
		c := qt.New(t)
		emptyCtx := context.Background()

		_, err := registrySet.CommodityRegistry.ListWithUser(emptyCtx)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*user context required.*")
	})

	t.Run("Nil User Context", func(t *testing.T) {
		c := qt.New(t)
		nilCtx := registry.WithUserContext(context.Background(), "")

		_, err := registrySet.CommodityRegistry.ListWithUser(nilCtx)
		c.Assert(err, qt.IsNotNil)
		c.Assert(err, qt.ErrorMatches, ".*user context required.*")
	})

	t.Run("Non-existent User ID", func(t *testing.T) {
		c := qt.New(t)
		nonExistentCtx := registry.WithUserContext(context.Background(), "non-existent-user-id")

		// Should not crash, but should return empty results
		commodities, err := registrySet.CommodityRegistry.ListWithUser(nonExistentCtx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities), qt.Equals, 0)
	})

	t.Run("Very Long User ID", func(t *testing.T) {
		c := qt.New(t)
		longUserID := string(make([]byte, 10000))
		longCtx := registry.WithUserContext(context.Background(), longUserID)

		// Should handle gracefully
		commodities, err := registrySet.CommodityRegistry.ListWithUser(longCtx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities), qt.Equals, 0)
	})

	t.Run("Special Characters in User ID", func(t *testing.T) {
		c := qt.New(t)
		specialUserID := "user-id-with-special-chars-!@#$%^&*()_+-=[]{}|;:,.<>?"
		specialCtx := registry.WithUserContext(context.Background(), specialUserID)

		// Should handle gracefully
		commodities, err := registrySet.CommodityRegistry.ListWithUser(specialCtx)
		c.Assert(err, qt.IsNil)
		c.Assert(len(commodities), qt.Equals, 0)
	})
}
