package apiserver_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/apiserver"
	"github.com/denisvmedia/inventario/models"
)

// promoteToSystemAdmin grants the seeded test user system-admin via
// the system_admin_grants registry (#1784) so the admin-gate test can
// compare the gated vs ungated paths without having to wire a second
// user fixture. Idempotent.
func promoteToSystemAdmin(c *qt.C, params apiserver.Params, user *models.User) {
	c.Helper()
	must.Must(params.FactorySet.SystemAdminGrantRegistry.Grant(context.Background(), user.ID, nil))
}

func TestAdminPing_DeniesNonAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
	// JSON:API envelope carries the wire code so the FE can branch.
	c.Assert(rr.Body.String(), qt.Contains, "admin.forbidden")
}

func TestAdminPing_AllowsSystemAdmin(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	promoteToSystemAdmin(c, params, user)

	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	var body apiserver.AdminPingResponse
	c.Assert(json.Unmarshal(rr.Body.Bytes(), &body), qt.IsNil)
	c.Assert(body.Ok, qt.IsTrue)
	c.Assert(body.Timestamp.IsZero(), qt.IsFalse)
}

func TestAdminPing_DeniesUnauthenticated(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}
