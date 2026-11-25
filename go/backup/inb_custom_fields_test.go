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

// customFieldFixture is an inbFixture with a second item, a remote, and
// custom fields: the TV has a serial, a fuel option (an odd one for a TV,
// but a select), a size and a link to the remote. "Idle" is a field no item
// has a value of.
type customFieldFixture struct {
	*inbFixture
	tvUUID, remoteUUID       string
	serial, fuel, size, link *models.CustomFieldDefinition
	idle                     *models.CustomFieldDefinition
}

func newCustomFieldFixture(c *qt.C) *customFieldFixture {
	f := newInbFixture(c)
	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	itemTypes := must.Must(f.fs.ItemTypeRegistryFactory.CreateUserRegistry(f.ctx))
	must.Must(itemTypes.EnsureBuiltins(f.ctx))
	electronics := must.Must(itemTypes.GetBySlug(f.ctx, models.CommodityTypeElectronics))

	serial := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{
		Name: "Serial", Type: models.CustomFieldTypeText,
		Templates: []models.CustomFieldTemplate{{ItemTypeID: electronics.ID, DefaultValue: models.JSONValue(`"SN-"`)}},
	}))
	fuel := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{
		Name: "Fuel", Type: models.CustomFieldTypeSelect,
		Options: models.ValuerSlice[models.CustomFieldOption]{{Label: "Petrol"}, {Label: "Diesel"}},
	}))
	size := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{Name: "Size", Type: models.CustomFieldTypeNumber}))
	link := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{Name: "Remote", Type: models.CustomFieldTypeItemLink}))
	idle := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{Name: "Idle", Type: models.CustomFieldTypeDate}))

	comReg := must.Must(f.fs.CommodityRegistryFactory.CreateUserRegistry(f.ctx))
	tv := must.Must(comReg.List(f.ctx))[0]
	remote := *tv
	remote.ID, remote.UUID, remote.Name, remote.ShortName = "", "", "Remote", "remote"
	remote.AssetNumber = nil
	created := must.Must(comReg.Create(f.ctx, remote))

	tv.CustomFields = models.CustomFieldValues{
		serial.ID: json.RawMessage(`"SN-42"`),
		fuel.ID:   json.RawMessage(`"` + fuel.Options[1].ID + `"`),
		size.ID:   json.RawMessage(`0`),
		link.ID:   json.RawMessage(`"` + created.ID + `"`),
	}
	tv = must.Must(comReg.Update(f.ctx, *tv))
	c.Assert(tv.CustomFields, qt.HasLen, 4)

	return &customFieldFixture{
		inbFixture: f, tvUUID: tv.UUID, remoteUUID: created.UUID,
		serial: serial, fuel: fuel, size: size, link: link, idle: idle,
	}
}

func TestINBExport_CustomFields(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)

	_, archive := f.runExport(c, signer)
	manifest, items := manifestAndItems(c, archive)

	byName := map[string]types.INBCustomField{}
	for _, cf := range manifest.CustomFields {
		byName[cf.Name] = cf
	}
	c.Assert(byName, qt.HasLen, 5, qt.Commentf("a whole-group export carries every field"))
	c.Check(byName["Serial"].ID, qt.Equals, f.serial.UUID)
	c.Check(byName["Serial"].Templates, qt.HasLen, 1)
	c.Check(byName["Serial"].Templates[0].ItemType, qt.Equals, string(models.CommodityTypeElectronics))
	c.Check(string(byName["Serial"].Templates[0].DefaultValue), qt.Equals, `"SN-"`)
	c.Check(byName["Fuel"].Options, qt.DeepEquals, []types.INBCustomFieldOption{
		{ID: f.fuel.Options[0].ID, Label: "Petrol"}, {ID: f.fuel.Options[1].ID, Label: "Diesel"},
	})

	tv := items[f.tvUUID]
	c.Check(string(tv.CustomFields[f.serial.UUID]), qt.Equals, `"SN-42"`)
	c.Check(string(tv.CustomFields[f.size.UUID]), qt.Equals, `0`)
	c.Check(string(tv.CustomFields[f.link.UUID]), qt.Equals, `"`+f.remoteUUID+`"`, qt.Commentf("a link names the item's UUID"))
}

