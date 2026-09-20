package services_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

type supplyLinkFixture struct {
	ctx     context.Context
	factory *registry.FactorySet
	svc     *services.SupplyLinkService
}

func newSupplyLinkFixture(c *qt.C) *supplyLinkFixture {
	c.Helper()

	factorySet := memory.NewFactorySet()
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
		Email: "u@example.com",
		Name:  "Tester",
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
	})

	return &supplyLinkFixture{
		ctx:     ctx,
		factory: factorySet,
		svc:     services.NewSupplyLinkService(factorySet),
	}
}

// seedCommodity creates a commodity and returns the id the registry assigned;
// memory registries mint their own, so the caller cannot pick one.
func (f *supplyLinkFixture) seedCommodity(c *qt.C, name string) string {
	c.Helper()
	reg, err := f.factory.CommodityRegistryFactory.CreateUserRegistry(f.ctx)
	c.Assert(err, qt.IsNil)
	created, err := reg.Create(f.ctx, models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeOther,
		Status:    models.CommodityStatusInUse,
		Count:     1,
	})
	c.Assert(err, qt.IsNil)
	return created.ID
}

func (f *supplyLinkFixture) newLink(commodityID, label, url string) models.SupplyLink {
	return models.SupplyLink{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: "tenant-1",
			GroupID:  "group-1",
		},
		CommodityID: commodityID,
		Label:       label,
		URL:         url,
	}
}

func (f *supplyLinkFixture) list(c *qt.C, commodityID string) []*models.SupplyLink {
	c.Helper()
	reg, err := f.factory.SupplyLinkRegistryFactory.CreateUserRegistry(f.ctx)
	c.Assert(err, qt.IsNil)
	links, err := reg.ListByCommodity(f.ctx, commodityID)
	c.Assert(err, qt.IsNil)
	return links
}

// New links land at the bottom of the list. The FE's "add link" CTA relies on
// it, and a link that appears in the middle of the list looks like the user
// edited the wrong row.
func TestSupplyLinkService_Create_Appends(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)
	commodityID := fx.seedCommodity(c, "coffee")

	first, err := fx.svc.Create(fx.ctx, fx.newLink(commodityID, "Filter", "https://example.com/filter"))
	c.Assert(err, qt.IsNil)
	c.Assert(first.SortOrder, qt.Equals, 0)

	second, err := fx.svc.Create(fx.ctx, fx.newLink(commodityID, "Descaler", "https://example.com/descaler"))
	c.Assert(err, qt.IsNil)
	c.Assert(second.SortOrder, qt.Equals, 1)
}

// An explicit position is honored — the reorder path sets one, and silently
// overwriting it would renumber a list the caller had just arranged.
func TestSupplyLinkService_Create_KeepsAnExplicitSortOrder(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)
	commodityID := fx.seedCommodity(c, "coffee")

	link := fx.newLink(commodityID, "Filter", "https://example.com/filter")
	link.SortOrder = 7
	created, err := fx.svc.Create(fx.ctx, link)
	c.Assert(err, qt.IsNil)
	c.Assert(created.SortOrder, qt.Equals, 7)
}

// The parent check is what turns "another group's commodity" and "no such
// commodity" into the same answer. Without it the write reaches the registry
// and RLS rejects it with something the frontend cannot render.
func TestSupplyLinkService_Create_RejectsAnUnreachableCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)

	_, err := fx.svc.Create(fx.ctx, fx.newLink("c-missing", "Filter", "https://example.com/filter"))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	c.Assert(fx.list(c, "c-missing"), qt.HasLen, 0)
}

func TestSupplyLinkService_Create_RejectsAnEmptyCommodityID(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)

	_, err := fx.svc.Create(fx.ctx, fx.newLink("", "Filter", "https://example.com/filter"))
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
}

func TestSupplyLinkService_Update_AppliesASparsePatch(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)
	commodityID := fx.seedCommodity(c, "coffee")

	link := fx.newLink(commodityID, "Filter", "https://example.com/filter")
	link.Notes = "buy the 2-pack"
	created, err := fx.svc.Create(fx.ctx, link)
	c.Assert(err, qt.IsNil)

	label := "Water filter"
	updated, err := fx.svc.Update(fx.ctx, created.ID, services.SupplyLinkPatch{Label: &label})
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Label, qt.Equals, "Water filter")
	// A nil field is "leave it alone". Treating it as "clear" would wipe the
	// notes every time the user renamed a link.
	c.Assert(updated.URL, qt.Equals, "https://example.com/filter")
	c.Assert(updated.Notes, qt.Equals, "buy the 2-pack")

	// An empty string is a value, not an absence: this is how the user clears
	// the notes field.
	empty := ""
	cleared, err := fx.svc.Update(fx.ctx, created.ID, services.SupplyLinkPatch{Notes: &empty})
	c.Assert(err, qt.IsNil)
	c.Assert(cleared.Notes, qt.Equals, "")
	c.Assert(cleared.Label, qt.Equals, "Water filter")
}

