package registry

import (
	"context"
	"errors"

	"github.com/denisvmedia/inventario/internal/errkit"
)

var (
	// ErrUserContextRequired is returned when user context is required but not found
	ErrUserContextRequired = errors.New("user context required")
	
	// ErrInvalidUserContext is returned when user context is invalid
	ErrInvalidUserContext = errors.New("invalid user context")
)

// UserIDFromContext extracts the user ID from the context
// This function looks for user ID in the context using the same pattern as apiserver
func UserIDFromContext(ctx context.Context) string {
	// Try to get user ID from context using the same key pattern as apiserver
	if userID, ok := ctx.Value("userID").(string); ok && userID != "" {
		return userID
	}
	
	// Fallback: try alternative key patterns that might be used
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		return userID
	}
	
	return ""
}

// ValidateUserContext validates that a user context is present and valid
func ValidateUserContext(ctx context.Context) error {
	userID := UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(ErrUserContextRequired)
	}
	
	// Additional validation can be added here if needed
	// For example, checking if the user ID format is valid
	
	return nil
}

// RequireUserID extracts and validates user ID from context
// Returns the user ID or an error if not found or invalid
func RequireUserID(ctx context.Context) (string, error) {
	userID := UserIDFromContext(ctx)
	if userID == "" {
		return "", errkit.WithStack(ErrUserContextRequired)
	}
	
	return userID, nil
}

// WithUserContext creates a new context with user ID
// This is a helper function for testing and internal use
func WithUserContext(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, "userID", userID)
}

// UserContextExecutor provides a helper for executing operations with user context
type UserContextExecutor struct {
	userID string
}

// NewUserContextExecutor creates a new user context executor
func NewUserContextExecutor(userID string) *UserContextExecutor {
	return &UserContextExecutor{userID: userID}
}

// Execute runs a function with user context set
func (e *UserContextExecutor) Execute(ctx context.Context, fn func(context.Context) error) error {
	if e.userID == "" {
		return errkit.WithStack(ErrInvalidUserContext)
	}

	userCtx := WithUserContext(ctx, e.userID)
	return fn(userCtx)
}

// ExecuteWithUserID is a convenience function to execute a function with user context
func ExecuteWithUserID(ctx context.Context, userID string, fn func(context.Context) error) error {
	if userID == "" {
		return errkit.WithStack(ErrInvalidUserContext)
	}

	userCtx := WithUserContext(ctx, userID)
	return fn(userCtx)
}
