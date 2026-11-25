package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

type draftFixture struct {
	ctx        context.Context
	factorySet *registry.FactorySet
	regs       *registry.Set
	drafts     *services.CommodityDraftService
	links      *services.FileLinkService
	areaID     string
}

func newDraftFixture(c *qt.C) *draftFixture {
	c.Helper()
	factorySet := memory.NewFactorySet()
	// Drafts belong to a group, so the context carries one.
	ctx := appctx.WithGroup(newTestContext(factorySet), &models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "draft-group"},
			TenantID: "test-tenant-id",
		},
		Slug:          "draft-group",
		GroupCurrency: "USD",
	})
	regs := must.Must(factorySet.CreateUserRegistrySet(ctx))
	location := must.Must(regs.LocationRegistry.Create(ctx, models.Location{Name: "Home"}))
	area := must.Must(regs.AreaRegistry.Create(ctx, models.Area{Name: "Garage", LocationID: location.ID}))
	return &draftFixture{
		ctx:        ctx,
		factorySet: factorySet,
		regs:       regs,
		drafts:     services.NewCommodityDraftService(factorySet),
		links:      services.NewFileLinkService(factorySet),
		areaID:     area.ID,
	}
}

func (f *draftFixture) draft(c *qt.C, key, name string) *models.CommodityDraft {
	c.Helper()
	draft, created, err := f.drafts.Create(f.ctx, key, models.CommodityDraftValues{"name": name})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	return draft
}

