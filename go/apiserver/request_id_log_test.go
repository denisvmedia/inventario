package apiserver_test

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"sync"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"go.5x5.cz/inventario/internal/logctx"
	"go.5x5.cz/inventario/models"
)

// The id in the X-Request-ID response header finds every line the request
// logged, wherever in the stack it was written, and not only the access log.
// These tests run the real router with the default logger wrapped the way
// cmd/inventario wraps it, and look for request_id on lines written by an
// early guard, by the authentication middleware and by a handler.

// logCapture is an io.Writer that a handler can write to while the test reads.
type logCapture struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logCapture) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// records returns the decoded JSON records whose message is msg.
func (l *logCapture) records(c *qt.C, msg string) []map[string]any {
	c.Helper()
	l.mu.Lock()
	defer l.mu.Unlock()
	var out []map[string]any
	scanner := bufio.NewScanner(bytes.NewReader(l.buf.Bytes()))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var rec map[string]any
		c.Assert(json.Unmarshal(scanner.Bytes(), &rec), qt.IsNil, qt.Commentf("line %q", scanner.Text()))
		if rec["msg"] == msg {
			out = append(out, rec)
		}
	}
	c.Assert(scanner.Err(), qt.IsNil)
	return out
}

// captureLogs points the default logger at a buffer until the test ends. The
// tests that call it must not run in parallel: the default logger is process
// state.
func captureLogs(c *qt.C) *logCapture {
	c.Helper()
	capture := &logCapture{}
	prev := slog.Default()
	slog.SetDefault(slog.New(logctx.NewHandler(
		slog.NewJSONHandler(capture, &slog.HandlerOptions{Level: slog.LevelDebug}))))
	c.Cleanup(func() { slog.SetDefault(prev) })
	return capture
}

func TestRequestID_IsOnTheLogLinesTheRequestWrites(t *testing.T) {
	for _, tc := range []struct {
		name    string
		method  string
		path    func(env *apiTokenEnv) string
		body    any
		headers map[string][]string
		status  int
		msg     string
	}{
		{
			name:    "a line from the tenant-header guard, ahead of everything else",
			method:  http.MethodGet,
			path:    func(*apiTokenEnv) string { return "/api/v1/groups" },
			headers: map[string][]string{"X-Tenant-ID": {"someone-else"}},
			status:  http.StatusForbidden,
			msg:     "Security violation: user-provided tenant ID in header",
		},
		{
			name:   "a line from the authentication middleware",
			method: http.MethodGet,
			path:   func(env *apiTokenEnv) string { return env.groupPath(env.group, "/locations") },
			headers: map[string][]string{
				"Authorization": {"Bearer " + must.Must(models.GenerateAPIToken()).Token},
			},
			status: http.StatusUnauthorized,
			msg:    "API token rejected",
		},
		{
			name:   "a line from a handler",
			method: http.MethodPost,
			path:   func(*apiTokenEnv) string { return "/api/v1/auth/login" },
			body:   map[string]any{"email": "nobody@example.com", "password": "Password123"},
			status: http.StatusUnauthorized,
			msg:    "Failed login attempt: user not found",
		},
	} {
		for _, supplied := range []string{"", "ticket-4417-attempt-2"} {
			name := tc.name + ", generated id"
			if supplied != "" {
				name = tc.name + ", client id"
			}
			t.Run(name, func(t *testing.T) {
				c := qt.New(t)
				env := newAPITokenEnv(c)
				logs := captureLogs(c)

				headers := map[string][]string{}
				maps.Copy(headers, tc.headers)
				if supplied != "" {
					headers["X-Request-ID"] = []string{supplied}
				}
				rr := env.do(apiTokenRequest{method: tc.method, path: tc.path(env), body: tc.body, headers: headers})
				c.Assert(rr.Code, qt.Equals, tc.status, qt.Commentf(rr.Body.String()))

				id := rr.Header().Get("X-Request-ID")
				c.Assert(id, qt.Not(qt.Equals), "")
				if supplied != "" {
					c.Assert(id, qt.Equals, supplied)
				}

				recs := logs.records(c, tc.msg)
				c.Assert(recs, qt.HasLen, 1)
				c.Assert(recs[0]["request_id"], qt.Equals, id)
			})
		}
	}
}
