package apiserver_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/apiserver"
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
	handler := apiserver.ValidateNoUserProvidedTenantID()(downstream)

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

// TestValidateNoUserProvidedTenantID_AdminSubtreeBodyCheckEnforced verifies
// that the request-body "tenant_id" check stays in force for admin paths —
// the #1748 exemption relaxes the query-parameter check ONLY.
func TestValidateNoUserProvidedTenantID_AdminSubtreeBodyCheckEnforced(t *testing.T) {
	c := qt.New(t)

	downstream := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	handler := apiserver.ValidateNoUserProvidedTenantID()(downstream)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/groups",
		strings.NewReader(`{"tenant_id":"tenant-xyz"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	c.Assert(rr.Code, qt.Equals, http.StatusForbidden)
}
