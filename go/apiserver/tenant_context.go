package apiserver

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	tenantCtxKey ctxValueKey = "tenant"
	tenantIDKey  ctxValueKey = "tenantID"
)

// TenantResolver interface defines methods for resolving tenant from HTTP requests
type TenantResolver interface {
	ResolveTenant(r *http.Request) (string, error)
}

// JWTTenantResolver resolves tenant from authenticated JWT token only
type JWTTenantResolver struct{}

// ResolveTenant extracts tenant ID from authenticated user context only
func (j *JWTTenantResolver) ResolveTenant(r *http.Request) (string, error) {
	// Extract tenant from authenticated user context, never from user input
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		return "", ErrTenantNotFound
	}

	// TODO: When multi-tenancy is fully implemented, get from user.TenantID
	// For now, derive tenant from user context or use default
	if user.TenantID != "" {
		return user.TenantID, nil
	}

	// Fallback to default tenant during transition period
	return "default-tenant", nil
}

// SubdomainTenantResolver resolves tenant from subdomain
type SubdomainTenantResolver struct {
	BaseDomain string // e.g., "inventario.com"
}

// ResolveTenant extracts tenant slug from subdomain
func (s *SubdomainTenantResolver) ResolveTenant(r *http.Request) (string, error) {
	host := r.Host
	if host == "" {
		return "", ErrTenantNotFound
	}

	// Remove port if present
	if colonIndex := strings.Index(host, ":"); colonIndex != -1 {
		host = host[:colonIndex]
	}

	// Check if it's a subdomain of our base domain
	if s.BaseDomain != "" && strings.HasSuffix(host, "."+s.BaseDomain) {
		// Extract subdomain
		subdomain := strings.TrimSuffix(host, "."+s.BaseDomain)
		if subdomain == "" || subdomain == "www" {
			return "", ErrTenantNotFound
		}
		return subdomain, nil
	}

	// If no base domain specified, treat the entire host as tenant slug
	// This is useful for development or when using custom domains
	if s.BaseDomain == "" {
		return host, nil
	}

	return "", ErrTenantNotFound
}

// TenantFromContext retrieves the tenant from the request context
func TenantFromContext(ctx context.Context) *models.Tenant {
	tenant, ok := ctx.Value(tenantCtxKey).(*models.Tenant)
	if !ok {
		return nil
	}
	return tenant
}

// TenantIDFromContext retrieves the tenant ID from the request context
func TenantIDFromContext(ctx context.Context) string {
	tenantID, ok := ctx.Value(tenantIDKey).(string)
	if !ok {
		return ""
	}
	return tenantID
}

// WithTenant adds a tenant to the context
func WithTenant(ctx context.Context, tenant *models.Tenant) context.Context {
	ctx = context.WithValue(ctx, tenantCtxKey, tenant)
	ctx = context.WithValue(ctx, tenantIDKey, tenant.ID)
	return ctx
}

// WithTenantID adds a tenant ID to the context
func WithTenantID(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantIDKey, tenantID)
}

