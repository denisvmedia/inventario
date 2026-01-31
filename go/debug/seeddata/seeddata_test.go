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
	c.Assert(users, qt.HasLen, 2)

	// Check that both users have the correct tenant ID
	for _, user := range users {
		c.Assert(user.TenantID, qt.Equals, tenant.ID)
		c.Assert(user.UserID, qt.Equals, user.ID) // Self-referencing user ID
	}

	// Check specific user details
	var adminUser, regularUser *models.User
	for _, user := range users {
		switch user.Email {
		case "admin@test-org.com":
			adminUser = user
		case "user2@test-org.com":
			regularUser = user
		}
	}

	c.Assert(adminUser, qt.IsNotNil)
	c.Assert(adminUser.Name, qt.Equals, "Test Administrator")
	c.Assert(adminUser.Role, qt.Equals, models.UserRoleAdmin)
	c.Assert(adminUser.IsActive, qt.Equals, true)

	c.Assert(regularUser, qt.IsNotNil)
	c.Assert(regularUser.Name, qt.Equals, "Test User 2")
	c.Assert(regularUser.Role, qt.Equals, models.UserRoleUser)
	c.Assert(regularUser.IsActive, qt.Equals, true)
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
	c.Assert(users, qt.HasLen, 2)
}
