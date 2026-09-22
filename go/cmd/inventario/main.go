package main

import (
	"log/slog"
	"os"
	"path/filepath"

	"go.5x5.cz/inventario/cmd/inventario/shared"
	"go.5x5.cz/inventario/internal/utcclock"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/registry/postgres"
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
	var handler slog.Handler
	if os.Getenv("INVENTARIO_LOG_FORMAT") == "json" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
		})
	}

	slog.SetDefault(slog.New(handler))
}

func main() {
	// Before anything reads a clock. See the package comment: the schema's
	// timestamp columns carry no zone, so the process has to supply one.
	utcclock.Pin()

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
