package services_test

import (
	"context"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
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

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"Kitchen", "front office", "kitchen"})
	c.Assert(err, qt.IsNil)
	c.Assert(tags, qt.HasLen, 2) // duplicate Kitchen / kitchen folds in
	c.Assert(tags["kitchen"].Color, qt.Equals, models.DefaultTagColor)
	// The first spelling names the tag; the label is the text as typed.
	c.Assert(tags["kitchen"].Label, qt.Equals, "Kitchen")
	c.Assert(tags["front-office"].Slug, qt.Equals, "front-office")
	c.Assert(tags["front-office"].Label, qt.Equals, "front office")

	// Idempotent — second call returns the same rows.
	tags2, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"kitchen", "front-office"})
	c.Assert(err, qt.IsNil)
	c.Assert(tags2["kitchen"].ID, qt.Equals, tags["kitchen"].ID)
	c.Assert(tags2["front-office"].ID, qt.Equals, tags["front-office"].ID)
}

func TestTagService_EnsureTagsExist_FiltersEmpty(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"   ", "###", ""})
	c.Assert(err, qt.IsNil)
	c.Assert(tags, qt.HasLen, 0)
}

func TestTagService_NormalizeAndEnsureSlugs(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	slugs, err := svc.NormalizeAndEnsureSlugs(ctx, models.TagKindCommodity, []string{"Kitchen", "Kitchen", "front office"})
	c.Assert(err, qt.IsNil)
	c.Assert(slugs, qt.HasLen, 2)
	c.Assert(slugs, qt.Contains, "kitchen")
	c.Assert(slugs, qt.Contains, "front-office")
}

// Every tag below must survive a save: the slug keeps the letters of any
// script, two tags that differ only in non-Latin letters stay apart, and the
// label is the text the user typed.
func TestTagService_NormalizeAndEnsureSlugs_KeepsEveryScript(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	typed := []string{"кухня", "на продажу", "書類", "Kuchyně", "Čaj", "électronique", "USB кабель", "USB хаб", "fragile"}
	slugs, err := svc.NormalizeAndEnsureSlugs(ctx, models.TagKindCommodity, typed)
	c.Assert(err, qt.IsNil)
	c.Assert(slugs, qt.DeepEquals, []string{
		"кухня", "на-продажу", "書類", "kuchyně", "čaj", "électronique", "usb-кабель", "usb-хаб", "fragile",
	})

	tagReg := must.Must(fs.TagRegistryFactory.CreateUserRegistry(ctx))
	for i, slug := range slugs {
		tag, err := tagReg.GetBySlug(ctx, models.TagKindCommodity, slug)
		c.Assert(err, qt.IsNil, qt.Commentf("slug %q", slug))
		c.Assert(tag.Label, qt.Equals, typed[i], qt.Commentf("slug %q", slug))
	}

	// Saving the stored slugs again, as the item form does when it is
	// reopened and saved unchanged, finds the same rows.
	again, err := svc.NormalizeAndEnsureSlugs(ctx, models.TagKindCommodity, slugs)
	c.Assert(err, qt.IsNil)
	c.Assert(again, qt.DeepEquals, slugs)
	all := must.Must(tagReg.List(ctx))
	c.Assert(all, qt.HasLen, len(typed))
}

// Precomposed and decomposed spellings of the same word are one tag, and the
// label is stored in NFC.
func TestTagService_NormalizeAndEnsureSlugs_FoldsUnicodeSpellings(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	slugs, err := svc.NormalizeAndEnsureSlugs(ctx, models.TagKindCommodity, []string{"Kuchyne\u030c", "Kuchyn\u011b", "KUCHYNĚ"})
	c.Assert(err, qt.IsNil)
	c.Assert(slugs, qt.DeepEquals, []string{"kuchyn\u011b"})

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, slugs)
	c.Assert(err, qt.IsNil)
	c.Assert(tags["kuchyn\u011b"].Label, qt.Equals, "Kuchyn\u011b")
}

