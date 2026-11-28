//go:build !legacy_xml_backup

package backup_test

import (
	"path"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	importpkg "go.5x5.cz/inventario/backup/import"
	"go.5x5.cz/inventario/models"
)

// importBackup imports the backup at blobKey as the user picked it under
// sourceFileName ("" for an import that did not send the name) and returns
// the file row the import created for it.
func importBackup(c *qt.C, f *inbFixture, blobKey, sourceFileName string) *models.FileEntity {
	c.Helper()
	signer := testSigner(c)
	svc := importpkg.NewImportService(f.fs, inbUploadLocation, signer)

	pending := models.NewImportedExport("imported backup", blobKey, sourceFileName)
	pending.TenantGroupAwareEntityID = models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID}
	expReg := must.Must(f.fs.ExportRegistryFactory.CreateUserRegistry(f.ctx))
	created := must.Must(expReg.Create(f.ctx, pending))
	c.Assert(created.SourceFileName, qt.Equals, sourceFileName)

	c.Assert(svc.ProcessImport(f.ctx, created.ID, blobKey), qt.IsNil)

	imported := must.Must(expReg.Get(f.ctx, created.ID))
	c.Assert(imported.Status, qt.Equals, models.ExportStatusCompleted)
	c.Assert(imported.FileID, qt.IsNotNil)
	fileReg := must.Must(f.fs.FileRegistryFactory.CreateUserRegistry(f.ctx))
	return must.Must(fileReg.Get(f.ctx, *imported.FileID))
}

// An imported backup's file is named after the file the user picked, without
// the extension, and downloads under that name.
func TestINBImport_FileNamedAfterPickedFile(t *testing.T) {
	c := qt.New(t)
	f := newInbFixture(c)
	blobKey, _ := f.runExport(c, testSigner(c))

	file := importBackup(c, f, blobKey, "Home backup 2026-11-28.inb")
	c.Assert(file.File.Path, qt.Equals, "Home backup 2026-11-28")
	c.Assert(file.File.OriginalPath, qt.Equals, blobKey)
}

// An import that did not send the name of the picked file keeps naming the
// file after the uploaded blob.
func TestINBImport_FileNamedAfterBlobWithoutPickedName(t *testing.T) {
	c := qt.New(t)
	f := newInbFixture(c)
	blobKey, _ := f.runExport(c, testSigner(c))

	file := importBackup(c, f, blobKey, "")
	c.Assert(file.File.Path, qt.Equals, strings.TrimSuffix(path.Base(blobKey), path.Ext(blobKey)))
}
