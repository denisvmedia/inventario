package initconfig

import (
	"bytes"
	_ "embed" // Embed the config template file
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/spf13/cobra"

	"github.com/denisvmedia/inventario/cmd/internal/command"
	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/internal/defaults"
)

//go:embed data/config.yaml.tmpl
var configTemplate string

type Command struct {
	command.Base
}

func New() *Command {
	c := &Command{}

	c.Base = command.NewBase(&cobra.Command{
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return c.initConfigCommand()
		},
	})

	return c
}

// generateSampleConfig creates the sample configuration content using shared defaults
func (c *Command) generateSampleConfig() (string, error) {
	cfg := defaults.New()

	tmpl, err := template.New("config").Parse(configTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse config template: %w", err)
	}

	var buf bytes.Buffer
	err = tmpl.Execute(&buf, cfg)
	if err != nil {
		return "", fmt.Errorf("failed to execute config template: %w", err)
	}

	return buf.String(), nil
}

// initConfigCommand handles the init-config command
func (c *Command) initConfigCommand() error {
	configFilePath := shared.GetConfigFile()
	configFileDir := filepath.Dir(configFilePath)

	// Create the inventario subdirectory
	err := os.MkdirAll(configFileDir, 0o755)
	if err != nil {
		slog.Error("Failed to create config directory", "dir", configFileDir, "error", err)
		return fmt.Errorf("failed to create config directory %s: %w", configFileDir, err)
	}

	// Check if the config file already exists (do not overwrite)
	_, err = os.Stat(configFilePath)
	switch {
	case err == nil:
		slog.Error("Configuration file already exists", "path", configFilePath)
		return fmt.Errorf("configuration file already exists at %s", configFilePath)
	case !os.IsNotExist(err):
		slog.Error("Failed to check config file existence", "path", configFilePath, "error", err)
		return fmt.Errorf("failed to check config file existence: %w", err)
	}

	// Generate the sample config content using shared defaults
	sampleContent, err := c.generateSampleConfig()
	if err != nil {
		slog.Error("Failed to generate sample config", "error", err)
		return fmt.Errorf("failed to generate sample config: %w", err)
	}

	// Write the sample config file
	err = os.WriteFile(configFilePath, []byte(sampleContent), 0o600)
	if err != nil {
		slog.Error("Failed to write config file", "path", configFilePath, "error", err)
		return fmt.Errorf("failed to write config file %s: %w", configFilePath, err)
	}

	// log.WithField("path", configFilePath).Info("Configuration file created successfully")
	fmt.Printf("✅ Configuration file created successfully at: %s\n", configFilePath)
	fmt.Println("\nYou can now edit this file to customize your Inventario settings.")
	fmt.Println("Environment variables and command-line flags will override these settings.")

	return nil
}
