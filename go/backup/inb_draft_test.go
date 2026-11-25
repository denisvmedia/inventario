//go:build !legacy_xml_backup

package backup_test

import (
	"archive/tar"
	"encoding/json"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// Open drafts of new items are in a whole-group backup (#2812, format 3.4),
// with the links of their files.

// draftFixture is an inbFixture with an open draft that names the fixture's
// area, a seller, tags and a type, a photo only the draft uses, and a
// receipt the draft shares with the TV. It also has a promoted draft and a
// draft being discarded, which a backup leaves out.
type draftFixture struct {
	*inbFixture
	draft      *models.CommodityDraft
	seller     *models.Seller
	areaUUID   string
	tvUUID     string
	photo      string
	receipt    string
	promoted   *models.CommodityDraft
	discarding *models.CommodityDraft
}

func newDraftFixture(c *qt.C) *draftFixture {
	f := newInbFixture(c)
	sellers := must.Must(f.fs.SellerRegistryFactory.CreateUserRegistry(f.ctx))
	seller := must.Must(sellers.Create(f.ctx, models.Seller{Name: "Drill Shop"}))
	area := must.Must(must.Must(f.fs.AreaRegistryFactory.CreateUserRegistry(f.ctx)).Get(f.ctx, f.areaID))
	tv := must.Must(must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx)).List(f.ctx))[0]

	drafts := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	draft, _ := must.Must2(drafts.CreateOrGet(f.ctx, models.CommodityDraft{
		ClientKey: "draft-in-backup",
		Values:    models.CommodityDraftValues{"name": "Half-done drill"},
	}))
	// Two saves, so the revision the archive keeps is not the first one.
	draft = must.Must(drafts.SaveValues(f.ctx, draft.ID, 1, models.CommodityDraftValues{"name": "Half-done drill"}))
	draft = must.Must(drafts.SaveValues(f.ctx, draft.ID, 2, models.CommodityDraftValues{
		"name":       "Half-done drill",
		"area_id":    f.areaID,
		"seller_id":  seller.ID,
		"tags":       []any{"Power Tools"},
		"type":       "equipment",
		"count":      "2",
		"client_tip": "kept as given",
	}))

	photo := f.attachEntityFile(c, models.FileLinkDraft, draft.ID, models.FileLinkMetaImages, "drill-photo", ".jpg", "image/jpeg", 128)
	receipt := f.attachCommodityFileFull(c, models.FileLinkMetaDocuments, "receipt", "receipt", ".pdf", "application/pdf", 256)
	receiptRow := f.fileByUUID(c, receipt)
	_, _, err := drafts.AttachFile(f.ctx, draft.ID, models.FileLink{FileID: receiptRow.ID, EntityType: models.FileLinkDraft, EntityID: draft.ID, Meta: models.FileLinkMetaDocuments})
	c.Assert(err, qt.IsNil)

	promoted, _ := must.Must2(drafts.CreateOrGet(f.ctx, models.CommodityDraft{ClientKey: "promoted-draft", Values: models.CommodityDraftValues{"name": "Finished lamp"}}))
	must.Must(drafts.Promote(f.ctx, promoted.ID, 1, "promotion-1", models.Commodity{
		Name: "Finished lamp", ShortName: "lamp", Type: models.CommodityTypeFurniture, Count: 1,
		Status: models.CommodityStatusInUse, OriginalPriceCurrency: "USD", Draft: true,
	}))
	discarding, _ := must.Must2(drafts.CreateOrGet(f.ctx, models.CommodityDraft{ClientKey: "discarding-draft", Values: models.CommodityDraftValues{"name": "Old idea"}}))
	must.Must(drafts.MarkDiscarding(f.ctx, discarding.ID))

	return &draftFixture{
		inbFixture: f, draft: draft, seller: seller, areaUUID: area.UUID, tvUUID: tv.UUID,
		photo: photo, receipt: receipt, promoted: promoted, discarding: discarding,
	}
}

func (f *draftFixture) draftsByKey(c *qt.C) map[string]*models.CommodityDraft {
	drafts := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	out := map[string]*models.CommodityDraft{}
	for _, d := range must.Must(drafts.List(f.ctx)) {
		out[d.ClientKey] = d
	}
	return out
}

