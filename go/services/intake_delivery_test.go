package services_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// paperlessDelivery is a delivery as the Paperless-ngx recipe of the docs
// sends it.
func paperlessDelivery(id string) services.IntakeDelivery {
	return services.IntakeDelivery{
		Title:        "Invoice " + id,
		SourceSystem: "paperless-ngx",
		DocumentID:   id,
		SourceURL:    "https://paperless.example/documents/" + id + "/",
		SourceName:   "Paperless-ngx",
		Seller:       "Hardware  Store",
		PurchaseDate: "2024-03-14",
		Amount:       "EUR129.90",
	}
}

func (f *intakeFixture) deliver(c *qt.C, in services.IntakeDelivery, file *services.IntakeDeliveredFile) *services.IntakeDeliveryResult {
	c.Helper()
	result, err := f.intake.Deliver(f.ctx, in, file)
	c.Assert(err, qt.IsNil)
	return result
}

// TestIntakeDeliver_MakesADraft: a delivery becomes one draft with the
// delivered values, in a session of its own whose source needs no scan, and
// the inbox shows the draft with where it came from.
func TestIntakeDeliver_MakesADraft(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)

	result := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(result.Created, qt.IsTrue)
	c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(result.File, qt.IsNil)
	doc := result.Document
	c.Assert(doc.ExternalSystem, qt.Equals, "paperless-ngx")
	c.Assert(doc.ExternalInstance, qt.Equals, "")
	c.Assert(doc.ExternalDocumentID, qt.Equals, "117")
	c.Assert(doc.SourceName, qt.Equals, "Paperless-ngx")
	c.Assert(doc.DraftID, qt.IsNotNil)
	c.Assert(doc.SessionID, qt.IsNotNil)
	c.Assert(doc.FileID, qt.IsNil)

	drafts := f.drafts(c)
	c.Assert(drafts, qt.HasLen, 1)
	c.Assert(drafts[0].ID, qt.Equals, *doc.DraftID)
	c.Assert(drafts[0].Values, qt.DeepEquals, models.CommodityDraftValues{
		"name":                            "Invoice 117",
		"urls":                            []any{"https://paperless.example/documents/117/"},
		services.IntakeDraftSellerNameKey: "Hardware Store",
		"purchase_date":                   "2024-03-14",
		"original_price":                  "129.90",
		"original_price_currency":         "EUR",
	})
	// No scan ran.
	c.Assert(f.provider.callCount(), qt.Equals, 0)

	session := f.get(c, f.ctx, *doc.SessionID)
	c.Assert(session.Session.Origin, qt.Equals, models.IntakeOriginExternal)
	c.Assert(session.Sources, qt.HasLen, 1)
	c.Assert(session.Sources[0].Source.Status, qt.Equals, models.IntakeSourceScanned)
	c.Assert(session.Sources[0].Source.ExternalDocumentID, qt.Equals, "117")

	// The inbox shows the draft once, with its source; the session has no
	// work left.
	regs := must.Must(f.factorySet.CreateUserRegistrySet(f.ctx))
	page, err := services.NewInboxService().List(f.ctx, regs, services.InboxListOptions{Limit: 50})
	c.Assert(err, qt.IsNil)
	c.Assert(page.Entries, qt.HasLen, 1)
	c.Assert(page.Entries[0].Kind, qt.Equals, services.InboxKindDraft)
	c.Assert(page.Entries[0].Source, qt.DeepEquals, &services.InboxEntrySource{
		Name: "Paperless-ngx",
		URL:  "https://paperless.example/documents/117/",
	})

	// A rescan of the delivered source is refused: there is nothing to scan.
	_, err = f.intake.Rescan(f.ctx, session.Session.ID, session.Sources[0].Source.ID)
	c.Assert(err, qt.ErrorIs, services.ErrIntakeSourceDelivered)
}

// TestIntakeDeliver_MissingDataStaysUnknown: a delivery with only the
// required fields makes a draft with only the name.
func TestIntakeDeliver_MissingDataStaysUnknown(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)

	result := f.deliver(c, services.IntakeDelivery{Title: "Manual", SourceSystem: "Paperless-NGX", DocumentID: " 5 "}, nil)
	c.Assert(result.Document.ExternalSystem, qt.Equals, "paperless-ngx")
	c.Assert(result.Document.ExternalDocumentID, qt.Equals, "5")
	drafts := f.drafts(c)
	c.Assert(drafts, qt.HasLen, 1)
	c.Assert(drafts[0].Values, qt.DeepEquals, models.CommodityDraftValues{"name": "Manual"})

	regs := must.Must(f.factorySet.CreateUserRegistrySet(f.ctx))
	page, err := services.NewInboxService().List(f.ctx, regs, services.InboxListOptions{Limit: 50})
	c.Assert(err, qt.IsNil)
	c.Assert(page.Entries, qt.HasLen, 1)
	// Without a name the system is shown.
	c.Assert(page.Entries[0].Source, qt.DeepEquals, &services.InboxEntrySource{Name: "paperless-ngx"})
}

// TestIntakeDeliver_Replay: the same delivery again returns the document and
// makes nothing; other values are refused and change nothing; another
// instance of the system is another document.
func TestIntakeDeliver_Replay(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)

	first := f.deliver(c, paperlessDelivery("117"), nil)

	// The same values, written differently, are the same delivery.
	same := paperlessDelivery("117")
	same.SourceSystem = "Paperless-ngx"
	same.Amount = "129.9"
	same.Currency = "eur"
	same.PurchaseDate = "2024-03-14T00:00:00Z"
	again := f.deliver(c, same, nil)
	c.Assert(again.Created, qt.IsFalse)
	c.Assert(again.Document.ID, qt.Equals, first.Document.ID)
	c.Assert(*again.Document.DraftID, qt.Equals, *first.Document.DraftID)
	c.Assert(again.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(f.drafts(c), qt.HasLen, 1)

	changed := paperlessDelivery("117")
	changed.Title = "Invoice 117 (corrected)"
	result, err := f.intake.Deliver(f.ctx, changed, nil)
	c.Assert(err, qt.ErrorIs, services.ErrIntakeDocumentChanged)
	c.Assert(result.Document.ID, qt.Equals, first.Document.ID)
	drafts := f.drafts(c)
	c.Assert(drafts, qt.HasLen, 1)
	c.Assert(drafts[0].Values["name"], qt.Equals, "Invoice 117")

	other := paperlessDelivery("117")
	other.SourceInstance = "office"
	c.Assert(f.deliver(c, other, nil).Created, qt.IsTrue)
	c.Assert(f.drafts(c), qt.HasLen, 2)
}

