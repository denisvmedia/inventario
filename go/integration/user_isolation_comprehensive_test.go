package integration_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TestUserIsolation_ComprehensiveScenarios tests complex real-world scenarios.
// Isolation is GROUP-scoped, so every party lives in its own group within a
// shared tenant — that is what makes the cross-party negative assertions
// meaningful.
func TestUserIsolation_ComprehensiveScenarios(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	defer cleanup()

	c.Run("Complex Entity Relationships", func(c *qt.C) {
		user1, user2 := newIsolationPair(c, fs)
		registrySet := must.Must(fs.CreateUserRegistrySet(user1.ctx))
		registrySet2 := must.Must(fs.CreateUserRegistrySet(user2.ctx))

		// User1 creates a location → area → commodity in group1.
		createdLocation1 := seedLocation(c, fs, user1, "User1 Warehouse")
		createdArea1 := seedArea(c, fs, user1, createdLocation1.ID, "User1 Storage Area")
		_ = seedCommodity(c, fs, user1, createdArea1.ID, "User1 Product", "UP1")

		// User2 (different group) cannot reach user1's location or area.
		_, err := registrySet2.LocationRegistry.Get(user2.ctx, createdLocation1.ID)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user2 tries to access user1's location"))

		_, err = registrySet2.AreaRegistry.Get(user2.ctx, createdArea1.ID)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user2 tries to access user1's area"))

		// User2 creates their own location with the SAME name in group2.
		_ = seedLocation(c, fs, user2, "User1 Warehouse")

		// Each user sees only their own location despite identical names.
		locations1, err := registrySet.LocationRegistry.List(user1.ctx)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to list locations for user1: %v", err))
		c.Assert(locations1, qt.HasLen, 1)
		c.Assert(locations1[0].ID, qt.Equals, createdLocation1.ID)

		locations2, err := registrySet2.LocationRegistry.List(user2.ctx)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to list locations for user2: %v", err))
		c.Assert(locations2, qt.HasLen, 1)
	})

	c.Run("Cross-User Update Attempts", func(c *qt.C) {
		// Three users, each in their own group; users 2 and 3 must not be able
		// to mutate user1's commodity.
		tenant := createIsolationTenant(c, fs)
		owner := newGroupedUser(c, fs, tenant, "owner@comprehensive.com", "Owner Group")
		attacker2 := newGroupedUser(c, fs, tenant, "attacker2@comprehensive.com", "Attacker2 Group")
		attacker3 := newGroupedUser(c, fs, tenant, "attacker3@comprehensive.com", "Attacker3 Group")

		registrySet2 := must.Must(fs.CreateUserRegistrySet(attacker2.ctx))
		registrySet3 := must.Must(fs.CreateUserRegistrySet(attacker3.ctx))

		createdLocation1 := seedLocation(c, fs, owner, "User1 Warehouse")
		createdArea1 := seedArea(c, fs, owner, createdLocation1.ID, "User1 Storage Area")
		created1 := seedCommodity(c, fs, owner, createdArea1.ID, "Original Name", "ON")

		// User2 tries to update user1's commodity.
		created1.Name = "Hacked Name"
		created1.ShortName = "HN"
		_, err := registrySet2.CommodityRegistry.Update(attacker2.ctx, *created1)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user2 tries to update user1's commodity"))

		// User3 tries the same.
		created1.Name = "Another Hack"
		_, err = registrySet3.CommodityRegistry.Update(attacker3.ctx, *created1)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when user3 tries to update user1's commodity"))

		// The commodity is unchanged when read back by its owner.
		registrySet1 := must.Must(fs.CreateUserRegistrySet(owner.ctx))
		retrieved, err := registrySet1.CommodityRegistry.Get(owner.ctx, created1.ID)
		c.Assert(err, qt.IsNil, qt.Commentf("Failed to retrieve commodity: %v", err))
		c.Assert(retrieved, qt.IsNotNil)
		c.Assert(retrieved.Name, qt.Equals, "Original Name")
		c.Assert(retrieved.ShortName, qt.Equals, "ON")
	})

	c.Run("Bulk Operations Isolation", func(c *qt.C) {
		// Three users, each in their own group, each creating 10 commodities.
		// Every user must see exactly their own 10.
		tenant := createIsolationTenant(c, fs)
		fixtures := []isolationFixture{
			newGroupedUser(c, fs, tenant, "user1t3@comprehensive.com", "Bulk Group 1"),
			newGroupedUser(c, fs, tenant, "user2t3@comprehensive.com", "Bulk Group 2"),
			newGroupedUser(c, fs, tenant, "user3t3@comprehensive.com", "Bulk Group 3"),
		}

		registrySets := make([]*registry.Set, len(fixtures))
		for i, f := range fixtures {
			registrySets[i] = must.Must(fs.CreateUserRegistrySet(f.ctx))
		}

		// Each user gets a location + area, then 10 commodities under it.
		for userIndex, f := range fixtures {
			loc := seedLocation(c, fs, f, fmt.Sprintf("User%d Bulk Location", userIndex+1))
			area := seedArea(c, fs, f, loc.ID, fmt.Sprintf("User%d Bulk Area", userIndex+1))
			for i := range 10 {
				_ = seedCommodity(c, fs, f, area.ID,
					fmt.Sprintf("User%d Commodity %d", userIndex+1, i),
					fmt.Sprintf("U%dC%d", userIndex+1, i))
			}
		}

		// Verify each user sees exactly their own 10 commodities.
		for userIndex, f := range fixtures {
			commodities, err := registrySets[userIndex].CommodityRegistry.List(f.ctx)
			c.Assert(err, qt.IsNil, qt.Commentf("Failed to list commodities for user %d: %v", userIndex+1, err))
			c.Assert(commodities, qt.HasLen, 10, qt.Commentf("Expected 10 commodities for user %d, got %d", userIndex+1, len(commodities)))
			for _, commodity := range commodities {
				c.Assert(commodity.GetCreatedByUserID(), qt.Equals, f.user.ID)
			}
		}
	})
}

