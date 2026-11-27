package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/registry/memory"
	"go.5x5.cz/inventario/services"
)

// What a refusal logs, checked against devdocs/logging.md: the line is there,
// at Warn at most, and it carries no credential the request held. Like the
// request id tests, these swap the default logger and must not run in
// parallel.

// String returns everything logged so far.
func (l *logCapture) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
}

func TestSignedURLMiddleware_RejectionLineLeavesOutTheSignature(t *testing.T) {
	c := qt.New(t)
	logs := captureLogs(c)

	fs := memory.NewFactorySet()
	signer := services.NewFileSigningService([]byte("test-signing-key-32-bytes-long!!"), 15*time.Minute)
	handler := apiserver.SignedURLMiddleware(signer, newMockUserRegistry(), fs.FileRegistryFactory, fs.LocationGroupRegistry)(okHandler())

	const signature = "c2lnbmF0dXJlLXRoYXQtaXMtbm90LWxvZ2dlZA"
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet,
		"/api/v1/files/download/report.pdf?sig="+signature+"&exp=9999999999&uid=user-1&fid=file-1", nil))
	c.Assert(rr.Code, qt.Equals, http.StatusUnauthorized)

	c.Assert(logs.String(), qt.Not(qt.Contains), signature)
	recs := logs.records(c, "Invalid signed URL access attempt")
	c.Assert(recs, qt.HasLen, 1)
	c.Assert(recs[0]["level"], qt.Equals, "WARN")
	c.Assert(recs[0]["path"], qt.Equals, "/api/v1/files/download/report.pdf")
}

func TestValidateNoUserProvidedTenantID_BodyLineLeavesOutTheBody(t *testing.T) {
	c := qt.New(t)
	logs := captureLogs(c)

	handler := apiserver.ValidateNoUserProvidedTenantID(false)(okHandler())

	const password = "Body-Password-7731"
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(
		`{"email":"someone@example.com","password":"`+password+`","tenant_id":"another-tenant"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)

	c.Assert(logs.String(), qt.Not(qt.Contains), password)
	recs := logs.records(c, "Security violation: user-provided tenant ID in request body")
	c.Assert(recs, qt.HasLen, 1)
	c.Assert(recs[0]["level"], qt.Equals, "WARN")
	c.Assert(recs[0]["path"], qt.Equals, "/api/v1/auth/login")
}

func TestCommodityCtx_MissingItemIsOneWarning(t *testing.T) {
	c := qt.New(t)
	env := newAPITokenEnv(c)
	logs := captureLogs(c)

	rr := env.session(http.MethodGet, env.groupPath(env.group, "/commodities/no-such-item"), nil)
	c.Assert(rr.Code, qt.Equals, http.StatusNotFound, qt.Commentf(rr.Body.String()))

	recs := logs.records(c, "CommodityCtx: Failed to get commodity")
	c.Assert(recs, qt.HasLen, 1)
	c.Assert(recs[0]["level"], qt.Equals, "WARN")
	c.Assert(recs[0]["commodity_id"], qt.Equals, "no-such-item")
	c.Assert(logs.records(c, "internal server error"), qt.HasLen, 0)
}
