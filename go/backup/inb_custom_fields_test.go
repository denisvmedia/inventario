//go:build !legacy_xml_backup

package backup_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
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
	c.Check(string(tv.CustomFields[f.size.UUID]), qt.Equals, `"0"`)
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
		must.Must(fields.DeleteWithValues(f.ctx, d.ID, registry.AnyValueCount))
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
	c.Check(string(tv.CustomFields[restored["Size"].ID]), qt.Equals, `"0"`, qt.Commentf("a zero is a value"))
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
	must.Must(fields.DeleteWithValues(f.ctx, f.size.ID, registry.AnyValueCount))
	size := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{Name: "size", Type: models.CustomFieldTypeText}))

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	gotFuel := must.Must(fields.Get(f.ctx, f.fuel.ID))
	c.Check(gotFuel.Options, qt.DeepEquals, models.ValuerSlice[models.CustomFieldOption]{
		{ID: f.fuel.Options[0].ID, Label: "Gasoline"},
		{ID: f.fuel.Options[1].ID, Label: "Diesel"},
	}, qt.Commentf("the relabeled option is the archived Petrol; the dropped one comes back with its ID"))

	tv := f.commodityByUUID(c, f.tvUUID)
	c.Check(string(tv.CustomFields[f.fuel.ID]), qt.Equals, `"`+f.fuel.Options[1].ID+`"`)
	_, hasSize := tv.CustomFields[size.ID]
	c.Check(hasSize, qt.IsFalse, qt.Commentf("a field of another type takes no archived value"))
	c.Check(final.ErrorCount > 0, qt.IsTrue)
}

// A field renamed since the backup is matched by its UUID: the restore
// neither creates a second field nor moves the values off the field a saved
// view or a template names.
func TestINBRoundTrip_RenamedCustomFieldIsMatchedByUUID(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)
	blobKey, _ := f.runExport(c, signer)

	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	serial := *f.serial
	serial.Name = "Serial number"
	serial.Templates = nil
	must.Must(fields.Update(f.ctx, serial))
	fuel := *f.fuel
	fuel.Name = "Fuel type"
	fuel.Options = models.ValuerSlice[models.CustomFieldOption]{{ID: f.fuel.Options[0].ID, Label: "Petrol"}, {ID: f.fuel.Options[1].ID, Label: "Gasoil"}}
	fuel.Templates = nil
	must.Must(fields.Update(f.ctx, fuel))

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))
	c.Check(final.ErrorCount, qt.Equals, 0, qt.Commentf("errors: %v", final.ErrorMessage))

	c.Check(must.Must(fields.Count(f.ctx)), qt.Equals, 5)
	c.Check(must.Must(fields.Get(f.ctx, f.serial.ID)).Name, qt.Equals, "Serial number", qt.Commentf("the group's name stays"))
	c.Check(must.Must(fields.Get(f.ctx, f.fuel.ID)).Options, qt.HasLen, 2)
	tv := f.commodityByUUID(c, f.tvUUID)
	c.Check(string(tv.CustomFields[f.serial.ID]), qt.Equals, `"SN-42"`)
	c.Check(string(tv.CustomFields[f.fuel.ID]), qt.Equals, `"`+f.fuel.Options[1].ID+`"`)
}

