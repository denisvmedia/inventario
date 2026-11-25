//go:build !legacy_xml_backup

package backup_test

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// itemTypeFixture is an inbFixture whose TV has the custom type "Vehicle",
// plus a custom type "Boat" no item has.
type itemTypeFixture struct {
	*inbFixture
	tvUUID  string
	types   registry.ItemTypeRegistry
	vehicle *models.ItemType
	boat    *models.ItemType
}

func newItemTypeFixture(c *qt.C) *itemTypeFixture {
	f := newInbFixture(c)
	reg := must.Must(f.fs.ItemTypeRegistryFactory.CreateUserRegistry(f.ctx))
	vehicle := must.Must(reg.Create(f.ctx, models.ItemType{Name: "Vehicle", Icon: "car"}))
	boat := must.Must(reg.Create(f.ctx, models.ItemType{Name: "Boat", Icon: "sailboat"}))

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	tv := must.Must(comReg.List(f.ctx))[0]
	tv.Type = vehicle.Slug
	tv = must.Must(comReg.Update(f.ctx, *tv))
	c.Assert(tv.Type, qt.Equals, models.CommodityType("vehicle"))

	return &itemTypeFixture{inbFixture: f, tvUUID: tv.UUID, types: reg, vehicle: vehicle, boat: boat}
}

func TestINBExport_ItemTypes(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newItemTypeFixture(c)

	_, archive := f.runExport(c, signer)
	manifest, items := manifestAndItems(c, archive)

	c.Check(manifest.Version, qt.Equals, export.INBFormatVersion)
	// A whole-group export carries every type, the unused ones too; a
	// built-in type keeps an empty name.
	want := make([]types.INBItemType, 0, len(models.BuiltinItemTypes)+2)
	for _, b := range models.BuiltinItemTypes {
		want = append(want, types.INBItemType{Slug: string(b.Slug), Icon: b.Icon})
	}
	want = append(want,
		types.INBItemType{Slug: "boat", Name: "Boat", Icon: "sailboat"},
		types.INBItemType{Slug: "vehicle", Name: "Vehicle", Icon: "car"},
	)
	c.Check(manifest.ItemTypes, qt.DeepEquals, want)
	c.Check(items[f.tvUUID].Type, qt.Equals, "vehicle")
}

func TestINBExport_SelectedItemsCarryOnlyTheirItemTypes(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newItemTypeFixture(c)

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
	c.Check(manifest.ItemTypes, qt.DeepEquals, []types.INBItemType{{Slug: "vehicle", Name: "Vehicle", Icon: "car"}})
}

func TestINBRoundTrip_ItemTypesAreRecreated(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newItemTypeFixture(c)
	blobKey, _ := f.runExport(c, signer)

	// The group drops the custom types and a built-in one before the
	// restore, so the restore has to create them again.
	other := must.Must(f.types.GetBySlug(f.ctx, models.CommodityTypeOther))
	must.Must(f.types.DeleteReassigning(f.ctx, f.vehicle.ID, other.ID))
	c.Assert(f.types.Delete(f.ctx, f.boat.ID), qt.IsNil)
	electronics := must.Must(f.types.GetBySlug(f.ctx, models.CommodityTypeElectronics))
	c.Assert(f.types.Delete(f.ctx, electronics.ID), qt.IsNil)

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	vehicle := must.Must(f.types.GetBySlug(f.ctx, "vehicle"))
	c.Check(vehicle.Name, qt.Equals, "Vehicle")
	c.Check(vehicle.Icon, qt.Equals, "car")
	c.Check(vehicle.ID, qt.Not(qt.Equals), f.vehicle.ID)
	boat := must.Must(f.types.GetBySlug(f.ctx, "boat"))
	c.Check(boat.Name, qt.Equals, "Boat")
	restoredElectronics := must.Must(f.types.GetBySlug(f.ctx, models.CommodityTypeElectronics))
	c.Check(restoredElectronics.Builtin(), qt.IsTrue)
	c.Check(restoredElectronics.Name, qt.Equals, "")

	tv := f.commodityByUUID(c, f.tvUUID)
	c.Check(tv.Type, qt.Equals, models.CommodityType("vehicle"))
}

func TestINBRoundTrip_ItemTypesAreMatchedBySlugThenName(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newItemTypeFixture(c)
	blobKey, _ := f.runExport(c, signer)

	// The group renamed "Vehicle" meanwhile: the slug still matches and
	// the group's name stays. "Boat" was replaced by a type with the same
	// name under another slug: the name matches.
	vehicle := *f.vehicle
	vehicle.Name = "Car"
	must.Must(f.types.Update(f.ctx, vehicle))
	c.Assert(f.types.Delete(f.ctx, f.boat.ID), qt.IsNil)
	craft := must.Must(f.types.Create(f.ctx, models.ItemType{Slug: "watercraft", Name: "boat"}))

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	list := must.Must(f.types.List(f.ctx))
	c.Check(list, qt.HasLen, len(models.BuiltinItemTypes)+2, qt.Commentf("nothing was created"))
	got := must.Must(f.types.Get(f.ctx, f.vehicle.ID))
	c.Check(got.Name, qt.Equals, "Car")
	_, err = f.types.GetBySlug(f.ctx, "boat")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
	c.Check(must.Must(f.types.Get(f.ctx, craft.ID)).Name, qt.Equals, "boat")

	tv := f.commodityByUUID(c, f.tvUUID)
	c.Check(tv.Type, qt.Equals, models.CommodityType("vehicle"))
}

func TestINBRestore_DryRunCreatesNoItemTypes(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newItemTypeFixture(c)
	blobKey, _ := f.runExport(c, signer)

	c.Assert(f.types.Delete(f.ctx, f.boat.ID), qt.IsNil)
	before := must.Must(f.types.Count(f.ctx))

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeAdd),
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Check(must.Must(f.types.Count(f.ctx)), qt.Equals, before)
}
