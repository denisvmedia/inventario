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

	"go.5x5.cz/inventario/models"
)

// #2479 / #967 H5: the correlation id reaches the audit row, not only the log
// line. Without it the two are joinable by timestamp and user, which is enough
// to guess a correlation and not enough to prove one — two requests from the
// same user in the same second are indistinguishable.

// auditRowsFor returns the audit rows whose action matches.
func auditRowsFor(c *qt.C, env adminTestEnv, action string) []*models.AuditLog {
	c.Helper()
	var out []*models.AuditLog
	for _, row := range must.Must(env.params.FactorySet.AuditLogRegistry.List(context.Background())) {
		if row.Action == action {
			out = append(out, row)
		}
	}
	return out
}

// An audit row written on the HTTP path carries the request's correlation id,
// and a caller-supplied one is honored — which is what makes correlation work
// across a proxy or a sibling service that already has an id for the operation.
func TestAuditRow_CarriesTheRequestID(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	target := createTestUserDirect(c, env.params, env.tenantID, "correlated@example.com", true, false)

	const supplied = "ticket-4417-attempt-2"
	rr := doAdminJSONRequestWithHeaders(t, env.handler, http.MethodPost,
		"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
		env.adminToken, revokeSessionsBody("correlation check"),
		map[string]string{"X-Request-Id": supplied})
	c.Assert(rr.Code, qt.Equals, http.StatusOK)

	rows := auditRowsFor(c, env, "admin.user_sessions_revoke")
	c.Assert(rows, qt.HasLen, 1)
	c.Assert(rows[0].RequestID, qt.IsNotNil, qt.Commentf("the audit row carries no request id"))
	c.Assert(*rows[0].RequestID, qt.Equals, supplied)
}

// Two requests from the same actor in the same second get different ids, which
// is the case a timestamp join cannot separate and the whole reason the column
// exists.
func TestAuditRow_RequestIDSeparatesRequestsInTheSameSecond(t *testing.T) {
	c := qt.New(t)
	env := newAdminEnv(c)
	first := createTestUserDirect(c, env.params, env.tenantID, "one@example.com", true, false)
	second := createTestUserDirect(c, env.params, env.tenantID, "two@example.com", true, false)

	for _, target := range []*models.User{first, second} {
		rr := doAdminJSONRequest(t, env.handler, http.MethodPost,
			"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
			env.adminToken, revokeSessionsBody("back to back"))
		c.Assert(rr.Code, qt.Equals, http.StatusOK)
	}

	rows := auditRowsFor(c, env, "admin.user_sessions_revoke")
	c.Assert(rows, qt.HasLen, 2)
	c.Assert(rows[0].RequestID, qt.IsNotNil)
	c.Assert(rows[1].RequestID, qt.IsNotNil)
	c.Assert(*rows[0].RequestID, qt.Not(qt.Equals), *rows[1].RequestID)
}

// doAdminJSONRequestWithHeaders is doAdminJSONRequest plus extra request
// headers. Kept here rather than widening the shared helper's signature, since
// only the correlation-id tests need it.
func doAdminJSONRequestWithHeaders(
	t *testing.T,
	handler http.Handler,
	method, path, bearerToken string,
	body any,
	headers map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	req, err := http.NewRequest(method, path, bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	return rr
}
