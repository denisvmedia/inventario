// Package blobs implements `inventario backfill blobs`, the one-shot
// data migration that rewrites legacy flat blob keys into the
// per-tenant namespace introduced by issue #1793.
//
// The command is idempotent and copy-first — see the
// services/blobbackfill package for the row-by-row semantics. Re-runs
// after a successful pass are no-ops; re-runs after a partial pass
// pick up where the previous one left off.
package blobs

import (
	"errors"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services/blobbackfill"
)

// Config carries the backfill command's flags.
type Config struct {
	UploadLocation string `yaml:"upload_location" env:"UPLOAD_LOCATION"`
	DryRun         bool   `yaml:"dry_run" env:"DRY_RUN"`
}

// Command is the `backfill blobs` cobra wrapper.
type Command struct {
	command.Base

	config Config
}

// New constructs the command with the supplied database config.
func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}
	shared.TryReadSection("backfill.blobs", &c.config)

	c.Base = command.NewBase(&cobra.Command{
		Use:   "blobs",
		Short: "Backfill legacy blob keys into the per-tenant namespace (#1793)",
		Long: `Rewrites every FileEntity row whose OriginalPath is a legacy flat blob
key into the canonical t/<tenant>/files/<basename> shape, copying the
physical blob first, updating the row, then deleting the legacy blob.
Image rows also have their derived thumbnail keys migrated.

The operation is idempotent: rows whose OriginalPath is already
tenant-prefixed are skipped. A partial run (interrupted by SIGTERM,
network blip, etc.) is safe to re-run; the second pass picks up only
the rows the first one didn't reach.

Examples:
  inventario backfill blobs --upload-location file:///srv/uploads
  inventario backfill blobs --upload-location s3://my-bucket --dry-run`,
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			return c.run(&c.config, dbConfig)
		},
	})

	c.registerFlags()
	return c
}

func (c *Command) registerFlags() {
	c.Cmd().Flags().StringVar(&c.config.UploadLocation, "upload-location", c.config.UploadLocation,
		"Bucket URL the server is reading/writing blobs to (e.g. file:///srv/uploads)")
	c.Cmd().Flags().BoolVar(&c.config.DryRun, "dry-run", c.config.DryRun,
		"Report what would change without touching the bucket or the database")
}

func (c *Command) run(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	out := c.Cmd().OutOrStdout()

	if err := dbConfig.Validate(); err != nil {
		return errxtrace.Wrap("database configuration error", err)
	}
	if strings.HasPrefix(dbConfig.DBDSN, "memory://") {
		return errors.New("backfill blobs requires a persistent database (memory:// is not supported)")
	}
	if strings.TrimSpace(cfg.UploadLocation) == "" {
		return errors.New("--upload-location is required")
	}

	registryFunc, ok := registry.GetRegistry(dbConfig.DBDSN)
	if !ok {
		// Don't echo dbConfig.DBDSN — Postgres DSNs embed credentials
		// in the URL, and the error message ends up in CLI output / logs.
		return errors.New("unsupported database type in DSN")
	}
	factorySet, err := registryFunc(registry.Config(dbConfig.DBDSN))
	if err != nil {
		return errxtrace.Wrap("failed to create registry factory set", err)
	}

	svc := blobbackfill.New(factorySet, cfg.UploadLocation)
	stats, err := svc.Run(c.Cmd().Context(), blobbackfill.Options{DryRun: cfg.DryRun})
	if err != nil {
		return errxtrace.Wrap("backfill aborted", err)
	}

	if cfg.DryRun {
		fmt.Fprintln(out, "[dry-run] no changes were written to the bucket or database.")
	}
	fmt.Fprintf(out, "Scanned:        %d\n", stats.RowsScanned)
	fmt.Fprintf(out, "Already-moved:  %d\n", stats.RowsAlreadyMoved)
	fmt.Fprintf(out, "Moved:          %d\n", stats.RowsMoved)
	fmt.Fprintf(out, "No blob key:    %d\n", stats.RowsSkippedNoKey)
	fmt.Fprintf(out, "Errored:        %d\n", stats.RowsErrored)
	fmt.Fprintf(out, "Thumbs moved:   %d\n", stats.ThumbsMoved)
	fmt.Fprintf(out, "Thumbs missing: %d\n", stats.ThumbsMissing)
	fmt.Fprintf(out, "Blobs copied:   %d\n", stats.BlobsCopied)
	fmt.Fprintf(out, "Blobs deleted:  %d\n", stats.BlobsDeleted)

	if stats.RowsErrored > 0 {
		return fmt.Errorf("backfill completed with %d row error(s); see logs", stats.RowsErrored)
	}
	return nil
}
