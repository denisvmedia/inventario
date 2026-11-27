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
	newService := newCollectionSetService

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

		removed := removeConfirmed(c, env, svc)
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

		removed := removeConfirmed(c, env, svc)
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

		again := removeConfirmed(c, env, svc)
		c.Check(again.DeletedItemTypes, qt.HasLen, 0)
		c.Check(again.DeletedFields, qt.HasLen, 0)

		// Once nothing uses them any more, removing again deletes them.
		update := *item
		update.Type = models.CommodityTypeOther
		update.CustomFields = models.CustomFieldValues{condition.ID: json.RawMessage(`null`)}
		_, err = env.CommoditiesA.Update(env.CtxA, update)
		c.Assert(err, qt.IsNil)
		later := removeConfirmed(c, env, svc)
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

		removed := removeConfirmed(c, env, svc)
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

		removed := removeConfirmed(c, env, svc)
		c.Check(removed.DeletedItemTypes, qt.HasLen, len(def.ItemTypes)-1)
		types, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Assert(types, qt.HasLen, 1)
		c.Check(types[0].Name, qt.Equals, "Cards")
	})

	runItemSetChangeContract(t, newEnv)
	runItemSetCompletenessContract(t, newEnv)
}

// newCollectionSetService returns the Collection set service of env's
// group A.
func newCollectionSetService(env CustomFieldEnv) *services.ItemSetService {
	return services.NewItemSetService(models.CollectionSet, env.TypesA, env.FieldsA)
}

// removeConfirmed removes from env's group A what a confirmation shown now
// would list.
func removeConfirmed(c *qt.C, env CustomFieldEnv, svc *services.ItemSetService) *services.ItemSetRemoveResult {
	c.Helper()
	st, err := svc.Status(env.CtxA)
	c.Assert(err, qt.IsNil)
	res, err := svc.Remove(env.CtxA, st.Removal())
	c.Assert(err, qt.IsNil)
	return res
}