func draftLink(file *models.FileEntity, draftID string) *models.FileLink {
	return file.LinkTo(models.FileLinkDraft, draftID)
}

func TestINBExport_Drafts(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDraftFixture(c)

	_, archive := f.runExport(c, signer)
	order, jsons := innerMembers(c, archive)

	var manifest types.INBManifest
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Check(manifest.Version, qt.Equals, "3.4")
	c.Check(manifest.DraftsFile, qt.Equals, types.INBDraftsMember)

	// After the location documents, before the files document.
	position := map[string]int{}
	for i, name := range order {
		position[name] = i
	}
	draftsAt, ok := position[types.INBDraftsMember]
	c.Assert(ok, qt.IsTrue)
	for name, i := range position {
		if strings.HasPrefix(name, "location-") {
			c.Check(i < draftsAt, qt.IsTrue, qt.Commentf("%s comes after the drafts", name))
		}
	}
	c.Check(draftsAt < position[types.INBFilesMember], qt.IsTrue)

	var doc types.INBDraftsDoc
	c.Assert(json.Unmarshal(jsons[types.INBDraftsMember], &doc), qt.IsNil)
	c.Assert(doc.Drafts, qt.HasLen, 1, qt.Commentf("only the open draft is exported"))
	got := doc.Drafts[0]
	c.Check(got.ID, qt.Equals, f.draft.UUID)
	c.Check(got.ClientKey, qt.Equals, "draft-in-backup")
	c.Check(got.Revision, qt.Equals, 3)
	c.Check(got.CreatedAt, qt.Equals, f.draft.CreatedAt.UTC().Format(time.RFC3339))
	c.Check(got.UpdatedAt, qt.Equals, f.draft.UpdatedAt.UTC().Format(time.RFC3339))
	c.Check(got.Values, qt.DeepEquals, models.CommodityDraftValues{
		"name":       "Half-done drill",
		"area_id":    f.areaUUID,
		"seller_id":  f.seller.UUID,
		"tags":       []any{"Power Tools"},
		"type":       "equipment",
		"count":      "2",
		"client_tip": "kept as given",
	})
	c.Check(manifest.Sellers, qt.HasLen, 1)

	var files types.INBFilesDoc
	c.Assert(json.Unmarshal(jsons[types.INBFilesMember], &files), qt.IsNil)
	refs := map[string]types.INBEntityFileRef{}
	for _, ref := range files.Files {
		if ref.LinkedEntityType == models.FileLinkDraft {
			c.Check(ref.LinkedEntityID, qt.Equals, f.draft.UUID)
			refs[ref.ID] = ref
		}
	}
	c.Assert(refs, qt.HasLen, 2)
	c.Check(refs[f.photo].LinkedEntityMeta, qt.Equals, models.FileLinkMetaImages)
	c.Check(strings.HasPrefix(refs[f.photo].Path, "files/_entity/draft/"+f.draft.UUID+"/images/"), qt.IsTrue, qt.Commentf("path %s", refs[f.photo].Path))
	c.Check(refs[f.receipt].LinkedEntityMeta, qt.Equals, models.FileLinkMetaDocuments)
	// The receipt's bytes are stored once, under the TV.
	c.Check(strings.Contains(refs[f.receipt].Path, "/"+f.tvUUID+"/documents/"), qt.IsTrue, qt.Commentf("path %s", refs[f.receipt].Path))

	for name, data := range jsons {
		for _, d := range []*models.CommodityDraft{f.promoted, f.discarding} {
			c.Check(string(data), qt.Not(qt.Contains), d.ClientKey, qt.Commentf("%s carries draft %s", name, d.ClientKey))
			c.Check(string(data), qt.Not(qt.Contains), d.UUID, qt.Commentf("%s carries draft %s", name, d.ClientKey))
		}
	}
}

func TestINBExport_SelectedItemsCarryNoDrafts(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDraftFixture(c)

	tv := f.commodityByUUID(c, f.tvUUID)
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
	c.Check(order, qt.Not(qt.Contains), types.INBDraftsMember)
	var manifest types.INBManifest
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	c.Check(manifest.DraftsFile, qt.Equals, "")
	for name, data := range jsons {
		c.Check(string(data), qt.Not(qt.Contains), f.draft.UUID, qt.Commentf("%s names the draft", name))
		c.Check(string(data), qt.Not(qt.Contains), f.photo, qt.Commentf("%s carries the draft's photo", name))
	}
}

