package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// Cross-plane regression tests for the #1785 Phase 3 migration. The
// cross-tenant /api/v1/admin/* CRUD surface is now gated by
// RequireBackofficeAuth (aud=backoffice, admin_id claim). These tests
// pin the boundary: a tenant-side JWT — even one carrying
// is_system_admin=true — MUST be rejected at /admin/_ping, and a
// back-office JWT MUST be accepted.

// TestAdmin_RejectsTenantJWTAfterMigration locks in the audience guard:
// the legacy "tenant user with IsSystemAdmin=true" path no longer
// reaches an admin handler. RequireBackofficeAuth rejects the tenant
// token at the door because its `aud` (omitted by the tenant mint) is
// not "backoffice". The response is plain-text 401 — the back-office
// middleware emits its rejections via http.Error rather than the
// JSON:API envelope, matching the rest of the back-office plane.
func TestAdmin_RejectsTenantJWTAfterMigration(t *testing.T) {
	c := qt.New(t)
	params, user, _ := newParams()
	// Flip is_system_admin on the tenant user — this would have admitted
	// the request under the legacy RequireSystemAdmin gate. Under the
	// back-office gate the flag is irrelevant: the audience guard fires
	// first.
	promoteToSystemAdmin(c, params, user)

	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	addTestUserAuthHeader(req, user.ID)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)
}

// TestAdmin_AcceptsBackofficeJWT is the positive side of the audience
// guard: a token signed with the same JWT secret but carrying
// aud=backoffice + admin_id reaches the admin handler and returns 200.
func TestAdmin_AcceptsBackofficeJWT(t *testing.T) {
	c := qt.New(t)
	params, _, _ := newParams()
	_, token := WithBackofficeAdmin(t, params)

	handler := apiserver.APIServer(params, &mockRestoreWorker{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/_ping", nil)
	addBackofficeAuthHeader(req, token)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusOK)
}
