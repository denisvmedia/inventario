package inventario

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/denisvmedia/inventario/cmd/inventario/initconfig"
	"github.com/denisvmedia/inventario/cmd/inventario/migrate"
	"github.com/denisvmedia/inventario/cmd/inventario/run"
	"github.com/denisvmedia/inventario/cmd/inventario/seed"
	"github.com/denisvmedia/inventario/internal/version"
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
     inventario init-config  # Create configuration file (optional)
     inventario migrate --db-dsn="postgres://user:pass@localhost/inventario"
     inventario seed --db-dsn="postgres://user:pass@localhost/inventario"
     inventario run --db-dsn="postgres://user:pass@localhost/inventario"

  2. Development with in-memory database:
     inventario run  # Uses memory:// database by default

  3. Production deployment:
     inventario init-config  # Create configuration file
     inventario migrate --db-dsn="postgres://user:pass@localhost/inventario"
     inventario run --addr=":8080" --db-dsn="postgres://user:pass@localhost/inventario"

DATABASE SUPPORT:
  • PostgreSQL: postgres://user:password@host:port/database
  • SQLite/BoltDB: boltdb://path/to/database.db
  • In-memory: memory:// (for testing and development)

Use "inventario [command] --help" for detailed information about each command.`,
	Args: cobra.NoArgs, // Disallow unknown subcommands
	RunE: func(cmd *cobra.Command, _ []string) error {
		return cmd.Help()
	},
}

// loadGlobalConfigFile loads the configuration file for all commands
func loadGlobalConfigFile() {
	// Get the user's config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return // Not a fatal error
	}

	// Define the config file path
	configFilePath := filepath.Join(configDir, "inventario", "config.yaml")

	// Check if the config file exists
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		return // Not an error if the file doesn't exist
	}

	// Set the config file for viper
	viper.SetConfigFile(configFilePath)

	// Read the config file
	// No manual mapping needed - config keys match flag names exactly
	viper.ReadInConfig() // Ignore errors, not fatal
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute(args ...string) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envPrefix)

	// Set up environment variable key mappings
	// This allows INVENTARIO_DB_DSN to map to db-dsn flag name
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))

	// Environment variables are automatically bound via AutomaticEnv() and SetEnvPrefix()
	// This enables the priority order: CLI flags > env vars > config file > defaults

	// Load configuration file before processing commands
	loadGlobalConfigFile()

	rootCmd.SetArgs(args)
	rootCmd.AddCommand(initconfig.NewInitConfigCommand())
	rootCmd.AddCommand(migrate.NewMigrateCommand())
	rootCmd.AddCommand(run.NewRunCommand())
	rootCmd.AddCommand(seed.NewSeedCommand())
	rootCmd.AddCommand(newVersionCommand())
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1) //revive:disable-line:deep-exit
	}
}

// newVersionCommand creates a version command
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display the current version, build information, and platform details.",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println(version.String())
		},
	}
}
