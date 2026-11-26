package valuation_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/google/go-cmp/cmp"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/internal/valuation"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// setupTestRegistry creates a test registry with test data and returns the
// user+group context the tests should pass to the valuator.
func setupTestRegistry(c *qt.C, groupCurrency string) (*registry.Set, context.Context) {
	c.Helper()

	nonGroupCurrency := "USD"

	if groupCurrency == "USD" {
		nonGroupCurrency = "EUR"
	}

	// Create a memory factory set for testing
	factorySet := memory.NewFactorySet()

	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}

	// Create the group via the registry so it has a real ID and the memory
	// commodity/area/location registries can stamp group_id consistently
	// with production. Attaching a bare in-memory LocationGroup with empty
	// ID would make group filtering a no-op and hide behaviour the valuator
	// actually depends on.
	slug := must.Must(models.GenerateGroupSlug())
	group := must.Must(factorySet.LocationGroupRegistry.Create(c.Context(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Slug:                slug,
		Name:                "Valuation Test Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       models.Currency(groupCurrency),
	}))

	ctx := appctx.WithGroup(appctx.WithUser(c.Context(), user), group)

	// Create user-aware registry set
	registrySet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	// Create locations
	location1, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Location 1",
	})
	c.Assert(err, qt.IsNil)

	location2, err := registrySet.LocationRegistry.Create(ctx, models.Location{
		Name: "Location 2",
	})
	c.Assert(err, qt.IsNil)

	// Create areas
	areaRegistry := registrySet.AreaRegistry
	area1, err := areaRegistry.Create(ctx, models.Area{
		Name:       "Area 1",
		LocationID: location1.ID,
	})
	c.Assert(err, qt.IsNil)

	area2, err := areaRegistry.Create(ctx, models.Area{
		Name:       "Area 2",
		LocationID: location2.ID,
	})
	c.Assert(err, qt.IsNil)

	area3, err := registrySet.AreaRegistry.Create(ctx, models.Area{
		Name:       "Area 3",
		LocationID: location2.ID,
	})
	c.Assert(err, qt.IsNil)

	// Create commodities
	commodityRegistry := registrySet.CommodityRegistry
	_, err = commodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Commodity 1",
		ShortName:             "C1",
		AreaID:                new(area1.ID),
		Count:                 2,
		OriginalPrice:         new(decimal.NewFromFloat(100.00)),
		OriginalPriceCurrency: models.Currency(groupCurrency),
		CurrentPrice:          new(decimal.NewFromFloat(100.00)), // 100
		Status:                models.CommodityStatusInUse,
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Commodity 2",
		ShortName:             "C2",
		AreaID:                new(area1.ID),
		Count:                 1,
		OriginalPrice:         new(decimal.NewFromFloat(200.00)),
		OriginalPriceCurrency: models.Currency(nonGroupCurrency),
		// No converted price and no current price: it has no value in the
		// group currency and counts as unvalued.
		Status:       models.CommodityStatusInUse,
		Draft:        false,
		Type:         models.CommodityTypeElectronics,
		PurchaseDate: models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{
		Name:                   "Commodity 3",
		ShortName:              "C3",
		AreaID:                 new(area2.ID),
		Count:                  3,
		OriginalPrice:          new(decimal.NewFromFloat(300.00)), // 300
		OriginalPriceCurrency:  models.Currency(groupCurrency),
		ConvertedOriginalPrice: nil,
		Status:                 models.CommodityStatusInUse,
		Draft:                  false,
		Type:                   models.CommodityTypeElectronics,
		PurchaseDate:           models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{ // 400
		Name:                  "Commodity 4",
		ShortName:             "C4",
		AreaID:                new(area2.ID),
		Count:                 1,
		OriginalPrice:         new(decimal.NewFromFloat(400.00)),
		OriginalPriceCurrency: models.Currency(groupCurrency),
		Status:                models.CommodityStatusInUse,
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = commodityRegistry.Create(ctx, models.Commodity{ // 0: sold
		Name:                  "Commodity 5",
		ShortName:             "C5",
		AreaID:                new(area3.ID),
		Count:                 1,
		OriginalPrice:         new(decimal.NewFromFloat(500.00)),
		OriginalPriceCurrency: models.Currency(nonGroupCurrency),
		CurrentPrice:          new(decimal.NewFromFloat(500.00)),
		Status:                models.CommodityStatusSold, // Not in use
		Draft:                 false,
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	_, err = registrySet.CommodityRegistry.Create(ctx, models.Commodity{ // 0: draft
		Name:                  "Commodity 6",
		ShortName:             "C6",
		AreaID:                new(area3.ID),
		Count:                 1,
		OriginalPrice:         new(decimal.NewFromFloat(600.00)),
		OriginalPriceCurrency: models.Currency(nonGroupCurrency),
		CurrentPrice:          new(decimal.NewFromFloat(600.00)),
		Status:                models.CommodityStatusInUse,
		Draft:                 true, // Value
		Type:                  models.CommodityTypeElectronics,
		PurchaseDate:          models.ToPDate("2023-01-01"),
	})
	c.Assert(err, qt.IsNil)

	return registrySet, ctx
}

func TestValuator_Calculate(t *testing.T) {
	for _, groupCurrency := range []string{"USD", "EUR"} {
		t.Run(groupCurrency, func(t *testing.T) {
			c := qt.New(t)
			registrySet, ctx := setupTestRegistry(c, groupCurrency)

			totals, err := valuation.NewValuator(ctx, registrySet).Calculate()
			c.Assert(err, qt.IsNil)

			// 100 + 300 + 400: prices already describe the whole lot, so the
			// count does not multiply them. Commodity 2 has no value in the
			// group currency; the sold commodity and the draft are left out
			// of the totals and of the counts.
			c.Assert(totals.Global.Value.String(), qt.Equals, "800")
			c.Assert(totals.Global.ValuedCount, qt.Equals, 3)
			c.Assert(totals.Global.UnvaluedCount, qt.Equals, 1)

			locations, err := registrySet.LocationRegistry.List(ctx)
			c.Assert(err, qt.IsNil)
			locationIDs := make(map[string]string)
			for _, l := range locations {
				locationIDs[l.Name] = l.ID
			}
			areas, err := registrySet.AreaRegistry.List(ctx)
			c.Assert(err, qt.IsNil)
			areaIDs := make(map[string]string)
			for _, a := range areas {
				areaIDs[a.Name] = a.ID
			}

			c.Assert(totals.ByLocation, qt.HasLen, 2)
			c.Assert(totals.ByLocation[locationIDs["Location 1"]], qt.CmpEquals(decimalCmp),
				valuation.Total{Value: decimal.NewFromInt(100), ValuedCount: 1, UnvaluedCount: 1})
			c.Assert(totals.ByLocation[locationIDs["Location 2"]], qt.CmpEquals(decimalCmp),
				valuation.Total{Value: decimal.NewFromInt(700), ValuedCount: 2})

			// Area 3 holds only the sold commodity and the draft.
			c.Assert(totals.ByArea, qt.HasLen, 2)
			c.Assert(totals.ByArea[areaIDs["Area 1"]], qt.CmpEquals(decimalCmp),
				valuation.Total{Value: decimal.NewFromInt(100), ValuedCount: 1, UnvaluedCount: 1})
			c.Assert(totals.ByArea[areaIDs["Area 2"]], qt.CmpEquals(decimalCmp),
				valuation.Total{Value: decimal.NewFromInt(700), ValuedCount: 2})
		})
	}
}

// TestValuator_Calculate_UnknownPrices covers the acquisitions whose price is
// unknown or zero: a known zero is a value, an unknown price is not.
func TestValuator_Calculate_UnknownPrices(t *testing.T) {
	c := qt.New(t)
	registrySet, ctx := setupEmptyGroup(c, "USD")

	location, err := registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Home"})
	c.Assert(err, qt.IsNil)
	area, err := registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Hall", LocationID: location.ID})
	c.Assert(err, qt.IsNil)

	commodities := []models.Commodity{
		{
			// A gift: no purchase date, no price at all.
			Name:                  "Gift",
			OriginalPriceCurrency: "USD",
		},
		{
			// A free acquisition: the price is known and it is zero.
			Name:                  "Free",
			OriginalPrice:         new(decimal.Zero),
			OriginalPriceCurrency: "USD",
			PurchaseDate:          models.ToPDate("2024-02-01"),
		},
		{
			// Nothing known about the purchase, but it has a current value.
			Name:                  "Appraised",
			OriginalPriceCurrency: "USD",
			CurrentPrice:          new(decimal.NewFromInt(250)),
		},
		{
			// Bought abroad: a known foreign price with its conversion.
			Name:                   "Imported",
			OriginalPrice:          new(decimal.NewFromInt(100)),
			OriginalPriceCurrency:  "EUR",
			ConvertedOriginalPrice: new(decimal.NewFromInt(110)),
			PurchaseDate:           models.ToPDate("2024-03-01"),
		},
		{
			// No area: it counts towards the global total only.
			Name:                  "Unassigned",
			OriginalPrice:         new(decimal.NewFromInt(40)),
			OriginalPriceCurrency: "USD",
		},
	}
	for i := range commodities {
		commodity := commodities[i]
		commodity.ShortName = commodity.Name
		commodity.Count = 1
		commodity.Type = models.CommodityTypeOther
		commodity.Status = models.CommodityStatusInUse
		if commodity.Name != "Unassigned" {
			commodity.AreaID = new(area.ID)
		}
		c.Assert(commodity.ValidateWithContext(validationContext(ctx, "USD")), qt.IsNil, qt.Commentf("%s", commodity.Name))
		_, err := registrySet.CommodityRegistry.Create(ctx, commodity)
		c.Assert(err, qt.IsNil)
	}

	totals, err := valuation.NewValuator(ctx, registrySet).Calculate()
	c.Assert(err, qt.IsNil)

	// 0 + 250 + 110 + 40; the gift is the one unvalued item.
	c.Assert(totals.Global.Value.String(), qt.Equals, "400")
	c.Assert(totals.Global.ValuedCount, qt.Equals, 4)
	c.Assert(totals.Global.UnvaluedCount, qt.Equals, 1)

	c.Assert(totals.ByArea[area.ID].Value.String(), qt.Equals, "360")
	c.Assert(totals.ByArea[area.ID].ValuedCount, qt.Equals, 3)
	c.Assert(totals.ByArea[area.ID].UnvaluedCount, qt.Equals, 1)
	c.Assert(totals.ByLocation[location.ID], qt.CmpEquals(decimalCmp), totals.ByArea[area.ID])
}

// TestValuator_Calculate_Statuses checks that only an item in use counts
// towards the totals: an item sold, lost, disposed, written off, donated or
// stolen is no longer the group's.
func TestValuator_Calculate_Statuses(t *testing.T) {
	tests := []struct {
		status models.CommodityStatus
		counts bool
	}{
		{models.CommodityStatusInUse, true},
		{models.CommodityStatusSold, false},
		{models.CommodityStatusLost, false},
		{models.CommodityStatusDisposed, false},
		{models.CommodityStatusWrittenOff, false},
		{models.CommodityStatusDonated, false},
		{models.CommodityStatusStolen, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			c := qt.New(t)
			registrySet, ctx := setupEmptyGroup(c, "USD")
			location := must.Must(registrySet.LocationRegistry.Create(ctx, models.Location{Name: "Home"}))
			area := must.Must(registrySet.AreaRegistry.Create(ctx, models.Area{Name: "Hall", LocationID: location.ID}))

			commodity := models.Commodity{
				Name:                  "Bike",
				ShortName:             "Bike",
				AreaID:                new(area.ID),
				Count:                 1,
				Type:                  models.CommodityTypeOther,
				OriginalPrice:         new(decimal.NewFromInt(300)),
				OriginalPriceCurrency: "USD",
				PurchaseDate:          models.ToPDate("2024-01-01"),
				Status:                tt.status,
			}
			if tt.status != models.CommodityStatusInUse {
				commodity.StatusDate = models.ToPDate("2026-11-26")
			}
			if tt.status == models.CommodityStatusSold {
				commodity.SalePrice = new(decimal.NewFromInt(100))
			}
			c.Assert(commodity.ValidateWithContext(validationContext(ctx, "USD")), qt.IsNil)
			must.Must(registrySet.CommodityRegistry.Create(ctx, commodity))

			totals, err := valuation.NewValuator(ctx, registrySet).Calculate()
			c.Assert(err, qt.IsNil)
			if !tt.counts {
				c.Assert(totals.Global, qt.CmpEquals(decimalCmp), valuation.Total{})
				c.Assert(totals.ByArea, qt.HasLen, 0)
				return
			}
			c.Assert(totals.Global, qt.CmpEquals(decimalCmp), valuation.Total{Value: decimal.NewFromInt(300), ValuedCount: 1})
			c.Assert(totals.ByArea[area.ID], qt.CmpEquals(decimalCmp), totals.Global)
		})
	}
}

var decimalCmp = cmp.Comparer(func(a, b decimal.Decimal) bool { return a.Equal(b) })

// setupEmptyGroup returns a registry set for a fresh group with no data.
func setupEmptyGroup(c *qt.C, groupCurrency string) (*registry.Set, context.Context) {
	c.Helper()
	factorySet := memory.NewFactorySet()
	user := &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "test-tenant-id",
			EntityID: models.EntityID{ID: "test-user-id"},
		},
	}
	group := must.Must(factorySet.LocationGroupRegistry.Create(c.Context(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Unknown Prices Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       models.Currency(groupCurrency),
	}))
	ctx := appctx.WithGroup(appctx.WithUser(c.Context(), user), group)
	return must.Must(factorySet.CreateUserRegistrySet(ctx)), ctx
}

func validationContext(ctx context.Context, groupCurrency string) context.Context {
	return validationctx.WithGroupCurrency(ctx, groupCurrency)
}
