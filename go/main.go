package main

import (
	"github.com/denisvmedia/inventario/cmd/inventario"
	"github.com/denisvmedia/inventario/registry/boltdb"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/registry/postgresql"
)

// @title Inventario API
// @version 1.0
// @description This is an Inventario daemon.

// @contact.name Inventario Support
// @contact.url https://github.com/denisvmedia/inventario/issues
// @contact.email ask@artprima.cz

// @license.name MIT

// @BasePath /api/v1

func registerDBBackends() {
	boltdb.Register()
	memory.Register()
	postgresql.Register()
}

func main() {
	registerDBBackends()

	inventario.Execute()
}
