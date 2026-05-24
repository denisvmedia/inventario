package apiserver_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
)

// tenantScanCap mirrors the unexported tenantScanMaxBodyBytes constant in
// security_middleware.go. Kept literal here so the test asserts the
// observable contract (1 MiB) without leaking the implementation symbol.
const tenantScanCap = 1 * 1024 * 1024

// Expected response bodies for the middleware's three short-circuit paths.
// http.Error always appends "\n" after the message, so these mirror what
// the wire-level response body looks like to a client. Kept as literal
// strings (rather than re-exporting from the apiserver package) so the
// test asserts the observable contract.
const (
	tenantSizeCapBody   = "Request body too large\n"
	tenantViolationBody = "Security violation: tenant information cannot be provided by user\n"
)

// TestValidateNoUserProvidedTenantID_AdminSubtreeQueryExemption verifies the
// narrow exemption added for #1748: the /api/v1/admin/* subtree is exempt
// from the query-parameter "tenant" check (so the documented ?tenantID=
// listing filter works), while every other path — and the header / body
// checks on every path including admin — stay fully enforced.
func TestValidateNoUserProvidedTenantID_AdminSubtreeQueryExemption(t *testing.T) {
	// downstream is the handler the middleware wraps; reaching it means the
	// middleware allowed the request through (200). A 403 means the
	// middleware rejected it before downstream ever ran.
	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiserver.ValidateNoUserProvidedTenantID(false)(downstream)

	tests := []struct {
		name       string
		method     string
		target     string
		header     [2]string // name, value; empty name = no header
		wantStatus int
	}{
		{
			name:       "admin path with tenantID query param is allowed",
			method:     http.MethodGet,
			target:     "/api/v1/admin/groups?tenantID=tenant-xyz",
			wantStatus: http.StatusOK,
		},
		{
			name:       "non-admin path with tenant query param is still rejected",
			method:     http.MethodGet,
			target:     "/api/v1/groups?tenantID=tenant-xyz",
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "admin path with tenant header is still rejected",
			method:     http.MethodGet,
			target:     "/api/v1/admin/groups",
			header:     [2]string{"X-Tenant-ID", "tenant-xyz"},
			wantStatus: http.StatusForbidden,
		},
		{
			// A sibling path that merely shares the "/api/v1/admin" stem
			// but is not under the "/api/v1/admin/" subtree must NOT be
			// exempt — the trailing slash in the prefix guards this.
			name:       "admin-prefixed sibling path is not exempt",
			method:     http.MethodGet,
			target:     "/api/v1/administrate?tenantID=tenant-xyz",
			wantStatus: http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			req := httptest.NewRequest(tc.method, tc.target, nil)
			if tc.header[0] != "" {
				req.Header.Set(tc.header[0], tc.header[1])
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, tc.wantStatus)
		})
	}
}

