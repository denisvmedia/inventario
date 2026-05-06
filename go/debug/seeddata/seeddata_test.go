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
	err := seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)

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

	// Create an in-memory registry for testing
	factorySet := memory.NewFactorySet()

	// Run seed data twice to ensure it's idempotent
	err := seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)

	err = seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)

	// Verify that we still have only one tenant and two users
	registrySet := factorySet.CreateServiceRegistrySet()
	tenants, err := registrySet.TenantRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.HasLen, 1)

	users, err := registrySet.UserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 3)
}
