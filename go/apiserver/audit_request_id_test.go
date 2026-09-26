package apiserver_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

// chi honors an incoming X-Request-Id verbatim, and the audit row now persists
// it — so the client decides what goes in the column unless the middleware
// bounds it. An unusable id is dropped rather than truncated: a truncated id
// correlates with nothing, and the row is better off NULL than holding junk.
func TestAuditRow_RefusesAnUnusableClientRequestID(t *testing.T) {
	c := qt.New(t)

	for _, tc := range []struct {
		name     string
		supplied string
		stored   bool
	}{
		{"a plausible id is kept", "ticket-4417", true},
		{"a UUID is kept", "3f8a1c62-9d4e-4b17-8c2a-0f5b6d7e8a91", true},
		{"exactly at the cap is kept", strings.Repeat("a", 128), true},
		{"one byte over the cap is dropped", strings.Repeat("a", 129), false},
		{"a kilobyte is dropped", strings.Repeat("a", 1024), false},
		{"a newline is dropped", "abc\ndef", false},
		{"a NUL is dropped", "abc\x00def", false},
		{"non-ASCII is dropped", "заявка-4417", false},
	} {
		c.Run(tc.name, func(c *qt.C) {
			env := newAdminEnv(c)
			target := createTestUserDirect(c, env.params, env.tenantID, "bounded@example.com", true, false)

			rr := doAdminJSONRequestWithHeaders(t, env.handler, http.MethodPost,
				"/api/v1/admin/users/"+target.ID+"/sessions/revoke",
				env.adminToken, revokeSessionsBody("bound check"),
				map[string]string{"X-Request-Id": tc.supplied})
			c.Assert(rr.Code, qt.Equals, http.StatusOK)

			rows := auditRowsFor(c, env, "admin.user_sessions_revoke")
			c.Assert(rows, qt.HasLen, 1)
			if tc.stored {
				c.Assert(rows[0].RequestID, qt.IsNotNil)
				c.Assert(*rows[0].RequestID, qt.Equals, tc.supplied)
				return
			}
			// Dropped, not truncated — so nothing in the column pretends to be
			// the caller's id.
			c.Assert(rows[0].RequestID, qt.IsNil,
				qt.Commentf("an unusable client id reached the audit row"))
		})
	}
}
