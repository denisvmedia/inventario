package apiserver

import (
	"log/slog"
	"net/http"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// SignedURLMiddleware creates middleware that validates signed URLs for file access.
// This middleware replaces JWT authentication for file downloads to prevent token
// exposure. It also stamps the file's group onto the context: downstream handlers
// call FileRegistryFactory.CreateUserRegistry which filters by group_id — without
// the group on context, that query matches no rows even for files the signed URL
// legitimately grants access to.
func SignedURLMiddleware(
	fileSigningService *services.FileSigningService,
	userRegistry registry.UserRegistry,
	fileRegistryFactory registry.FileRegistryFactory,
	groupRegistry registry.LocationGroupRegistry,
) func(http.Handler) http.Handler {
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

			// Add user to context for downstream handlers first — the
			// service-mode file lookup below doesn't need it, but the
			// downstream file registry / streaming code does.
			ctx := appctx.WithUser(r.Context(), user)

			// Resolve the file's group and stamp it on the context. The
			// service-mode file registry bypasses tenant/group filtering,
			// which is safe here because the signed URL's HMAC has already
			// authorised access to this specific file for this specific
			// user; we just need to figure out which group to scope
			// downstream registries to.
			fileServiceReg := fileRegistryFactory.CreateServiceRegistry()
			file, err := fileServiceReg.Get(ctx, claims.FileID)
			if err != nil {
				slog.Warn("Signed URL access attempt for missing file",
					"user_id", user.ID,
					"file_id", claims.FileID,
					"error", err.Error(),
					"remote_addr", r.RemoteAddr)
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}
			// Defence in depth: reject signed URLs pointing at a file that
			// belongs to a different tenant than the signed-in user. The
			// HMAC already scopes to user+file, so this is only reachable
			// if someone manages to forge a collision — fail closed.
			if file.TenantID != user.TenantID {
				slog.Warn("Signed URL cross-tenant access attempt",
					"user_id", user.ID,
					"file_id", claims.FileID,
					"file_tenant_id", file.TenantID,
					"user_tenant_id", user.TenantID,
					"remote_addr", r.RemoteAddr)
				http.Error(w, "File not found", http.StatusNotFound)
				return
			}

			if file.GroupID != "" {
				group, err := groupRegistry.Get(ctx, file.GroupID)
				if err != nil {
					slog.Warn("Signed URL access attempt for file whose group cannot be loaded",
						"user_id", user.ID,
						"file_id", claims.FileID,
						"group_id", file.GroupID,
						"error", err.Error(),
						"remote_addr", r.RemoteAddr)
					http.Error(w, "File not found", http.StatusNotFound)
					return
				}
				ctx = appctx.WithGroup(ctx, group)
			}

			// Log successful file access for security monitoring
			slog.Debug("Signed URL file access granted",
				"user_id", user.ID,
				"user_email", user.Email,
				"file_id", claims.FileID,
				"group_id", file.GroupID,
				"expires_at", claims.ExpiresAt,
				"path", r.URL.Path,
				"remote_addr", r.RemoteAddr)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