// runItemSetChangeContract checks the set against changes made between a
// status and the next step: a row coming into use after a confirmation,
// and a set row renamed to another entry's name.
func runItemSetChangeContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Helper()
	def := models.CollectionSet
	newService := newCollectionSetService

	t.Run("item set: removing deletes only the rows it is given and keeps one that came into use", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		art := res.Status.ItemTypes[0].ItemType
		coins := res.Status.ItemTypes[2].ItemType
		provenance := res.Status.Fields[4].Field
		sel := services.ItemSetRemoval{ItemTypeIDs: []string{art.ID, coins.ID}, FieldIDs: []string{provenance.ID}}

		// The confirmation listed Coins; an item gets the type before the
		// remove runs.
		coin := customFieldItem("Thaler", env.AreaA, nil)
		coin.Type = coins.Slug
		_, err = env.CommoditiesA.Create(env.CtxA, coin)
		c.Assert(err, qt.IsNil)

		removed, err := svc.Remove(env.CtxA, sel)
		c.Assert(err, qt.IsNil)
		c.Check(removed.DeletedItemTypes, qt.DeepEquals, []string{"Art"})
		c.Check(removed.DeletedFields, qt.DeepEquals, []string{"Provenance"})
		c.Check(removed.KeptItemTypes, qt.DeepEquals, []string{"Coins"})
		c.Check(removed.KeptFields, qt.HasLen, 0)
		types, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(types, qt.HasLen, len(models.BuiltinItemTypes)+len(def.ItemTypes)-1)
		fields, err := env.FieldsA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(fields, qt.HasLen, len(def.Fields)-1)

		// A built-in type's ID is not the set's and is never deleted.
		other := mustTypeBySlug(c, env.TypesA, env.CtxA, models.CommodityTypeOther)
		none, err := svc.Remove(env.CtxA, services.ItemSetRemoval{ItemTypeIDs: []string{other.ID}})
		c.Assert(err, qt.IsNil)
		c.Check(none.DeletedItemTypes, qt.HasLen, 0)
		c.Check(none.KeptItemTypes, qt.HasLen, 0)
		_, err = env.TypesA.Get(env.CtxA, other.ID)
		c.Check(err, qt.IsNil)
	})

	t.Run("item set: removing keeps a type and a field that only open drafts use", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		coins := res.Status.ItemTypes[2].ItemType
		stamps := res.Status.ItemTypes[3].ItemType
		c.Assert(stamps.Name, qt.Equals, "Stamps")
		condition := res.Status.Fields[0].Field
		draft := mustDraft(c, env.DraftsA, env.CtxA, "set-coin", models.CommodityDraftValues{
			"type":          string(coins.Slug),
			"custom_fields": map[string]any{condition.ID: map[string]any{"value": "mint", "currency": ""}},
		})

		st, err := svc.Status(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(st.ItemTypes[2].KeepReason, qt.Equals, services.ItemSetKeepInUse)
		c.Check(st.ItemTypes[2].Usage.Drafts, qt.Equals, 1)
		c.Check(st.Fields[0].KeepReason, qt.Equals, services.ItemSetKeepInUse)
		c.Check(st.Fields[0].Usage.Drafts, qt.Equals, 1)
		sel := st.Removal()
		c.Check(sel.ItemTypeIDs, qt.Not(qt.Contains), coins.ID)
		c.Check(sel.FieldIDs, qt.Not(qt.Contains), condition.ID)

		// The confirmation listed Stamps; a draft gets the type before the
		// remove runs.
		mustDraft(c, env.DraftsA, env.CtxA, "set-stamp", models.CommodityDraftValues{"type": string(stamps.Slug)})
		removed, err := svc.Remove(env.CtxA, sel)
		c.Assert(err, qt.IsNil)
		c.Check(removed.KeptItemTypes, qt.DeepEquals, []string{"Stamps"})
		c.Check(removed.DeletedItemTypes, qt.Not(qt.Contains), "Coins")
		c.Check(removed.DeletedItemTypes, qt.Not(qt.Contains), "Stamps")
		c.Check(removed.DeletedFields, qt.Not(qt.Contains), "Condition")
		_, err = env.TypesA.Get(env.CtxA, coins.ID)
		c.Check(err, qt.IsNil)
		_, err = env.FieldsA.Get(env.CtxA, condition.ID)
		c.Check(err, qt.IsNil)
		got, err := env.DraftsA.Get(env.CtxA, draft.ID)
		c.Assert(err, qt.IsNil)
		c.Check(got.Revision, qt.Equals, draft.Revision, qt.Commentf("the draft is left as it was"))
	})

	t.Run("item set: a set row renamed to another entry's name takes that entry", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		// Stamps goes, Coins is renamed "Stamps"; Condition goes, Signed is
		// renamed "Condition"; Provenance goes, so adding has a field to
		// create on the set's types.
		c.Assert(env.TypesA.Delete(env.CtxA, res.Status.ItemTypes[3].ItemType.ID), qt.IsNil)
		coins := *res.Status.ItemTypes[2].ItemType
		coins.Name = "stamps"
		_, err = env.TypesA.Update(env.CtxA, coins)
		c.Assert(err, qt.IsNil)
		_, err = env.FieldsA.DeleteWithValues(env.CtxA, res.Status.Fields[0].Field.ID, 0)
		c.Assert(err, qt.IsNil)
		signed := *res.Status.Fields[3].Field
		signed.Name = "Condition"
		signed.Templates = nil
		_, err = env.FieldsA.Update(env.CtxA, signed)
		c.Assert(err, qt.IsNil)
		_, err = env.FieldsA.DeleteWithValues(env.CtxA, res.Status.Fields[4].Field.ID, 0)
		c.Assert(err, qt.IsNil)

		st, err := svc.Status(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(st.ItemTypes[3].State, qt.Equals, services.ItemSetEntryTaken)
		c.Check(st.ItemTypes[3].TakenBy, qt.Equals, "coins")
		c.Check(st.ItemTypes[3].ItemType, qt.IsNil)
		c.Check(st.ItemTypes[2].ItemType.ID, qt.Equals, coins.ID)
		c.Check(st.Fields[0].State, qt.Equals, services.ItemSetEntryTaken)
		c.Check(st.Fields[0].TakenBy, qt.Equals, "signed")
		c.Check(st.Fields[4].State, qt.Equals, services.ItemSetEntryMissing)
		c.Check(st.Complete(), qt.IsFalse)

		again, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(again.CreatedItemTypes, qt.Equals, 0)
		c.Check(again.CreatedFields, qt.Equals, 1, qt.Commentf("Provenance"))
		c.Check(again.Status.Complete(), qt.IsTrue)
		c.Check(again.Status.ItemTypes[3].State, qt.Equals, services.ItemSetEntryTaken)
		provenance := again.Status.Fields[4].Field
		c.Check(provenance.Templates, qt.HasLen, len(def.ItemTypes)-1, qt.Commentf("each type once"))
		c.Check(templateTypeIDs(again.Status.Fields[3].Field.Templates), qt.DeepEquals, templateTypeIDs(provenance.Templates),
			qt.Commentf("the renamed Signed is on each of the set's types once"))

		types, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(types, qt.HasLen, len(models.BuiltinItemTypes)+len(def.ItemTypes)-1)

		// The taken entry has no row of its own, so removing deletes the
		// renamed row once.
		removed := removeConfirmed(c, env, svc)
		c.Check(removed.DeletedItemTypes, qt.HasLen, len(def.ItemTypes)-1)
		c.Check(removed.DeletedItemTypes, qt.Contains, "stamps")
	})
}

