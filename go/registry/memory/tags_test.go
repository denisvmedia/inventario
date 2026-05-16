package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// tagFixture creates a fresh in-memory factory triple (location, area,
// commodity, file, tag) wired the way `NewFactorySet` does it. Tests use the
// returned ctx with a populated user/group to drive RLS-equivalent filtering.
type tagFixture struct {
	ctx          context.Context
	tagReg       registry.TagRegistry
	commodityReg registry.CommodityRegistry
	fileReg      registry.FileRegistry
}

func newTagFixture(c *qt.C, groupID string) tagFixture {
	c.Helper()

	locFactory := memory.NewLocationRegistryFactory()
	areaFactory := memory.NewAreaRegistryFactory(locFactory)
	commodityFactory := memory.NewCommodityRegistryFactory(areaFactory)
	fileFactory := memory.NewFileRegistryFactory()
	tagFactory := memory.NewTagRegistryFactory(commodityFactory, fileFactory)

	ctx := appctx.WithUser(c.Context(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "user-1"},
			TenantID: "tenant-1",
		},
	})
	ctx = appctx.WithGroup(ctx, &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: groupID},
			TenantID: "tenant-1",
		},
		Slug: groupID,
	})

	return tagFixture{
		ctx:          ctx,
		tagReg:       tagFactory.MustCreateUserRegistry(ctx),
		commodityReg: commodityFactory.MustCreateUserRegistry(ctx),
		fileReg:      fileFactory.MustCreateUserRegistry(ctx),
	}
}

func TestTagRegistry_Memory_CreateAndGetBySlug(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	created, err := fx.tagReg.Create(fx.ctx, models.Tag{
		Slug:  "kitchen",
		Label: "Kitchen",
		Color: models.TagColorAmber,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.Slug, qt.Equals, "kitchen")

	got, err := fx.tagReg.GetBySlug(fx.ctx, "kitchen")
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.Color, qt.Equals, models.TagColorAmber)

	_, err = fx.tagReg.GetBySlug(fx.ctx, "missing")
	c.Assert(err, qt.IsNotNil)
}

func TestTagRegistry_Memory_ListPaginatedSortByLabel(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	for _, slug := range []string{"banana", "apple", "cherry"} {
		_, err := fx.tagReg.Create(fx.ctx, models.Tag{
			Slug:  slug,
			Label: slug,
			Color: models.TagColorMuted,
		})
		c.Assert(err, qt.IsNil)
	}

	got, total, err := fx.tagReg.ListPaginated(fx.ctx, 0, 10, registry.TagListOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(total, qt.Equals, 3)
	c.Assert(got, qt.HasLen, 3)
	c.Assert(got[0].Slug, qt.Equals, "apple")
	c.Assert(got[1].Slug, qt.Equals, "banana")
	c.Assert(got[2].Slug, qt.Equals, "cherry")

	desc, _, err := fx.tagReg.ListPaginated(fx.ctx, 0, 10, registry.TagListOptions{
		SortDesc: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(desc[0].Slug, qt.Equals, "cherry")
}

func TestTagRegistry_Memory_GetUsage(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	_, err := fx.tagReg.Create(fx.ctx, models.Tag{
		Slug: "kitchen", Label: "Kitchen", Color: models.TagColorAmber,
	})
	c.Assert(err, qt.IsNil)

	// Two commodities reference 'kitchen'; one references nothing.
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name:   "fridge",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeWhiteGoods,
		Tags:   models.ValuerSlice[string]{"kitchen"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name:   "oven",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeWhiteGoods,
		Tags:   models.ValuerSlice[string]{"kitchen", "appliance"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name:   "vase",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeOther,
	})
	c.Assert(err, qt.IsNil)

	// One file references 'kitchen'.
	_, err = fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title:    "fridge-photo",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Tags:     models.StringSlice{"kitchen"},
		File: &models.File{
			Path: "fridge-photo", OriginalPath: "fridge-photo.jpg", Ext: ".jpg", MIMEType: "image/jpeg",
		},
	})
	c.Assert(err, qt.IsNil)

	usage, err := fx.tagReg.GetUsage(fx.ctx, "kitchen")
	c.Assert(err, qt.IsNil)
	c.Assert(usage.Commodities, qt.Equals, 2)
	c.Assert(usage.Files, qt.Equals, 1)

	usage, err = fx.tagReg.GetUsage(fx.ctx, "appliance")
	c.Assert(err, qt.IsNil)
	c.Assert(usage.Commodities, qt.Equals, 1)
	c.Assert(usage.Files, qt.Equals, 0)
}

func TestTagRegistry_Memory_RewriteSlugReferences(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	_, err := fx.tagReg.Create(fx.ctx, models.Tag{
		Slug: "kitchen", Label: "Kitchen", Color: models.TagColorAmber,
	})
	c.Assert(err, qt.IsNil)
	cmd1, err := fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name:   "fridge",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeWhiteGoods,
		Tags:   models.ValuerSlice[string]{"kitchen", "appliance"},
	})
	c.Assert(err, qt.IsNil)
	file1, err := fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title: "photo", Type: models.FileTypeImage, Category: models.FileCategoryImages,
		Tags: models.StringSlice{"kitchen"},
		File: &models.File{Path: "p", OriginalPath: "p.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)

	commCount, fileCount, err := fx.tagReg.RewriteSlugReferences(fx.ctx, "kitchen", "kitchen-area")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)
	c.Assert(fileCount, qt.Equals, 1)

	got, err := fx.commodityReg.Get(fx.ctx, cmd1.ID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"kitchen-area", "appliance"})

	gotFile, err := fx.fileReg.Get(fx.ctx, file1.ID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(gotFile.Tags), qt.DeepEquals, []string{"kitchen-area"})
}

