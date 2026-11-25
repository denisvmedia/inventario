package registrytest

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CustomFieldEnv is what RunCustomFieldContract needs from a backend:
// registries of two groups of one tenant, the contexts that carry each
// group's scope, and an area of group A items can be placed in. Both groups
// start without custom fields and without items.
type CustomFieldEnv struct {
	CtxA, CtxB                 context.Context
	FieldsA, FieldsB           registry.CustomFieldRegistry
	TypesA                     registry.ItemTypeRegistry
	CommoditiesA, CommoditiesB registry.CommodityRegistry
	AreaA, AreaB               string
}

// RunCustomFieldContract checks the behavior registry.CustomFieldRegistry
// and the custom field values of registry.CommodityRegistry promise (#2803):
// names are unique per group and types never change, item writes check each
// value against its field, a null clears a value and a missing key keeps
// it, links name another live item of the group, a dropped option or a
// deleted field takes its values along, templates name the group's item
// types, and the list filters, sorts and search by custom field values.
func RunCustomFieldContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Helper()

	t.Run("create normalizes, names are unique per group, a group sees only its own fields", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		plate, err := env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{Name: "  License   plate ", Type: models.CustomFieldTypeText})
		c.Assert(err, qt.IsNil)
		c.Check(plate.ID, qt.Not(qt.Equals), "")
		c.Check(plate.Name, qt.Equals, "License plate")
		c.Check(plate.Type, qt.Equals, models.CustomFieldTypeText)
		c.Check(plate.Templates, qt.HasLen, 0)

		for _, dup := range []string{"license plate", "LICENSE PLATE", "License\tplate"} {
			_, err := env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{Name: dup, Type: models.CustomFieldTypeNumber})
			c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists, qt.Commentf("name %q", dup))
		}
		_, err = env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{Name: "Fuel", Type: models.CustomFieldTypeSelect})
		c.Check(err, qt.IsNotNil, qt.Commentf("a select field needs options"))

		fuel, err := env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{
			Name: "Fuel", Type: models.CustomFieldTypeSelect,
			Options: models.ValuerSlice[models.CustomFieldOption]{{Label: "Petrol"}, {Label: " Diesel "}},
		})
		c.Assert(err, qt.IsNil)
		c.Assert(fuel.Options, qt.HasLen, 2)
		c.Check(fuel.Options[0].ID, qt.Not(qt.Equals), "")
		c.Check(fuel.Options[1].Label, qt.Equals, "Diesel")

		list, err := env.FieldsA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(fieldNames(list), qt.DeepEquals, []string{"Fuel", "License plate"})

		got, err := env.FieldsA.GetByName(env.CtxA, "license plate")
		c.Assert(err, qt.IsNil)
		c.Check(got.ID, qt.Equals, plate.ID)

		inB, err := env.FieldsB.Create(env.CtxB, models.CustomFieldDefinition{Name: "License plate", Type: models.CustomFieldTypeText})
		c.Assert(err, qt.IsNil)
		_, err = env.FieldsB.Get(env.CtxB, plate.ID)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)
		listB, err := env.FieldsB.List(env.CtxB)
		c.Assert(err, qt.IsNil)
		c.Check(fieldIDs(listB), qt.DeepEquals, []string{inB.ID})
	})

	t.Run("update renames, keeps the type and the values", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		field := mustField(c, env.FieldsA, env.CtxA, "Plate", models.CustomFieldTypeText)
		item := mustCustomFieldItem(c, env, "Car", models.CustomFieldValues{field.ID: rawJSON(`"AB 123"`)})

		renamed := *field
		renamed.Name = "Registration"
		renamed.Type = models.CustomFieldTypeNumber
		updated, err := env.FieldsA.Update(env.CtxA, renamed)
		c.Assert(err, qt.IsNil)
		c.Check(updated.Name, qt.Equals, "Registration")
		c.Check(updated.Type, qt.Equals, models.CustomFieldTypeText)
		c.Check(updated.CreatedAt.Equal(field.CreatedAt), qt.IsTrue)

		got, err := env.CommoditiesA.Get(env.CtxA, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(string(got.CustomFields[field.ID]), qt.Equals, `"AB 123"`)

		other := mustField(c, env.FieldsA, env.CtxA, "Other", models.CustomFieldTypeText)
		clash := *other
		clash.Name = "registration"
		_, err = env.FieldsA.Update(env.CtxA, clash)
		c.Check(err, qt.ErrorIs, registry.ErrAlreadyExists)
	})

	runCustomFieldValueContract(t, newEnv)
	runCustomFieldLinkContract(t, newEnv)
	runCustomFieldDeleteContract(t, newEnv)
	runCustomFieldTemplateContract(t, newEnv)
	runCustomFieldQueryContract(t, newEnv)
}

func runCustomFieldValueContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Run("item writes store a value of every type in its canonical form", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		text := mustField(c, env.FieldsA, env.CtxA, "Text", models.CustomFieldTypeText)
		long := mustField(c, env.FieldsA, env.CtxA, "Notes", models.CustomFieldTypeLongText)
		number := mustField(c, env.FieldsA, env.CtxA, "Number", models.CustomFieldTypeNumber)
		date := mustField(c, env.FieldsA, env.CtxA, "Date", models.CustomFieldTypeDate)
		flag := mustField(c, env.FieldsA, env.CtxA, "Flag", models.CustomFieldTypeBoolean)
		link := mustField(c, env.FieldsA, env.CtxA, "Web", models.CustomFieldTypeURL)
		money := mustField(c, env.FieldsA, env.CtxA, "Paid", models.CustomFieldTypeMoney)
		fuel := mustSelectField(c, env, "Fuel", "Petrol", "Diesel")

		item := mustCustomFieldItem(c, env, "Car", models.CustomFieldValues{
			text.ID:   rawJSON(`"  AB 123 "`),
			long.ID:   rawJSON(`"line one\nline two"`),
			number.ID: rawJSON(`"0012.50"`),
			date.ID:   rawJSON(`"2026-03-01"`),
			flag.ID:   rawJSON(`false`),
			link.ID:   rawJSON(`"https://example.com/manual"`),
			money.ID:  rawJSON(`{"amount": 1999.5, "currency": "eur"}`),
			fuel.ID:   rawJSON(`"` + fuel.Options[1].ID + `"`),
		})
		want := map[string]string{
			text.ID:   `"AB 123"`,
			long.ID:   `"line one\nline two"`,
			number.ID: `12.5`,
			date.ID:   `"2026-03-01"`,
			flag.ID:   `false`,
			link.ID:   `"https://example.com/manual"`,
			money.ID:  `{"amount":1999.50,"currency":"EUR"}`,
			fuel.ID:   `"` + fuel.Options[1].ID + `"`,
		}
		c.Check(valueStrings(item.CustomFields), qt.DeepEquals, want, qt.Commentf("returned by Create"))
		got, err := env.CommoditiesA.Get(env.CtxA, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(valueStrings(got.CustomFields), qt.DeepEquals, want, qt.Commentf("read back"))

		page, _, err := env.CommoditiesA.ListPaginated(env.CtxA, 0, 10, registry.CommodityListOptions{})
		c.Assert(err, qt.IsNil)
		c.Assert(page, qt.HasLen, 1)
		c.Check(valueStrings(page[0].CustomFields), qt.DeepEquals, want, qt.Commentf("listed"))
	})

	t.Run("a nil map keeps the values, a missing key keeps one, a null clears one", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		a := mustField(c, env.FieldsA, env.CtxA, "A", models.CustomFieldTypeText)
		b := mustField(c, env.FieldsA, env.CtxA, "B", models.CustomFieldTypeText)
		flag := mustField(c, env.FieldsA, env.CtxA, "Flag", models.CustomFieldTypeBoolean)
		item := mustCustomFieldItem(c, env, "Thing", models.CustomFieldValues{
			a.ID: rawJSON(`"one"`), b.ID: rawJSON(`""`), flag.ID: rawJSON(`false`),
		})
		c.Check(valueStrings(item.CustomFields), qt.DeepEquals, map[string]string{a.ID: `"one"`, b.ID: `""`, flag.ID: `false`},
			qt.Commentf("empty text and false are values"))

		item.Name = "Renamed"
		item.CustomFields = nil
		updated, err := env.CommoditiesA.Update(env.CtxA, *item)
		c.Assert(err, qt.IsNil)
		c.Check(valueStrings(updated.CustomFields), qt.DeepEquals, map[string]string{a.ID: `"one"`, b.ID: `""`, flag.ID: `false`})

		updated.CustomFields = models.CustomFieldValues{a.ID: rawJSON(`null`), flag.ID: rawJSON(`true`), "deleted-field": rawJSON(`null`)}
		updated, err = env.CommoditiesA.Update(env.CtxA, *updated)
		c.Assert(err, qt.IsNil)
		c.Check(valueStrings(updated.CustomFields), qt.DeepEquals, map[string]string{b.ID: `""`, flag.ID: `true`})

		got, err := env.CommoditiesA.Get(env.CtxA, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(valueStrings(got.CustomFields), qt.DeepEquals, map[string]string{b.ID: `""`, flag.ID: `true`})
	})

	t.Run("refused values fail the whole write and name their fields", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		number := mustField(c, env.FieldsA, env.CtxA, "Number", models.CustomFieldTypeNumber)
		fuel := mustSelectField(c, env, "Fuel", "Petrol")
		fieldB := mustField(c, env.FieldsB, env.CtxB, "In B", models.CustomFieldTypeText)
		item := mustCustomFieldItem(c, env, "Thing", models.CustomFieldValues{number.ID: rawJSON(`1`)})

		item.Name = "Changed"
		item.CustomFields = models.CustomFieldValues{
			number.ID:  rawJSON(`"many"`),
			fuel.ID:    rawJSON(`"nosuchoption"`),
			fieldB.ID:  rawJSON(`"x"`),
			"no-field": rawJSON(`"x"`),
		}
		_, err := env.CommoditiesA.Update(env.CtxA, *item)
		c.Assert(err, qt.ErrorIs, registry.ErrInvalidCustomFieldValue)
		valueErr, ok := errors.AsType[*registry.CustomFieldValueError](err)
		c.Assert(ok, qt.IsTrue)
		c.Check(sortedKeys(valueErr.Fields), qt.DeepEquals, sortedStrings(number.ID, fuel.ID, fieldB.ID, "no-field"))

		got, err := env.CommoditiesA.Get(env.CtxA, item.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.Name, qt.Equals, "Thing", qt.Commentf("nothing of the write is stored"))
		c.Check(valueStrings(got.CustomFields), qt.DeepEquals, map[string]string{number.ID: `1`})

		_, err = env.CommoditiesA.Create(env.CtxA, customFieldItem("New", env.AreaA, models.CustomFieldValues{fieldB.ID: rawJSON(`"x"`)}))
		c.Check(err, qt.ErrorIs, registry.ErrInvalidCustomFieldValue)
	})
}

func runCustomFieldLinkContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Run("a link names another live item of the group and goes when the item is purged", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		link := mustField(c, env.FieldsA, env.CtxA, "Charger", models.CustomFieldTypeItemLink)
		charger := mustCustomFieldItem(c, env, "Charger", nil)
		phone := mustCustomFieldItem(c, env, "Phone", models.CustomFieldValues{link.ID: rawJSON(`"` + charger.ID + `"`)})
		c.Check(string(phone.CustomFields[link.ID]), qt.Equals, `"`+charger.ID+`"`)

		phone.CustomFields = models.CustomFieldValues{link.ID: rawJSON(`"` + phone.ID + `"`)}
		_, err := env.CommoditiesA.Update(env.CtxA, *phone)
		c.Check(err, qt.ErrorIs, registry.ErrInvalidCustomFieldValue, qt.Commentf("a link to itself"))

		inB, err := env.CommoditiesB.Create(env.CtxB, customFieldItem("Elsewhere", env.AreaB, nil))
		c.Assert(err, qt.IsNil)
		phone.CustomFields = models.CustomFieldValues{link.ID: rawJSON(`"` + inB.ID + `"`)}
		_, err = env.CommoditiesA.Update(env.CtxA, *phone)
		c.Check(err, qt.ErrorIs, registry.ErrInvalidCustomFieldValue, qt.Commentf("a link to another group's item"))

		// Trash keeps the link, and an unchanged link to a trashed item is
		// kept by a later write; a new link to one is refused.
		_, err = env.CommoditiesA.Trash(env.CtxA, charger.ID, "", time.Now())
		c.Assert(err, qt.IsNil)
		got, err := env.CommoditiesA.Get(env.CtxA, phone.ID)
		c.Assert(err, qt.IsNil)
		c.Check(string(got.CustomFields[link.ID]), qt.Equals, `"`+charger.ID+`"`)
		got.Name = "Phone 2"
		got, err = env.CommoditiesA.Update(env.CtxA, *got)
		c.Assert(err, qt.IsNil)
		c.Check(string(got.CustomFields[link.ID]), qt.Equals, `"`+charger.ID+`"`)

		other := mustCustomFieldItem(c, env, "Tablet", nil)
		other.CustomFields = models.CustomFieldValues{link.ID: rawJSON(`"` + charger.ID + `"`)}
		_, err = env.CommoditiesA.Update(env.CtxA, *other)
		c.Check(err, qt.ErrorIs, registry.ErrInvalidCustomFieldValue, qt.Commentf("a new link to a trashed item"))

		purged, err := env.CommoditiesA.PurgeTrashed(env.CtxA, charger.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(purged, qt.IsTrue)
		got, err = env.CommoditiesA.Get(env.CtxA, phone.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.CustomFields, qt.HasLen, 0)
	})
}

func runCustomFieldDeleteContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Run("a dropped option takes its values and template defaults along", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		car := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeOther)
		fuel := mustSelectField(c, env, "Fuel", "Petrol", "Diesel")
		petrol, diesel := fuel.Options[0], fuel.Options[1]
		withTemplate := *fuel
		withTemplate.Templates = []models.CustomFieldTemplate{{ItemTypeID: car.ID, DefaultValue: models.JSONValue(`"` + diesel.ID + `"`)}}
		_, err := env.FieldsA.Update(env.CtxA, withTemplate)
		c.Assert(err, qt.IsNil)
		a := mustCustomFieldItem(c, env, "A", models.CustomFieldValues{fuel.ID: rawJSON(`"` + petrol.ID + `"`)})
		b := mustCustomFieldItem(c, env, "B", models.CustomFieldValues{fuel.ID: rawJSON(`"` + diesel.ID + `"`)})

		usage, err := env.FieldsA.Usage(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(usage[fuel.ID].Options, qt.DeepEquals, map[string]int{petrol.ID: 1, diesel.ID: 1})

		drop := *fuel
		drop.Options = models.ValuerSlice[models.CustomFieldOption]{{ID: petrol.ID, Label: "Gasoline"}}
		drop.Templates = nil
		updated, err := env.FieldsA.Update(env.CtxA, drop)
		c.Assert(err, qt.IsNil)
		c.Check(updated.Options, qt.DeepEquals, models.ValuerSlice[models.CustomFieldOption]{{ID: petrol.ID, Label: "Gasoline"}})
		c.Assert(updated.Templates, qt.HasLen, 1)
		c.Check(updated.Templates[0].DefaultValue.IsNull(), qt.IsTrue)

		gotA, err := env.CommoditiesA.Get(env.CtxA, a.ID)
		c.Assert(err, qt.IsNil)
		c.Check(string(gotA.CustomFields[fuel.ID]), qt.Equals, `"`+petrol.ID+`"`, qt.Commentf("a relabeled option keeps its values"))
		gotB, err := env.CommoditiesA.Get(env.CtxA, b.ID)
		c.Assert(err, qt.IsNil)
		c.Check(gotB.CustomFields, qt.HasLen, 0)
	})

	t.Run("a field in use is deleted only with its values", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		field := mustField(c, env.FieldsA, env.CtxA, "Serial", models.CustomFieldTypeText)
		unused := mustField(c, env.FieldsA, env.CtxA, "Unused", models.CustomFieldTypeText)
		live := mustCustomFieldItem(c, env, "Live", models.CustomFieldValues{field.ID: rawJSON(`"1"`)})
		trashed := mustCustomFieldItem(c, env, "Trashed", models.CustomFieldValues{field.ID: rawJSON(`"2"`)})
		_, err := env.CommoditiesA.Trash(env.CtxA, trashed.ID, "", time.Now())
		c.Assert(err, qt.IsNil)

		usage, err := env.FieldsA.Usage(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(usage[field.ID].Live, qt.Equals, 1)
		c.Check(usage[field.ID].Trashed, qt.Equals, 1)
		c.Check(usage[field.ID].Total(), qt.Equals, 2)
		_, listed := usage[unused.ID]
		c.Check(listed, qt.IsFalse)

		err = env.FieldsA.Delete(env.CtxA, field.ID)
		c.Check(err, qt.ErrorIs, registry.ErrCustomFieldInUse)
		_, err = env.FieldsA.Get(env.CtxA, field.ID)
		c.Assert(err, qt.IsNil)

		c.Assert(env.FieldsA.Delete(env.CtxA, unused.ID), qt.IsNil)
		_, err = env.FieldsA.Get(env.CtxA, unused.ID)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)

		n, err := env.FieldsA.DeleteWithValues(env.CtxA, field.ID)
		c.Assert(err, qt.IsNil)
		c.Check(n, qt.Equals, 2)
		_, err = env.FieldsA.Get(env.CtxA, field.ID)
		c.Check(err, qt.ErrorIs, registry.ErrNotFound)
		got, err := env.CommoditiesA.Get(env.CtxA, live.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.CustomFields, qt.HasLen, 0)
		gotTrashed, err := env.CommoditiesA.GetTrashed(env.CtxA, trashed.ID)
		c.Assert(err, qt.IsNil)
		c.Check(gotTrashed.CustomFields, qt.HasLen, 0)
	})
}

func runCustomFieldTemplateContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Run("templates name the group's item types and are replaced or kept", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		other := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeOther)
		vehicle, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "Vehicle"})
		c.Assert(err, qt.IsNil)

		field, err := env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{
			Name: "Wheels", Type: models.CustomFieldTypeNumber,
			Templates: []models.CustomFieldTemplate{{ItemTypeID: vehicle.ID, DefaultValue: models.JSONValue(`"4"`)}},
		})
		c.Assert(err, qt.IsNil)
		c.Check(field.Templates, qt.DeepEquals, []models.CustomFieldTemplate{{ItemTypeID: vehicle.ID, DefaultValue: models.JSONValue(`4`)}})

		_, err = env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{
			Name: "Doors", Type: models.CustomFieldTypeNumber,
			Templates: []models.CustomFieldTemplate{{ItemTypeID: "no-such-type"}},
		})
		c.Check(err, qt.ErrorIs, registry.ErrInvalidInput)
		_, err = env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{
			Name: "Doors", Type: models.CustomFieldTypeNumber,
			Templates: []models.CustomFieldTemplate{{ItemTypeID: vehicle.ID, DefaultValue: models.JSONValue(`"four"`)}},
		})
		c.Check(err, qt.IsNotNil, qt.Commentf("a default must be a valid value"))

		rename := *field
		rename.Name = "Wheel count"
		rename.Templates = nil
		updated, err := env.FieldsA.Update(env.CtxA, rename)
		c.Assert(err, qt.IsNil)
		c.Check(updated.Templates, qt.HasLen, 1, qt.Commentf("nil keeps the templates"))

		replace := *updated
		replace.Templates = []models.CustomFieldTemplate{{ItemTypeID: other.ID}}
		updated, err = env.FieldsA.Update(env.CtxA, replace)
		c.Assert(err, qt.IsNil)
		c.Check(updated.Templates, qt.HasLen, 1)
		c.Check(updated.Templates[0].ItemTypeID, qt.Equals, other.ID)
		c.Check(updated.Templates[0].DefaultValue.IsNull(), qt.IsTrue)

		both := *updated
		both.Templates = []models.CustomFieldTemplate{{ItemTypeID: other.ID}, {ItemTypeID: vehicle.ID}}
		_, err = env.FieldsA.Update(env.CtxA, both)
		c.Assert(err, qt.IsNil)
		c.Assert(env.TypesA.Delete(env.CtxA, vehicle.ID), qt.IsNil)
		got, err := env.FieldsA.Get(env.CtxA, field.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.Templates, qt.DeepEquals, []models.CustomFieldTemplate{{ItemTypeID: other.ID}},
			qt.Commentf("deleting the item type drops its entry"))
	})
}

func runCustomFieldQueryContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Run("the list filters, sorts and searches by custom field values", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)

		plate := mustField(c, env.FieldsA, env.CtxA, "Plate", models.CustomFieldTypeText)
		km := mustField(c, env.FieldsA, env.CtxA, "Kilometers", models.CustomFieldTypeNumber)
		inspected := mustField(c, env.FieldsA, env.CtxA, "Inspected", models.CustomFieldTypeDate)
		electric := mustField(c, env.FieldsA, env.CtxA, "Electric", models.CustomFieldTypeBoolean)
		fuel := mustSelectField(c, env, "Fuel", "Petrol", "Diesel", "Hydrogen")
		petrol, diesel := fuel.Options[0].ID, fuel.Options[1].ID

		mustCustomFieldItem(c, env, "Alpha", models.CustomFieldValues{
			plate.ID: rawJSON(`"AB-100"`), km.ID: rawJSON(`0`), inspected.ID: rawJSON(`"2025-01-10"`),
			electric.ID: rawJSON(`false`), fuel.ID: rawJSON(`"` + diesel + `"`),
		})
		mustCustomFieldItem(c, env, "Bravo", models.CustomFieldValues{
			plate.ID: rawJSON(`"xy-200"`), km.ID: rawJSON(`15000.5`), inspected.ID: rawJSON(`"2026-06-01"`),
			electric.ID: rawJSON(`true`), fuel.ID: rawJSON(`"` + petrol + `"`),
		})
		mustCustomFieldItem(c, env, "Charlie", nil)

		list := func(opts registry.CommodityListOptions) []string {
			c.Helper()
			items, total, err := env.CommoditiesA.ListPaginated(env.CtxA, 0, 50, opts)
			c.Assert(err, qt.IsNil)
			c.Check(total, qt.Equals, len(items))
			names := make([]string, 0, len(items))
			for _, it := range items {
				names = append(names, it.Name)
			}
			return names
		}
		filter := func(fs ...registry.CustomFieldFilter) []string {
			return list(registry.CommodityListOptions{CustomFields: fs})
		}
		f := func(def *models.CustomFieldDefinition, op registry.CustomFieldFilterOp) registry.CustomFieldFilter {
			return registry.CustomFieldFilter{FieldID: def.ID, Type: def.Type, Op: op}
		}
		withText := func(ff registry.CustomFieldFilter, s string) registry.CustomFieldFilter { ff.Text = s; return ff }
		withNumber := func(ff registry.CustomFieldFilter, s string) registry.CustomFieldFilter {
			ff.Number = decimal.RequireFromString(s)
			return ff
		}

		c.Check(filter(f(km, registry.CustomFieldOpSet)), qt.DeepEquals, []string{"Alpha", "Bravo"}, qt.Commentf("zero is a value"))
		c.Check(filter(f(km, registry.CustomFieldOpUnset)), qt.DeepEquals, []string{"Charlie"})
		c.Check(filter(withNumber(f(km, registry.CustomFieldOpEq), "0")), qt.DeepEquals, []string{"Alpha"})
		c.Check(filter(withNumber(f(km, registry.CustomFieldOpGte), "1")), qt.DeepEquals, []string{"Bravo"})
		c.Check(filter(withNumber(f(km, registry.CustomFieldOpLte), "15000.5")), qt.DeepEquals, []string{"Alpha", "Bravo"})
		c.Check(filter(withText(f(plate, registry.CustomFieldOpContains), "XY")), qt.DeepEquals, []string{"Bravo"})
		c.Check(filter(withText(f(plate, registry.CustomFieldOpEq), "ab-100")), qt.DeepEquals, []string{"Alpha"})
		c.Check(filter(withText(f(plate, registry.CustomFieldOpContains), "%")), qt.HasLen, 0, qt.Commentf("a wildcard is literal"))
		c.Check(filter(withText(f(inspected, registry.CustomFieldOpGte), "2026-01-01")), qt.DeepEquals, []string{"Bravo"})
		c.Check(filter(withText(f(inspected, registry.CustomFieldOpLte), "2025-01-10")), qt.DeepEquals, []string{"Alpha"})
		boolFilter := f(electric, registry.CustomFieldOpEq)
		c.Check(filter(boolFilter), qt.DeepEquals, []string{"Alpha"}, qt.Commentf("false is a value"))
		inFilter := f(fuel, registry.CustomFieldOpIn)
		inFilter.OptionIDs = []string{petrol, fuel.Options[2].ID}
		c.Check(filter(inFilter), qt.DeepEquals, []string{"Bravo"})
		c.Check(filter(f(km, registry.CustomFieldOpSet), withText(f(plate, registry.CustomFieldOpContains), "ab")), qt.DeepEquals, []string{"Alpha"})
		c.Check(filter(registry.CustomFieldFilter{FieldID: "deleted-field", Op: registry.CustomFieldOpUnset}), qt.HasLen, 0,
			qt.Commentf("a filter on a field that does not exist keeps nothing"))

		sorted := func(def *models.CustomFieldDefinition, desc bool) []string {
			return list(registry.CommodityListOptions{
				CustomFieldSort: &registry.CustomFieldSort{FieldID: def.ID, Type: def.Type, OptionOrder: def.OptionIDs()},
				SortDesc:        desc,
			})
		}
		c.Check(sorted(km, false), qt.DeepEquals, []string{"Alpha", "Bravo", "Charlie"})
		c.Check(sorted(km, true), qt.DeepEquals, []string{"Bravo", "Alpha", "Charlie"}, qt.Commentf("no value sorts last both ways"))
		c.Check(sorted(plate, false), qt.DeepEquals, []string{"Alpha", "Bravo", "Charlie"}, qt.Commentf("text ignores case"))
		c.Check(sorted(fuel, false), qt.DeepEquals, []string{"Bravo", "Alpha", "Charlie"}, qt.Commentf("options sort in their order"))
		c.Check(sorted(electric, true), qt.DeepEquals, []string{"Bravo", "Alpha", "Charlie"})
		c.Check(sorted(inspected, true), qt.DeepEquals, []string{"Bravo", "Alpha", "Charlie"})

		search := func(term string, refs ...registry.CustomFieldOptionRef) []string {
			return list(registry.CommodityListOptions{
				Search: term,
				SearchCustomFields: registry.CustomFieldSearch{
					TextFields: map[string]bool{plate.ID: true},
					Options:    map[string][]registry.CustomFieldOptionRef{term: refs},
				},
			})
		}
		c.Check(search("xy-2"), qt.DeepEquals, []string{"Bravo"})
		c.Check(search("diese", registry.CustomFieldOptionRef{FieldID: fuel.ID, OptionID: diesel, Label: "Diesel"}), qt.DeepEquals, []string{"Alpha"})
		c.Check(search("15000"), qt.HasLen, 0, qt.Commentf("a number is not searched"))
	})
}