func TestINBRoundTrip_CustomFieldsAreRecreated(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)
	blobKey, _ := f.runExport(c, signer)

	// The group loses its fields before the restore, so the restore has to
	// create them again.
	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	for _, d := range []*models.CustomFieldDefinition{f.serial, f.fuel, f.size, f.link, f.idle} {
		must.Must(fields.DeleteWithValues(f.ctx, d.ID))
	}

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	restored := map[string]*models.CustomFieldDefinition{}
	for _, d := range must.Must(fields.List(f.ctx)) {
		restored[d.Name] = d
	}
	c.Assert(restored, qt.HasLen, 5)
	c.Check(restored["Serial"].ID, qt.Not(qt.Equals), f.serial.ID)
	c.Check(restored["Serial"].Templates, qt.HasLen, 1)
	c.Check(restored["Fuel"].Options, qt.DeepEquals, f.fuel.Options, qt.Commentf("options keep their IDs"))

	tv := f.commodityByUUID(c, f.tvUUID)
	remote := f.commodityByUUID(c, f.remoteUUID)
	c.Check(string(tv.CustomFields[restored["Serial"].ID]), qt.Equals, `"SN-42"`)
	c.Check(string(tv.CustomFields[restored["Fuel"].ID]), qt.Equals, `"`+f.fuel.Options[1].ID+`"`)
	c.Check(string(tv.CustomFields[restored["Size"].ID]), qt.Equals, `0`, qt.Commentf("a zero is a value"))
	c.Check(string(tv.CustomFields[restored["Remote"].ID]), qt.Equals, `"`+remote.ID+`"`, qt.Commentf("the link names the restored item"))
}

func TestINBRoundTrip_CustomFieldsAreMatchedByName(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)
	blobKey, _ := f.runExport(c, signer)

	// Meanwhile the group relabeled one option and dropped the other, and
	// gave "Size" another type under the same name.
	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	fuel := *f.fuel
	fuel.Options = models.ValuerSlice[models.CustomFieldOption]{{ID: f.fuel.Options[0].ID, Label: "Gasoline"}}
	fuel.Templates = nil
	must.Must(fields.Update(f.ctx, fuel))
	must.Must(fields.DeleteWithValues(f.ctx, f.size.ID))
	size := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{Name: "size", Type: models.CustomFieldTypeText}))

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	gotFuel := must.Must(fields.Get(f.ctx, f.fuel.ID))
	c.Assert(gotFuel.Options, qt.HasLen, 3, qt.Commentf("the archived options the field lacks are added: %v", gotFuel.Options))
	c.Check(gotFuel.Options[0].Label, qt.Equals, "Gasoline")
	c.Check(gotFuel.Options[1].Label, qt.Equals, "Petrol")
	c.Check(gotFuel.Options[2].Label, qt.Equals, "Diesel")

	tv := f.commodityByUUID(c, f.tvUUID)
	c.Check(string(tv.CustomFields[f.fuel.ID]), qt.Equals, `"`+gotFuel.Options[2].ID+`"`)
	_, hasSize := tv.CustomFields[size.ID]
	c.Check(hasSize, qt.IsFalse, qt.Commentf("a field of another type takes no archived value"))
	c.Check(final.ErrorCount > 0, qt.IsTrue)
}

func TestINBRestore_DryRunCreatesNoCustomFields(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)
	blobKey, _ := f.runExport(c, signer)

	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	must.Must(fields.DeleteWithValues(f.ctx, f.idle.ID))

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeAdd),
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Check(must.Must(fields.Count(f.ctx)), qt.Equals, 4)
	_, err = fields.GetByName(f.ctx, "Idle")
	c.Check(err, qt.ErrorIs, registry.ErrNotFound)
}
