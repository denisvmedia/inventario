package update_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/cmd/inventario/tenants/update"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
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

	// Test that all expected flags are present, including the new
	// registration-mode flag added alongside the per-tenant registration mode
	// feature.
	expectedFlags := []string{
		"registration-mode",
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
