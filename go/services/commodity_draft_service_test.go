package services_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"

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
		{name: "a form field of the wrong type", key: "k", values: models.CommodityDraftValues{"short_name": nil}},
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

// TestCommodityDraftService_Create_ReplayAfterTheTypeWasDeleted: a repeated
// create with a key the group already has returns that draft, even when the
// type in its values was deleted since. A new draft with that type is
// refused, and a key whose draft is being discarded stays refused.
func TestCommodityDraftService_Create_ReplayAfterTheTypeWasDeleted(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	boat := must.Must(f.regs.ItemTypeRegistry.Create(f.ctx, models.ItemType{Name: "Boat"}))
	values := models.CommodityDraftValues{"name": "Dinghy", "type": string(boat.Slug)}
	first, created, err := f.drafts.Create(f.ctx, "replayed-key", values)
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)

	c.Assert(f.regs.ItemTypeRegistry.Delete(f.ctx, boat.ID), qt.IsNil)

	replayed, created, err := f.drafts.Create(f.ctx, "replayed-key", values)
	c.Assert(err, qt.IsNil)
	c.Check(created, qt.IsFalse)
	c.Check(replayed.ID, qt.Equals, first.ID)

	_, _, err = f.drafts.Create(f.ctx, "new-key", values)
	verr, ok := errors.AsType[validation.Errors](err)
	c.Assert(ok, qt.IsTrue, qt.Commentf("got %v", err))
	c.Check(verr["values"], qt.IsNotNil)

	_, err = f.regs.CommodityDraftRegistry.MarkDiscarding(f.ctx, first.ID)
	c.Assert(err, qt.IsNil)
	_, _, err = f.drafts.Create(f.ctx, "replayed-key", values)
	c.Check(err, qt.ErrorIs, registry.ErrDraftDiscarded)
}