// runItemSetCompletenessContract checks what makes the set complete and
// which type a removal keeps as the group's last.
func runItemSetCompletenessContract(t *testing.T, newEnv func(t *testing.T) CustomFieldEnv) {
	t.Helper()
	def := models.CollectionSet
	newService := newCollectionSetService

	t.Run("item set: the group's own types and fields are complete once the set's fields are on its types", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)
		for _, e := range def.ItemTypes {
			_, err := env.TypesA.Create(env.CtxA, models.ItemType{Name: e.Name, Icon: e.Icon})
			c.Assert(err, qt.IsNil)
		}
		for _, e := range def.Fields {
			_, err := env.FieldsA.Create(env.CtxA, models.CustomFieldDefinition{
				Name: e.Name, Type: e.Type, Options: slices.Clone(models.ValuerSlice[models.CustomFieldOption](e.Options)),
			})
			c.Assert(err, qt.IsNil)
		}

		st, err := svc.Status(env.CtxA)
		c.Assert(err, qt.IsNil)
		for _, fs := range st.Fields {
			c.Check(fs.State, qt.Equals, services.ItemSetEntryOwn, qt.Commentf("field %s", fs.Entry.Key))
			c.Check(fs.MissingTemplates, qt.Equals, len(def.ItemTypes), qt.Commentf("field %s", fs.Entry.Key))
		}
		c.Check(st.Complete(), qt.IsFalse, qt.Commentf("every row is there, but no field is offered on the types"))

		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(res.CreatedItemTypes, qt.Equals, 0)
		c.Check(res.CreatedFields, qt.Equals, 0)
		c.Check(res.AddedTemplates, qt.Equals, len(def.Fields)*len(def.ItemTypes))
		c.Check(res.Status.Complete(), qt.IsTrue)

		// A member takes a field off one of the set's types; adding puts it
		// back.
		signed := *res.Status.Fields[3].Field
		signed.Templates = signed.Templates[1:]
		_, err = env.FieldsA.Update(env.CtxA, signed)
		c.Assert(err, qt.IsNil)
		st, err = svc.Status(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(st.Fields[3].MissingTemplates, qt.Equals, 1)
		c.Check(st.Complete(), qt.IsFalse)
		again, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(again.AddedTemplates, qt.Equals, 1)
		c.Check(again.Status.Complete(), qt.IsTrue)
	})

	t.Run("item set: a removal keeps a last type only when it would leave the group none", func(t *testing.T) {
		c := qt.New(t)
		env := newEnv(t)
		svc := newService(env)
		res, err := svc.Add(env.CtxA)
		c.Assert(err, qt.IsNil)
		for _, b := range models.BuiltinItemTypes {
			builtin := mustTypeBySlug(c, env.TypesA, env.CtxA, b.Slug)
			c.Assert(env.TypesA.Delete(env.CtxA, builtin.ID), qt.IsNil)
		}

		// The status keeps Cards as the last type, yet removing Cards alone
		// leaves the six other types of the set.
		cards := res.Status.ItemTypes[6].ItemType
		only, err := svc.Remove(env.CtxA, services.ItemSetRemoval{ItemTypeIDs: []string{cards.ID}})
		c.Assert(err, qt.IsNil)
		c.Check(only.DeletedItemTypes, qt.DeepEquals, []string{"Cards"})
		c.Check(only.KeptItemTypes, qt.HasLen, 0)

		// Listing every type left keeps one of them.
		var left []string
		for _, ts := range only.Status.ItemTypes {
			if ts.ItemType != nil {
				left = append(left, ts.ItemType.ID)
			}
		}
		c.Assert(left, qt.HasLen, len(def.ItemTypes)-1)
		rest, err := svc.Remove(env.CtxA, services.ItemSetRemoval{ItemTypeIDs: left})
		c.Assert(err, qt.IsNil)
		c.Check(rest.DeletedItemTypes, qt.HasLen, len(def.ItemTypes)-2)
		c.Check(rest.KeptItemTypes, qt.DeepEquals, []string{"Watches"})
		types, err := env.TypesA.List(env.CtxA)
		c.Assert(err, qt.IsNil)
		c.Check(types, qt.HasLen, 1)
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
