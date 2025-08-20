package apiserver

import (
	"net/http"
)

// SimpleTenantResolver resolves tenant from X-Tenant-ID header with fallback
type SimpleTenantResolver struct {
	DefaultTenantID string
}

// NewSimpleTenantResolver creates a new simple tenant resolver with fallback
func NewSimpleTenantResolver(defaultTenantID string) *SimpleTenantResolver {
	return &SimpleTenantResolver{
		DefaultTenantID: defaultTenantID,
	}
}

// ResolveTenant extracts tenant ID from X-Tenant-ID header or returns default
func (s *SimpleTenantResolver) ResolveTenant(r *http.Request) (string, error) {
	tenantID := r.Header.Get("X-Tenant-ID")
	if tenantID == "" {
		// Return default tenant ID for backward compatibility
		if s.DefaultTenantID != "" {
			return s.DefaultTenantID, nil
		}
		return "", ErrTenantNotFound
	}
	return tenantID, nil
}

// SimpleUserResolver resolves user from X-User-ID header with fallback
type SimpleUserResolver struct {
	DefaultUserID string
}

// NewSimpleUserResolver creates a new simple user resolver with fallback
func NewSimpleUserResolver(defaultUserID string) *SimpleUserResolver {
	return &SimpleUserResolver{
		DefaultUserID: defaultUserID,
	}
}

// ResolveUser extracts user ID from X-User-ID header or returns default
func (s *SimpleUserResolver) ResolveUser(r *http.Request) (string, error) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		// Return default user ID for backward compatibility
		if s.DefaultUserID != "" {
			return s.DefaultUserID, nil
		}
		return "", ErrUserNotFound
	}
	return userID, nil
}
