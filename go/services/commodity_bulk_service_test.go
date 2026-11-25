package services_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/commodityquery"
)

type bulkFixture struct {
	ctx    context.Context
	fs     *registry.FactorySet
	set    *registry.Set
	svc    *services.CommodityBulkService
	events registry.CommodityEventRegistry
}

func newBulkFixture(c *qt.C) *bulkFixture {
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
		Slug:          "g1",
		GroupCurrency: "USD",
	})
	ctx = validationctx.WithGroupCurrency(ctx, "USD")
	set := must.Must(fs.CreateUserRegistrySet(ctx))
	events := services.NewCommodityEventService(fs)
	svc := services.NewCommodityBulkService(
		services.NewTagService(fs),
		events,
		services.NewTrashService(fs, "file://"+c.TempDir()),
	)
	return &bulkFixture{
		ctx:    ctx,
		fs:     fs,
		set:    set,
		svc:    svc,
		events: fs.CommodityEventRegistryFactory.CreateServiceRegistry(),
	}
}

func (f *bulkFixture) item(c *qt.C, name string, mutators ...func(*models.Commodity)) *models.Commodity {
	c.Helper()
	item := models.Commodity{
		Name:                  name,
		ShortName:             strings.ReplaceAll(name, " ", ""),
		Type:                  models.CommodityTypeOther,
		Status:                models.CommodityStatusInUse,
		Count:                 1,
		OriginalPriceCurrency: "USD",
		OriginalPrice:         new(decimal.NewFromInt(10)),
	}
	for _, m := range mutators {
		m(&item)
	}
	return must.Must(f.set.CommodityRegistry.Create(f.ctx, item))
}

func (f *bulkFixture) get(c *qt.C, id string) *models.Commodity {
	c.Helper()
	return must.Must(f.set.CommodityRegistry.Get(f.ctx, id))
}

func (f *bulkFixture) eventKinds(c *qt.C, id string) []models.CommodityEventKind {
	c.Helper()
	var kinds []models.CommodityEventKind
	for _, e := range must.Must(f.events.List(f.ctx)) {
		if e.CommodityID == id {
			kinds = append(kinds, e.Kind)
		}
	}
	return kinds
}

// byID names items by id only, as an ids selection does.
func byID(ids ...string) []services.BulkTarget {
	targets := make([]services.BulkTarget, 0, len(ids))
	for _, id := range ids {
		targets = append(targets, services.BulkTarget{ID: id})
	}
	return targets
}

func targetIDs(targets []services.BulkTarget) []string {
	ids := make([]string, 0, len(targets))
	for _, t := range targets {
		ids = append(ids, t.ID)
	}
	return ids
}

func withTags(tags ...string) func(*models.Commodity) {
	return func(c *models.Commodity) { c.Tags = tags }
}

func parsePatch(c *qt.C, in services.CommodityPatchInput) services.CommodityPatch {
	c.Helper()
	patch, err := services.ParseCommodityPatch(c.Context(), in)
	c.Assert(err, qt.IsNil)
	return patch
}

func TestParseCommodityPatch_Refuses(t *testing.T) {
	tooMany := make([]string, services.MaxBulkTags+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("tag-%d", i)
	}
	tests := []struct {
		name  string
		in    services.CommodityPatchInput
		field string
	}{
		{"a tag both added and removed", services.CommodityPatchInput{AddTags: []string{"Garage"}, RemoveTags: []string{"garage"}}, "tags"},
		{"a tag that is no tag", services.CommodityPatchInput{AddTags: []string{"###"}}, "tags"},
		{"too many tags", services.CommodityPatchInput{RemoveTags: tooMany}, "tags"},
		{"an unknown type", services.CommodityPatchInput{Type: new("spaceship")}, "type"},
		{"an unknown status", services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "gone"}}, "status"},
		{"sold", services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "sold", Date: "2026-01-02"}}, "status"},
		{"in use with a date", services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "in_use", Date: "2026-01-02"}}, "status_date"},
		{"in use with a note", services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "in_use", Note: "x"}}, "status_note"},
		{"leaving in use without a date", services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "lost"}}, "status_date"},
		{"a date that is no date", services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "lost", Date: "02.01.2026"}}, "status_date"},
		{"a note over 1024 characters", services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "lost", Date: "2026-01-02", Note: strings.Repeat("ž", 1025)}}, "status_note"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			_, err := services.ParseCommodityPatch(c.Context(), tt.in)
			errs, ok := errors.AsType[validation.Errors](err)
			c.Assert(ok, qt.IsTrue, qt.Commentf("err = %v", err))
			c.Assert(errs[tt.field], qt.IsNotNil, qt.Commentf("errs = %v", errs))
		})
	}
}

