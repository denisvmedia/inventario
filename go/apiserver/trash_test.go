package apiserver_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/internal/checkers"
	"go.5x5.cz/inventario/models"
)

type trashAPIEnv struct {
	c       *qt.C
	params  apiserver.Params
	handler http.Handler
	user    *models.User
	group   *models.LocationGroup
}

func newTrashAPIEnv(c *qt.C) *trashAPIEnv {
	c.Helper()
	params, user, group := newParams()
	return &trashAPIEnv{
		c:       c,
		params:  params,
		handler: apiserver.APIServer(params, &mockRestoreWorker{}),
		user:    user,
		group:   group,
	}
}

func (e *trashAPIEnv) do(method, path, body string) *httptest.ResponseRecorder {
	e.c.Helper()
	req, err := http.NewRequest(method, "/api/v1/g/"+e.group.Slug+path, bytes.NewBufferString(body))
	e.c.Assert(err, qt.IsNil)
	req.Header.Set("Content-Type", "application/vnd.api+json")
	req.Header.Set("Accept", "application/vnd.api+json")
	addTestUserAuthHeader(req, e.user.ID)
	rr := httptest.NewRecorder()
	e.handler.ServeHTTP(rr, req)
	return rr
}

func (e *trashAPIEnv) setRole(role models.GroupRole) {
	e.c.Helper()
	ctx := context.Background()
	memberships := must.Must(e.params.FactorySet.GroupMembershipRegistry.ListByUser(ctx, e.user.TenantID, e.user.ID))
	for _, m := range memberships {
		if m.GroupID == e.group.ID {
			m.Role = role
			must.Must(e.params.FactorySet.GroupMembershipRegistry.Update(ctx, *m))
			return
		}
	}
	e.c.Fatalf("user is not a member of group %s", e.group.ID)
}

func (e *trashAPIEnv) commodityIDs() []string {
	e.c.Helper()
	all := must.Must(getRegistrySetFromParams(e.params, e.user).CommodityRegistry.List(context.Background()))
	ids := make([]string, 0, len(all))
	for _, item := range all {
		ids = append(ids, item.ID)
	}
	return ids
}