// TestINBRoundTrip_DraftsAreRestored is the memory half of the round trip; the
// postgres half is in go/integration. A full replace removes the group's
// drafts and recreates the archive's open drafts with new ids.
func TestINBRoundTrip_DraftsAreRestored(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDraftFixture(c)
	blobKey, _ := f.runExport(c, signer)

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	byKey := f.draftsByKey(c)
	c.Assert(byKey, qt.HasLen, 1, qt.Commentf("the promoted and the discarding draft are gone, the open one is back"))
	restored := byKey["draft-in-backup"]
	c.Assert(restored, qt.IsNotNil)
	c.Check(restored.ID, qt.Not(qt.Equals), f.draft.ID)
	c.Check(restored.UUID, qt.Not(qt.Equals), f.draft.UUID)
	c.Check(restored.Status, qt.Equals, models.CommodityDraftOpen)
	c.Check(restored.Revision, qt.Equals, 3)
	c.Check(restored.CreatedAt.Equal(f.draft.CreatedAt.Truncate(time.Second)), qt.IsTrue, qt.Commentf("created %v, want %v", restored.CreatedAt, f.draft.CreatedAt))
	c.Check(restored.UpdatedAt.Equal(f.draft.UpdatedAt.Truncate(time.Second)), qt.IsTrue, qt.Commentf("updated %v, want %v", restored.UpdatedAt, f.draft.UpdatedAt))
	c.Check(restored.CreatedByUserID, qt.Equals, f.user.ID)
	c.Check(restored.Values, qt.DeepEquals, models.CommodityDraftValues{
		"name":       "Half-done drill",
		"area_id":    f.areaByUUID(c, f.areaUUID).ID,
		"seller_id":  f.seller.ID,
		"tags":       []any{"power-tools"},
		"type":       "equipment",
		"count":      "2",
		"client_tip": "kept as given",
	})
	tags := must.Must(f.fs.TagRegistryFactory.CreateUserRegistry(f.ctx))
	tag, err := tags.GetBySlug(f.ctx, models.TagKindCommodity, "power-tools")
	c.Assert(err, qt.IsNil, qt.Commentf("the restore creates the draft's tag, as for an item"))
	c.Check(tag.Label, qt.Equals, "Power Tools")

	photo := f.fileByUUID(c, f.photo)
	c.Assert(photo.Links, qt.HasLen, 1)
	c.Check(photo.Links[0].EntityType, qt.Equals, models.FileLinkDraft)
	c.Check(photo.Links[0].EntityID, qt.Equals, restored.ID)
	c.Check(photo.Links[0].Meta, qt.Equals, models.FileLinkMetaImages)
	c.Check(photo.Category, qt.Equals, models.FileCategoryImages)

	receipt := f.fileByUUID(c, f.receipt)
	c.Check(receipt.Links, qt.HasLen, 2)
	c.Check(draftLink(receipt, restored.ID), qt.IsNotNil)
	c.Check(receipt.LinkTo(models.FileLinkCommodity, f.commodityByUUID(c, f.tvUUID).ID), qt.IsNotNil)
}

