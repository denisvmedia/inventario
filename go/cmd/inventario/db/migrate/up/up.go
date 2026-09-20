package up

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/cmd/internal/command"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/schema/migrations/migrator"
)

// ExitCodeDirtyMigration is returned when a previous attempt left a migration
// half-applied. It is distinct from the generic failure status so a retry
// wrapper can stop instead of exhausting its budget on a decided outcome.
const ExitCodeDirtyMigration = 3

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
	fmt.Printf("Database: %s\n", shared.RedactDSN(dsn))
	fmt.Println()

	migratorArgs := migrator.Args{
		DryRun: dryRun,
	}
	err := migr.MigrateUp(context.Background(), migratorArgs)
	if errors.Is(err, migrator.ErrDirtyMigration) {
		// Hand the caller a status it can branch on. Deploy wrappers retry
		// `migrate up` because most failures are transient — an unreachable
		// database, a lock held by a concurrent rollout — but a dirty
		// revision is decided, and retrying it only delays the point at
		// which someone looks at the logs (#2416).
		return command.WithExitCode(err, ExitCodeDirtyMigration)
	}
	return err
}