func TestParseCommodityPatch_NormalizesTags(t *testing.T) {
	c := qt.New(t)
	patch := parsePatch(c, services.CommodityPatchInput{
		AddTags:    []string{"Power Tools", "power tools", "Insured"},
		RemoveTags: []string{"Old"},
		Status:     &services.CommodityStatusInput{Value: "disposed", Date: "2026-11-25", Note: "clear-out"},
	})
	c.Assert(patch.AddTags, qt.DeepEquals, []string{"power-tools", "insured"})
	c.Assert(patch.RemoveTags, qt.DeepEquals, []string{"old"})
	c.Assert(patch.Status.Status, qt.Equals, models.CommodityStatusDisposed)
	c.Assert(string(*patch.Status.Date), qt.Equals, "2026-11-25")
	c.Assert(patch.Status.Note, qt.Equals, "clear-out")
}

func TestCommodityPatch_Apply(t *testing.T) {
	sold := func(c *models.Commodity) {
		c.Status = models.CommodityStatusSold
		c.StatusDate = models.ToPDate("2025-05-05")
		c.StatusNote = "to Bob"
		c.SalePrice = new(decimal.NewFromInt(5))
	}
	tests := []struct {
		name    string
		item    func(*models.Commodity)
		patch   services.CommodityPatchInput
		changed bool
		check   func(c *qt.C, got models.Commodity)
	}{
		{
			name:    "adds and removes tags and keeps the others in order",
			item:    withTags("b", "old", "a"),
			patch:   services.CommodityPatchInput{AddTags: []string{"new", "a"}, RemoveTags: []string{"old"}},
			changed: true,
			check: func(c *qt.C, got models.Commodity) {
				c.Assert([]string(got.Tags), qt.DeepEquals, []string{"b", "a", "new"})
			},
		},
		{
			name:    "a tag the item already has changes nothing",
			item:    withTags("a"),
			patch:   services.CommodityPatchInput{AddTags: []string{"a"}, RemoveTags: []string{"missing"}},
			changed: false,
		},
		{
			name:    "removing the last tag leaves an empty list",
			item:    withTags("a"),
			patch:   services.CommodityPatchInput{RemoveTags: []string{"a"}},
			changed: true,
			check: func(c *qt.C, got models.Commodity) {
				c.Assert(got.Tags, qt.IsNotNil)
				c.Assert(got.Tags, qt.HasLen, 0)
			},
		},
		{
			name:    "sets the type",
			patch:   services.CommodityPatchInput{Type: new("electronics")},
			changed: true,
			check: func(c *qt.C, got models.Commodity) {
				c.Assert(got.Type, qt.Equals, models.CommodityTypeElectronics)
			},
		},
		{
			name:    "the same type changes nothing",
			patch:   services.CommodityPatchInput{Type: new("other")},
			changed: false,
		},
		{
			name:    "leaving in use records the date and note",
			patch:   services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "lost", Date: "2026-11-25", Note: "airport"}},
			changed: true,
			check: func(c *qt.C, got models.Commodity) {
				c.Assert(got.Status, qt.Equals, models.CommodityStatusLost)
				c.Assert(string(*got.StatusDate), qt.Equals, "2026-11-25")
				c.Assert(got.StatusNote, qt.Equals, "airport")
			},
		},
		{
			name:    "back to in use clears the date, note and sale price",
			item:    sold,
			patch:   services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "in_use"}},
			changed: true,
			check: func(c *qt.C, got models.Commodity) {
				c.Assert(got.Status, qt.Equals, models.CommodityStatusInUse)
				c.Assert(got.StatusDate, qt.IsNil)
				c.Assert(got.StatusNote, qt.Equals, "")
				c.Assert(got.SalePrice, qt.IsNil)
			},
		},
		{
			name:    "another status replaces the date and note and clears the sale price",
			item:    sold,
			patch:   services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "lost", Date: "2026-11-25"}},
			changed: true,
			check: func(c *qt.C, got models.Commodity) {
				c.Assert(got.SalePrice, qt.IsNil)
				c.Assert(string(*got.StatusDate), qt.Equals, "2026-11-25")
			},
		},
		{
			name: "the same status changes nothing",
			item: func(c *models.Commodity) {
				c.Status = models.CommodityStatusLost
				c.StatusDate = models.ToPDate("2025-01-01")
			},
			patch:   services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "lost", Date: "2026-11-25"}},
			changed: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			item := models.Commodity{Type: models.CommodityTypeOther, Status: models.CommodityStatusInUse}
			if tt.item != nil {
				tt.item(&item)
			}
			original := item
			got, changed := parsePatch(c, tt.patch).Apply(item)
			c.Assert(changed, qt.Equals, tt.changed)
			if !tt.changed {
				c.Assert(got, qt.DeepEquals, original)
			}
			if tt.check != nil {
				tt.check(c, got)
			}
			// The item passed in is not modified.
			c.Assert(item, qt.DeepEquals, original)
		})
	}
}