// TestValidateNoUserProvidedTenantID_RejectTenantBodyCamelCaseVariants
// verifies the camelCase coverage added for #1782: a body containing a
// JSON key such as "tenantId" or "tenantID" lowercases to "tenantid",
// which earlier slipped past the blacklist (the pre-fix patterns only
// matched "tenant_id" and quoted "tenant"). The fix adds the quoted
// "tenantid" substring so JSON keys are caught while bare-word free text
// remains untouched. The test also keeps a positive case for the load-
// bearing snake_case "tenant_id" pattern so a future refactor cannot
// silently drop it.
func TestValidateNoUserProvidedTenantID_RejectTenantBodyCamelCaseVariants(t *testing.T) {
	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiserver.ValidateNoUserProvidedTenantID(false)(downstream)

	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{
			name:       "camelCase tenantId json key is rejected",
			body:       `{"tenantId":"tenant-xyz"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "camelCase tenantID json key is rejected",
			body:       `{"tenantID":"tenant-xyz"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "mixed case TenantId json key is rejected case-insensitively",
			body:       `{"TenantId":"tenant-xyz"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "whitespace between key and colon is still rejected",
			body:       `{"tenantId" : "tenant-xyz"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:       "snake_case tenant_id json key is still rejected",
			body:       `{"tenant_id":"tenant-xyz"}`,
			wantStatus: http.StatusForbidden,
		},
		{
			name:        "snake_case tenant_id form-encoded body is still rejected",
			contentType: "application/x-www-form-urlencoded",
			body:        `tenant_id=tenant-xyz`,
			wantStatus:  http.StatusForbidden,
		},
		{
			name:       "clean body without tenant fields is allowed",
			body:       `{"name":"my group"}`,
			wantStatus: http.StatusOK,
		},
		{
			name:       "bare word tenantid in free text is not flagged",
			body:       `{"description":"contains the word tenantid as free text"}`,
			wantStatus: http.StatusOK,
		},
		{
			// The double-quoted "\"tenantid\"" pattern is the only camelCase
			// substring added — see the rationale in rejectTenantBody's doc
			// comment. A single-quoted occurrence inside a description-style
			// string must NOT trip the check; this guards against a future
			// reflex to mirror the new pattern with a 'tenantid' substring.
			name:       "single-quoted tenantid inside a json string value is not flagged",
			body:       `{"description":"contains 'tenantid' in single quotes"}`,
			wantStatus: http.StatusOK,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/groups",
				strings.NewReader(tc.body))
			contentType := tc.contentType
			if contentType == "" {
				contentType = "application/json"
			}
			req.Header.Set("Content-Type", contentType)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, tc.wantStatus)
		})
	}
}

