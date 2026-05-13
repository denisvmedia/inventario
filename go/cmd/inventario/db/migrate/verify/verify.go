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

The check is bounded by --timeout (default 30s) so CI / orchestration
scripts get a deterministic failure on a slow or unreachable DB rather
than a hung command.

Examples:
  inventario migrate verify
  inventario migrate verify --timeout=5s`,
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
	c.Cmd().Flags().DurationVar(&c.config.Timeout, "timeout", c.config.Timeout, "Bound the DB round-trip so a hung connection can't stall CI/operator scripts")
}

func (c *Command) verify(dbConfig *shared.DatabaseConfig) error {
	dsn := dbConfig.DBDSN
	m := migrator.NewWithFallback(dsn, c.config.MigrationsDir)

	ctx, cancel := context.WithTimeout(context.Background(), c.config.Timeout)
	defer cancel()

	fmt.Println("=== MIGRATE VERIFY ===")
	if err := m.VerifySchemaUpToDate(ctx); err != nil {
		// All errors propagate up to root cobra, which os.Exit(1)s — see
		// cmd/inventario/inventario.go. Operator scripts that care about
		// "schema lagged vs DB unreachable" should grep the stderr message
		// or use errors.Is in a wrapping Go caller; we deliberately don't
		// promise distinct exit codes the harness can't actually emit.
		if errors.Is(err, migrator.ErrSchemaLagsBinary) {
			return err
		}
		return fmt.Errorf("verify failed: %w", err)
	}

	fmt.Println("Schema is up-to-date with the binary's embedded migrations.")
	return nil
}
