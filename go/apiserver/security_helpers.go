package apiserver

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SecurityError represents different types of security violations
type SecurityError struct {
	Type    string
	Message string
	Code    int
}

var (
	ErrUserContextRequired = SecurityError{
		Type:    "user_context_required",
		Message: "User authentication required",
		Code:    http.StatusUnauthorized,
	}
	ErrUserContextInvalid = SecurityError{
		Type:    "user_context_invalid",
		Message: "Invalid user context",
		Code:    http.StatusUnauthorized,
	}
	ErrUserAccountDisabled = SecurityError{
		Type:    "user_account_disabled",
		Message: "User account disabled",
		Code:    http.StatusForbidden,
	}
	ErrInsufficientPermissions = SecurityError{
		Type:    "insufficient_permissions",
		Message: "Insufficient permissions for this operation",
		Code:    http.StatusForbidden,
	}
	ErrResourceNotFound = SecurityError{
		Type:    "resource_not_found",
		Message: "Resource not found",
		Code:    http.StatusNotFound,
	}
)

// ValidateUserContext validates that a user context is present and the user is active
func ValidateUserContext(r *http.Request) (*models.User, *SecurityError) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		logSecurityViolation(r, "missing_user_context", "No user context found in request")
		return nil, &ErrUserContextRequired
	}

	// Validate user has required fields for security
	if user.ID == "" {
		logSecurityViolation(r, "empty_user_id", "User ID is empty")
		return nil, &ErrUserContextInvalid
	}

	if user.TenantID == "" {
		logSecurityViolation(r, "empty_tenant_id", "Tenant ID is empty")
		return nil, &ErrUserContextInvalid
	}

	// Validate user is active
	if !user.IsActive {
		logSecurityViolation(r, "inactive_user", "Inactive user attempted access")
		return nil, &ErrUserAccountDisabled
	}

	return user, nil
}

// GetUserAwareCommodityRegistry returns a user-aware commodity registry
func GetUserAwareCommodityRegistry(r *http.Request, baseRegistry registry.CommodityRegistry) (registry.CommodityRegistry, *SecurityError) {
	// Validate user context first
	_, secErr := ValidateUserContext(r)
	if secErr != nil {
		return nil, secErr
	}

	userReg, err := baseRegistry.WithCurrentUser(r.Context())
	if err != nil {
		logSecurityViolation(r, "registry_context_error", "Failed to create user-aware commodity registry")
		return nil, &ErrUserContextInvalid
	}

	return userReg, nil
}

// GetUserAwareAreaRegistry returns a user-aware area registry
func GetUserAwareAreaRegistry(r *http.Request, baseRegistry registry.AreaRegistry) (registry.AreaRegistry, *SecurityError) {
	// Validate user context first
	_, secErr := ValidateUserContext(r)
	if secErr != nil {
		return nil, secErr
	}

	userReg, err := baseRegistry.WithCurrentUser(r.Context())
	if err != nil {
		logSecurityViolation(r, "registry_context_error", "Failed to create user-aware area registry")
		return nil, &ErrUserContextInvalid
	}

	return userReg, nil
}

// GetUserAwareLocationRegistry returns a user-aware location registry
func GetUserAwareLocationRegistry(r *http.Request, baseRegistry registry.LocationRegistry) (registry.LocationRegistry, *SecurityError) {
	// Validate user context first
	_, secErr := ValidateUserContext(r)
	if secErr != nil {
		return nil, secErr
	}

	userReg, err := baseRegistry.WithCurrentUser(r.Context())
	if err != nil {
		logSecurityViolation(r, "registry_context_error", "Failed to create user-aware location registry")
		return nil, &ErrUserContextInvalid
	}

	return userReg, nil
}

// GetUserAwareFileRegistry returns a user-aware file registry
func GetUserAwareFileRegistry(r *http.Request, baseRegistry registry.FileRegistry) (registry.FileRegistry, *SecurityError) {
	// Validate user context first
	_, secErr := ValidateUserContext(r)
	if secErr != nil {
		return nil, secErr
	}

	userReg, err := baseRegistry.WithCurrentUser(r.Context())
	if err != nil {
		logSecurityViolation(r, "registry_context_error", "Failed to create user-aware file registry")
		return nil, &ErrUserContextInvalid
	}

	return userReg, nil
}

// GetUserAwareExportRegistry returns a user-aware export registry
func GetUserAwareExportRegistry(r *http.Request, baseRegistry registry.ExportRegistry) (registry.ExportRegistry, *SecurityError) {
	// Validate user context first
	_, secErr := ValidateUserContext(r)
	if secErr != nil {
		return nil, secErr
	}

	userReg, err := baseRegistry.WithCurrentUser(r.Context())
	if err != nil {
		logSecurityViolation(r, "registry_context_error", "Failed to create user-aware export registry")
		return nil, &ErrUserContextInvalid
	}

	return userReg, nil
}