// TestIntakeDeliver_RecordOutlivesTheWork: a replay after the session is
// deleted, the draft discarded or the item finished makes nothing, and the
// state says what became of the draft.
func TestIntakeDeliver_RecordOutlivesTheWork(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	drafts := services.NewCommodityDraftService(f.factorySet)

	deleted := f.deliver(c, paperlessDelivery("1"), nil)
	c.Assert(f.intake.Delete(f.ctx, *deleted.Document.SessionID), qt.IsNil)
	replay := f.deliver(c, paperlessDelivery("1"), nil)
	c.Assert(replay.Created, qt.IsFalse)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(f.drafts(c), qt.HasLen, 1)

	discarded := f.deliver(c, paperlessDelivery("2"), nil)
	c.Assert(drafts.Discard(f.ctx, *discarded.Document.DraftID), qt.IsNil)
	replay = f.deliver(c, paperlessDelivery("2"), nil)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentRemoved)
	c.Assert(f.drafts(c), qt.HasLen, 1)

	regs := must.Must(f.factorySet.CreateUserRegistrySet(f.ctx))
	location := must.Must(regs.LocationRegistry.Create(f.ctx, models.Location{Name: "Home"}))
	area := must.Must(regs.AreaRegistry.Create(f.ctx, models.Area{Name: "Office", LocationID: location.ID}))
	draft := must.Must(regs.CommodityDraftRegistry.Get(f.ctx, *deleted.Document.DraftID))
	item, _, err := drafts.Promote(f.ctx, draft.ID, draft.Revision, "promotion-1", models.Commodity{
		Name:      "Invoice 1",
		ShortName: "Invoice 1",
		Type:      models.CommodityTypeOther,
		AreaID:    new(area.ID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
		Tags:      models.ValuerSlice[string]{},
	})
	c.Assert(err, qt.IsNil)
	replay = f.deliver(c, paperlessDelivery("1"), nil)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentItem)
	c.Assert(*replay.Document.CommodityID, qt.Equals, item.ID)
	// The promoted draft is the only one left; the replay made none.
	left := f.drafts(c)
	c.Assert(left, qt.HasLen, 1)
	c.Assert(left[0].Status, qt.Equals, models.CommodityDraftPromoted)

	// The item finds the document it came from.
	docs, err := f.intake.ListDocuments(f.ctx, nil, []string{item.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(docs, qt.HasLen, 1)
	c.Assert(docs[0].ID, qt.Equals, deleted.Document.ID)
	both, err := f.intake.ListDocuments(f.ctx, []string{*deleted.Document.DraftID, *discarded.Document.DraftID}, []string{item.ID})
	c.Assert(err, qt.IsNil)
	c.Assert(both, qt.HasLen, 2)
	states, err := f.intake.DocumentStates(f.ctx, both)
	c.Assert(err, qt.IsNil)
	c.Assert(states, qt.DeepEquals, map[string]models.IntakeDocumentState{
		deleted.Document.ID:   models.IntakeDocumentItem,
		discarded.Document.ID: models.IntakeDocumentRemoved,
	})
}

// TestIntakeDeliver_File: the delivered file becomes a file of the group
// attached to the draft; a replay with the same file stores nothing, another
// file is a change.
func TestIntakeDeliver_File(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)

	blob := &services.IntakeDeliveredFile{
		BlobKey:  "t/tenant-a/files/invoice-117.pdf",
		Name:     "invoice-117.pdf",
		MIMEType: "application/pdf",
		Size:     2048,
		SHA256:   "aaaa",
	}
	result := f.deliver(c, paperlessDelivery("117"), blob)
	c.Assert(result.File, qt.IsNotNil)
	c.Assert(result.File.Title, qt.Equals, "invoice-117")
	c.Assert(result.File.File.Ext, qt.Equals, ".pdf")
	c.Assert(result.File.File.OriginalPath, qt.Equals, blob.BlobKey)
	c.Assert(result.File.Category, qt.Equals, models.FileCategoryDocuments)
	c.Assert(*result.Document.FileID, qt.Equals, result.File.ID)

	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	attached := must.Must(files.ListByLinkedEntity(f.ctx, models.FileLinkDraft, *result.Document.DraftID))
	c.Assert(attached, qt.HasLen, 1)
	c.Assert(attached[0].ID, qt.Equals, result.File.ID)

	replay := f.deliver(c, paperlessDelivery("117"), &services.IntakeDeliveredFile{
		BlobKey: "t/tenant-a/files/second.pdf", Name: "invoice-117.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa",
	})
	c.Assert(replay.Created, qt.IsFalse)
	c.Assert(replay.File, qt.IsNil)
	c.Assert(must.Must(files.List(f.ctx)), qt.HasLen, 1)

	_, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), &services.IntakeDeliveredFile{
		BlobKey: "t/tenant-a/files/third.pdf", Name: "invoice-117.pdf", MIMEType: "application/pdf", Size: 10, SHA256: "bbbb",
	})
	c.Assert(err, qt.ErrorIs, services.ErrIntakeDocumentChanged)
	// Leaving the file out is a change too.
	_, err = f.intake.Deliver(f.ctx, paperlessDelivery("117"), nil)
	c.Assert(err, qt.ErrorIs, services.ErrIntakeDocumentChanged)
}

