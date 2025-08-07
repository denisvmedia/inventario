package apiserver

import (
	"context"
	"net/http"
	"strings"

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

// HeaderTenantResolver resolves tenant from X-Tenant-ID header
type HeaderTenantResolver struct{}

// ResolveTenant extracts tenant ID from X-Tenant-ID header
func (h *HeaderTenantResolver) ResolveTenant(r *http.Request) (string, error) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		return "", ErrTenantNotFound
	}
	return tenantID, nil
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
func TenantMiddleware(resolver TenantResolver, tenantRegistry registry.TenantRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Resolve tenant ID from request
			tenantID, err := resolver.ResolveTenant(r)
			if err != nil {
				http.Error(w, "Tenant not found", http.StatusBadRequest)
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
