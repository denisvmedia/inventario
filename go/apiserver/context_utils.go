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

// ExtractUserFromRequest extracts user ID from request context
// This function replaces ExtractTenantUserFromRequest for user-only authentication
func ExtractUserFromRequest(r *http.Request) string {
	user := GetUserFromRequest(r)
	if user != nil {
		return user.ID
	}
	return ""
}

// ExtractTenantUserFromRequest extracts user ID from request context for backward compatibility
// Deprecated: Use ExtractUserFromRequest instead
func ExtractTenantUserFromRequest(r *http.Request) (tenantID, userID string) {
	user := GetUserFromRequest(r)
	if user != nil {
		// In user-only mode, we use the user's tenant_id for backward compatibility
		// but the primary identifier is the user_id
		return user.TenantID, user.ID
	}

	// Fallback to default IDs for backward compatibility during transition
	SetDefaultTenantUserIDs(&tenantID, &userID)
	return tenantID, userID
}

// ExtractUserFromContext extracts user ID from context
// This function replaces ExtractTenantUserFromContext for user-only authentication
func ExtractUserFromContext(ctx context.Context) string {
	user := UserFromContext(ctx)
	if user != nil {
		return user.ID
	}
	return ""
}

// ExtractTenantUserFromContext extracts user ID from context for backward compatibility
// Deprecated: Use ExtractUserFromContext instead
func ExtractTenantUserFromContext(ctx context.Context) (tenantID, userID string) {
	user := UserFromContext(ctx)
	if user != nil {
		// In user-only mode, we use the user's tenant_id for backward compatibility
		// but the primary identifier is the user_id
		return user.TenantID, user.ID
	}

	// Fallback to default IDs for backward compatibility during transition
	SetDefaultTenantUserIDs(&tenantID, &userID)
	return tenantID, userID
}
