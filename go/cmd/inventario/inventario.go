package inventario

import (
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denisvmedia/inventario/cmd/inventario/migrate"
	"github.com/denisvmedia/inventario/cmd/inventario/run"
	"github.com/denisvmedia/inventario/cmd/inventario/seed"
)

const (
	envPrefix = "INVENTARIO"
)

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

COMMON WORKFLOWS:
  1. First-time setup:
     inventario migrate --db-dsn="postgres://user:pass@localhost/inventario"
     inventario seed --db-dsn="postgres://user:pass@localhost/inventario"
     inventario run --db-dsn="postgres://user:pass@localhost/inventario"

  2. Development with in-memory database:
     inventario run  # Uses memory:// database by default

  3. Production deployment:
     inventario migrate --db-dsn="postgres://user:pass@localhost/inventario"
     inventario run --addr=":8080" --db-dsn="postgres://user:pass@localhost/inventario"

DATABASE SUPPORT:
  • PostgreSQL: postgres://user:password@host:port/database
  • SQLite/BoltDB: boltdb://path/to/database.db
  • In-memory: memory:// (for testing and development)

Use "inventario [command] --help" for detailed information about each command.`,
	Args:  cobra.NoArgs, // Disallow unknown subcommands
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envPrefix)

	rootCmd.SetArgs(args)
	rootCmd.AddCommand(run.NewRunCommand())
	rootCmd.AddCommand(seed.NewSeedCommand())
	rootCmd.AddCommand(migrate.NewMigrateCommand())
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1) //revive:disable-line:deep-exit
	}
}