func TestResolveBulkSelection_IDs(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	targets, err := services.ResolveBulkSelection(f.ctx, f.set, services.BulkSelection{IDs: []string{"b", "a", "b"}})
	c.Assert(err, qt.IsNil)
	c.Assert(targets, qt.DeepEquals, byID("b", "a"))
}

func TestResolveBulkSelection_Query(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	garageB := f.item(c, "Garage B", withTags("garage"))
	garageA := f.item(c, "Garage A", withTags("garage"))
	f.item(c, "Kitchen", withTags("kitchen"))
	inTrash := f.item(c, "Garage Trashed", withTags("garage"))
	_, err := services.NewTrashService(f.fs, "file://"+c.TempDir()).TrashCommodity(f.ctx, inTrash.ID)
	c.Assert(err, qt.IsNil)

	query := must.Must(commodityquery.Parse(map[string][]string{"tags": {"garage"}}))

	digest := commodityquery.Digest([]string{garageB.ID, garageA.ID})

	c.Run("selects the live items the query lists, in its order, with their rows", func(c *qt.C) {
		targets, err := services.ResolveBulkSelection(f.ctx, f.set, services.BulkSelection{Query: &query, ExpectedCount: 2, ExpectedDigest: digest})
		c.Assert(err, qt.IsNil)
		c.Assert(targetIDs(targets), qt.DeepEquals, []string{garageA.ID, garageB.ID})
		c.Assert(targets[0].Row.Name, qt.Equals, "Garage A")
	})

	c.Run("the list digest of the query is the one it expects", func(c *qt.C) {
		listed, ok, err := commodityquery.SelectionDigest(f.ctx, f.set, query, services.MaxBulkQueryItems)
		c.Assert(err, qt.IsNil)
		c.Assert(ok, qt.IsTrue)
		c.Assert(listed, qt.Equals, digest)
	})

	c.Run("refuses when the query selects a different number", func(c *qt.C) {
		_, err := services.ResolveBulkSelection(f.ctx, f.set, services.BulkSelection{Query: &query, ExpectedCount: 1, ExpectedDigest: digest})
		changed, ok := errors.AsType[*services.BulkSelectionChangedError](err)
		c.Assert(ok, qt.IsTrue, qt.Commentf("err = %v", err))
		c.Assert(changed.Expected, qt.Equals, 1)
		c.Assert(changed.Matched, qt.Equals, 2)
	})

	c.Run("refuses as many items with one swapped", func(c *qt.C) {
		seen := commodityquery.Digest([]string{garageA.ID, "an-item-since-untagged"})
		_, err := services.ResolveBulkSelection(f.ctx, f.set, services.BulkSelection{Query: &query, ExpectedCount: 2, ExpectedDigest: seen})
		changed, ok := errors.AsType[*services.BulkSelectionChangedError](err)
		c.Assert(ok, qt.IsTrue, qt.Commentf("err = %v", err))
		c.Assert(changed.Matched, qt.Equals, 2)
	})

	c.Run("refuses items in Trash", func(c *qt.C) {
		trashed := query
		trashed.Trashed = true
		_, err := services.ResolveBulkSelection(f.ctx, f.set, services.BulkSelection{Query: &trashed, ExpectedCount: 1, ExpectedDigest: digest})
		qErr, ok := errors.AsType[*commodityquery.Error](err)
		c.Assert(ok, qt.IsTrue, qt.Commentf("err = %v", err))
		c.Assert(qErr.Param, qt.Equals, "trashed")
	})
}

func TestResolveBulkSelection_TooMany(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	for i := range services.MaxBulkQueryItems + 1 {
		f.item(c, fmt.Sprintf("Item %03d", i))
	}
	query := commodityquery.Default()
	_, err := services.ResolveBulkSelection(f.ctx, f.set, services.BulkSelection{Query: &query, ExpectedCount: services.MaxBulkQueryItems + 1})
	tooMany, ok := errors.AsType[*services.BulkTooManyError](err)
	c.Assert(ok, qt.IsTrue, qt.Commentf("err = %v", err))
	c.Assert(tooMany.Matched, qt.Equals, services.MaxBulkQueryItems+1)
	c.Assert(tooMany.Max, qt.Equals, services.MaxBulkQueryItems)
}