func customFieldItem(name, areaID string, values models.CustomFieldValues) models.Commodity {
	return models.Commodity{
		Name:         name,
		ShortName:    name,
		Type:         models.CommodityTypeOther,
		AreaID:       new(areaID),
		Count:        1,
		Status:       models.CommodityStatusInUse,
		CustomFields: values,
	}
}

func mustCustomFieldItem(c *qt.C, env CustomFieldEnv, name string, values models.CustomFieldValues) *models.Commodity {
	c.Helper()
	item, err := env.CommoditiesA.Create(env.CtxA, customFieldItem(name, env.AreaA, values))
	c.Assert(err, qt.IsNil)
	return item
}

func mustField(c *qt.C, reg registry.CustomFieldRegistry, ctx context.Context, name string, t models.CustomFieldType) *models.CustomFieldDefinition {
	c.Helper()
	d, err := reg.Create(ctx, models.CustomFieldDefinition{Name: name, Type: t})
	c.Assert(err, qt.IsNil)
	return d
}

func mustSelectField(c *qt.C, env CustomFieldEnv, name string, labels ...string) *models.CustomFieldDefinition {
	c.Helper()
	options := make(models.ValuerSlice[models.CustomFieldOption], 0, len(labels))
	for _, l := range labels {
		options = append(options, models.CustomFieldOption{Label: l})
	}
	d, err := env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{Name: name, Type: models.CustomFieldTypeSelect, Options: options})
	c.Assert(err, qt.IsNil)
	return d
}

func rawJSON(s string) json.RawMessage {
	return json.RawMessage(s)
}

func valueStrings(values models.CustomFieldValues) map[string]string {
	out := make(map[string]string, len(values))
	for k, v := range values {
		out[k] = string(v)
	}
	return out
}

func fieldNames(defs []*models.CustomFieldDefinition) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.Name)
	}
	return out
}

func fieldIDs(defs []*models.CustomFieldDefinition) []string {
	out := make([]string, 0, len(defs))
	for _, d := range defs {
		out = append(out, d.ID)
	}
	return out
}

func sortedKeys(m map[string]error) []string {
	return slices.Sorted(maps.Keys(m))
}

func sortedStrings(in ...string) []string {
	return slices.Sorted(slices.Values(in))
}
