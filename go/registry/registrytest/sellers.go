package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// SellerEnv is what RunSellerContract needs from a backend: registries of
// two groups of one tenant, each with an area items can be placed in, and the
// contexts that carry each group's scope. Both groups start without sellers
// and without items.
type SellerEnv struct {
	CtxA, CtxB                 context.Context
	SellersA, SellersB         registry.SellerRegistry
	CommoditiesA, CommoditiesB registry.CommodityRegistry
	AreaA, AreaB               string
}

// RunSellerContract checks the behavior registry.SellerRegistry promises:
// one name per group whatever its case and spacing, sorted listing, item
// counts, deletion that unlinks items, the seller filter of the item list,
// and that a group sees and links only its own sellers.
func RunSellerContract(t *testing.T, newEnv func(t *testing.T) SellerEnv) {
	t.Helper()

	t.Run("create normalizes and refuses a duplicate name", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		created, err := env.SellersA.Create(env.CtxA, models.Seller{
			Name: "  Hardware   Store ", Website: " https://hw.example ", Contact: " Ann ", Note: " cash only ",
		})
		c.Assert(err, qt.IsNil)
		c.Check(created.ID, qt.Not(qt.Equals), "")
		c.Check(created.Name, qt.Equals, "Hardware Store")
		c.Check(created.NameKey, qt.Equals, "hardware store")
		c.Check(created.Website, qt.Equals, "https://hw.example")
		c.Check(created.Contact, qt.Equals, "Ann")
		c.Check(created.Note, qt.Equals, "cash only")

		got, err := env.SellersA.Get(env.CtxA, created.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.Name, qt.Equals, "Hardware Store")
		c.Check(got.GroupID, qt.Equals, created.GroupID)

		for _, dup := range []string{"hardware store", "HARDWARE STORE", " Hardware\tStore "} {
			_, err = env.SellersA.Create(env.CtxA, models.Seller{Name: dup})
			c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists, qt.Commentf("name %q", dup))
		}

		// The same name in another group is a different seller.
		_, err = env.SellersB.Create(env.CtxB, models.Seller{Name: "Hardware Store"})
		c.Assert(err, qt.IsNil)
	})

	t.Run("update keeps the name unique and allows a case change", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		a, err := env.SellersA.Create(env.CtxA, models.Seller{Name: "Alpha"})
		c.Assert(err, qt.IsNil)
		b, err := env.SellersA.Create(env.CtxA, models.Seller{Name: "Beta"})
		c.Assert(err, qt.IsNil)

		b.Name = "ALPHA"
		_, err = env.SellersA.Update(env.CtxA, *b)
		c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists)

		a.Name = "ALPHA"
		a.Note = "renamed"
		updated, err := env.SellersA.Update(env.CtxA, *a)
		c.Assert(err, qt.IsNil)
		c.Check(updated.Name, qt.Equals, "ALPHA")
		c.Check(updated.Note, qt.Equals, "renamed")

		got, err := env.SellersA.GetByName(env.CtxA, " alpha ")
		c.Assert(err, qt.IsNil)
		c.Check(got.ID, qt.Equals, a.ID)

		_, err = env.SellersA.GetByName(env.CtxA, "gamma")
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	})

	t.Run("list is sorted by name and scoped to the group", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		for _, name := range []string{"zeta", "Alpha", "beta"} {
			_, err := env.SellersA.Create(env.CtxA, models.Seller{Name: name})
			c.Assert(err, qt.IsNil)
		}
		_, err := env.SellersB.Create(env.CtxB, models.Seller{Name: "Other group"})
		c.Assert(err, qt.IsNil)

		list, err := env.SellersA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		names := make([]string, 0, len(list))
		for _, s := range list {
			names = append(names, s.Name)
		}
		c.Check(names, qt.DeepEquals, []string{"Alpha", "beta", "zeta"})

		count, err := env.SellersA.Count(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(count, qt.Equals, 3)
	})

	t.Run("item counts, filter and delete", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		shop, err := env.SellersA.Create(env.CtxA, models.Seller{Name: "Shop"})
		c.Assert(err, qt.IsNil)
		market, err := env.SellersA.Create(env.CtxA, models.Seller{Name: "Market"})
		c.Assert(err, qt.IsNil)
		idle, err := env.SellersA.Create(env.CtxA, models.Seller{Name: "Idle"})
		c.Assert(err, qt.IsNil)

		drill := createSellerItem(c, env.CtxA, env.CommoditiesA, env.AreaA, "Drill", &shop.ID, false)
		saw := createSellerItem(c, env.CtxA, env.CommoditiesA, env.AreaA, "Saw", &shop.ID, true)
		lamp := createSellerItem(c, env.CtxA, env.CommoditiesA, env.AreaA, "Lamp", &market.ID, false)
		createSellerItem(c, env.CtxA, env.CommoditiesA, env.AreaA, "Rug", nil, false)
		kettle := createSellerItem(c, env.CtxA, env.CommoditiesA, env.AreaA, "Kettle", &shop.ID, false)
		_, err = env.CommoditiesA.Trash(env.CtxA, kettle.ID, "registrytest", time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC))
		c.Assert(err, qt.IsNil)

		counts, err := env.SellersA.ItemCounts(env.CtxA)
		c.Assert(err, qt.IsNil)
		// The draft counts and the item in Trash does not, as on every
		// other list of items.
		c.Check(counts, qt.DeepEquals, map[string]int{shop.ID: 2, market.ID: 1})
		c.Check(counts[idle.ID], qt.Equals, 0)

		filtered := listSellerItems(c, env.CtxA, env.CommoditiesA, []string{shop.ID}, false)
		c.Check(filtered, qt.DeepEquals, []string{"Drill", "Saw"})
		filtered = listSellerItems(c, env.CtxA, env.CommoditiesA, []string{shop.ID, market.ID}, false)
		c.Check(filtered, qt.DeepEquals, []string{"Drill", "Lamp", "Saw"})
		filtered = listSellerItems(c, env.CtxA, env.CommoditiesA, []string{"no-such-seller"}, false)
		c.Check(filtered, qt.HasLen, 0)
		filtered = listSellerItems(c, env.CtxA, env.CommoditiesA, []string{shop.ID}, true)
		c.Check(filtered, qt.DeepEquals, []string{"Kettle"})

		c.Assert(env.SellersA.Delete(env.CtxA, shop.ID), qt.IsNil)
		_, err = env.SellersA.Get(env.CtxA, shop.ID)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)

		for _, id := range []string{drill.ID, saw.ID} {
			item, err := env.CommoditiesA.Get(env.CtxA, id)
			c.Assert(err, qt.IsNil)
			c.Check(item.SellerID, qt.IsNil, qt.Commentf("item %s still points at the deleted seller", item.Name))
		}
		item, err := env.CommoditiesA.Get(env.CtxA, lamp.ID)
		c.Assert(err, qt.IsNil)
		c.Check(item.SellerID, qt.DeepEquals, &market.ID)
		// The item in Trash loses the seller too and comes back without it.
		restored, err := env.CommoditiesA.Restore(env.CtxA, kettle.ID)
		c.Assert(err, qt.IsNil)
		c.Check(restored.SellerID, qt.IsNil)

		counts, err = env.SellersA.ItemCounts(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(counts, qt.DeepEquals, map[string]int{market.ID: 1})
	})

	t.Run("an empty seller ID is no seller", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		item := createSellerItem(c, env.CtxA, env.CommoditiesA, env.AreaA, "Vase", new(""), false)
		c.Check(item.SellerID, qt.IsNil)
		got, err := env.CommoditiesA.Get(env.CtxA, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.SellerID, qt.IsNil)
	})

	t.Run("a group cannot see or link another group's seller", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		foreign, err := env.SellersB.Create(env.CtxB, models.Seller{Name: "Foreign"})
		c.Assert(err, qt.IsNil)

		_, err = env.SellersA.Get(env.CtxA, foreign.ID)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)
		c.Check(env.SellersA.Delete(env.CtxA, foreign.ID), qt.ErrorIs, registry.ErrNotFound)
		_, err = env.SellersB.Get(env.CtxB, foreign.ID)
		c.Check(err, qt.IsNil, qt.Commentf("the failed delete from group A removed group B's seller"))

		_, err = env.CommoditiesA.Create(env.CtxA, sellerItem("Smuggled", env.AreaA, &foreign.ID, false))
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)

		item := createSellerItem(c, env.CtxA, env.CommoditiesA, env.AreaA, "Honest", nil, false)
		item.SellerID = &foreign.ID
		_, err = env.CommoditiesA.Update(env.CtxA, *item)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	})
}