// TestValidateNoUserProvidedTenantID_AdminSubtreeBodyCheckEnforced verifies
// that the request-body "tenant_id" check stays in force for admin paths —
// the #1748 exemption relaxes the query-parameter check ONLY.
func TestValidateNoUserProvidedTenantID_AdminSubtreeBodyCheckEnforced(t *testing.T) {
	c := qt.New(t)

	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiserver.ValidateNoUserProvidedTenantID(false)(downstream)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups",
		strings.NewReader(`{"tenant_id":"tenant-xyz"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}

// buildJSONPadded constructs a JSON document that (when scanned for
// tenant_id substrings) contains no positive match. The shape is
// {"x":"<padding>"} with ASCII 'a' bytes as padding. The returned slice
// is EXACTLY `size` bytes long when `size >= len(prefix)+len(suffix)`
// (the only regime any caller in this test exercises); below that
// threshold it returns the literal `{}` (2 bytes) as a sentinel so
// degenerate inputs cannot produce malformed output.
func buildJSONPadded(size int) []byte {
	const prefix = `{"x":"`
	const suffix = `"}`
	if size < len(prefix)+len(suffix) {
		// Out-of-contract sentinel — callers here always request a size
		// well above the wrapper width, so this branch is purely defensive.
		return []byte(`{}`)
	}
	padLen := size - len(prefix) - len(suffix)
	buf := make([]byte, 0, size)
	buf = append(buf, prefix...)
	for range padLen {
		buf = append(buf, 'a')
	}
	buf = append(buf, suffix...)
	return buf
}

// buildMultipartBody constructs a multipart/form-data body of approximately
// totalSize bytes, optionally prefixed by extra named form fields. Each
// extraFieldName is added as a separate form-data part containing the
// fixed string "ignored"; this lets callers force a specific field-name
// substring (e.g. "tenant_id") into the body to test the multipart-skip
// path. Returns the body and the Content-Type header value (which carries
// the boundary). Any writer error fails the test immediately — a panic
// from the multipart pipeline would otherwise surface as a confusing
// goroutine trace rather than a clear test failure.
func buildMultipartBody(tb testing.TB, totalSize int, extraFieldNames ...string) ([]byte, string) {
	tb.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, name := range extraFieldNames {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", `form-data; name="`+name+`"`)
		part, err := w.CreatePart(h)
		if err != nil {
			tb.Fatalf("buildMultipartBody: CreatePart(%q) failed: %v", name, err)
		}
		if _, err := part.Write([]byte("ignored")); err != nil {
			tb.Fatalf("buildMultipartBody: write ignored value for %q failed: %v", name, err)
		}
	}

	// Add a file part and pad it out to reach approximately totalSize.
	fh := make(textproto.MIMEHeader)
	fh.Set("Content-Disposition", `form-data; name="file"; filename="big.bin"`)
	fh.Set("Content-Type", "application/octet-stream")
	part, err := w.CreatePart(fh)
	if err != nil {
		tb.Fatalf("buildMultipartBody: CreatePart(file) failed: %v", err)
	}
	// Pad to (approximately) totalSize. We aim a bit shy so that closing
	// the writer (which appends a trailing boundary) does not over-shoot
	// dramatically; for the assertions in this test the exact size does
	// not matter, only that it is well above tenantScanCap.
	padLen := max(
		// leave headroom for boundaries
		totalSize-buf.Len()-256, 0)
	pad := make([]byte, padLen)
	for i := range pad {
		pad[i] = 'A'
	}
	if _, err := part.Write(pad); err != nil {
		tb.Fatalf("buildMultipartBody: pad write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("buildMultipartBody: writer Close failed: %v", err)
	}

	return buf.Bytes(), w.FormDataContentType()
}

// TestValidateNoUserProvidedTenantID_RejectTenantBody_SizeCap covers the
// rejectTenantBody size-cap and multipart-skip behaviour added for #1826.
//
// Cases:
//
//	(a) JSON well below cap     -> 200
//	(b) JSON exactly at cap     -> 200
//	(c) JSON one byte over cap  -> 413
//	(d) Form one byte over cap  -> 413
//	(e) Multipart over cap, no tenant_id substring -> 200 (multipart skip)
//	(f) Multipart over cap, with tenant_id field name -> 200 (anti-regression)
//	(g) JSON with tenant_id under cap -> 403 (scan still fires)
//	(h) GET with huge body      -> 200 (method gate first; no read)
func TestValidateNoUserProvidedTenantID_RejectTenantBody_SizeCap(t *testing.T) {
	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiserver.ValidateNoUserProvidedTenantID(false)(downstream)

	multipartClean, multipartCleanCT := buildMultipartBody(t, 2*tenantScanCap)
	multipartWithTenant, multipartWithTenantCT := buildMultipartBody(t, 2*tenantScanCap, "tenant_id")

	// Every case sets contentType and wantBody explicitly — empty
	// strings are the literal expected values, not "skip this assertion"
	// sentinels. Project rule: no conditionals in tests.
	tests := []struct {
		name        string
		method      string
		target      string
		contentType string
		body        []byte
		wantStatus  int
		wantBody    string // exact response body the client receives
	}{
		{
			name:        "json well below cap is allowed",
			method:      http.MethodPost,
			target:      "/api/v1/areas",
			contentType: "application/json",
			body:        buildJSONPadded(1024),
			wantStatus:  http.StatusOK,
			wantBody:    "", // downstream only WriteHeaders, no body
		},
		{
			name:        "json exactly at cap is allowed",
			method:      http.MethodPost,
			target:      "/api/v1/areas",
			contentType: "application/json",
			body:        buildJSONPadded(tenantScanCap),
			wantStatus:  http.StatusOK,
			wantBody:    "",
		},
		{
			name:        "json one byte over cap is rejected with 413",
			method:      http.MethodPost,
			target:      "/api/v1/areas",
			contentType: "application/json",
			body:        buildJSONPadded(tenantScanCap + 1),
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantBody:    tenantSizeCapBody,
		},
		{
			// Body is exactly cap+1: "x=" (2 bytes) + cap-1 padding 'a's.
			name:        "form-encoded one byte over cap is rejected with 413",
			method:      http.MethodPost,
			target:      "/api/v1/areas",
			contentType: "application/x-www-form-urlencoded",
			body:        append([]byte("x="), bytes.Repeat([]byte("a"), tenantScanCap-1)...),
			wantStatus:  http.StatusRequestEntityTooLarge,
			wantBody:    tenantSizeCapBody,
		},
		{
			name:        "multipart well over cap with no tenant_id is allowed (skip)",
			method:      http.MethodPost,
			target:      "/api/v1/files",
			contentType: multipartCleanCT,
			body:        multipartClean,
			wantStatus:  http.StatusOK,
			wantBody:    "",
		},
		{
			name:        "multipart well over cap with tenant_id field name is allowed (skip)",
			method:      http.MethodPost,
			target:      "/api/v1/files",
			contentType: multipartWithTenantCT,
			body:        multipartWithTenant,
			wantStatus:  http.StatusOK,
			wantBody:    "",
		},
		{
			name:        "json with tenant_id under cap is still rejected with 403",
			method:      http.MethodPost,
			target:      "/api/v1/areas",
			contentType: "application/json",
			body:        []byte(`{"tenant_id":"x"}`),
			wantStatus:  http.StatusForbidden,
			wantBody:    tenantViolationBody,
		},
		{
			name:        "GET with huge body is passed through (method gate)",
			method:      http.MethodGet,
			target:      "/api/v1/areas",
			contentType: "application/json",
			body:        buildJSONPadded(tenantScanCap + 1),
			wantStatus:  http.StatusOK,
			wantBody:    "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			req := httptest.NewRequest(tc.method, tc.target, bytes.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, tc.wantStatus)
			c.Assert(rr.Body.String(), qt.Equals, tc.wantBody)
		})
	}
}

// TestValidateNoUserProvidedTenantID_RejectTenantBody_HugeBodyNoOOM proves
// the LimitReader actually bounds the read: sending a 10 MiB non-multipart
// body must complete with a 413 response and no allocation blow-up. The
// fact that this test runs in a normal unit-test environment without
// exhausting memory is the proof.
func TestValidateNoUserProvidedTenantID_RejectTenantBody_HugeBodyNoOOM(t *testing.T) {
	c := qt.New(t)

	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiserver.ValidateNoUserProvidedTenantID(false)(downstream)

	const giant = 10 * 1024 * 1024 // 10 MiB
	body := bytes.Repeat([]byte("a"), giant)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/areas", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusRequestEntityTooLarge)
}

// TestValidateNoUserProvidedTenantID_TestTenantHeaderExemption covers the
// #1851 e2e gate. With the exemption enabled, the single namespaced
// X-Inventario-Test-Tenant header passes the rejectTenantHeader scan —
// every other tenant-named header still fails closed. With the
// exemption disabled (the production default), even the namespaced
// header is rejected. This is the gate the cross-tenant Playwright
// fixture relies on to drive callbacks under a chosen tenant.
func TestValidateNoUserProvidedTenantID_TestTenantHeaderExemption(t *testing.T) {
	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name            string
		allowTestHeader bool
		headerName      string
		headerValue     string
		wantStatus      int
	}{
		{
			name:            "namespaced test header allowed when gate is on",
			allowTestHeader: true,
			headerName:      apiserver.TestTenantHeaderName,
			headerValue:     "tenant2",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "namespaced test header rejected when gate is off",
			allowTestHeader: false,
			headerName:      apiserver.TestTenantHeaderName,
			headerValue:     "tenant2",
			wantStatus:      http.StatusForbidden,
		},
		{
			name:            "generic X-Tenant-ID rejected even when gate is on",
			allowTestHeader: true,
			headerName:      "X-Tenant-ID",
			headerValue:     "tenant2",
			wantStatus:      http.StatusForbidden,
		},
		{
			name:            "any other tenant-named header rejected even when gate is on",
			allowTestHeader: true,
			headerName:      "X-My-Tenant-Override",
			headerValue:     "tenant2",
			wantStatus:      http.StatusForbidden,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			handler := apiserver.ValidateNoUserProvidedTenantID(tc.allowTestHeader)(downstream)

			req := httptest.NewRequest(http.MethodGet, "/api/v1/auth/oauth/providers", nil)
			req.Header.Set(tc.headerName, tc.headerValue)
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, tc.wantStatus)
		})
	}
}
