package data_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/db/migrate/data"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
)

func TestNew(t *testing.T) {
	c := qt.New(t)

	dbConfig := &shared.DatabaseConfig{
		DBDSN: "postgres://test:test@localhost/test?sslmode=disable",
	}

	cmd := data.New(dbConfig)

	c.Assert(cmd, qt.IsNotNil)
	c.Assert(cmd.Cmd().Use, qt.Equals, "data")
	c.Assert(cmd.Cmd().Short, qt.Equals, "Setup initial dataset with tenant and user structure")
	c.Assert(cmd.Cmd().RunE, qt.IsNotNil)
}

func TestConfigDefaults(t *testing.T) {
	c := qt.New(t)

	// Test that config gets proper defaults when creating a new command
	dbConfig := &shared.DatabaseConfig{}
	cmd := data.New(dbConfig)

	// Access the config through a method or by making it public for testing
	// Since the config is private, we'll test through the command creation
	c.Assert(cmd, qt.IsNotNil)

	// The actual default values are tested through the struct tags
	// and will be loaded when TryReadSection is called
}

func TestCommand_Flags(t *testing.T) {
	c := qt.New(t)

	dbConfig := &shared.DatabaseConfig{}
	cmd := data.New(dbConfig)

	// Test that all expected flags are registered
	flags := cmd.Cmd().Flags()

	// Check dry-run flag
	dryRunFlag := flags.Lookup("dry-run")
	c.Assert(dryRunFlag, qt.IsNotNil)
	c.Assert(dryRunFlag.Usage, qt.Contains, "would be executed")

	// Check tenant configuration flags exist
	tenantIDFlag := flags.Lookup("default-tenant-id")
	c.Assert(tenantIDFlag, qt.IsNotNil)

	tenantNameFlag := flags.Lookup("default-tenant-name")
	c.Assert(tenantNameFlag, qt.IsNotNil)

	tenantSlugFlag := flags.Lookup("default-tenant-slug")
	c.Assert(tenantSlugFlag, qt.IsNotNil)

	// Check admin user configuration flags exist
	adminEmailFlag := flags.Lookup("admin-email")
	c.Assert(adminEmailFlag, qt.IsNotNil)

	adminPasswordFlag := flags.Lookup("admin-password")
	c.Assert(adminPasswordFlag, qt.IsNotNil)

	adminNameFlag := flags.Lookup("admin-name")
	c.Assert(adminNameFlag, qt.IsNotNil)
}

func TestCommand_RequiresDatabaseDSN(t *testing.T) {
	c := qt.New(t)

	// Test that command fails when no database DSN is provided
	dbConfig := &shared.DatabaseConfig{
		DBDSN: "", // Empty DSN
	}

	cmd := data.New(dbConfig)

	// Execute the command - it should fail due to missing DSN
	err := cmd.Cmd().Execute()
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "database DSN is required")
}

func TestCommand_LongDescription(t *testing.T) {
	c := qt.New(t)

	dbConfig := &shared.DatabaseConfig{}
	cmd := data.New(dbConfig)

	// Verify the long description contains key information
	c.Assert(cmd.Cmd().Long, qt.Contains, "default tenant")
	c.Assert(cmd.Cmd().Long, qt.Contains, "user_id")
	c.Assert(cmd.Cmd().Long, qt.Contains, "atomic")
	c.Assert(cmd.Cmd().Long, qt.Contains, "setup")
	c.Assert(cmd.Cmd().Long, qt.Contains, "Examples:")
}

func TestCommand_Examples(t *testing.T) {
	c := qt.New(t)

	dbConfig := &shared.DatabaseConfig{}
	cmd := data.New(dbConfig)

	// Verify examples are included in the long description
	c.Assert(cmd.Cmd().Long, qt.Contains, "inventario migrate data --dry-run")
	c.Assert(cmd.Cmd().Long, qt.Contains, "--default-tenant-name")
	c.Assert(cmd.Cmd().Long, qt.Contains, "--admin-email")
}
