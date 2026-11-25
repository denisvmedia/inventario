//go:build !legacy_xml_backup

package backup_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Intake sessions (#2810) are a work queue and stay out of backups. A file
// only an intake source holds is exported as a standalone file, so its bytes
// survive; a full replace leaves the group with no sessions.

func TestINBRoundTrip_IntakeSessionsStayOut(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	receipt := f.attachStandaloneFile(c, "receipt", ".jpg", "image/jpeg", 128)
	intake := must.Must(f.fs.IntakeRegistryFactory.CreateUserRegistry(f.ctx))
	session, created, err := intake.CreateSession(f.ctx, models.IntakeSession{ClientKey: "batch-in-backup"},
		[]registry.IntakeSourceInput{{FileIDs: []string{f.fileByUUID(c, receipt).ID}}})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Assert(f.fileByUUID(c, receipt).Links, qt.HasLen, 1)

	blobKey, archive := f.runExport(c, signer)
	_, jsons := innerMembers(c, archive)
	var files types.INBFilesDoc
	c.Assert(json.Unmarshal(jsons[types.INBFilesMember], &files), qt.IsNil)
	var found bool
	for _, ref := range files.Files {
		if ref.ID == receipt {
			found = true
			c.Check(ref.LinkedEntityType, qt.Equals, "", qt.Commentf("exported as a standalone file"))
		}
	}
	c.Check(found, qt.IsTrue, qt.Commentf("the file only the session holds is exported"))
	for name, data := range jsons {
		c.Check(string(data), qt.Not(qt.Contains), session.ClientKey, qt.Commentf("%s names the session", name))
		c.Check(string(data), qt.Not(qt.Contains), session.UUID, qt.Commentf("%s names the session", name))
	}

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Assert(final.ErrorCount, qt.Equals, 0)

	sessions := must.Must(intake.List(f.ctx))
	c.Check(sessions, qt.HasLen, 0, qt.Commentf("a full replace leaves no intake session"))
	restored := f.fileByUUID(c, receipt)
	c.Check(restored.Links, qt.HasLen, 0)
}
