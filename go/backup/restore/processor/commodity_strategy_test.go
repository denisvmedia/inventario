package processor

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// Commodities are the leaf every file, image, invoice and manual hangs off,
// so their restore strategy has the same shape as the location and area ones
// but a wider blast radius when it picks the wrong branch. #2114 N4 flagged
// updateCommodity at 0%.

// TestCommodityStrategy walks the decision table. Dry-run keeps it cheap for
// the branches that decide rather than write; the write itself is covered
// below against a real in-memory registry.
func TestCommodityStrategy(t *testing.T) {
	existingCom := &models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "db-com-1"},
		},
		Name: "Drill (as stored)",
	}
	existingCom.UUID = "uuid-com-1"

	tests := []struct {
		name     string
		strategy types.RestoreStrategy
		existing *models.Commodity
		updated  int
		skipped  int
	}{
		{"merge add, already there", types.RestoreStrategyMergeAdd, existingCom, 0, 1},
		{"merge update, already there", types.RestoreStrategyMergeUpdate, existingCom, 1, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			stats, existing, idMapping := newStrategyFixture()
			p := &RestoreOperationProcessor{}

			incoming := &models.Commodity{Name: "Drill (from backup)"}
			err := p.applyStrategyForCommodityModel(context.Background(), incoming, tt.existing,
				"uuid-com-1", stats, existing, idMapping, dryRun(tt.strategy))

			c.Assert(err, qt.IsNil)
			c.Check(stats.UpdatedCount, qt.Equals, tt.updated)
			c.Check(stats.SkippedCount, qt.Equals, tt.skipped)
			// A skip is not a restore: it must not be counted as one.
			c.Check(stats.CommodityCount, qt.Equals, tt.updated)
			c.Check(stats.CreatedCount, qt.Equals, 0)
		})
	}
}

// merge_update writes over the row that is already there, which means it has
// to adopt that row's identity rather than the backup's. Writing the backup's
// id would update a row that does not exist, or create a second one, and every
// file, image and invoice pointing at the live commodity would be orphaned.
func TestCommodityMergeUpdateAdoptsTheExistingIdentity(t *testing.T) {
	c := qt.New(t)
	stats, existing, idMapping := newStrategyFixture()
	p := &RestoreOperationProcessor{}

	existingCom := &models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "db-com-1"},
		},
		Name: "Drill (as stored)",
	}
	existingCom.UUID = "uuid-com-1"

	incoming := &models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{ID: "backup-com-9"},
		},
		Name: "Drill (from backup)",
	}
	incoming.UUID = "uuid-from-backup"

	err := p.applyStrategyForCommodityModel(context.Background(), incoming, existingCom,
		"uuid-com-1", stats, existing, idMapping, dryRun(types.RestoreStrategyMergeUpdate))

	c.Assert(err, qt.IsNil)
	c.Check(incoming.ID, qt.Equals, "db-com-1")
	c.Check(incoming.UUID, qt.Equals, "uuid-com-1")
	c.Check(stats.UpdatedCount, qt.Equals, 1)
}

// A dry run is a preview, so it reports what it would do and touches nothing.
// The counters it produces are what the UI shows before the user commits.
func TestUpdateCommodityDryRunWritesNothing(t *testing.T) {
	c := qt.New(t)
	stats, existing, _ := newStrategyFixture()

	// A nil factorySet is the assertion: reaching the registry would panic.
	p := &RestoreOperationProcessor{}
	incoming := &models.Commodity{Name: "Drill (from backup)"}

	err := p.updateCommodity(context.Background(), incoming, "uuid-com-1", stats, existing,
		types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate, DryRun: true})

	c.Assert(err, qt.IsNil)
	c.Check(stats.UpdatedCount, qt.Equals, 1)
	c.Check(stats.CommodityCount, qt.Equals, 1)
	c.Check(existing.Commodities, qt.HasLen, 0)
}

