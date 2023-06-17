package apiserver_test

import (
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func newLocationRegistry() registry.LocationRegistry {
	var locationsRegistry = registry.NewMemoryLocationRegistry()

	must.Must(locationsRegistry.Create(models.Location{
		Name:    "Location 1",
		Address: "Address 1",
	}))

	must.Must(locationsRegistry.Create(models.Location{
		Name:    "Location 2",
		Address: "Address 2",
	}))

	return locationsRegistry
}

func newAreaRegistry(locationRegistry registry.LocationRegistry) registry.AreaRegistry {
	var areaRegistry = registry.NewMemoryAreaRegistry(locationRegistry)

	locations := must.Must(locationRegistry.List())

	must.Must(areaRegistry.Create(models.Area{
		ID:         "1",
		Name:       "Area 1",
		LocationID: locations[0].ID,
	}))

	must.Must(areaRegistry.Create(models.Area{
		ID:         "2",
		Name:       "Area 2",
		LocationID: locations[0].ID,
	}))

	return areaRegistry
}

func newCommodityRegistry(areaRegistry registry.AreaRegistry) registry.CommodityRegistry {
	var commodityRegistry = registry.NewMemoryCommodityRegistry(areaRegistry)

	areas := must.Must(areaRegistry.List())

	must.Must(commodityRegistry.Create(models.Commodity{
		ID:            "1",
		Name:          "Commodity 1",
		ShortName:     "C1",
		AreaID:        areas[0].ID,
		Type:          models.CommodityTypeFurniture,
		Count:         10,
		OriginalPrice: must.Must(decimal.NewFromString("2000.00")),
	}))

	must.Must(commodityRegistry.Create(models.Commodity{
		ID:            "2",
		Name:          "Commodity 2",
		ShortName:     "C2",
		AreaID:        areas[0].ID,
		Type:          models.CommodityTypeElectronics,
		Count:         5,
		OriginalPrice: must.Must(decimal.NewFromString("1500.00")),
	}))

	return commodityRegistry
}

func newParams() apiserver.Params {
	var params apiserver.Params
	params.LocationRegistry = newLocationRegistry()
	params.AreaRegistry = newAreaRegistry(params.LocationRegistry)
	params.CommodityRegistry = newCommodityRegistry(params.AreaRegistry)
	return params
}
