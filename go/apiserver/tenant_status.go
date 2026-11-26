package apiserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// tenantSuspendedMessage is the body of the 403 a request gets when the
// tenant behind it is not active. PublicTenantMiddleware writes the same one.
const tenantSuspendedMessage = "Tenant suspended"

// userTenantIsActive reports whether the user's tenant is active.
//
// PublicTenantMiddleware refuses every request whose host belongs to a
// tenant that is not active. That alone covers the password and OAuth
// sign-ins, which only find a user of the host's tenant, and API tokens,
// which only authenticate on their own tenant's host. Other credentials are
// not tied to a host: an access token, a refresh cookie or an MFA challenge
// minted on one tenant's host can be sent to another's, and a magic link
// outlives a move of its user to another tenant. This check closes that gap
// by looking at the user's own tenant. When the request resolved to that
// tenant, the context already holds it; otherwise the tenant row is read.
//
// A nil registry answers true: only tests that do not wire one get there.
func userTenantIsActive(ctx context.Context, tenants registry.TenantRegistry, user *models.User) (bool, error) {
	if resolved := TenantFromContext(ctx); resolved != nil && resolved.ID == user.TenantID {
		return resolved.Status == models.TenantStatusActive, nil
	}
	if tenants == nil {
		return true, nil
	}
	tenant, err := tenants.Get(ctx, user.TenantID)
	if errors.Is(err, registry.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return tenant.Status == models.TenantStatusActive, nil
}

// refuseInactiveUserTenant writes a 403 "Tenant suspended" and returns true
// when the user's tenant is not active, and a 500 when that cannot be told.
// It returns false, writing nothing, when the request may go on.
func refuseInactiveUserTenant(w http.ResponseWriter, r *http.Request, tenants registry.TenantRegistry, user *models.User) bool {
	active, err := userTenantIsActive(r.Context(), tenants, user)
	if err != nil {
		slog.Error("Failed to check the user's tenant status", "user_id", user.ID, "tenant_id", user.TenantID, "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return true
	}
	if !active {
		slog.Warn("Refused a request for a user whose tenant is not active",
			"user_id", user.ID, "tenant_id", user.TenantID, "method", r.Method, "path", r.URL.Path)
		http.Error(w, tenantSuspendedMessage, http.StatusForbidden)
		return true
	}
	return false
}

// requireActiveUserTenant refuses a request whose authenticated user belongs
// to a tenant that is not active. It goes after the middleware that puts the
// user in the context and passes a request without a user through, leaving
// that refusal to the authentication middleware.
func requireActiveUserTenant(tenants registry.TenantRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if user := appctx.UserFromContext(r.Context()); user != nil {
				if refuseInactiveUserTenant(w, r, tenants, user) {
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
