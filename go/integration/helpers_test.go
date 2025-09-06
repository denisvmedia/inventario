package integration_test

import (
	"bytes"
	"fmt"

	"github.com/denisvmedia/inventario/cmd/inventario/db/bootstrap/apply"
	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/up"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

// setupFreshDatabase runs bootstrap and migration commands to set up a fresh database
// This is a shared helper function used by multiple integration tests
func setupFreshDatabase(dsn string) error {
	// Step 1: Run bootstrap migrations
	dbConfig := &shared.DatabaseConfig{DBDSN: dsn}
	bootstrapCmd := apply.New(dbConfig)

	// Register the database flags for the bootstrap command
	shared.RegisterLocalDatabaseFlags(bootstrapCmd.Cmd(), dbConfig)

	// Capture output
	var bootstrapOutput bytes.Buffer
	bootstrapCmd.Cmd().SetOut(&bootstrapOutput)
	bootstrapCmd.Cmd().SetErr(&bootstrapOutput)

	// Set bootstrap arguments
	bootstrapCmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
		"--username=inventario",
		"--username-for-migrations=inventario",
	})

	if err := bootstrapCmd.Cmd().Execute(); err != nil {
		return fmt.Errorf("bootstrap failed: %w\nOutput: %s", err, bootstrapOutput.String())
	}

	// Step 2: Run schema migrations
	migrateCmd := up.New(dbConfig)

	// Register the database flags for the migration command
	shared.RegisterLocalDatabaseFlags(migrateCmd.Cmd(), dbConfig)

	// Capture output
	var migrateOutput bytes.Buffer
	migrateCmd.Cmd().SetOut(&migrateOutput)
	migrateCmd.Cmd().SetErr(&migrateOutput)

	// Set migration arguments
	migrateCmd.Cmd().SetArgs([]string{
		"--db-dsn=" + dsn,
	})

	if err := migrateCmd.Cmd().Execute(); err != nil {
		return fmt.Errorf("migration failed: %w\nOutput: %s", err, migrateOutput.String())
	}

	return nil
}
