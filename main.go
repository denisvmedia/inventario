package main

import (
	"github.com/denisvmedia/inventario/cmd/inventario"
)

// @title Inventario API
// @version 1.0
// @description This is an Inventario daemon.

// @contact.name API Support
// @contact.url https://github.com/denisvmedia/inventario/issues
// @contact.email ask@artprima.cz

// @license.name MIT

// @BasePath /api/v1

func main() {
	inventario.Execute()
}