func TestTagRegistry_Memory_StripSlugReferences(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	cmd1, err := fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name:   "fridge",
		Status: models.CommodityStatusInUse,
		Type:   models.CommodityTypeWhiteGoods,
		Tags:   models.ValuerSlice[string]{"kitchen", "appliance"},
	})
	c.Assert(err, qt.IsNil)

	commCount, _, err := fx.tagReg.StripSlugReferences(fx.ctx, "kitchen")
	c.Assert(err, qt.IsNil)
	c.Assert(commCount, qt.Equals, 1)

	got, err := fx.commodityReg.Get(fx.ctx, cmd1.ID)
	c.Assert(err, qt.IsNil)
	c.Assert([]string(got.Tags), qt.DeepEquals, []string{"appliance"})
}

func TestTagRegistry_Memory_SearchRanksByUsage(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	_, err := fx.tagReg.Create(fx.ctx, models.Tag{Slug: "alpha", Label: "Alpha", Color: models.TagColorMuted})
	c.Assert(err, qt.IsNil)
	_, err = fx.tagReg.Create(fx.ctx, models.Tag{Slug: "beta", Label: "Beta", Color: models.TagColorMuted})
	c.Assert(err, qt.IsNil)
	_, err = fx.tagReg.Create(fx.ctx, models.Tag{Slug: "gamma", Label: "Gamma", Color: models.TagColorMuted})
	c.Assert(err, qt.IsNil)

	// gamma has 2 commodity uses, alpha has 1, beta has 0.
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "x", Status: models.CommodityStatusInUse, Type: models.CommodityTypeOther,
		Tags: models.ValuerSlice[string]{"gamma"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "y", Status: models.CommodityStatusInUse, Type: models.CommodityTypeOther,
		Tags: models.ValuerSlice[string]{"gamma", "alpha"},
	})
	c.Assert(err, qt.IsNil)

	got, err := fx.tagReg.Search(fx.ctx, "", 10, registry.TagScopeAny)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.HasLen, 3)
	c.Assert(got[0].Slug, qt.Equals, "gamma")
	c.Assert(got[1].Slug, qt.Equals, "alpha")
	c.Assert(got[2].Slug, qt.Equals, "beta")
}