// TestINBRestore_MergeKeepsTheGroupsDraft: a merge does not overwrite a draft
// the group has with the same client key; it keeps it and reports it.
func TestINBRestore_MergeKeepsTheGroupsDraft(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newDraftFixture(c)
	drafts := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	other, _ := must.Must2(drafts.CreateOrGet(f.ctx, models.CommodityDraft{ClientKey: "second-draft", Values: models.CommodityDraftValues{"name": "Shelf"}}))
	blobKey, _ := f.runExport(c, signer)

	// After the backup: the first draft moves on and loses its photo for
	// good, and the second draft is discarded.
	newer := must.Must(drafts.SaveValues(f.ctx, f.draft.ID, 3, models.CommodityDraftValues{"name": "Finished drill?"}))
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(fileReg.Delete(f.ctx, f.fileByUUID(c, f.photo).ID), qt.IsNil)
	c.Assert(drafts.Delete(f.ctx, other.ID), qt.IsNil)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	byKey := f.draftsByKey(c)
	kept := byKey["draft-in-backup"]
	c.Assert(kept, qt.IsNotNil)
	c.Check(kept.ID, qt.Equals, f.draft.ID)
	c.Check(kept.Revision, qt.Equals, newer.Revision)
	c.Check(kept.Values, qt.DeepEquals, models.CommodityDraftValues{"name": "Finished drill?"})
	c.Assert(byKey["second-draft"], qt.IsNotNil, qt.Commentf("a draft the group no longer has is restored"))
	c.Check(byKey["second-draft"].ID, qt.Not(qt.Equals), other.ID)

	// The photo only the kept draft used comes back as a standalone file.
	photo := f.fileByUUID(c, f.photo)
	c.Check(photo.Links, qt.HasLen, 0)

	steps := must.Must(f.fs.RestoreStepRegistryFactory.CreateServiceRegistry().ListByRestoreOperation(f.ctx, final.ID))
	var skipped []string
	for _, s := range steps {
		if s.Result == models.RestoreStepResultSkipped && strings.HasPrefix(s.Name, "Draft: ") {
			skipped = append(skipped, s.Name)
		}
	}
	c.Check(skipped, qt.DeepEquals, []string{"Draft: Half-done drill"})
}

