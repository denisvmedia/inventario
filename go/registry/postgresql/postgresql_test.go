package postgresql_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgresql"
)

// TestPostgreSQLRegistration tests that the PostgreSQL registry is registered correctly
func TestPostgreSQLRegistration(t *testing.T) {
	c := qt.New(t)

	// Register the PostgreSQL registry
	postgresql.Register()

	// Get the registry
	registries := registry.Registries()
	c.Assert(registries, qt.Not(qt.IsNil))

	// Check that the PostgreSQL registry is registered
	_, ok := registries["postgresql"]
	c.Assert(ok, qt.IsTrue)
}

// TestParsePostgreSQLURL tests the ParsePostgreSQLURL function
func TestParsePostgreSQLURL(t *testing.T) {
	c := qt.New(t)

	// Test cases
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "basic URL",
			input:    "postgresql://localhost:5432",
			expected: "postgres://localhost:5432",
		},
		{
			name:     "URL with database",
			input:    "postgresql://localhost:5432/inventario",
			expected: "postgres://localhost:5432/inventario",
		},
		{
			name:     "URL with username and password",
			input:    "postgresql://username:password@localhost:5432/inventario",
			expected: "postgres://username:password@localhost:5432/inventario",
		},
		{
			name:     "URL with query parameters",
			input:    "postgresql://localhost:5432/inventario?sslmode=disable",
			expected: "postgres://localhost:5432/inventario?sslmode=disable",
		},
	}

	// Run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			// Parse the URL
			parsed, err := registry.Config(tc.input).Parse()
			c.Assert(err, qt.IsNil)

			// Convert to PostgreSQL connection string
			result := postgresql.ParsePostgreSQLURL(parsed)
			c.Assert(result, qt.Equals, tc.expected)
		})
	}
}

// Integration tests for PostgreSQL
// These tests require a PostgreSQL server to be running
// They are skipped by default unless the POSTGRES_TEST_DSN environment variable is set

func TestPostgreSQLIntegration(t *testing.T) {
	// Skip if no PostgreSQL DSN is provided
	dsn := os.Getenv("POSTGRES_TEST_DSN")
	if dsn == "" {
		t.Skip("Skipping PostgreSQL integration tests: POSTGRES_TEST_DSN not set")
	}

	c := qt.New(t)

	// Create a test database
	pool, err := pgxpool.New(context.Background(), dsn)
	c.Assert(err, qt.IsNil)
	defer pool.Close()

	// Clean up any existing test tables
	_, err = pool.Exec(context.Background(), `
		DROP TABLE IF EXISTS locations CASCADE;
		DROP TABLE IF EXISTS areas CASCADE;
		DROP TABLE IF EXISTS commodities CASCADE;
		DROP TABLE IF EXISTS images CASCADE;
		DROP TABLE IF EXISTS invoices CASCADE;
		DROP TABLE IF EXISTS manuals CASCADE;
		DROP TABLE IF EXISTS settings CASCADE;
	`)
	c.Assert(err, qt.IsNil)

	// Create a registry set
	registrySet, err := postgresql.NewRegistrySet(registry.Config(dsn))
	c.Assert(err, qt.IsNil)
	c.Assert(registrySet, qt.Not(qt.IsNil))

	// Validate the registry set
	err = registrySet.Validate()
	c.Assert(err, qt.IsNil)

	// Check that all registries are created
	c.Assert(registrySet.LocationRegistry, qt.Not(qt.IsNil))
	c.Assert(registrySet.AreaRegistry, qt.Not(qt.IsNil))
	c.Assert(registrySet.CommodityRegistry, qt.Not(qt.IsNil))
	c.Assert(registrySet.ImageRegistry, qt.Not(qt.IsNil))
	c.Assert(registrySet.InvoiceRegistry, qt.Not(qt.IsNil))
	c.Assert(registrySet.ManualRegistry, qt.Not(qt.IsNil))
	c.Assert(registrySet.SettingsRegistry, qt.Not(qt.IsNil))
}