// TenantMiddleware creates middleware that resolves and validates tenant context
// This version includes security validation to ensure users can only access their own tenant
func TenantMiddleware(resolver TenantResolver, tenantRegistry registry.TenantRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// SECURITY: Get authenticated user first
			user := appctx.UserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			// Resolve tenant ID from authenticated context only (never from user input)
			tenantID, err := resolver.ResolveTenant(r)
			if err != nil {
				http.Error(w, "Tenant not found", http.StatusBadRequest)
				return
			}

			// SECURITY: Validate that the authenticated user belongs to this tenant
			if user.TenantID != "" && user.TenantID != tenantID {
				// Log security violation
				slog.Error("Security violation: user attempted to access different tenant",
					"user_id", user.ID,
					"user_tenant", user.TenantID,
					"requested_tenant", tenantID,
					"user_agent", r.UserAgent(),
					"remote_addr", r.RemoteAddr,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Unauthorized: access denied", http.StatusForbidden)
				return
			}

			// Get tenant from registry
			tenant, err := tenantRegistry.Get(r.Context(), tenantID)
			if err != nil {
				http.Error(w, "Invalid tenant", http.StatusUnauthorized)
				return
			}

			// Check tenant status
			if tenant.Status != models.TenantStatusActive {
				http.Error(w, "Tenant suspended", http.StatusForbidden)
				return
			}

			// Add tenant to context
			ctx := WithTenant(r.Context(), tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireTenant middleware ensures that a tenant is present in the context
func RequireTenant() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := TenantFromContext(r.Context())
			if tenant == nil {
				http.Error(w, "Tenant context required", http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// TenantAwareMiddleware creates middleware that adds tenant context for tenant-aware operations
// This is a lighter version that only adds tenant ID to context without full validation
func TenantAwareMiddleware(resolver TenantResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to resolve tenant ID from request
			tenantID, err := resolver.ResolveTenant(r)
			if err != nil {
				// If no tenant can be resolved, continue without tenant context
				// This allows for backward compatibility with non-tenant-aware endpoints
				next.ServeHTTP(w, r)
				return
			}

			// Add tenant ID to context
			ctx := WithTenantID(r.Context(), tenantID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// HostTenantResolver resolves the tenant from the HTTP request Host header.
// In single-tenant mode (BaseDomain is empty), it returns an empty slug as a
// signal that the single available tenant should be used.
// In multi-tenant mode it extracts the subdomain and uses it as the tenant slug.
type HostTenantResolver struct {
	BaseDomain string // optional; e.g. "inventario.com"; empty = single-tenant mode
}

// ResolveTenant returns the tenant slug derived from the request host.
// An empty string signals single-tenant mode (the caller should pick the one tenant).
func (h *HostTenantResolver) ResolveTenant(r *http.Request) (string, error) {
	if h.BaseDomain == "" {
		return "", nil // single-tenant: middleware picks the one tenant from the registry
	}

	host := r.Host
	if idx := strings.Index(host, ":"); idx != -1 {
		host = host[:idx]
	}

	if !strings.HasSuffix(host, "."+h.BaseDomain) {
		return "", nil // root domain or unrelated host → single-tenant fallback
	}

	subdomain := strings.TrimSuffix(host, "."+h.BaseDomain)
	if subdomain == "" || subdomain == "www" {
		return "", nil
	}

	return subdomain, nil
}

// PublicTenantMiddleware resolves the tenant from the request and stores it in
// the context so that both public and authenticated handlers can call TenantIDFromContext.
// It does not require a JWT and is safe to place before auth middleware.
//
// When the resolver returns an empty slug (single-tenant mode), the middleware
// fetches the system-wide default tenant via GetDefault, which is enforced by a
// partial unique index to guarantee only one tenant can be marked as default.
func PublicTenantMiddleware(resolver TenantResolver, tenantRegistry registry.TenantRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug, err := resolver.ResolveTenant(r)
			if err != nil {
				slog.Error("Tenant resolution failed", "error", err, "host", r.Host)
				http.Error(w, "Tenant resolution failed", http.StatusServiceUnavailable)
				return
			}

			var tenant *models.Tenant
			if slug != "" {
				tenant, err = tenantRegistry.GetBySlug(r.Context(), slug)
				if err != nil {
					http.Error(w, "Tenant not found", http.StatusNotFound)
					return
				}
			} else {
				// Single-tenant mode: use the tenant marked as default in the database.
				tenant, err = tenantRegistry.GetDefault(r.Context())
				if err != nil {
					slog.Error("Failed to get default tenant", "error", err)
					http.Error(w, "No default tenant configured", http.StatusServiceUnavailable)
					return
				}
			}

			if tenant.Status != models.TenantStatusActive {
				http.Error(w, "Tenant suspended", http.StatusForbidden)
				return
			}

			ctx := WithTenant(r.Context(), tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