func TestTrashAPI_DeleteListRestore(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()
	c.Assert(len(ids) >= 2, qt.IsTrue, qt.Commentf("fixture needs two items"))

	rr := env.do(http.MethodDelete, "/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodPost, "/commodities/bulk-delete", `{"data":{"type":"commodities","attributes":{"ids":["`+ids[1]+`"]}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))

	// Deleted items are gone from the item list and waiting in Trash.
	rr = env.do(http.MethodGet, "/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)

	rr = env.do(http.MethodGet, "/trash/commodities", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.total"), float64(2))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.retention_days"), float64(30))
	c.Check(rr.Body.String(), qt.Contains, `"deleted_at"`)
	c.Check(rr.Body.String(), qt.Contains, `"expires_at"`)

	rr = env.do(http.MethodPost, "/trash/commodities/"+ids[0]+"/restore", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.id"), ids[0])
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.area_removed"), false)

	rr = env.do(http.MethodPost, "/trash/commodities/restore", `{"data":{"type":"commodities","attributes":{"ids":["`+ids[1]+`","`+ids[0]+`"]}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.succeeded[0]"), ids[1])
	// The first item is already back, so it is not in Trash any more.
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.failed[0].id"), ids[0])

	rr = env.do(http.MethodGet, "/commodities/"+ids[1], "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}

func TestTrashAPI_RoleGates(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()

	rr := env.do(http.MethodDelete, "/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	// A viewer sees the Trash but cannot restore.
	env.setRole(models.GroupRoleViewer)
	rr = env.do(http.MethodGet, "/trash/commodities", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	rr = env.do(http.MethodPost, "/trash/commodities/"+ids[0]+"/restore", "")
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)

	// A user can restore and delete to Trash, but not delete for good.
	env.setRole(models.GroupRoleUser)
	rr = env.do(http.MethodDelete, "/trash/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	rr = env.do(http.MethodDelete, "/trash", "")
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	rr = env.do(http.MethodPost, "/trash/commodities/purge", `{"data":{"type":"commodities","attributes":{"ids":["`+ids[0]+`"]}}}`)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)

	// An admin can.
	env.setRole(models.GroupRoleAdmin)
	rr = env.do(http.MethodDelete, "/trash/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodPost, "/trash/commodities/"+ids[0]+"/restore", "")
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)

	rr = env.do(http.MethodDelete, "/commodities/"+ids[1], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)
	rr = env.do(http.MethodDelete, "/trash", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data.attributes.commodities"), float64(1))
}

func TestTrashAPI_FileTrashedWithItemIsRefusedAlone(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()

	ctx := createTestUserContextWithGroup(env.user.ID, env.user.TenantID, env.group.ID)
	set := must.Must(env.params.FactorySet.CreateUserRegistrySet(ctx))
	file := must.Must(set.FileRegistry.Create(ctx, models.FileEntity{
		Title:    "manual",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		Links:    []models.FileLink{{EntityType: models.FileLinkCommodity, EntityID: ids[0], Meta: models.FileLinkMetaDocuments}},
		File: &models.File{
			Path:         "manual",
			OriginalPath: "manual.pdf",
			Ext:          ".pdf",
			MIMEType:     "application/pdf",
		},
	}))

	linked := must.Must(set.FileRegistry.ListByLinkedEntity(ctx, models.FileLinkCommodity, ids[0]))

	rr := env.do(http.MethodDelete, "/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent)

	rr = env.do(http.MethodPost, "/trash/files/"+file.ID+"/restore", "")
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity, qt.Commentf("body=%s", rr.Body.String()))
	c.Check(rr.Body.String(), qt.Contains, "trash.file_with_item")

	// Only files deleted on their own are listed under files.
	rr = env.do(http.MethodGet, "/trash/files", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.meta.total"), float64(0))
	rr = env.do(http.MethodGet, "/trash/commodities", "")
	// The fixture item has files of its own; every one of them went along.
	c.Check(rr.Body.String(), checkers.JSONPathEquals("$.data[0].meta.file_count"), float64(len(linked)))
}

func TestTrashAPI_ScheduleOfATrashedItemIsReadOnly(t *testing.T) {
	c := qt.New(t)
	env := newTrashAPIEnv(c)
	ids := env.commodityIDs()
	c.Assert(ids, qt.Not(qt.HasLen), 0)

	ctx := getRegistrySetFromParams(env.params, env.user)
	schedule := must.Must(ctx.MaintenanceScheduleRegistry.Create(context.Background(), models.MaintenanceSchedule{
		CommodityID:  ids[0],
		Title:        "Oil change",
		IntervalDays: 90,
		NextDueAt:    models.Date("2026-12-01"),
		Enabled:      true,
	}))

	rr := env.do(http.MethodDelete, "/commodities/"+ids[0], "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))

	path := "/maintenance/" + schedule.ID
	for _, call := range []struct{ method, path string }{
		{http.MethodPatch, path},
		{http.MethodPost, path + "/done"},
		{http.MethodDelete, path},
	} {
		rr = env.do(call.method, call.path, `{"data":{"type":"maintenance_schedules","attributes":{"title":"Changed"}}}`)
		c.Check(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf("%s %s body=%s", call.method, call.path, rr.Body.String()))
	}
	got := must.Must(ctx.MaintenanceScheduleRegistry.Get(context.Background(), schedule.ID))
	c.Assert(got.Title, qt.Equals, "Oil change")

	rr = env.do(http.MethodPost, "/trash/commodities/"+ids[0]+"/restore", "")
	c.Assert(rr.Code, qt.Equals, http.StatusOK, qt.Commentf("body=%s", rr.Body.String()))
	rr = env.do(http.MethodDelete, path, "")
	c.Assert(rr.Code, qt.Equals, http.StatusNoContent, qt.Commentf("body=%s", rr.Body.String()))
}
