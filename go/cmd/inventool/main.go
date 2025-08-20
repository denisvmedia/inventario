package main

import (
	"os"
	"path/filepath"

	"github.com/denisvmedia/inventario/cmd/inventario/shared"
	"github.com/denisvmedia/inventario/registry/postgres"
)

// @title Inventario API
// @version 1.0
// @description This is an Inventario daemon.

// @contact.name Inventario Support
// @contact.url https://github.com/denisvmedia/inventario/issues
// @contact.email ask@artprima.cz

// @license.name MIT

// @BasePath /api/v1

func registerDBBackends() (cleanup func() error) {
	// Register backends with the traditional registry system
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

func main() {
	shared.SetEnvPrefix("INVENTOOL")
	shared.SetConfigFile(configPath())

	cleanup := registerDBBackends()
	defer func() {
		err := cleanup()
		if err != nil {
			panic(err)
		}
	}()
	Execute()
}
