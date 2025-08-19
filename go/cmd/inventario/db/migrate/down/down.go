package down

import (
	"context"
	"fmt"
	"strconv"

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
		Use:   "down [target-version]",
		Short: "Rollback migrations to a specific version",
		Long: `Rollback database migrations to a specific version.

WARNING: Down migrations can cause data loss! Always backup your database
before running down migrations in production.

Examples:
  inventario migrate down 5                # Rollback to version 5
  inventario migrate down 5 --dry-run      # Preview rollback to version 5
  inventario migrate down 5 --confirm      # Skip confirmation prompt`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			targetVersionStr := args[0]
			targetVersion, err := strconv.Atoi(targetVersionStr)
			if err != nil {
				return fmt.Errorf("invalid target version: %s", targetVersionStr)
			}
			return c.migrateDown(&c.config, dbConfig, targetVersion)
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

// migrateDown handles the migrate down subcommand
func (c *Command) migrateDown(cfg *Config, dbConfig *shared.DatabaseConfig, targetVersion int) error {
	dryRun := cfg.DryRun
	confirm := cfg.Confirm
	dsn := dbConfig.DBDSN

	migr := migrator.NewWithFallback(dsn, c.config.MigrationsDir)

	fmt.Println("=== MIGRATE DOWN ===")
	fmt.Printf("Database: %s\n", dsn)
	fmt.Printf("Target version: %d\n", targetVersion)
	fmt.Println()

	return migr.MigrateDown(context.Background(), targetVersion, dryRun, confirm)
}