// The URL is rendered as an anchor, so a relative one silently breaks. The
// patch path has to validate, not just the create path.
func TestSupplyLinkService_Update_ValidatesThePatchedRow(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)
	commodityID := fx.seedCommodity(c, "coffee")

	created, err := fx.svc.Create(fx.ctx, fx.newLink(commodityID, "Filter", "https://example.com/filter"))
	c.Assert(err, qt.IsNil)

	bad := "not-a-url"
	_, err = fx.svc.Update(fx.ctx, created.ID, services.SupplyLinkPatch{URL: &bad})
	c.Assert(err, qt.IsNotNil)

	// The rejected patch must not have been written.
	links := fx.list(c, commodityID)
	c.Assert(links, qt.HasLen, 1)
	c.Assert(links[0].URL, qt.Equals, "https://example.com/filter")
}

func TestSupplyLinkService_Update_UnknownID(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)

	label := "Anything"
	_, err := fx.svc.Update(fx.ctx, "nope", services.SupplyLinkPatch{Label: &label})
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestSupplyLinkService_Delete(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)
	commodityID := fx.seedCommodity(c, "coffee")

	created, err := fx.svc.Create(fx.ctx, fx.newLink(commodityID, "Filter", "https://example.com/filter"))
	c.Assert(err, qt.IsNil)

	c.Assert(fx.svc.Delete(fx.ctx, created.ID), qt.IsNil)
	c.Assert(fx.list(c, commodityID), qt.HasLen, 0)
	c.Assert(fx.svc.Delete(fx.ctx, created.ID), qt.ErrorIs, registry.ErrNotFound)
}

func TestSupplyLinkService_Reorder(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)
	commodityID := fx.seedCommodity(c, "coffee")

	var ids []string
	for _, label := range []string{"A", "B", "C"} {
		created, err := fx.svc.Create(fx.ctx, fx.newLink(commodityID, label, "https://example.com/"+label))
		c.Assert(err, qt.IsNil)
		ids = append(ids, created.ID)
	}

	c.Assert(fx.svc.Reorder(fx.ctx, commodityID, []string{ids[2], ids[0], ids[1]}), qt.IsNil)

	links := fx.list(c, commodityID)
	c.Assert(links, qt.HasLen, 3)
	// Dense 0..N-1, in the order asked for. Gaps would make the next append
	// land in the middle.
	c.Assert(links[0].Label, qt.Equals, "C")
	c.Assert(links[0].SortOrder, qt.Equals, 0)
	c.Assert(links[1].Label, qt.Equals, "A")
	c.Assert(links[1].SortOrder, qt.Equals, 1)
	c.Assert(links[2].Label, qt.Equals, "B")
	c.Assert(links[2].SortOrder, qt.Equals, 2)
}

// Anything that is not a full permutation is refused. A partial reorder would
// leave the omitted rows holding positions that no longer interleave with the
// renumbered ones, which is a list the user cannot fix from the UI.
func TestSupplyLinkService_Reorder_RefusesAnythingButAPermutation(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)
	commodityID := fx.seedCommodity(c, "coffee")
	otherID := fx.seedCommodity(c, "kettle")

	var ids []string
	for _, label := range []string{"A", "B"} {
		created, err := fx.svc.Create(fx.ctx, fx.newLink(commodityID, label, "https://example.com/"+label))
		c.Assert(err, qt.IsNil)
		ids = append(ids, created.ID)
	}
	foreign, err := fx.svc.Create(fx.ctx, fx.newLink(otherID, "Other", "https://example.com/other"))
	c.Assert(err, qt.IsNil)

	for name, order := range map[string][]string{
		"too short":                 {ids[0]},
		"duplicate id":              {ids[0], ids[0]},
		"another commodity's":       {ids[0], foreign.ID},
		"an id that does not exist": {ids[0], "ghost"},
	} {
		c.Assert(fx.svc.Reorder(fx.ctx, commodityID, order), qt.ErrorIs, registry.ErrNotFound,
			qt.Commentf("reorder with %s", name))
	}

	// The refused calls left the original order alone.
	links := fx.list(c, commodityID)
	c.Assert(links, qt.HasLen, 2)
	c.Assert(links[0].Label, qt.Equals, "A")
	c.Assert(links[1].Label, qt.Equals, "B")
}

func TestSupplyLinkService_Reorder_RejectsAnUnreachableCommodity(t *testing.T) {
	c := qt.New(t)
	fx := newSupplyLinkFixture(c)

	c.Assert(fx.svc.Reorder(fx.ctx, "c-missing", nil), qt.ErrorIs, registry.ErrNotFound)
}
