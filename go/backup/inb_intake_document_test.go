//go:build !legacy_xml_backup

package backup_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// The record of the documents other systems delivered is in a whole-group
// backup (#2809, format 3.7), so a delivery repeated after a restore makes no
// second draft.

// deliveredFixture is an inbFixture with three delivered documents: one whose
// draft waits in the inbox, one whose draft became an item, and one whose
// draft was discarded.
type deliveredFixture struct {
	*inbFixture
	intake    *services.IntakeService
	open      *models.IntakeDocument
	finished  *models.IntakeDocument
	discarded *models.IntakeDocument
	itemUUID  string
}

func delivery(id string) services.IntakeDelivery {
	return services.IntakeDelivery{
		Title:        "Invoice " + id,
		SourceSystem: "paperless-ngx",
		DocumentID:   id,
		SourceURL:    "https://paperless.example/documents/" + id + "/",
		SourceName:   "Paperless-ngx",
	}
}

func newDeliveredFixture(c *qt.C) *deliveredFixture {
	f := newInbFixture(c)
	intake := services.NewIntakeService(f.fs, nil, services.IntakeConfig{})
	drafts := services.NewCommodityDraftService(f.fs)
	deliver := func(id string) *models.IntakeDocument {
		result, err := intake.Deliver(f.ctx, delivery(id), nil)
		c.Assert(err, qt.IsNil)
		c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
		return result.Document
	}
	open := deliver("1")
	finished := deliver("2")
	discarded := deliver("3")

	draftReg := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	draft := must.Must(draftReg.Get(f.ctx, *finished.DraftID))
	item, _, err := drafts.Promote(f.ctx, draft.ID, draft.Revision, "promotion-1", models.Commodity{
		Name: "Invoice 2", ShortName: "invoice", Type: models.CommodityTypeOther, Count: 1,
		Status: models.CommodityStatusInUse, OriginalPriceCurrency: "USD", AreaID: new(f.areaID),
	})
	c.Assert(err, qt.IsNil)
	c.Assert(drafts.Discard(f.ctx, *discarded.DraftID), qt.IsNil)

	return &deliveredFixture{inbFixture: f, intake: intake, open: open, finished: finished, discarded: discarded, itemUUID: item.UUID}
}

func (f *deliveredFixture) documents(c *qt.C) map[string]*models.IntakeDocument {
	reg := must.Must(f.fs.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
	out := map[string]*models.IntakeDocument{}
	for _, doc := range must.Must(reg.List(f.ctx)) {
		out[doc.ExternalDocumentID] = doc
	}
	return out
}

func TestINBExport_IntakeDocuments(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)

	_, archive := f.runExport(c, signer)
	order, jsons := innerMembers(c, archive)

	var manifest types.INBManifest
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Check(manifest.Version, qt.Equals, export.INBFormatVersion)
	c.Check(manifest.Version, qt.Equals, "3.7")
	c.Check(manifest.IntakeDocumentsFile, qt.Equals, types.INBIntakeDocumentsMember)

	// After the drafts, before the files document.
	position := map[string]int{}
	for i, name := range order {
		position[name] = i
	}
	at, ok := position[types.INBIntakeDocumentsMember]
	c.Assert(ok, qt.IsTrue, qt.Commentf("members: %v", order))
	c.Check(position[types.INBDraftsMember] < at, qt.IsTrue)
	if files, ok := position[types.INBFilesMember]; ok {
		c.Check(at < files, qt.IsTrue)
	}

	var doc types.INBIntakeDocumentsDoc
	c.Assert(json.Unmarshal(jsons[types.INBIntakeDocumentsMember], &doc), qt.IsNil)
	c.Assert(doc.Documents, qt.HasLen, 3)
	byID := map[string]types.INBIntakeDocument{}
	for _, d := range doc.Documents {
		byID[d.ExternalDocumentID] = d
	}
	openDraft := must.Must(must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx)).Get(f.ctx, *f.open.DraftID))
	c.Check(byID["1"].ID, qt.Equals, f.open.UUID)
	c.Check(byID["1"].DraftID, qt.Equals, openDraft.UUID)
	c.Check(byID["1"].DraftMade, qt.IsTrue)
	c.Check(byID["1"].PayloadHash, qt.Equals, f.open.PayloadHash)
	c.Check(byID["1"].SourceURL, qt.Equals, "https://paperless.example/documents/1/")
	c.Check(byID["1"].CreatedAt, qt.Equals, f.open.CreatedAt.UTC().Format(time.RFC3339))
	// The promoted draft is not in the archive, the item is.
	c.Check(byID["2"].DraftID, qt.Equals, "")
	c.Check(byID["2"].CommodityID, qt.Equals, f.itemUUID)
	c.Check(byID["2"].DraftMade, qt.IsTrue)
	c.Check(byID["3"].DraftID, qt.Equals, "")
	c.Check(byID["3"].CommodityID, qt.Equals, "")
	c.Check(byID["3"].DraftMade, qt.IsTrue)
	// Database ids stay out of the archive.
	c.Check(string(jsons[types.INBIntakeDocumentsMember]), qt.Not(qt.Contains), f.open.ID)
	c.Check(string(jsons[types.INBIntakeDocumentsMember]), qt.Not(qt.Contains), *f.open.SessionID)
}

