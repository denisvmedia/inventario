// Package verify exposes the `inventario migrate verify` subcommand. It
// compares the binary's embedded migration files against the version row in
// schema_migrations and fails non-zero when the DB is behind — the symptom
// of the docker-compose stale-image bug documented in issue #1655.
package verify

import (
	"context"
	"errors"
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

// New constructs the `migrate verify` subcommand.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "verify",
		Short: "Verify the database schema is up-to-date with this binary",
		Long: `Compare the highest migration version embedded in this binary against
the version recorded in schema_migrations. Exits non-zero with a clear
diagnostic when the database is behind — the typical symptom of a
docker-compose stack whose migrate container shipped a stale image (#1655).

Examples:
  inventario migrate verify

Exit codes:
  0  schema is up-to-date (or DB is ahead — warning logged)
  1  schema lags the binary's embedded migrations
  2  could not perform the check (DB unreachable, etc.)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.verify(dbConfig)
		},
	})

	c.registerFlags()
	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("migrate", &c.config)
	c.Cmd().Flags().StringVar(&c.config.MigrationsDir, "migrations-dir", c.config.MigrationsDir, "Directory containing migration files (fallback when no embedded migrations)")
	c.Cmd().Flags().Lookup("migrations-dir").Hidden = true
}

func (c *Command) verify(dbConfig *shared.DatabaseConfig) error {
	dsn := dbConfig.DBDSN
	m := migrator.NewWithFallback(dsn, c.config.MigrationsDir)

	fmt.Println("=== MIGRATE VERIFY ===")
	if err := m.VerifySchemaUpToDate(context.Background()); err != nil {
		if errors.Is(err, migrator.ErrSchemaLagsBinary) {
			// Caller scripts in CI care about this specific exit code.
			return err
		}
		return fmt.Errorf("verify failed: %w", err)
	}

	fmt.Println("Schema is up-to-date with the binary's embedded migrations.")
	return nil
}
