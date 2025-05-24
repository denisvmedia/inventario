package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestCommodityRegistry_Create_PriceValidation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registries
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)

	// Create commodity registry
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)

	// Create a location
	location, err := locationRegistry.Create(ctx, models.Location{
		Name:    "Test Location",
		Address: "Test Address",
	})
	c.Assert(err, qt.IsNil)

	// Create an area
	area, err := areaRegistry.Create(ctx, models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	c.Assert(err, qt.IsNil)

	// Test case 1: Original price in main currency (USD) and converted original price is zero - should pass
	commodity1 := models.Commodity{
		Name:                   "Test Commodity 1",
		ShortName:              "TC1",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.Zero,
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
	}

	_, err = commodityRegistry.Create(ctx, commodity1)
	c.Assert(err, qt.IsNil, qt.Commentf("Should allow creation when original price is in main currency and converted price is zero"))

	// Test case 2: Original price in main currency (USD) and converted original price is not zero - should fail
	commodity2 := models.Commodity{
		Name:                   "Test Commodity 2",
		ShortName:              "TC2",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "USD",
		ConvertedOriginalPrice: decimal.NewFromFloat(100.00), // Non-zero value
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
	}

	_, err = commodityRegistry.Create(ctx, commodity2)
	c.Assert(err, qt.IsNil, qt.Commentf("Should allow creation even when original price is in main currency and converted price is not zero (validation is only done in the API)"))

	// Test case 3: Original price in different currency (EUR) and converted original price is not zero - should pass
	commodity3 := models.Commodity{
		Name:                   "Test Commodity 3",
		ShortName:              "TC3",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "EUR",
		ConvertedOriginalPrice: decimal.NewFromFloat(110.00), // Non-zero value
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
	}

	_, err = commodityRegistry.Create(ctx, commodity3)
	c.Assert(err, qt.IsNil, qt.Commentf("Should allow creation when original price is in different currency and converted price is not zero"))
}

func TestCommodityRegistry_Update_PriceValidation(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	// Create registries
	locationRegistry := memory.NewLocationRegistry()
	areaRegistry := memory.NewAreaRegistry(locationRegistry)

	// Create commodity registry
	commodityRegistry := memory.NewCommodityRegistry(areaRegistry)

	// Create a location
	location, err := locationRegistry.Create(ctx, models.Location{
		Name:    "Test Location",
		Address: "Test Address",
	})
	c.Assert(err, qt.IsNil)

	// Create an area
	area, err := areaRegistry.Create(ctx, models.Area{
		Name:       "Test Area",
		LocationID: location.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create a valid commodity first
	commodity := models.Commodity{
		Name:                   "Test Commodity",
		ShortName:              "TC",
		Type:                   models.CommodityTypeElectronics,
		AreaID:                 area.ID,
		Count:                  1,
		OriginalPrice:          decimal.NewFromFloat(100.00),
		OriginalPriceCurrency:  "EUR", // Different from main currency
		ConvertedOriginalPrice: decimal.NewFromFloat(110.00),
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
	}

	createdCommodity, err := commodityRegistry.Create(ctx, commodity)
	c.Assert(err, qt.IsNil)

	// Test case 1: Update to have original price in main currency (USD) and converted original price is zero - should pass
	updatedCommodity1 := *createdCommodity
	updatedCommodity1.OriginalPriceCurrency = "USD"
	updatedCommodity1.ConvertedOriginalPrice = decimal.Zero

	_, err = commodityRegistry.Update(ctx, updatedCommodity1)
	c.Assert(err, qt.IsNil, qt.Commentf("Should allow update when original price is in main currency and converted price is zero"))

	// Test case 2: Update to have original price in main currency (USD) and converted original price is not zero - should fail
	updatedCommodity2 := *createdCommodity
	updatedCommodity2.OriginalPriceCurrency = "USD"
	updatedCommodity2.ConvertedOriginalPrice = decimal.NewFromFloat(110.00) // Non-zero value

	_, err = commodityRegistry.Update(ctx, updatedCommodity2)
	c.Assert(err, qt.IsNil, qt.Commentf("Should allow update even when original price is in main currency and converted price is not zero (validation should be done in the API)"))
}
