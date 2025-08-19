package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/common/version"
	"github.com/denisvmedia/inventario/cmd/inventario/db"
	"github.com/denisvmedia/inventario/cmd/inventario/features"
	"github.com/denisvmedia/inventario/cmd/inventario/initconfig"
	"github.com/denisvmedia/inventario/cmd/inventario/run"
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
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
  • Support for multiple database backends (PostgreSQL, SQLite, BoltDB, in-memory)
  • File upload and attachment management
  • Database migration and seeding capabilities
  • RESTful API with JSON responses

TODO: complete command docs

Use "inventario [command] --help" for detailed information about each command.`,
		Args: cobra.NoArgs, // Disallow unknown subcommands
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	rootCmd.SetArgs(args)
	rootCmd.AddCommand(initconfig.New().Cmd())
	rootCmd.AddCommand(db.New())
	rootCmd.AddCommand(run.New().Cmd())
	rootCmd.AddCommand(features.New())
	rootCmd.AddCommand(version.New())
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1) //revive:disable-line:deep-exit
	}
}
