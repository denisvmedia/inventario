package memory_test

import (
	"context"
	"sort"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// assetNumberFixture holds two groups of the same tenant on one memory
// factory set, so per-group numbering can be told apart from a global
// counter.
type assetNumberFixture struct {
	factorySet *registry.FactorySet
	ctxA, ctxB context.Context
	setA, setB *registry.Set
}

func newAssetNumberFixture(c *qt.C) *assetNumberFixture {
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

	groupCtx := func(id string) (context.Context, *registry.Set) {
		group := &models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: id},
				TenantID: user.TenantID,
			},
			Slug:          id + "-slug",
			Name:          id,
			GroupCurrency: "USD",
		}
		ctx := appctx.WithGroup(appctx.WithUser(context.Background(), user), group)
		return ctx, must.Must(factorySet.CreateUserRegistrySet(ctx))
	}
	f := &assetNumberFixture{factorySet: factorySet}
	f.ctxA, f.setA = groupCtx("group-a")
	f.ctxB, f.setB = groupCtx("group-b")
	return f
}

func newAssetCommodity(name string) models.Commodity {
	return models.Commodity{
		Name:      name,
		ShortName: name,
		Status:    models.CommodityStatusInUse,
		Type:      models.CommodityTypeOther,
		Count:     1,
	}
}

func assetNumberOf(c *qt.C, commodity *models.Commodity) int {
	c.Helper()
	c.Assert(commodity.AssetNumber, qt.IsNotNil, qt.Commentf("commodity %q has no asset number", commodity.Name))
	return *commodity.AssetNumber
}

func TestCommodityRegistry_AssetNumber_SequentialPerGroup(t *testing.T) {
	c := qt.New(t)
	f := newAssetNumberFixture(c)

	for i, want := range []int{1, 2, 3} {
		item := newAssetCommodity("a" + string(rune('0'+i)))
		// A caller-supplied number is ignored.
		item.AssetNumber = new(500)
		created, err := f.setA.CommodityRegistry.Create(f.ctxA, item)
		c.Assert(err, qt.IsNil)
		c.Assert(assetNumberOf(c, created), qt.Equals, want)
	}

	created, err := f.setB.CommodityRegistry.Create(f.ctxB, newAssetCommodity("b1"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, created), qt.Equals, 1)
}

func TestCommodityRegistry_AssetNumber_ContinuesAfterHighest(t *testing.T) {
	c := qt.New(t)
	f := newAssetNumberFixture(c)

	first, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("first"))
	c.Assert(err, qt.IsNil)
	second, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("second"))
	c.Assert(err, qt.IsNil)

	// Deleting a lower number leaves a gap that is not refilled.
	c.Assert(f.setA.CommodityRegistry.Delete(f.ctxA, first.ID), qt.IsNil)
	third, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("third"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, second), qt.Equals, 2)
	c.Assert(assetNumberOf(c, third), qt.Equals, 3)
}

func TestCommodityRegistry_AssetNumber_UpdateKeepsStoredNumber(t *testing.T) {
	c := qt.New(t)
	f := newAssetNumberFixture(c)

	created, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("thing"))
	c.Assert(err, qt.IsNil)

	for _, sent := range []*int{nil, new(99)} {
		edit := *created
		edit.Name = "renamed"
		edit.AssetNumber = sent
		updated, err := f.setA.CommodityRegistry.Update(f.ctxA, edit)
		c.Assert(err, qt.IsNil)
		c.Assert(assetNumberOf(c, updated), qt.Equals, 1)

		got, err := f.setA.CommodityRegistry.Get(f.ctxA, created.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(assetNumberOf(c, got), qt.Equals, 1)
		c.Assert(got.Name, qt.Equals, "renamed")
	}
}

func TestCommodityRegistry_AssetNumber_RestoreSeam(t *testing.T) {
	c := qt.New(t)
	f := newAssetNumberFixture(c)

	kept, err := f.setA.CommodityRegistry.Create(registry.WithRestoreAssetNumber(f.ctxA, 42), newAssetCommodity("kept"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, kept), qt.Equals, 42)

	// 42 is taken now: the restored item gets the next free number instead.
	clash, err := f.setA.CommodityRegistry.Create(registry.WithRestoreAssetNumber(f.ctxA, 42), newAssetCommodity("clash"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, clash), qt.Equals, 43)

	// Another group is unaffected by group A's numbers.
	other, err := f.setB.CommodityRegistry.Create(registry.WithRestoreAssetNumber(f.ctxB, 42), newAssetCommodity("other"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, other), qt.Equals, 42)

	// A plain create continues after the highest number.
	next, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("next"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, next), qt.Equals, 44)
}

