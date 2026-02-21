package apiserver

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

var (
	// DefaultTenantID is used as a fallback tenant ID during the transition to user-only authentication
	// TODO: Remove this when user-only GetByEmail method is implemented
	DefaultTenantID = "test-tenant-id"
)

const (
	// accessTokenExpiration defines how long access JWT tokens remain valid.
	accessTokenExpiration = 15 * time.Minute
	// refreshTokenExpiration defines how long refresh tokens remain valid.
	refreshTokenExpiration = 30 * 24 * time.Hour
	// refreshTokenCookieName is the name of the httpOnly cookie carrying the refresh token.
	refreshTokenCookieName = "refresh_token"
	// refreshTokenCookiePath limits the cookie to the auth endpoints only.
	refreshTokenCookiePath = "/api/v1/auth" // #nosec G101 -- this is a URL path, not a credential
)

// AuthAPI handles authentication endpoints.
type AuthAPI struct {
	userRegistry         registry.UserRegistry
	refreshTokenRegistry registry.RefreshTokenRegistry
	blacklistService     services.TokenBlacklister
	jwtSecret            []byte
}

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse is returned on successful login or token refresh.
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"` // seconds until access token expiry
	User        *models.User `json:"user"`
}

// LogoutResponse is returned on successful logout.
type LogoutResponse struct {
	Message string `json:"message"`
}

