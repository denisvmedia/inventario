package registrytest

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
)

// runItemSetContract checks services.ItemSetService over the backend's item
// type and custom field registries (#1607): adding the Collection set is
// idempotent and reuses a member's own rows of the same name, removing it
// deletes only what it created and nothing uses, and both are safe to
// repeat. It runs as part of RunCustomFieldContract, whose env has the
// registries it needs.
func runItemSetContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Helper()
	def := models.CollectionSet
	newService := func(env CustomFieldEnv) *services.ItemSetService {
		return services.NewItemSetService(def, env.TypesA, env.FieldsA)
	}

	t.Run("item set: adding creates the types and fields once, with every field on every type", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		before, err := svc.Status(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(before.Complete(), qt.IsFalse)
		for _, ts := range before.ItemTypes {
			c.Check(ts.State, qt.Equals, services.ItemSetEntryMissing, qt.Commentf("type %s", ts.Entry.Key))
		}

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(res.CreatedItemTypes, qt.Equals, len(def.ItemTypes))
		c.Check(res.CreatedFields, qt.Equals, len(def.Fields))
		c.Check(res.AddedTemplates, qt.Equals, 0)
		c.Assert(res.Status.Complete(), qt.IsTrue)

		typeIDs := make([]string, 0, len(def.ItemTypes))
		for i, ts := range res.Status.ItemTypes {
			c.Check(ts.State, qt.Equals, services.ItemSetEntryAdded)
			c.Check(ts.Removable(), qt.IsTrue)
			c.Check(ts.ItemType.Name, qt.Equals, def.ItemTypes[i].Name)
			c.Check(ts.ItemType.Icon, qt.Equals, def.ItemTypes[i].Icon)
			c.Check(ts.ItemType.SetEntry, qt.Equals, def.Entry(def.ItemTypes[i].Key))
			typeIDs = append(typeIDs, ts.ItemType.ID)
		}
		slices.Sort(typeIDs)
		for i, fs := range res.Status.Fields {
			c.Check(fs.State, qt.Equals, services.ItemSetEntryAdded)
			c.Check(fs.Field.Name, qt.Equals, def.Fields[i].Name)
			c.Check(fs.Field.Type, qt.Equals, def.Fields[i].Type)
			c.Check(templateTypeIDs(fs.Field.Templates), qt.DeepEquals, typeIDs, qt.Commentf("field %s", fs.Entry.Key))
		}
		condition := res.Status.Fields[0].Field
		c.Check(condition.OptionIDs(), qt.DeepEquals, []string{"mint", "verygood", "good", "fair", "poor"})

		again, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(again.CreatedItemTypes, qt.Equals, 0)
		c.Check(again.CreatedFields, qt.Equals, 0)
		c.Check(again.AddedTemplates, qt.Equals, 0)
		types, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(types, qt.HasLen, len(models.BuiltinItemTypes)+len(def.ItemTypes))
		fields, err := env.FieldsA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(fields, qt.HasLen, len(def.Fields))

		inB, err := env.FieldsB.List(env.CtxB)
		c.Assert(err, qt.IsNil)
		c.Check(inB, qt.HasLen, 0, qt.Commentf("another group gets nothing"))
	})

	t.Run("item set: a member's own type and field of the same name are used and kept", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		other := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeOther)
		books, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: "BOOKS", Icon: "book-open"})
		c.Assert(err, qt.IsNil)
		condition, err := env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{
			Name: "  condition ", Type: models.CustomFieldTypeText,
			Templates: []models.CustomFieldTemplate{{ItemTypeID: other.ID, DefaultValue: models.JSONValue(`"used"`)}},
		})
		c.Assert(err, qt.IsNil)

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(res.CreatedItemTypes, qt.Equals, len(def.ItemTypes)-1)
		c.Check(res.CreatedFields, qt.Equals, len(def.Fields)-1)
		c.Check(res.AddedTemplates, qt.Equals, len(def.ItemTypes), qt.Commentf("the own field goes on every type of the set"))

		booksStatus := res.Status.ItemTypes[1]
		c.Check(booksStatus.State, qt.Equals, services.ItemSetEntryOwn)
		c.Check(booksStatus.ItemType.ID, qt.Equals, books.ID)
		c.Check(booksStatus.KeepReason, qt.Equals, services.ItemSetKeepOwn)
		conditionStatus := res.Status.Fields[0]
		c.Check(conditionStatus.State, qt.Equals, services.ItemSetEntryOwn)
		c.Check(conditionStatus.Field.ID, qt.Equals, condition.ID)
		c.Check(conditionStatus.Field.Type, qt.Equals, models.CustomFieldTypeText, qt.Commentf("the own field keeps its type"))
		c.Check(templatesByType(conditionStatus.Field.Templates)[other.ID], qt.Equals, `"used"`)
		c.Check(templatesByType(res.Status.Fields[1].Field.Templates)[books.ID], qt.Equals, "",
			qt.Commentf("a field of the set goes on the own type's template"))

		removed, err := svc.Remove(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(removed.DeletedItemTypes, qt.HasLen, len(def.ItemTypes)-1)
		c.Check(removed.DeletedFields, qt.HasLen, len(def.Fields)-1)
		gotBooks, err := env.TypesA.Get(env.CtxA, books.ID)
		c.Assert(err, qt.IsNil)
		c.Check(gotBooks.Name, qt.Equals, "BOOKS")
		gotCondition, err := env.FieldsA.Get(env.CtxA, condition.ID)
		c.Assert(err, qt.IsNil)
		c.Check(templatesByType(gotCondition.Templates), qt.DeepEquals, map[string]string{other.ID: `"used"`, books.ID: ""},
			qt.Commentf("the own field keeps its entries on types that stay"))
		c.Check(removed.Status.ItemTypes[1].State, qt.Equals, services.ItemSetEntryOwn)
		c.Check(removed.Status.Fields[0].State, qt.Equals, services.ItemSetEntryOwn)
	})

	t.Run("item set: removing deletes what nothing uses, keeps what items use, and can be repeated", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		coins := res.Status.ItemTypes[2].ItemType
		c.Assert(coins.Name, qt.Equals, "Coins")
		condition := res.Status.Fields[0].Field
		appraisalDate := res.Status.Fields[7].Field
		c.Assert(appraisalDate.Name, qt.Equals, "Appraisal date")

		coin := customFieldItem("Thaler", env.AreaA, models.CustomFieldValues{condition.ID: rawJSON(`"mint"`)})
		coin.Type = coins.Slug
		coinItem, err := env.CommoditiesA.Create(env.CtxA, coin)
		c.Assert(err, qt.IsNil)
		appraised := mustCustomFieldItem(c, env, "Vase", models.CustomFieldValues{appraisalDate.ID: rawJSON(`"2026-01-15"`)})
		_, err = env.CommoditiesA.Trash(env.CtxA, appraised.ID, "", time.Now())
		c.Assert(err, qt.IsNil)

		st, err := svc.Status(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(st.ItemTypes[2].KeepReason, qt.Equals, services.ItemSetKeepInUse)
		c.Check(st.ItemTypes[2].Usage.Live, qt.Equals, 1)
		c.Check(st.Fields[0].KeepReason, qt.Equals, services.ItemSetKeepInUse)
		c.Check(st.Fields[7].KeepReason, qt.Equals, services.ItemSetKeepInUse, qt.Commentf("an item in Trash counts"))
		c.Check(st.Fields[7].Usage.Trashed, qt.Equals, 1)

		removed, err := svc.Remove(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(removed.DeletedItemTypes, qt.DeepEquals, []string{"Art", "Books", "Stamps", "Records", "Watches", "Cards"})
		c.Check(removed.DeletedFields, qt.DeepEquals, []string{
			"Catalog reference", "Edition or variant", "Signed", "Provenance", "Certificate of authenticity", "Appraised value",
		})
		c.Check(removed.Status.ItemTypes[2].State, qt.Equals, services.ItemSetEntryAdded)
		c.Check(removed.Status.ItemTypes[0].State, qt.Equals, services.ItemSetEntryMissing)
		item, err := env.CommoditiesA.Get(env.CtxA, coinItem.ID)
		c.Assert(err, qt.IsNil)
		c.Check(item.Type, qt.Equals, coins.Slug)
		c.Check(string(item.CustomFields[condition.ID]), qt.Equals, `"mint"`)

		again, err := svc.Remove(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(again.DeletedItemTypes, qt.HasLen, 0)
		c.Check(again.DeletedFields, qt.HasLen, 0)

		// Once nothing uses them any more, removing again deletes them.
		update := *item
		update.Type = models.CommodityTypeOther
		update.CustomFields = models.CustomFieldValues{condition.ID: json.RawMessage(`null`)}
		_, err = env.CommoditiesA.Update(env.CtxA, update)
		c.Assert(err, qt.IsNil)
		later, err := svc.Remove(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(later.DeletedItemTypes, qt.DeepEquals, []string{"Coins"})
		c.Check(later.DeletedFields, qt.DeepEquals, []string{"Condition"})

		// Adding after a remove reuses what stayed and creates the rest.
		readded, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(readded.CreatedItemTypes, qt.Equals, len(def.ItemTypes))
		c.Check(readded.CreatedFields, qt.Equals, len(def.Fields)-1, qt.Commentf("the used Appraisal date stayed"))
		c.Check(readded.AddedTemplates, qt.Equals, len(def.ItemTypes))
		c.Check(readded.Status.Complete(), qt.IsTrue)
	})

	t.Run("item set: a renamed type or field is still the set's", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		cards := *res.Status.ItemTypes[6].ItemType
		cards.Name = "Trading cards"
		_, err = env.TypesA.Update(env.CtxA, cards)
		c.Assert(err, qt.IsNil)
		signed := *res.Status.Fields[3].Field
		signed.Name = "Autographed"
		signed.Templates = nil
		_, err = env.FieldsA.Update(env.CtxA, signed)
		c.Assert(err, qt.IsNil)

		again, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(again.CreatedItemTypes, qt.Equals, 0)
		c.Check(again.CreatedFields, qt.Equals, 0)
		c.Check(again.Status.ItemTypes[6].ItemType.Name, qt.Equals, "Trading cards")
		c.Check(again.Status.ItemTypes[6].State, qt.Equals, services.ItemSetEntryAdded)
		c.Check(again.Status.Fields[3].Field.Name, qt.Equals, "Autographed")

		removed, err := svc.Remove(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(removed.DeletedItemTypes, qt.Contains, "Trading cards")
		c.Check(removed.DeletedFields, qt.Contains, "Autographed")
	})

	t.Run("item set: removing keeps the group's last type", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		_, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		for _, b := range models.BuiltinItemTypes {
			builtin := mustTypeBySlug(c, env.TypesA, env.CtxA, b.Slug)
			c.Assert(env.TypesA.Delete(env.CtxA, builtin.ID), qt.IsNil)
		}

		st, err := svc.Status(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(st.ItemTypes[6].KeepReason, qt.Equals, services.ItemSetKeepLastType)

		removed, err := svc.Remove(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(removed.DeletedItemTypes, qt.HasLen, len(def.ItemTypes)-1)
		types, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Assert(types, qt.HasLen, 1)
		c.Check(types[0].Name, qt.Equals, "Cards")
	})
}

// templateTypeIDs returns the sorted item type IDs of templates.
func templateTypeIDs(templates []models.CustomFieldTemplate) []string {
	out := make([]string, 0, len(templates))
	for _, t := range templates {
		out = append(out, t.ItemTypeID)
	}
	slices.Sort(out)
	return out
}
