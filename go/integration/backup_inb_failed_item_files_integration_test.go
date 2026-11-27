//go:build !legacy_xml_backup

package integration_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	exportpkg "go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore"
	restoretypes "go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/internal/backupsign"
	_ "go.5x5.cz/inventario/internal/fileblob" // register the file:// blob driver
	"go.5x5.cz/inventario/internal/inb"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
	"go.5x5.cz/inventario/services"
)

// TestINBRestoreFailedItemSharedFilePostgres is the postgres half of the
// failed-item shared-file restore: a receipt shared by an item in each of two
// locations has its bytes stored after the first location's document. When
// that item fails to restore, the other item still gets the receipt, and a
// dry run ends the way the real restore does.
func TestINBRestoreFailedItemSharedFilePostgres(t *testing.T) {
	c := qt.New(t)
	dsn := testDSN(t)
	c.Assert(setupFreshDatabase(dsn), qt.IsNil)

	registrySetFunc, cleanup := postgres.NewPostgresRegistrySet()
	defer cleanup()
	fs, err := registrySetFunc(registry.Config(dsn))
	c.Assert(err, qt.IsNil)

	ctx := context.Background()
	uniq := fmt.Sprintf("%d", time.Now().UnixNano())
	uploadLocation := "file://" + c.TempDir() + "?create_dir=1"
	tenant, user, group := seedFidelityTenant(c, fs, ctx, uniq)
	uctx := appctx.WithGroup(appctx.WithUser(ctx, user), group)

	ids := models.TenantGroupAwareEntityID{TenantID: tenant.ID, GroupID: group.ID, CreatedByUserID: user.ID}
	comReg := must.Must(fs.CommodityRegistryFactory.CreateUserRegistry(uctx))
	var links []models.FileLink
	itemNames := map[string]string{}
	for _, name := range []string{"Home", "Office"} {
		loc := must.Must(must.Must(fs.LocationRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Location{TenantGroupAwareEntityID: ids, Name: name}))
		area := must.Must(must.Must(fs.AreaRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Area{TenantGroupAwareEntityID: ids, Name: name + " shelf", LocationID: loc.ID}))
		item := must.Must(comReg.Create(uctx, models.Commodity{
			TenantGroupAwareEntityID: ids,
			Name:                     name + " lamp",
			ShortName:                "lamp",
			Type:                     models.CommodityTypeFurniture,
			AreaID:                   new(area.ID),
			Count:                    1,
			Status:                   models.CommodityStatusInUse,
			OriginalPriceCurrency:    "USD",
			Draft:                    true,
		}))
		links = append(links, models.FileLink{EntityType: models.FileLinkCommodity, EntityID: item.ID, Meta: models.FileLinkMetaDocuments})
		itemNames[item.UUID] = item.Name
	}
	fileReg := must.Must(fs.FileRegistryFactory.CreateUserRegistry(uctx))
	receipt := must.Must(fileReg.Create(uctx, models.FileEntity{
		TenantGroupAwareEntityID: ids,
		Title:                    "receipt",
		Type:                     models.FileTypeDocument,
		Category:                 models.FileCategoryDocuments,
		Tags:                     models.StringSlice{},
		Links:                    links,
		File:                     &models.File{Path: "receipt", Ext: ".pdf", MIMEType: "application/pdf"},
	}))
	receipt.OriginalPath = "t/" + tenant.ID + "/files/" + receipt.UUID + ".pdf"
	receipt.File.SizeBytes = 700
	must.Must(fileReg.Update(uctx, *receipt))
	bucket := must.Must(blob.OpenBucket(uctx, uploadLocation))
	defer bucket.Close()
	c.Assert(bucket.WriteAll(uctx, receipt.OriginalPath, bytes.Repeat([]byte("r"), 700), nil), qt.IsNil)

	signer := must.Must(backupsign.NewSigner(make([]byte, backupsign.SeedSize)))
	srv := fs.CreateServiceRegistrySet()
	exportRec := must.Must(srv.ExportRegistry.Create(ctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", tenant.ID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusPending,
		Description:              "shared receipt export",
	}))
	c.Assert(exportpkg.NewExportService(fs, uploadLocation, signer).ProcessExport(ctx, exportRec.ID), qt.IsNil)
	exportRec = must.Must(srv.ExportRegistry.Get(ctx, exportRec.ID))
	c.Assert(exportRec.Status, qt.Equals, models.ExportStatusCompleted)

	archive := must.Must(bucket.ReadAll(uctx, exportRec.FilePath))
	broken, intact := breakFirstSharer(c, signer, archive, receipt.UUID, itemNames)
	key := "t/" + tenant.ID + "/restores/failed-sharer.inb"
	c.Assert(bucket.WriteAll(uctx, key, broken, nil), qt.IsNil)

	dry := runINBRestore(c, fs, signer, uploadLocation, uctx, key, models.RestoreOptions{Strategy: string(restoretypes.RestoreStrategyFullReplace), DryRun: true})
	c.Check(dry.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("dry run error: %s", dry.ErrorMessage))

	final := runINBRestore(c, fs, signer, uploadLocation, uctx, key, models.RestoreOptions{Strategy: string(restoretypes.RestoreStrategyFullReplace)})
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("restore error: %s", final.ErrorMessage))
	c.Check(final.ErrorCount, qt.Equals, 1, qt.Commentf("only the broken item is an error"))

	// A postgres restore gives the rows it creates new UUIDs, so the
	// receipt is found by its title.
	var restored *models.FileEntity
	for _, f := range must.Must(fileReg.List(uctx)) {
		if f.Title == receipt.Title {
			c.Assert(restored, qt.IsNil, qt.Commentf("the receipt is restored once"))
			restored = f
		}
	}
	c.Assert(restored, qt.IsNotNil)
	c.Assert(restored.Links, qt.HasLen, 1)
	c.Check(must.Must(comReg.Get(uctx, restored.Links[0].EntityID)).Name, qt.Equals, intact)
	c.Check(must.Must(bucket.ReadAll(uctx, restored.OriginalPath)), qt.DeepEquals, bytes.Repeat([]byte("r"), 700))
}

