package apiserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	userCtxKey ctxValueKey = "user"
	userIDKey  ctxValueKey = "userID"
)

// UserResolver interface defines methods for resolving user from HTTP requests
type UserResolver interface {
	ResolveUser(r *http.Request) (string, error)
}

// JWTUserResolver resolves user from JWT token in Authorization header
type JWTUserResolver struct {
	jwtSecret []byte
}

// NewJWTUserResolver creates a new JWT user resolver
func NewJWTUserResolver(jwtSecret []byte) *JWTUserResolver {
	return &JWTUserResolver{
		jwtSecret: jwtSecret,
	}
}

// ResolveUser extracts user ID from JWT token in Authorization header
func (j *JWTUserResolver) ResolveUser(r *http.Request) (string, error) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrUserNotFound
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return "", ErrUserNotFound
	}

	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return j.jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return "", ErrUserNotFound
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", ErrUserNotFound
	}

	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", ErrUserNotFound
	}

	return userID, nil
}

// UserFromContext retrieves the user from the request context
func UserFromContext(ctx context.Context) *models.User {
	user, ok := ctx.Value(userCtxKey).(*models.User)
	if !ok {
		return nil
	}
	return user
}

// UserIDFromContext retrieves the user ID from the request context
func UserIDFromContext(ctx context.Context) string {
	userID, ok := ctx.Value(userIDKey).(string)
	if !ok {
		return ""
	}
	return userID
}

// WithUser adds a user to the context
func WithUser(ctx context.Context, user *models.User) context.Context {
	ctx = context.WithValue(ctx, userCtxKey, user)
	ctx = context.WithValue(ctx, userIDKey, user.ID)
	return ctx
}

// WithUserID adds a user ID to the context
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

// UserMiddleware creates middleware that resolves and validates user context
func UserMiddleware(resolver UserResolver, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Resolve user ID from request
			userID, err := resolver.ResolveUser(r)
			if err != nil {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}

			// Get user from registry
			user, err := userRegistry.Get(r.Context(), userID)
			if err != nil {
				http.Error(w, "Invalid user", http.StatusUnauthorized)
				return
			}

			// Check user status
			if !user.IsActive {
				http.Error(w, "User account disabled", http.StatusForbidden)
				return
			}

			// Add user to context
			ctx := WithUser(r.Context(), user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireUser middleware ensures that a user is present in the context
func RequireUser() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http.Error(w, "User context required", http.StatusInternalServerError)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// UserAwareMiddleware creates middleware that adds user context for user-aware operations
// This is a lighter version that only adds user ID to context without full validation
func UserAwareMiddleware(resolver UserResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try to resolve user ID from request
			userID, err := resolver.ResolveUser(r)
			if err != nil {
				// If no user can be resolved, continue without user context
				// This allows for backward compatibility with non-user-aware endpoints
				next.ServeHTTP(w, r)
				return
			}

			// Add user ID to context
			ctx := WithUserID(r.Context(), userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
