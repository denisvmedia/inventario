package reset

import (
	"context"
	"fmt"

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
		Use:   "reset",
		Short: "Drop all tables and recreate from scratch",
		Long: `Drop all database tables and recreate the schema from scratch.

This command performs a complete database reset by:
1. Dropping all existing tables, indexes, and constraints
2. Applying all migrations from the beginning

WARNING: This operation will DELETE ALL DATA in the database!
Always backup your database before running this command in production.

Examples:
  inventario migrate reset                     # Reset database (with confirmation)
  inventario migrate reset --confirm           # Reset without confirmation prompt
  inventario migrate reset --dry-run           # Preview what would be reset`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.migrateReset(&c.config, dbConfig)
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

// migrateReset handles the migrate reset subcommand
func (c *Command) migrateReset(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	dryRun := cfg.DryRun
	confirm := cfg.Confirm
	dsn := dbConfig.DBDSN

	migr := migrator.NewWithFallback(dsn, c.config.MigrationsDir)

	fmt.Println("=== MIGRATE RESET ===")
	fmt.Printf("Database: %s\n", dsn)
	fmt.Println()

	migratorArgs := migrator.Args{
		DryRun: dryRun,
	}
	return migr.ResetDatabase(context.Background(), migratorArgs, confirm)
}
