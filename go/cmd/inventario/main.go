package main

import (
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/memory"
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

	// Combine cleanup functions
	cleanup = func() error {
		return postgresCleanup()
	}

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
	Execute()
}
