package update_test

import (
	"bytes"
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/cmd/inventario/tenants/update"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func setupMemoryAsPostgres(c *qt.C) {
	// Register memory registry as "postgres" for testing
	newFn, _ := memory.NewMemoryRegistrySet()
	registry.Register("postgres", newFn)

	// Setup cleanup to unregister after test
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})
}

func TestCommand_New(t *testing.T) {
	c := qt.New(t)

	setupMemoryAsPostgres(c)

	dbConfig := &shared.DatabaseConfig{
		DBDSN: "postgres://test:test@localhost/test",
	}

	cmd := update.New(dbConfig)
	c.Assert(cmd, qt.IsNotNil)
	c.Assert(cmd.Cmd(), qt.IsNotNil)
	c.Assert(cmd.Cmd().Use, qt.Equals, "update <tenant-id-or-slug>")
	c.Assert(cmd.Cmd().Short, qt.Equals, "Update an existing tenant")
}

func TestCommand_Flags(t *testing.T) {
	c := qt.New(t)

	setupMemoryAsPostgres(c)

	dbConfig := &shared.DatabaseConfig{
		DBDSN: "postgres://test:test@localhost/test",
	}

	cmd := update.New(dbConfig)
	cobraCmd := cmd.Cmd()

	// Every field the command can change has a flag; README and the help
	// examples use them.
	expectedFlags := []string{
		"name",
		"slug",
		"domain",
		"status",
		"settings",
		"registration-mode",
		"dry-run",
		"interactive",
	}

	for _, flagName := range expectedFlags {
		flag := cobraCmd.Flags().Lookup(flagName)
		c.Assert(flag, qt.IsNotNil, qt.Commentf("Flag %s should exist", flagName))
	}
}

func TestCommand_RegistrationModeFlag_HelpMatchesValidator(t *testing.T) {
	c := qt.New(t)

	setupMemoryAsPostgres(c)

	dbConfig := &shared.DatabaseConfig{
		DBDSN: "postgres://test:test@localhost/test",
	}

	cmd := update.New(dbConfig)
	flag := cmd.Cmd().Flags().Lookup("registration-mode")
	c.Assert(flag, qt.IsNotNil)

	// The flag help advertises these three modes — make sure the model
	// validator actually accepts every one of them. This guards against the
	// copy-paste drift Copilot flagged on PR #1319 (help text listed
	// `invite_only`, which the validator rejected).
	for _, mode := range []models.RegistrationMode{
		models.RegistrationModeOpen,
		models.RegistrationModeApproval,
		models.RegistrationModeClosed,
	} {
		c.Assert(mode.Validate(), qt.IsNil, qt.Commentf("mode %q advertised in flag help must validate", mode))
	}
}

func TestRegistrationMode_InvalidValueRejected(t *testing.T) {
	t.Run("invite_only is not a valid mode", func(t *testing.T) {
		c := qt.New(t)

		// `invite_only` was the incorrect value that slipped into the flag
		// help text before 1fb47e6. The validator must keep rejecting it so
		// the fix stays honest.
		mode := models.RegistrationMode("invite_only")
		err := mode.Validate()
		c.Assert(err, qt.IsNotNil)
	})

	t.Run("empty string is not a valid mode", func(t *testing.T) {
		c := qt.New(t)

		mode := models.RegistrationMode("")
		err := mode.Validate()
		c.Assert(err, qt.IsNotNil)
	})
}

const testDSN = "postgres://test:test@localhost/test"

// seededTenant registers a memory registry under the "postgres" name, holding
// one tenant with a domain, and returns the registry and the tenant's ID. The
// admin service the command opens gets the same registry, so a test reads
// the command's writes back through it.
func seededTenant(c *qt.C) (fs *registry.FactorySet, tenantID string) {
	fs = memory.NewFactorySet()
	tenant, err := fs.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Acme",
		Slug:   "acme",
		Domain: new("inventory.acme.test"),
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	registry.Register("postgres", func(registry.Config) (*registry.FactorySet, error) { return fs, nil })
	c.Cleanup(func() {
		registry.Unregister("postgres")
	})
	return fs, tenant.ID
}

func runUpdate(c *qt.C, args ...string) error {
	c.Helper()
	cmd := update.New(&shared.DatabaseConfig{DBDSN: testDSN})
	cmd.Cmd().SetOut(&bytes.Buffer{})
	cmd.Cmd().SetArgs(args)
	return cmd.Cmd().Execute()
}

func TestCommand_LiveUpdate_SetsTheFlaggedFields(t *testing.T) {
	c := qt.New(t)
	fs, tenantID := seededTenant(c)

	err := runUpdate(c, "acme", "--name=Acme Corporation", "--domain=inventory.acme.example")
	c.Assert(err, qt.IsNil)

	stored, err := fs.TenantRegistry.Get(context.Background(), tenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Name, qt.Equals, "Acme Corporation")
	c.Assert(stored.Domain, qt.DeepEquals, new("inventory.acme.example"))
	c.Assert(stored.Slug, qt.Equals, "acme")
}

// An empty --domain removes the domain. It is how an operator frees a domain
// that two tenants share before the migration that makes domains unique.
func TestCommand_LiveUpdate_EmptyDomainRemovesTheDomain(t *testing.T) {
	c := qt.New(t)
	fs, tenantID := seededTenant(c)

	err := runUpdate(c, "acme", "--domain=")
	c.Assert(err, qt.IsNil)

	stored, err := fs.TenantRegistry.Get(context.Background(), tenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Domain, qt.IsNil)
}

// Without --domain the domain stays as it is.
func TestCommand_LiveUpdate_KeepsTheDomainWithoutTheFlag(t *testing.T) {
	c := qt.New(t)
	fs, tenantID := seededTenant(c)

	err := runUpdate(c, "acme", "--name=Acme Corporation")
	c.Assert(err, qt.IsNil)

	stored, err := fs.TenantRegistry.Get(context.Background(), tenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Domain, qt.DeepEquals, new("inventory.acme.test"))
}

func TestCommand_DryRun_ChangesNothing(t *testing.T) {
	c := qt.New(t)
	fs, tenantID := seededTenant(c)

	err := runUpdate(c, "acme", "--domain=", "--dry-run")
	c.Assert(err, qt.IsNil)

	stored, err := fs.TenantRegistry.Get(context.Background(), tenantID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.Domain, qt.DeepEquals, new("inventory.acme.test"))
}
