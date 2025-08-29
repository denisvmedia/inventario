package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/registry/postgres"
)

func registerDBBackends() (cleanup func() error) {
	// Register backends with the traditional registry system
	memory.Register()
	postgresCleanup := postgres.Register()

	// Combine cleanup functions
	cleanup = func() error {
		return postgresCleanup()
	}

	return cleanup
}

func configPath() string {
	// Get the user's config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "config.yaml"
	}

	// Define the config file path
	configFilePath := filepath.Join(configDir, "inventario", "config.yaml")

	// Check if the config file exists
	if _, err := os.Stat(configFilePath); err != nil && !os.IsNotExist(err) {
		panic(err)
	}

	return configFilePath
}

func setupSlog() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
	}))
	slog.SetDefault(logger)
}

func main() {
	shared.SetEnvPrefix("INVENTARIO")
	shared.SetConfigFile(configPath())

	setupSlog()

	cleanup := registerDBBackends()
	defer func() {
		err := cleanup()
		if err != nil {
			panic(err)
		}
	}()
	Execute()
}
