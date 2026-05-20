package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/denisvmedia/inventario/appctx"
)

// adminForbiddenCode is the JSON:API error code emitted by RequireSystemAdmin
// when the caller lacks platform-admin privileges. Kept as a constant so
// future admin endpoints can re-use the same wire code without duplicating
// the literal.
const adminForbiddenCode = "admin.forbidden"

// RequireSystemAdmin gates a route subtree on models.User.IsSystemAdmin.
// It MUST run after JWTMiddleware (which populates the user-in-context);
// the JSON:API 403 it emits when the user lacks the flag is the only
// response a non-admin will ever see from /api/v1/admin/* — every handler
// behind this middleware is allowed to assume the caller is a system admin.
//
// Logs at Warn level on every block so a misconfigured FE that probes admin
// endpoints from a regular session is visible in operator logs. The log
// line includes user_id + path so a real privilege-probe can be
// distinguished from FE drift.
func RequireSystemAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := appctx.UserFromContext(r.Context())
		if user == nil {
			// JWTMiddleware should have populated this. Reaching here means
			// the middleware chain is wired wrong — fail closed with 401
			// rather than 403 so the operator notices the misconfiguration
			// (a missing user is "not authenticated", not "not authorized").
			// Use ErrMissingUserContext rather than ErrNotSystemAdmin so the
			// 401 path doesn't surface "admin privileges required" copy for
			// what is fundamentally an auth-wiring problem.
			slog.Warn("RequireSystemAdmin: no user in context — middleware chain misconfigured",
				"path", r.URL.Path)
			_ = unauthorizedError(w, r, ErrMissingUserContext)
			return
		}
		if !user.IsSystemAdmin {
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
//     has already ended) an impersonation session.
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
func RequireSystemAdminOrImpersonating(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := appctx.UserFromContext(r.Context())
		if user == nil {
			slog.Warn("RequireSystemAdminOrImpersonating: no user in context — middleware chain misconfigured",
				"path", r.URL.Path)
			_ = unauthorizedError(w, r, ErrMissingUserContext)
			return
		}
		if user.IsSystemAdmin || isImpersonatedRequest(r.Context()) {
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
