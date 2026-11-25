package services_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// draftItem creates a live item marked as draft.
func (f *draftFixture) draftItem(c *qt.C, name string, status models.CommodityStatus) *models.Commodity {
	c.Helper()
	item := f.item(name)
	item.Status = status
	item.Draft = true
	return must.Must(f.regs.CommodityRegistry.Create(f.ctx, item))
}

type inboxRow struct {
	Kind    services.InboxKind
	Title   string
	Reasons []services.InboxReason
	Files   int
}

func inboxRows(entries []services.InboxEntry) []inboxRow {
	out := make([]inboxRow, 0, len(entries))
	for _, e := range entries {
		out = append(out, inboxRow{Kind: e.Kind, Title: e.Title, Reasons: e.Reasons, Files: e.FileCount})
	}
	return out
}

func TestInboxService_EmptyGroup(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	svc := services.NewInboxService()

	counts, err := svc.Count(f.ctx, f.regs)
	c.Assert(err, qt.IsNil)
	c.Check(counts.Total, qt.Equals, 0)

	page, err := svc.List(f.ctx, f.regs, services.InboxListOptions{Limit: 50})
	c.Assert(err, qt.IsNil)
	c.Check(page.Entries, qt.HasLen, 0)
	c.Check(page.Total, qt.Equals, 0)
}

func TestInboxService_DefaultSources(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	svc := services.NewInboxService()

	saw := f.draft(c, "key-saw", "Saw")
	drill := f.draft(c, "key-drill", "Drill")
	f.draft(c, "key-untitled", "")
	// Saving the saw makes it the most recently saved draft.
	_, err := f.drafts.Save(f.ctx, saw.ID, saw.Revision, models.CommodityDraftValues{"name": "Hand saw"})
	c.Assert(err, qt.IsNil)
	f.file(c, "saw-photo", draftLink(saw.ID))

	// A promoted draft and a discarded one are not unfinished any more.
	promoted := f.draft(c, "key-promoted", "Promoted")
	_, _, err = f.drafts.Promote(f.ctx, promoted.ID, promoted.Revision, "promote-1", f.item("Promoted"))
	c.Assert(err, qt.IsNil)
	discarded := f.draft(c, "key-discarded", "Discarded")
	c.Assert(f.drafts.Discard(f.ctx, discarded.ID), qt.IsNil)

	// Items marked as draft wait for review whatever their status; a
	// reviewed item does not, and neither does a draft item in Trash.
	f.draftItem(c, "Lamp", models.CommodityStatusInUse)
	f.draftItem(c, "Bike", models.CommodityStatusSold)
	trashed := f.draftItem(c, "Chair", models.CommodityStatusInUse)
	_, err = f.regs.CommodityRegistry.Trash(f.ctx, trashed.ID, "test", time.Now())
	c.Assert(err, qt.IsNil)
	must.Must(f.regs.CommodityRegistry.Create(f.ctx, f.item("Reviewed table")))

	page, err := svc.List(f.ctx, f.regs, services.InboxListOptions{Limit: 50})
	c.Assert(err, qt.IsNil)
	draft := []services.InboxReason{services.InboxReasonDraft}
	itemDraft := []services.InboxReason{services.InboxReasonItemDraft}
	c.Check(inboxRows(page.Entries), qt.DeepEquals, []inboxRow{
		{Kind: services.InboxKindDraft, Title: "Hand saw", Reasons: draft, Files: 1},
		{Kind: services.InboxKindDraft, Title: "", Reasons: draft},
		{Kind: services.InboxKindDraft, Title: "Drill", Reasons: draft},
		// The most recently added item first.
		{Kind: services.InboxKindCommodity, Title: "Bike", Reasons: itemDraft},
		{Kind: services.InboxKindCommodity, Title: "Lamp", Reasons: itemDraft},
	})
	c.Check(page.Total, qt.Equals, 5)
	c.Check(page.Counts, qt.DeepEquals, services.InboxCounts{
		Total:   5,
		Reasons: map[services.InboxReason]int{services.InboxReasonDraft: 3, services.InboxReasonItemDraft: 2},
	})
	c.Check(page.Entries[2].ID, qt.Equals, drill.ID)
	c.Check(page.Entries[0].UpdatedAt, qt.IsNotNil)
	c.Check(page.Entries[3].UpdatedAt, qt.IsNil)

	counts, err := svc.Count(f.ctx, f.regs)
	c.Assert(err, qt.IsNil)
	c.Check(counts, qt.DeepEquals, page.Counts)
}

