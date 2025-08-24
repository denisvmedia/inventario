package appctx

import (
	"context"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// userIDKey is the context key for storing user ID
	userIDKey  contextKey = "userID"
	userCtxKey contextKey = "user"
)

// UserIDFromContext extracts the user ID from the context
// This function looks for user ID in the context using the typed key
func UserIDFromContext(ctx context.Context) string {
	// Try to get user ID from context using the typed key
	if userID, ok := ctx.Value(userIDKey).(string); ok && userID != "" {
		return userID
	}
	return ""
}

func UserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(userCtxKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

func WithUser(ctx context.Context, user *models.User) context.Context {
	ctx = context.WithValue(ctx, userCtxKey, user)
	ctx = context.WithValue(ctx, userIDKey, user.ID)
	return ctx
}

// ValidateUserContext validates that a user context is present and valid
func ValidateUserContext(ctx context.Context) error {
	user := UserFromContext(ctx)
	if user != nil {
		return errkit.WithStack(ErrUserContextRequired)
	}

	return nil
}

func RequireUserFromContext(ctx context.Context) (*models.User, error) {
	user := UserFromContext(ctx)
	if user == nil {
		return nil, errkit.WithStack(ErrUserContextRequired)
	}

	return user, nil
}
