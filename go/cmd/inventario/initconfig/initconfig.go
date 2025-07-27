package initconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/internal/defaults"
	"github.com/denisvmedia/inventario/internal/log"
)

var initConfigCmd = &cobra.Command{
	Use:   "init-config",
	Short: "Initialize a configuration file from a sample template",
	Long: `Initialize a configuration file creates a sample configuration file in the
standard user config directory. This file can be used to configure the Inventario
application with your preferred settings instead of using command-line flags.

The configuration file will be created at:
  • Linux/macOS: ~/.config/inventario/config.yaml
  • Windows: %APPDATA%/inventario/config.yaml

USAGE EXAMPLES:

  Create a new configuration file:
    inventario init-config

  The generated configuration file includes:
    • Server settings (bind address, upload location)
    • Database configuration (connection string)
    • Worker settings (concurrent export/import limits)

CONFIGURATION FILE FORMAT:

  The generated config.yaml file uses YAML format and includes comments
  explaining each setting. You can modify the values to match your
  environment and preferences.

  Example configuration sections:
    • server: Network and file upload settings
    • database: Connection and storage settings
    • workers: Background processing limits

PREREQUISITES:
  • Write permissions to the user config directory
  • The config directory will be created if it doesn't exist

NOTES:
  • If a configuration file already exists, this command will fail
  • Use environment variables or command-line flags to override config file settings
  • The configuration file is optional - all settings have sensible defaults
  • UI settings (theme, debug info) are configured through the web interface, not this file`,
	RunE: initConfigCommand,
}

// generateSampleConfig creates the sample configuration content using shared defaults
func generateSampleConfig() string {
	cfg := defaults.New()

	return fmt.Sprintf(`# Inventario Configuration File
# This file contains default settings for the Inventario application.
# You can modify these values to match your environment and preferences.
#
# Environment variables and command-line flags will override these settings.
# Environment variable format: INVENTARIO_<FLAG_NAME> (e.g., INVENTARIO_ADDR)

# Configuration keys match the command-line flag names exactly
# This eliminates the need for manual mapping between config and flags

# Network address and port where the server will listen
# Format: "[host]:port" (e.g., ":8080", "localhost:3333", "0.0.0.0:8080")
# Use ":0" to let the system choose an available port
addr: "%s"

# Location for uploaded files
# Supports local filesystem and cloud storage URLs
# Local examples:
#   - "file:///var/lib/inventario/uploads?create_dir=1"
#   - "file://./uploads?create_dir=1" (relative to working directory)
# The "create_dir=1" parameter creates the directory if it doesn't exist
upload-location: "%s"

# Database connection string supporting multiple backends:
# • PostgreSQL (recommended - full feature support): "postgres://user:password@host:port/database?sslmode=disable"
# • BoltDB (basic features only): "boltdb://path/to/database.db"
# • In-memory (testing only): "memory://" (data lost on restart, useful for testing)
#
# PostgreSQL provides advanced features like:
# - Full-text search with ranking
# - JSONB operators for complex queries
# - Advanced indexing (GIN, GiST, partial indexes)
# - Similarity search and aggregations
# Other databases use fallback implementations with reduced performance
db-dsn: "%s"

# Enable PostgreSQL-specific advanced features (ignored for other databases)
# When true, PostgreSQL will use full-text search, JSONB operators, and advanced indexing
# When false, PostgreSQL will use basic SQL queries similar to other databases
enable-advanced-features: true

# Fallback behavior when advanced features are not supported
# Options: "error", "warn", "silent"
# - "error": Return errors when advanced features are requested
# - "warn": Log warnings and use fallback implementations
# - "silent": Silently use fallback implementations
unsupported-feature-handling: "warn"

# Maximum number of concurrent export processes
# Higher values allow more exports to run simultaneously but use more resources
max-concurrent-exports: %d

# Maximum number of concurrent import processes
# Higher values allow more imports to run simultaneously but use more resources
max-concurrent-imports: %d

# NOTE: User interface settings (theme, debug info, date format) and system settings
# (main currency) are stored in the database and configured through the web interface
# at http://localhost:3333/settings - they are not configured through this file.
#
# The settings above correspond directly to the command-line flags supported by the
# 'inventario run' command. For a complete list of supported flags, run:
#   inventario run --help
`, cfg.Server.Addr, cfg.Server.UploadLocation, cfg.Database.DSN, cfg.Workers.MaxConcurrentExports, cfg.Workers.MaxConcurrentImports)
}

func NewInitConfigCommand() *cobra.Command {
	return initConfigCmd
}

func initConfigCommand(_ *cobra.Command, _ []string) error {
	// Get the user's config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		log.WithError(err).Error("Failed to get user config directory")
		return fmt.Errorf("failed to get user config directory: %w", err)
	}

	// Create the inventario subdirectory
	inventarioConfigDir := filepath.Join(configDir, "inventario")
	err = os.MkdirAll(inventarioConfigDir, 0o755)
	if err != nil {
		log.WithError(err).WithField("dir", inventarioConfigDir).Error("Failed to create config directory")
		return fmt.Errorf("failed to create config directory %s: %w", inventarioConfigDir, err)
	}

	// Define the config file path
	configFilePath := filepath.Join(inventarioConfigDir, "config.yaml")

	// Check if the config file already exists
	if _, err := os.Stat(configFilePath); err == nil {
		log.WithField("path", configFilePath).Error("Configuration file already exists")
		return fmt.Errorf("configuration file already exists at %s", configFilePath)
	}

	// Generate the sample config content using shared defaults
	sampleContent := generateSampleConfig()

	// Write the sample config file
	err = os.WriteFile(configFilePath, []byte(sampleContent), 0o600)
	if err != nil {
		log.WithError(err).WithField("path", configFilePath).Error("Failed to write config file")
		return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
	}

	// log.WithField("path", configFilePath).Info("Configuration file created successfully")
	fmt.Printf("✅ Configuration file created successfully at: %s\n", configFilePath)          //nolint:forbidigo // CLI output is OK
	fmt.Println("\nYou can now edit this file to customize your Inventario settings.")        //nolint:forbidigo // CLI output is OK
	fmt.Println("Environment variables and command-line flags will override these settings.") //nolint:forbidigo // CLI output is OK

	return nil
}