func TestInboxService_ReasonFilterAndPages(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	svc := services.NewInboxService()
	f.draft(c, "key-1", "First")
	f.draftItem(c, "Lamp", models.CommodityStatusInUse)
	f.draftItem(c, "Bike", models.CommodityStatusInUse)

	page, err := svc.List(f.ctx, f.regs, services.InboxListOptions{Reason: services.InboxReasonItemDraft, Limit: 1})
	c.Assert(err, qt.IsNil)
	c.Check(inboxRows(page.Entries), qt.DeepEquals, []inboxRow{
		{Kind: services.InboxKindCommodity, Title: "Bike", Reasons: []services.InboxReason{services.InboxReasonItemDraft}},
	})
	// The filter narrows the entries, not the counts of the whole inbox.
	c.Check(page.Total, qt.Equals, 2)
	c.Check(page.Counts.Total, qt.Equals, 3)

	page, err = svc.List(f.ctx, f.regs, services.InboxListOptions{Reason: services.InboxReasonItemDraft, Offset: 1, Limit: 1})
	c.Assert(err, qt.IsNil)
	c.Check(inboxRows(page.Entries), qt.DeepEquals, []inboxRow{
		{Kind: services.InboxKindCommodity, Title: "Lamp", Reasons: []services.InboxReason{services.InboxReasonItemDraft}},
	})

	page, err = svc.List(f.ctx, f.regs, services.InboxListOptions{Offset: 10, Limit: 5})
	c.Assert(err, qt.IsNil)
	c.Check(page.Entries, qt.HasLen, 0)
	c.Check(page.Total, qt.Equals, 3)
}

// fakeSource returns fixed entries, as a later source (intake, extraction)
// would.
type fakeSource struct {
	entries []services.InboxEntry
	err     error
}

func (s fakeSource) Refs(context.Context, *registry.Set) ([]services.InboxRef, error) {
	refs := make([]services.InboxRef, 0, len(s.entries))
	for _, e := range s.entries {
		refs = append(refs, services.InboxRef{Kind: e.Kind, ID: e.ID, Reasons: e.Reasons})
	}
	return refs, s.err
}

func (s fakeSource) Collect(context.Context, *registry.Set) ([]services.InboxEntry, error) {
	return s.entries, s.err
}

func TestInboxService_OneEntryPerRecord(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	draft := f.draft(c, "key-1", "Receipt scan")
	later := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	intakeFailed := services.InboxReason("intake_failed")

	// A second source flags the same draft for another reason, names it
	// twice, and flags a record of its own.
	svc := services.NewInboxService(
		services.DraftInboxSource{},
		fakeSource{entries: []services.InboxEntry{
			{Kind: services.InboxKindDraft, ID: draft.ID, Reasons: []services.InboxReason{intakeFailed}, UpdatedAt: &later},
			{Kind: services.InboxKindDraft, ID: draft.ID, Reasons: []services.InboxReason{intakeFailed, services.InboxReasonDraft}},
			{Kind: "intake", ID: "candidate-2", Title: "Second product", Reasons: []services.InboxReason{intakeFailed}},
			{Kind: "intake", ID: "no-reason", Title: "Nothing to do"},
		}},
	)

	page, err := svc.List(f.ctx, f.regs, services.InboxListOptions{Limit: 50})
	c.Assert(err, qt.IsNil)
	c.Check(inboxRows(page.Entries), qt.DeepEquals, []inboxRow{
		{Kind: services.InboxKindDraft, Title: "Receipt scan", Reasons: []services.InboxReason{services.InboxReasonDraft, intakeFailed}},
		{Kind: "intake", Title: "Second product", Reasons: []services.InboxReason{intakeFailed}},
	})
	c.Check(*page.Entries[0].UpdatedAt, qt.Equals, later)
	c.Check(page.Counts, qt.DeepEquals, services.InboxCounts{
		Total:   2,
		Reasons: map[services.InboxReason]int{services.InboxReasonDraft: 1, intakeFailed: 2},
	})

	page, err = svc.List(f.ctx, f.regs, services.InboxListOptions{Reason: intakeFailed, Limit: 50})
	c.Assert(err, qt.IsNil)
	c.Check(page.Total, qt.Equals, 2)

	// The count, built from refs alone, merges the records the same way.
	counts, err := svc.Count(f.ctx, f.regs)
	c.Assert(err, qt.IsNil)
	c.Check(counts, qt.DeepEquals, page.Counts)
}

func TestInboxService_SourceError(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	boom := errors.New("boom")
	svc := services.NewInboxService(services.DraftInboxSource{}, fakeSource{err: boom})

	_, err := svc.Count(f.ctx, f.regs)
	c.Check(err, qt.ErrorIs, boom)
	_, err = svc.List(f.ctx, f.regs, services.InboxListOptions{Limit: 50})
	c.Check(err, qt.ErrorIs, boom)
	_, err = svc.Count(f.ctx, nil)
	c.Check(err, qt.ErrorIs, registry.ErrFieldRequired)
}

