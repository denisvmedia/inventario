package apply

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/schema/bootstrap"
)

type Command struct {
	command.Base

	config Config
}

func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "apply",
		Short: "Apply bootstrap SQL migrations to the database",
		Long: `Apply all bootstrap SQL migrations to the specified PostgreSQL database.

Bootstrap migrations handle initial database setup that requires elevated privileges,
such as creating extensions, roles, and setting up default privileges. These migrations
must be run before regular Ptah migrations.

The migrations are idempotent and can be safely run multiple times. They require
a privileged database user (typically with SUPERUSER privileges) to execute
successfully.

Template Variables:
  {{.Username}}             - Operational database username (default: inventario)
  {{.UsernameForMigrations}} - Migration database username (defaults to Username)

Examples:
  inventario migrate bootstrap apply --db-dsn="postgres://admin:pass@localhost/inventario"
  inventario migrate bootstrap apply --username=myapp --db-dsn="postgres://admin:pass@localhost/inventario"
  inventario migrate bootstrap apply --username=myapp --username-for-migrations=migrator --db-dsn="postgres://admin:pass@localhost/inventario"
  inventario migrate bootstrap apply --dry-run --db-dsn="postgres://admin:pass@localhost/inventario"

IMPORTANT: This command requires a privileged database user with sufficient
permissions to create extensions and manage roles.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.bootstrapApply(dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("bootstrap", &c.config)
	shared.RegisterBootstrapFlags(c.Cmd(), &c.config.Username, &c.config.UsernameForMigrations)
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)
}

func (c *Command) bootstrapApply(dbConfig *shared.DatabaseConfig) error {
	if err := dbConfig.Validate(); err != nil {
		return err
	}

	dsn := dbConfig.DBDSN
	username := c.config.Username
	usernameForMigrations := c.config.UsernameForMigrations
	dryRun := c.config.DryRun

	// Default usernameForMigrations to username if not provided
	if usernameForMigrations == "" {
		usernameForMigrations = username
	}

	// Create bootstrap migrator
	migrator := bootstrap.New()

	// Prepare arguments
	args := bootstrap.ApplyArgs{
		DSN: dsn,
		Template: bootstrap.TemplateData{
			Username:              username,
			UsernameForMigrations: usernameForMigrations,
		},
		DryRun: dryRun,
	}

	// Apply bootstrap migrations
	ctx := context.Background()
	if err := migrator.Apply(ctx, args); err != nil {
		return fmt.Errorf("failed to apply bootstrap migrations: %w", err)
	}

	return nil
}