// newGroupedUser is a convenience for sub-tests that need more than the two
// users newIsolationPair provides: one user in their own fresh group within the
// given tenant, with a ready WithUser+WithGroup context.
func newGroupedUser(c *qt.C, fs *registry.FactorySet, tenant *models.Tenant, email, groupName string) isolationFixture {
	c.Helper()
	user := createTestUser(c, fs.UserRegistry, tenant.ID, email)
	group := createTestGroup(c, fs, tenant.ID, user.ID, groupName)
	return isolationFixture{
		tenant: tenant,
		user:   user,
		group:  group,
		ctx:    userGroupContext(context.Background(), user, group),
	}
}

// TestUserIsolation_EdgeCases tests edge cases and boundary conditions around
// the user-aware registry-set factory.
func TestUserIsolation_EdgeCases(t *testing.T) {
	c := qt.New(t)
	fs, cleanup := setupTestDatabase(t)
	c.Cleanup(cleanup)

	c.Run("Empty User Context", func(c *qt.C) {
		emptyCtx := context.Background()
		_, err := fs.CreateUserRegistrySet(emptyCtx)
		c.Assert(err, qt.IsNotNil, qt.Commentf("Expected error when no user context is provided"))
		c.Assert(err.Error(), qt.Contains, "user context required")
	})

	c.Run("Non-existent User ID", func(c *qt.C) {
		// A well-formed but unknown user with a group context still resolves a
		// registry set; it simply sees no rows (RLS denies everything).
		nonExistentCtx := userGroupContext(
			context.Background(),
			&models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "non-existent-id"},
					TenantID: "non-existent-tenant-id",
				},
			},
			&models.LocationGroup{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "non-existent-group-id"},
					TenantID: "non-existent-tenant-id",
				},
				GroupCurrency: models.Currency("USD"),
			},
		)

		registrySet, err := fs.CreateUserRegistrySet(nonExistentCtx)
		c.Assert(err, qt.IsNil)
		commodities, err := registrySet.CommodityRegistry.List(nonExistentCtx)
		c.Assert(err, qt.IsNil, qt.Commentf("Expected no error for non-existent user"))
		c.Assert(commodities, qt.HasLen, 0, qt.Commentf("Expected 0 commodities for non-existent user"))
	})

	c.Run("Very Long User ID", func(c *qt.C) {
		longUserID := strings.Repeat("a", 10000)
		longCtx := userGroupContext(
			context.Background(),
			&models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: longUserID},
					TenantID: "non-existent-tenant-id",
				},
			},
			&models.LocationGroup{
				TenantAwareEntityID: models.TenantAwareEntityID{
					EntityID: models.EntityID{ID: "non-existent-group-id"},
					TenantID: "non-existent-tenant-id",
				},
				GroupCurrency: models.Currency("USD"),
			},
		)

		// Should handle gracefully. Denial at registry-set creation is
		// acceptable; if it succeeds, isolation must still hold — a
		// non-existent tenant/group must surface zero rows, never a leak.
		registrySet, err := fs.CreateUserRegistrySet(longCtx)
		if err != nil {
			return
		}
		commodities, err := registrySet.CommodityRegistry.List(longCtx)
		if err == nil {
			c.Assert(commodities, qt.HasLen, 0)
		}
	})
}
