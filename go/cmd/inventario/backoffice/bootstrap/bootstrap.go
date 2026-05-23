// Package bootstrap implements `inventario backoffice bootstrap`.
//
// Stamps the first platform-operator identity into a fresh deployment
// (issue #1785 Phase 1). The command is idempotent on the email — a
// re-run with the same email prints "user already exists" and exits 0.
// A re-run with a different email refuses unless --force is passed, so
// an operator can't accidentally create a sprawl of back-office users.
package bootstrap

import (
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services/backoffice"
)

// Config carries the bootstrap command's flags.
type Config struct {
	Email    string `yaml:"email" env:"EMAIL"`
	Name     string `yaml:"name" env:"NAME"`
	Role     string `yaml:"role" env:"ROLE"`
	Password string `yaml:"password" env:"PASSWORD"`
	Force    bool   `yaml:"force" env:"FORCE"`
}

// Command is the cobra wrapper around `backoffice bootstrap`.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("backoffice.bootstrap", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "bootstrap",
		Short: "Create the first back-office (platform-operator) user",
		Long: `Stamp the first back-office user into a fresh deployment.

Back-office users live OUTSIDE the tenant model: they have no tenant_id,
no group membership, and no RLS scoping. They authenticate against the
back-office auth plane (added in Phase 2 of issue #1785).

IDEMPOTENCY:

  • Re-running with the same --email prints "user already exists" and
    exits 0 (safe to call from provisioning scripts).
  • Re-running with a different --email refuses unless --force is
    passed, so a deployment can't accidentally accumulate operators.

PASSWORD:

  • If --password is omitted, a strong random password is generated
    and printed ONCE to stdout. Copy it before the terminal scrolls —
    it is NOT stored anywhere and cannot be recovered.
  • If --password is supplied, it must meet the same complexity rules
    as models.ValidatePassword (8+ chars, upper, lower, digit).

ROLE:

  • Defaults to platform_admin (full mutation rights — the only sensible
    seed value).
  • support_agent is also accepted for non-first invocations under --force.

Examples:
  inventario backoffice bootstrap --email admin@example.com --name "Ops Admin"
  inventario backoffice bootstrap --email second@example.com --name "Second" --force
  inventario backoffice bootstrap --email a@example.com --name "A" --password 'Sup3rSecret!'`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	c.Cmd().Flags().StringVar(&c.config.Email, "email", c.config.Email, "Back-office user email (required)")
	c.Cmd().Flags().StringVar(&c.config.Name, "name", c.config.Name, "Back-office user display name (required)")
	c.Cmd().Flags().StringVar(&c.config.Role, "role", c.config.Role, "Back-office role: platform_admin (default) or support_agent")
	c.Cmd().Flags().StringVar(&c.config.Password, "password", c.config.Password, "Password (auto-generated if omitted)")
	c.Cmd().Flags().BoolVar(&c.config.Force, "force", c.config.Force, "Allow adding a second+ back-office user when one already exists")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	// dbConfig.Validate already rejects non-postgres DSNs at the
	// shared-validator layer (memory://, file:// etc all fail with
	// "only support PostgreSQL"). No second guard needed here.
	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}

	email := strings.TrimSpace(cfg.Email)
	if email == "" {
		return errors.New("--email is required")
	}
	name := strings.TrimSpace(cfg.Name)
	if name == "" {
		return errors.New("--name is required")
	}

	// Role defaults to platform_admin at the service layer; reject
	// invalid free-form values here so the CLI error message includes
	// the valid set without the service having to know about CLI
	// presentation.
	role := models.BackofficeRole(strings.TrimSpace(cfg.Role))
	if role == "" {
		role = models.BackofficeRolePlatformAdmin
	}
	if !role.IsValid() {
		return fmt.Errorf("--role must be one of: support_agent, platform_admin (got %q)", string(role))
	}

	svc, err := backoffice.NewService(dbConfig)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := svc.Close(); closeErr != nil {
			fmt.Fprintf(out, "Warning: failed to close backoffice service: %v\n", closeErr)
		}
	}()

	result, err := svc.Bootstrap(c.Cmd().Context(), backoffice.BootstrapRequest{
		Email:    email,
		Name:     name,
		Role:     role,
		Password: cfg.Password,
		Force:    cfg.Force,
	})
	if err != nil {
		return errxtrace.Wrap("failed to bootstrap backoffice user", err)
	}

	if result.AlreadyExisted {
		fmt.Fprintf(out, "ℹ️  Back-office user %s (%s) already exists; no changes made.\n",
			result.User.Name, result.User.Email)
		return nil
	}

	fmt.Fprintf(out, "✅ Created back-office user %s (%s) with role %s.\n",
		result.User.Name, result.User.Email, string(result.User.Role))
	if result.GeneratedPassword != "" {
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "🔑 Generated password (copy this NOW — it will not be shown again):")
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "    "+result.GeneratedPassword)
		fmt.Fprintln(out, "")
		fmt.Fprintln(out, "The operator should sign in with this password and rotate it on first login.")
	}
	return nil
}
