// Package grant implements `inventario admin grant-system-admin`.
package grant

import (
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services/admin"
)

// Config carries the grant-system-admin command's flags.
type Config struct {
	Email string `yaml:"email" env:"EMAIL"`
}

// Command is the `admin grant-system-admin` cobra wrapper.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("admin.grant_system_admin", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "grant-system-admin",
		Short: "Grant platform-wide system-admin to a user",
		Long: `Grant the system-admin flag to the user identified by --email.

System administrators have cross-tenant administrative privileges
(access to /api/v1/admin/*). They are distinct from per-group admins
— grant-system-admin does NOT add the user to any group.

The operation is idempotent: granting to an already-admin user prints
"already a system admin" and exits 0.

Examples:
  inventario admin grant-system-admin --email admin@acme.com`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	// `--email` is "required" but enforced inside run() (with a friendlier
	// error than cobra's default) so the same shape is reached whether the
	// command is invoked through the root binary or directly from tests.
	c.Cmd().Flags().StringVar(&c.config.Email, "email", c.config.Email, "Email of the user to grant system-admin (required)")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return errors.New("admin commands are not supported for memory databases; use PostgreSQL")
	}
	email := strings.TrimSpace(cfg.Email)
	if email == "" {
		return errors.New("--email is required")
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

	user, hadFlag, err := adminService.GrantSystemAdmin(c.Cmd().Context(), email)
	if err != nil {
		return errxtrace.Wrap("failed to grant system-admin", err)
	}

	if hadFlag {
		fmt.Fprintf(out, "ℹ️  %s (%s) is already a system administrator.\n", user.Name, user.Email)
		return nil
	}
	fmt.Fprintf(out, "✅ Granted system-admin to %s (%s).\n", user.Name, user.Email)
	return nil
}
