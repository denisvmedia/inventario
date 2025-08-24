package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	// defaultTenantID is used as a fallback tenant ID during the transition to user-only authentication
	// TODO: Remove this when user-only GetByEmail method is implemented
	defaultTenantID = "test-tenant-id"

	// jwtTokenExpiration defines how long JWT tokens remain valid
	jwtTokenExpiration = 24 * time.Hour
)

type AuthAPI struct {
	userRegistry registry.UserRegistry
	jwtSecret    []byte
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token     string       `json:"token"`
	User      *models.User `json:"user"`
	ExpiresAt time.Time    `json:"expires_at"`
}

type LogoutRequest struct {
	Token string `json:"token"`
}

type LogoutResponse struct {
	Message string `json:"message"`
}

// login handles user authentication without tenant context
func (api *AuthAPI) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Get user by email - for user-only mode, we use a default tenant ID
	// Note: This uses the existing GetByEmail method with a default tenant ID
	// In a future version, this should be replaced with a user-only GetByEmail method
	user, err := api.userRegistry.GetByEmail(r.Context(), defaultTenantID, req.Email)
	if err != nil {
		log.WithError(err).WithField("email", req.Email).Warn("Failed login attempt: user not found")
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check password
	if !user.CheckPassword(req.Password) {
		log.WithField("email", req.Email).WithField("user_id", user.ID).Warn("Failed login attempt: invalid password")
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check if user is active
	if !user.IsActive {
		log.WithField("email", req.Email).WithField("user_id", user.ID).Warn("Failed login attempt: user account disabled")
		http.Error(w, "User account disabled", http.StatusForbidden)
		return
	}

	// Generate JWT token without tenant information
	expiresAt := time.Now().Add(jwtTokenExpiration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     expiresAt.Unix(),
	})

	tokenString, err := token.SignedString(api.jwtSecret)
	if err != nil {
		log.WithError(err).WithField("user_id", user.ID).Error("Failed to generate JWT token")
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		// Log error but don't fail the login
		log.WithError(err).WithField("user_id", user.ID).Error("Failed to update user last login time")
	}

	response := LoginResponse{
		Token:     tokenString,
		User:      user,
		ExpiresAt: expiresAt,
	}

	// Log successful login
	log.WithField("email", user.Email).WithField("user_id", user.ID).WithField("role", user.Role).Info("Successful user login")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.WithError(err).WithField("user_id", user.ID).Error("Failed to encode login response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// logout handles user logout (token invalidation)
func (api *AuthAPI) logout(w http.ResponseWriter, r *http.Request) {
	// For now, logout is handled client-side by removing the token
	// In a production system, you might want to maintain a blacklist of tokens
	// or use shorter-lived tokens with refresh tokens

	response := LogoutResponse{
		Message: "Logged out successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// handleGetCurrentUser returns the current authenticated user
func (api *AuthAPI) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// Auth sets up the authentication API routes
func Auth(userRegistry registry.UserRegistry, jwtSecret []byte) func(r chi.Router) {
	api := &AuthAPI{
		userRegistry: userRegistry,
		jwtSecret:    jwtSecret,
	}

	return func(r chi.Router) {
		r.Post("/login", api.login)
		r.Post("/logout", api.logout)
		// handleGetCurrentUser requires authentication, so it should be in protected routes
		r.With(RequireAuth(jwtSecret, userRegistry)).Get("/me", api.handleGetCurrentUser)
	}
}