// Typed text can be longer than the label limit while its slug fits, because
// the slug drops spaces and punctuation. The label is cut to the limit rather
// than failing the save.
func TestTagService_NormalizeAndEnsureSlugs_CapsLongTypedLabel(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	typed := strings.TrimSpace(strings.Repeat("ж, ", 22))     // 65 runes
	slug := strings.TrimSuffix(strings.Repeat("ж-", 22), "-") // 43 runes
	slugs, err := svc.NormalizeAndEnsureSlugs(ctx, models.TagKindCommodity, []string{typed})
	c.Assert(err, qt.IsNil)
	c.Assert(slugs, qt.DeepEquals, []string{slug})

	tagReg := must.Must(fs.TagRegistryFactory.CreateUserRegistry(ctx))
	tag, err := tagReg.GetBySlug(ctx, models.TagKindCommodity, slug)
	c.Assert(err, qt.IsNil)
	c.Assert(tag.Label, qt.Equals, string([]rune(typed)[:models.MaxTagLabelRunes]))
}

// Tag references read back from a backup are slugs; a tag created for one
// gets a label title-cased from the slug in any script.
func TestTagService_NormalizeAndEnsureRestoredSlugs_LabelsFromSlug(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	slugs, err := svc.NormalizeAndEnsureRestoredSlugs(ctx, models.TagKindCommodity,
		[]string{"new-tag-a", "на-продажу", "書類", "Legacy Free Text"})
	c.Assert(err, qt.IsNil)
	c.Assert(slugs, qt.DeepEquals, []string{"new-tag-a", "на-продажу", "書類", "legacy-free-text"})

	tagReg := must.Must(fs.TagRegistryFactory.CreateUserRegistry(ctx))
	want := map[string]string{
		"new-tag-a":        "New Tag A",
		"на-продажу":       "На Продажу",
		"書類":               "書類",
		"legacy-free-text": "Legacy Free Text",
	}
	for slug, label := range want {
		tag, err := tagReg.GetBySlug(ctx, models.TagKindCommodity, slug)
		c.Assert(err, qt.IsNil, qt.Commentf("slug %q", slug))
		c.Assert(tag.Label, qt.Equals, label, qt.Commentf("slug %q", slug))
	}
}

func TestTagService_RenameTag_RewritesReferences(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"kitchen"})
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

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"kitchen", "bedroom"})
	c.Assert(err, qt.IsNil)

	_, err = svc.RenameTag(ctx, tags["kitchen"].ID, "", "bedroom", "")
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, registry.ErrAlreadyExists)
}

func TestTagService_DeleteTag_RefusesWhenInUse(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"kitchen"})
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
	c.Assert(err, qt.ErrorIs, services.ErrTagInUse)
	c.Assert(usage.Commodities, qt.Equals, 1)
	c.Assert(usage.Files, qt.Equals, 0)

	// Tag still exists.
	_, err = fs.TagRegistryFactory.MustCreateUserRegistry(ctx).GetBySlug(ctx, models.TagKindCommodity, "kitchen")
	c.Assert(err, qt.IsNil)
}

func TestTagService_DeleteTag_ForceStripsAndDeletes(t *testing.T) {
	c := qt.New(t)
	ctx, fs := newTagServiceFixture(c)
	svc := services.NewTagService(fs)

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"kitchen"})
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
	_, err = fs.TagRegistryFactory.MustCreateUserRegistry(ctx).GetBySlug(ctx, models.TagKindCommodity, "kitchen")
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

	tags, err := svc.EnsureTagsExist(ctx, models.TagKindCommodity, []string{"kitchen"})
	c.Assert(err, qt.IsNil)

	usage, err := svc.DeleteTag(ctx, tags["kitchen"].ID, false)
	c.Assert(err, qt.IsNil)
	c.Assert(usage.Commodities, qt.Equals, 0)
	c.Assert(usage.Files, qt.Equals, 0)
}
