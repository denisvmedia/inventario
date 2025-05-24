package migrations

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestMigrations(t *testing.T) {
	// This test requires a PostgreSQL database to be running
	// It's meant to be run manually or in a CI environment with PostgreSQL
	// Skip it in normal test runs
	t.Skip("Skipping migration test as it requires a PostgreSQL database")

	// Connect to the database
	pool, err := pgxpool.New(context.Background(), "postgres://postgres:postgres@localhost:5432/inventario_test")
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run migrations
	err = RunMigrations(context.Background(), pool)
	if err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	// Verify migrations were applied
	var version int
	err = pool.QueryRow(context.Background(), `
		SELECT COALESCE(MAX(version), 0) FROM schema_migrations
	`).Scan(&version)
	if err != nil {
		t.Fatalf("Failed to get current schema version: %v", err)
	}

	// Check that at least one migration was applied
	if version < 1 {
		t.Errorf("Expected at least one migration to be applied, got version %d", version)
	}

	// Verify tables were created
	tables := []string{"locations", "areas", "commodities", "images", "invoices", "manuals", "settings"}
	for _, table := range tables {
		var exists bool
		err = pool.QueryRow(context.Background(), `
			SELECT EXISTS (
				SELECT FROM information_schema.tables 
				WHERE table_schema = 'public' 
				AND table_name = $1
			)
		`, table).Scan(&exists)
		if err != nil {
			t.Fatalf("Failed to check if table %s exists: %v", table, err)
		}
		if !exists {
			t.Errorf("Expected table %s to exist", table)
		}
	}
}