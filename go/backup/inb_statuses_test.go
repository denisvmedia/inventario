//go:build !legacy_xml_backup

package backup_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
)

// TestINBRoundTrip_Statuses exports an item in each status that leaves in
// use, donated and stolen included, and checks that a restore brings every
// status back with its date and note.
func TestINBRoundTrip_Statuses(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newInbFixture(c)

	statuses := []models.CommodityStatus{
		models.CommodityStatusSold,
		models.CommodityStatusLost,
		models.CommodityStatusDisposed,
		models.CommodityStatusWrittenOff,
		models.CommodityStatusDonated,
		models.CommodityStatusStolen,
	}
	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	uuids := map[models.CommodityStatus]string{}
	for _, status := range statuses {
		com := models.Commodity{
			TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: "tenant-a", GroupID: f.group.ID, CreatedByUserID: f.user.ID},
			Name:                     "Item " + string(status),
			ShortName:                string(status),
			Type:                     models.CommodityTypeOther,
			AreaID:                   new(f.areaID),
			Count:                    1,
			Status:                   status,
			StatusDate:               models.ToPDate("2026-11-26"),
			StatusNote:               "note " + string(status),
			OriginalPriceCurrency:    "USD",
		}
		if status == models.CommodityStatusSold {
			com.SalePrice = new(decimal.NewFromInt(40))
		}
		uuids[status] = must.Must(comReg.Create(f.ctx, com)).UUID
	}

	blobKey, archive := f.runExport(c, signer)
	_, items := manifestAndItems(c, archive)
	for status, uuid := range uuids {
		c.Check(items[uuid].Status, qt.Equals, string(status))
	}

	final, err := restoreInb(c, f, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	for status, uuid := range uuids {
		got := f.commodityByUUID(c, uuid)
		c.Check(got.Status, qt.Equals, status)
		c.Assert(got.StatusDate, qt.IsNotNil, qt.Commentf("status %s", status))
		c.Check(string(*got.StatusDate), qt.Equals, "2026-11-26")
		c.Check(got.StatusNote, qt.Equals, "note "+string(status))
	}
	c.Check(f.commodityByUUID(c, uuids[models.CommodityStatusSold]).SalePrice.String(), qt.Equals, "40")
	c.Check(f.commodityByUUID(c, uuids[models.CommodityStatusDonated]).SalePrice, qt.IsNil)
}