func TestCommodityBulkService_UpdateTags(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	drill := f.item(c, "Drill", withTags("tools", "old"))
	saw := f.item(c, "Saw")
	insured := f.item(c, "Insured", withTags("insured"))

	patch := parsePatch(c, services.CommodityPatchInput{AddTags: []string{"Insured"}, RemoveTags: []string{"old"}})
	c.Assert(f.svc.EnsureTags(f.ctx, &patch), qt.IsNil)
	out := f.svc.Update(f.ctx, f.set.CommodityRegistry, byID(drill.ID, saw.ID, insured.ID, "missing"), patch)

	c.Assert(out.Succeeded, qt.DeepEquals, []string{drill.ID, saw.ID})
	c.Assert(out.Skipped, qt.DeepEquals, []services.BulkSkip{{ID: insured.ID, Name: "Insured"}})
	c.Assert(out.Failed, qt.HasLen, 1)
	c.Assert(out.Failed[0].ID, qt.Equals, "missing")
	c.Assert(out.Failed[0].Err, qt.ErrorIs, registry.ErrNotFound)

	c.Assert([]string(f.get(c, drill.ID).Tags), qt.DeepEquals, []string{"tools", "insured"})
	c.Assert([]string(f.get(c, saw.ID).Tags), qt.DeepEquals, []string{"insured"})

	// One history event per changed item, none for the skipped one.
	c.Assert(f.eventKinds(c, drill.ID), qt.DeepEquals, []models.CommodityEventKind{models.CommodityEventKindUpdated})
	c.Assert(f.eventKinds(c, saw.ID), qt.DeepEquals, []models.CommodityEventKind{models.CommodityEventKindUpdated})
	c.Assert(f.eventKinds(c, insured.ID), qt.HasLen, 0)

	// A tag the group did not have is created with the label as typed.
	tag, err := f.set.TagRegistry.GetBySlug(f.ctx, models.TagKindCommodity, "insured")
	c.Assert(err, qt.IsNil)
	c.Assert(tag.Label, qt.Equals, "Insured")
}

func TestCommodityBulkService_UpdateStatus(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	lamp := f.item(c, "Lamp")
	chair := f.item(c, "Chair")
	// An item stored without a short name does not pass the item validation;
	// a bulk change reports it as failed and leaves it as it is.
	broken := f.item(c, "Broken", func(c *models.Commodity) { c.ShortName = "" })

	patch := parsePatch(c, services.CommodityPatchInput{
		Status: &services.CommodityStatusInput{Value: "disposed", Date: "2026-11-25", Note: "clear-out"},
	})
	out := f.svc.Update(f.ctx, f.set.CommodityRegistry, byID(lamp.ID, chair.ID, broken.ID), patch)

	c.Assert(out.Succeeded, qt.DeepEquals, []string{lamp.ID, chair.ID})
	c.Assert(out.Failed, qt.HasLen, 1)
	c.Assert(out.Failed[0].ID, qt.Equals, broken.ID)
	c.Assert(out.Failed[0].Name, qt.Equals, "Broken")
	_, isValidation := errors.AsType[validation.Errors](out.Failed[0].Err)
	c.Assert(isValidation, qt.IsTrue, qt.Commentf("err = %v", out.Failed[0].Err))

	got := f.get(c, lamp.ID)
	c.Assert(got.Status, qt.Equals, models.CommodityStatusDisposed)
	c.Assert(string(*got.StatusDate), qt.Equals, "2026-11-25")
	c.Assert(got.StatusNote, qt.Equals, "clear-out")
	c.Assert(f.eventKinds(c, lamp.ID), qt.DeepEquals, []models.CommodityEventKind{models.CommodityEventKindStatusChanged})
	c.Assert(f.get(c, broken.ID).Status, qt.Equals, models.CommodityStatusInUse)
	c.Assert(f.eventKinds(c, broken.ID), qt.HasLen, 0)

	// Back to in use clears what the status change recorded.
	revert := parsePatch(c, services.CommodityPatchInput{Status: &services.CommodityStatusInput{Value: "in_use"}})
	out = f.svc.Update(f.ctx, f.set.CommodityRegistry, byID(lamp.ID), revert)
	c.Assert(out.Succeeded, qt.DeepEquals, []string{lamp.ID})
	got = f.get(c, lamp.ID)
	c.Assert(got.Status, qt.Equals, models.CommodityStatusInUse)
	c.Assert(got.StatusDate, qt.IsNil)
	c.Assert(got.StatusNote, qt.Equals, "")
}

