package restore_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/processor"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// restoreTagsXML is a minimal full-replace payload that restores one location,
// one area, and two commodities carrying four distinct tag slugs. Two of the
// slugs (`electronics`, `entertainment`) are pre-seeded as `tags` rows by the
// test; the remaining two (`new-tag-a`, `new-tag-b`) must be auto-created by
// the restore path.
const restoreTagsXML = `<?xml version="1.0" encoding="UTF-8"?>
<inventory>
  <locations>
    <location id="11111111-1111-1111-1111-111111111111">
      <locationName>Home</locationName>
      <address>123 Main St</address>
    </location>
  </locations>
  <areas>
    <area id="22222222-2222-2222-2222-222222222222">
      <areaName>Living Room</areaName>
      <locationId>11111111-1111-1111-1111-111111111111</locationId>
    </area>
  </areas>
  <commodities>
    <commodity id="33333333-3333-3333-3333-333333333333">
      <commodityName>Smart TV</commodityName>
      <shortName>TV</shortName>
      <type>electronics</type>
      <areaId>22222222-2222-2222-2222-222222222222</areaId>
      <count>1</count>
      <originalPrice>1000</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <convertedOriginalPrice>0</convertedOriginalPrice>
      <currentPrice>900</currentPrice>
      <serialNumber>TV-1</serialNumber>
      <extraSerialNumbers></extraSerialNumbers>
      <partNumbers></partNumbers>
      <tags>
        <tag>electronics</tag>
        <tag>new-tag-a</tag>
      </tags>
      <status>in_use</status>
      <purchaseDate>2024-01-15</purchaseDate>
      <registeredDate>2024-01-16</registeredDate>
      <urls></urls>
      <comments></comments>
      <draft>false</draft>
    </commodity>
    <commodity id="44444444-4444-4444-4444-444444444444">
      <commodityName>Sound Bar</commodityName>
      <shortName>SB</shortName>
      <type>electronics</type>
      <areaId>22222222-2222-2222-2222-222222222222</areaId>
      <count>1</count>
      <originalPrice>300</originalPrice>
      <originalPriceCurrency>USD</originalPriceCurrency>
      <convertedOriginalPrice>0</convertedOriginalPrice>
      <currentPrice>250</currentPrice>
      <serialNumber>SB-1</serialNumber>
      <extraSerialNumbers></extraSerialNumbers>
      <partNumbers></partNumbers>
      <tags>
        <tag>entertainment</tag>
        <tag>new-tag-b</tag>
      </tags>
      <status>in_use</status>
      <purchaseDate>2024-02-01</purchaseDate>
      <registeredDate>2024-02-02</registeredDate>
      <urls></urls>
      <comments></comments>
      <draft>false</draft>
    </commodity>
  </commodities>
</inventory>`

// setupRestoreTagsTest stamps a tenant + user + group, ensures the user/group
// context is set on ctx, and returns the factory plus the prepared context.
// The group uses USD as main currency so the restored commodities (priced in
// USD) skip the converted-price validation entirely.
func setupRestoreTagsTest(c *qt.C) (*registry.FactorySet, models.User, *models.LocationGroup) {
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "tenant-1487",
			EntityID: models.EntityID{ID: "user-1487"},
		},
		Email: "1487@example.com",
		Name:  "Restore Tags User",
	}
	tenant := models.Tenant{
		EntityID: models.EntityID{ID: "tenant-1487"},
		Name:     "Tenant 1487",
	}

	factorySet := memory.NewFactorySet()
	must.Must(factorySet.TenantRegistry.Create(c.Context(), tenant))
	createdUser := must.Must(factorySet.UserRegistry.Create(c.Context(), user))

	ctx := appctx.WithUser(c.Context(), createdUser)
	slug := must.Must(models.GenerateGroupSlug())
	group := must.Must(factorySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: createdUser.TenantID},
		Name:                "Group 1487",
		Slug:                slug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           createdUser.ID,
		MainCurrency:        models.Currency("USD"),
	}))
	must.Must(factorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: createdUser.TenantID},
		GroupID:             group.ID,
		MemberUserID:        createdUser.ID,
		Role:                models.GroupRoleAdmin,
	}))
	return factorySet, *createdUser, group
}