// TestTagRegistry_Memory_SearchScoped exercises the per-scope filter +
// ranking added for #1628: scope=commodity strictly excludes file-only
// tags (and vice versa), and a tag used on both scopes appears in both
// scoped result sets.
func TestTagRegistry_Memory_SearchScoped(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	for _, slug := range []string{"kitchen", "invoice", "warranty", "unused"} {
		_, err := fx.tagReg.Create(fx.ctx, models.Tag{
			Slug: slug, Label: slug, Color: models.TagColorMuted,
		})
		c.Assert(err, qt.IsNil)
	}

	// kitchen → commodity only; invoice → file only; warranty → both.
	_, err := fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "fridge", Status: models.CommodityStatusInUse, Type: models.CommodityTypeWhiteGoods,
		Tags: models.ValuerSlice[string]{"kitchen", "warranty"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title: "f", Type: models.FileTypeDocument, Category: models.FileCategoryDocuments,
		Tags: models.StringSlice{"invoice", "warranty"},
		File: &models.File{Path: "f", OriginalPath: "f.pdf", Ext: ".pdf", MIMEType: "application/pdf"},
	})
	c.Assert(err, qt.IsNil)

	commodityScope, err := fx.tagReg.Search(fx.ctx, "", 10, registry.TagScopeCommodity)
	c.Assert(err, qt.IsNil)
	commodityScopeSlugs := make([]string, 0, len(commodityScope))
	for _, t := range commodityScope {
		commodityScopeSlugs = append(commodityScopeSlugs, t.Slug)
	}
	c.Assert(commodityScopeSlugs, qt.Contains, "kitchen")
	c.Assert(commodityScopeSlugs, qt.Contains, "warranty")
	c.Assert(commodityScopeSlugs, qt.Not(qt.Contains), "invoice") // file-only excluded
	c.Assert(commodityScopeSlugs, qt.Not(qt.Contains), "unused")  // zero usage excluded

	fileScope, err := fx.tagReg.Search(fx.ctx, "", 10, registry.TagScopeFile)
	c.Assert(err, qt.IsNil)
	fileScopeSlugs := make([]string, 0, len(fileScope))
	for _, t := range fileScope {
		fileScopeSlugs = append(fileScopeSlugs, t.Slug)
	}
	c.Assert(fileScopeSlugs, qt.Contains, "invoice")
	c.Assert(fileScopeSlugs, qt.Contains, "warranty")
	c.Assert(fileScopeSlugs, qt.Not(qt.Contains), "kitchen")
	c.Assert(fileScopeSlugs, qt.Not(qt.Contains), "unused")

	// TagScopeAny includes every tag even with zero usage — preserves
	// the legacy pre-#1628 behaviour for the merged "All tags" tab.
	all, err := fx.tagReg.Search(fx.ctx, "", 10, registry.TagScopeAny)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 4)
}

