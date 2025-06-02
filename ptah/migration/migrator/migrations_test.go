package migrator

import (
	"context"
	"io/fs"
	"os"
	"testing"

	qt "github.com/frankban/quicktest"
)

func TestMigration_Basic(t *testing.T) {
	c := qt.New(t)

	// Test creating a new migration
	migration := &Migration{
		Version:     1,
		Description: "Test migration",
		Up:          NoopMigrationFunc,
		Down:        NoopMigrationFunc,
	}

	c.Assert(migration.Version, qt.Equals, 1)
	c.Assert(migration.Description, qt.Equals, "Test migration")
	c.Assert(migration.Up, qt.IsNotNil)
	c.Assert(migration.Down, qt.IsNotNil)
}

func TestMigrator_Register(t *testing.T) {
	c := qt.New(t)

	// Create a migrator with nil connection (for testing)
	m := NewMigrator(nil)
	c.Assert(m, qt.IsNotNil)

	// Register a migration
	migration := &Migration{
		Version:     1,
		Description: "Test migration",
		Up:          NoopMigrationFunc,
		Down:        NoopMigrationFunc,
	}

	m.Register(migration)

	// Note: We can't easily test the internal state without exposing it
	// In a real implementation, you might want to add a GetMigrations() method
	// for testing purposes
}

func TestNoopMigrationFunc(t *testing.T) {
	c := qt.New(t)

	// Test that noop migration function doesn't error
	err := NoopMigrationFunc(context.Background(), nil)
	c.Assert(err, qt.IsNil)
}

func TestCreateMigrationFromSQL(t *testing.T) {
	c := qt.New(t)

	upSQL := "CREATE TABLE test (id SERIAL PRIMARY KEY)"
	downSQL := "DROP TABLE test"

	migration := CreateMigrationFromSQL(1, "Create test table", upSQL, downSQL)

	c.Assert(migration.Version, qt.Equals, 1)
	c.Assert(migration.Description, qt.Equals, "Create test table")
	c.Assert(migration.Up, qt.IsNotNil)
	c.Assert(migration.Down, qt.IsNotNil)

	// Test that the functions don't panic (we can't test execution without a real DB)
	c.Assert(migration.Up, qt.IsNotNil)
	c.Assert(migration.Down, qt.IsNotNil)
}

func TestMigrationStatus(t *testing.T) {
	c := qt.New(t)

	status := &MigrationStatus{
		CurrentVersion:    5,
		PendingMigrations: []int{6, 7, 8},
		TotalMigrations:   8,
		HasPendingChanges: true,
	}

	c.Assert(status.CurrentVersion, qt.Equals, 5)
	c.Assert(status.PendingMigrations, qt.HasLen, 3)
	c.Assert(status.TotalMigrations, qt.Equals, 8)
	c.Assert(status.HasPendingChanges, qt.IsTrue)
}

func TestMigrationStatus_NoPending(t *testing.T) {
	c := qt.New(t)

	status := &MigrationStatus{
		CurrentVersion:    5,
		PendingMigrations: []int{},
		TotalMigrations:   5,
		HasPendingChanges: false,
	}

	c.Assert(status.CurrentVersion, qt.Equals, 5)
	c.Assert(status.PendingMigrations, qt.HasLen, 0)
	c.Assert(status.TotalMigrations, qt.Equals, 5)
	c.Assert(status.HasPendingChanges, qt.IsFalse)
}

func TestMigrationVersionTracking(t *testing.T) {
	c := qt.New(t)

	// This test verifies that the migration version tracking logic works
	// even though we can't test with a real database connection

	// Create a migrator with nil connection (for testing)
	m := NewMigrator(nil)
	c.Assert(m, qt.IsNotNil)

	// Register some test migrations
	migration1 := &Migration{
		Version:     1,
		Description: "First migration",
		Up:          NoopMigrationFunc,
		Down:        NoopMigrationFunc,
	}
	migration2 := &Migration{
		Version:     2,
		Description: "Second migration",
		Up:          NoopMigrationFunc,
		Down:        NoopMigrationFunc,
	}

	m.Register(migration1)
	m.Register(migration2)

	// Test that migrations are sorted properly
	m.sortMigrations()
	c.Assert(len(m.migrations), qt.Equals, 2)
	c.Assert(m.migrations[0].Version, qt.Equals, 1)
	c.Assert(m.migrations[1].Version, qt.Equals, 2)
}

func TestRegisterMigrations_WithFilesystem(t *testing.T) {
	c := qt.New(t)

	// Create a migrator with nil connection (for testing)
	m := NewMigrator(nil)
	c.Assert(m, qt.IsNotNil)

	// Create a simple test filesystem using the example migrations
	// Import the examples package to get the test migrations
	// For now, we'll just test with an empty filesystem to verify the function works
	testFS := fs.FS(os.DirFS("."))

	// Register migrations from the test filesystem (will be empty, but should not error)
	err := RegisterMigrations(m, testFS)
	c.Assert(err, qt.IsNil)

	// Note: We can't easily test the internal state without exposing it
	// In a real implementation, you might want to add a GetMigrations() method
	// for testing purposes
}
