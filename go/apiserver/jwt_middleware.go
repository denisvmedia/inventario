package apiserver

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// JWTMiddleware creates middleware that validates JWT tokens and extracts user context
func JWTMiddleware(jwtSecret []byte, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var tokenString string

			// Try to get token from Authorization header first
			authHeader := r.Header.Get("Authorization")
			if authHeader != "" {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
				if tokenString == authHeader {
					http.Error(w, "Bearer token required", http.StatusUnauthorized)
					return
				}
			} else {
				// If no Authorization header, try to get token from query parameter
				tokenString = r.URL.Query().Get("token")
				if tokenString == "" {
					http.Error(w, "Authorization header or token query parameter required", http.StatusUnauthorized)
					return
				}
			}

			token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
				if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
				}
				return jwtSecret, nil
			})

			if err != nil || !token.Valid {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok {
				http.Error(w, "Invalid token claims", http.StatusUnauthorized)
				return
			}

			userID, ok := claims["user_id"].(string)
			if !ok {
				http.Error(w, "Invalid user ID in token", http.StatusUnauthorized)
				return
			}

			// Get user from registry
			user, err := userRegistry.Get(r.Context(), userID)
			if err != nil {
				http.Error(w, "User not found", http.StatusUnauthorized)
				return
			}

			// Check if user is active
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

// RequireAuth is an alias for JWTMiddleware for backward compatibility
func RequireAuth(jwtSecret []byte, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
	return JWTMiddleware(jwtSecret, userRegistry)
}

// FileAccessMiddleware creates middleware specifically for file access that supports both
// Authorization header and query parameter authentication for direct browser access
func FileAccessMiddleware(jwtSecret []byte, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
	return JWTMiddleware(jwtSecret, userRegistry) // Use the updated JWTMiddleware that supports both methods
}

// RequireRole middleware ensures that the authenticated user has the specified role
func RequireRole(role models.UserRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user := UserFromContext(r.Context())
			if user == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			if user.Role != role {
				http.Error(w, "Insufficient permissions", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