// TestTagRegistry_Memory_ListPaginatedScoped mirrors SearchScoped for the
// list endpoint: scope filter + correct totals.
func TestTagRegistry_Memory_ListPaginatedScoped(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	for _, slug := range []string{"kitchen", "invoice", "warranty", "unused"} {
		_, err := fx.tagReg.Create(fx.ctx, models.Tag{
			Slug: slug, Label: slug, Color: models.TagColorMuted,
		})
		c.Assert(err, qt.IsNil)
	}
	_, err := fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "fridge", Status: models.CommodityStatusInUse, Type: models.CommodityTypeWhiteGoods,
		Tags: models.ValuerSlice[string]{"kitchen", "warranty"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title: "f", Type: models.FileTypeDocument, Category: models.FileCategoryDocuments,
		Tags: models.StringSlice{"invoice", "warranty"},
		File: &models.File{Path: "f", OriginalPath: "f.pdf", Ext: ".pdf", MIMEType: "application/pdf"},
	})
	c.Assert(err, qt.IsNil)

	gotComm, totalComm, err := fx.tagReg.ListPaginated(fx.ctx, 0, 10, registry.TagListOptions{
		Scope: registry.TagScopeCommodity,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(totalComm, qt.Equals, 2)
	slugsComm := make([]string, 0, len(gotComm))
	for _, t := range gotComm {
		slugsComm = append(slugsComm, t.Slug)
	}
	c.Assert(slugsComm, qt.Contains, "kitchen")
	c.Assert(slugsComm, qt.Contains, "warranty")

	gotFile, totalFile, err := fx.tagReg.ListPaginated(fx.ctx, 0, 10, registry.TagListOptions{
		Scope: registry.TagScopeFile,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(totalFile, qt.Equals, 2)
	slugsFile := make([]string, 0, len(gotFile))
	for _, t := range gotFile {
		slugsFile = append(slugsFile, t.Slug)
	}
	c.Assert(slugsFile, qt.Contains, "invoice")
	c.Assert(slugsFile, qt.Contains, "warranty")

	gotAny, totalAny, err := fx.tagReg.ListPaginated(fx.ctx, 0, 10, registry.TagListOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(totalAny, qt.Equals, 4)
	c.Assert(gotAny, qt.HasLen, 4)
}

func TestTagRegistry_Memory_GetUsageBatch(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	for _, slug := range []string{"kitchen", "appliance", "garden"} {
		_, err := fx.tagReg.Create(fx.ctx, models.Tag{
			Slug: slug, Label: slug, Color: models.TagColorMuted,
		})
		c.Assert(err, qt.IsNil)
	}

	_, err := fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "fridge", Status: models.CommodityStatusInUse, Type: models.CommodityTypeWhiteGoods,
		Tags: models.ValuerSlice[string]{"kitchen", "appliance"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "oven", Status: models.CommodityStatusInUse, Type: models.CommodityTypeWhiteGoods,
		Tags: models.ValuerSlice[string]{"kitchen"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title: "f", Type: models.FileTypeImage, Category: models.FileCategoryImages,
		Tags: models.StringSlice{"appliance"},
		File: &models.File{Path: "f", OriginalPath: "f.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)

	usage, err := fx.tagReg.GetUsageBatch(fx.ctx, []string{"kitchen", "appliance", "garden"})
	c.Assert(err, qt.IsNil)
	c.Assert(usage["kitchen"].Commodities, qt.Equals, 2)
	c.Assert(usage["kitchen"].Files, qt.Equals, 0)
	c.Assert(usage["appliance"].Commodities, qt.Equals, 1)
	c.Assert(usage["appliance"].Files, qt.Equals, 1)
	c.Assert(usage["garden"].Commodities, qt.Equals, 0)
	c.Assert(usage["garden"].Files, qt.Equals, 0)

	// Empty input returns empty map without touching the registries.
	empty, err := fx.tagReg.GetUsageBatch(fx.ctx, nil)
	c.Assert(err, qt.IsNil)
	c.Assert(empty, qt.HasLen, 0)
}

func TestTagRegistry_Memory_GetStats(t *testing.T) {
	c := qt.New(t)
	fx := newTagFixture(c, "group-1")

	for _, slug := range []string{"a", "b", "c"} {
		_, err := fx.tagReg.Create(fx.ctx, models.Tag{
			Slug: slug, Label: slug, Color: models.TagColorMuted,
		})
		c.Assert(err, qt.IsNil)
	}

	// 2 tagged commodities + 1 untagged.
	_, err := fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "x1", Status: models.CommodityStatusInUse, Type: models.CommodityTypeOther,
		Tags: models.ValuerSlice[string]{"a"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "x2", Status: models.CommodityStatusInUse, Type: models.CommodityTypeOther,
		Tags: models.ValuerSlice[string]{"a", "b"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.commodityReg.Create(fx.ctx, models.Commodity{
		Name: "x3", Status: models.CommodityStatusInUse, Type: models.CommodityTypeOther,
	})
	c.Assert(err, qt.IsNil)

	// 1 tagged file + 2 untagged.
	_, err = fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title: "f1", Type: models.FileTypeImage, Category: models.FileCategoryImages,
		Tags: models.StringSlice{"c"},
		File: &models.File{Path: "f1", OriginalPath: "f1.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title: "f2", Type: models.FileTypeImage, Category: models.FileCategoryImages,
		File: &models.File{Path: "f2", OriginalPath: "f2.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)
	_, err = fx.fileReg.Create(fx.ctx, models.FileEntity{
		Title: "f3", Type: models.FileTypeImage, Category: models.FileCategoryImages,
		File: &models.File{Path: "f3", OriginalPath: "f3.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)

	stats, err := fx.tagReg.GetStats(fx.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(stats.TagsTotal, qt.Equals, 3)
	c.Assert(stats.ItemsTagged, qt.Equals, 2)
	c.Assert(stats.ItemsUntagged, qt.Equals, 1)
	c.Assert(stats.FilesTagged, qt.Equals, 1)
	c.Assert(stats.FilesUntagged, qt.Equals, 2)
}

func TestTagRegistry_Memory_CrossGroupIsolation(t *testing.T) {
	c := qt.New(t)

	// Two fixtures, sharing nothing — different groups.
	g1 := newTagFixture(c, "group-1")
	g2 := newTagFixture(c, "group-2")

	_, err := g1.tagReg.Create(g1.ctx, models.Tag{Slug: "g1-only", Label: "G1", Color: models.TagColorMuted})
	c.Assert(err, qt.IsNil)

	// Tag created in g1 must not be visible in g2.
	_, err = g2.tagReg.GetBySlug(g2.ctx, "g1-only")
	c.Assert(err, qt.IsNotNil)
}