func TestINBExport_SelectedItemsCarryNoIntakeDocuments(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)

	tv := f.commodityByUUID(c, f.itemUUID)
	_, archive := f.runExportRow(c, signer, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
		Type:                     models.ExportTypeSelectedItems,
		Status:                   models.ExportStatusPending,
		Description:              "selected backup",
		SelectedItems: models.ValuerSlice[models.ExportSelectedItem]{
			{ID: tv.ID, Type: models.ExportSelectedItemTypeCommodity, Name: tv.Name},
		},
	})
	order, jsons := innerMembers(c, archive)
	c.Check(order, qt.Not(qt.Contains), types.INBIntakeDocumentsMember)
	var manifest types.INBManifest
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Check(manifest.IntakeDocumentsFile, qt.Equals, "")
}

// TestINBRoundTrip_IntakeDocumentsAreRestored: a full replace recreates the
// record with the restored draft and item, and a delivery repeated after it
// makes nothing, also for the document whose draft is gone.
func TestINBRoundTrip_IntakeDocumentsAreRestored(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)
	blobKey, _ := f.runExport(c, signer)

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	docs := f.documents(c)
	c.Assert(docs, qt.HasLen, 3)
	drafts := must.Must(must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx))
	c.Assert(drafts, qt.HasLen, 1)
	c.Check(docs["1"].ID, qt.Not(qt.Equals), f.open.ID)
	c.Check(docs["1"].PayloadHash, qt.Equals, f.open.PayloadHash)
	c.Check(docs["1"].SourceName, qt.Equals, "Paperless-ngx")
	c.Check(docs["1"].CreatedAt.Equal(f.open.CreatedAt.Truncate(time.Second)), qt.IsTrue)
	c.Assert(docs["1"].DraftID, qt.IsNotNil)
	c.Check(*docs["1"].DraftID, qt.Equals, drafts[0].ID)
	c.Check(docs["1"].SessionID, qt.IsNil)
	c.Assert(docs["2"].CommodityID, qt.IsNotNil)
	c.Check(*docs["2"].CommodityID, qt.Equals, f.commodityByUUID(c, f.itemUUID).ID)
	// The draft made for document 3 is gone; the record still says one was
	// made.
	c.Assert(docs["3"].DraftID, qt.IsNotNil)
	c.Check(*docs["3"].DraftID, qt.Equals, "")

	want := map[string]models.IntakeDocumentState{
		"1": models.IntakeDocumentDraft,
		"2": models.IntakeDocumentItem,
		"3": models.IntakeDocumentRemoved,
	}
	for id, state := range want {
		result, err := f.intake.Deliver(f.ctx, delivery(id), nil)
		c.Assert(err, qt.IsNil)
		c.Check(result.Created, qt.IsFalse)
		c.Check(result.State, qt.Equals, state, qt.Commentf("document %s", id))
	}
	c.Check(must.Must(must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx)), qt.HasLen, 1)

	steps := must.Must(f.fs.RestoreStepRegistryFactory.CreateServiceRegistry().ListByRestoreOperation(f.ctx, final.ID))
	var reasons []string
	for _, s := range steps {
		if s.Name == "Delivered documents" {
			reasons = append(reasons, s.Reason)
		}
	}
	c.Check(reasons, qt.DeepEquals, []string{"3 restored, 0 kept as they were, 0 failed"})
}