// TestIntakeDeliver_Validation: a malformed value is refused with the field
// it belongs to, and nothing is stored.
func TestIntakeDeliver_Validation(t *testing.T) {
	long := make([]rune, 256)
	for i := range long {
		long[i] = 'x'
	}
	tests := []struct {
		name  string
		edit  func(*services.IntakeDelivery)
		field string
	}{
		{"no title", func(d *services.IntakeDelivery) { d.Title = "  " }, "title"},
		{"title too long", func(d *services.IntakeDelivery) { d.Title = string(long) }, "title"},
		{"no system", func(d *services.IntakeDelivery) { d.SourceSystem = "" }, "source_system"},
		{"a document id without a system", func(d *services.IntakeDelivery) { d.SourceSystem = " " }, "source_system"},
		{"system with spaces", func(d *services.IntakeDelivery) { d.SourceSystem = "paper less" }, "source_system"},
		{"no document id", func(d *services.IntakeDelivery) { d.DocumentID = "" }, "document_id"},
		{"relative url", func(d *services.IntakeDelivery) { d.SourceURL = "/documents/1/" }, "source_url"},
		{"file url", func(d *services.IntakeDelivery) { d.SourceURL = "file:///etc/passwd" }, "source_url"},
		{"javascript url", func(d *services.IntakeDelivery) { d.SourceURL = "javascript:alert(1)" }, "source_url"},
		{"url with credentials", func(d *services.IntakeDelivery) { d.SourceURL = "https://user:pw@paperless.example/" }, "source_url"},
		{"date", func(d *services.IntakeDelivery) { d.PurchaseDate = "14.03.2024" }, "purchase_date"},
		{"negative amount", func(d *services.IntakeDelivery) { d.Amount = "-1" }, "amount"},
		{"three decimals", func(d *services.IntakeDelivery) { d.Amount = "1.234" }, "amount"},
		{"unknown code in the amount", func(d *services.IntakeDelivery) { d.Amount = "XYZ1.00" }, "amount"},
		{"unknown currency", func(d *services.IntakeDelivery) { d.Currency = "XYZ" }, "currency"},
		{"currency differs from the amount", func(d *services.IntakeDelivery) { d.Currency = "USD" }, "currency"},
		{"fourteen digits before the point", func(d *services.IntakeDelivery) { d.Amount = "EUR12345678901234.00" }, "amount"},
		{"a NUL in the title", func(d *services.IntakeDelivery) { d.Title = "Invoice\x00117" }, "title"},
		{"a NUL in the seller", func(d *services.IntakeDelivery) { d.Seller = "Hard\x00ware" }, "seller"},
		{"a NUL in the document id", func(d *services.IntakeDelivery) { d.DocumentID = "117\x00" }, "document_id"},
		{"a NUL in the system name", func(d *services.IntakeDelivery) { d.SourceName = "Paper\x00less" }, "source_name"},
		{"invalid UTF-8 in the document id", func(d *services.IntakeDelivery) { d.DocumentID = "117\xff" }, "document_id"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			f := newIntakeFixture(c)
			in := paperlessDelivery("1")
			tc.edit(&in)
			_, err := f.intake.Deliver(f.ctx, in, nil)
			errs, ok := errorsAsValidation(err)
			c.Assert(ok, qt.IsTrue, qt.Commentf("got %v", err))
			c.Assert(errs[tc.field], qt.IsNotNil, qt.Commentf("got %v", errs))
			c.Assert(f.drafts(c), qt.HasLen, 0)
			docs := must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
			c.Assert(must.Must(docs.List(f.ctx)), qt.HasLen, 0)
		})
	}
}

// TestIntakeDeliver_FinishesHalfDoneWork: a document recorded without its
// draft, as a delivery that failed half way leaves it, gets the draft on the
// replay.
func TestIntakeDeliver_FinishesHalfDoneWork(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	first := f.deliver(c, paperlessDelivery("117"), nil)
	f.forgetDraft(c, first.Document)

	replay := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(replay.Created, qt.IsFalse)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(*replay.Document.DraftID, qt.Equals, *first.Document.DraftID)
	c.Assert(f.drafts(c), qt.HasLen, 1)
}

// forgetDraft makes the document look as a call that failed after making the
// draft and before recording it leaves it: the record is made again with the
// file and the session only.
func (f *intakeFixture) forgetDraft(c *qt.C, doc *models.IntakeDocument) {
	c.Helper()
	docs := must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(docs.Delete(f.ctx, doc.ID), qt.IsNil)
	half := *doc
	half.DraftID = nil
	stored, _, err := docs.CreateOrGet(f.ctx, half)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DraftID, qt.IsNil)
	c.Assert(stored.SessionID, qt.IsNotNil)
}

var errLostWrite = errors.New("the write was lost")

// lossyDocuments is a document registry whose records of a file, a session
// or a draft fail while asked to, as writes lost to the database would.
type lossyDocuments struct {
	registry.IntakeDocumentRegistryFactory
	loseFile, loseSession, loseDraft, loseGet atomic.Bool
}

func (f *lossyDocuments) CreateUserRegistry(ctx context.Context) (registry.IntakeDocumentRegistry, error) {
	reg, err := f.IntakeDocumentRegistryFactory.CreateUserRegistry(ctx)
	return &lossyDocumentRegistry{IntakeDocumentRegistry: reg, f: f}, err
}

type lossyDocumentRegistry struct {
	registry.IntakeDocumentRegistry
	f *lossyDocuments
}

func (r *lossyDocumentRegistry) RecordProgress(ctx context.Context, id string, progress registry.IntakeDocumentProgress) (*models.IntakeDocument, error) {
	if (progress.FileID != nil && r.f.loseFile.Load()) ||
		(progress.SessionID != nil && r.f.loseSession.Load()) ||
		(progress.DraftID != nil && r.f.loseDraft.Load()) {
		return nil, errLostWrite
	}
	return r.IntakeDocumentRegistry.RecordProgress(ctx, id, progress)
}

func (r *lossyDocumentRegistry) Get(ctx context.Context, id string) (*models.IntakeDocument, error) {
	if r.f.loseGet.Load() {
		return nil, errLostWrite
	}
	return r.IntakeDocumentRegistry.Get(ctx, id)
}

// TestIntakeDeliver_DocumentReadFails: the document cannot be read again
// before the session is made. The delivery returns the error with the record
// it made, and the repeat finishes the delivery.
func TestIntakeDeliver_DocumentReadFails(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	lossy := f.lossyDocuments()
	lossy.loseGet.Store(true)
	result, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), nil)
	lossy.loseGet.Store(false)
	c.Assert(err, qt.ErrorIs, errLostWrite)
	c.Assert(result.Document, qt.IsNotNil)
	c.Assert(result.Document.SessionID, qt.IsNil)

	repeat := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(repeat.Document.ID, qt.Equals, result.Document.ID)
	c.Assert(repeat.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(f.drafts(c), qt.HasLen, 1)
}