// TestINBRestore_DraftReferencesThatDoNotMap: an area, a seller or a type the
// restore cannot map is left out of the draft, never stored as given.
func TestINBRestore_DraftReferencesThatDoNotMap(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	loc := must.Must(must.Must(f.fs.LocationRegistryFactory.CreateUserRegistry(f.ctx)).Get(f.ctx, f.locID))

	draftsDoc := types.INBDraftsDoc{Drafts: []types.INBDraft{{
		ID:        "archived-draft",
		ClientKey: "hand-built",
		Revision:  4,
		Values: models.CommodityDraftValues{
			"name":      "Kettle",
			"area_id":   f.areaID, // a DB id, not an archived area
			"seller_id": "no-such-seller",
			"type":      "spaceship",
			"status":    "in_use",
		},
	}}}
	archive := signedArchiveFromInnerTar(c, signer, func(tw *tar.Writer) {
		writeMember(c, tw, &tar.Header{Name: "manifest.json", Mode: 0o600, Typeflag: tar.TypeReg}, []byte(`{"version":"3.4"}`))
		writeMember(c, tw, &tar.Header{Name: "location-home.json", Mode: 0o600, Typeflag: tar.TypeReg}, must.Must(json.Marshal(inbLocationOnlyDoc(loc))))
		writeMember(c, tw, &tar.Header{Name: types.INBDraftsMember, Mode: 0o600, Typeflag: tar.TypeReg}, must.Must(json.Marshal(draftsDoc)))
	})
	key := "t/tenant-a/restores/drafts-unmapped.inb"
	writeArchive(c, f, key, archive)

	final, err := restoreInbWithOptions(c, f, signer, key, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	drafts := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	restored := must.Must(drafts.GetByClientKey(f.ctx, "hand-built"))
	c.Check(restored.Values, qt.DeepEquals, models.CommodityDraftValues{"name": "Kettle", "status": "in_use"})
	c.Check(restored.Revision, qt.Equals, 4)
}

// TestINBRestore_DraftsIntoAnotherGroup restores a group's backup into another
// group of the tenant. The draft is linked to the copies of its files the
// restore makes in that group, never to the files of the group the backup
// came from, and its area and seller are the ones the restore made there.
func TestINBRestore_DraftsIntoAnotherGroup(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	src := newDraftFixture(c)
	blobKey, _ := src.runExport(c, signer)

	groupB := must.Must(src.fs.LocationGroupRegistry.Create(src.ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-a"},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Group B",
		Status:              models.LocationGroupStatusActive,
		GroupCurrency:       "USD",
		CreatedBy:           src.user.ID,
	}))
	dst := &inbFixture{fs: src.fs, ctx: appctx.WithGroup(appctx.WithUser(src.ctx, src.user), groupB), user: src.user, group: groupB}

	final, err := restoreInbWithOptions(c, dst, signer, blobKey, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	drafts := must.Must(src.fs.CommodityDraftRegistryFactory.CreateUserRegistry(dst.ctx))
	restored := must.Must(drafts.GetByClientKey(dst.ctx, "draft-in-backup"))
	c.Check(restored.GroupID, qt.Equals, groupB.ID)

	areas := must.Must(must.Must(src.fs.AreaRegistryFactory.CreateUserRegistry(dst.ctx)).List(dst.ctx))
	c.Assert(areas, qt.HasLen, 1)
	c.Check(restored.Values["area_id"], qt.Equals, areas[0].ID)
	c.Check(areas[0].GroupID, qt.Equals, groupB.ID)
	sellerB := must.Must(must.Must(src.fs.SellerRegistryFactory.CreateUserRegistry(dst.ctx)).GetByName(dst.ctx, "Drill Shop"))
	c.Check(sellerB.ID, qt.Not(qt.Equals), src.seller.ID)
	c.Check(restored.Values["seller_id"], qt.Equals, sellerB.ID)

	sourcePhoto := src.fileByUUID(c, src.photo)
	linked := must.Must(must.Must(src.fs.FileRegistryFactory.CreateUserRegistry(dst.ctx)).ListByLinkedEntity(dst.ctx, models.FileLinkDraft, restored.ID))
	c.Assert(linked, qt.HasLen, 2)
	for _, file := range linked {
		c.Check(file.GroupID, qt.Equals, groupB.ID)
		c.Check(file.ID, qt.Not(qt.Equals), sourcePhoto.ID)
	}

	// The source group's draft and files are as they were.
	c.Check(sourcePhoto.Links, qt.HasLen, 1)
	c.Check(draftLink(sourcePhoto, src.draft.ID), qt.IsNotNil)
	sourceDraft := must.Must(must.Must(src.fs.CommodityDraftRegistryFactory.CreateUserRegistry(src.ctx)).Get(src.ctx, src.draft.ID))
	c.Check(sourceDraft.Values["area_id"], qt.Equals, src.areaID)
}

// TestINBRestore_ArchiveWithoutDrafts: a backup made before drafts were in
// backups restores as before, and a merge leaves the group's drafts alone.
func TestINBRestore_ArchiveWithoutDrafts(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)
	drafts := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	draft, _ := must.Must2(drafts.CreateOrGet(f.ctx, models.CommodityDraft{ClientKey: "local-draft", Values: models.CommodityDraftValues{"name": "Bike"}}))
	photo := f.attachEntityFile(c, models.FileLinkDraft, draft.ID, models.FileLinkMetaImages, "bike", ".jpg", "image/jpeg", 64)

	key := versionGateArchive(c, f, signer, `{"version":"3.3"}`)
	final, err := restoreInbWithOptions(c, f, signer, key, models.RestoreOptions{Strategy: string(types.RestoreStrategyMergeAdd)})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	kept := must.Must(drafts.Get(f.ctx, draft.ID))
	c.Check(kept.Values, qt.DeepEquals, models.CommodityDraftValues{"name": "Bike"})
	c.Check(draftLink(f.fileByUUID(c, photo), draft.ID), qt.IsNotNil)
}

// TestINBRestore_DryRunWritesNoDrafts: a dry run reports the drafts it would
// restore and changes none. A dry run of a full replace restores no item, so
// the receipt's bytes, stored under the TV, match no reference; the draft's
// later reference to them must not fail the run as a missing member.
func TestINBRestore_DryRunWritesNoDrafts(t *testing.T) {
	for _, strategy := range []types.RestoreStrategy{types.RestoreStrategyFullReplace, types.RestoreStrategyMergeAdd} {
		t.Run(string(strategy), func(t *testing.T) {
			c := qt.New(t)
			signer := testSigner(c)
			f := newDraftFixture(c)
			blobKey, _ := f.runExport(c, signer)
			before := f.draftsByKey(c)

			final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{Strategy: string(strategy), DryRun: true})
			c.Assert(err, qt.IsNil)
			c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

			after := f.draftsByKey(c)
			c.Assert(after, qt.HasLen, len(before))
			for key, d := range before {
				c.Check(after[key].ID, qt.Equals, d.ID)
				c.Check(after[key].Revision, qt.Equals, d.Revision)
			}
		})
	}
}