func (f *draftFixture) file(c *qt.C, name string, links ...models.FileLink) *models.FileEntity {
	c.Helper()
	return must.Must(f.regs.FileRegistry.Create(f.ctx, models.FileEntity{
		Title:    name,
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Links:    links,
		File:     &models.File{Path: name, OriginalPath: name + ".jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	}))
}

func (f *draftFixture) item(name string) models.Commodity {
	return models.Commodity{
		Name:      name,
		ShortName: name,
		Type:      models.CommodityTypeEquipment,
		Status:    models.CommodityStatusInUse,
		Count:     1,
		AreaID:    new(f.areaID),
	}
}

func draftLink(draftID string) models.FileLink {
	return models.FileLink{EntityType: models.FileLinkDraft, EntityID: draftID, Meta: models.FileLinkMetaImages}
}

func TestCommodityDraftService_Create_ValidatesTheClientFields(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		values models.CommodityDraftValues
	}{
		{name: "empty client key", key: ""},
		{name: "client key with a space", key: "a b"},
		{name: "client key too long", key: strings.Repeat("k", models.CommodityDraftClientKeyMaxLength+1)},
		{name: "values too large", key: "k", values: models.CommodityDraftValues{"comments": strings.Repeat("x", models.CommodityDraftValuesMaxBytes)}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			f := newDraftFixture(c)
			_, _, err := f.drafts.Create(f.ctx, tc.key, tc.values)
			c.Assert(err, qt.IsNotNil)
			count := must.Must(f.regs.CommodityDraftRegistry.Count(f.ctx))
			c.Assert(count, qt.Equals, 0)
		})
	}
}

func TestCommodityDraftService_Create_AcceptsIncompleteValues(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)

	// No name, no type, no short name: a draft is not an item.
	draft, created, err := f.drafts.Create(f.ctx, "intake:session-1:candidate-2", models.CommodityDraftValues{"comments": "blue one"})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(draft.Values.Title(), qt.Equals, "")
}

func TestCommodityDraftService_Promote(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	draft := f.draft(c, "k", "Drill")
	photo := f.file(c, "photo", draftLink(draft.ID))

	item, created, err := f.drafts.Promote(f.ctx, draft.ID, draft.Revision, f.item("Drill"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)

	moved := must.Must(f.regs.FileRegistry.Get(f.ctx, photo.ID))
	c.Assert(moved.Links, qt.HasLen, 1)
	c.Assert(moved.Links[0].EntityType, qt.Equals, models.FileLinkCommodity)
	c.Assert(moved.Links[0].EntityID, qt.Equals, item.ID)

	events, _, err := f.regs.CommodityEventRegistry.ListByCommodity(f.ctx, item.ID, 0, 10, registry.CommodityEventListOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)
	c.Assert(events[0].Kind, qt.Equals, models.CommodityEventKindCreated)

	// A retry returns the same item and records nothing.
	again, created, err := f.drafts.Promote(f.ctx, draft.ID, draft.Revision, f.item("Drill"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsFalse)
	c.Assert(again.ID, qt.Equals, item.ID)
	events, _, err = f.regs.CommodityEventRegistry.ListByCommodity(f.ctx, item.ID, 0, 10, registry.CommodityEventListOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)

	// Once the item is gone, the retry cannot hand it out.
	_, err = services.NewTrashService(f.factorySet, uploadLocationForTempDir(c.TempDir())).TrashCommodity(f.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	_, _, err = f.drafts.Promote(f.ctx, draft.ID, draft.Revision, f.item("Drill"))
	c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)

	// A promoted draft cannot be discarded.
	c.Assert(f.drafts.Discard(f.ctx, draft.ID), qt.ErrorIs, registry.ErrDraftPromoted)
}

func TestCommodityDraftService_Discard(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	draft := f.draft(c, "k", "Drill")
	other := f.draft(c, "k2", "Saw")

	// Only on this draft: goes to Trash on its own.
	onlyHere := f.file(c, "only-here", draftLink(draft.ID))
	// Also on another draft: stays and loses this link.
	shared := f.file(c, "shared", draftLink(draft.ID), draftLink(other.ID))
	// Also on an area: stays.
	onArea := f.file(c, "on-area", draftLink(draft.ID),
		models.FileLink{EntityType: models.FileLinkArea, EntityID: f.areaID, Meta: models.FileLinkMetaImages})
	// Also on an item in Trash: goes to Trash with that item.
	trashedItem := must.Must(f.regs.CommodityRegistry.Create(f.ctx, f.item("Old")))
	withItem := f.file(c, "with-item", draftLink(draft.ID),
		models.FileLink{EntityType: models.FileLinkCommodity, EntityID: trashedItem.ID, Meta: models.FileLinkMetaImages})
	_, err := f.regs.CommodityRegistry.Trash(f.ctx, trashedItem.ID, "user", time.Now())
	c.Assert(err, qt.IsNil)

	c.Assert(f.drafts.Discard(f.ctx, draft.ID), qt.IsNil)

	_, err = f.regs.CommodityDraftRegistry.Get(f.ctx, draft.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	trashed, err := f.regs.FileRegistry.GetTrashed(f.ctx, onlyHere.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(trashed.DeletedWithCommodityID, qt.IsNil, qt.Commentf("a Trash entry of its own, restorable as a standalone file"))
	c.Assert(trashed.Links, qt.HasLen, 0)

	kept := must.Must(f.regs.FileRegistry.Get(f.ctx, shared.ID))
	c.Assert(kept.Links, qt.HasLen, 1)
	c.Assert(kept.Links[0].EntityID, qt.Equals, other.ID)

	kept = must.Must(f.regs.FileRegistry.Get(f.ctx, onArea.ID))
	c.Assert(kept.Links, qt.HasLen, 1)
	c.Assert(kept.Links[0].EntityType, qt.Equals, models.FileLinkArea)

	trashed, err = f.regs.FileRegistry.GetTrashed(f.ctx, withItem.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(trashed.DeletedWithCommodityID, qt.IsNotNil)
	c.Assert(*trashed.DeletedWithCommodityID, qt.Equals, trashedItem.ID)

	// Discarding again is a 404, not a second cleanup.
	c.Assert(f.drafts.Discard(f.ctx, draft.ID), qt.ErrorIs, registry.ErrNotFound)
}

func TestFileLinkService_AttachToDraft(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	draft := f.draft(c, "k", "Drill")
	scan := f.file(c, "scan")

	// A JPEG put in the draft's documents is a document, as on an item.
	attached, created, err := f.links.Attach(f.ctx, scan.ID, models.FileLinkDraft, draft.ID, models.FileLinkMetaDocuments)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(attached.Category, qt.Equals, models.FileCategoryDocuments)
	_, created, err = f.links.Attach(f.ctx, scan.ID, models.FileLinkDraft, draft.ID, "")
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsFalse, qt.Commentf("attaching twice is idempotent"))

	_, _, err = f.links.Attach(f.ctx, scan.ID, models.FileLinkDraft, "missing", "")
	c.Assert(err, qt.ErrorIs, services.ErrFileLinkTargetNotFound)
	_, _, err = f.links.Attach(f.ctx, scan.ID, models.FileLinkDraft, draft.ID, models.FileLinkMetaFiles)
	c.Assert(err, qt.ErrorIs, services.ErrFileLinkInvalid)

	_, _, err = f.drafts.Promote(f.ctx, draft.ID, draft.Revision, f.item("Drill"))
	c.Assert(err, qt.IsNil)
	late := f.file(c, "late")
	_, _, err = f.links.Attach(f.ctx, late.ID, models.FileLinkDraft, draft.ID, "")
	c.Assert(err, qt.ErrorIs, services.ErrFileLinkTargetNotFound, qt.Commentf("a promoted draft takes no more files"))
}