func (f *intakeFixture) lossyDocuments() *lossyDocuments {
	w := &lossyDocuments{IntakeDocumentRegistryFactory: f.factorySet.IntakeDocumentRegistryFactory}
	f.factorySet.IntakeDocumentRegistryFactory = w
	return w
}

// TestIntakeDeliver_DraftNotRecorded: the draft was made and the document
// could not record it. The repeat finds the draft by its key, also after the
// member deleted the session and after the draft became an item, and makes
// no second one.
func TestIntakeDeliver_DraftNotRecorded(t *testing.T) {
	fail := func(c *qt.C, f *intakeFixture, id string) *models.CommodityDraft {
		c.Helper()
		lossy := f.lossyDocuments()
		lossy.loseDraft.Store(true)
		result, err := f.intake.Deliver(f.ctx, paperlessDelivery(id), nil)
		lossy.loseDraft.Store(false)
		c.Assert(err, qt.ErrorIs, errLostWrite)
		// The result still names the document the call stopped on.
		c.Assert(result.Document, qt.IsNotNil)
		c.Assert(result.Document.SessionID, qt.IsNotNil)
		c.Assert(result.Document.DraftID, qt.IsNil)
		drafts := f.drafts(c)
		c.Assert(drafts, qt.HasLen, 1)
		return drafts[0]
	}

	t.Run("repeat", func(t *testing.T) {
		c := qt.New(t)
		f := newIntakeFixture(c)
		draft := fail(c, f, "1")
		replay := f.deliver(c, paperlessDelivery("1"), nil)
		c.Assert(replay.State, qt.Equals, models.IntakeDocumentDraft)
		c.Assert(*replay.Document.DraftID, qt.Equals, draft.ID)
		c.Assert(f.drafts(c), qt.HasLen, 1)
	})

	t.Run("after the session was deleted", func(t *testing.T) {
		c := qt.New(t)
		f := newIntakeFixture(c)
		draft := fail(c, f, "1")
		docs := must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
		doc := must.Must(docs.GetByExternalID(f.ctx, "paperless-ngx", "", "1"))
		c.Assert(f.intake.Delete(f.ctx, *doc.SessionID), qt.IsNil)
		for range 2 {
			replay := f.deliver(c, paperlessDelivery("1"), nil)
			c.Assert(replay.State, qt.Equals, models.IntakeDocumentDraft)
			c.Assert(*replay.Document.DraftID, qt.Equals, draft.ID)
			c.Assert(f.drafts(c), qt.HasLen, 1)
		}
	})

	t.Run("after the draft became an item", func(t *testing.T) {
		c := qt.New(t)
		f := newIntakeFixture(c)
		draft := fail(c, f, "1")
		regs := must.Must(f.factorySet.CreateUserRegistrySet(f.ctx))
		location := must.Must(regs.LocationRegistry.Create(f.ctx, models.Location{Name: "Home"}))
		area := must.Must(regs.AreaRegistry.Create(f.ctx, models.Area{Name: "Office", LocationID: location.ID}))
		item, _, err := services.NewCommodityDraftService(f.factorySet).Promote(f.ctx, draft.ID, draft.Revision, "promotion-1", models.Commodity{
			Name: "Invoice 1", ShortName: "Invoice 1", Type: models.CommodityTypeOther, AreaID: new(area.ID),
			Count: 1, Status: models.CommodityStatusInUse, Tags: models.ValuerSlice[string]{},
		})
		c.Assert(err, qt.IsNil)
		replay := f.deliver(c, paperlessDelivery("1"), nil)
		c.Assert(replay.State, qt.Equals, models.IntakeDocumentItem)
		c.Assert(*replay.Document.CommodityID, qt.Equals, item.ID)
		docs, err := f.intake.ListDocuments(f.ctx, nil, []string{item.ID})
		c.Assert(err, qt.IsNil)
		c.Assert(docs, qt.HasLen, 1)
	})

	// Another member finishes the draft and the delivery is never repeated:
	// the promotion finds the record by the draft's key.
	t.Run("promoted with no repeat", func(t *testing.T) {
		c := qt.New(t)
		f := newIntakeFixture(c)
		draft := fail(c, f, "1")
		role := models.GroupRoleUser
		member := f.as(f.member(c, "member@example.com", &role))
		regs := must.Must(f.factorySet.CreateUserRegistrySet(member))
		location := must.Must(regs.LocationRegistry.Create(member, models.Location{Name: "Home"}))
		area := must.Must(regs.AreaRegistry.Create(member, models.Area{Name: "Office", LocationID: location.ID}))
		item, _, err := services.NewCommodityDraftService(f.factorySet).Promote(member, draft.ID, draft.Revision, "promotion-1", models.Commodity{
			Name: "Invoice 1", ShortName: "Invoice 1", Type: models.CommodityTypeOther, AreaID: new(area.ID),
			Count: 1, Status: models.CommodityStatusInUse, Tags: models.ValuerSlice[string]{},
		})
		c.Assert(err, qt.IsNil)

		docs, err := f.intake.ListDocuments(f.ctx, nil, []string{item.ID})
		c.Assert(err, qt.IsNil)
		c.Assert(docs, qt.HasLen, 1)
		c.Assert(docs[0].ExternalDocumentID, qt.Equals, "1")
		c.Assert(*docs[0].DraftID, qt.Equals, draft.ID)
		states, err := f.intake.DocumentStates(f.ctx, docs)
		c.Assert(err, qt.IsNil)
		c.Assert(states[docs[0].ID], qt.Equals, models.IntakeDocumentItem)
	})
}

// TestIntakeDeliver_SessionDeletedBeforeTheDraft: the member deleted the
// session before its draft was made. The repeat brings neither back: the
// document says its draft is removed, and stays so.
func TestIntakeDeliver_SessionDeletedBeforeTheDraft(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	first := f.deliver(c, paperlessDelivery("1"), nil)
	regs := must.Must(f.factorySet.CreateUserRegistrySet(f.ctx))
	c.Assert(regs.CommodityDraftRegistry.Delete(f.ctx, *first.Document.DraftID), qt.IsNil)
	f.forgetDraft(c, first.Document)
	c.Assert(f.intake.Delete(f.ctx, *first.Document.SessionID), qt.IsNil)

	for range 2 {
		replay := f.deliver(c, paperlessDelivery("1"), nil)
		c.Assert(replay.State, qt.Equals, models.IntakeDocumentRemoved)
		c.Assert(*replay.Document.DraftID, qt.Equals, "")
		c.Assert(f.drafts(c), qt.HasLen, 0)
	}
	changed := paperlessDelivery("1")
	changed.Title = "Another invoice"
	_, err := f.intake.Deliver(f.ctx, changed, nil)
	c.Assert(err, qt.ErrorIs, services.ErrIntakeDocumentChanged)
}

