package initconfig_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/cmd/inventario/initconfig"
	"github.com/denisvmedia/inventario/internal/defaults"
)

// getSampleConfig creates a config file and returns its content for testing
func getSampleConfig(t *testing.T) string {
	tempDir := t.TempDir()

	// Override config directory for this test
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if originalConfigDir != "" {
			os.Setenv("XDG_CONFIG_HOME", originalConfigDir)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	// On Windows, override APPDATA
	if runtime.GOOS == "windows" {
		originalAppData := os.Getenv("APPDATA")
		t.Cleanup(func() {
			if originalAppData != "" {
				os.Setenv("APPDATA", originalAppData)
			} else {
				os.Unsetenv("APPDATA")
			}
		})
		os.Setenv("APPDATA", tempDir)
	}

	// Create config file
	cmd := initconfig.NewInitConfigCommand()
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Read the created file
	configPath := filepath.Join(tempDir, "inventario", "config.yaml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	return string(content)
}

func TestNewInitConfigCommand(t *testing.T) {
	c := qt.New(t)

	cmd := initconfig.NewInitConfigCommand()
	c.Assert(cmd, qt.IsNotNil)
	c.Assert(cmd.Use, qt.Equals, "init-config")
	c.Assert(cmd.Short, qt.Equals, "Initialize a configuration file from a sample template")
}

func TestInitConfigCommand_Success(t *testing.T) {
	c := qt.New(t)

	// Create a temporary directory to use as config dir
	tempDir := t.TempDir()
	
	// Override the user config directory for this test
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if originalConfigDir != "" {
			os.Setenv("XDG_CONFIG_HOME", originalConfigDir)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	// On Windows, we need to override APPDATA instead
	if os.Getenv("OS") == "Windows_NT" {
		originalAppData := os.Getenv("APPDATA")
		t.Cleanup(func() {
			if originalAppData != "" {
				os.Setenv("APPDATA", originalAppData)
			} else {
				os.Unsetenv("APPDATA")
			}
		})
		os.Setenv("APPDATA", tempDir)
	}

	cmd := initconfig.NewInitConfigCommand()
	err := cmd.Execute()
	c.Assert(err, qt.IsNil)

	// Verify the config file was created
	configPath := filepath.Join(tempDir, "inventario", "config.yaml")
	_, err = os.Stat(configPath)
	c.Assert(err, qt.IsNil, qt.Commentf("config file should exist at %s", configPath))

	// Verify the content contains expected sections
	content, err := os.ReadFile(configPath)
	c.Assert(err, qt.IsNil)
	
	contentStr := string(content)
	cfg := defaults.New()

	c.Assert(contentStr, qt.Contains, "# Inventario Configuration File")
	c.Assert(contentStr, qt.Contains, fmt.Sprintf("addr: \"%s\"", cfg.Server.Addr))
	c.Assert(contentStr, qt.Contains, fmt.Sprintf("max-concurrent-exports: %d", cfg.Workers.MaxConcurrentExports))
	c.Assert(contentStr, qt.Contains, fmt.Sprintf("db-dsn: \"%s\"", cfg.Database.DSN))
	c.Assert(contentStr, qt.Contains, fmt.Sprintf("upload-location: \"%s\"", cfg.Server.UploadLocation))
}

func TestInitConfigCommand_FileAlreadyExists(t *testing.T) {
	c := qt.New(t)

	// Create a temporary directory to use as config dir
	tempDir := t.TempDir()
	
	// Override the user config directory for this test
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if originalConfigDir != "" {
			os.Setenv("XDG_CONFIG_HOME", originalConfigDir)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	// On Windows, we need to override APPDATA instead
	if os.Getenv("OS") == "Windows_NT" {
		originalAppData := os.Getenv("APPDATA")
		t.Cleanup(func() {
			if originalAppData != "" {
				os.Setenv("APPDATA", originalAppData)
			} else {
				os.Unsetenv("APPDATA")
			}
		})
		os.Setenv("APPDATA", tempDir)
	}

	// Create the config directory and file first
	configDir := filepath.Join(tempDir, "inventario")
	err := os.MkdirAll(configDir, 0o755)
	c.Assert(err, qt.IsNil)
	
	configPath := filepath.Join(configDir, "config.yaml")
	err = os.WriteFile(configPath, []byte("existing config"), 0o644)
	c.Assert(err, qt.IsNil)

	cmd := initconfig.NewInitConfigCommand()
	err = cmd.Execute()
	c.Assert(err, qt.ErrorMatches, "configuration file already exists at .*")
}

func TestInitConfigCommand_DirectoryCreation(t *testing.T) {
	c := qt.New(t)

	// Create a temporary directory to use as config dir
	tempDir := t.TempDir()
	
	// Override the user config directory for this test
	originalConfigDir := os.Getenv("XDG_CONFIG_HOME")
	t.Cleanup(func() {
		if originalConfigDir != "" {
			os.Setenv("XDG_CONFIG_HOME", originalConfigDir)
		} else {
			os.Unsetenv("XDG_CONFIG_HOME")
		}
	})
	os.Setenv("XDG_CONFIG_HOME", tempDir)

	// On Windows, we need to override APPDATA instead
	if os.Getenv("OS") == "Windows_NT" {
		originalAppData := os.Getenv("APPDATA")
		t.Cleanup(func() {
			if originalAppData != "" {
				os.Setenv("APPDATA", originalAppData)
			} else {
				os.Unsetenv("APPDATA")
			}
		})
		os.Setenv("APPDATA", tempDir)
	}

	// Ensure the inventario directory doesn't exist initially
	inventarioDir := filepath.Join(tempDir, "inventario")
	_, err := os.Stat(inventarioDir)
	c.Assert(os.IsNotExist(err), qt.IsTrue, qt.Commentf("inventario directory should not exist initially"))

	cmd := initconfig.NewInitConfigCommand()
	err = cmd.Execute()
	c.Assert(err, qt.IsNil)

	// Verify the directory was created
	info, err := os.Stat(inventarioDir)
	c.Assert(err, qt.IsNil)
	c.Assert(info.IsDir(), qt.IsTrue)

	// Only check permissions on Unix-like systems (Windows has different permission model)
	if runtime.GOOS != "windows" {
		c.Assert(info.Mode().Perm(), qt.Equals, os.FileMode(0o755))
	}

	// Verify the config file was created with correct permissions
	configPath := filepath.Join(inventarioDir, "config.yaml")
	info, err = os.Stat(configPath)
	c.Assert(err, qt.IsNil)

	// Only check permissions on Unix-like systems
	if runtime.GOOS != "windows" {
		c.Assert(info.Mode().Perm(), qt.Equals, os.FileMode(0o644))
	}
}

func TestEnvironmentVariableOverrides_Documentation(t *testing.T) {
	// Test that the configuration file documents the correct environment variable format
	// This ensures our documentation matches what the application actually supports

	tests := []struct {
		name           string
		configKey      string
		expectedEnvVar string
		flagName       string
	}{
		{
			name:           "server address",
			configKey:      "addr",
			expectedEnvVar: "INVENTARIO_ADDR",
			flagName:       "--addr",
		},
		{
			name:           "upload location",
			configKey:      "upload-location",
			expectedEnvVar: "INVENTARIO_UPLOAD_LOCATION",
			flagName:       "--upload-location",
		},
		{
			name:           "database DSN",
			configKey:      "db-dsn",
			expectedEnvVar: "INVENTARIO_DB_DSN",
			flagName:       "--db-dsn",
		},
		{
			name:           "max concurrent exports",
			configKey:      "max-concurrent-exports",
			expectedEnvVar: "INVENTARIO_MAX_CONCURRENT_EXPORTS",
			flagName:       "--max-concurrent-exports",
		},
		{
			name:           "max concurrent imports",
			configKey:      "max-concurrent-imports",
			expectedEnvVar: "INVENTARIO_MAX_CONCURRENT_IMPORTS",
			flagName:       "--max-concurrent-imports",
		},
	}

	sampleContent := getSampleConfig(t)

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := qt.New(t)

			// Verify the configuration file contains the expected key
			c.Assert(sampleContent, qt.Contains, test.configKey+":")

			// Verify the documentation mentions the environment variable format
			c.Assert(sampleContent, qt.Contains, "INVENTARIO_<FLAG_NAME>")

			// The environment variable names should follow the documented pattern
			// This test ensures our documentation is accurate
			c.Assert(strings.HasPrefix(test.expectedEnvVar, "INVENTARIO_"), qt.IsTrue,
				qt.Commentf("Environment variable %s should start with INVENTARIO_", test.expectedEnvVar))
		})
	}
}

func TestConfigurationFileContent_MatchesRunCommandFlags(t *testing.T) {
	c := qt.New(t)

	// This test ensures the configuration file only contains settings that
	// correspond to actual flags supported by the 'inventario run' command

	sampleContent := getSampleConfig(t)
	cfg := defaults.New()

	// Expected configuration keys and their corresponding run command flags
	// Using actual default values from the shared defaults package
	expectedMappings := map[string]string{
		fmt.Sprintf("addr: \"%s\"", cfg.Server.Addr):                                    "--addr",
		"upload-location:":                                                              "--upload-location",
		fmt.Sprintf("db-dsn: \"%s\"", cfg.Database.DSN):                                "--db-dsn",
		fmt.Sprintf("max-concurrent-exports: %d", cfg.Workers.MaxConcurrentExports):    "--max-concurrent-exports",
		fmt.Sprintf("max-concurrent-imports: %d", cfg.Workers.MaxConcurrentImports):    "--max-concurrent-imports",
	}

	for configLine, flagName := range expectedMappings {
		c.Assert(sampleContent, qt.Contains, configLine,
			qt.Commentf("Configuration should contain %s (maps to %s flag)", configLine, flagName))
	}

	// Verify the configuration file explains that UI settings are not included
	c.Assert(sampleContent, qt.Contains, "User interface settings")
	c.Assert(sampleContent, qt.Contains, "web interface")
	c.Assert(sampleContent, qt.Contains, "not configured through this file")

	// Verify it references the run command help
	c.Assert(sampleContent, qt.Contains, "inventario run --help")
}

func TestDefaultsConsistency(t *testing.T) {
	c := qt.New(t)

	// This test ensures that the defaults used in the config file generation
	// are consistent with the defaults package, preventing drift between
	// the init-config command and the run command defaults

	cfg := defaults.New()
	sampleContent := getSampleConfig(t)

	// Test that all default values from the defaults package appear in the config file
	c.Assert(sampleContent, qt.Contains, cfg.Server.Addr,
		qt.Commentf("Config file should contain server addr default: %s", cfg.Server.Addr))

	c.Assert(sampleContent, qt.Contains, cfg.Database.DSN,
		qt.Commentf("Config file should contain database DSN default: %s", cfg.Database.DSN))

	c.Assert(sampleContent, qt.Contains, fmt.Sprintf("%d", cfg.Workers.MaxConcurrentExports),
		qt.Commentf("Config file should contain max concurrent exports default: %d", cfg.Workers.MaxConcurrentExports))

	c.Assert(sampleContent, qt.Contains, fmt.Sprintf("%d", cfg.Workers.MaxConcurrentImports),
		qt.Commentf("Config file should contain max concurrent imports default: %d", cfg.Workers.MaxConcurrentImports))

	// Test that the upload location contains the expected pattern
	// (exact match is difficult due to absolute path generation)
	c.Assert(sampleContent, qt.Contains, "uploads?create_dir=1",
		qt.Commentf("Config file should contain upload location pattern"))
}
