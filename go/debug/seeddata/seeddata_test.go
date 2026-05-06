package seeddata_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSeedData(t *testing.T) {
	c := qt.New(t)

	// Create an in-memory registry for testing
	factorySet := memory.NewFactorySet()

	// Test that seed data creation works without errors
	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsFalse)

	// Verify that a tenant was created
	registrySet := factorySet.CreateServiceRegistrySet()
	tenants, err := registrySet.TenantRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.HasLen, 1)

	tenant := tenants[0]
	c.Assert(tenant.Name, qt.Equals, "Test Organization")
	c.Assert(tenant.Slug, qt.Equals, "test-org")
	c.Assert(tenant.Status, qt.Equals, models.TenantStatusActive)

	// Verify that users were created with the correct tenant ID
	users, err := registrySet.UserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 3)

	// Check that all users have the correct tenant ID. The legacy
	// users.user_id self-FK was dropped by issue #1289 Gap B — the row's
	// own id is authoritative, so there is nothing left to assert on a
	// separate user_id field.
	for _, user := range users {
		c.Assert(user.TenantID, qt.Equals, tenant.ID)
	}

	// Check specific user details
	var adminUser, regularUser, orphanUser *models.User
	for _, user := range users {
		switch user.Email {
		case "admin@test-org.com":
			adminUser = user
		case "user2@test-org.com":
			regularUser = user
		case "orphan@test-org.com":
			orphanUser = user
		}
	}

	c.Assert(adminUser, qt.IsNotNil)
	c.Assert(adminUser.Name, qt.Equals, "Test Administrator")
	c.Assert(adminUser.IsActive, qt.Equals, true)

	c.Assert(regularUser, qt.IsNotNil)
	c.Assert(regularUser.Name, qt.Equals, "Test User 2")
	c.Assert(regularUser.IsActive, qt.Equals, true)

	// Orphan must be active so it can authenticate, but must hold zero
	// group memberships so e2e tests exercise the real `/api/v1/groups`
	// empty-collection response (issue #1277).
	c.Assert(orphanUser, qt.IsNotNil)
	c.Assert(orphanUser.IsActive, qt.Equals, true)
	memberships, err := registrySet.GroupMembershipRegistry.ListByUser(context.Background(), tenant.ID, orphanUser.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(memberships, qt.HasLen, 0)
}

func TestSeedDataIdempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create an in-memory registry for testing
	factorySet := memory.NewFactorySet()

	// First seed should report a fresh insert (alreadySeeded=false).
	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsFalse)

	// Snapshot data-table counts after the first seed; the bug from #1482
	// was that a second call doubled these (locations went 3→6, areas 6→12,
	// commodities ×2), so this is the assertion that pins the regression.
	registrySet := factorySet.CreateServiceRegistrySet()
	locationsAfterFirst, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	areasAfterFirst, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	commoditiesAfterFirst, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)

	// Second seed against the same DB must be a no-op signalled by alreadySeeded=true.
	alreadySeeded, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsTrue)

	// Verify that we still have only one tenant and three users
	// (admin + user2 + orphan — the orphan fixture is gated on the
	// test-org tenant; see SeedData).
	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.HasLen, 1)

	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 3)

	// And the additive data tables — locations, areas, commodities — must
	// have the same counts as after the first seed.
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, len(locationsAfterFirst))

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, len(areasAfterFirst))

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, len(commoditiesAfterFirst))
}

// TestSeedDataDoesNotCreateOrphanInNonTestTenant guards the security gate on
// `ensureOrphanUser`: the orphan fixture has a well-known email and password,
// so it must never be planted outside the test-org tenant. Seeding into an
// arbitrary tenant (e.g. `/api/v1/seed?tenant_slug=acme`) must skip the
// orphan creation entirely.
func TestSeedDataDoesNotCreateOrphanInNonTestTenant(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()

	_, err := registrySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Acme Corp",
		Slug:   "acme",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	_, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{TenantSlug: "acme"})
	c.Assert(err, qt.IsNil)

	users, err := registrySet.UserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	for _, u := range users {
		c.Assert(u.Email, qt.Not(qt.Equals), "orphan@test-org.com")
	}
}
