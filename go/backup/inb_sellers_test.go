//go:build !legacy_xml_backup

package backup_test

import (
	"encoding/json"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
)

// sellerFixture is an inbFixture whose TV was bought from "Hardware Store",
// plus a second seller no item names.
type sellerFixture struct {
	*inbFixture
	tvUUID   string
	shop     *models.Seller
	idleShop *models.Seller
}

func newSellerFixture(c *qt.C) *sellerFixture {
	f := newInbFixture(c)
	sellers := must.Must(f.fs.SellerRegistryFactory.CreateUserRegistry(f.ctx))
	shop := must.Must(sellers.Create(f.ctx, models.Seller{
		Name: "Hardware Store", Website: "https://hw.example", Contact: "Ann", Note: "cash only",
	}))
	idle := must.Must(sellers.Create(f.ctx, models.Seller{Name: "Idle Shop"}))

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	tv := must.Must(comReg.List(f.ctx))[0]
	tv.SellerID = &shop.ID
	tv = must.Must(comReg.Update(f.ctx, *tv))
	c.Assert(tv.SellerID, qt.DeepEquals, &shop.ID)

	return &sellerFixture{inbFixture: f, tvUUID: tv.UUID, shop: shop, idleShop: idle}
}

func manifestAndItems(c *qt.C, archive []byte) (types.INBManifest, map[string]types.INBCommodity) {
	_, jsons := innerMembers(c, archive)
	var manifest types.INBManifest
	c.Assert(json.Unmarshal(jsons["manifest.json"], &manifest), qt.IsNil)
	items := map[string]types.INBCommodity{}
	for name, body := range jsons {
		if name == "manifest.json" {
			continue
		}
		var doc types.INBLocationDoc
		if json.Unmarshal(body, &doc) != nil {
			continue
		}
		for _, com := range doc.Commodities {
			items[com.ID] = com
		}
	}
	return manifest, items
}

func TestINBExport_Sellers(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSellerFixture(c)

	_, archive := f.runExport(c, signer)
	manifest, items := manifestAndItems(c, archive)

	c.Check(manifest.Version, qt.Equals, "3.3")
	// A whole-group export carries every seller, the one without items too.
	c.Check(manifest.Sellers, qt.DeepEquals, []types.INBSeller{
		{ID: f.shop.UUID, Name: "Hardware Store", Website: "https://hw.example", Contact: "Ann", Note: "cash only"},
		{ID: f.idleShop.UUID, Name: "Idle Shop"},
	})
	c.Check(items[f.tvUUID].SellerID, qt.Equals, f.shop.UUID)
}

func TestINBExport_SelectedItemsCarryOnlyTheirSellers(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSellerFixture(c)

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
	manifest, _ := manifestAndItems(c, archive)
	c.Assert(manifest.Sellers, qt.HasLen, 1)
	c.Check(manifest.Sellers[0].ID, qt.Equals, f.shop.UUID)
}

func TestINBRoundTrip_SellersAreRecreated(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSellerFixture(c)
	blobKey, _ := f.runExport(c, signer)

	// The group loses its sellers before the restore, so the restore has to
	// create them again.
	sellers := must.Must(f.fs.SellerRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(sellers.Delete(f.ctx, f.shop.ID), qt.IsNil)
	c.Assert(sellers.Delete(f.ctx, f.idleShop.ID), qt.IsNil)

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	restored := must.Must(sellers.List(f.ctx))
	c.Assert(restored, qt.HasLen, 2)
	shop := restored[0]
	c.Check(shop.Name, qt.Equals, "Hardware Store")
	c.Check(shop.Website, qt.Equals, "https://hw.example")
	c.Check(shop.Contact, qt.Equals, "Ann")
	c.Check(shop.Note, qt.Equals, "cash only")
	c.Check(shop.ID, qt.Not(qt.Equals), f.shop.ID)
	c.Check(restored[1].Name, qt.Equals, "Idle Shop")

	tv := f.commodityByUUID(c, f.tvUUID)
	c.Check(tv.SellerID, qt.DeepEquals, &shop.ID)
}

func TestINBRoundTrip_SellersAreMatchedByName(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSellerFixture(c)
	blobKey, _ := f.runExport(c, signer)

	// The group renamed the seller's website meanwhile; a restore reuses
	// the seller with the same name and keeps the group's version of it.
	sellers := must.Must(f.fs.SellerRegistryFactory.CreateUserRegistry(f.ctx))
	shop := *f.shop
	shop.Website = "https://new.example"
	must.Must(sellers.Update(f.ctx, shop))

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)

	restored := must.Must(sellers.List(f.ctx))
	c.Assert(restored, qt.HasLen, 2)
	c.Check(restored[0].ID, qt.Equals, f.shop.ID)
	c.Check(restored[0].Website, qt.Equals, "https://new.example")

	tv := f.commodityByUUID(c, f.tvUUID)
	c.Check(tv.SellerID, qt.DeepEquals, &f.shop.ID)
}

func TestINBRestore_DryRunCreatesNoSellers(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newSellerFixture(c)
	blobKey, _ := f.runExport(c, signer)

	sellers := must.Must(f.fs.SellerRegistryFactory.CreateUserRegistry(f.ctx))
	c.Assert(sellers.Delete(f.ctx, f.idleShop.ID), qt.IsNil)

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeAdd),
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Check(must.Must(sellers.Count(f.ctx)), qt.Equals, 1)
}
