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
	boltdb.Register()
	memory.Register()
	cleanup = postgres.Register()
	migrations.RegisterMigrators()

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
