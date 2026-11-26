package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

func collectionSetRequest(env groupTestEnv, method string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/groups/"+env.group.ID+"/collection-set", nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
}

func decodeItemSet(c *qt.C, w *httptest.ResponseRecorder) *jsonapi.ItemSetResponse {
	c.Helper()
	var resp jsonapi.ItemSetResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &resp), qt.IsNil, qt.Commentf("body: %s", w.Body.String()))
	c.Assert(resp.Data, qt.IsNotNil)
	return &resp
}

// groupRegistries returns the registry set of group, as the group-scoped
// routes see it.
func groupRegistries(env groupTestEnv, group *models.LocationGroup) (*registry.Set, context.Context) {
	ctx := appctx.WithGroup(appctx.WithUser(context.Background(), env.user), group)
	return must.Must(env.factorySet.CreateUserRegistrySet(ctx)), ctx
}

func TestGroupsAPI_CollectionSet_AddIsIdempotent(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))

	w := collectionSetRequest(env, http.MethodGet)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	got := decodeItemSet(c, w)
	c.Check(got.Data.ID, qt.Equals, "collection")
	c.Check(got.Data.Attributes.Complete, qt.IsFalse)
	c.Check(got.Data.Attributes.ItemTypes, qt.HasLen, 7)
	c.Check(got.Data.Attributes.CustomFields, qt.HasLen, 8)
	c.Check(got.Data.Attributes.ItemTypes[0].State, qt.Equals, "missing")
	c.Check(got.Meta, qt.IsNil)

	w = collectionSetRequest(env, http.MethodPost)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	got = decodeItemSet(c, w)
	c.Check(got.Data.Attributes.Complete, qt.IsTrue)
	c.Check(got.Meta.CreatedItemTypes, qt.Equals, 7)
	c.Check(got.Meta.CreatedCustomFields, qt.Equals, 8)
	for _, row := range got.Data.Attributes.ItemTypes {
		c.Check(row.State, qt.Equals, "added")
		c.Check(row.Removable, qt.IsTrue)
		c.Check(row.ItemTypeID, qt.Not(qt.Equals), "")
	}
	c.Check(w.Body.String(), checkers.JSONPathEquals("$.data.attributes.custom_fields[0].field_type"), "select")

	w = collectionSetRequest(env, http.MethodPost)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	got = decodeItemSet(c, w)
	c.Check(got.Meta.CreatedItemTypes, qt.Equals, 0)
	c.Check(got.Meta.CreatedCustomFields, qt.Equals, 0)

	set, ctx := groupRegistries(env, env.group)
	types := must.Must(set.ItemTypeRegistry.List(ctx))
	c.Check(types, qt.HasLen, len(models.BuiltinItemTypes)+7)

	c.Check(env.audit.actions(), qt.DeepEquals, []string{"group_collection_set_add", "group_collection_set_add"})
	ev := env.audit.snapshotEvents()[0]
	c.Check(ev.Success, qt.IsTrue)
	c.Check(*ev.EntityID, qt.Equals, env.group.ID)
	c.Check(*ev.EntityType, qt.Equals, "location_group")
}

func TestGroupsAPI_CollectionSet_RemoveKeepsWhatItemsUse(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))

	w := collectionSetRequest(env, http.MethodPost)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	added := decodeItemSet(c, w)
	coinsID := added.Data.Attributes.ItemTypes[2].ItemTypeID
	conditionID := added.Data.Attributes.CustomFields[0].CustomFieldID

	set, ctx := groupRegistries(env, env.group)
	coins := must.Must(set.ItemTypeRegistry.Get(ctx, coinsID))
	location := must.Must(set.LocationRegistry.Create(ctx, models.Location{Name: "Home"}))
	area := must.Must(set.AreaRegistry.Create(ctx, models.Area{Name: "Shelf", LocationID: location.ID}))
	must.Must(set.CommodityRegistry.Create(ctx, models.Commodity{
		Name:         "Thaler",
		ShortName:    "Thaler",
		Type:         coins.Slug,
		AreaID:       new(area.ID),
		Count:        1,
		Status:       models.CommodityStatusInUse,
		CustomFields: models.CustomFieldValues{conditionID: json.RawMessage(`"mint"`)},
	}))

	w = collectionSetRequest(env, http.MethodDelete)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	got := decodeItemSet(c, w)
	c.Check(got.Meta.DeletedItemTypes, qt.DeepEquals, []string{"Art", "Books", "Stamps", "Records", "Watches", "Cards"})
	c.Check(got.Meta.DeletedCustomFields, qt.HasLen, 7)
	c.Check(got.Data.Attributes.Complete, qt.IsFalse)
	kept := got.Data.Attributes.ItemTypes[2]
	c.Check(kept.Name, qt.Equals, "Coins")
	c.Check(kept.State, qt.Equals, "added")
	c.Check(kept.Removable, qt.IsFalse)
	c.Check(kept.KeepReason, qt.Equals, "in_use")
	c.Check(kept.ItemCount, qt.Equals, 1)
	c.Check(got.Data.Attributes.CustomFields[0].KeepReason, qt.Equals, "in_use")
	c.Check(got.Data.Attributes.CustomFields[1].State, qt.Equals, "missing")

	w = collectionSetRequest(env, http.MethodDelete)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("removing again is safe; body: %s", w.Body.String()))
	c.Check(decodeItemSet(c, w).Meta.DeletedItemTypes, qt.HasLen, 0)

	c.Check(env.audit.actions(), qt.DeepEquals, []string{
		"group_collection_set_add", "group_collection_set_remove", "group_collection_set_remove",
	})
}