// TestIntakeDeliver_SessionOfALostRecord: a session carries the document's
// identity, but the group lost the record of the delivery that made it. A
// delivery of the document is refused, with the same values too, since what
// the first one held is not known, and so is its repeat; nothing is stored,
// the record included. The draft that session made is refused the same way
// once the member deleted the session; after it is discarded, the document
// makes a new one.
func TestIntakeDeliver_SessionOfALostRecord(t *testing.T) {
	changed := paperlessDelivery("117")
	changed.Title = "Another invoice"
	for name, in := range map[string]services.IntakeDelivery{"the first values": paperlessDelivery("117"), "other values": changed} {
		t.Run(name, func(t *testing.T) {
			c := qt.New(t)
			f := newIntakeFixture(c)
			blob := func(key string) *services.IntakeDeliveredFile {
				return &services.IntakeDeliveredFile{BlobKey: key, Name: "invoice.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa"}
			}
			first := f.deliver(c, paperlessDelivery("117"), blob("t/tenant-a/files/first.pdf"))
			docs := must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
			c.Assert(docs.Delete(f.ctx, first.Document.ID), qt.IsNil)
			files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
			refused := func() {
				c.Helper()
				result, err := f.intake.Deliver(f.ctx, in, blob("t/tenant-a/files/again.pdf"))
				c.Assert(err, qt.ErrorIs, services.ErrIntakeDocumentConflict)
				c.Assert(result.File, qt.IsNil)
				c.Assert(result.Document.DraftID, qt.IsNil)
				c.Assert(must.Must(files.List(f.ctx)), qt.HasLen, 1)
				c.Assert(f.drafts(c), qt.HasLen, 1)
				c.Assert(must.Must(docs.List(f.ctx)), qt.HasLen, 0)
			}
			refused()
			refused()

			c.Assert(f.intake.Delete(f.ctx, *first.Document.SessionID), qt.IsNil)
			refused()

			c.Assert(services.NewCommodityDraftService(f.factorySet).Discard(f.ctx, *first.Document.DraftID), qt.IsNil)
			result := f.deliver(c, in, blob("t/tenant-a/files/again.pdf"))
			c.Assert(result.Created, qt.IsTrue)
			c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
			c.Assert(*result.Document.DraftID, qt.Not(qt.Equals), *first.Document.DraftID)
		})
	}
}

// TestIntakeDeliver_FileNotRecorded: a file the document could not record is
// deleted again, so its blob can go and the repeat stores the file once.
func TestIntakeDeliver_FileNotRecorded(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	blob := &services.IntakeDeliveredFile{BlobKey: "t/tenant-a/files/a.pdf", Name: "a.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa"}
	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))

	lossy := f.lossyDocuments()
	lossy.loseFile.Store(true)
	result, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), blob)
	lossy.loseFile.Store(false)
	c.Assert(err, qt.ErrorIs, errLostWrite)
	c.Assert(result.File, qt.IsNil)
	c.Assert(must.Must(files.List(f.ctx)), qt.HasLen, 0)

	replay := f.deliver(c, paperlessDelivery("117"), blob)
	c.Assert(replay.File, qt.IsNotNil)
	c.Assert(must.Must(files.List(f.ctx)), qt.HasLen, 1)
	c.Assert(f.drafts(c), qt.HasLen, 1)
}

// TestIntakeDeliver_ChangedAfterAFailure: a delivery that failed half way
// stands as it was sent. Other values are refused also then, and make
// nothing; the first values finish the delivery with them.
func TestIntakeDeliver_ChangedAfterAFailure(t *testing.T) {
	for _, tc := range []struct {
		name string
		lose func(*lossyDocuments) *atomic.Bool
	}{
		{"before the file was recorded", func(l *lossyDocuments) *atomic.Bool { return &l.loseFile }},
		{"before the session was recorded", func(l *lossyDocuments) *atomic.Bool { return &l.loseSession }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			f := newIntakeFixture(c)
			blob := &services.IntakeDeliveredFile{BlobKey: "t/tenant-a/files/a.pdf", Name: "a.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa"}
			lose := tc.lose(f.lossyDocuments())
			lose.Store(true)
			_, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), blob)
			lose.Store(false)
			c.Assert(err, qt.ErrorIs, errLostWrite)

			corrected := paperlessDelivery("117")
			corrected.Title = "Invoice 117, corrected"
			_, err = f.intake.Deliver(f.ctx, corrected, blob)
			c.Assert(err, qt.ErrorIs, services.ErrIntakeDocumentChanged)

			result := f.deliver(c, paperlessDelivery("117"), blob)
			c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
			drafts := f.drafts(c)
			c.Assert(drafts, qt.HasLen, 1)
			c.Assert(drafts[0].Values["name"], qt.Equals, "Invoice 117")
		})
	}
}

// TestIntakeDeliver_Concurrent: deliveries of one document at once make one
// record, one draft and one file, and each gets the document.
func TestIntakeDeliver_Concurrent(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	const n = 16
	var (
		wg            sync.WaitGroup
		created, kept atomic.Int32
		errs          = make([]error, n)
	)
	for i := range n {
		wg.Go(func() {
			blob := &services.IntakeDeliveredFile{BlobKey: fmt.Sprintf("t/tenant-a/files/%d.pdf", i), Name: "invoice.pdf", MIMEType: "application/pdf", Size: 10, SHA256: "aaaa"}
			result, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), blob)
			errs[i] = err
			if err != nil {
				return
			}
			if result.Created {
				created.Add(1)
			}
			if result.File != nil {
				kept.Add(1)
			}
		})
	}
	wg.Wait()
	for _, err := range errs {
		c.Assert(err, qt.IsNil)
	}
	c.Assert(created.Load(), qt.Equals, int32(1))
	c.Assert(kept.Load(), qt.Equals, int32(1))
	c.Assert(f.drafts(c), qt.HasLen, 1)
	c.Assert(must.Must(files.List(f.ctx)), qt.HasLen, 1)
}

