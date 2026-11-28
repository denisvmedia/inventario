//go:build !legacy_xml_backup

package backup_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// TestINBRoundTrip_ItemDates checks that a restored item keeps the day it
// was added and the day it was last changed, through the cover photo the
// restore sets on it after creating it.
func TestINBRoundTrip_ItemDates(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	photoUUID := f.attachCommodityFile(c, "images", 64)
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	tv := must.Must(comReg.List(f.ctx))[0]
	tv.CoverFileID = new(f.fileByUUID(c, photoUUID).ID)
	tv.RegisteredDate = models.ToPDate("2021-03-04")
	tv.LastModifiedDate = models.ToPDate("2022-05-06")
	must.Must(comReg.Update(registry.WithKeptModifiedDate(f.ctx), *tv))

	blobKey, _ := f.runExport(c, signer)
	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	restored := f.commodityByUUID(c, tv.UUID)
	c.Assert(restored.CoverFileID, qt.IsNotNil)
	c.Assert(restored.RegisteredDate, qt.DeepEquals, models.ToPDate("2021-03-04"))
	c.Assert(restored.LastModifiedDate, qt.DeepEquals, models.ToPDate("2022-05-06"))
}

// TestINBRestore_ItemWithoutDates checks that an archived item that carries
// no dates is restored as added and changed today, in UTC.
func TestINBRestore_ItemWithoutDates(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	doc := inbDocForFixture(c, f, func(com *types.INBCommodity) {
		com.RegisteredDate = ""
		com.LastModifiedDate = ""
	})
	archive := signedArchiveFromDoc(c, signer, doc)
	bucket := must.Must(blob.OpenBucket(f.ctx, inbUploadLocation))
	key := "t/tenant-a/restores/no-dates.inb"
	c.Assert(bucket.WriteAll(f.ctx, key, archive, nil), qt.IsNil)
	bucket.Close()

	first := models.NewDate(time.Now())
	final, err := restoreInb(c, f, signer, key)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	days := []models.Date{first, models.NewDate(time.Now())}

	restored := f.commodityByUUID(c, doc.Commodities[0].ID)
	c.Assert(restored.RegisteredDate, qt.IsNotNil)
	c.Assert(days, qt.Contains, *restored.RegisteredDate)
	c.Assert(restored.LastModifiedDate, qt.IsNotNil)
	c.Assert(days, qt.Contains, *restored.LastModifiedDate)
}