// HandleSecurityError renders a security error with consistent format and logging
func HandleSecurityError(w http.ResponseWriter, r *http.Request, secErr *SecurityError) {
	logSecurityViolation(r, secErr.Type, secErr.Message)

	// Return 404 for RLS violations and resource access issues as per project standards
	if secErr.Type == "resource_not_found" ||
		secErr.Type == "user_context_invalid" ||
		secErr.Code == http.StatusForbidden {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	// Return 403 for inactive users specifically
	if secErr.Type == "user_account_disabled" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	http.Error(w, secErr.Message, secErr.Code)
}

// ValidateEntityAccess validates that a user can access a specific entity
// This is a simplified version that works with the current type system
func ValidateEntityAccess(r *http.Request, entityID string, entityType string) *SecurityError {
	// Validate user context first
	_, secErr := ValidateUserContext(r)
	if secErr != nil {
		return secErr
	}

	// Basic validation - entity ID should not be empty
	if entityID == "" {
		logSecurityViolation(r, "invalid_entity_id", "Empty entity ID provided")
		return &SecurityError{
			Type:    "invalid_entity_id",
			Message: "Invalid entity identifier",
			Code:    http.StatusBadRequest,
		}
	}

	// Additional validation can be added here based on entity type
	// For now, we rely on the registry-level security checks
	return nil
}

// ValidateInputSanitization performs basic input validation and sanitization
func ValidateInputSanitization(r *http.Request, input any) *SecurityError {
	// Basic validation - can be extended based on specific needs
	if input == nil {
		logSecurityViolation(r, "invalid_input", "Null input provided")
		return &SecurityError{
			Type:    "invalid_input",
			Message: "Invalid input data",
			Code:    http.StatusBadRequest,
		}
	}

	// Add more specific validation based on input type
	// This is a placeholder for more comprehensive input validation
	return nil
}

// SecurityViolationContext provides additional context for security violations
type SecurityViolationContext struct {
	EntityID     string
	EntityType   string
	Operation    string
	ResourcePath string
	Headers      map[string]string
	Severity     string // "low", "medium", "high", "critical"
}

// logSecurityViolation logs security violations with consistent format
func logSecurityViolation(r *http.Request, violationType, message string) {
	logSecurityViolationWithContext(r, violationType, message, nil)
}

// logSecurityViolationWithContext logs security violations with additional context
func logSecurityViolationWithContext(r *http.Request, violationType, message string, ctx *SecurityViolationContext) {
	user := appctx.UserFromContext(r.Context())
	userID := ""
	tenantID := ""
	userEmail := ""

	if user != nil {
		userID = user.ID
		tenantID = user.TenantID
		userEmail = user.Email
	}

	// Determine severity based on violation type
	var severity string
	if ctx != nil && ctx.Severity != "" {
		severity = ctx.Severity
	} else {
		severity = determineSeverity(violationType)
	}

	logFields := []any{
		"violation_type", violationType,
		"message", message,
		"severity", severity,
		"user_id", userID,
		"tenant_id", tenantID,
		"user_email", userEmail,
		"method", r.Method,
		"path", r.URL.Path,
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
		"query_params", r.URL.RawQuery,
		"timestamp", "now",
	}

	// Add context-specific fields
	if ctx != nil {
		if ctx.EntityID != "" {
			logFields = append(logFields, "entity_id", ctx.EntityID)
		}
		if ctx.EntityType != "" {
			logFields = append(logFields, "entity_type", ctx.EntityType)
		}
		if ctx.Operation != "" {
			logFields = append(logFields, "operation", ctx.Operation)
		}
		if ctx.ResourcePath != "" {
			logFields = append(logFields, "resource_path", ctx.ResourcePath)
		}
		if len(ctx.Headers) > 0 {
			for key, value := range ctx.Headers {
				logFields = append(logFields, "header_"+strings.ToLower(key), value)
			}
		}
	}

	// Log with appropriate level based on severity
	switch severity {
	case "critical":
		slog.Error("CRITICAL security violation detected", logFields...)
	case "high":
		slog.Error("HIGH security violation detected", logFields...)
	case "medium":
		slog.Warn("Security violation detected", logFields...)
	case "low":
		slog.Info("Security event detected", logFields...)
	default:
		slog.Warn("Security violation detected", logFields...)
	}

	// Additional monitoring for critical violations
	if severity == "critical" || severity == "high" {
		// This could trigger alerts, rate limiting, or account suspension
		slog.Error("Security alert triggered",
			"alert_type", "security_violation",
			"violation_type", violationType,
			"user_id", userID,
			"tenant_id", tenantID,
			"remote_addr", r.RemoteAddr,
			"requires_investigation", true,
		)
	}
}

// determineSeverity assigns severity levels to different violation types
func determineSeverity(violationType string) string {
	criticalViolations := map[string]bool{
		"sql_injection_attempt": true,
		"path_traversal":        true,
		"code_injection":        true,
		"privilege_escalation":  true,
		"authentication_bypass": true,
	}

	highViolations := map[string]bool{
		"cross_user_access":        true,
		"cross_tenant_access":      true,
		"unauthorized_file_access": true,
		"malicious_file_upload":    true,
		"brute_force_attempt":      true,
		"token_manipulation":       true,
	}

	mediumViolations := map[string]bool{
		"invalid_input":          true,
		"missing_user_context":   true,
		"registry_context_error": true,
		"entity_access_denied":   true,
		"user_account_disabled":  true,
	}

	if criticalViolations[violationType] {
		return "critical"
	}
	if highViolations[violationType] {
		return "high"
	}
	if mediumViolations[violationType] {
		return "medium"
	}
	return "low"
}

// RequireUserContext is a helper that validates user context and returns user or handles error
func RequireUserContext(w http.ResponseWriter, r *http.Request) (*models.User, bool) {
	user, secErr := ValidateUserContext(r)
	if secErr != nil {
		HandleSecurityError(w, r, secErr)
		return nil, false
	}
	return user, true
}
