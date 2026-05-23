package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/registry"
)

// adminForbiddenCode is the JSON:API error code emitted by RequireSystemAdmin
// when the caller lacks platform-admin privileges. Kept as a constant so
// future admin endpoints can re-use the same wire code without duplicating
// the literal.
const adminForbiddenCode = "admin.forbidden"

// RequireSystemAdmin gates a route subtree on the presence of a row in
// `system_admin_grants` for the authenticated user (#1784). It MUST run
// after JWTMiddleware (which populates the user-in-context). Every
// handler behind this middleware is allowed to assume the caller is a
// genuine system admin.
//
// The registry argument is required (panics at startup if nil) — the
// previous static `RequireSystemAdmin(next)` shape that read
// `user.IsSystemAdmin` is gone: the privilege now lives outside the
// users row and the gate has to actually hit the grant store on every
// admin request. The lookup is O(1) thanks to the unique index on
// user_id.
//
// On registry error the middleware fails CLOSED with 500 — a transient
// DB error must not silently 403 every admin request (an admin who
// can't reach their admin surface won't know the difference between
// "revoked" and "DB down"). The 500 is emitted via
// internalServerError so the operator-side log line carries the
// underlying error.
//
// Logs at Warn on every block so a misconfigured FE that probes admin
// endpoints from a regular session is visible in operator logs.
func RequireSystemAdmin(grants registry.SystemAdminGrantRegistry) func(http.Handler) http.Handler {
	if grants == nil {
		panic("apiserver.RequireSystemAdmin requires non-nil SystemAdminGrantRegistry")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := appctx.UserFromContext(r.Context())
			if user == nil {
				// JWTMiddleware should have populated this. Reaching here means
				// the middleware chain is wired wrong — fail closed with 401
				// rather than 403 so the operator notices the misconfiguration
				// (a missing user is "not authenticated", not "not authorized").
				slog.Warn("RequireSystemAdmin: no user in context — middleware chain misconfigured",
					"path", r.URL.Path)
				_ = unauthorizedError(w, r, ErrMissingUserContext)
				return
			}
			ok, err := grants.Exists(r.Context(), user.ID)
			if err != nil {
				slog.Error("RequireSystemAdmin: grant lookup failed",
					"user_id", user.ID,
					"path", r.URL.Path,
					"error", err,
				)
				_ = internalServerError(w, r, err)
				return
			}
			if !ok {
				slog.Warn("RequireSystemAdmin: access denied",
					"user_id", user.ID,
					"tenant_id", user.TenantID,
					"path", r.URL.Path,
				)
				_ = codedForbiddenError(w, r, ErrNotSystemAdmin, adminForbiddenCode)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireSystemAdminOrImpersonating gates the JWT-middleware-backed
// impersonation lifecycle endpoints — currently only GET
// /admin/impersonation/current (see adminAuthenticatedRoutes). It does
// NOT gate POST /admin/impersonation/end: that endpoint is mounted bare,
// outside the JWT/middleware chain, and self-validates the impersonation
// token and marker cookie itself (see Admin() and endImpersonation).
//
// It admits two callers:
//
//  1. A genuine system admin — the operator who has not yet started (or
//     has already ended) an impersonation session. Verified by looking
//     up a row in `system_admin_grants` (#1784).
//  2. A request running inside an impersonation session — the access
//     token carries `imp=true`. Such a token deliberately has
//     `is_system_admin=false` (an impersonated session must never wield
//     platform-admin authority), so plain RequireSystemAdmin would reject
//     it — yet the operator must still be able to read the impersonation
//     state of the very session they started. Admitting impersonation
//     tokens here lets `current` work while keeping every state-changing
//     admin endpoint behind the strict RequireSystemAdmin gate.
//
// Like RequireSystemAdmin it MUST run after JWTMiddleware. The handler
// behind it re-validates the impersonation claim itself, so this
// middleware only widens the gate — it does not weaken any handler-side
// check.
func RequireSystemAdminOrImpersonating(grants registry.SystemAdminGrantRegistry) func(http.Handler) http.Handler {
	if grants == nil {
		panic("apiserver.RequireSystemAdminOrImpersonating requires non-nil SystemAdminGrantRegistry")
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := appctx.UserFromContext(r.Context())
			if user == nil {
				slog.Warn("RequireSystemAdminOrImpersonating: no user in context — middleware chain misconfigured",
					"path", r.URL.Path)
				_ = unauthorizedError(w, r, ErrMissingUserContext)
				return
			}
			if isImpersonatedRequest(r.Context()) {
				next.ServeHTTP(w, r)
				return
			}
			ok, err := grants.Exists(r.Context(), user.ID)
			if err != nil {
				slog.Error("RequireSystemAdminOrImpersonating: grant lookup failed",
					"user_id", user.ID,
					"path", r.URL.Path,
					"error", err,
				)
				_ = internalServerError(w, r, err)
				return
			}
			if ok {
				next.ServeHTTP(w, r)
				return
			}
			slog.Warn("RequireSystemAdminOrImpersonating: access denied",
				"user_id", user.ID,
				"tenant_id", user.TenantID,
				"path", r.URL.Path,
			)
			_ = codedForbiddenError(w, r, ErrNotSystemAdmin, adminForbiddenCode)
		})
	}
}
