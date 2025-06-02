package migrator

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/dbschema"
)

func TestInitializeDebug(t *testing.T) {
	c := qt.New(t)

	// Skip if no PostgreSQL URL is provided
	dbURL := "postgres://ptah_user:ptah_password@localhost:5432/ptah_test?sslmode=disable"

	// Connect to database
	conn, err := dbschema.ConnectToDatabase(dbURL)
	if err != nil {
		t.Skipf("Skipping test: failed to connect to database: %v", err)
	}
	defer conn.Close()

	// Clean up any existing schema_migrations table to ensure a clean test
	ctx := context.Background()
	_, _ = conn.Exec("DROP TABLE IF EXISTS schema_migrations")

	// Create a migrator
	m := NewMigrator(conn)

	// Test Initialize method directly
	err = m.Initialize(ctx)
	c.Assert(err, qt.IsNil, qt.Commentf("Initialize should not fail"))

	// Test that the table was created
	var count int
	row := conn.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_name = 'schema_migrations'")
	err = row.Scan(&count)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1, qt.Commentf("schema_migrations table should exist"))

	// Test GetCurrentVersion
	version, err := m.GetCurrentVersion(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(version, qt.Equals, 0, qt.Commentf("Initial version should be 0"))
}
