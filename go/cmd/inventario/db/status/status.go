package status

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/schema/migrations/migrator"
)

type Command struct {
	command.Base

	config Config
}

// New creates the migrate status subcommand
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "status",
		Short: "Show current migration status",
		Long: `Display the current migration status including:
- Current database version
- Total number of migrations
- Number of pending migrations
- List of pending migrations (with --verbose)

Examples:
  inventario migrate status                # Show basic status
  inventario migrate status --verbose      # Show detailed status with pending migrations`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.migrateStatusCommand(dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("migrate", &c.config)
	c.Cmd().Flags().StringVar(&c.config.MigrationsDir, "migrations-dir", c.config.MigrationsDir, "Directory containing migration files")
	c.Cmd().Flags().Lookup("migrations-dir").Hidden = true
	c.Cmd().Flags().Bool("verbose", false, "Show detailed status information")
}

// migrateStatusCommand handles the migrate status subcommand
func (c *Command) migrateStatusCommand(dbConfig *shared.DatabaseConfig) error {
	verbose, _ := c.Cmd().Flags().GetBool("verbose")
	dsn := dbConfig.DBDSN

	migr := migrator.NewWithFallback(dsn, c.config.MigrationsDir)

	return migr.PrintMigrationStatus(context.Background(), verbose)
}
