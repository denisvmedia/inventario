package apiserver_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
)

func itemTypeBySlug(c *qt.C, list jsonapi.ItemTypesResponse, slug string) *jsonapi.ItemTypeResponseData {
	c.Helper()
	for _, d := range list.Data {
		if d.Attributes.Slug == slug {
			return d
		}
	}
	c.Fatalf("no item type with slug %q", slug)
	return nil
}

func listItemTypes(c *qt.C, env *trashAPIEnv) jsonapi.ItemTypesResponse {
	c.Helper()
	rr := env.do(http.MethodGet, "/item-types", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var list jsonapi.ItemTypesResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &list), qt.IsNil)
	return list
}

func TestItemTypes_ListSeedsBuiltinsWithCounts(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	list := listItemTypes(c, env)
	c.Assert(list.Data, qt.HasLen, len(models.BuiltinItemTypes))
	c.Check(list.Meta.ItemTypes, qt.Equals, len(models.BuiltinItemTypes))
	for i, b := range models.BuiltinItemTypes {
		d := list.Data[i]
		c.Check(d.Type, qt.Equals, "item_types")
		c.Check(d.Attributes.Slug, qt.Equals, string(b.Slug))
		c.Check(d.Attributes.Name, qt.Equals, "", qt.Commentf("a built-in type is named by the client"))
		c.Check(d.Attributes.Icon, qt.Equals, b.Icon)
		c.Check(d.Attributes.Builtin, qt.IsTrue)
	}
	// The fixture holds one furniture item and one electronics item.
	c.Check(itemTypeBySlug(c, list, "furniture").Attributes.ItemCount, qt.Equals, 1)
	c.Check(itemTypeBySlug(c, list, "electronics").Attributes.ItemCount, qt.Equals, 1)
	c.Check(itemTypeBySlug(c, list, "clothes").Attributes.ItemCount, qt.Equals, 0)

	// The name key is not part of the API.
	rr := env.do(http.MethodGet, "/item-types", "")
	c.Check(rr.Body.String(), qt.Not(qt.Contains), "name_key")
}

