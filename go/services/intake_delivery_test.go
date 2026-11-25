package services_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/models"
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

	// Forget the draft as if the call failed before recording it: drop the
	// record and recreate it with only the session.
	docs := must.Must(f.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(docs.Delete(f.ctx, first.Document.ID), qt.IsNil)
	half := *first.Document
	half.DraftID = nil
	stored, _, err := docs.CreateOrGet(f.ctx, half)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DraftID, qt.IsNil)

	replay := f.deliver(c, paperlessDelivery("117"), nil)
	c.Assert(replay.Created, qt.IsFalse)
	c.Assert(replay.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(*replay.Document.DraftID, qt.Equals, *first.Document.DraftID)
	c.Assert(f.drafts(c), qt.HasLen, 1)
}