// TestIntakeDeliver_LargeAmount: the largest amount an item holds keeps
// every digit.
func TestIntakeDeliver_LargeAmount(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	in := paperlessDelivery("9")
	in.Amount = "EUR9999999999999.99"
	result := f.deliver(c, in, nil)
	draft := must.Must(must.Must(f.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx)).Get(f.ctx, *result.Document.DraftID))
	c.Assert(draft.Values["original_price"], qt.Equals, "9999999999999.99")
}

// TestIntakeDeliver_OutlivesTheAccountThatDelivered: the account of the
// token that delivered the document is deleted. The record stays with the
// group, so a delivery from a new token finds the item.
func TestIntakeDeliver_OutlivesTheAccountThatDelivered(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	role := models.GroupRoleUser
	integrator := f.member(c, "integrator@example.com", &role)
	first, err := f.intake.Deliver(f.as(integrator), paperlessDelivery("117"), nil)
	c.Assert(err, qt.IsNil)
	regs := must.Must(f.factorySet.CreateUserRegistrySet(f.ctx))
	location := must.Must(regs.LocationRegistry.Create(f.ctx, models.Location{Name: "Home"}))
	area := must.Must(regs.AreaRegistry.Create(f.ctx, models.Area{Name: "Office", LocationID: location.ID}))
	draft := must.Must(regs.CommodityDraftRegistry.Get(f.ctx, *first.Document.DraftID))
	item, _, err := services.NewCommodityDraftService(f.factorySet).Promote(f.ctx, draft.ID, draft.Revision, "promotion-1", models.Commodity{
		Name: "Invoice 117", ShortName: "Invoice 117", Type: models.CommodityTypeOther, AreaID: new(area.ID),
		Count: 1, Status: models.CommodityStatusInUse, Tags: models.ValuerSlice[string]{},
	})
	c.Assert(err, qt.IsNil)

	c.Assert(f.factorySet.UserPurger.PurgeUserDependents(context.Background(), "tenant-a", integrator.ID), qt.IsNil)
	docs := must.Must(must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx))
	c.Assert(docs, qt.HasLen, 1)
	c.Assert(docs[0].CreatedByUserID, qt.Equals, f.user.ID)

	again := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(again.Created, qt.IsFalse)
	c.Assert(again.State, qt.Equals, models.IntakeDocumentItem)
	c.Assert(*again.Document.CommodityID, qt.Equals, item.ID)
	for _, d := range f.drafts(c) {
		c.Assert(d.Status, qt.Not(qt.Equals), models.CommodityDraftOpen)
	}
}

// TestIntakeDeliver_OpenDraftOutlivesTheAccountThatDelivered: the draft a
// delivery made goes to the member who gets its record, with the member's
// own drafts left out. A replay finds it waiting in the inbox.
func TestIntakeDeliver_OpenDraftOutlivesTheAccountThatDelivered(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	role := models.GroupRoleUser
	integrator := f.member(c, "integrator@example.com", &role)
	first, err := f.intake.Deliver(f.as(integrator), paperlessDelivery("117"), nil)
	c.Assert(err, qt.IsNil)
	drafts := must.Must(f.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(f.as(integrator)))
	own, _, err := drafts.CreateOrGet(f.as(integrator), models.CommodityDraft{ClientKey: "form-1", Values: models.CommodityDraftValues{"name": "Lamp"}})
	c.Assert(err, qt.IsNil)

	c.Assert(f.factorySet.UserPurger.PurgeUserDependents(context.Background(), "tenant-a", integrator.ID), qt.IsNil)
	left := f.drafts(c)
	c.Assert(left, qt.HasLen, 1)
	c.Assert(left[0].ID, qt.Equals, *first.Document.DraftID)
	c.Assert(left[0].CreatedByUserID, qt.Equals, f.user.ID)
	c.Assert(left[0].ID, qt.Not(qt.Equals), own.ID)

	again := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(again.Created, qt.IsFalse)
	c.Assert(again.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(*again.Document.DraftID, qt.Equals, *first.Document.DraftID)
	c.Assert(f.drafts(c), qt.HasLen, 1)
}

// TestIntakeDeliver_SessionKeyIsReserved: a member cannot make a batch
// under the key of the session a delivery makes, so a delivery that stopped
// before it made its session never takes a member's batch for its own.
func TestIntakeDeliver_SessionKeyIsReserved(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	blob := &services.IntakeDeliveredFile{BlobKey: "t/tenant-a/files/a.pdf", Name: "a.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa"}
	lossy := f.lossyDocuments()
	lossy.loseFile.Store(true)
	_, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), blob)
	lossy.loseFile.Store(false)
	c.Assert(err, qt.ErrorIs, errLostWrite)
	doc := must.Must(must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx)).GetByExternalID(f.ctx, "paperless-ngx", "", "117"))

	photo := f.photo(c, "member-photo")
	_, _, err = f.intake.CreateSession(f.ctx, models.IntakeDeliverySessionKeyPrefix+doc.ID,
		[]services.IntakeSourceRequest{{FileIDs: []string{photo.ID}}})
	errs, ok := errorsAsValidation(err)
	c.Assert(ok, qt.IsTrue, qt.Commentf("got %v", err))
	c.Assert(errs["client_key"], qt.IsNotNil)

	result := f.deliver(c, paperlessDelivery("117"), blob)
	c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
	session := must.Must(must.Must(f.factorySet.IntakeRegistryFactory.CreateUserRegistry(f.ctx)).Get(f.ctx, *result.Document.SessionID))
	c.Assert(session.Origin, qt.Equals, models.IntakeOriginExternal)
}

// failingAttach is a draft registry whose AttachFile fails while asked to.
type failingAttach struct {
	registry.CommodityDraftRegistryFactory
	fail atomic.Bool
}

type failingAttachRegistry struct {
	registry.CommodityDraftRegistry
	f *failingAttach
}

func (f *failingAttach) CreateUserRegistry(ctx context.Context) (registry.CommodityDraftRegistry, error) {
	reg, err := f.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	return &failingAttachRegistry{CommodityDraftRegistry: reg, f: f}, err
}