func TestItemTypes_CreateRenameAndItemWrites(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	rr := env.do(http.MethodPost, "/item-types", `{"data":{"type":"item_types","attributes":{"name":"  Vehicle ","icon":"car"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var created jsonapi.ItemTypeResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &created), qt.IsNil)
	c.Check(created.Data.Attributes.Slug, qt.Equals, "vehicle")
	c.Check(created.Data.Attributes.Name, qt.Equals, "Vehicle")
	c.Check(created.Data.Attributes.Icon, qt.Equals, "car")
	c.Check(created.Data.Attributes.Builtin, qt.IsFalse)
	vehicleID := created.Data.ID

	rr = env.do(http.MethodPost, "/item-types", `{"data":{"type":"item_types","attributes":{"name":"VEHICLE"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "item_type.name_taken")
	rr = env.do(http.MethodPost, "/item-types", `{"data":{"type":"item_types","attributes":{"name":"furniture"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("a built-in type holds its default name; body=%s", rr.Body.String()))

	// An item can take the new type through the API.
	rr = env.do(http.MethodPost, "/commodities", commodityBodyWithType("Car", "vehicle"))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body=%s", rr.Body.String()))
	var car jsonapi.CommodityResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &car), qt.IsNil)

	// A type the group does not have is refused.
	rr = env.do(http.MethodPost, "/commodities", commodityBodyWithType("Rocket", "spaceship"))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	assertUnknownItemTypeError(c, rr.Body.Bytes())

	// Renaming keeps the slug, so the item and a type filter keep working.
	rr = env.do(http.MethodPatch, "/item-types/"+vehicleID, `{"data":{"id":"`+vehicleID+`","type":"item_types","attributes":{"name":"Car"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var renamed jsonapi.ItemTypeResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &renamed), qt.IsNil)
	c.Check(renamed.Data.Attributes.Name, qt.Equals, "Car")
	c.Check(renamed.Data.Attributes.Slug, qt.Equals, "vehicle")
	c.Check(renamed.Data.Attributes.Icon, qt.Equals, "car", qt.Commentf("a partial patch keeps the icon"))
	c.Check(renamed.Data.Attributes.ItemCount, qt.Equals, 1)

	rr = env.do(http.MethodGet, "/commodities?include_inactive=true&type=vehicle", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var filtered jsonapi.CommoditiesResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &filtered), qt.IsNil)
	c.Assert(filtered.Data, qt.HasLen, 1)
	c.Check(filtered.Data[0].ID, qt.Equals, car.Data.ID)

	// A built-in type can be renamed and given back its default name.
	list := listItemTypes(c, env)
	other := itemTypeBySlug(c, list, "other")
	rr = env.do(http.MethodPatch, "/item-types/"+other.ID, `{"data":{"id":"`+other.ID+`","type":"item_types","attributes":{"name":"Misc"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodPatch, "/item-types/"+other.ID, `{"data":{"id":"`+other.ID+`","type":"item_types","attributes":{"name":""}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var reset jsonapi.ItemTypeResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &reset), qt.IsNil)
	c.Check(reset.Data.Attributes.Name, qt.Equals, "")
	c.Check(reset.Data.Attributes.Builtin, qt.IsTrue)
}

func TestItemTypes_GroupKeepsAtMostTheLimit(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)

	for i := len(listItemTypes(c, env).Data); i < models.MaxItemTypesPerGroup; i++ {
		rr := env.do(http.MethodPost, "/item-types", fmt.Sprintf(`{"data":{"type":"item_types","attributes":{"name":"Kind %d"}}}`, i))
		c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("type %d: body=%s", i, rr.Body.String()))
	}
	rr := env.do(http.MethodPost, "/item-types", `{"data":{"type":"item_types","attributes":{"name":"One too many"}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	var got struct {
		Errors []struct {
			Code string         `json:"code"`
			Meta map[string]int `json:"meta"`
		} `json:"errors"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &got), qt.IsNil)
	c.Assert(got.Errors, qt.HasLen, 1)
	c.Check(got.Errors[0].Code, qt.Equals, "item_type.limit")
	c.Check(got.Errors[0].Meta["limit"], qt.Equals, models.MaxItemTypesPerGroup)
	c.Check(listItemTypes(c, env).Data, qt.HasLen, models.MaxItemTypesPerGroup)
}

func TestItemTypes_RejectsInvalidInput(t *testing.T) {
	env := newTrashAPIEnv(qt.New(t))
	for _, tc := range []struct {
		name, body string
	}{
		{name: "no name", body: `{"data":{"type":"item_types","attributes":{"name":"   "}}}`},
		{name: "a name over the limit", body: `{"data":{"type":"item_types","attributes":{"name":"` + strings.Repeat("x", models.MaxItemTypeNameRunes+1) + `"}}}`},
		{name: "an icon that is not kebab case", body: `{"data":{"type":"item_types","attributes":{"name":"Boat","icon":"Sail Boat"}}}`},
		{name: "the wrong resource type", body: `{"data":{"type":"tags","attributes":{"name":"Boat"}}}`},
		{name: "an ID on create", body: `{"data":{"id":"x","type":"item_types","attributes":{"name":"Boat"}}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			env.c = c
			rr := env.do(http.MethodPost, "/item-types", tc.body)
			c.Check(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
		})
	}
}

func TestItemTypes_DeleteBlocksOrReassigns(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regSet := getRegistrySetFromParams(env.params, env.user)
	ctx := context.Background()

	list := listItemTypes(c, env)
	furniture := itemTypeBySlug(c, list, "furniture")
	equipment := itemTypeBySlug(c, list, "equipment")
	clothes := itemTypeBySlug(c, list, "clothes")

	// Put a second furniture item in Trash: it counts and moves too.
	area := must.Must(regSet.AreaRegistry.List(ctx))[0]
	stool := must.Must(regSet.CommodityRegistry.Create(ctx, models.Commodity{
		Name: "Stool", ShortName: "stool", AreaID: new(area.ID), Status: models.CommodityStatusInUse,
		Type: models.CommodityTypeFurniture, Count: 1,
	}))
	_, err := regSet.CommodityRegistry.Trash(ctx, stool.ID, "test", time.Now())
	c.Assert(err, qt.IsNil)

	rr := env.do(http.MethodDelete, "/item-types/"+furniture.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "item_type.in_use")
	var conflict struct {
		Errors []struct {
			Meta map[string]int `json:"meta"`
		} `json:"errors"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &conflict), qt.IsNil)
	c.Assert(conflict.Errors, qt.HasLen, 1)
	c.Check(conflict.Errors[0].Meta, qt.DeepEquals, map[string]int{"item_count": 1, "trashed_item_count": 1, "draft_count": 0})

	rr = env.do(http.MethodDelete, "/item-types/"+furniture.ID+"?reassign_to="+furniture.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodDelete, "/item-types/"+furniture.ID+"?reassign_to=missing", "")
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))

	rr = env.do(http.MethodDelete, "/item-types/"+furniture.ID+"?reassign_to="+equipment.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var deleted jsonapi.ItemTypeDeleteResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &deleted), qt.IsNil)
	c.Check(deleted.Meta.ReassignedItems, qt.Equals, 2)

	rr = env.do(http.MethodGet, "/item-types/"+furniture.ID, "")
	c.Check(rr.Code, qt.Equals, http.StatusNotFound)
	list = listItemTypes(c, env)
	c.Check(itemTypeBySlug(c, list, "equipment").Attributes.ItemCount, qt.Equals, 1)
	c.Check(itemTypeBySlug(c, list, "equipment").Attributes.TrashedItemCount, qt.Equals, 1)

	// An unused type goes without a target.
	rr = env.do(http.MethodDelete, "/item-types/"+clothes.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	// The last type stays.
	list = listItemTypes(c, env)
	keep := list.Data[0]
	for _, d := range list.Data[1:] {
		rr = env.do(http.MethodDelete, "/item-types/"+d.ID+"?reassign_to="+keep.ID, "")
		c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	}
	rr = env.do(http.MethodDelete, "/item-types/"+keep.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "item_type.last")
}

