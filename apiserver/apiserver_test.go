package apiserver_test

import (
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func newLocationRegistry() registry.LocationRegistry {
	var locationsRegistry = registry.NewMemoryLocationRegistry()

	must.Must(locationsRegistry.Create(models.Location{
		Name:    "LocationResponse 1",
		Address: "Address 1",
	}))

	must.Must(locationsRegistry.Create(models.Location{
		Name:    "LocationResponse 2",
		Address: "Address 2",
	}))

	return locationsRegistry
}

func newAreaRegistry(locationRegistry registry.LocationRegistry) registry.AreaRegistry {
	var areaRegistry = registry.NewMemoryAreaRegistry(locationRegistry)

	locations := must.Must(locationRegistry.List())

	must.Must(areaRegistry.Create(models.Area{
		ID:         "1",
		Name:       "LocationResponse 1",
		LocationID: locations[0].ID,
	}))

	must.Must(areaRegistry.Create(models.Area{
		ID:         "1",
		Name:       "LocationResponse 2",
		LocationID: locations[0].ID,
	}))

	return areaRegistry
}
