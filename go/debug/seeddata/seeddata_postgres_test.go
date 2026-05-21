//go:build integration

package seeddata_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/postgres"
)

func TestSeedDataPostgreSQL(t *testing.T) {
	c := qt.New(t)

	// Connect to test database
	dsn := "postgres://inventario:inventario_password@localhost:5432/inventario?sslmode=disable"
	db, err := sqlx.Open("postgres", dsn)
	c.Assert(err, qt.IsNil)
	defer db.Close()

	// Test connection
	err = db.Ping()
	c.Assert(err, qt.IsNil)

	// Create factory set with PostgreSQL
	factorySet := postgres.NewFactorySet(db)

	// Clean up any existing test data first
	cleanupTestData(c, db)

	// Test that seed data creation works without errors
	_, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)

	// Verify that a tenant was created
	registrySet := factorySet.CreateServiceRegistrySet()
	tenants, err := registrySet.TenantRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(len(tenants) >= 1, qt.IsTrue, qt.Commentf("Expected at least 1 tenant, got %d", len(tenants)))

	// Find the test tenant
	var testTenant *models.Tenant
	for _, tenant := range tenants {
		if tenant.Slug == "test-org" {
			testTenant = tenant
			break
		}
	}
	c.Assert(testTenant, qt.IsNotNil, qt.Commentf("Test tenant with slug 'test-org' not found"))
	c.Assert(testTenant.Name, qt.Equals, "Test Organization")
	c.Assert(testTenant.Status, qt.Equals, models.TenantStatusActive)

	// Verify that users were created with the correct tenant ID.
	// Seven well-known fixture users land in test-org after #1758:
	// admin, user2, orphan, family (owner of the secondary group),
	// teammate (second member of admin's primary group), sysadmin
	// (platform system admin) and blocktarget (block/unblock fixture).
	users, err := registrySet.UserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(len(users) >= 7, qt.IsTrue, qt.Commentf("Expected at least 7 users, got %d", len(users)))

	// Find the test users
	var adminUser, regularUser, orphanUser, familyUser, sysadminUser *models.User
	for _, user := range users {
		switch user.Email {
		case "admin@test-org.com":
			adminUser = user
		case "user2@test-org.com":
			regularUser = user
		case "orphan@test-org.com":
			orphanUser = user
		case "family@test-org.com":
			familyUser = user
		case "sysadmin@test-org.com":
			sysadminUser = user
		}
	}

	c.Assert(adminUser, qt.IsNotNil, qt.Commentf("Admin user not found"))
	c.Assert(adminUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(adminUser.Name, qt.Equals, "Test Administrator")
	c.Assert(adminUser.IsActive, qt.Equals, true)

	c.Assert(regularUser, qt.IsNotNil, qt.Commentf("Regular user not found"))
	c.Assert(regularUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(regularUser.Name, qt.Equals, "Test User 2")
	c.Assert(regularUser.IsActive, qt.Equals, true)

	// Orphan: active so it can authenticate, zero memberships so e2e tests
	// hit the real `/api/v1/groups` empty-collection response (issue #1277).
	c.Assert(orphanUser, qt.IsNotNil, qt.Commentf("Orphan user not found"))
	c.Assert(orphanUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(orphanUser.IsActive, qt.Equals, true)
	memberships, err := registrySet.GroupMembershipRegistry.ListByUser(context.Background(), testTenant.ID, orphanUser.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(memberships, qt.HasLen, 0)

	// Family user owns the secondary group (Family).
	c.Assert(familyUser, qt.IsNotNil, qt.Commentf("family user not found"))
	c.Assert(familyUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(familyUser.IsActive, qt.IsTrue)

	// Sysadmin: the is_system_admin flag must round-trip through the
	// Postgres INSERT/SELECT path (issue #1758).
	c.Assert(sysadminUser, qt.IsNotNil, qt.Commentf("sysadmin user not found"))
	c.Assert(sysadminUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(sysadminUser.IsActive, qt.IsTrue)
	c.Assert(sysadminUser.IsSystemAdmin, qt.IsTrue)
}

func cleanupTestData(c *qt.C, db *sqlx.DB) {
	// Clean up in reverse order of dependencies
	queries := []string{
		"DELETE FROM users WHERE email IN ('admin@test-org.com', 'user2@test-org.com', 'orphan@test-org.com', 'family@test-org.com', 'teammate@test-org.com', 'sysadmin@test-org.com', 'blocktarget@test-org.com')",
		"DELETE FROM tenants WHERE slug = 'test-org'",
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		// Ignore errors as tables might not exist or be empty
		_ = err
	}
}
