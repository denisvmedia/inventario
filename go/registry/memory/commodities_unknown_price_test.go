package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func newUnknownPriceSet(c *qt.C) (context.Context, *registry.Set) {
	c.Helper()
	factorySet := memory.NewFactorySet()
	user, err := factorySet.CreateServiceRegistrySet().UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
		Email: "u@example.com",
		Name:  "User",
	})
	c.Assert(err, qt.IsNil)
	group := &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: user.TenantID,
		},
		Slug:          "group-1-slug",
		Name:          "Group",
		GroupCurrency: "USD",
	}
	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), user), group)
	return ctx, must.Must(factorySet.CreateUserRegistrySet(ctx))
}

func createPriced(c *qt.C, ctx context.Context, set *registry.Set, com models.Commodity) *models.Commodity {
	c.Helper()
	com.ShortName = com.Name
	com.Type = models.CommodityTypeOther
	com.Status = models.CommodityStatusInUse
	com.Count = 1
	com.OriginalPriceCurrency = "USD"
	return must.Must(set.CommodityRegistry.Create(ctx, com))
}

func TestCommodityRegistry_PricesRoundTripUnknownZeroAndKnown(t *testing.T) {
	c := qt.New(t)
	ctx, set := newUnknownPriceSet(c)

	for _, price := range []*decimal.Decimal{nil, new(decimal.Zero), new(decimal.RequireFromString("19.99"))} {
		created := createPriced(c, ctx, set, models.Commodity{Name: "item", OriginalPrice: price, CurrentPrice: price})
		got := must.Must(set.CommodityRegistry.Get(ctx, created.ID))
		c.Assert(models.SamePrice(got.OriginalPrice, price), qt.IsTrue)
		c.Assert(models.SamePrice(got.CurrentPrice, price), qt.IsTrue)
		c.Assert(got.LegacyZeroPrices, qt.IsFalse)
	}
}

func TestCommodityRegistry_ListPaginated_UnknownSortsLast(t *testing.T) {
	c := qt.New(t)
	ctx, set := newUnknownPriceSet(c)

	createPriced(c, ctx, set, models.Commodity{Name: "cheap", OriginalPrice: new(decimal.NewFromInt(5)), CurrentPrice: new(decimal.NewFromInt(5)), PurchaseDate: models.ToPDate("2022-01-01")})
	createPriced(c, ctx, set, models.Commodity{Name: "unknown"})
	createPriced(c, ctx, set, models.Commodity{Name: "free", OriginalPrice: new(decimal.Zero), CurrentPrice: new(decimal.Zero), PurchaseDate: models.ToPDate("2021-01-01")})
	createPriced(c, ctx, set, models.Commodity{Name: "dear", OriginalPrice: new(decimal.NewFromInt(50)), CurrentPrice: new(decimal.NewFromInt(50)), PurchaseDate: models.ToPDate("2023-01-01")})

	names := func(field registry.CommoditySortField, desc bool) []string {
		items, _, err := set.CommodityRegistry.ListPaginated(ctx, 0, 100, registry.CommodityListOptions{SortField: field, SortDesc: desc})
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

func TestCommodityRegistry_BackfillLegacyPrices_NothingToRewrite(t *testing.T) {
	c := qt.New(t)
	ctx, set := newUnknownPriceSet(c)

	// Create never stores the legacy flag, so a zero written now is kept.
	created := createPriced(c, ctx, set, models.Commodity{Name: "free", OriginalPrice: new(decimal.Zero), CurrentPrice: new(decimal.Zero)})

	rewritten, err := set.CommodityRegistry.BackfillLegacyPrices(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(rewritten, qt.Equals, 0)

	got := must.Must(set.CommodityRegistry.Get(ctx, created.ID))
	c.Assert(got.CurrentPrice, qt.IsNotNil)
	c.Assert(got.CurrentPrice.IsZero(), qt.IsTrue)
}

func TestCommodityRegistry_GroupCurrencyItemDropsConvertedPrice(t *testing.T) {
	c := qt.New(t)
	ctx, set := newUnknownPriceSet(c)
	ctx = validationctx.WithGroupCurrency(ctx, "USD")

	// A client written before #2813 sends a converted zero for "not
	// entered" and omits the original price.
	created := createPriced(c, ctx, set, models.Commodity{Name: "gift", ConvertedOriginalPrice: new(decimal.Zero)})
	c.Assert(created.ConvertedOriginalPrice, qt.IsNil)

	created.ConvertedOriginalPrice = new(decimal.Zero)
	updated, err := set.CommodityRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.ConvertedOriginalPrice, qt.IsNil)
	_, known := updated.Value("USD")
	c.Assert(known, qt.IsFalse)
}

func TestCommodityRegistry_BackfillRegisteredDates_NothingToDate(t *testing.T) {
	c := qt.New(t)
	ctx, set := newUnknownPriceSet(c)

	// Create dates every item, so there is nothing left to date.
	created := createPriced(c, ctx, set, models.Commodity{Name: "item"})
	c.Assert(created.RegisteredDate, qt.IsNotNil)

	dated, pending, err := set.CommodityRegistry.BackfillRegisteredDates(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(dated, qt.Equals, 0)
	c.Assert(pending, qt.IsFalse)
}