// TestRestore_AutoCreatesTagsForNewSlugs verifies issue #1487's primary AC:
// a restore that mixes pre-existing slugs with new ones produces exactly the
// missing tag rows, and leaves the pre-existing rows alone.
func TestRestore_AutoCreatesTagsForNewSlugs(t *testing.T) {
	c := qt.New(t)

	factorySet, user, group := setupRestoreTagsTest(c)
	ctx := appctx.WithGroup(appctx.WithUser(c.Context(), &user), group)

	// Pre-seed two tags with non-default colors so we can detect any churn
	// (color or label) caused by the restore path mistakenly re-creating
	// them via Update.
	tagReg := must.Must(factorySet.TagRegistryFactory.CreateUserRegistry(ctx))
	preExisting := []models.Tag{
		{Slug: "electronics", Label: "Electronics (curated)", Color: models.TagColorBlue},
		{Slug: "entertainment", Label: "Entertainment (curated)", Color: models.TagColorGreen},
	}
	for _, t := range preExisting {
		must.Must(tagReg.Create(ctx, t))
	}

	entityService := services.NewEntityService(factorySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op-1487", factorySet, entityService, "")

	stats, err := proc.RestoreFromXML(ctx, strings.NewReader(restoreTagsXML), types.RestoreOptions{
		Strategy: types.RestoreStrategyFullReplace,
		DryRun:   false,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))

	// After the restore: exactly four tag rows total — the two pre-existing
	// ones plus the two new auto-created ones. No duplicates.
	all, err := tagReg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 4)

	bySlug := make(map[string]*models.Tag, len(all))
	for _, tag := range all {
		bySlug[tag.Slug] = tag
	}

	// Pre-existing rows preserved verbatim.
	c.Assert(bySlug["electronics"], qt.IsNotNil)
	c.Assert(bySlug["electronics"].Label, qt.Equals, "Electronics (curated)")
	c.Assert(bySlug["electronics"].Color, qt.Equals, models.TagColorBlue)
	c.Assert(bySlug["entertainment"], qt.IsNotNil)
	c.Assert(bySlug["entertainment"].Label, qt.Equals, "Entertainment (curated)")
	c.Assert(bySlug["entertainment"].Color, qt.Equals, models.TagColorGreen)

	// New rows created with default color and a label derived from the slug.
	c.Assert(bySlug["new-tag-a"], qt.IsNotNil)
	c.Assert(bySlug["new-tag-a"].Color, qt.Equals, models.DefaultTagColor)
	c.Assert(bySlug["new-tag-a"].Label, qt.Equals, "New Tag A")
	c.Assert(bySlug["new-tag-b"], qt.IsNotNil)
	c.Assert(bySlug["new-tag-b"].Color, qt.Equals, models.DefaultTagColor)
	c.Assert(bySlug["new-tag-b"].Label, qt.Equals, "New Tag B")
}

// TestRestore_AutoCreateTagsIsIdempotent verifies AC #2: re-running the same
// restore is a no-op on the tags table (no duplicate-key errors, no churn).
func TestRestore_AutoCreateTagsIsIdempotent(t *testing.T) {
	c := qt.New(t)

	factorySet, user, group := setupRestoreTagsTest(c)
	ctx := appctx.WithGroup(appctx.WithUser(c.Context(), &user), group)

	entityService := services.NewEntityService(factorySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op-1487-idempotent", factorySet, entityService, "")

	for range 2 {
		stats, err := proc.RestoreFromXML(ctx, strings.NewReader(restoreTagsXML), types.RestoreOptions{
			Strategy: types.RestoreStrategyFullReplace,
			DryRun:   false,
		})
		c.Assert(err, qt.IsNil)
		c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))
	}

	tagReg := must.Must(factorySet.TagRegistryFactory.CreateUserRegistry(ctx))
	all, err := tagReg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 4, qt.Commentf("expected exactly 4 tag rows after two restores, got %d", len(all)))
}

