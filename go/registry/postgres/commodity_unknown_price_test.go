package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// unknownPricePGFixture is a user registry set plus the service registry and
// a superuser handle for writing rows the way an older binary left them.
type unknownPricePGFixture struct {
	ctx     context.Context
	set     *registry.Set
	service registry.CommodityRegistry
	db      *sqlx.DB
	areaID  string
}

func newUnknownPricePGFixture(t *testing.T) *unknownPricePGFixture {
	t.Helper()
	c := qt.New(t)

	set, cleanup := setupTestRegistrySet(t)
	t.Cleanup(cleanup)
	user := getTestUser(c, set)

	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	factorySet := createRegistrySetFromPool(pool)

	location := createTestLocation(c, set)
	area := createTestArea(c, set, location.GetID())

	return &unknownPricePGFixture{
		ctx:     appctx.WithUser(context.Background(), user),
		set:     set,
		service: factorySet.CommodityRegistryFactory.CreateServiceRegistry(),
		db:      sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx"),
		areaID:  area.GetID(),
	}
}

func (f *unknownPricePGFixture) create(c *qt.C, com models.Commodity) *models.Commodity {
	c.Helper()
	com.ShortName = com.Name
	com.Type = models.CommodityTypeOther
	com.Status = models.CommodityStatusInUse
	com.Count = 1
	com.AreaID = new(f.areaID)
	if com.OriginalPriceCurrency == "" {
		com.OriginalPriceCurrency = "USD"
	}
	created, err := f.set.CommodityRegistry.Create(f.ctx, com)
	c.Assert(err, qt.IsNil)
	return created
}

func TestCommodityRegistry_PricesRoundTripUnknownZeroAndKnown(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)

	testCases := []struct {
		name  string
		price *decimal.Decimal
	}{
		{name: "unknown", price: nil},
		{name: "zero", price: new(decimal.Zero)},
		{name: "positive", price: new(decimal.RequireFromString("19.99"))},
	}
	for _, tc := range testCases {
		created := f.create(c, models.Commodity{
			Name:          tc.name,
			OriginalPrice: tc.price,
			CurrentPrice:  tc.price,
		})
		got, err := f.set.CommodityRegistry.Get(f.ctx, created.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(models.SamePrice(got.OriginalPrice, tc.price), qt.IsTrue, qt.Commentf("%s original: %v", tc.name, got.OriginalPrice))
		c.Assert(models.SamePrice(got.CurrentPrice, tc.price), qt.IsTrue, qt.Commentf("%s current: %v", tc.name, got.CurrentPrice))
		c.Assert(got.ConvertedOriginalPrice, qt.IsNil)
		c.Assert(got.PurchaseDate, qt.IsNil)
	}

	// Each price can become unknown again on its own.
	free := f.create(c, models.Commodity{Name: "free", OriginalPrice: new(decimal.Zero), CurrentPrice: new(decimal.NewFromInt(5))})
	free.CurrentPrice = nil
	_, err := f.set.CommodityRegistry.Update(f.ctx, *free)
	c.Assert(err, qt.IsNil)
	got, err := f.set.CommodityRegistry.Get(f.ctx, free.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.CurrentPrice, qt.IsNil)
	c.Assert(got.OriginalPrice.String(), qt.Equals, "0")
}

func TestCommodityRegistry_ListPaginated_UnknownSortsLast(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)

	f.create(c, models.Commodity{Name: "cheap", OriginalPrice: new(decimal.NewFromInt(5)), CurrentPrice: new(decimal.NewFromInt(5)), PurchaseDate: models.ToPDate("2022-01-01")})
	f.create(c, models.Commodity{Name: "unknown"})
	f.create(c, models.Commodity{Name: "free", OriginalPrice: new(decimal.Zero), CurrentPrice: new(decimal.Zero), PurchaseDate: models.ToPDate("2021-01-01")})
	f.create(c, models.Commodity{Name: "dear", OriginalPrice: new(decimal.NewFromInt(50)), CurrentPrice: new(decimal.NewFromInt(50)), PurchaseDate: models.ToPDate("2023-01-01")})

	names := func(field registry.CommoditySortField, desc bool) []string {
		items, _, err := f.set.CommodityRegistry.ListPaginated(f.ctx, 0, 100, registry.CommodityListOptions{SortField: field, SortDesc: desc})
		c.Assert(err, qt.IsNil)
		out := make([]string, 0, len(items))
		for _, item := range items {
			out = append(out, item.Name)
		}
		return out
	}

	for _, field := range []registry.CommoditySortField{registry.CommoditySortOriginalPrice, registry.CommoditySortCurrentPrice, registry.CommoditySortPurchaseDate} {
		c.Assert(names(field, false), qt.DeepEquals, []string{"free", "cheap", "dear", "unknown"}, qt.Commentf("%s asc", field))
		c.Assert(names(field, true), qt.DeepEquals, []string{"dear", "cheap", "free", "unknown"}, qt.Commentf("%s desc", field))
	}
}