func (r *failingAttachRegistry) AttachFile(ctx context.Context, draftID string, link models.FileLink) (*models.FileLink, bool, error) {
	if r.f.fail.Load() {
		return nil, false, errLostWrite
	}
	return r.CommodityDraftRegistry.AttachFile(ctx, draftID, link)
}

// TestIntakeDeliver_FileNotAttached: the draft was made but its file could
// not be attached. The repeat attaches it and finishes the product before it
// records the draft.
func TestIntakeDeliver_FileNotAttached(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	blob := &services.IntakeDeliveredFile{BlobKey: "t/tenant-a/files/a.pdf", Name: "a.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa"}
	w := &failingAttach{CommodityDraftRegistryFactory: f.factorySet.CommodityDraftRegistryFactory}
	f.factorySet.CommodityDraftRegistryFactory = w
	w.fail.Store(true)
	_, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), blob)
	w.fail.Store(false)
	c.Assert(err, qt.ErrorIs, errLostWrite)
	c.Assert(f.drafts(c), qt.HasLen, 1)

	replay := f.deliver(c, paperlessDelivery("117"), blob)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentDraft)
	draft := f.drafts(c)[0]
	c.Assert(*replay.Document.DraftID, qt.Equals, draft.ID)
	files := must.Must(must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx)).ListByLinkedEntity(f.ctx, models.FileLinkDraft, draft.ID))
	c.Assert(files, qt.HasLen, 1)
	c.Assert(files[0].ID, qt.Equals, *replay.Document.FileID)
	detail := must.Must(f.intake.GetSession(f.ctx, *replay.Document.SessionID))
	c.Assert(detail.Candidates, qt.HasLen, 1)
	c.Assert(detail.Candidates[0].Outcome, qt.Equals, models.IntakeCandidateCreated)
}

// beforeCreateSession runs hook once, in the first CreateSession after it
// is armed, before the session is made.
type beforeCreateSession struct {
	registry.IntakeRegistryFactory
	armed atomic.Bool
	hook  func()
}

type beforeCreateSessionRegistry struct {
	registry.IntakeRegistry
	f *beforeCreateSession
}

func (b *beforeCreateSession) CreateUserRegistry(ctx context.Context) (registry.IntakeRegistry, error) {
	reg, err := b.IntakeRegistryFactory.CreateUserRegistry(ctx)
	return &beforeCreateSessionRegistry{IntakeRegistry: reg, f: b}, err
}

func (r *beforeCreateSessionRegistry) CreateSession(ctx context.Context, session models.IntakeSession, sources []registry.IntakeSourceInput, maxQueued int) (*models.IntakeSession, bool, error) {
	if r.f.armed.CompareAndSwap(true, false) {
		r.f.hook()
	}
	return r.IntakeRegistry.CreateSession(ctx, session, sources, maxQueued)
}

// TestIntakeDeliver_SessionDeletedDuringADelivery: while one delivery is
// about to make the session, a concurrent one makes and records its own,
// with the draft, and the member deletes that session. The first delivery
// does not bring the session back: it deletes the one it made and ends with
// the draft the deleted session made.
func TestIntakeDeliver_SessionDeletedDuringADelivery(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	w := &beforeCreateSession{IntakeRegistryFactory: f.factorySet.IntakeRegistryFactory}
	f.factorySet.IntakeRegistryFactory = w
	var other *services.IntakeDeliveryResult
	w.hook = func() {
		other = f.deliver(c, paperlessDelivery("117"), nil)
		c.Assert(f.intake.Delete(f.ctx, *other.Document.SessionID), qt.IsNil)
	}
	w.armed.Store(true)

	result := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(other, qt.IsNotNil)
	c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(*result.Document.DraftID, qt.Equals, *other.Document.DraftID)
	c.Assert(f.drafts(c), qt.HasLen, 1)
	sessions := must.Must(must.Must(f.factorySet.IntakeRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx))
	c.Assert(sessions, qt.HasLen, 0)
}

// TestIntakeDeliver_SourceAppearsAfterTheFileIsStored: a source with the
// document's identity, made for a record the group no longer has, appears
// after the delivery stored its file and before it makes the session. The
// delivery is refused and leaves neither its record nor its file, so the
// caller deletes the blob.
func TestIntakeDeliver_SourceAppearsAfterTheFileIsStored(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	orig := f.factorySet.IntakeRegistryFactory
	w := &beforeCreateSession{IntakeRegistryFactory: orig}
	f.factorySet.IntakeRegistryFactory = w
	w.hook = func() {
		_, _, err := must.Must(orig.CreateUserRegistry(f.ctx)).CreateSession(f.ctx,
			models.IntakeSession{ClientKey: models.IntakeDeliverySessionKeyPrefix + "lost-record", Origin: models.IntakeOriginExternal},
			[]registry.IntakeSourceInput{{
				OneItem:            true,
				ExternalSystem:     "paperless-ngx",
				ExternalDocumentID: "117",
				Delivered:          &registry.IntakeScan{},
			}},
			0)
		c.Assert(err, qt.IsNil)
	}
	w.armed.Store(true)

	blob := &services.IntakeDeliveredFile{BlobKey: "t/tenant-a/files/a.pdf", Name: "a.pdf", MIMEType: "application/pdf", Size: 2048, SHA256: "aaaa"}
	result, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), blob)
	c.Assert(err, qt.ErrorIs, services.ErrIntakeDocumentConflict)
	c.Assert(w.armed.Load(), qt.IsFalse, qt.Commentf("the source appeared after the file was stored"))
	c.Assert(result.File, qt.IsNil)
	files := must.Must(f.factorySet.FileRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(must.Must(files.List(f.ctx)), qt.HasLen, 0)
	docs := must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(must.Must(docs.List(f.ctx)), qt.HasLen, 0)
}

// failingSessionDelete is an intake registry whose Delete fails while asked
// to.
type failingSessionDelete struct {
	registry.IntakeRegistryFactory
	fail atomic.Bool
}

type failingSessionDeleteRegistry struct {
	registry.IntakeRegistry
	f *failingSessionDelete
}

func (f *failingSessionDelete) CreateUserRegistry(ctx context.Context) (registry.IntakeRegistry, error) {
	reg, err := f.IntakeRegistryFactory.CreateUserRegistry(ctx)
	return &failingSessionDeleteRegistry{IntakeRegistry: reg, f: f}, err
}