// restoreFixture builds an in-memory tenant with one group, one location and
// one area, and returns a processor wired to it plus the user/group context.
func restoreFixture(c *qt.C) (*RestoreOperationProcessor, context.Context, *models.Area) {
	c.Helper()

	ctx := context.Background()
	factorySet := memory.NewFactorySet()

	tenant := must.Must(factorySet.TenantRegistry.Create(ctx, models.Tenant{
		Name: "Test Organization", Slug: "test-org", Status: models.TenantStatusActive, IsDefault: true,
	}))
	user := must.Must(factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               "restore@example.com",
		Name:                "Restore User",
		IsActive:            true,
	}))
	group := must.Must(factorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Name:                "Test Group",
		Slug:                must.Must(models.GenerateGroupSlug()),
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
		GroupCurrency:       models.Currency("USD"),
	}))

	userCtx := appctx.WithGroup(appctx.WithUser(ctx, user), group)
	registrySet := must.Must(factorySet.CreateUserRegistrySet(userCtx))

	location := must.Must(registrySet.LocationRegistry.Create(userCtx, models.Location{
		Name: "Workshop", Address: "1 Bench Rd",
	}))
	area := must.Must(registrySet.AreaRegistry.Create(userCtx, models.Area{
		Name: "Shelf", LocationID: location.ID,
	}))

	p := &RestoreOperationProcessor{
		factorySet: factorySet,
		tagService: services.NewTagService(factorySet),
	}
	return p, userCtx, area
}

// The write path: the row is really updated, and the caller's map of what
// exists is refreshed so later references resolve to the new row rather than
// the one the backup described.
func TestUpdateCommodityWritesAndRefreshesTheExistingMap(t *testing.T) {
	c := qt.New(t)

	p, ctx, area := restoreFixture(c)
	registrySet := must.Must(p.factorySet.CreateUserRegistrySet(ctx))

	stored := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Drill (as stored)",
		ShortName:             "drill",
		AreaID:                &area.ID,
		Type:                  models.CommodityTypeEquipment,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPriceCurrency: models.Currency("USD"),
	}))

	stats, existing, _ := newStrategyFixture()
	incoming := *stored
	incoming.Name = "Drill (from backup)"

	err := p.updateCommodity(ctx, &incoming, "uuid-com-1", stats, existing,
		types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate})
	c.Assert(err, qt.IsNil)

	c.Check(stats.UpdatedCount, qt.Equals, 1)
	c.Check(stats.CommodityCount, qt.Equals, 1)

	reread := must.Must(registrySet.CommodityRegistry.Get(ctx, stored.ID))
	c.Check(reread.Name, qt.Equals, "Drill (from backup)")

	c.Assert(existing.Commodities["uuid-com-1"], qt.IsNotNil)
	c.Check(existing.Commodities["uuid-com-1"].ID, qt.Equals, stored.ID)
	c.Check(existing.Commodities["uuid-com-1"].Name, qt.Equals, "Drill (from backup)")
}

// A backup carries tag slugs, not tag rows. Restoring into a group that has
// never seen them has to create them, or the commodity comes back referencing
// tags that do not exist and the Tags page shows nothing for them.
func TestUpdateCommodityCreatesTheTagsTheBackupReferences(t *testing.T) {
	c := qt.New(t)

	p, ctx, area := restoreFixture(c)
	registrySet := must.Must(p.factorySet.CreateUserRegistrySet(ctx))

	stored := must.Must(registrySet.CommodityRegistry.Create(ctx, models.Commodity{
		Name:                  "Drill (as stored)",
		ShortName:             "drill",
		AreaID:                &area.ID,
		Type:                  models.CommodityTypeEquipment,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPriceCurrency: models.Currency("USD"),
	}))

	stats, existing, _ := newStrategyFixture()
	incoming := *stored
	incoming.Tags = models.ValuerSlice[string]{"power-tools", "workshop"}

	err := p.updateCommodity(ctx, &incoming, "uuid-com-1", stats, existing,
		types.RestoreOptions{Strategy: types.RestoreStrategyMergeUpdate})
	c.Assert(err, qt.IsNil)

	for _, slug := range []string{"power-tools", "workshop"} {
		tag, terr := registrySet.TagRegistry.GetBySlug(ctx, models.TagKindCommodity, slug)
		c.Check(terr, qt.IsNil, qt.Commentf("tag %q was not created", slug))
		c.Check(tag, qt.IsNotNil)
	}

	reread := must.Must(registrySet.CommodityRegistry.Get(ctx, stored.ID))
	c.Check([]string(reread.Tags), qt.DeepEquals, []string{"power-tools", "workshop"})
}
