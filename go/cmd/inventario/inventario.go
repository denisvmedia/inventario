package main

import (
	"os"

	"github.com/spf13/cobra"

	"go.5x5.cz/inventario/cmd/common/version"
	"go.5x5.cz/inventario/cmd/internal/command"
	"go.5x5.cz/inventario/cmd/inventario/admin"
	"go.5x5.cz/inventario/cmd/inventario/backfill"
	"go.5x5.cz/inventario/cmd/inventario/backoffice"
	"go.5x5.cz/inventario/cmd/inventario/backup"
	"go.5x5.cz/inventario/cmd/inventario/db"
	"go.5x5.cz/inventario/cmd/inventario/features"
	"go.5x5.cz/inventario/cmd/inventario/initconfig"
	"go.5x5.cz/inventario/cmd/inventario/run"
	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/cmd/inventario/tenants"
	"go.5x5.cz/inventario/cmd/inventario/users"
	"go.5x5.cz/inventario/cmd/inventario/workers"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
	var dbConfig shared.DatabaseConfig

	var rootCmd = &cobra.Command{
		Use:   "inventario",
		Short: "Inventario application",
		Long: `Inventario is a comprehensive personal inventory management application
designed to help you organize, track, and manage your personal belongings.

The application provides a web-based interface for managing your inventory items,
including their locations, categories, and other metadata. It supports multiple
database backends and provides both CLI and web interfaces.

FEATURES:
  • Web-based inventory management interface
  • Support for multiple database backends (PostgreSQL, in-memory)
  • File upload and attachment management
  • Database migration and seeding capabilities
  • RESTful API with JSON responses
  • User and tenant management commands

TODO: complete command docs

Use "inventario [command] --help" for detailed information about each command.`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	// Register database flags for commands that need them
	shared.RegisterDatabaseFlags(rootCmd, &dbConfig)

	rootCmd.SetArgs(args)
	rootCmd.AddCommand(initconfig.New().Cmd())
	rootCmd.AddCommand(db.New())
	rootCmd.AddCommand(run.New().Cmd())
	rootCmd.AddCommand(features.New())
	rootCmd.AddCommand(tenants.New(&dbConfig))
	rootCmd.AddCommand(users.New(&dbConfig))
	rootCmd.AddCommand(admin.New(&dbConfig))
	// Top-level `workers` group (#1308) — soft-pause/resume/status of the
	// background workers. Distinct from `run workers` (which starts the
	// worker process): this group MUTATES the shared pause state of an
	// already-running deployment.
	rootCmd.AddCommand(workers.New(&dbConfig))
	rootCmd.AddCommand(backfill.New(&dbConfig))
	rootCmd.AddCommand(backup.New())
	rootCmd.AddCommand(backoffice.New(&dbConfig))
	rootCmd.AddCommand(version.New())
	err := rootCmd.Execute()
	if err != nil {
		// Most failures exit 1. A command that wants a wrapper script to be
		// able to tell its failure apart — `db migrate up` on a dirty
		// revision, which no amount of retrying will clear (#2416) — returns
		// an error carrying its own status. See command.ExitCoder.
		os.Exit(command.ExitCodeFor(err)) //revive:disable-line:deep-exit
	}
}