func (r *failingSessionDeleteRegistry) Delete(ctx context.Context, id string) error {
	if r.f.fail.Load() {
		return errLostWrite
	}
	return r.IntakeRegistry.Delete(ctx, id)
}

// TestIntakeDeliver_ExtraSessionDeletedLater: as in
// TestIntakeDeliver_SessionDeletedDuringADelivery, the delivery made a
// session after a concurrent one recorded its own and the member deleted
// that, and it cannot delete the one it made. The next delivery of the
// document deletes it. A session a document names stays.
func TestIntakeDeliver_ExtraSessionDeletedLater(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	failing := &failingSessionDelete{IntakeRegistryFactory: f.factorySet.IntakeRegistryFactory}
	w := &beforeCreateSession{IntakeRegistryFactory: failing}
	f.factorySet.IntakeRegistryFactory = w
	var other *services.IntakeDeliveryResult
	w.hook = func() {
		other = f.deliver(c, paperlessDelivery("117"), nil)
		c.Assert(f.intake.Delete(f.ctx, *other.Document.SessionID), qt.IsNil)
		failing.fail.Store(true)
	}
	w.armed.Store(true)
	_, err := f.intake.Deliver(f.ctx, paperlessDelivery("117"), nil)
	failing.fail.Store(false)
	c.Assert(err, qt.ErrorIs, errLostWrite)
	sessions := must.Must(f.factorySet.IntakeRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(must.Must(sessions.List(f.ctx)), qt.HasLen, 1)

	kept := f.deliver(c, paperlessDelivery("118"), nil)
	replay := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(*replay.Document.DraftID, qt.Equals, *other.Document.DraftID)
	f.deliver(c, paperlessDelivery("118"), nil)
	left := must.Must(sessions.List(f.ctx))
	c.Assert(left, qt.HasLen, 1)
	c.Assert(left[0].ID, qt.Equals, *kept.Document.SessionID)
	c.Assert(f.drafts(c), qt.HasLen, 2)
}

// promoteOnSecondGet promotes the draft right after the first Get of it
// once armed, so the next read finds it promoted.
type promoteOnSecondGet struct {
	registry.CommodityDraftRegistryFactory
	armed   atomic.Bool
	promote func(*models.CommodityDraft)
}

type promoteOnSecondGetRegistry struct {
	registry.CommodityDraftRegistry
	f *promoteOnSecondGet
}

func (p *promoteOnSecondGet) CreateUserRegistry(ctx context.Context) (registry.CommodityDraftRegistry, error) {
	reg, err := p.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	return &promoteOnSecondGetRegistry{CommodityDraftRegistry: reg, f: p}, err
}

func (r *promoteOnSecondGetRegistry) Get(ctx context.Context, id string) (*models.CommodityDraft, error) {
	draft, err := r.CommodityDraftRegistry.Get(ctx, id)
	if err == nil && r.f.armed.CompareAndSwap(true, false) {
		r.f.promote(draft)
	}
	return draft, err
}

// TestIntakeDeliver_PromotedWhileAnswering: the draft becomes an item between
// the reads of the replay. The answer that says item names the item.
func TestIntakeDeliver_PromotedWhileAnswering(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	first := f.deliver(c, paperlessDelivery("117"), nil)
	regs := must.Must(f.factorySet.CreateUserRegistrySet(f.ctx))
	location := must.Must(regs.LocationRegistry.Create(f.ctx, models.Location{Name: "Home"}))
	area := must.Must(regs.AreaRegistry.Create(f.ctx, models.Area{Name: "Office", LocationID: location.ID}))
	orig := f.factorySet.CommodityDraftRegistryFactory
	w := &promoteOnSecondGet{CommodityDraftRegistryFactory: orig}
	var item *models.Commodity
	w.promote = func(draft *models.CommodityDraft) {
		f.factorySet.CommodityDraftRegistryFactory = orig
		defer func() { f.factorySet.CommodityDraftRegistryFactory = w }()
		var err error
		item, _, err = services.NewCommodityDraftService(f.factorySet).Promote(f.ctx, draft.ID, draft.Revision, "promotion-1", models.Commodity{
			Name: "Invoice 117", ShortName: "Invoice 117", Type: models.CommodityTypeOther, AreaID: new(area.ID),
			Count: 1, Status: models.CommodityStatusInUse, Tags: models.ValuerSlice[string]{},
		})
		c.Assert(err, qt.IsNil)
	}
	f.factorySet.CommodityDraftRegistryFactory = w
	w.armed.Store(true)

	replay := f.deliver(c, paperlessDelivery("117"), nil)
	f.factorySet.CommodityDraftRegistryFactory = orig
	c.Assert(item, qt.IsNotNil)
	c.Assert(*replay.Document.DraftID, qt.Equals, *first.Document.DraftID)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentItem)
	c.Assert(replay.Document.CommodityID, qt.IsNotNil)
	c.Assert(*replay.Document.CommodityID, qt.Equals, item.ID)
}

// TestIntakeListDocuments_OldestFirst: documents found by their draft and by
// their item come back as one list, oldest first.
func TestIntakeListDocuments_OldestFirst(t *testing.T) {
	c := qt.New(t)
	f := newIntakeFixture(c)
	docs := must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
	at := time.Date(2026, 3, 14, 12, 0, 0, 0, time.UTC)
	newer, _, err := docs.CreateOrGet(f.ctx, models.IntakeDocument{
		ExternalSystem: "paperless-ngx", ExternalDocumentID: "2", PayloadHash: "h2", DraftID: new("draft-2"), CreatedAt: at.Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)
	older, _, err := docs.CreateOrGet(f.ctx, models.IntakeDocument{
		ExternalSystem: "paperless-ngx", ExternalDocumentID: "1", PayloadHash: "h1", CommodityID: new("item-1"), CreatedAt: at,
	})
	c.Assert(err, qt.IsNil)

	got := must.Must(f.intake.ListDocuments(f.ctx, []string{"draft-2"}, []string{"item-1"}))
	c.Assert(got, qt.HasLen, 2)
	c.Assert(got[0].ID, qt.Equals, older.ID)
	c.Assert(got[1].ID, qt.Equals, newer.ID)
}