func sellerItem(name, areaID string, sellerID *string, draft bool) models.Commodity {
	return models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		AreaID:    &areaID,
		Count:     1,
		Status:    models.CommodityStatusInUse,
		Draft:     draft,
		SellerID:  sellerID,
	}
}

func createSellerItem(c *qt.C, ctx context.Context, reg registry.CommodityRegistry, areaID, name string, sellerID *string, draft bool) *models.Commodity {
	c.Helper()
	item, err := reg.Create(ctx, sellerItem(name, areaID, sellerID, draft))
	c.Assert(err, qt.IsNil)
	return item
}

// listSellerItems returns the names of the items, drafts included, bought
// from one of sellerIDs, sorted by name.
func listSellerItems(c *qt.C, ctx context.Context, reg registry.CommodityRegistry, sellerIDs []string, trashed bool) []string {
	c.Helper()
	items, _, err := reg.ListPaginated(ctx, 0, 100, registry.CommodityListOptions{
		SellerIDs:       sellerIDs,
		IncludeInactive: true,
		Trashed:         trashed,
		SortField:       registry.CommoditySortName,
	})
	c.Assert(err, qt.IsNil)
	names := make([]string, 0, len(items))
	for _, it := range items {
		names = append(names, it.Name)
	}
	return names
}