// TestINBRestore_MergeKeepsTheGroupsIntakeDocument: a merge keeps the group's
// record of a document and brings back the one the group no longer has.
func TestINBRestore_MergeKeepsTheGroupsIntakeDocument(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)
	blobKey, _ := f.runExport(c, signer)

	reg := must.Must(f.fs.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(reg.Delete(f.ctx, f.discarded.ID), qt.IsNil)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	docs := f.documents(c)
	c.Assert(docs, qt.HasLen, 3)
	c.Check(docs["1"].ID, qt.Equals, f.open.ID)
	c.Check(docs["2"].ID, qt.Equals, f.finished.ID)
	c.Check(docs["3"].ID, qt.Not(qt.Equals), f.discarded.ID)
	c.Assert(docs["3"].DraftID, qt.IsNotNil)
	c.Check(*docs["3"].DraftID, qt.Equals, "")
}

func TestINBRestore_DryRunWritesNoIntakeDocuments(t *testing.T) {
	for _, strategy := range []types.RestoreStrategy{types.RestoreStrategyFullReplace, types.RestoreStrategyMergeAdd} {
		t.Run(string(strategy), func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newDeliveredFixture(c)
			blobKey, _ := f.runExport(c, signer)
			before := f.documents(c)

			final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(strategy), DryRun: true})
			c.Assert(err, qt.IsNil)
			c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

			after := f.documents(c)
			c.Assert(after, qt.HasLen, len(before))
			for id, doc := range before {
				c.Check(after[id].ID, qt.Equals, doc.ID)
			}
		})
	}
}

// TestINBRestore_FullReplaceFromAnArchiveWithoutIntakeDocuments: a full
// replace from a backup made before the record was in backups removes it
// with the drafts and items it named.
func TestINBRestore_FullReplaceFromAnArchiveWithoutIntakeDocuments(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)

	key := versionGateArchive(c, f.inbFixture, signer, `{"version":"3.5"}`)
	final, err := restoreInb(c, f.inbFixture, signer, key)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Check(f.documents(c), qt.HasLen, 0)
}

// TestINBRestore_MergeNamesTheKeptDraft: the group lost its record of two
// documents but kept their drafts, which a merge keeps instead of the
// archived copies. Each restored record names the group's draft: the open
// one, and the one that became an item with its item.
func TestINBRestore_MergeNamesTheKeptDraft(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)
	promoted := must.Must(f.intake.Deliver(f.ctx, delivery("4"), nil)).Document
	blobKey, _ := f.runExport(c, signer)

	drafts := services.NewCommodityDraftService(f.fs)
	draftReg := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	draft := must.Must(draftReg.Get(f.ctx, *promoted.DraftID))
	item, _, err := drafts.Promote(f.ctx, draft.ID, draft.Revision, "promotion-4", models.Commodity{
		Name: "Invoice 4", ShortName: "invoice", Type: models.CommodityTypeOther, Count: 1,
		Status: models.CommodityStatusInUse, OriginalPriceCurrency: "USD", AreaID: new(f.areaID),
	})
	c.Assert(err, qt.IsNil)
	reg := must.Must(f.fs.IntakeDocumentRegistryFactory.CreateUserRegistry(f.ctx))
	for _, doc := range []*models.IntakeDocument{f.open, promoted} {
		c.Assert(reg.Delete(f.ctx, doc.ID), qt.IsNil)
	}

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	docs := f.documents(c)
	c.Assert(docs["1"].DraftID, qt.IsNotNil)
	c.Check(*docs["1"].DraftID, qt.Equals, *f.open.DraftID)
	c.Assert(docs["4"].DraftID, qt.IsNotNil)
	c.Check(*docs["4"].DraftID, qt.Equals, *promoted.DraftID)
	c.Assert(docs["4"].CommodityID, qt.IsNotNil)
	c.Check(*docs["4"].CommodityID, qt.Equals, item.ID)
	states, err := f.intake.DocumentStates(f.ctx, []*models.IntakeDocument{docs["1"], docs["4"]})
	c.Assert(err, qt.IsNil)
	c.Check(states, qt.DeepEquals, map[string]models.IntakeDocumentState{
		docs["1"].ID: models.IntakeDocumentDraft,
		docs["4"].ID: models.IntakeDocumentItem,
	})
	// No archived draft came back next to the kept ones.
	open := 0
	for _, d := range must.Must(draftReg.List(f.ctx)) {
		if d.Status == models.CommodityDraftOpen {
			open++
		}
	}
	c.Check(open, qt.Equals, 1)
}

// promoteOnList promotes a draft the first time the export lists the drafts,
// as a promotion that commits between the documents read and the drafts
// read would.
type promoteOnList struct {
	registry.CommodityDraftRegistryFactory
	once sync.Once
	run  func()
}

type promoteOnListRegistry struct {
	registry.CommodityDraftRegistry
	f *promoteOnList
}

