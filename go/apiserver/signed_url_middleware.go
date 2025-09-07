package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// SignedURLMiddleware creates middleware that validates signed URLs for file access
// This middleware replaces JWT authentication for file downloads to prevent token exposure
func SignedURLMiddleware(fileSigningService *services.FileSigningService, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only validate signed URLs for GET requests (file downloads)
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed for signed URLs", http.StatusMethodNotAllowed)
				return
			}

			// Validate the signed URL
			claims, err := fileSigningService.ValidateSignedURL(r.URL.Path, r.URL.Query())
			if err != nil {
				slog.Warn("Invalid signed URL access attempt",
					"path", r.URL.Path,
					"query", r.URL.RawQuery,
					"error", err.Error(),
					"remote_addr", r.RemoteAddr,
					"user_agent", r.UserAgent())
				http.Error(w, "Invalid or expired file URL", http.StatusUnauthorized)
				return
			}

			// Validate that the user still exists and is active
			user, err := userRegistry.Get(r.Context(), claims.UserID)
			if err != nil {
				slog.Warn("Signed URL access attempt with invalid user",
					"user_id", claims.UserID,
					"file_id", claims.FileID,
					"error", err.Error(),
					"remote_addr", r.RemoteAddr)
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}

			if !user.IsActive {
				slog.Warn("Signed URL access attempt by inactive user",
					"user_id", claims.UserID,
					"file_id", claims.FileID,
					"user_email", user.Email,
					"remote_addr", r.RemoteAddr)
				http.Error(w, "User account disabled", http.StatusForbidden)
				return
			}

			// Add user to context for downstream handlers
			ctx := appctx.WithUser(r.Context(), user)

			// Log successful file access for security monitoring
			slog.Debug("Signed URL file access granted",
				"user_id", user.ID,
				"user_email", user.Email,
				"file_id", claims.FileID,
				"expires_at", claims.ExpiresAt,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