func TestCommodityRegistry_AssetNumber_ConcurrentCreatesGetDistinctNumbers(t *testing.T) {
	c := qt.New(t)
	f := newAssetNumberFixture(c)

	const n = 25
	numbers := make([]int, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			created, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("item"))
			if err != nil || created.AssetNumber == nil {
				return
			}
			numbers[i] = *created.AssetNumber
		})
	}
	wg.Wait()

	sort.Ints(numbers)
	for i, got := range numbers {
		c.Assert(got, qt.Equals, i+1)
	}
}

func TestCommodityRegistry_BackfillAssetNumbers(t *testing.T) {
	c := qt.New(t)
	f := newAssetNumberFixture(c)

	// Rows written below the asset-number logic stand for commodities that
	// predate the column.
	legacy := func(ctx context.Context, set *registry.Set, name string) *models.Commodity {
		reg, ok := set.CommodityRegistry.(*memory.CommodityRegistry)
		c.Assert(ok, qt.IsTrue)
		created, err := reg.Registry.CreateWithUser(ctx, newAssetCommodity(name))
		c.Assert(err, qt.IsNil)
		c.Assert(created.AssetNumber, qt.IsNil)
		return created
	}

	oldA1 := legacy(f.ctxA, f.setA, "old-a1")
	numbered, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("numbered"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, numbered), qt.Equals, 1)
	oldA2 := legacy(f.ctxA, f.setA, "old-a2")
	oldB1 := legacy(f.ctxB, f.setB, "old-b1")

	service := f.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	count, err := service.BackfillAssetNumbers(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 3)

	get := func(ctx context.Context, set *registry.Set, id string) int {
		got, err := set.CommodityRegistry.Get(ctx, id)
		c.Assert(err, qt.IsNil)
		return assetNumberOf(c, got)
	}
	// Group A continues after its highest number, in creation order.
	c.Assert(get(f.ctxA, f.setA, oldA1.ID), qt.Equals, 2)
	c.Assert(get(f.ctxA, f.setA, oldA2.ID), qt.Equals, 3)
	c.Assert(get(f.ctxA, f.setA, numbered.ID), qt.Equals, 1)
	c.Assert(get(f.ctxB, f.setB, oldB1.ID), qt.Equals, 1)

	// Idempotent: a second pass finds nothing to do.
	count, err = service.BackfillAssetNumbers(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	next, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity("next"))
	c.Assert(err, qt.IsNil)
	c.Assert(assetNumberOf(c, next), qt.Equals, 4)
}

func TestCommodityRegistry_ListPaginated_SearchByAssetID(t *testing.T) {
	c := qt.New(t)
	f := newAssetNumberFixture(c)

	for _, name := range []string{"Drill", "Ladder", "Toolbox"} {
		_, err := f.setA.CommodityRegistry.Create(f.ctxA, newAssetCommodity(name))
		c.Assert(err, qt.IsNil)
	}

	for _, q := range []string{"A-0002", "a-2", "A2", "#2", "2"} {
		c.Run(q, func(c *qt.C) {
			items, total, err := f.setA.CommodityRegistry.ListPaginated(f.ctxA, 0, 10, registry.CommodityListOptions{
				Search:          q,
				IncludeInactive: true,
			})
			c.Assert(err, qt.IsNil)
			c.Assert(total, qt.Equals, 1)
			c.Assert(items, qt.HasLen, 1)
			c.Assert(items[0].Name, qt.Equals, "Ladder")
		})
	}

	// A name search still works and a number nobody holds matches nothing.
	items, _, err := f.setA.CommodityRegistry.ListPaginated(f.ctxA, 0, 10, registry.CommodityListOptions{Search: "drill", IncludeInactive: true})
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.HasLen, 1)
	items, _, err = f.setA.CommodityRegistry.ListPaginated(f.ctxA, 0, 10, registry.CommodityListOptions{Search: "A-0009", IncludeInactive: true})
	c.Assert(err, qt.IsNil)
	c.Assert(items, qt.HasLen, 0)
}
