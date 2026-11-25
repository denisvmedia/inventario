package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

type savedViewsEnv struct {
	handler   http.Handler
	slug      string
	adminID   string
	viewerID  string
	otherUser string
}

func newSavedViewsEnv(c *qt.C) *savedViewsEnv {
	c.Helper()
	params, admin, group := newParams()
	ctx := context.Background()
	join := func(email string, role models.GroupRole) string {
		u := createTestUserDirect(c, params, admin.TenantID, email, true, false)
		must.Must(params.FactorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: admin.TenantID},
			GroupID:             group.ID,
			MemberUserID:        u.ID,
			Role:                role,
		}))
		return u.ID
	}
	return &savedViewsEnv{
		handler:   apiserver.APIServer(params, &mockRestoreWorker{}),
		slug:      group.Slug,
		adminID:   admin.ID,
		viewerID:  join("saved-views-viewer@example.com", models.GroupRoleViewer),
		otherUser: join("saved-views-user@example.com", models.GroupRoleUser),
	}
}

func (e *savedViewsEnv) do(c *qt.C, method, path, userID string, body any) *httptest.ResponseRecorder {
	c.Helper()
	var reader *bytes.Reader
	if body != nil {
		reader = bytes.NewReader(must.Must(json.Marshal(body)))
	} else {
		reader = bytes.NewReader(nil)
	}
	req := must.Must(http.NewRequest(method, "/api/v1/g/"+e.slug+"/saved-views"+path, reader))
	req.Header.Set("Content-Type", "application/vnd.api+json")
	addTestUserAuthHeader(req, userID)
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

func savedViewBody(attrs map[string]any) map[string]any {
	return map[string]any{"data": map[string]any{"type": "saved_views", "attributes": attrs}}
}

func (e *savedViewsEnv) create(c *qt.C, userID string, attrs map[string]any) string {
	c.Helper()
	rr := e.do(c, http.MethodPost, "", userID, savedViewBody(attrs))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", rr.Body.String()))
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &resp), qt.IsNil)
	return resp.Data.ID
}

func TestSavedViews_CreateListAndGet(t *testing.T) {
	c := qt.New(t)
	env := newSavedViewsEnv(c)

	rr := env.do(c, http.MethodPost, "", env.viewerID, savedViewBody(map[string]any{
		"name":            "Fragile",
		"icon":            "star",
		"query":           "tags=Fragile&include_inactive=false",
		"layout":          "list",
		"show_in_sidebar": true,
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusCreated, qt.Commentf("body: %s", rr.Body.String()))
	body := rr.Body.Bytes()
	c.Check(body, checkers.JSONPathEquals("$.data.type"), "saved_views")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.query"), "include_inactive=false&tags=fragile")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.icon"), "star")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.layout"), "list")
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.owned"), true)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.can_edit"), true)
	c.Check(body, checkers.JSONPathEquals("$.data.attributes.shared"), false)

	var created struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(body, &created), qt.IsNil)

	rr = env.do(c, http.MethodGet, "", env.viewerID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 1)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.limits.max_name_length"), float64(models.MaxSavedViewNameRunes))
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.limits.can_share"), false)

	// The view is personal: another member does not see it.
	rr = env.do(c, http.MethodGet, "", env.otherUser, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), 0)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.meta.limits.can_share"), true)
	rr = env.do(c, http.MethodGet, "/"+created.Data.ID, env.otherUser, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
}

func TestSavedViews_RefusesWhatTheContractRefuses(t *testing.T) {
	c := qt.New(t)
	env := newSavedViewsEnv(c)

	rr := env.do(c, http.MethodPost, "", env.adminID, savedViewBody(map[string]any{
		"name":  "Old warranties",
		"query": "warranty_expires_before=2026-01-01",
	}))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", rr.Body.String()))
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.errors[0].code"), "query.invalid_parameter")
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.errors[0].meta.parameter"), "warranty_expires_before")

	rr = env.do(c, http.MethodPost, "", env.adminID, savedViewBody(map[string]any{"name": "", "query": ""}))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)

	rr = env.do(c, http.MethodPost, "", env.adminID, savedViewBody(map[string]any{"name": "Rocket", "icon": "rocket"}))
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}