func TestCommodityBulkService_MoveAndDelete(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	location := must.Must(f.set.LocationRegistry.Create(f.ctx, models.Location{Name: "Home"}))
	area := must.Must(f.set.AreaRegistry.Create(f.ctx, models.Area{Name: "Garage", LocationID: location.ID}))
	there := f.item(c, "There", func(c *models.Commodity) { c.AreaID = new(area.ID) })
	loose := f.item(c, "Loose")

	out := f.svc.Move(f.ctx, f.set.CommodityRegistry, byID(there.ID, loose.ID), area.ID)
	c.Assert(out.Succeeded, qt.DeepEquals, []string{loose.ID})
	c.Assert(out.Skipped, qt.DeepEquals, []services.BulkSkip{{ID: there.ID, Name: "There"}})
	c.Assert(*f.get(c, loose.ID).AreaID, qt.Equals, area.ID)
	c.Assert(f.eventKinds(c, loose.ID), qt.DeepEquals, []models.CommodityEventKind{models.CommodityEventKindMoved})

	out = f.svc.Delete(f.ctx, byID(there.ID, loose.ID))
	c.Assert(out.Succeeded, qt.DeepEquals, []string{there.ID, loose.ID})
	_, err := f.set.CommodityRegistry.GetTrashed(f.ctx, loose.ID)
	c.Assert(err, qt.IsNil)

	// Deleting again finds nothing to delete.
	out = f.svc.Delete(f.ctx, byID(loose.ID))
	c.Assert(out.Failed, qt.HasLen, 1)
	c.Assert(out.Failed[0].Err, qt.ErrorIs, registry.ErrNotFound)
}

// fileTrashFails is a file registry whose Trash always fails, as when the
// database drops the connection between the item and its files.
type fileTrashFails struct{ registry.FileRegistry }

func (fileTrashFails) Trash(context.Context, string, string, string, time.Time) error {
	return errors.New("connection lost")
}

type fileTrashFailsFactory struct{ registry.FileRegistryFactory }

func (f fileTrashFailsFactory) CreateUserRegistry(ctx context.Context) (registry.FileRegistry, error) {
	reg, err := f.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, err
	}
	return fileTrashFails{reg}, nil
}

func TestCommodityBulkService_DeleteKeepsAnItemWhoseFilesStayed(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	item := f.item(c, "Camera")
	must.Must(f.set.FileRegistry.Create(f.ctx, models.FileEntity{
		Title:    "camera",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		Links:    []models.FileLink{{EntityType: models.FileLinkCommodity, EntityID: item.ID, Meta: models.FileLinkMetaImages}},
		File:     &models.File{Path: "camera", OriginalPath: "blobs/camera.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	}))

	fs := *f.fs
	fs.FileRegistryFactory = fileTrashFailsFactory{f.fs.FileRegistryFactory}
	svc := services.NewCommodityBulkService(
		services.NewTagService(&fs),
		services.NewCommodityEventService(&fs),
		services.NewTrashService(&fs, "file://"+c.TempDir()),
	)

	// The item is in Trash, so the delete succeeded and Undo must offer it;
	// the note says its file did not follow.
	out := svc.Delete(f.ctx, byID(item.ID))
	c.Assert(out.Succeeded, qt.DeepEquals, []string{item.ID})
	c.Assert(out.Failed, qt.HasLen, 0)
	c.Assert(out.Notes, qt.DeepEquals, []services.BulkNote{{ID: item.ID, Name: "Camera", Code: services.BulkNoteFilesKept}})
	_, err := f.set.CommodityRegistry.GetTrashed(f.ctx, item.ID)
	c.Assert(err, qt.IsNil)
}

func TestCommodityBulkService_StopsWhenTheRequestEnds(t *testing.T) {
	c := qt.New(t)
	f := newBulkFixture(c)
	a := f.item(c, "A")
	b := f.item(c, "B")
	ctx, cancel := context.WithCancel(f.ctx)
	cancel()

	patch := parsePatch(c, services.CommodityPatchInput{Type: new("electronics")})
	out := f.svc.Update(ctx, f.set.CommodityRegistry, byID(a.ID, b.ID), patch)
	c.Assert(out.Succeeded, qt.HasLen, 0)
	c.Assert(out.Failed, qt.HasLen, 2)
	c.Assert(f.get(c, a.ID).Type, qt.Equals, models.CommodityTypeOther)
}
