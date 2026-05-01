// Package filesbackfill exposes `inventario db migrate files-backfill` —
// the one-shot ops tool that copies legacy commodity-scoped images,
// invoices, and manuals rows into the unified `files` table introduced
// under epic #1397. The actual SQL lives in services/files_backfill so it
// can be shared with integration tests and any future automation.
package filesbackfill

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	_ "github.com/lib/pq" // PostgreSQL driver, matches `migrate data`
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/services/files_backfill"
)

// Command wraps the cobra entry point so it follows the rest of the
// `inventario migrate` family (data, up, down, list).
type Command struct {
	command.Base

	config Config
}

// New creates the `files-backfill` subcommand.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	shared.TryReadSection("migrate.files_backfill", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "files-backfill",
		Short: "Backfill legacy images/invoices/manuals into the unified files table",
		Long: `Copy every row from the legacy commodity-scoped tables (images, invoices,
manuals) into the unified ` + "`files`" + ` table with proper category, type, and
linked_entity metadata. The legacy tables are not touched — the cutover that
removes them is a separate, later migration (#1421).

Idempotency: rows whose uuid already exists in ` + "`files`" + ` are skipped, so
re-running after a partial failure or after the FE has resumed uploading
directly into ` + "`files`" + ` is safe and produces zero new rows.

The whole run executes inside a single transaction. With --dry-run the
transaction is rolled back at the end, so the audit row counts reflect what
would happen with zero side effects.

Examples:
  # Preview what would be migrated
  inventario db migrate files-backfill --dry-run

  # Perform the actual backfill
  inventario db migrate files-backfill`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.RegisterDryRunFlag(c.Cmd(), &c.config.DryRun)
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	dsn := dbConfig.DBDSN
	if dsn == "" {
		return fmt.Errorf("database DSN is required")
	}

	fmt.Println("=== FILES BACKFILL ===")
	fmt.Printf("Database: %s\n", redactDSN(dsn))
	if cfg.DryRun {
		fmt.Println("Mode: DRY RUN (transaction will be rolled back)")
	} else {
		fmt.Println("Mode: LIVE")
	}
	fmt.Println()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	mgr := files_backfill.NewManager(db)
	var stats *files_backfill.Stats
	if cfg.DryRun {
		stats, err = mgr.PreviewOnly(context.Background())
	} else {
		stats, err = mgr.Apply(context.Background())
	}
	if err != nil {
		return fmt.Errorf("backfill failed: %w", err)
	}

	printStats(stats)

	if cfg.DryRun {
		fmt.Println("\n💡 Re-run without --dry-run to apply.")
	} else {
		fmt.Printf("\n🎉 Backfill complete — %d rows inserted.\n", stats.TotalInserted())
	}
	return nil
}

// redactDSN replaces the password component of a postgres:// DSN with
// `***` so the audit banner doesn't leak credentials into terminal
// scrollback, CI logs, or operator runbooks. We rebuild via direct
// string replacement rather than `u.User = url.UserPassword(..., "***")`
// because `u.String()` percent-encodes the asterisks, defeating the
// readability the redaction is supposed to provide. Returns the
// original string unchanged for non-URL DSNs (memory://, malformed
// input) so we don't silently drop information that might help
// debugging.
func redactDSN(dsn string) string {
	u, err := url.Parse(dsn)
	if err != nil || u.User == nil {
		return dsn
	}
	pw, ok := u.User.Password()
	if !ok || pw == "" {
		return dsn
	}
	return strings.Replace(dsn, ":"+pw+"@", ":***@", 1)
}

func printStats(stats *files_backfill.Stats) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "Source\tTotal\tMigrated\tPending\tInserted")
	fmt.Fprintln(w, "------\t-----\t--------\t-------\t--------")
	for _, src := range stats.Sources {
		fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\n", src.Source, src.Total, src.Migrated, src.Pending, src.Inserted)
	}
	_ = w.Flush()
}