// A dry run writes no field, but counts each archived field the restore
// would refuse.
func TestINBRestore_DryRunCountsCustomFieldConflicts(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)
	blobKey, _ := f.runExport(c, signer)

	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	must.Must(fields.DeleteWithValues(f.ctx, f.size.ID, registry.AnyValueCount))
	must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{Name: "size", Type: models.CustomFieldTypeText}))

	final, err := restoreInbWithOptions(c, f.inbFixture, signer, blobKey, models.RestoreOptions{
		Strategy: string(types.RestoreStrategyMergeAdd),
		DryRun:   true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(final.Status, qt.Equals, models.RestoreStatusCompleted)
	c.Check(final.ErrorCount, qt.Equals, 1, qt.Commentf("the Size field of another type"))
	c.Check(must.Must(fields.Count(f.ctx)), qt.Equals, 5)
}

// A link to an item in Trash stays in the archive. A restore that cannot
// link to that item leaves the link out and counts it.
func TestINBRoundTrip_LinkToAnItemInTrash(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	src := newCustomFieldFixture(c)
	comReg := must.Must(src.fs.CommodityRegistryFactory.CreateUserRegistry(src.ctx))
	remote := src.commodityByUUID(c, src.remoteUUID)
	must.Must(comReg.Trash(src.ctx, remote.ID, src.user.ID, time.Now()))

	blobKey, archive := src.runExport(c, signer)
	_, items := manifestAndItems(c, archive)
	c.Check(string(items[src.tvUUID].CustomFields[src.link.UUID]), qt.Equals, `"`+src.remoteUUID+`"`)

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
	c.Check(final.ErrorCount, qt.Equals, 1, qt.Commentf("the link to the item in Trash"))
	tv := dst.commodityByUUID(c, src.tvUUID)
	c.Check(tv.CustomFields, qt.HasLen, 3, qt.Commentf("every value but the link restores: %v", tv.CustomFields))
}

func TestINBRestore_DryRunCreatesNoCustomFields(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)
	blobKey, _ := f.runExport(c, signer)

	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	must.Must(fields.DeleteWithValues(f.ctx, f.idle.ID, registry.AnyValueCount))

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

// A template entry for an item type the restore cannot create, because the
// group already has the most item types it keeps, is left out: it would
// otherwise land on the fallback type.
func TestINBRestore_FullGroupLeavesOutTheTemplatesOfMissingTypes(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	f := newCustomFieldFixture(c)
	itemTypes := must.Must(f.fs.ItemTypeRegistryFactory.CreateUserRegistry(f.ctx))
	fields := must.Must(f.fs.CustomFieldRegistryFactory.CreateUserRegistry(f.ctx))
	vehicle := must.Must(itemTypes.Create(f.ctx, models.ItemType{Name: "Vehicle"}))
	plate := must.Must(fields.Create(f.ctx, models.CustomFieldDefinition{
		Name: "Plate", Type: models.CustomFieldTypeText,
		Templates: []models.CustomFieldTemplate{{ItemTypeID: vehicle.ID}},
	}))
	blobKey, _ := f.runExport(c, signer)

	other := must.Must(itemTypes.GetBySlug(f.ctx, models.CommodityTypeOther))
	must.Must(fields.DeleteWithValues(f.ctx, plate.ID, registry.AnyValueCount))
	must.Must(itemTypes.DeleteReassigning(f.ctx, vehicle.ID, other.ID))
	for i := must.Must(itemTypes.Count(f.ctx)); i < models.MaxItemTypesPerGroup; i++ {
		must.Must(itemTypes.Create(f.ctx, models.ItemType{Name: fmt.Sprintf("Kind %d", i)}))
	}

	final, err := restoreInb(c, f.inbFixture, signer, blobKey)
	c.Assert(err, qt.IsNil)
	c.Check(final.Status, qt.Equals, models.RestoreStatusCompleted, qt.Commentf("errors: %v", final.ErrorMessage))

	restored, err := fields.GetByName(f.ctx, "Plate")
	c.Assert(err, qt.IsNil)
	c.Check(restored.Templates, qt.HasLen, 0)
}

// TestINBRoundTrip_DraftCustomFieldsFollowTheFields: a draft's custom field
// values are archived like an item's and map onto the restored fields, a
// link onto the restored item, in a group whose fields have other ids.
func TestINBRoundTrip_DraftCustomFieldsFollowTheFields(t *testing.T) {
	c := qt.New(t)
	signer := testSigner(c)
	src := newCustomFieldFixture(c)
	remote := src.commodityByUUID(c, src.remoteUUID)
	drafts := must.Must(src.fs.CommodityDraftRegistryFactory.CreateUserRegistry(src.ctx))
	must.Must2(drafts.CreateOrGet(src.ctx, models.CommodityDraft{
		ClientKey: "draft-with-fields",
		Values: models.CommodityDraftValues{
			"name": "Second TV",
			// Entries as the item form keeps them, and one as an item
			// stores it.
			"custom_fields": map[string]any{
				src.serial.ID:   map[string]any{"value": "SN-7", "currency": ""},
				src.fuel.ID:     map[string]any{"value": src.fuel.Options[0].ID},
				src.size.ID:     float64(0),
				src.link.ID:     map[string]any{"value": remote.ID},
				"field-gone-id": map[string]any{"value": "dropped"},
			},
		},
	}))

	blobKey, archive := src.runExport(c, signer)
	_, jsons := innerMembers(c, archive)
	var doc types.INBDraftsDoc
	c.Assert(json.Unmarshal(jsons[types.INBDraftsMember], &doc), qt.IsNil)
	c.Assert(doc.Drafts, qt.HasLen, 1)
	c.Check(doc.Drafts[0].Values["custom_fields"], qt.DeepEquals, map[string]any{
		src.serial.UUID: map[string]any{"value": "SN-7", "currency": ""},
		src.fuel.UUID:   map[string]any{"value": src.fuel.Options[0].ID},
		src.size.UUID:   float64(0),
		src.link.UUID:   map[string]any{"value": src.remoteUUID},
	}, qt.Commentf("keyed by field UUID, a link naming the item's UUID, an unknown field left out"))

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

	fieldsB := map[string]*models.CustomFieldDefinition{}
	for _, d := range must.Must(must.Must(src.fs.CustomFieldRegistryFactory.CreateUserRegistry(dst.ctx)).List(dst.ctx)) {
		fieldsB[d.Name] = d
	}
	c.Assert(fieldsB["Serial"], qt.IsNotNil)
	c.Check(fieldsB["Serial"].ID, qt.Not(qt.Equals), src.serial.ID)
	var remoteB *models.Commodity
	for _, com := range must.Must(must.Must(src.fs.CommodityRegistryFactory.CreateUserRegistry(dst.ctx)).List(dst.ctx)) {
		if com.Name == "Remote" {
			remoteB = com
		}
	}
	c.Assert(remoteB, qt.IsNotNil)
	c.Check(remoteB.ID, qt.Not(qt.Equals), remote.ID)

	restored := must.Must(must.Must(src.fs.CommodityDraftRegistryFactory.CreateUserRegistry(dst.ctx)).GetByClientKey(dst.ctx, "draft-with-fields"))
	c.Check(restored.Values["custom_fields"], qt.DeepEquals, map[string]any{
		fieldsB["Serial"].ID: map[string]any{"value": "SN-7", "currency": ""},
		fieldsB["Fuel"].ID:   map[string]any{"value": fieldsB["Fuel"].Options[0].ID},
		fieldsB["Size"].ID:   float64(0),
		fieldsB["Remote"].ID: map[string]any{"value": remoteB.ID},
	})
}
