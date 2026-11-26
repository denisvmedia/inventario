package commodityquery_test

import (
	"encoding/json"
	"net/url"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/commodityquery"
)

func TestList_CustomFields(t *testing.T) {
	c := qt.New(t)
	ctx, set := newMemorySet(c)

	plate, err := set.CustomFieldRegistry.Create(ctx, models.CustomFieldDefinition{Name: "Plate", Type: models.CustomFieldTypeText})
	c.Assert(err, qt.IsNil)
	km, err := set.CustomFieldRegistry.Create(ctx, models.CustomFieldDefinition{Name: "Km", Type: models.CustomFieldTypeNumber})
	c.Assert(err, qt.IsNil)
	fuel, err := set.CustomFieldRegistry.Create(ctx, models.CustomFieldDefinition{
		Name: "Fuel", Type: models.CustomFieldTypeSelect,
		Options: models.ValuerSlice[models.CustomFieldOption]{{Label: "Petrol"}, {Label: "Diesel"}},
	})
	c.Assert(err, qt.IsNil)
	petrol, diesel := fuel.Options[0].ID, fuel.Options[1].ID
	day, err := set.CustomFieldRegistry.Create(ctx, models.CustomFieldDefinition{Name: "Day", Type: models.CustomFieldTypeDate})
	c.Assert(err, qt.IsNil)

	withValues := func(name string, values map[string]string) {
		c.Helper()
		item := seedItem(c, ctx, set, name)
		item.CustomFields = models.CustomFieldValues{}
		for id, v := range values {
			item.CustomFields[id] = json.RawMessage(v)
		}
		_, err := set.CommodityRegistry.Update(ctx, *item)
		c.Assert(err, qt.IsNil)
	}
	withValues("Car", map[string]string{plate.ID: `"AB-1"`, km.ID: `120000`, fuel.ID: `"` + diesel + `"`})
	withValues("Moped", map[string]string{plate.ID: `"XY-9"`, km.ID: `0`, fuel.ID: `"` + petrol + `"`})
	seedItem(c, ctx, set, "Bike")

	list := func(query string) []string {
		c.Helper()
		values, err := url.ParseQuery(query)
		c.Assert(err, qt.IsNil)
		q, err := commodityquery.Parse(values)
		c.Assert(err, qt.IsNil)
		page, err := commodityquery.List(ctx, set, q, 0, 50)
		c.Assert(err, qt.IsNil)
		return listNames(page)
	}

	c.Check(list("cf="+km.ID+":set"), qt.DeepEquals, []string{"Car", "Moped"})
	c.Check(list("cf="+km.ID+":unset"), qt.DeepEquals, []string{"Bike"})
	c.Check(list("cf="+km.ID+":gte:1"), qt.DeepEquals, []string{"Car"})
	c.Check(list("cf="+fuel.ID+":in:"+petrol+","+diesel), qt.DeepEquals, []string{"Car", "Moped"})
	c.Check(list("cf="+fuel.ID+":eq:"+petrol), qt.DeepEquals, []string{"Moped"})
	c.Check(list("cf="+plate.ID+":contains:ab"), qt.DeepEquals, []string{"Car"})
	c.Check(list("cf=deleted-field:set"), qt.HasLen, 0, qt.Commentf("a deleted field keeps nothing"))
	c.Check(list("sort=-cf."+km.ID), qt.DeepEquals, []string{"Car", "Moped", "Bike"})
	c.Check(list("sort=cf."+km.ID), qt.DeepEquals, []string{"Moped", "Car", "Bike"})
	c.Check(list("sort=cf.deleted-field"), qt.DeepEquals, []string{"Bike", "Car", "Moped"}, qt.Commentf("falls back to the name"))

	values, err := url.ParseQuery("q=diesel")
	c.Assert(err, qt.IsNil)
	q, err := commodityquery.Parse(values)
	c.Assert(err, qt.IsNil)
	page, err := commodityquery.List(ctx, set, q, 0, 50)
	c.Assert(err, qt.IsNil)
	c.Assert(listNames(page), qt.DeepEquals, []string{"Car"})
	c.Check(page.Matches[page.Commodities[0].ID], qt.DeepEquals, []registry.CommoditySearchMatch{
		{Field: registry.CommoditySearchFieldCustomFields, Value: "Diesel", CustomFieldID: fuel.ID},
	})

	for _, refused := range []string{
		"cf=" + km.ID + ":contains:1",
		"cf=" + plate.ID + ":gte:1",
		"cf=" + fuel.ID + ":eq:" + petrol + "," + diesel,
		"cf=" + day.ID + ":eq:0000-01-01",
	} {
		values, err := url.ParseQuery(refused)
		c.Assert(err, qt.IsNil)
		q, err := commodityquery.Parse(values)
		c.Assert(err, qt.IsNil)
		_, err = commodityquery.List(ctx, set, q, 0, 50)
		var qErr *commodityquery.Error
		c.Check(err, qt.ErrorAs, &qErr, qt.Commentf("query %s", refused))
	}
}
