package apiserver

import (
	"context"
	"net/http"

	"github.com/denisvmedia/inventario/models"
)

// GetTenantIDFromRequest extracts tenant ID from request context
// Returns empty string if no tenant context is available
func GetTenantIDFromRequest(r *http.Request) string {
	return TenantIDFromContext(r.Context())
}

// GetUserIDFromRequest extracts user ID from request context
// Returns empty string if no user context is available
func GetUserIDFromRequest(r *http.Request) string {
	return UserIDFromContext(r.Context())
}

// GetTenantFromRequest extracts tenant from request context
// Returns nil if no tenant context is available
func GetTenantFromRequest(r *http.Request) *models.Tenant {
	return TenantFromContext(r.Context())
}

// GetUserFromRequest extracts user from request context
// Returns nil if no user context is available
func GetUserFromRequest(r *http.Request) *models.User {
	return UserFromContext(r.Context())
}

// GetTenantIDFromContext extracts tenant ID from context
// Returns empty string if no tenant context is available
func GetTenantIDFromContext(ctx context.Context) string {
	return TenantIDFromContext(ctx)
}

// GetUserIDFromContext extracts user ID from context
// Returns empty string if no user context is available
func GetUserIDFromContext(ctx context.Context) string {
	return UserIDFromContext(ctx)
}

// SetDefaultTenantUserIDs sets default tenant and user IDs if they are empty
// This is a temporary function to maintain backward compatibility
// TODO: Remove this when proper tenant/user context is fully implemented
func SetDefaultTenantUserIDs(tenantID, userID *string) {
	if *tenantID == "" {
		*tenantID = "test-tenant-id" // Use the same ID as our tests and seeding
	}
	if *userID == "" {
		*userID = "test-user-id" // Use the same ID as our tests and seeding
	}
}

// ExtractTenantUserFromRequest extracts tenant and user IDs from request context
// Falls back to default IDs if context is not available (for backward compatibility)
// TODO: Remove fallback when proper authentication is fully implemented
func ExtractTenantUserFromRequest(r *http.Request) (tenantID, userID string) {
	tenantID = GetTenantIDFromRequest(r)
	userID = GetUserIDFromRequest(r)
	
	// Fallback to default IDs for backward compatibility
	SetDefaultTenantUserIDs(&tenantID, &userID)
	
	return tenantID, userID
}

// ExtractTenantUserFromContext extracts tenant and user IDs from context
// Falls back to default IDs if context is not available (for backward compatibility)
// TODO: Remove fallback when proper authentication is fully implemented
func ExtractTenantUserFromContext(ctx context.Context) (tenantID, userID string) {
	tenantID = GetTenantIDFromContext(ctx)
	userID = GetUserIDFromContext(ctx)
	
	// Fallback to default IDs for backward compatibility
	SetDefaultTenantUserIDs(&tenantID, &userID)
	
	return tenantID, userID
}
