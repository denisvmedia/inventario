package apiserver

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
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
	// TODO: Implement user-only GetByEmail method in the registry
	user, err := api.userRegistry.GetByEmail(r.Context(), "test-tenant-id", req.Email)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check password
	if !user.CheckPassword(req.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check if user is active
	if !user.IsActive {
		http.Error(w, "User account disabled", http.StatusForbidden)
		return
	}

	// Generate JWT token without tenant information
	expiresAt := time.Now().Add(24 * time.Hour)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     expiresAt.Unix(),
	})

	tokenString, err := token.SignedString(api.jwtSecret)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		// Log error but don't fail the login
		// TODO: Add proper logging
	}

	response := LoginResponse{
		Token:     tokenString,
		User:      user,
		ExpiresAt: expiresAt,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
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

// getCurrentUser returns the current authenticated user
func (api *AuthAPI) getCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r.Context())
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
		// getCurrentUser requires authentication, so it should be in protected routes
		r.With(RequireAuth(jwtSecret, userRegistry)).Get("/me", api.getCurrentUser)
	}
}
