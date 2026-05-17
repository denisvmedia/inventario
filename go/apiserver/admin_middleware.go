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
			slog.Warn("RequireSystemAdmin: no user in context — middleware chain misconfigured",
				"path", r.URL.Path)
			_ = unauthorizedError(w, r, ErrNotSystemAdmin)
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
