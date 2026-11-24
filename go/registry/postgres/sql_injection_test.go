package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

// Every list query that takes a value from a request gets the injection
// payloads in every such slot. Each call has to succeed and match nothing,
// since no row contains the payload: a payload that escaped its argument would
// either fail to parse or widen the match. Afterwards the seeded rows are all
// still there, and a value with quotes in it still finds the row it names.
func TestListQueries_Postgres_TreatInjectionPayloadsAsData(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA
	set := fx.groupASet
	serviceSet := fx.factorySet.CreateServiceRegistrySet()

	seedTagCommodity(c, set, ctx, fx.areaAID, "O'Brien's drill")
	seedTagFile(c, set, ctx, "O'Brien manual")
	_, err := set.TagRegistry.Create(ctx, models.Tag{
		Kind:  models.TagKindCommodity,
		Slug:  "obrien",
		Label: "O'Brien",
		Color: models.DefaultTagColor,
	})
	c.Assert(err, qt.IsNil)

	commodities, err := set.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	tags, err := set.TagRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)

	for _, payload := range postgres.SQLInjectionPayloads() {
		c.Run(payload, func(c *qt.C) {
			assertNoCommodities(c, set, ctx, payload)
			assertNoFiles(c, set, ctx, payload)
			assertNoTagsOrEvents(c, set, ctx, payload)
			assertNoAdminRows(c, set, fx.user.TenantID, payload)

			areas, total, err := set.AreaRegistry.ListPaginated(ctx, 0, 10, registry.AreaListOptions{LocationID: payload})
			c.Assert(err, qt.IsNil)
			c.Check(total, qt.Equals, 0)
			c.Check(areas, qt.HasLen, 0)

			schedules, total, err := set.MaintenanceScheduleRegistry.ListPaginated(ctx, 0, 10, registry.MaintenanceListOptions{DueBefore: payload})
			c.Assert(err, qt.IsNil)
			c.Check(total, qt.Equals, 0)
			c.Check(schedules, qt.HasLen, 0)

			// A purged id that names no group excludes nothing, so the user
			// still owns content; a tenant and user that do not exist own none.
			owns, err := fx.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(context.Background(), fx.user.TenantID, fx.user.ID, []string{payload})
			c.Assert(err, qt.IsNil)
			c.Check(owns, qt.IsTrue)
			owns, err = fx.factorySet.UserContentOwnershipChecker.HasRetainedOwnedContent(context.Background(), payload, payload, []string{payload})
			c.Assert(err, qt.IsNil)
			c.Check(owns, qt.IsFalse)

			cursor := registry.OrphanCandidateCursor{CreatedAt: time.Now().Add(-time.Hour), ID: payload}
			orphans, err := serviceSet.FileRegistry.ListOrphanCandidates(context.Background(), time.Now(), cursor, 10)
			c.Assert(err, qt.IsNil)
			c.Check(orphans, qt.HasLen, 0)
		})
	}

	commoditiesAfter, err := set.CommodityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(commoditiesAfter, qt.Equals, commodities)
	tagsAfter, err := set.TagRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Check(tagsAfter, qt.Equals, tags)

	found, total, err := set.CommodityRegistry.ListPaginated(ctx, 0, 10, registry.CommodityListOptions{Search: "o'brien"})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 1)
	c.Assert(found, qt.HasLen, 1)
	c.Check(found[0].Name, qt.Equals, "O'Brien's drill")

	files, err := set.FileRegistry.Search(ctx, "O'Brien", nil, nil, nil, nil, nil)
	c.Assert(err, qt.IsNil)
	c.Check(files, qt.HasLen, 1)

	tagHits, err := set.TagRegistry.Search(ctx, "O'Brien", 10, models.TagKindCommodity)
	c.Assert(err, qt.IsNil)
	c.Check(tagHits, qt.HasLen, 1)
}

func assertNoCommodities(c *qt.C, set *registry.Set, ctx context.Context, payload string) {
	c.Helper()
	for _, opts := range []registry.CommodityListOptions{
		{Search: payload},
		{AreaID: payload},
		{Types: []models.CommodityType{models.CommodityType(payload)}, IncludeInactive: true},
		{Statuses: []models.CommodityStatus{models.CommodityStatus(payload)}},
		{WarrantyExpiresBefore: payload},
	} {
		got, total, err := set.CommodityRegistry.ListPaginated(ctx, 0, 10, opts)
		c.Assert(err, qt.IsNil)
		c.Check(total, qt.Equals, 0)
		c.Check(got, qt.HasLen, 0)
	}
}

func assertNoFiles(c *qt.C, set *registry.Set, ctx context.Context, payload string) {
	c.Helper()
	files, err := set.FileRegistry.Search(ctx, payload, nil, nil, nil, nil, nil)
	c.Assert(err, qt.IsNil)
	c.Check(files, qt.HasLen, 0)

	files, err = set.FileRegistry.Search(ctx, "", nil, nil, []string{payload}, nil, nil)
	c.Assert(err, qt.IsNil)
	c.Check(files, qt.HasLen, 0)

	fileType := models.FileType(payload)
	files, total, err := set.FileRegistry.ListPaginated(ctx, 0, 10, &fileType, nil, &payload, &payload)
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 0)
	c.Check(files, qt.HasLen, 0)

	counts, _, err := set.FileRegistry.CountByCategory(ctx, payload, nil, []string{payload}, nil, nil)
	c.Assert(err, qt.IsNil)
	for category, n := range counts {
		c.Check(n, qt.Equals, 0, qt.Commentf("category %s", category))
	}
}

func assertNoTagsOrEvents(c *qt.C, set *registry.Set, ctx context.Context, payload string) {
	c.Helper()
	tags, total, err := set.TagRegistry.ListPaginated(ctx, 0, 10, registry.TagListOptions{Search: payload})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 0)
	c.Check(tags, qt.HasLen, 0)

	tags, err = set.TagRegistry.Search(ctx, payload, 10, models.TagKindCommodity)
	c.Assert(err, qt.IsNil)
	c.Check(tags, qt.HasLen, 0)

	events, total, err := set.CommodityEventRegistry.ListByCommodity(ctx, payload, 0, 10, registry.CommodityEventListOptions{
		Kinds: []models.CommodityEventKind{models.CommodityEventKind(payload)},
	})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 0)
	c.Check(events, qt.HasLen, 0)
}

func assertNoAdminRows(c *qt.C, set *registry.Set, tenantID, payload string) {
	c.Helper()
	ctx := context.Background()

	tenants, total, err := set.TenantRegistry.ListAdmin(ctx, registry.AdminTenantListOptions{Query: payload})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 0)
	c.Check(tenants, qt.HasLen, 0)

	users, total, err := set.UserRegistry.ListAdminByTenant(ctx, tenantID, registry.AdminUserListOptions{Query: payload})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 0)
	c.Check(users, qt.HasLen, 0)

	users, total, err = set.UserRegistry.ListAdminByTenant(ctx, payload, registry.AdminUserListOptions{})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 0)
	c.Check(users, qt.HasLen, 0)

	for _, opts := range []registry.AdminGroupListOptions{
		{Query: payload},
		{TenantID: payload},
		{Status: payload},
	} {
		groups, total, err := set.LocationGroupRegistry.ListAdmin(ctx, opts)
		c.Assert(err, qt.IsNil)
		c.Check(total, qt.Equals, 0)
		c.Check(groups, qt.HasLen, 0)
	}
}