// TestRestore_MergeAddSkipsTagAutoCreate verifies that MergeAdd does not
// auto-create tag rows for commodities that are skipped because they
// already exist. The hook fires on the actual persistence path, not before
// strategy dispatch — otherwise a MergeAdd preview would leak unused tag
// rows into the destination group for every existing commodity.
func TestRestore_MergeAddSkipsTagAutoCreate(t *testing.T) {
	c := qt.New(t)

	factorySet, user, group := setupRestoreTagsTest(c)
	ctx := appctx.WithGroup(appctx.WithUser(c.Context(), &user), group)

	// Pre-seed the same location, area and the two commodities that the XML
	// payload references. With MergeAdd, the restore should classify both
	// commodities as "skip" and never reach the persistence path — so none
	// of the XML's tag slugs should produce tag rows.
	locReg := must.Must(factorySet.LocationRegistryFactory.CreateUserRegistry(ctx))
	createdLoc := must.Must(locReg.Create(ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{UUID: "11111111-1111-1111-1111-111111111111"},
			TenantID:        user.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name:    "Home",
		Address: "123 Main St",
	}))
	areaReg := must.Must(factorySet.AreaRegistryFactory.CreateUserRegistry(ctx))
	createdArea := must.Must(areaReg.Create(ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{UUID: "22222222-2222-2222-2222-222222222222"},
			TenantID:        user.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name:       "Living Room",
		LocationID: createdLoc.ID,
	}))
	comReg := must.Must(factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx))
	for _, uuid := range []string{
		"33333333-3333-3333-3333-333333333333",
		"44444444-4444-4444-4444-444444444444",
	} {
		must.Must(comReg.Create(ctx, models.Commodity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
				EntityID:        models.EntityID{UUID: uuid},
				TenantID:        user.TenantID,
				GroupID:         group.ID,
				CreatedByUserID: user.ID,
			},
			Name:   "Pre-existing " + uuid,
			AreaID: createdArea.ID,
		}))
	}

	entityService := services.NewEntityService(factorySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op-1487-mergeadd-skip", factorySet, entityService, "")

	stats, err := proc.RestoreFromXML(ctx, strings.NewReader(restoreTagsXML), types.RestoreOptions{
		Strategy: types.RestoreStrategyMergeAdd,
		DryRun:   false,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))
	c.Assert(stats.SkippedCount, qt.Not(qt.Equals), 0, qt.Commentf("expected the pre-existing commodities to be skipped"))

	tagReg := must.Must(factorySet.TagRegistryFactory.CreateUserRegistry(ctx))
	tags, err := tagReg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(tags, qt.HasLen, 0, qt.Commentf("MergeAdd skipped both commodities; no tag rows should be auto-created, got %d", len(tags)))
}

// TestRestore_DryRunDoesNotAutoCreateTags verifies that DryRun mode previews
// the restore without mutating the tags table. Uses MergeAdd with the target
// location + area pre-seeded so the commodity references resolve under dry
// run (FullReplace + DryRun does not exercise child-entity references in the
// in-memory `existing` map — that's a pre-existing limitation of the
// processor and orthogonal to the tag hook this test guards).
func TestRestore_DryRunDoesNotAutoCreateTags(t *testing.T) {
	c := qt.New(t)

	factorySet, user, group := setupRestoreTagsTest(c)
	ctx := appctx.WithGroup(appctx.WithUser(c.Context(), &user), group)

	// Pre-seed the location and area whose UUIDs the XML references so
	// dry-run validation passes without running the actual create branch.
	locReg := must.Must(factorySet.LocationRegistryFactory.CreateUserRegistry(ctx))
	must.Must(locReg.Create(ctx, models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{UUID: "11111111-1111-1111-1111-111111111111"},
			TenantID:        user.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name:    "Home",
		Address: "123 Main St",
	}))
	areaReg := must.Must(factorySet.AreaRegistryFactory.CreateUserRegistry(ctx))
	createdLocs := must.Must(locReg.List(ctx))
	c.Assert(createdLocs, qt.HasLen, 1)
	must.Must(areaReg.Create(ctx, models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID:        models.EntityID{UUID: "22222222-2222-2222-2222-222222222222"},
			TenantID:        user.TenantID,
			GroupID:         group.ID,
			CreatedByUserID: user.ID,
		},
		Name:       "Living Room",
		LocationID: createdLocs[0].ID,
	}))

	entityService := services.NewEntityService(factorySet, "")
	proc := processor.NewRestoreOperationProcessor("test-op-1487-dryrun", factorySet, entityService, "")

	stats, err := proc.RestoreFromXML(ctx, strings.NewReader(restoreTagsXML), types.RestoreOptions{
		Strategy: types.RestoreStrategyMergeAdd,
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(stats.ErrorCount, qt.Equals, 0, qt.Commentf("restore errors: %v", stats.Errors))

	tagReg := must.Must(factorySet.TagRegistryFactory.CreateUserRegistry(ctx))
	all, err := tagReg.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 0, qt.Commentf("dry-run should not create tag rows, got %d", len(all)))
}
