package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
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
	rateLimiter          services.AuthRateLimiter
	csrfService          services.CSRFService
	auditService         services.AuditLogger
	emailService         services.EmailService
	jwtSecret            []byte
}

func (api *AuthAPI) sendPasswordChangedNotification(user *models.User) {
	if api.emailService == nil || user == nil {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := api.emailService.SendPasswordChangedEmail(ctx, user.Email, user.Name, time.Now()); err != nil {
			slog.Error("Failed to send password-changed notification email",
				"user_id", user.ID,
				"error", err,
			)
		}
	}()
}

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// ChangePasswordRequest is the body for POST /auth/change-password.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// LoginResponse is returned on successful login or token refresh.
type LoginResponse struct {
	AccessToken string       `json:"access_token"`
	TokenType   string       `json:"token_type"`
	ExpiresIn   int          `json:"expires_in"` // seconds until access token expiry
	CSRFToken   string       `json:"csrf_token"` // CSRF token for protecting state-changing requests
	User        *models.User `json:"user"`
}

// LogoutResponse is returned on successful logout.
type LogoutResponse struct {
	Message string `json:"message"`
}

// login handles user authentication and issues both an access token and a refresh token.
// @Summary Login
// @Description Authenticate a user with email and password. Issues an access token in the response body and sets a refresh token as an httpOnly cookie.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body LoginRequest true "Login credentials"
// @Success 200 {object} LoginResponse "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized - invalid credentials"
// @Failure 403 {string} string "Forbidden - account disabled"
// @Failure 429 {string} string "Too Many Requests - account locked"
// @Router /auth/login [post]
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

	// Account lockout is enforced per email to mitigate distributed brute force.
	if api.rateLimiter != nil {
		locked, resetAt, err := api.rateLimiter.IsAccountLocked(r.Context(), req.Email)
		if err != nil {
			// Fail-open: do not make auth unavailable due to limiter backend outages.
			slog.Error("Failed to check account lockout", "error", err)
		} else if locked {
			retryAfter := max(int(time.Until(resetAt).Seconds()), 0)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			http.Error(w, "Too many failed login attempts. Please try again later.", http.StatusTooManyRequests)
			return
		}
	}

	user, err := api.userRegistry.GetByEmail(r.Context(), DefaultTenantID, req.Email)
	if err != nil {
		slog.Warn("Failed login attempt: user not found", "email", req.Email, "error", err)
		if api.rateLimiter != nil {
			if _, _, rlErr := api.rateLimiter.RecordFailedLogin(r.Context(), req.Email); rlErr != nil {
				slog.Error("Failed to record failed login", "error", rlErr)
			}
		}
		errMsg := "user not found"
		api.logAuth(r.Context(), "login", nil, nil, false, r, &errMsg)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !user.CheckPassword(req.Password) {
		slog.Warn("Failed login attempt: invalid password", "email", req.Email, "user_id", user.ID)
		if api.rateLimiter != nil {
			if _, _, rlErr := api.rateLimiter.RecordFailedLogin(r.Context(), req.Email); rlErr != nil {
				slog.Error("Failed to record failed login", "error", rlErr)
			}
		}
		errMsg := "invalid password"
		api.logAuth(r.Context(), "login", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !user.IsActive {
		slog.Warn("Failed login attempt: user account disabled", "email", req.Email, "user_id", user.ID)
		errMsg := "account disabled"
		api.logAuth(r.Context(), "login", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "User account disabled", http.StatusForbidden)
		return
	}

	// Successful authentication: clear any prior failed-login counters.
	if api.rateLimiter != nil {
		if err := api.rateLimiter.ClearFailedLogins(r.Context(), req.Email); err != nil {
			slog.Error("Failed to clear failed login counters", "error", err)
		}
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
	api.logAuth(r.Context(), "login", &user.ID, &user.TenantID, true, r, nil)

	// Generate a CSRF token for this session.
	csrfToken := api.generateCSRFTokenForUser(r.Context(), user.ID)

	writeLoginResponse(w, accessTokenString, csrfToken, user)
}

// refresh issues a new access token using a valid refresh token cookie.
// @Summary Refresh access token
// @Description Issue a new short-lived access token using the refresh token stored in the httpOnly cookie.
// @Tags auth
// @Produce json
// @Success 200 {object} LoginResponse "OK"
// @Failure 401 {string} string "Unauthorized - missing, invalid, or expired refresh token"
// @Failure 501 {string} string "Not Implemented - refresh tokens not supported"
// @Router /auth/refresh [post]
func (api *AuthAPI) refresh(w http.ResponseWriter, r *http.Request) {
	if api.refreshTokenRegistry == nil {
		http.Error(w, "Refresh tokens not supported", http.StatusNotImplemented)
		return
	}
	cookie, err := r.Cookie(refreshTokenCookieName)
	if err != nil {
		// Cookie missing: nothing to clear — do not set MaxAge=-1 for a non-existent cookie.
		http.Error(w, "Refresh token required", http.StatusUnauthorized)
		return
	}

	tokenHash := models.HashRefreshToken(cookie.Value)

	refreshToken, err := api.refreshTokenRegistry.GetByTokenHash(r.Context(), tokenHash)
	if err != nil {
		slog.Warn("Refresh token not found", "error", err)
		clearRefreshCookie(w, r)
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	if !refreshToken.IsValid() {
		slog.Warn("Expired or revoked refresh token", "token_id", refreshToken.ID)
		clearRefreshCookie(w, r)
		http.Error(w, "Refresh token expired or revoked", http.StatusUnauthorized)
		return
	}

	user, err := api.userRegistry.Get(r.Context(), refreshToken.UserID)
	if err != nil || !user.IsActive {
		slog.Warn("Refresh token for invalid/inactive user", "user_id", refreshToken.UserID)
		clearRefreshCookie(w, r)
		http.Error(w, "User not found or inactive", http.StatusUnauthorized)
		return
	}

	// Reject refresh if the user has been force-blacklisted (e.g. after a password change).
	if api.blacklistService != nil {
		blacklisted, blErr := api.blacklistService.IsUserBlacklisted(r.Context(), user.ID)
		if blErr != nil {
			// Fail-open: consistent with checkTokenBlacklist in jwt_middleware.go.
			// A Redis outage should not lock users out of the refresh flow.
			slog.Error("Failed to check user blacklist on refresh", "user_id", user.ID, "error", blErr)
		}
		if blacklisted {
			slog.Warn("Blacklisted user attempted token refresh", "user_id", user.ID)
			clearRefreshCookie(w, r)
			http.Error(w, "User not found or inactive", http.StatusUnauthorized)
			return
		}
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

	// Re-generate the CSRF token so it stays in sync with the new access token.
	csrfToken := api.generateCSRFTokenForUser(r.Context(), user.ID)

	writeLoginResponse(w, accessTokenString, csrfToken, user)
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
	// Parse with signature verification. We allow tokens that are already expired
	// (e.g. near-expiry tokens sent during logout) but reject any token with an
	// invalid signature to prevent a client from blacklisting arbitrary JTIs.
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return api.jwtSecret, nil
	})
	if token == nil {
		return
	}
	// Skip blacklisting if there is any error other than token expiry
	// (e.g. invalid signature, unsupported algorithm).
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
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
	// Cap the expiry to 2× accessTokenExpiration as a defence-in-depth measure
	// against artificially large exp values.
	expiresAt := time.Unix(int64(exp), 0)
	maxExpiry := time.Now().Add(2 * accessTokenExpiration)
	if expiresAt.After(maxExpiry) {
		expiresAt = maxExpiry
	}
	if err := api.blacklistService.BlacklistToken(ctx, jti, expiresAt); err != nil {
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
// @Summary Logout
// @Description Revoke the current session's access token and clear the refresh token cookie.
// @Tags auth
// @Produce json
// @Success 200 {object} LogoutResponse "OK"
// @Router /auth/logout [post]
func (api *AuthAPI) logout(w http.ResponseWriter, r *http.Request) {
	// Blacklist the current access token so it cannot be reused within its remaining TTL.
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		api.blacklistAccessToken(r.Context(), authHeader)
	}

	// Revoke the refresh token from the database.
	if cookie, err := r.Cookie(refreshTokenCookieName); err == nil {
		api.revokeRefreshToken(r.Context(), cookie.Value)
	}

	// Delete CSRF token for this user.
	currentUser := appctx.UserFromContext(r.Context())
	if currentUser != nil && api.csrfService != nil {
		if err := api.csrfService.DeleteToken(r.Context(), currentUser.ID); err != nil {
			slog.Error("Failed to delete CSRF token on logout", "user_id", currentUser.ID, "error", err)
		}
	}

	// Audit the logout event.
	if currentUser != nil {
		api.logAuth(r.Context(), "logout", &currentUser.ID, &currentUser.TenantID, true, r, nil)
	}

	// Clear the refresh token cookie.
	secureCookie := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     refreshTokenCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(LogoutResponse{Message: "Logged out successfully"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleGetCurrentUser returns the current authenticated user.
// It also refreshes and exposes the CSRF token in the X-CSRF-Token response
// header so the frontend can recover its CSRF token after a page reload.
// @Summary Get current user
// @Description Return the currently authenticated user's profile. Also refreshes the CSRF token in the X-CSRF-Token response header.
// @Tags auth
// @Produce json
// @Success 200 {object} models.User "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/me [get]
func (api *AuthAPI) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// Ensure the CSRF token is in the response header (generate if missing, e.g.
	// after a page reload where the in-memory token was lost).
	api.writeCSRFHeader(w, r.Context(), user.ID)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(user); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// generateCSRFTokenForUser issues a new CSRF token for the given user.
// Returns "" when csrfService is nil or on error (errors are logged).
func (api *AuthAPI) generateCSRFTokenForUser(ctx context.Context, userID string) string {
	if api.csrfService == nil {
		return ""
	}
	token, err := api.csrfService.GenerateToken(ctx, userID)
	if err != nil {
		slog.Error("Failed to generate CSRF token", "user_id", userID, "error", err)
		// Non-fatal: the middleware fails-open on storage errors.
	}
	return token
}

// writeCSRFHeader retrieves (or regenerates) the CSRF token for the user and
// sets the X-CSRF-Token response header. No-op when csrfService is nil.
func (api *AuthAPI) writeCSRFHeader(w http.ResponseWriter, ctx context.Context, userID string) {
	if api.csrfService == nil {
		return
	}
	token, err := api.csrfService.GetToken(ctx, userID)
	if err != nil {
		slog.Error("Failed to get CSRF token for /auth/me", "user_id", userID, "error", err)
		return
	}
	if token == "" {
		// Token has expired or was lost — regenerate it.
		token, err = api.csrfService.GenerateToken(ctx, userID)
		if err != nil {
			slog.Error("Failed to regenerate CSRF token for /auth/me", "user_id", userID, "error", err)
			return
		}
	}
	if token != "" {
		w.Header().Set(csrfHeaderName, token)
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
	RateLimiter          services.AuthRateLimiter
	CSRFService          services.CSRFService
	AuditService         services.AuditLogger
	EmailService         services.EmailService
	JWTSecret            []byte
}

// Auth sets up the authentication API routes.
func Auth(params AuthParams) func(r chi.Router) {
	api := &AuthAPI{
		userRegistry:         params.UserRegistry,
		refreshTokenRegistry: params.RefreshTokenRegistry,
		blacklistService:     params.BlacklistService,
		rateLimiter:          params.RateLimiter,
		csrfService:          params.CSRFService,
		auditService:         params.AuditService,
		emailService:         params.EmailService,
		jwtSecret:            params.JWTSecret,
	}

	return func(r chi.Router) {
		r.With(AuthLoginRateLimitMiddleware(params.RateLimiter)).Post("/login", api.login)
		r.Post("/refresh", api.refresh)
		r.Post("/logout", api.logout)
		// Routes requiring authentication
		r.With(RequireAuth(params.JWTSecret, params.UserRegistry, params.BlacklistService)).Get("/me", api.handleGetCurrentUser)
		r.With(RequireAuth(params.JWTSecret, params.UserRegistry, params.BlacklistService)).Post("/change-password", api.handleChangePassword)
	}
}

// handleChangePassword allows an authenticated user to change their own password.
// On success it revokes all existing refresh tokens and blacklists existing access
// tokens so that all active sessions are invalidated and the user must re-login.
// @Summary Change password
// @Description Change the authenticated user's password. All existing sessions are invalidated on success.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body ChangePasswordRequest true "Change password request"
// @Success 200 {object} map[string]string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/change-password [post]
func (api *AuthAPI) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var req ChangePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		http.Error(w, "Current and new passwords are required", http.StatusBadRequest)
		return
	}

	// Verify the current password before allowing the change.
	if !user.CheckPassword(req.CurrentPassword) {
		slog.Warn("Password change failed: incorrect current password", "user_id", user.ID, "email", user.Email)
		errMsg := "incorrect current password"
		api.logAuth(r.Context(), "password_change", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	// Validate the new password meets complexity requirements.
	if err := models.ValidatePassword(req.NewPassword); err != nil {
		errMsg := "new password does not meet complexity requirements"
		api.logAuth(r.Context(), "password_change", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Apply the new password hash.
	if err := user.SetPassword(req.NewPassword); err != nil {
		slog.Error("Failed to hash new password", "user_id", user.ID, "error", err)
		errMsg := "failed to hash new password"
		api.logAuth(r.Context(), "password_change", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Failed to process new password", http.StatusInternalServerError)
		return
	}
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to update user password", "user_id", user.ID, "error", err)
		errMsg := "failed to update user password"
		api.logAuth(r.Context(), "password_change", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	// Revoke all refresh tokens for the user so existing sessions are invalidated.
	if api.refreshTokenRegistry != nil {
		if err := api.refreshTokenRegistry.RevokeByUserID(r.Context(), user.ID); err != nil {
			slog.Error("Failed to revoke refresh tokens after password change", "user_id", user.ID, "error", err)
		}
	}

	// Blacklist the current access token and all user tokens so they cannot be reused.
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		api.blacklistAccessToken(r.Context(), authHeader)
	}
	if api.blacklistService != nil {
		if err := api.blacklistService.BlacklistUserTokens(r.Context(), user.ID, 2*accessTokenExpiration); err != nil {
			slog.Error("Failed to blacklist user tokens after password change", "user_id", user.ID, "error", err)
		}
	}

	slog.Info("Password changed successfully", "user_id", user.ID, "email", user.Email)
	api.logAuth(r.Context(), "password_change", &user.ID, &user.TenantID, true, r, nil)
	api.sendPasswordChangedNotification(user)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"message": "Password changed successfully. Please login again.",
	}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// logAuth is a nil-safe wrapper around auditService.LogAuth.
// It is a no-op when auditService has not been configured.
func (api *AuthAPI) logAuth(ctx context.Context, action string, userID, tenantID *string, success bool, r *http.Request, errMsg *string) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAuth(ctx, action, userID, tenantID, success, r, errMsg)
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

	// Set Secure flag only when the connection is already over HTTPS to allow
	// local development over plain HTTP.
	secureCookie := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")

	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    rawToken,
		Path:     refreshTokenCookiePath,
		MaxAge:   int(refreshTokenExpiration.Seconds()),
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
	return nil
}

// writeLoginResponse encodes and writes a LoginResponse to w.
func writeLoginResponse(w http.ResponseWriter, accessToken, csrfToken string, user *models.User) {
	w.Header().Set("Content-Type", "application/json")
	resp := LoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(accessTokenExpiration.Seconds()),
		CSRFToken:   csrfToken,
		User:        user,
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// clearRefreshCookie instructs the browser to delete the refresh token cookie
// by setting MaxAge=-1. This should be called on all failure paths in refresh()
// where the cookie is present but invalid/expired/revoked, so that the browser
// does not keep sending a stale token and causing repeated 401 loops.
func clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
	secureCookie := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	http.SetCookie(w, &http.Cookie{
		Name:     refreshTokenCookieName,
		Value:    "",
		Path:     refreshTokenCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
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
	// Fall back to RemoteAddr. Use net.SplitHostPort to correctly handle
	// both IPv4 ("1.2.3.4:port") and IPv6 ("[::1]:port") addresses.
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}