func TestSavedViews_SharingFollowsTheRole(t *testing.T) {
	c := qt.New(t)
	env := newSavedViewsEnv(c)

	rr := env.do(c, http.MethodPost, "", env.viewerID, savedViewBody(map[string]any{"name": "Shared", "shared": true}))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden, qt.Commentf("body: %s", rr.Body.String()))
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.errors[0].code"), "saved_view.forbidden")

	id := env.create(c, env.otherUser, map[string]any{"name": "For everyone", "shared": true, "query": "status=in_use"})

	// The viewer sees the shared view and may not change it.
	rr = env.do(c, http.MethodGet, "/"+id, env.viewerID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.owned"), false)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.can_edit"), false)
	rr = env.do(c, http.MethodPatch, "/"+id, env.viewerID, savedViewBody(map[string]any{"name": "Taken"}))
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	rr = env.do(c, http.MethodDelete, "/"+id, env.viewerID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)

	// The admin may edit and delete it.
	rr = env.do(c, http.MethodPatch, "/"+id, env.adminID, savedViewBody(map[string]any{"name": "Renamed", "show_on_dashboard": true}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", rr.Body.String()))
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.name"), "Renamed")
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.show_on_dashboard"), true)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.query"), "status=in_use")
	rr = env.do(c, http.MethodDelete, "/"+id, env.adminID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
}

func TestSavedViews_PatchValidatesWhatItChanges(t *testing.T) {
	c := qt.New(t)
	env := newSavedViewsEnv(c)
	id := env.create(c, env.viewerID, map[string]any{"name": "Mine"})

	for name, attrs := range map[string]map[string]any{
		"empty name":     {"name": ""},
		"unknown icon":   {"icon": "rocket"},
		"unknown layout": {"layout": "cards"},
		"bad query":      {"query": "lent_out=false"},
	} {
		c.Run(name, func(c *qt.C) {
			rr := env.do(c, http.MethodPatch, "/"+id, env.viewerID, savedViewBody(attrs))
			c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body: %s", rr.Body.String()))
		})
	}

	rr := env.do(c, http.MethodPatch, "/"+id, env.viewerID, savedViewBody(map[string]any{"layout": ""}))
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", rr.Body.String()))
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.name"), "Mine")
}

func TestSavedViews_CountsAndReorder(t *testing.T) {
	c := qt.New(t)
	env := newSavedViewsEnv(c)

	all := env.create(c, env.adminID, map[string]any{"name": "Everything", "show_in_sidebar": true})
	none := env.create(c, env.adminID, map[string]any{"name": "Nothing", "show_in_sidebar": true, "query": "q=no-such-item-anywhere"})
	hidden := env.create(c, env.adminID, map[string]any{"name": "Hidden"})

	rr := env.do(c, http.MethodGet, "/counts", env.adminID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body: %s", rr.Body.String()))
	var counts struct {
		Data struct {
			Counts map[string]int `json:"counts"`
		} `json:"data"`
	}
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &counts), qt.IsNil)
	c.Assert(counts.Data.Counts[all] > 0, qt.IsTrue, qt.Commentf("counts: %v", counts.Data.Counts))
	c.Assert(counts.Data.Counts[none], qt.Equals, 0)
	_, counted := counts.Data.Counts[hidden]
	c.Assert(counted, qt.IsFalse)

	// The count is the total the items list reports for the same query.
	req := must.Must(http.NewRequest(http.MethodGet, "/api/v1/g/"+env.slug+"/commodities", nil))
	addTestUserAuthHeader(req, env.adminID)
	list := httptest.NewRecorder()
	env.handler.ServeHTTP(list, req)
	c.Assert(list.Code, qt.Equals, http.StatusOK)
	c.Assert(list.Header().Get("X-Total"), qt.Equals, strconv.Itoa(counts.Data.Counts[all]))

	rr = env.do(c, http.MethodPost, "/reorder", env.adminID, map[string]any{
		"data": map[string]any{"type": "saved_views_reorder", "attributes": map[string]any{"ids": []string{hidden, all}}},
	})
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body: %s", rr.Body.String()))
	rr = env.do(c, http.MethodGet, "", env.adminID, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[0].attributes.name"), "Hidden")
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[1].attributes.name"), "Nothing")
	c.Check(rr.Body.Bytes(), checkers.JSONPathEquals("$.data[2].attributes.name"), "Everything")

	rr = env.do(c, http.MethodPost, "/reorder", env.adminID, map[string]any{
		"data": map[string]any{"type": "saved_views_reorder", "attributes": map[string]any{"ids": []string{}}},
	})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
}
