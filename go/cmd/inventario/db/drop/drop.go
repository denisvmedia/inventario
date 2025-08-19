package drop

import (
	"context"
	"fmt"
	"testing/fstest"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/schema/migrations/migrator"
)

type Command struct {
	command.Base

	config Config
}

func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "drop",
		Short: "Drop all database tables and data",
		Long: `Drop all database tables, indexes, constraints, and data.

This command completely cleans the database by dropping all tables.
Unlike 'reset', this command does NOT recreate the schema afterward.

WARNING: This operation will DELETE ALL DATA and SCHEMA in the database!
Always backup your database before running this command in production.

Examples:
  inventario migrate drop                      # Drop all tables (with confirmation)
  inventario migrate drop --confirm            # Drop without confirmation prompt
  inventario migrate drop --dry-run            # Preview what would be dropped`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.migrateDrop(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("migrate", &c.config)
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)
	c.Cmd().Flags().StringVar(&c.config.MigrationsDir, "migrations-dir", c.config.MigrationsDir, "Directory containing migration files")
	c.Cmd().Flags().Lookup("migrations-dir").Hidden = true
	c.Cmd().Flags().BoolVar(&c.config.Confirm, "confirm", c.config.Confirm, "Skip confirmation prompt (dangerous!)")
}

// migrateDrop handles the migrate drop subcommand
func (c *Command) migrateDrop(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	dryRun := cfg.DryRun
	confirm := cfg.Confirm
	dsn := dbConfig.DBDSN

	migr := migrator.New(dsn, fstest.MapFS{})

	fmt.Println("=== MIGRATE DROP ===")
	fmt.Printf("Database: %s\n", dsn)
	fmt.Println()

	return migr.DropDatabase(context.Background(), dryRun, confirm)
}
