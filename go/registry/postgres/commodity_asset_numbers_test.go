package postgres_test

import (
	"context"
	"sort"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// assetNumberPGFixture is two groups of one tenant plus a superuser handle
// for writing rows the way an older binary left them.
type assetNumberPGFixture struct {
	ctx        context.Context
	setA, setB *registry.Set
	service    registry.CommodityRegistry
	db         *sqlx.DB
}

func newAssetNumberPGFixture(t *testing.T) *assetNumberPGFixture {
	t.Helper()
	c := qt.New(t)

	setA, cleanup := setupTestRegistrySet(t)
	t.Cleanup(cleanup)
	user := getTestUser(c, setA)

	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	factorySet := createRegistrySetFromPool(pool)

	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	groupB, err := factorySet.CreateServiceRegistrySet().LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Name:                "Second Group",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	return &assetNumberPGFixture{
		ctx:     appctx.WithUser(context.Background(), user),
		setA:    setA,
		setB:    postgres.NewRegistrySetWithUserAndGroupID(db, user.ID, user.TenantID, groupB.ID),
		service: factorySet.CommodityRegistryFactory.CreateServiceRegistry(),
		db:      db,
	}
}

func newPGAssetCommodity(name string) models.Commodity {
	return models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		Status:    models.CommodityStatusInUse,
		Count:     1,
	}
}

func pgAssetNumber(c *qt.C, commodity *models.Commodity) int {
	c.Helper()
	c.Assert(commodity.AssetNumber, qt.IsNotNil, qt.Commentf("commodity %q has no asset number", commodity.Name))
	return *commodity.AssetNumber
}

func TestCommodityRegistry_AssetNumber_PerGroupAndServerManaged(t *testing.T) {
	f := newAssetNumberPGFixture(t)
	c := qt.New(t)

	for _, want := range []int{1, 2, 3} {
		item := newPGAssetCommodity("item")
		item.AssetNumber = new(777)
		created, err := f.setA.CommodityRegistry.Create(f.ctx, item)
		c.Assert(err, qt.IsNil)
		c.Assert(pgAssetNumber(c, created), qt.Equals, want)
	}

	other, err := f.setB.CommodityRegistry.Create(f.ctx, newPGAssetCommodity("other"))
	c.Assert(err, qt.IsNil)
	c.Assert(pgAssetNumber(c, other), qt.Equals, 1)

	// Update keeps the stored number whatever the caller sends.
	edit := *other
	edit.Name = "renamed"
	edit.AssetNumber = new(55)
	updated, err := f.setB.CommodityRegistry.Update(f.ctx, edit)
	c.Assert(err, qt.IsNil)
	c.Assert(pgAssetNumber(c, updated), qt.Equals, 1)
	got, err := f.setB.CommodityRegistry.Get(f.ctx, other.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(pgAssetNumber(c, got), qt.Equals, 1)
	c.Assert(got.Name, qt.Equals, "renamed")
}

func TestCommodityRegistry_AssetNumber_RestoreSeam(t *testing.T) {
	f := newAssetNumberPGFixture(t)
	c := qt.New(t)

	kept, err := f.setA.CommodityRegistry.Create(registry.WithRestoreAssetNumber(f.ctx, 42), newPGAssetCommodity("kept"))
	c.Assert(err, qt.IsNil)
	c.Assert(pgAssetNumber(c, kept), qt.Equals, 42)

	clash, err := f.setA.CommodityRegistry.Create(registry.WithRestoreAssetNumber(f.ctx, 42), newPGAssetCommodity("clash"))
	c.Assert(err, qt.IsNil)
	c.Assert(pgAssetNumber(c, clash), qt.Equals, 43)

	next, err := f.setA.CommodityRegistry.Create(f.ctx, newPGAssetCommodity("next"))
	c.Assert(err, qt.IsNil)
	c.Assert(pgAssetNumber(c, next), qt.Equals, 44)
}

func TestCommodityRegistry_AssetNumber_ConcurrentCreates(t *testing.T) {
	f := newAssetNumberPGFixture(t)
	c := qt.New(t)

	const n = 12
	numbers := make([]int, n)
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			created, err := f.setA.CommodityRegistry.Create(f.ctx, newPGAssetCommodity("item"))
			errs[i] = err
			if err == nil && created.AssetNumber != nil {
				numbers[i] = *created.AssetNumber
			}
		})
	}
	wg.Wait()

	for _, err := range errs {
		c.Assert(err, qt.IsNil)
	}
	sort.Ints(numbers)
	for i, got := range numbers {
		c.Assert(got, qt.Equals, i+1)
	}
}

