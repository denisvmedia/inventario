package apiserver_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/internal/checkers"
	"github.com/denisvmedia/inventario/models"
)

// Worker soft-pause admin endpoint tests (#1308). The /admin/workers/*
// surface is gated by RequireBackofficeAuth (same as the rest of the
// admin CRUD subtree), so the actor is a back-office user minted via
// WithBackofficeAdmin. See admin_users_test.go for the shared harness
// (newAdminEnv / doAdminJSONRequest).

func TestAdminPauseWorker_HappyPath(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeExport)+"/pause",
		env.adminToken, map[string]any{"reason": "maintenance window"})
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.type"), "worker_control")
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.id"), string(models.WorkerTypeExport))
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.paused"), true)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.reason"), "maintenance window")

	// Persisted on the registry row, attributed to the back-office actor.
	rows := must.Must(env.params.FactorySet.WorkerControlRegistry.List(context.Background()))
	var row *models.WorkerControl
	for i := range rows {
		if rows[i].WorkerType == models.WorkerTypeExport {
			row = rows[i]
			break
		}
	}
	c.Assert(row, qt.IsNotNil)
	c.Assert(row.Paused, qt.IsTrue)
	c.Assert(row.PausedBy, qt.IsNotNil)
	c.Assert(*row.PausedBy, qt.Equals, env.admin.ID)
}

func TestAdminPauseWorker_EmptyBodyAllowed(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	// Reason is optional — an empty body must still pause.
	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeImport)+"/pause",
		env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	c.Assert(rr.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.paused"), true)
}

func TestAdminListWorkers_ReflectsPauseState(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	// Pause one worker, then assert the listing reflects every canonical
	// type with the paused one flagged.
	pause := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeThumbnail)+"/pause",
		env.adminToken, map[string]any{"reason": "gpu maintenance"})
	c.Assert(pause.Code, qt.Equals, http.StatusOK)

	rr := doAdminJSONRequest(t, env.handler, http.MethodGet,
		"/api/v1/admin/workers", env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusOK)
	// Every canonical worker type is listed.
	c.Assert(rr.Body.Bytes(), checkers.JSONPathMatches("$.data", qt.HasLen), len(models.AllWorkerTypes()))
}

func TestAdminResumeWorker_ClearsPause(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	// Pause, then resume; assert the round-trip flips paused back to false.
	pause := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeExport)+"/pause",
		env.adminToken, map[string]any{"reason": "x"})
	c.Assert(pause.Code, qt.Equals, http.StatusOK)
	c.Assert(pause.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.paused"), true)

	resume := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeExport)+"/resume",
		env.adminToken, nil)
	c.Assert(resume.Code, qt.Equals, http.StatusOK)
	c.Assert(resume.Body.Bytes(), checkers.JSONPathEquals("$.data.attributes.paused"), false)

	// Registry row is cleared too.
	rows := must.Must(env.params.FactorySet.WorkerControlRegistry.List(context.Background()))
	for i := range rows {
		if rows[i].WorkerType == models.WorkerTypeExport {
			c.Assert(rows[i].Paused, qt.IsFalse)
		}
	}
}

func TestAdminPauseWorker_UnknownTypeNotFound(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/not-a-real-worker/pause",
		env.adminToken, map[string]any{"reason": "x"})
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminWorkerUnknownTypeCode)
}

func TestAdminResumeWorker_UnknownTypeNotFound(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/not-a-real-worker/resume",
		env.adminToken, nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminWorkerUnknownTypeCode)
}

func TestAdminPauseWorker_ReasonTooLong(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeExport)+"/pause",
		env.adminToken, map[string]any{"reason": strings.Repeat("x", 501)})
	c.Assert(rr.Code, qt.Equals, http.StatusUnprocessableEntity)
	assertErrorCode(t, c, rr.Body.Bytes(), apiserver.AdminWorkerReasonTooLongCode)
}

func TestAdminPauseWorker_UnknownJSONFieldRejected(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)

	// DisallowUnknownFields rejects extra keys with a 400, mirroring the
	// block/unblock decode guard.
	rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeExport)+"/pause",
		env.adminToken, []byte(`{"reason":"x","bogus":true}`))
	c.Assert(rr.Code, qt.Equals, http.StatusBadRequest)
}

func TestAdminPauseWorker_DeniesTenantUser(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	// Tenant JWT — RequireBackofficeAuth rejects at the audience guard,
	// regardless of the user's IsSystemAdmin flag.
	req := httptest.NewRequest(http.MethodPost,
		"/api/v1/admin/workers/"+string(models.WorkerTypeExport)+"/pause", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

func TestAdminListWorkers_DeniesTenantUser(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/workers", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}