func (p *promoteOnList) CreateUserRegistry(ctx context.Context) (registry.CommodityDraftRegistry, error) {
	reg, err := p.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	return &promoteOnListRegistry{CommodityDraftRegistry: reg, f: p}, err
}

func (r *promoteOnListRegistry) List(ctx context.Context) ([]*models.CommodityDraft, error) {
	r.f.once.Do(r.f.run)
	return r.CommodityDraftRegistry.List(ctx)
}

// A promotion between the documents read and the drafts read: the document
// was read naming the open draft, the drafts read finds it promoted. The
// archive names the item, so the restored item keeps its document.
func TestINBExport_PromotionBetweenTheReads(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)
	orig := f.fs.CommodityDraftRegistryFactory
	wrap := &promoteOnList{CommodityDraftRegistryFactory: orig}
	var item *models.Commodity
	wrap.run = func() {
		draft := must.Must(must.Must(orig.CreateUserRegistry(f.ctx)).Get(f.ctx, *f.open.DraftID))
		f.fs.CommodityDraftRegistryFactory = orig
		defer func() { f.fs.CommodityDraftRegistryFactory = wrap }()
		var err error
		item, _, err = services.NewCommodityDraftService(f.fs).Promote(f.ctx, draft.ID, draft.Revision, "promotion-between", models.Commodity{
			Name: "Invoice 1", ShortName: "invoice", Type: models.CommodityTypeOther, Count: 1,
			Status: models.CommodityStatusInUse, OriginalPriceCurrency: "USD", AreaID: new(f.areaID),
		})
		c.Assert(err, qt.IsNil)
	}
	f.fs.CommodityDraftRegistryFactory = wrap
	blobKey, _ := f.runExport(c, signer)
	f.fs.CommodityDraftRegistryFactory = orig
	c.Assert(item, qt.IsNotNil)

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	restored := f.commodityByUUID(c, item.UUID)
	c.Assert(restored, qt.IsNotNil)
	docs := must.Must(f.intake.ListDocuments(f.ctx, nil, []string{restored.ID}))
	c.Assert(docs, qt.HasLen, 1)
	states := must.Must(f.intake.DocumentStates(f.ctx, docs))
	c.Assert(states[docs[0].ID], qt.Equals, models.IntakeDocumentItem)
}

// draftNotRecordedYet lists the documents as an export reads them while a
// delivery has made the draft of document "1" and not recorded it yet.
type draftNotRecordedYet struct {
	registry.IntakeDocumentRegistryFactory
}

type draftNotRecordedYetRegistry struct {
	registry.IntakeDocumentRegistry
}

func (d *draftNotRecordedYet) CreateUserRegistry(ctx context.Context) (registry.IntakeDocumentRegistry, error) {
	reg, err := d.IntakeDocumentRegistryFactory.CreateUserRegistry(ctx)
	return &draftNotRecordedYetRegistry{IntakeDocumentRegistry: reg}, err
}

func (r *draftNotRecordedYetRegistry) List(ctx context.Context) ([]*models.IntakeDocument, error) {
	docs, err := r.IntakeDocumentRegistry.List(ctx)
	for i, doc := range docs {
		if doc.ExternalDocumentID == "1" {
			docs[i] = new(*doc)
			docs[i].DraftID = nil
		}
	}
	return docs, err
}

// A document exported before its delivery recorded the draft: the archive
// carries the draft, the record names none. The delivery repeated after the
// restore takes the restored draft and makes no session.
func TestINBRoundTrip_DraftNotRecordedAtTheExport(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDeliveredFixture(c)
	orig := f.fs.IntakeDocumentRegistryFactory
	f.fs.IntakeDocumentRegistryFactory = &draftNotRecordedYet{orig}
	blobKey, _ := f.runExport(c, signer)
	f.fs.IntakeDocumentRegistryFactory = orig

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(f.documents(c)["1"].DraftID, qt.IsNil)
	drafts := must.Must(must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx))
	c.Assert(drafts, qt.HasLen, 1)
	sessions := must.Must(f.fs.IntakeRegistryFactory.CreateUserRegistry(f.ctx))
	before := len(must.Must(sessions.List(f.ctx)))

	result, err := f.intake.Deliver(f.ctx, delivery("1"), nil)
	c.Assert(err, qt.IsNil)
	c.Assert(result.State, qt.Equals, models.IntakeDocumentDraft)
	c.Assert(*result.Document.DraftID, qt.Equals, drafts[0].ID)
	c.Assert(must.Must(sessions.List(f.ctx)), qt.HasLen, before)
}