func TestCommodityRegistry_BackfillAssetNumbers(t *testing.T) {
	f := newAssetNumberPGFixture(t)
	c := qt.New(t)

	create := func(set *registry.Set, name, registered string) *models.Commodity {
		item := newPGAssetCommodity(name)
		item.RegisteredDate = models.ToPDate(models.Date(registered))
		created, err := set.CommodityRegistry.Create(f.ctx, item)
		c.Assert(err, qt.IsNil)
		return created
	}
	// Created in this order, but registered on dates that put "older" first.
	newer := create(f.setA, "newer", "2024-05-01")
	older := create(f.setA, "older", "2023-01-01")
	kept := create(f.setA, "kept", "2022-01-01")
	otherGroup := create(f.setB, "other-group", "2024-01-01")

	// Strip the numbers the way rows written before the column look; keep
	// one so the backfill has to continue after it.
	_, err := f.db.ExecContext(context.Background(),
		`UPDATE commodities SET asset_number = NULL WHERE id IN ($1, $2, $3)`, newer.ID, older.ID, otherGroup.ID)
	c.Assert(err, qt.IsNil)
	_, err = f.db.ExecContext(context.Background(),
		`UPDATE commodities SET asset_number = 10 WHERE id = $1`, kept.ID)
	c.Assert(err, qt.IsNil)

	count, err := f.service.BackfillAssetNumbers(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 3)

	number := func(set *registry.Set, id string) int {
		got, err := set.CommodityRegistry.Get(f.ctx, id)
		c.Assert(err, qt.IsNil)
		return pgAssetNumber(c, got)
	}
	// No history events here, so the registered date decides the order.
	c.Assert(number(f.setA, older.ID), qt.Equals, 11)
	c.Assert(number(f.setA, newer.ID), qt.Equals, 12)
	c.Assert(number(f.setA, kept.ID), qt.Equals, 10)
	c.Assert(number(f.setB, otherGroup.ID), qt.Equals, 1)

	count, err = f.service.BackfillAssetNumbers(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	next, err := f.setA.CommodityRegistry.Create(f.ctx, newPGAssetCommodity("next"))
	c.Assert(err, qt.IsNil)
	c.Assert(pgAssetNumber(c, next), qt.Equals, 13)
}

func TestCommodityRegistry_ListPaginated_SearchByAssetID(t *testing.T) {
	f := newAssetNumberPGFixture(t)
	c := qt.New(t)

	for _, name := range []string{"Drill", "Ladder", "Toolbox"} {
		_, err := f.setA.CommodityRegistry.Create(f.ctx, newPGAssetCommodity(name))
		c.Assert(err, qt.IsNil)
	}

	for _, q := range []string{"A-0002", "a-2", "#2", "2"} {
		c.Run(q, func(c *qt.C) {
			items, total, err := f.setA.CommodityRegistry.ListPaginated(f.ctx, 0, 10, registry.CommodityListOptions{
				Search:          q,
				IncludeInactive: true,
			})
			c.Assert(err, qt.IsNil)
			c.Assert(total, qt.Equals, 1)
			c.Assert(items, qt.HasLen, 1)
			c.Assert(items[0].Name, qt.Equals, "Ladder")
		})
	}

	items, _, err := f.setA.CommodityRegistry.ListPaginated(f.ctx, 0, 10, registry.CommodityListOptions{Search: "A-0009", IncludeInactive: true})
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.HasLen, 0)
}
