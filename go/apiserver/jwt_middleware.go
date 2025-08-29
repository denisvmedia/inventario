package apiserver

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// extractTokenFromRequest extracts JWT token from Authorization header or query parameter
func extractTokenFromRequest(r *http.Request) (string, error) {
	// Try to get token from Authorization header first
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		if tokenString == authHeader {
			return "", fmt.Errorf("bearer token required")
		}
		return tokenString, nil
	}

	// If no Authorization header, try to get token from query parameter
	tokenString := r.URL.Query().Get("token")
	if tokenString == "" {
		return "", fmt.Errorf("authorization header or token query parameter required")
	}
	return tokenString, nil
}

// validateJWTToken validates the JWT token and returns the claims
func validateJWTToken(tokenString string, jwtSecret []byte) (jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return jwtSecret, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}

	// Explicitly validate expiration claim exists and is valid
	if exp, ok := claims["exp"]; !ok {
		return nil, fmt.Errorf("token missing expiration claim")
	} else if expFloat, ok := exp.(float64); !ok {
		return nil, fmt.Errorf("invalid expiration claim format")
	} else if int64(expFloat) <= time.Now().Unix() {
		return nil, fmt.Errorf("token expired")
	}

	return claims, nil
}

// extractUserIDFromClaims extracts user ID from JWT claims
func extractUserIDFromClaims(claims jwt.MapClaims) (string, error) {
	userID, ok := claims["user_id"].(string)
	if !ok {
		return "", fmt.Errorf("invalid user ID in token")
	}
	return userID, nil
}

// validateUser retrieves and validates the user from the registry
func validateUser(r *http.Request, userID string, userRegistry registry.UserRegistry) (*models.User, error) {
	user, err := userRegistry.Get(r.Context(), userID)
	if err != nil {
		return nil, fmt.Errorf("user not found")
	}

	if !user.IsActive {
		return nil, fmt.Errorf("user account disabled")
	}

	return user, nil
}

// JWTMiddleware creates middleware that validates JWT tokens and extracts user context
func JWTMiddleware(jwtSecret []byte, userRegistry registry.UserRegistry) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract token from request
			tokenString, err := extractTokenFromRequest(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// Validate JWT token
			claims, err := validateJWTToken(tokenString, jwtSecret)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// Extract user ID from claims
			userID, err := extractUserIDFromClaims(claims)
			if err != nil {
				http.Error(w, err.Error(), http.StatusUnauthorized)
				return
			}

			// Validate user
			user, err := validateUser(r, userID, userRegistry)
			if err != nil {
				if err.Error() == "user account disabled" {
					http.Error(w, err.Error(), http.StatusForbidden)
				} else {
					http.Error(w, err.Error(), http.StatusUnauthorized)
				}
				return
			}

			// Add user to context
			ctx := appctx.WithUser(r.Context(), user)
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
			user := appctx.UserFromContext(r.Context())
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
