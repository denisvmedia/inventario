package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/services"
)

const (
	// csrfHeaderName is the header name used to carry the CSRF token in requests
	// and to expose it in responses (e.g. from GET /auth/me).
	csrfHeaderName = "X-CSRF-Token"
)

// CSRFMiddleware validates CSRF tokens for state-changing HTTP requests.
//
// The middleware relies on the authenticated user being present in the request
// context (set by JWTMiddleware). Safe methods (GET, HEAD, OPTIONS) are always
// allowed without a CSRF token.
//
// Passing a nil csrfService disables CSRF validation entirely; this is
// intended only for test environments.
//
// On backend errors the middleware fails open: a Redis/storage outage must
// not take down the API. Operators should monitor CSRF service errors and
// ensure backend availability.
func CSRFMiddleware(csrfService services.CSRFService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// A nil service disables CSRF protection (test/dev shortcut).
			if csrfService == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Safe methods do not require CSRF protection.
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
				return
			}

			user := appctx.UserFromContext(r.Context())
			if user == nil {
				// No authenticated user in context — let downstream auth
				// middleware return the appropriate 401.
				next.ServeHTTP(w, r)
				return
			}

			tokenFromHeader := r.Header.Get(csrfHeaderName)
			if tokenFromHeader == "" {
				slog.Warn("CSRF token missing in state-changing request",
					"method", r.Method,
					"path", r.URL.Path,
					"user_id", user.ID,
				)
				http.Error(w, "CSRF token required", http.StatusForbidden)
				return
			}

			storedToken, err := csrfService.GetToken(r.Context(), user.ID)
			if err != nil {
				// Fail-open: a storage outage must not block all writes.
				slog.Error("CSRF service error; allowing request (fail-open)",
					"error", err,
					"user_id", user.ID,
					"method", r.Method,
					"path", r.URL.Path,
				)
				next.ServeHTTP(w, r)
				return
			}

			if storedToken == "" {
				slog.Warn("No CSRF token found for user; session may have expired",
					"method", r.Method,
					"path", r.URL.Path,
					"user_id", user.ID,
				)
				http.Error(w, "CSRF token invalid or expired", http.StatusForbidden)
				return
			}

			if tokenFromHeader != storedToken {
				slog.Warn("CSRF token mismatch",
					"method", r.Method,
					"path", r.URL.Path,
					"user_id", user.ID,
				)
				http.Error(w, "CSRF token mismatch", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
