package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
)

func putHiddenSections(t *testing.T, env groupTestEnv, sections any) *httptest.ResponseRecorder {
	t.Helper()

	body := map[string]any{
		"data": map[string]any{
			"type":       "groups",
			"attributes": map[string]any{"hidden_sections": sections},
		},
	}
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal put body: %v", err)
	}

	req := httptest.NewRequest(http.MethodPut, "/groups/"+env.group.ID+"/hidden-sections", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	return w
}

func decodeGroup(c *qt.C, w *httptest.ResponseRecorder) *models.LocationGroup {
	c.Helper()
	var resp jsonapi.LocationGroupResponse
	c.Assert(json.Unmarshal(w.Body.Bytes(), &resp), qt.IsNil)
	c.Assert(resp.Data, qt.IsNotNil)
	return resp.Data.Attributes
}

func (env groupTestEnv) setRole(c *qt.C, role models.GroupRole) {
	c.Helper()
	ctx := context.Background()
	memberships := must.Must(env.factorySet.GroupMembershipRegistry.ListByGroup(ctx, env.group.ID))
	c.Assert(memberships, qt.HasLen, 1)
	m := memberships[0]
	m.Role = role
	must.Must(env.factorySet.GroupMembershipRegistry.Update(ctx, *m))
}

func TestGroupsAPI_GetGroup_HidesNothingByDefault(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))

	req := httptest.NewRequest(http.MethodGet, "/groups/"+env.group.ID, nil)
	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))

	var raw struct {
		Data struct {
			Attributes map[string]json.RawMessage `json:"attributes"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(w.Body.Bytes(), &raw), qt.IsNil)
	c.Assert(string(raw.Data.Attributes["hidden_sections"]), qt.Equals, "[]")
}

func TestGroupsAPI_SetHiddenSections(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))

	w := putHiddenSections(t, env, []string{"upcoming", "warranties", "upcoming"})
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	got := decodeGroup(c, w)
	c.Assert(got.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionUpcoming})
	c.Assert(got.Name, qt.Equals, "Test Group")

	stored := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
	c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties, models.GroupSectionUpcoming})

	w = putHiddenSections(t, env, []string{})
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	c.Assert(decodeGroup(c, w).HiddenSections, qt.DeepEquals, models.GroupSections{})
}

func TestGroupsAPI_SetHiddenSections_Refuses(t *testing.T) {
	tests := []struct {
		name     string
		sections any
	}{
		{name: "unknown section", sections: []string{"warranties", "files"}},
		{name: "missing list", sections: nil},
		{name: "not a list", sections: "warranties"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			env := newGroupTestEnv(t, models.Currency("USD"))

			w := putHiddenSections(t, env, tt.sections)
			c.Assert(w.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", w.Body.String()))

			stored := must.Must(env.factorySet.LocationGroupRegistry.Get(context.Background(), env.group.ID))
			c.Assert(stored.HiddenSections, qt.HasLen, 0)
		})
	}
}

// Every member reads the setting with the group; only an admin or owner
// changes it.
func TestGroupsAPI_SetHiddenSections_AdminOnly(t *testing.T) {
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

			w := putHiddenSections(t, env, []string{"loans"})
			c.Assert(w.Code, qt.Equals, tt.want, qt.Commentf("body: %s", w.Body.String()))

			w = patchGroup(t, env, map[string]any{"name": "Test Group", "hidden_sections": []string{"loans"}})
			c.Assert(w.Code, qt.Equals, tt.want, qt.Commentf("body: %s", w.Body.String()))

			req := httptest.NewRequest(http.MethodGet, "/groups/"+env.group.ID, nil)
			r := httptest.NewRecorder()
			env.router.ServeHTTP(r, req)
			c.Assert(r.Code, qt.Equals, http.StatusOK)
			stored := decodeGroup(c, r)
			if tt.want == http.StatusOK {
				c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionLoans})
			} else {
				c.Assert(stored.HiddenSections, qt.DeepEquals, models.GroupSections{})
			}
		})
	}
}

func TestGroupsAPI_CreateGroup_WithHiddenSections(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, "")

	w := postGroup(t, env, map[string]any{"name": "Stamps", "hidden_sections": []string{"warranties"}})
	c.Assert(w.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", w.Body.String()))
	created := decodeGroup(c, w)
	c.Assert(created.HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionWarranties})

	w = postGroup(t, env, map[string]any{"name": "Household"})
	c.Assert(w.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", w.Body.String()))
	c.Assert(decodeGroup(c, w).HiddenSections, qt.DeepEquals, models.GroupSections{})

	w = postGroup(t, env, map[string]any{"name": "Bad", "hidden_sections": []string{"tags"}})
	c.Assert(w.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", w.Body.String()))
}

func TestGroupsAPI_UpdateGroup_KeepsHiddenSectionsUnlessGiven(t *testing.T) {
	c := qt.New(t)
	env := newGroupTestEnv(t, models.Currency("USD"))
	c.Assert(putHiddenSections(t, env, []string{"maintenance"}).Code, qt.Equals, http.StatusOK)

	w := patchGroup(t, env, map[string]any{"name": "Renamed"})
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	c.Assert(decodeGroup(c, w).HiddenSections, qt.DeepEquals, models.GroupSections{models.GroupSectionMaintenance})

	w = patchGroup(t, env, map[string]any{"name": "Renamed", "hidden_sections": []string{}})
	c.Assert(w.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", w.Body.String()))
	c.Assert(decodeGroup(c, w).HiddenSections, qt.DeepEquals, models.GroupSections{})
}
