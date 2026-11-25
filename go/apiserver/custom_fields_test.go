package apiserver_test

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
)

func createCustomFieldAPI(c *qt.C, env *trashAPIEnv, attrs string) jsonapi.CustomFieldResponse {
	c.Helper()
	rr := env.do(http.MethodPost, "/custom-fields", `{"data":{"type":"custom_fields","attributes":`+attrs+`}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var resp jsonapi.CustomFieldResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	return resp
}

func commodityBodyWithCustomFields(name, values string) string {
	return `{"data":{"type":"commodities","attributes":{"name":"` + name + `","short_name":"` + strings.ToLower(name) +
		`","type":"other","count":1,"status":"in_use","draft":true,"original_price_currency":"USD","custom_fields":` + values + `}}}`
}

func TestCustomFields_CRUDAndItemValues(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	plate := createCustomFieldAPI(c, env, `{"name":" License  plate ","type":"text"}`)
	c.Check(plate.Data.Type, qt.Equals, "custom_fields")
	c.Check(plate.Data.Attributes.Name, qt.Equals, "License plate")
	c.Check(plate.Data.Attributes.Type, qt.Equals, "text")
	c.Check(plate.Data.Attributes.Options, qt.HasLen, 0)
	fuel := createCustomFieldAPI(c, env, `{"name":"Fuel","type":"select","options":[{"label":"Petrol"},{"label":"Diesel"}]}`)
	c.Assert(fuel.Data.Attributes.Options, qt.HasLen, 2)
	petrol, diesel := fuel.Data.Attributes.Options[0].ID, fuel.Data.Attributes.Options[1].ID

	rr := env.do(http.MethodPost, "/custom-fields", `{"data":{"type":"custom_fields","attributes":{"name":"LICENSE PLATE","type":"number"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "custom_field.name_taken")
	rr = env.do(http.MethodPost, "/custom-fields", `{"data":{"type":"custom_fields","attributes":{"name":"Size","type":"colour"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodPost, "/custom-fields", `{"data":{"type":"custom_fields","attributes":{"name":"Size","type":"select"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "options_required")

	// An item write stores the values; a refused one is a 422 per field.
	rr = env.do(http.MethodPost, "/commodities", commodityBodyWithCustomFields("Car",
		`{"`+plate.Data.ID+`":" AB 123 ","`+fuel.Data.ID+`":"`+diesel+`"}`))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var car jsonapi.CommodityResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &car), qt.IsNil)
	c.Check(string(car.Data.Attributes.CustomFields[plate.Data.ID]), qt.Equals, `"AB 123"`)

	rr = env.do(http.MethodPost, "/commodities", commodityBodyWithCustomFields("Bad", `{"`+fuel.Data.ID+`":"nope","unknown":"x"}`))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "custom_field.invalid_value")
	var errs jsonapi.Errors
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &errs), qt.IsNil)
	c.Assert(errs.Errors, qt.HasLen, 1)
	var detail struct {
		Error struct {
			CustomFields map[string]string `json:"custom_fields"`
		} `json:"error"`
	}
	c.Assert(json.Unmarshal(errs.Errors[0].UserError, &detail), qt.IsNil)
	c.Check(detail.Error.CustomFields, qt.HasLen, 2)
	c.Check(detail.Error.CustomFields["unknown"], qt.Not(qt.Equals), "")

	// The list carries usage per field and per option.
	rr = env.do(http.MethodGet, "/custom-fields", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var list jsonapi.CustomFieldsResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	c.Check(list.Meta.CustomFields, qt.Equals, 2)
	c.Check(list.Meta.Limit, qt.Equals, models.MaxCustomFieldsPerGroup)
	c.Assert(list.Data, qt.HasLen, 2)
	c.Check(list.Data[0].Attributes.Name, qt.Equals, "Fuel")
	c.Check(list.Data[0].Attributes.ItemCount, qt.Equals, 1)
	c.Check(list.Data[0].Attributes.Options[1].ItemCount, qt.Equals, 1)
	c.Check(rr.Body.String(), qt.Not(qt.Contains), "name_key")

	// A rename keeps the values; the type cannot change.
	rr = env.do(http.MethodPatch, "/custom-fields/"+plate.Data.ID,
		`{"data":{"id":"`+plate.Data.ID+`","type":"custom_fields","attributes":{"name":"Registration","type":"text"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodPatch, "/custom-fields/"+plate.Data.ID,
		`{"data":{"id":"`+plate.Data.ID+`","type":"custom_fields","attributes":{"type":"number"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "custom_field_type_immutable")
	rr = env.do(http.MethodGet, "/commodities/"+car.Data.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var got jsonapi.CommodityResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &got), qt.IsNil)
	c.Check(string(got.Data.Attributes.CustomFields[plate.Data.ID]), qt.Equals, `"AB 123"`)

	// Dropping an option clears the values that hold it.
	rr = env.do(http.MethodPatch, "/custom-fields/"+fuel.Data.ID,
		`{"data":{"id":"`+fuel.Data.ID+`","type":"custom_fields","attributes":{"options":[{"id":"`+petrol+`","label":"Gasoline"},{"label":"Electric"}]}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var relabeled jsonapi.CustomFieldResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &relabeled), qt.IsNil)
	c.Assert(relabeled.Data.Attributes.Options, qt.HasLen, 2)
	c.Check(relabeled.Data.Attributes.Options[0].Label, qt.Equals, "Gasoline")
	c.Check(relabeled.Data.Attributes.Options[1].ID, qt.Not(qt.Equals), "")
	c.Check(relabeled.Data.Attributes.ItemCount, qt.Equals, 0)

	// The list filters and sorts by a field through the query contract.
	rr = env.do(http.MethodGet, "/commodities?cf="+url.QueryEscape(plate.Data.ID+":contains:ab"), "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var filtered jsonapi.CommoditiesResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &filtered), qt.IsNil)
	c.Assert(filtered.Data, qt.HasLen, 1)
	c.Check(filtered.Data[0].ID, qt.Equals, car.Data.ID)
	rr = env.do(http.MethodGet, "/commodities?cf="+url.QueryEscape(plate.Data.ID+":gte:1"), "")
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("a text field takes no range; body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "query.invalid_parameter")
	rr = env.do(http.MethodGet, "/search?type=commodities&cf="+url.QueryEscape(plate.Data.ID+":gte:1"), "")
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest, qt.Commentf("body=%s", rr.Body.String()))

	// A field in use is deleted only with its values.
	rr = env.do(http.MethodDelete, "/custom-fields/"+plate.Data.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "custom_field.in_use")
	c.Check(rr.Body.String(), qt.Contains, `"item_count":1`)
	rr = env.do(http.MethodDelete, "/custom-fields/"+plate.Data.ID+"?delete_values=maybe", "")
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodDelete, "/custom-fields/"+plate.Data.ID+"?delete_values=true", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var deleted jsonapi.CustomFieldDeleteResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &deleted), qt.IsNil)
	c.Check(deleted.Meta.DeletedValues, qt.Equals, 1)
	rr = env.do(http.MethodGet, "/custom-fields/"+plate.Data.ID, "")
	c.Check(rr.Code, qt.Equals, http.StatusNotFound)

	// A saved filter on the deleted field now lists nothing.
	rr = env.do(http.MethodGet, "/commodities?cf="+url.QueryEscape(plate.Data.ID+":set"), "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &filtered), qt.IsNil)
	c.Check(filtered.Data, qt.HasLen, 0)
}

func TestCustomFields_LinkedItemsOnTheItemPage(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	link := createCustomFieldAPI(c, env, `{"name":"Charger","type":"item_link"}`)
	rr := env.do(http.MethodPost, "/commodities", commodityBodyWithCustomFields("Charger", `{}`))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var charger jsonapi.CommodityResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &charger), qt.IsNil)
	rr = env.do(http.MethodPost, "/commodities", commodityBodyWithCustomFields("Phone", `{"`+link.Data.ID+`":"`+charger.Data.ID+`"}`))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var phone jsonapi.CommodityResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &phone), qt.IsNil)

	page := func() jsonapi.CommodityResponse {
		c.Helper()
		rr := env.do(http.MethodGet, "/commodities/"+phone.Data.ID, "")
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
		var resp jsonapi.CommodityResponse
		c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
		return resp
	}
	got := page()
	c.Assert(got.Meta, qt.IsNotNil)
	c.Check(got.Meta.LinkedItems, qt.DeepEquals, map[string]jsonapi.LinkedItemSummary{charger.Data.ID: {Name: "Charger"}})

	rr = env.do(http.MethodDelete, "/commodities/"+charger.Data.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	got = page()
	c.Assert(got.Meta, qt.IsNotNil)
	c.Check(got.Meta.LinkedItems, qt.DeepEquals, map[string]jsonapi.LinkedItemSummary{charger.Data.ID: {Name: "Charger", Trashed: true}})
}

func TestCustomFields_ViewerCannotWrite(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	env.setRole(models.GroupRoleViewer)

	rr := env.do(http.MethodGet, "/custom-fields", "")
	c.Check(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodPost, "/custom-fields", `{"data":{"type":"custom_fields","attributes":{"name":"Size","type":"text"}}}`)
	c.Check(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body=%s", rr.Body.String()))
}