func TestCommodityRegistry_BackfillLegacyPrices(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)

	// A row an older binary wrote: zero meant "not set" for every price.
	legacy := f.create(c, models.Commodity{
		Name:                   "legacy",
		OriginalPrice:          new(decimal.NewFromInt(50)),
		OriginalPriceCurrency:  "EUR",
		ConvertedOriginalPrice: new(decimal.NewFromInt(55)),
	})
	legacyFree := f.create(c, models.Commodity{Name: "legacy-free", OriginalPrice: new(decimal.Zero)})
	_, err := f.db.ExecContext(c.Context(),
		`UPDATE commodities SET current_price = 0, converted_original_price = 0, legacy_zero_prices = true WHERE id = ANY($1)`,
		[]string{legacy.ID, legacyFree.ID})
	c.Assert(err, qt.IsNil)

	// A row written now: its known zero current value must survive.
	current := f.create(c, models.Commodity{Name: "current", OriginalPrice: new(decimal.NewFromInt(10)), CurrentPrice: new(decimal.Zero)})

	rewritten, err := f.service.BackfillLegacyPrices(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(rewritten, qt.Equals, 2)

	got, err := f.set.CommodityRegistry.Get(f.ctx, legacy.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.CurrentPrice, qt.IsNil)
	c.Assert(got.ConvertedOriginalPrice, qt.IsNil)
	c.Assert(got.OriginalPrice.String(), qt.Equals, "50")

	// The purchase price keeps its zero: it may be a real free acquisition.
	got, err = f.set.CommodityRegistry.Get(f.ctx, legacyFree.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.OriginalPrice, qt.IsNotNil)
	c.Assert(got.OriginalPrice.String(), qt.Equals, "0")
	c.Assert(got.CurrentPrice, qt.IsNil)

	got, err = f.set.CommodityRegistry.Get(f.ctx, current.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.CurrentPrice, qt.IsNotNil)
	c.Assert(got.CurrentPrice.String(), qt.Equals, "0")

	rewritten, err = f.service.BackfillLegacyPrices(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(rewritten, qt.Equals, 0)
}

func TestCommodityRegistry_UpdateBeforeBackfill(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)

	echoed := f.create(c, models.Commodity{
		Name: "echoed", OriginalPrice: new(decimal.NewFromInt(50)), OriginalPriceCurrency: "EUR",
		ConvertedOriginalPrice: new(decimal.NewFromInt(55)),
	})
	changed := f.create(c, models.Commodity{Name: "changed", OriginalPrice: new(decimal.NewFromInt(50))})
	_, err := f.db.ExecContext(c.Context(),
		`UPDATE commodities SET current_price = 0, converted_original_price = 0, legacy_zero_prices = true WHERE id = ANY($1)`,
		[]string{echoed.ID, changed.ID})
	c.Assert(err, qt.IsNil)

	// A full-row write before the backfill reaches the rows: one sends back
	// the legacy zeros it read (an edit of another field, a status change),
	// the other enters a current value of its own.
	staleEcho, err := f.set.CommodityRegistry.Get(f.ctx, echoed.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(staleEcho.CurrentPrice.IsZero(), qt.IsTrue)
	staleEcho.Comments = "edited"
	_, err = f.set.CommodityRegistry.Update(f.ctx, *staleEcho)
	c.Assert(err, qt.IsNil)

	staleChanged, err := f.set.CommodityRegistry.Get(f.ctx, changed.ID)
	c.Assert(err, qt.IsNil)
	staleChanged.CurrentPrice = new(decimal.NewFromInt(30))
	_, err = f.set.CommodityRegistry.Update(f.ctx, *staleChanged)
	c.Assert(err, qt.IsNil)

	// Both writes cleared the flag, so the backfill has nothing left.
	rewritten, err := f.service.BackfillLegacyPrices(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(rewritten, qt.Equals, 0)

	gotEcho, err := f.set.CommodityRegistry.Get(f.ctx, echoed.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(gotEcho.CurrentPrice, qt.IsNil)
	c.Assert(gotEcho.ConvertedOriginalPrice, qt.IsNil)
	c.Assert(gotEcho.Comments, qt.Equals, "edited")

	gotChanged, err := f.set.CommodityRegistry.Get(f.ctx, changed.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(gotChanged.CurrentPrice, qt.IsNotNil)
	c.Assert(gotChanged.CurrentPrice.String(), qt.Equals, "30")
}

func TestCommodityRegistry_GroupCurrencyItemDropsConvertedPrice(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)
	ctx := validationctx.WithGroupCurrency(f.ctx, "USD")

	// A client written before #2813 sends a converted zero for "not
	// entered" and omits the original price.
	created, err := f.set.CommodityRegistry.Create(ctx, models.Commodity{
		Name: "gift", ShortName: "gift", Type: models.CommodityTypeOther,
		Status: models.CommodityStatusInUse, Count: 1, AreaID: new(f.areaID),
		OriginalPriceCurrency: "USD", ConvertedOriginalPrice: new(decimal.Zero),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.ConvertedOriginalPrice, qt.IsNil)

	created.ConvertedOriginalPrice = new(decimal.Zero)
	_, err = f.set.CommodityRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)

	got, err := f.set.CommodityRegistry.Get(f.ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ConvertedOriginalPrice, qt.IsNil)
	_, known := got.Value("USD")
	c.Assert(known, qt.IsFalse)
}
