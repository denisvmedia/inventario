package main

import (
	"github.com/denisvmedia/inventario/cmd/inventario"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/registry/migrations"
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
	boltdb.Register()
	memory.Register()
	postgresCleanup := postgres.Register()
	migrations.RegisterMigrators()

	// Combine cleanup functions
	cleanup = func() error {
		return postgresCleanup()
	}

	// Also register with the enhanced factory for capability detection
	// TODO: Implement enhanced factory registration
	// registry.RegisterBackendWithFactory("boltdb", func(c registry.Config) (*registry.Set, error) {
	//	return boltdb.NewRegistrySet(c)
	// })
	// registry.RegisterBackendWithFactory("memory", func(c registry.Config) (*registry.Set, error) {
	//	return memory.NewRegistrySet(c)
	// })

	// PostgreSQL registration is handled in postgres.Register()
	// The enhanced factory will automatically detect PostgreSQL capabilities

	return cleanup
}

func main() {
	cleanup := registerDBBackends()
	defer func() {
		err := cleanup()
		if err != nil {
			panic(err)
		}
	}()
	inventario.Execute()
}