func TestInboxReason_Valid(t *testing.T) {
	c := qt.New(t)
	for _, r := range services.InboxReasons() {
		c.Check(r.Valid(), qt.IsTrue, qt.Commentf("%s", r))
	}
	c.Check(services.InboxReason("no_location").Valid(), qt.IsFalse)
	c.Check(services.InboxReason("").Valid(), qt.IsFalse)
}

func TestInboxService_IntakeSessions(t *testing.T) {
	c := qt.New(t)
	f := newDraftFixture(c)
	svc := services.NewInboxService()
	intake := f.regs.IntakeRegistry
	worker := f.factorySet.IntakeRegistryFactory.CreateServiceRegistry()
	session := func(key string, files ...string) *models.IntakeSession {
		var ids []string
		for _, name := range files {
			ids = append(ids, f.file(c, name).ID)
		}
		s, _, err := intake.CreateSession(f.ctx, models.IntakeSession{ClientKey: key}, []registry.IntakeSourceInput{{FileIDs: ids}}, 0)
		c.Assert(err, qt.IsNil)
		time.Sleep(5 * time.Millisecond)
		return s
	}
	scan := func(names ...string) {
		src := must.Must(worker.ClaimNextSource(f.ctx, time.Now().Add(time.Second)))
		var found registry.IntakeScan
		for _, name := range names {
			found.Candidates = append(found.Candidates, models.IntakeCandidate{
				Fields:   models.IntakeFields{"name": {Value: name, Confidence: 0.9}},
				Selected: true, AcceptedFields: []string{"name"}, Quantity: 1,
			})
		}
		c.Assert(worker.StoreScan(f.ctx, src.ID, src.Attempts, found, time.Now()), qt.IsNil)
	}

	// A batch whose products all have an outcome is finished.
	done := session("done", "done.jpg")
	scan("Kettle")
	cands := must.Must(intake.ListCandidates(f.ctx, done.ID))
	must.Must(intake.SetCandidateOutcome(f.ctx, cands[0].ID, models.IntakeCandidateCreated, new("draft-1"), ""))
	// A batch with a product to decide on.
	review := session("review", "receipt.jpg", "box.jpg")
	scan("Lamp", "Bulb")
	// A batch whose file could not be scanned.
	failed := session("failed", "blurry.jpg")
	src := must.Must(worker.ClaimNextSource(f.ctx, time.Now().Add(time.Second)))
	c.Assert(worker.FailSource(f.ctx, src.ID, src.Attempts, services.IntakeErrorFileUnreadable), qt.IsNil)
	// A batch still in the queue.
	queued := session("queued", "later.jpg")
	f.draft(c, "key-saw", "Saw")

	counts, err := svc.Count(f.ctx, f.regs)
	c.Assert(err, qt.IsNil)
	c.Check(counts, qt.DeepEquals, services.InboxCounts{
		Total:   4,
		Reasons: map[services.InboxReason]int{services.InboxReasonDraft: 1, services.InboxReasonIntake: 3},
	})

	page, err := svc.List(f.ctx, f.regs, services.InboxListOptions{Limit: 50})
	c.Assert(err, qt.IsNil)
	c.Check(page.Counts, qt.DeepEquals, counts)
	intakeReason := []services.InboxReason{services.InboxReasonIntake}
	c.Check(inboxRows(page.Entries), qt.DeepEquals, []inboxRow{
		{Kind: services.InboxKindDraft, Title: "Saw", Reasons: []services.InboxReason{services.InboxReasonDraft}},
		// The batches come last, newest first.
		{Kind: services.InboxKindIntakeSession, Reasons: intakeReason, Files: 1},
		{Kind: services.InboxKindIntakeSession, Reasons: intakeReason, Files: 1},
		{Kind: services.InboxKindIntakeSession, Reasons: intakeReason, Files: 2},
	})
	ids := []string{page.Entries[1].ID, page.Entries[2].ID, page.Entries[3].ID}
	c.Check(ids, qt.DeepEquals, []string{queued.ID, failed.ID, review.ID})
	c.Check(page.Entries[1].Intake.State(), qt.Equals, models.IntakeSessionProcessing)
	c.Check(page.Entries[2].Intake.Failed, qt.Equals, 1)
	c.Check(page.Entries[3].Intake.Undecided, qt.Equals, 2)
	c.Check(page.Entries[3].UpdatedAt, qt.IsNotNil)
	c.Check(page.Entries[0].Intake, qt.IsNil)

	page, err = svc.List(f.ctx, f.regs, services.InboxListOptions{Reason: services.InboxReasonIntake, Limit: 1, Offset: 2})
	c.Assert(err, qt.IsNil)
	c.Check(page.Total, qt.Equals, 3)
	c.Check(page.Entries, qt.HasLen, 1)
	c.Check(page.Entries[0].ID, qt.Equals, review.ID)
}
