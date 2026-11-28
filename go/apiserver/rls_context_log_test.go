package apiserver_test

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/apiserver"
	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
)

// The RLS middleware's refusals name the user by id. The address is for
// sign-in, registration and recovery lines only (devdocs/logging.md).
func TestRLSContextMiddleware_RefusalLeavesTheAddressOut(t *testing.T) {
	const address = "rls-refused@example.com"
	for _, tc := range []struct {
		name   string
		user   *models.User
		status int
		msg    string
	}{
		{
			name: "user without an id",
			user: &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1"},
				Email:               address,
				IsActive:            true,
			},
			status: http.StatusUnauthorized,
			msg:    "RLS Security Violation: User ID is empty",
		},
		{
			name: "user without a tenant",
			user: &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "user-1"}},
				Email:               address,
				IsActive:            true,
			},
			status: http.StatusUnauthorized,
			msg:    "RLS Security Violation: Tenant ID is empty",
		},
		{
			name: "inactive user",
			user: &models.User{
				TenantAwareEntityID: models.TenantAwareEntityID{TenantID: "tenant-1", EntityID: models.EntityID{ID: "user-1"}},
				Email:               address,
			},
			status: http.StatusForbidden,
			msg:    "RLS Security Violation: Inactive user attempted access",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			var buf bytes.Buffer
			prev := slog.Default()
			slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
			c.Cleanup(func() { slog.SetDefault(prev) })

			next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
				c.Error("the refused request reached the handler")
			})
			req := httptest.NewRequest(http.MethodGet, "/api/v1/g/home/locations", nil)
			req = req.WithContext(appctx.WithUser(req.Context(), tc.user))
			rr := httptest.NewRecorder()
			apiserver.RLSContextMiddleware(nil)(next).ServeHTTP(rr, req)

			c.Assert(rr.Code, qt.Equals, tc.status)
			c.Assert(buf.String(), qt.Contains, tc.msg)
			c.Assert(buf.String(), qt.Not(qt.Contains), address)
		})
	}
}