// breakFirstSharer blanks the name of the item that references fileUUID in
// the location document right before the file's bytes, signs the archive
// again, and returns it with the name of the other item of itemNames, which
// maps the UUIDs of the two items that share the file to their names.
func breakFirstSharer(c *qt.C, signer *backupsign.Signer, archive []byte, fileUUID string, itemNames map[string]string) ([]byte, string) {
	c.Helper()
	tr := tar.NewReader(bytes.NewReader(archive))
	_ = must.Must(tr.Next()) // signature member
	_ = must.Must(io.ReadAll(tr))
	_ = must.Must(tr.Next()) // payload member
	itr := tar.NewReader(must.Must(gzip.NewReader(bytes.NewReader(must.Must(io.ReadAll(tr))))))
	type member struct {
		hdr  *tar.Header
		body []byte
	}
	var members []member
	carrier := -1
	for {
		hdr, err := itr.Next()
		if err == io.EOF {
			break
		}
		c.Assert(err, qt.IsNil)
		body := must.Must(io.ReadAll(itr))
		if carrier < 0 && strings.HasPrefix(hdr.Name, "files/") && strings.Contains(hdr.Name, fileUUID) {
			for i := len(members) - 1; i >= 0 && carrier < 0; i-- {
				if strings.HasPrefix(members[i].hdr.Name, "location-") {
					carrier = i
				}
			}
		}
		members = append(members, member{hdr: hdr, body: body})
	}
	c.Assert(carrier >= 0, qt.IsTrue)

	var doc restoretypes.INBLocationDoc
	c.Assert(json.Unmarshal(members[carrier].body, &doc), qt.IsNil)
	broken := ""
	for i := range doc.Commodities {
		for _, ref := range doc.Commodities[i].Documents {
			if ref.ID == fileUUID {
				doc.Commodities[i].Name = ""
				broken = doc.Commodities[i].ID
			}
		}
	}
	c.Assert(broken, qt.Not(qt.Equals), "")
	members[carrier].body = must.Must(json.Marshal(doc))

	var payload bytes.Buffer
	gzw := gzip.NewWriter(&payload)
	tw := tar.NewWriter(gzw)
	for _, m := range members {
		m.hdr.Size = int64(len(m.body))
		c.Assert(tw.WriteHeader(m.hdr), qt.IsNil)
		must.Must(tw.Write(m.body))
	}
	c.Assert(tw.Close(), qt.IsNil)
	c.Assert(gzw.Close(), qt.IsNil)
	digest := backupsign.NewDigest()
	_, _ = digest.Write(payload.Bytes())
	var out bytes.Buffer
	c.Assert(inb.WriteContainer(&out, signer.SignDigest(digest.Sum(nil)), bytes.NewReader(payload.Bytes()), int64(payload.Len())), qt.IsNil)

	c.Assert(itemNames, qt.HasLen, 2)
	var intact string
	for uuid, name := range itemNames {
		if uuid != broken {
			intact = name
		}
	}
	return out.Bytes(), intact
}

// runINBRestore restores filePath into the group of uctx with opts and
// returns the restore operation's final state.
func runINBRestore(c *qt.C, fs *registry.FactorySet, signer *backupsign.Signer, uploadLocation string, uctx context.Context, filePath string, opts models.RestoreOptions) *models.RestoreOperation {
	c.Helper()
	user := appctx.UserFromContext(uctx)
	group := appctx.GroupFromContext(uctx)
	svc := restore.NewRestoreService(fs, services.NewEntityService(fs, uploadLocation), uploadLocation, signer)

	exportRow := must.Must(must.Must(fs.ExportRegistryFactory.CreateUserRegistry(uctx)).Create(uctx, models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", user.TenantID, group.ID, user.ID),
		Type:                     models.ExportTypeFullDatabase,
		Status:                   models.ExportStatusCompleted,
		Description:              "restore source",
		FilePath:                 filePath,
	}))
	roReg := must.Must(fs.RestoreOperationRegistryFactory.CreateUserRegistry(uctx))
	op := must.Must(roReg.Create(uctx, models.RestoreOperation{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID("", user.TenantID, group.ID, user.ID),
		ExportID:                 exportRow.ID,
		Status:                   models.RestoreStatusPending,
		Description:              "restore",
		Options:                  opts,
	}))
	// A failed restore returns its error here and records it on the
	// operation, which the caller asserts on.
	_ = svc.ProcessRestoreOperation(uctx, op.ID, uploadLocation)
	return must.Must(roReg.Get(uctx, op.ID))
}
