package migratedown_test

import (
	"context"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/ptah/cmd/migratedown"
	"github.com/denisvmedia/inventario/ptah/executor"
	"github.com/denisvmedia/inventario/ptah/migrator"
)

func TestMigrateDownCommand_Creation(t *testing.T) {
	c := qt.New(t)

	cmd := migratedown.NewMigrateDownCommand()
	c.Assert(cmd, qt.IsNotNil)
	c.Assert(cmd.Use, qt.Equals, "migrate-down")
	c.Assert(cmd.Short, qt.Contains, "Roll back migrations")
}

// TestMigrateDownCommand_Integration tests the actual migration logic
// This test requires a real database connection and is skipped if no test database is available
func TestMigrateDownCommand_Integration(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping integration test")
	}

	c := qt.New(t)

	// Create a temporary directory for test migrations
	tempDir := t.TempDir()

	// Create test migration files
	upSQL := `CREATE TABLE test_table (id INTEGER PRIMARY KEY);`
	downSQL := `DROP TABLE test_table;`

	err := os.WriteFile(tempDir+"/001_create_test_table.up.sql", []byte(upSQL), 0644)
	c.Assert(err, qt.IsNil)

	err = os.WriteFile(tempDir+"/001_create_test_table.down.sql", []byte(downSQL), 0644)
	c.Assert(err, qt.IsNil)

	// Connect to database
	conn, err := executor.ConnectToDatabase(dbURL)
	c.Assert(err, qt.IsNil)
	defer conn.Close()

	// Apply migration first
	migrationsFS := os.DirFS(tempDir)
	err = migrator.RunMigrations(context.Background(), conn, migrationsFS)
	c.Assert(err, qt.IsNil)

	// Verify migration was applied
	status, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
	c.Assert(err, qt.IsNil)
	c.Assert(status.CurrentVersion, qt.Equals, 1)

	// Test the migrate down command
	cmd := migratedown.NewMigrateDownCommand()
	cmd.SetArgs([]string{
		"--db-url", dbURL,
		"--migrations-dir", tempDir,
		"--target", "0",
		"--confirm", // Skip confirmation prompt
	})

	err = cmd.Execute()
	c.Assert(err, qt.IsNil)

	// Verify migration was rolled back
	finalStatus, err := migrator.GetMigrationStatus(context.Background(), conn, migrationsFS)
	c.Assert(err, qt.IsNil)
	c.Assert(finalStatus.CurrentVersion, qt.Equals, 0)
}
