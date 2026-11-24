package postgres_test

import (
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// Tags typed in any script survive a save end to end on postgres: the
// service keeps their letters in the slug and their text in the label, the
// commodity insert (which re-locks every referenced tag row in its own tx)
// finds the rows the service created, and the JSONB column round-trips the
// slugs byte for byte.
func TestTagService_Postgres_UnicodeTagsRoundTrip(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	svc := services.NewTagService(fx.factorySet)

	typed := []string{"кухня", "на продажу", "書類", "Kuchyně", "Čaj", "électronique", "USB кабель", "USB хаб", "fragile"}
	slugs, err := svc.NormalizeAndEnsureSlugs(fx.ctxA, models.TagKindCommodity, typed)
	c.Assert(err, qt.IsNil)
	c.Assert(slugs, qt.DeepEquals, []string{
		"кухня", "на-продажу", "書類", "kuchyně", "čaj", "électronique", "usb-кабель", "usb-хаб", "fragile",
	})

	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "kettle", slugs...)
	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(cmd.Tags), qt.DeepEquals, slugs)

	for i, slug := range slugs {
		tag, err := fx.groupASet.TagRegistry.GetBySlug(fx.ctxA, models.TagKindCommodity, slug)
		c.Assert(err, qt.IsNil, qt.Commentf("slug %q", slug))
		c.Assert(tag.Label, qt.Equals, typed[i], qt.Commentf("slug %q", slug))

		// Usage is counted with `tags @> jsonb_build_array(slug)`.
		usage, err := fx.groupASet.TagRegistry.GetUsage(fx.ctxA, models.TagKindCommodity, slug)
		c.Assert(err, qt.IsNil, qt.Commentf("slug %q", slug))
		c.Assert(usage.Commodities, qt.Equals, 1, qt.Commentf("slug %q", slug))
	}

	_, total, err := fx.groupASet.TagRegistry.ListPaginated(fx.ctxA, 0, 100, registry.TagListOptions{Kind: models.TagKindCommodity})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, len(typed))
}

// Rename and force-delete rewrite JSONB arrays with jsonb_agg / jsonb
// containment; a non-ASCII slug has to match and be replaced like any other.
func TestTagRegistry_Postgres_UnicodeSlugRewriteAndStrip(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	cmdID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "table", "кухня", "書類", "fragile")

	commCount, _, err := fx.groupASet.TagRegistry.RewriteSlugReferences(fx.ctxA, models.TagKindCommodity, "кухня", "столовая")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)

	commCount, _, err = fx.groupASet.TagRegistry.StripSlugReferences(fx.ctxA, models.TagKindCommodity, "書類")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)

	cmd, err := fx.groupASet.CommodityRegistry.Get(fx.ctxA, cmdID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(cmd.Tags), qt.HasLen, 2)
	c.Assert([]string(cmd.Tags), qt.Contains, "столовая")
	c.Assert([]string(cmd.Tags), qt.Contains, "fragile")
}

// The Files page filters with `tags @> $1::jsonb`; the JSON the registry
// builds for a Cyrillic slug has to match the stored array.
func TestFileRegistry_Postgres_SearchByUnicodeTag(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	wantID := seedTagFile(c, fx.groupASet, fx.ctxA, "receipt", "чек", "на-продажу")
	seedTagFile(c, fx.groupASet, fx.ctxA, "photo", "фото")

	files, err := fx.groupASet.FileRegistry.Search(fx.ctxA, "", nil, nil, []string{"чек"}, nil, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 1)
	c.Assert(files[0].ID, qt.Equals, wantID)

	files, err = fx.groupASet.FileRegistry.Search(fx.ctxA, "", nil, nil, []string{"чек", "на-продажу"}, nil, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, 1)
	c.Assert(files[0].ID, qt.Equals, wantID)
}

// Autocomplete matches a Cyrillic substring against both label and slug.
func TestTagRegistry_Postgres_SearchUnicodeSubstring(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, models.TagKindCommodity, "кухня")
	mustCreateTag(c, fx.groupASet.TagRegistry, fx.ctxA, models.TagKindCommodity, "kitchen")

	tags, err := fx.groupASet.TagRegistry.Search(fx.ctxA, "кух", 10, models.TagKindCommodity)
	c.Assert(err, qt.IsNil)
	c.Assert(tags, qt.HasLen, 1)
	c.Assert(tags[0].Slug, qt.Equals, "кухня")
}
