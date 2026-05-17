// Package revoke implements `inventario admin revoke-system-admin`.
package revoke

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services/admin"
)

// Config carries the revoke-system-admin command's flags.
type Config struct {
	Email     string `yaml:"email" env:"EMAIL"`
	AllowZero bool   `yaml:"allow_zero" env:"ALLOW_ZERO" env-default:"false"`
}

// Command is the `admin revoke-system-admin` cobra wrapper.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("admin.revoke_system_admin", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "revoke-system-admin",
		Short: "Revoke platform-wide system-admin from a user",
		Long: `Revoke the system-admin flag from the user identified by --email.

By default the command refuses to revoke the last remaining system
administrator — otherwise an operator could lock themselves out of every
admin surface. Pass --allow-zero to override the guard (for deliberate
platform-shutdown scenarios).

The operation is idempotent: revoking from a non-admin user prints
"is not a system administrator" and exits 0.

Examples:
  inventario admin revoke-system-admin --email admin@acme.com

  # Override the last-admin guard:
  inventario admin revoke-system-admin --email admin@acme.com --allow-zero`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// `--email` is "required" but enforced inside run() so the same shape
	// is reached whether the command is invoked through the root binary or
	// directly from tests.
	c.Cmd().Flags().StringVar(&c.config.Email, "email", c.config.Email, "Email of the user to revoke system-admin from (required)")
	c.Cmd().Flags().BoolVar(&c.config.AllowZero, "allow-zero", c.config.AllowZero, "Allow revoking the last remaining system admin (default false)")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("database configuration error: %w", err)
	}
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return fmt.Errorf("admin commands are not supported for memory databases; use PostgreSQL")
	}
	if strings.TrimSpace(cfg.Email) == "" {
		return fmt.Errorf("--email is required")
	}

	adminService, err := admin.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := adminService.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close admin service: %v\n", closeErr)
		}
	}()

	user, hadFlag, err := adminService.RevokeSystemAdmin(context.Background(), cfg.Email, cfg.AllowZero)
	switch {
	case errors.Is(err, admin.ErrLastSystemAdmin):
		// Friendly hint so the operator doesn't have to recall the override flag.
		fmt.Fprintf(out, "❌ Refusing to revoke the last system administrator. Re-run with --allow-zero to override.\n")
		return err
	case err != nil:
		return fmt.Errorf("failed to revoke system-admin: %w", err)
	}

	if !hadFlag {
		fmt.Fprintf(out, "ℹ️  %s (%s) is not a system administrator — nothing to revoke.\n", user.Name, user.Email)
		return nil
	}
	fmt.Fprintf(out, "✅ Revoked system-admin from %s (%s).\n", user.Name, user.Email)
	return nil
}