// Every member sees the set with the group; only an admin or owner adds or
// removes it.
func TestGroupsAPI_CollectionSet_AdminOnly(t *testing.T) {
	tests := []struct {
		role models.GroupRole
		want int
	}{
		{role: models.GroupRoleOwner, want: http.StatusOK},
		{role: models.GroupRoleAdmin, want: http.StatusOK},
		{role: models.GroupRoleUser, want: http.StatusForbidden},
		{role: models.GroupRoleViewer, want: http.StatusForbidden},
	}
	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			c := qt.New(t)
			env := newGroupTestEnv(t, models.Currency("USD"))
			env.setRole(c, tt.role)

			w := collectionSetRequest(env, http.MethodGet)
			c.Check(w.Code, qt.Equals, http.StatusOK, qt.Commentf("every member reads it; body: %s", w.Body.String()))
			w = collectionSetRequest(env, http.MethodPost)
			c.Check(w.Code, qt.Equals, tt.want, qt.Commentf("add; body: %s", w.Body.String()))
			w = collectionSetRequest(env, http.MethodDelete)
			c.Check(w.Code, qt.Equals, tt.want, qt.Commentf("remove; body: %s", w.Body.String()))

			set, ctx := groupRegistries(env, env.group)
			fields := must.Must(set.CustomFieldRegistry.List(ctx))
			if tt.want == http.StatusForbidden {
				c.Check(fields, qt.HasLen, 0)
				c.Check(env.audit.actions(), qt.HasLen, 0)
			}
		})
	}
}

func TestGroupsAPI_CollectionSet_RefusesAGroupBeingDeleted(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))
	env.group.Status = models.LocationGroupStatusPendingDeletion
	must.Must(env.factorySet.LocationGroupRegistry.Update(context.Background(), *env.group))

	for _, method := range []string{http.MethodPost, http.MethodDelete} {
		w := collectionSetRequest(env, method)
		c.Check(w.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("%s; body: %s", method, w.Body.String()))
		c.Check(w.Body.String(), checkers.JSONPathEquals("$.errors[0].code"), "group.not_active")
	}
}

func TestGroupsAPI_CreateGroup_WithCollectionSet(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))

	w := postGroup(t, env, map[string]any{
		"name":            "Coins",
		"group_currency":  "EUR",
		"hidden_sections": []string{"warranties"},
		"collection_set":  true,
	})
	c.Assert(w.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", w.Body.String()))
	var resp jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Meta, qt.IsNotNil)
	c.Check(resp.Meta.CollectionSet, qt.DeepEquals, &jsonapi.GroupCreateItemSetOutcome{
		Added: true, CreatedItemTypes: 7, CreatedCustomFields: 8,
	})
	c.Check(resp.Data.Attributes.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})
	c.Check(w.Body.String(), qt.Not(qt.Contains), `"collection_set":true`, qt.Commentf("the flag is not stored on the group"))

	group := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), resp.Data.ID))
	set, ctx := groupRegistries(env, group)
	fields := must.Must(set.CustomFieldRegistry.List(ctx))
	c.Check(fields, qt.HasLen, 8)
	c.Check(fields[0].Templates, qt.HasLen, 7)

	c.Check(env.audit.actions(), qt.DeepEquals, []string{"group_collection_set_add"})
	c.Check(*env.audit.snapshotEvents()[0].EntityID, qt.Equals, group.ID)
}

func TestGroupsAPI_CreateGroup_WithoutCollectionSet(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))

	w := postGroup(t, env, map[string]any{"name": "Home", "group_currency": "EUR"})
	c.Assert(w.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", w.Body.String()))
	var resp jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &resp), qt.IsNil)
	c.Check(resp.Meta, qt.IsNil)

	group := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), resp.Data.ID))
	set, ctx := groupRegistries(env, group)
	c.Check(must.Must(set.CustomFieldRegistry.List(ctx)), qt.HasLen, 0)
	c.Check(env.audit.actions(), qt.HasLen, 0)
}

func TestGroupsAPI_UpdateGroup_RefusesCollectionSet(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))

	w := patchGroup(t, env, map[string]any{"name": "Renamed", "collection_set": true})
	c.Assert(w.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", w.Body.String()))
	c.Check(w.Body.String(), checkers.JSONPathEquals("$.errors[0].error.errorCodes.attributes.collection_set.code"), "validation_create_only")

	stored := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Check(stored.Name, qt.Equals, "Test Group")
}