func TestItemTypes_DeleteCountsAndMovesDrafts(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	regSet := getRegistrySetFromParams(env.params, env.user)
	ctx := context.Background()

	list := listItemTypes(c, env)
	clothes := itemTypeBySlug(c, list, "clothes")
	equipment := itemTypeBySlug(c, list, "equipment")
	draft, created, err := regSet.CommodityDraftRegistry.CreateOrGet(ctx, models.CommodityDraft{
		ClientKey: "draft-clothes",
		Values:    models.CommodityDraftValues{"type": "clothes", "name": "Coat"},
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsTrue)
	c.Check(itemTypeBySlug(c, listItemTypes(c, env), "clothes").Attributes.DraftCount, qt.Equals, 1)

	// No item has the type, and the draft alone keeps it from going
	// without a target.
	rr := env.do(http.MethodDelete, "/item-types/"+clothes.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusConflict, qt.Commentf("body=%s", rr.Body.String()))
	var conflict struct {
		Errors []struct {
			Meta map[string]int `json:"meta"`
		} `json:"errors"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &conflict), qt.IsNil)
	c.Assert(conflict.Errors, qt.HasLen, 1)
	c.Check(conflict.Errors[0].Meta, qt.DeepEquals, map[string]int{"item_count": 0, "trashed_item_count": 0, "draft_count": 1})

	rr = env.do(http.MethodDelete, "/item-types/"+clothes.ID+"?reassign_to="+equipment.ID, "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	var deleted jsonapi.ItemTypeDeleteResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &deleted), qt.IsNil)
	c.Check(deleted.Meta.ReassignedItems, qt.Equals, 0)

	got, err := regSet.CommodityDraftRegistry.Get(ctx, draft.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.Values.DraftItemType(), qt.Equals, models.CommodityTypeEquipment)
	c.Check(got.Values["name"], qt.Equals, "Coat")
	c.Check(got.Revision, qt.Equals, draft.Revision+1)
	c.Check(itemTypeBySlug(c, listItemTypes(c, env), "equipment").Attributes.DraftCount, qt.Equals, 1)
}

func TestItemTypes_ViewerCanReadButNotWrite(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	list := listItemTypes(c, env)
	other := itemTypeBySlug(c, list, "other")

	env.setRole(models.GroupRoleViewer)
	c.Check(env.do(http.MethodGet, "/item-types", "").Code, qt.Equals, http.StatusOK)
	c.Check(env.do(http.MethodGet, "/item-types/"+other.ID, "").Code, qt.Equals, http.StatusOK)
	c.Check(env.do(http.MethodPost, "/item-types", `{"data":{"type":"item_types","attributes":{"name":"Boat"}}}`).Code, qt.Equals, http.StatusForbidden)
	c.Check(env.do(http.MethodPatch, "/item-types/"+other.ID, `{"data":{"id":"`+other.ID+`","type":"item_types","attributes":{"name":"Misc"}}}`).Code, qt.Equals, http.StatusForbidden)
	c.Check(env.do(http.MethodDelete, "/item-types/"+other.ID, "").Code, qt.Equals, http.StatusForbidden)

	env.setRole(models.GroupRoleUser)
	c.Check(env.do(http.MethodPost, "/item-types", `{"data":{"type":"item_types","attributes":{"name":"Boat"}}}`).Code, qt.Equals, http.StatusCreated)
}

// assertUnknownItemTypeError checks the 422 an item write gets for a type
// the group lacks: the item_type.unknown code, and the field error under
// data.attributes.type with its validation code, where the frontend's
// extractFieldErrors and extractFieldErrorCodes read it.
func assertUnknownItemTypeError(c *qt.C, body []byte) {
	c.Helper()
	var got struct {
		Errors []struct {
			Code  string `json:"code"`
			Error struct {
				Type  string `json:"type"`
				Error struct {
					Data struct {
						Attributes map[string]string `json:"attributes"`
					} `json:"data"`
				} `json:"error"`
				ErrorCodes struct {
					Data struct {
						Attributes map[string]struct {
							Code string `json:"code"`
						} `json:"attributes"`
					} `json:"data"`
				} `json:"errorCodes"`
			} `json:"error"`
		} `json:"errors"`
	}
	c.Assert(json.Unmarshal(body, &got), qt.IsNil, qt.Commentf("body=%s", body))
	c.Assert(got.Errors, qt.HasLen, 1, qt.Commentf("body=%s", body))
	e := got.Errors[0]
	c.Check(e.Code, qt.Equals, "item_type.unknown")
	c.Check(e.Error.Type, qt.Equals, "validation.Errors")
	c.Check(e.Error.Error.Data.Attributes["type"], qt.Equals, "no item type with this slug in the group")
	c.Check(e.Error.ErrorCodes.Data.Attributes["type"].Code, qt.Equals, "item_type_unknown")
}

func commodityBodyWithType(name, slug string) string {
	return `{"data":{"type":"commodities","attributes":{"name":"` + name + `","short_name":"` + strings.ToLower(name) +
		`","type":"` + slug + `","count":1,"status":"in_use","draft":true,"original_price_currency":"USD"}}}`
}
