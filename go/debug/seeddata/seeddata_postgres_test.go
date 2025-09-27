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
	err = seeddata.SeedData(factorySet)
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

	// Verify that users were created with the correct tenant ID
	users, err := registrySet.UserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(len(users) >= 2, qt.IsTrue, qt.Commentf("Expected at least 2 users, got %d", len(users)))

	// Find the test users
	var adminUser, regularUser *models.User
	for _, user := range users {
		if user.Email == "admin@test-org.com" {
			adminUser = user
		} else if user.Email == "user2@test-org.com" {
			regularUser = user
		}
	}

	c.Assert(adminUser, qt.IsNotNil, qt.Commentf("Admin user not found"))
	c.Assert(adminUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(adminUser.UserID, qt.Equals, adminUser.ID) // Self-referencing user ID
	c.Assert(adminUser.Name, qt.Equals, "Test Administrator")
	c.Assert(adminUser.Role, qt.Equals, models.UserRoleAdmin)
	c.Assert(adminUser.IsActive, qt.Equals, true)

	c.Assert(regularUser, qt.IsNotNil, qt.Commentf("Regular user not found"))
	c.Assert(regularUser.TenantID, qt.Equals, testTenant.ID)
	c.Assert(regularUser.UserID, qt.Equals, regularUser.ID) // Self-referencing user ID
	c.Assert(regularUser.Name, qt.Equals, "Test User 2")
	c.Assert(regularUser.Role, qt.Equals, models.UserRoleUser)
	c.Assert(regularUser.IsActive, qt.Equals, true)
}

func cleanupTestData(c *qt.C, db *sqlx.DB) {
	// Clean up in reverse order of dependencies
	queries := []string{
		"DELETE FROM users WHERE email IN ('admin@test-org.com', 'user2@test-org.com')",
		"DELETE FROM tenants WHERE slug = 'test-org'",
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		// Ignore errors as tables might not exist or be empty
		_ = err
	}
}