// login handles user authentication and issues both an access token and a refresh token.
func (api *AuthAPI) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	user, err := api.userRegistry.GetByEmail(r.Context(), DefaultTenantID, req.Email)
	if err != nil {
		slog.Warn("Failed login attempt: user not found", "email", req.Email, "error", err)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !user.CheckPassword(req.Password) {
		slog.Warn("Failed login attempt: invalid password", "email", req.Email, "user_id", user.ID)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !user.IsActive {
		slog.Warn("Failed login attempt: user account disabled", "email", req.Email, "user_id", user.ID)
		http.Error(w, "User account disabled", http.StatusForbidden)
		return
	}

	// Issue short-lived access token with a unique JTI for revocation support.
	accessTokenString, _, err := api.issueAccessToken(user)
	if err != nil {
		slog.Error("Failed to generate access token", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Issue and persist a long-lived refresh token.
	if err := api.issueRefreshTokenCookie(w, r, user); err != nil {
		slog.Error("Failed to issue refresh token", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	// Update last login timestamp (best-effort).
	now := time.Now()
	user.LastLoginAt = &now
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to update user last login time", "user_id", user.ID, "error", err)
	}

	slog.Info("Successful user login", "email", user.Email, "user_id", user.ID, "role", user.Role)

	writeLoginResponse(w, accessTokenString, user)
}

// refresh issues a new access token using a valid refresh token cookie.
func (api *AuthAPI) refresh(w http.ResponseWriter, r *http.Request) {
	if api.refreshTokenRegistry == nil {
		http.Error(w, "Refresh tokens not supported", http.StatusNotImplemented)
		return
	}
	cookie, err := r.Cookie(refreshTokenCookieName)
	if err != nil {
		http.Error(w, "Refresh token required", http.StatusUnauthorized)
		return
	}

	tokenHash := models.HashRefreshToken(cookie.Value)

	refreshToken, err := api.refreshTokenRegistry.GetByTokenHash(r.Context(), tokenHash)
	if err != nil {
		slog.Warn("Refresh token not found", "error", err)
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	if !refreshToken.IsValid() {
		slog.Warn("Expired or revoked refresh token", "token_id", refreshToken.ID)
		http.Error(w, "Refresh token expired or revoked", http.StatusUnauthorized)
		return
	}

	user, err := api.userRegistry.Get(r.Context(), refreshToken.UserID)
	if err != nil || !user.IsActive {
		slog.Warn("Refresh token for invalid/inactive user", "user_id", refreshToken.UserID)
		http.Error(w, "User not found or inactive", http.StatusUnauthorized)
		return
	}

	accessTokenString, _, err := api.issueAccessToken(user)
	if err != nil {
		slog.Error("Failed to generate access token on refresh", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Update last-used timestamp on the refresh token (best-effort).
	now := time.Now()
	refreshToken.LastUsedAt = &now
	if _, err := api.refreshTokenRegistry.Update(r.Context(), *refreshToken); err != nil {
		slog.Error("Failed to update refresh token last_used_at", "token_id", refreshToken.ID, "error", err)
	}

	writeLoginResponse(w, accessTokenString, user)
}

// blacklistAccessToken extracts claims from a Bearer token header and blacklists its JTI.
func (api *AuthAPI) blacklistAccessToken(ctx context.Context, authHeader string) {
	if api.blacklistService == nil {
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return
	}
	// Parse without validation to extract claims even for near-expired tokens.
	token, _ := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		return api.jwtSecret, nil
	})
	if token == nil {
		return
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return
	}
	jti, ok := claims["jti"].(string)
	if !ok {
		return
	}
	exp, ok := claims["exp"].(float64)
	if !ok {
		return
	}
	if err := api.blacklistService.BlacklistToken(ctx, jti, time.Unix(int64(exp), 0)); err != nil {
		slog.Error("Failed to blacklist access token", "error", err)
	}
}

// revokeRefreshToken looks up a refresh token by raw value and marks it revoked.
func (api *AuthAPI) revokeRefreshToken(ctx context.Context, rawToken string) {
	if api.refreshTokenRegistry == nil {
		return
	}
	tokenHash := models.HashRefreshToken(rawToken)
	rt, err := api.refreshTokenRegistry.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		return
	}
	now := time.Now()
	rt.RevokedAt = &now
	if _, err := api.refreshTokenRegistry.Update(ctx, *rt); err != nil {
		slog.Error("Failed to revoke refresh token", "token_id", rt.ID, "error", err)
	}
}

// logout revokes the current access token and refresh token.
func (api *AuthAPI) logout(w http.ResponseWriter, r *http.Request) {
	// Blacklist the current access token so it cannot be reused within its remaining TTL.
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		api.blacklistAccessToken(r.Context(), authHeader)
	}

	// Revoke the refresh token from the database.
	if cookie, err := r.Cookie(refreshTokenCookieName); err == nil {
		api.revokeRefreshToken(r.Context(), cookie.Value)
	}

	// Clear the refresh token cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     refreshTokenCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(LogoutResponse{Message: "Logged out successfully"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleGetCurrentUser returns the current authenticated user.
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

// -----------------------------------------------------------------------
// Auth route registration
// -----------------------------------------------------------------------

// AuthParams holds all dependencies needed by the auth API.
type AuthParams struct {
	UserRegistry         registry.UserRegistry
	RefreshTokenRegistry registry.RefreshTokenRegistry
	BlacklistService     services.TokenBlacklister
	JWTSecret            []byte
}

// Auth sets up the authentication API routes.
func Auth(params AuthParams) func(r chi.Router) {
	api := &AuthAPI{
		userRegistry:         params.UserRegistry,
		refreshTokenRegistry: params.RefreshTokenRegistry,
		blacklistService:     params.BlacklistService,
		jwtSecret:            params.JWTSecret,
	}

	return func(r chi.Router) {
		r.Post("/login", api.login)
		r.Post("/refresh", api.refresh)
		r.Post("/logout", api.logout)
		// /me requires authentication
		r.With(RequireAuth(params.JWTSecret, params.UserRegistry, params.BlacklistService)).Get("/me", api.handleGetCurrentUser)
	}
}

// -----------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------

// issueAccessToken creates and signs a short-lived JWT with a unique JTI.
// Returns the signed token string, its expiry time, and any error.
func (api *AuthAPI) issueAccessToken(user *models.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(accessTokenExpiration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":     uuid.New().String(),
		"user_id": user.ID,
		"role":    string(user.Role),
		"exp":     expiresAt.Unix(),
		"iat":     time.Now().Unix(),
	})
	tokenString, err := token.SignedString(api.jwtSecret)
	return tokenString, expiresAt, err
}

// issueRefreshTokenCookie generates a refresh token, stores it in the database, and
// sets it as an httpOnly cookie on the response.
// If no refreshTokenRegistry is configured, the cookie is skipped.
func (api *AuthAPI) issueRefreshTokenCookie(w http.ResponseWriter, r *http.Request, user *models.User) error {
	if api.refreshTokenRegistry == nil {
		return nil
	}

	rawToken, tokenHash, err := models.GenerateRefreshToken()
	if err != nil {
		return err
	}

	rt := models.RefreshToken{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		TokenHash: tokenHash,
		ExpiresAt: time.Now().Add(refreshTokenExpiration),
		IPAddress: getClientIP(r),
		UserAgent: r.UserAgent(),
	}

	if _, err := api.refreshTokenRegistry.Create(r.Context(), rt); err != nil {
		return err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    rawToken,
		Path:     refreshTokenCookiePath,
		MaxAge:   int(refreshTokenExpiration.Seconds()),
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

// writeLoginResponse encodes and writes a LoginResponse to w.
func writeLoginResponse(w http.ResponseWriter, accessToken string, user *models.User) {
	w.Header().Set("Content-Type", "application/json")
	resp := LoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(accessTokenExpiration.Seconds()),
		User:        user,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// getClientIP extracts the real client IP from the request, respecting
// common proxy headers.
func getClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first (original client) IP.
		parts := strings.SplitN(xff, ",", 2)
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Fall back to RemoteAddr (strips port if present).
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}
