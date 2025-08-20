package up

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
		Use:   "up",
		Short: "Apply all pending migrations",
		Long: `Apply all pending database migrations to bring the schema up to date.

Each migration runs in its own transaction, so if any migration fails,
it will be rolled back and the migration process will stop.

Examples:
  inventario migrate up                    # Apply all pending migrations
  inventario migrate up --dry-run          # Preview what would be applied`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.migrateUp(&c.config, dbConfig)
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
}

// migrateUp handles the migrate up subcommand
func (c *Command) migrateUp(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	dryRun := cfg.DryRun
	dsn := dbConfig.DBDSN

	migr := migrator.NewWithFallback(dsn, c.config.MigrationsDir)

	fmt.Println("=== MIGRATE UP ===")
	fmt.Printf("Database: %s\n", dsn)
	fmt.Println()

	migratorArgs := migrator.Args{
		DryRun: dryRun,
	}
	return migr.MigrateUp(context.Background(), migratorArgs)
}
