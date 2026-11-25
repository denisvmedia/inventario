package backup_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/models"
)

// TestINBRoundTrip_DraftFileIsStandalone: drafts are not in a backup (#2812).
// A file attached only to a draft is still a file of the group, so the backup
// carries it as a standalone file, and the restore brings it back without a
// link to a draft the archive does not hold.
func TestINBRoundTrip_DraftFileIsStandalone(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	drafts := must.Must(f.fs.CommodityDraftRegistryFactory.CreateUserRegistry(f.ctx))
	draft, _ := must.Must2(drafts.CreateOrGet(f.ctx, models.CommodityDraft{
		ClientKey: "draft-key-in-backup",
		Values:    models.CommodityDraftValues{"name": "Half-done drill"},
	}))
	photo := f.attachEntityFile(c, models.FileLinkDraft, draft.ID, models.FileLinkMetaImages, "drill-photo", ".jpg", "image/jpeg", 128)

	blobKey, archive := f.runExport(c, signer)

	_, jsons := innerMembers(c, archive)
	c.Assert(string(jsons["files/_index.json"]), qt.Contains, photo)
	for name, data := range jsons {
		c.Assert(string(data), qt.Not(qt.Contains), draft.ID, qt.Commentf("document %s mentions the draft", name))
		c.Assert(string(data), qt.Not(qt.Contains), "Half-done drill", qt.Commentf("document %s carries draft values", name))
	}

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	restored := f.fileByUUID(c, photo)
	c.Assert(restored.Links, qt.HasLen, 0)
}
