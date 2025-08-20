package list

import (
	"fmt"
	"io/fs"

	"github.com/go-extras/go-kit/must"
	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/schema/migrations"
)

type Command struct {
	command.Base

	config Config
}

func New(dbConfig *shared.DatabaseConfig) *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
		Use:   "list",
		Short: "List all available *.sql migration files",
		Long: `List all available *.sql migration files in the migrations directory.

This command displays all *.sql migration files that are available,
showing their filenames in sorted order. This is useful for understanding
what migrations are available and their naming convention.

Examples:
  inventario migrate list                  # List all *.sql migration files`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.listMigrations(&c.config, dbConfig)
		},
	})

	c.registerFlags()

	return c
}

func (c *Command) registerFlags() {
	shared.TryReadSection("migrate", &c.config)
	c.Cmd().Flags().StringVar(&c.config.MigrationsDir, "migrations-dir", c.config.MigrationsDir, "Directory containing migration files")
	c.Cmd().Flags().Lookup("migrations-dir").Hidden = true
}

// listMigrations handles the migrate list subcommand
func (c *Command) listMigrations(cfg *Config, dbConfig *shared.DatabaseConfig) error {
	var migFS fs.FS
	if migrations.HasEmbeddedMigrations() {
		migFS = must.Must(migrations.EmbeddedMigrationsFS())
	} else {
		migFS = migrations.MigrationsFS(c.config.MigrationsDir)
	}

	fmt.Println("=== MIGRATION FILES ===")
	if migrations.HasEmbeddedMigrations() {
		fmt.Println("Source: Embedded migrations")
	} else {
		fmt.Printf("Source: %s\n", c.config.MigrationsDir)
	}
	fmt.Println()

	files, err := migrations.ListMigrations(migFS)
	if err != nil {
		return fmt.Errorf("failed to list migration files: %w", err)
	}

	if len(files) == 0 {
		fmt.Println("No *.sql migration files found.")
		return nil
	}

	fmt.Printf("Found %d *.sql migration files:\n", len(files))
	for i, file := range files {
		fmt.Printf("  %d. %s\n", i+1, file)
	}

	return nil
}