func TestCommodityDraftService_Promote(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	draft := f.draft(c, "k", "Drill")
	photo := f.file(c, "photo", draftLink(draft.ID))

	item, created, err := f.drafts.Promote(f.ctx, draft.ID, draft.Revision, "key-1", f.item("Drill"))
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

	// A retry with the same key returns the same item and records nothing.
	again, created, err := f.drafts.Promote(f.ctx, draft.ID, draft.Revision, "key-1", f.item("Drill"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsFalse)
	c.Assert(again.ID, qt.Equals, item.ID)
	events, _, err = f.regs.CommodityEventRegistry.ListByCommodity(f.ctx, item.ID, 0, 10, registry.CommodityEventListOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(events, qt.HasLen, 1)

	// Another device's promotion is told the draft is an item, and does not
	// take this item for its own.
	_, _, err = f.drafts.Promote(f.ctx, draft.ID, draft.Revision, "key-2", f.item("Drill"))
	c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted)

	// Once the item is gone, the retry cannot hand it out.
	_, err = services.NewTrashService(f.factorySet, uploadLocationForTempDir(c.TempDir())).TrashCommodity(f.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	_, _, err = f.drafts.Promote(f.ctx, draft.ID, draft.Revision, "key-1", f.item("Drill"))
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

func TestCommodityDraftService_DiscardFinishesAnInterruptedDiscard(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	draft := f.draft(c, "k", "Drill")
	photo := f.file(c, "photo", draftLink(draft.ID))

	// A discard that stopped after taking the draft out of play.
	_, err := f.regs.CommodityDraftRegistry.MarkDiscarding(f.ctx, draft.ID)
	c.Assert(err, qt.IsNil)

	// The draft is out of reach: its key cannot make it again and nothing
	// can be attached.
	_, _, err = f.drafts.Create(f.ctx, "k", models.CommodityDraftValues{"name": "Drill"})
	c.Assert(err, qt.ErrorIs, registry.ErrDraftDiscarded)
	late := f.file(c, "late")
	_, _, err = f.links.Attach(f.ctx, late.ID, models.FileLinkDraft, draft.ID, "")
	c.Assert(err, qt.ErrorIs, registry.ErrDraftDiscarded)

	c.Assert(f.drafts.Discard(f.ctx, draft.ID), qt.IsNil)
	_, err = f.regs.CommodityDraftRegistry.Get(f.ctx, draft.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = f.regs.FileRegistry.GetTrashed(f.ctx, photo.ID)
	c.Assert(err, qt.IsNil)
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

	_, _, err = f.drafts.Promote(f.ctx, draft.ID, draft.Revision, "key-1", f.item("Drill"))
	c.Assert(err, qt.IsNil)
	late := f.file(c, "late")
	_, _, err = f.links.Attach(f.ctx, late.ID, models.FileLinkDraft, draft.ID, "")
	c.Assert(err, qt.ErrorIs, registry.ErrDraftPromoted, qt.Commentf("a promoted draft takes no more files"))
}

// TestCommodityDraftService_FinishStaleDiscards: a discard that stopped half
// way is finished once it is ten minutes old, so its files go to Trash and
// its client key is free again. A younger one may still be running and is
// left alone, and so is an open draft.
func TestCommodityDraftService_FinishStaleDiscards(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	stopped := f.draft(c, "stopped", "Drill")
	photo := f.file(c, "photo", draftLink(stopped.ID))
	open := f.draft(c, "open", "Saw")
	_, err := f.regs.CommodityDraftRegistry.MarkDiscarding(f.ctx, stopped.ID)
	c.Assert(err, qt.IsNil)

	soon := services.NewCommodityDraftService(f.factorySet,
		services.WithCommodityDraftClock(func() time.Time { return time.Now().Add(5 * time.Minute) }))
	finished, err := soon.FinishStaleDiscards(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(finished, qt.Equals, 0)
	_, err = f.regs.CommodityDraftRegistry.Get(f.ctx, stopped.ID)
	c.Assert(err, qt.IsNil)

	later := services.NewCommodityDraftService(f.factorySet,
		services.WithCommodityDraftClock(func() time.Time { return time.Now().Add(11 * time.Minute) }))
	finished, err = later.FinishStaleDiscards(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(finished, qt.Equals, 1)
	_, err = f.regs.CommodityDraftRegistry.Get(f.ctx, stopped.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = f.regs.FileRegistry.GetTrashed(f.ctx, photo.ID)
	c.Assert(err, qt.IsNil)
	_, err = f.regs.CommodityDraftRegistry.Get(f.ctx, open.ID)
	c.Assert(err, qt.IsNil)

	again, created, err := f.drafts.Create(f.ctx, "stopped", models.CommodityDraftValues{"name": "Drill"})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(again.ID, qt.Not(qt.Equals), stopped.ID)
}

// failingDeleteDrafts is a draft registry factory whose registries fail to
// delete the draft with the id failID.
type failingDeleteDrafts struct {
	registry.CommodityDraftRegistryFactory
	failID string
}

func (f failingDeleteDrafts) CreateUserRegistry(ctx context.Context) (registry.CommodityDraftRegistry, error) {
	reg, err := f.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return failingDeleteDraftRegistry{CommodityDraftRegistry: reg, failID: f.failID}, nil
}

type failingDeleteDraftRegistry struct {
	registry.CommodityDraftRegistry
	failID string
}

var errDraftDeleteFails = errors.New("the draft cannot be deleted")

func (r failingDeleteDraftRegistry) DeleteDiscarding(ctx context.Context, id string) error {
	if id == r.failID {
		return errDraftDeleteFails
	}
	return r.CommodityDraftRegistry.DeleteDiscarding(ctx, id)
}

// TestCommodityDraftService_FinishStaleDiscards_OneFailureDoesNotStopTheRest:
// a stopped discard that keeps failing is reported, and the stopped discards
// after it are still finished.
func TestCommodityDraftService_FinishStaleDiscards_OneFailureDoesNotStopTheRest(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	var stopped []*models.CommodityDraft
	for _, key := range []string{"first", "poisoned", "last"} {
		d := f.draft(c, key, key)
		_, err := f.regs.CommodityDraftRegistry.MarkDiscarding(f.ctx, d.ID)
		c.Assert(err, qt.IsNil)
		stopped = append(stopped, d)
	}
	poisoned := stopped[1]

	factorySet := *f.factorySet
	factorySet.CommodityDraftRegistryFactory = failingDeleteDrafts{
		CommodityDraftRegistryFactory: f.factorySet.CommodityDraftRegistryFactory,
		failID:                        poisoned.ID,
	}
	later := services.NewCommodityDraftService(&factorySet,
		services.WithCommodityDraftClock(func() time.Time { return time.Now().Add(11 * time.Minute) }))
	finished, err := later.FinishStaleDiscards(f.ctx)
	c.Assert(err, qt.ErrorIs, errDraftDeleteFails)
	c.Assert(finished, qt.Equals, 2)

	for _, d := range stopped {
		_, err := f.regs.CommodityDraftRegistry.Get(f.ctx, d.ID)
		if d.ID == poisoned.ID {
			c.Check(err, qt.IsNil, qt.Commentf("the failing draft is still there"))
			continue
		}
		c.Check(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("draft %s", d.ClientKey))
	}
}
