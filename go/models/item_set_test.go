package models_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestItemSetEntry(t *testing.T) {
	tests := []struct {
		entry models.ItemSetEntry
		set   models.ItemSet
		valid bool
	}{
		{entry: models.NewItemSetEntry(models.ItemSetCollection, "coins"), set: models.ItemSetCollection, valid: true},
		{entry: "collection/appraisal_date", set: models.ItemSetCollection, valid: true},
		{entry: "collection/some_later_key", set: models.ItemSetCollection, valid: true},
		{entry: "", set: "", valid: false},
		{entry: "collection", set: models.ItemSetCollection, valid: false},
		{entry: "collection/", set: models.ItemSetCollection, valid: false},
		{entry: "collection/Coins", set: models.ItemSetCollection, valid: false},
		{entry: "collection/coins/x", set: models.ItemSetCollection, valid: false},
		{entry: "household/coins", set: "household", valid: false},
	}
	for _, tt := range tests {
		t.Run(string(tt.entry), func(t *testing.T) {
			c := qt.New(t)
			c.Check(tt.entry.Set(), qt.Equals, tt.set)
			c.Check(tt.entry.IsValid(), qt.Equals, tt.valid)
		})
	}
}

// Every type and field of a set must be one a member could create: a set
// that fails validation would fail halfway through every add.
func TestCollectionSet_IsValid(t *testing.T) {
	c := qt.New(t)
	def, ok := models.ItemSetDefinitionFor(models.ItemSetCollection)
	c.Assert(ok, qt.IsTrue)
	c.Check(def.Set, qt.Equals, models.ItemSetCollection)
	_, ok = models.ItemSetDefinitionFor("household")
	c.Check(ok, qt.IsFalse)

	keys := map[string]bool{}
	for _, e := range def.ItemTypes {
		c.Check(keys[e.Key], qt.IsFalse, qt.Commentf("duplicate key %s", e.Key))
		keys[e.Key] = true
		c.Check(def.Entry(e.Key).IsValid(), qt.IsTrue)
		t := models.ItemType{Name: e.Name, Icon: e.Icon}
		t.Normalize()
		c.Check(t.ValidateFields(context.Background()), qt.IsNil, qt.Commentf("type %s", e.Key))
	}
	for _, e := range def.Fields {
		c.Check(keys[e.Key], qt.IsFalse, qt.Commentf("duplicate key %s", e.Key))
		keys[e.Key] = true
		c.Check(def.Entry(e.Key).IsValid(), qt.IsTrue)
		f := models.CustomFieldDefinition{Name: e.Name, Type: e.Type, Options: e.Options}
		f.Normalize()
		c.Check(f.ValidateFields(context.Background()), qt.IsNil, qt.Commentf("field %s", e.Key))
	}
}
