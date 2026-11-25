//go:build !legacy_xml_backup

package backup_test

import (
	"encoding/json"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// The record of the documents other systems delivered is in a whole-group
// backup (#2809, format 3.6), so a delivery repeated after a restore makes no
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
	c.Check(manifest.Version, qt.Equals, "3.6")
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
