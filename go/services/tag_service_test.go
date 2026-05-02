package services_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func newTagServiceFixture(c *qt.C) (context.Context, *registry.FactorySet) {
	c.Helper()
	fs := memory.NewFactorySet()

	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "group-1"},
			TenantID: "tenant-1",
		},
		Slug: "g1",
	})
	return ctx, fs
}

func TestTagService_EnsureTagsExist_AutoCreates(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, []string{"Kitchen", "front office", "kitchen"})
	c.Assert(err, qt.IsNil)
	c.Assert(tags, qt.HasLen, 2) // duplicate Kitchen / kitchen folds in
	c.Assert(tags["kitchen"].Color, qt.Equals, models.DefaultTagColor)
	c.Assert(tags["kitchen"].Label, qt.Equals, "Kitchen")
	c.Assert(tags["front-office"].Slug, qt.Equals, "front-office")
	c.Assert(tags["front-office"].Label, qt.Equals, "Front Office")

	// Idempotent — second call returns the same rows.
	tags2, err := svc.EnsureTagsExist(ctx, []string{"kitchen", "front-office"})
	c.Assert(err, qt.IsNil)
	c.Assert(tags2["kitchen"].ID, qt.Equals, tags["kitchen"].ID)
	c.Assert(tags2["front-office"].ID, qt.Equals, tags["front-office"].ID)
}

func TestTagService_EnsureTagsExist_FiltersEmpty(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, []string{"   ", "###", ""})
	c.Assert(err, qt.IsNil)
	c.Assert(tags, qt.HasLen, 0)
}

func TestTagService_NormalizeAndEnsureSlugs(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	slugs, err := svc.NormalizeAndEnsureSlugs(ctx, []string{"Kitchen", "Kitchen", "front office"})
	c.Assert(err, qt.IsNil)
	c.Assert(slugs, qt.HasLen, 2)
	c.Assert(slugs, qt.Contains, "kitchen")
	c.Assert(slugs, qt.Contains, "front-office")
}

func TestTagService_RenameTag_RewritesReferences(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, []string{"kitchen"})
	c.Assert(err, qt.IsNil)

	// Seed a commodity referencing the slug.
	commodityReg, err := fs.CommodityRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	cmd, err := commodityReg.Create(ctx, models.Commodity{
		Name:   "fridge",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeWhiteGoods,
		Tags:   models.ValuerSlice[string]{"kitchen"},
	})
	c.Assert(err, qt.IsNil)

	// Rename the slug.
	updated, err := svc.RenameTag(ctx, tags["kitchen"].ID, "Kitchen Area", "kitchen-area", models.TagColorAmber)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Slug, qt.Equals, "kitchen-area")
	c.Assert(updated.Label, qt.Equals, "Kitchen Area")
	c.Assert(updated.Color, qt.Equals, models.TagColorAmber)

	// Commodity reference is rewritten.
	got, err := commodityReg.Get(ctx, cmd.ID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"kitchen-area"})
}

func TestTagService_RenameTag_ConflictsWithExistingSlug(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, []string{"kitchen", "bedroom"})
	c.Assert(err, qt.IsNil)

	_, err = svc.RenameTag(ctx, tags["kitchen"].ID, "", "bedroom", "")
	c.Assert(err, qt.IsNotNil)
	c.Assert(errors.Is(err, registry.ErrAlreadyExists), qt.IsTrue)
}

func TestTagService_DeleteTag_RefusesWhenInUse(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, []string{"kitchen"})
	c.Assert(err, qt.IsNil)

	commodityReg, err := fs.CommodityRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	_, err = commodityReg.Create(ctx, models.Commodity{
		Name:   "fridge",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeWhiteGoods,
		Tags:   models.ValuerSlice[string]{"kitchen"},
	})
	c.Assert(err, qt.IsNil)

	usage, err := svc.DeleteTag(ctx, tags["kitchen"].ID, false)
	c.Assert(errors.Is(err, services.ErrTagInUse), qt.IsTrue)
	c.Assert(usage.Commodities, qt.Equals, 1)
	c.Assert(usage.Files, qt.Equals, 0)

	// Tag still exists.
	_, err = fs.TagRegistryFactory.MustCreateUserRegistry(ctx).GetBySlug(ctx, "kitchen")
	c.Assert(err, qt.IsNil)
}

func TestTagService_DeleteTag_ForceStripsAndDeletes(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, []string{"kitchen"})
	c.Assert(err, qt.IsNil)

	commodityReg, err := fs.CommodityRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	cmd, err := commodityReg.Create(ctx, models.Commodity{
		Name:   "fridge",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeWhiteGoods,
		Tags:   models.ValuerSlice[string]{"kitchen", "appliance"},
	})
	c.Assert(err, qt.IsNil)

	usage, err := svc.DeleteTag(ctx, tags["kitchen"].ID, true)
	c.Assert(err, qt.IsNil)
	c.Assert(usage.Commodities, qt.Equals, 1)

	// Tag is gone.
	_, err = fs.TagRegistryFactory.MustCreateUserRegistry(ctx).GetBySlug(ctx, "kitchen")
	c.Assert(err, qt.IsNotNil)

	// Commodity reference is stripped, other tags preserved.
	got, err := commodityReg.Get(ctx, cmd.ID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"appliance"})
}

func TestTagService_DeleteTag_NoUsage(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, []string{"kitchen"})
	c.Assert(err, qt.IsNil)

	usage, err := svc.DeleteTag(ctx, tags["kitchen"].ID, false)
	c.Assert(err, qt.IsNil)
	c.Assert(usage.Commodities, qt.Equals, 0)
	c.Assert(usage.Files, qt.Equals, 0)
}
