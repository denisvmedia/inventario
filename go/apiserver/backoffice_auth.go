package apiserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-extras/errx"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// Back-office auth plane (issue #1785, Phase 2).
//
// This is a SECOND auth plane, completely isolated from the tenant-side
// /api/v1/auth/* surface. The boundary is enforced by three independent
// invariants any one of which is sufficient to keep the planes apart:
//
//  1. Different `aud` claim: tenant tokens carry `aud == "tenant"`,
//     back-office tokens `aud == "backoffice"`. validateBackofficeToken
//     rejects anything that isn't backoffice; JWTMiddleware (tenant)
//     rejects anything that IS backoffice.
//
//  2. Different identifying claim: tenant tokens carry `user_id` (points
//     at users.id), back-office tokens carry `admin_id` (points at
//     backoffice_users.id). The presence of EITHER claim on the wrong
//     plane is treated as proof of misuse and rejected even when `aud`
//     somehow agrees.
//
//  3. Different cookie name + path: back-office refresh cookie is
//     `backoffice_refresh_token` at `/api/v1/backoffice`. A tenant
//     cookie can never reach the back-office refresh handler, and vice
//     versa, even if a browser holds both.
//
// The JWT signing secret is shared with the tenant plane — rotating it
// already invalidates everything, so a separate secret would double
// operator surface without adding isolation. The `aud` claim is the
// actual boundary.
const (
	// backofficeTokenAudience is the canonical `aud` claim value for
	// every JWT minted by the back-office auth plane. Both the access
	// token and the optional MFA challenge token carry it (Phase 4).
	// The tenant plane does NOT currently stamp an `aud` claim — the
	// historical mint omits it — so the cross-plane guard relies on
	// rejecting this value rather than enforcing a tenant value. A
	// future tenant-side hardening can stamp its own audience without
	// breaking the back-office guard here.
	backofficeTokenAudience = "backoffice"
	// backofficeAccessTokenExpiration mirrors the tenant accessTokenExpiration —
	// 15 min — so back-office sessions exhibit the same "stay short, refresh
	// often" cadence the tenant plane uses. Diverging without a reason
	// would only complicate auditing.
	backofficeAccessTokenExpiration = accessTokenExpiration
	// backofficeRefreshTokenExpiration mirrors refreshTokenExpiration (30 days).
	backofficeRefreshTokenExpiration = refreshTokenExpiration
	// backofficeRefreshTokenCookieName is deliberately distinct from the
	// tenant `refresh_token` cookie. A browser carrying both must not
	// accidentally use one on the other plane's endpoints.
	backofficeRefreshTokenCookieName = "backoffice_refresh_token" // #nosec G101 -- a cookie name, not a credential
	// backofficeRefreshTokenCookiePath scopes the cookie to the
	// back-office subtree only — a tenant request to `/api/v1/auth/refresh`
	// will NOT receive it.
	backofficeRefreshTokenCookiePath = "/api/v1/backoffice" // #nosec G101 -- a URL path, not a credential
	// Back-office audit action strings — kept as constants so the wire
	// values are pinned in one place and tests can assert against them.
	backofficeActionLogin       = "backoffice.login"
	backofficeActionLoginFailed = "backoffice.login_failed"
	backofficeActionLogout      = "backoffice.logout"
	backofficeActionRefresh     = "backoffice.refresh"
	backofficeActionMFARequired = "backoffice.login_mfa_required"
)

// ErrBackofficeAccountDisabled is surfaced when login (or refresh)
// finds a back-office row with `is_active=false`. Distinct sentinel so
// the FE can render "your account has been suspended" copy without
// branching on a status-code-only signal.
var ErrBackofficeAccountDisabled = errx.NewSentinel("backoffice account disabled")

// ErrBackofficeInvalidCredentials is what every wrong-email / wrong-password /
// missing-row failure resolves to. A single sentinel keeps the response
// identical across the three internal paths so an attacker can't probe for
// account existence.
var ErrBackofficeInvalidCredentials = errx.NewSentinel("invalid credentials")

// BackofficeLoginRequest is the body for POST /backoffice/auth/login.
type BackofficeLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// BackofficeProfile is the trimmed view of a BackofficeUser returned in
// login / refresh / me responses. It drops PasswordHash entirely
// (already json:"-" on the model, but spelled out here so callers reading
// the type can see the contract) and exposes only fields the FE renders.
type BackofficeProfile struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        string     `json:"name"`
	Role        string     `json:"role"`
	MFAEnforced bool       `json:"mfa_enforced"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`
}

func backofficeProfileFromUser(u *models.BackofficeUser) *BackofficeProfile {
	if u == nil {
		return nil
	}
	return &BackofficeProfile{
		ID:          u.ID,
		Email:       u.Email,
		Name:        u.Name,
		Role:        string(u.Role),
		MFAEnforced: u.MFAEnforced,
		LastLoginAt: u.LastLoginAt,
	}
}

// BackofficeLoginResponse is returned on successful login or refresh.
type BackofficeLoginResponse struct {
	AccessToken string             `json:"access_token"`
	TokenType   string             `json:"token_type"`
	ExpiresIn   int                `json:"expires_in"`
	User        *BackofficeProfile `json:"user"`
}

// BackofficeMFARequiredResponse is the Phase-4 placeholder shape the
// login handler will return when MFAEnforced=true on the back-office
// user. Phase 2 never returns this — the placeholder branch is wired
// behind a TODO so the eventual MFA work knows exactly where to hook in.
type BackofficeMFARequiredResponse struct {
	MFARequired bool   `json:"mfa_required"`
	Email       string `json:"email"`
}

// BackofficeLogoutResponse is the trivial body returned by logout.
type BackofficeLogoutResponse struct {
	Message string `json:"message"`
}

// BackofficeAuthAPI handles the back-office auth endpoints.
type BackofficeAuthAPI struct {
	backofficeUserRegistry registry.BackofficeUserRegistry
	refreshTokenRegistry   registry.BackofficeRefreshTokenRegistry
	blacklistService       services.TokenBlacklister
	rateLimiter            services.AuthRateLimiter
	auditService           services.AuditLogger
	jwtSecret              []byte
}

// BackofficeAuthParams holds the wiring for the back-office auth router.
type BackofficeAuthParams struct {
	BackofficeUserRegistry         registry.BackofficeUserRegistry
	BackofficeRefreshTokenRegistry registry.BackofficeRefreshTokenRegistry
	BlacklistService               services.TokenBlacklister
	RateLimiter                    services.AuthRateLimiter
	AuditService                   services.AuditLogger
	JWTSecret                      []byte
}

// BackofficeAuth mounts the back-office auth routes. The login, refresh,
// and logout handlers are public (no Bearer auth) but live behind the
// same per-IP AuthLoginRateLimitMiddleware tenant /auth/login uses
// — share the limiter so an attacker who exhausts one plane's lockout
// budget doesn't also need to exhaust the other.
//
// /me is the only authenticated route here — it uses RequireBackofficeAuth
// rather than RequireAuth so a tenant-plane access token can NEVER load
// a back-office profile (and vice versa).
func BackofficeAuth(params BackofficeAuthParams) func(r chi.Router) {
	api := &BackofficeAuthAPI{
		backofficeUserRegistry: params.BackofficeUserRegistry,
		refreshTokenRegistry:   params.BackofficeRefreshTokenRegistry,
		blacklistService:       params.BlacklistService,
		rateLimiter:            params.RateLimiter,
		auditService:           params.AuditService,
		jwtSecret:              params.JWTSecret,
	}

	requireBackofficeAuth := RequireBackofficeAuth(
		params.JWTSecret,
		params.BackofficeUserRegistry,
		params.BlacklistService,
	)
	return func(r chi.Router) {
		// Rate-limit the password endpoints exactly like the tenant
		// equivalents — same limiter instance is wired in from APIServer().
		r.With(AuthLoginRateLimitMiddleware(params.RateLimiter)).Post("/login", api.login)
		r.Post("/refresh", api.refresh)
		r.Post("/logout", api.logout)
		// /me is the only route that requires a back-office bearer
		// token — every other route is unauthenticated by design (login
		// runs before authentication, refresh runs on cookie + Bearer).
		r.With(requireBackofficeAuth).Get("/me", api.handleGetCurrentUser)
	}
}

// login handles back-office authentication.
// @Summary Back-office login
// @Description Authenticate a back-office (platform-operator) user with email + password. Issues a `backoffice` aud access token in the body and sets a `backoffice_refresh_token` cookie at `/api/v1/backoffice`.
// @Tags backoffice-auth
// @Accept json
// @Produce json
// @Param data body BackofficeLoginRequest true "Login credentials"
// @Success 200 {object} BackofficeLoginResponse "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized - invalid credentials"
// @Failure 403 {string} string "Forbidden - account disabled"
// @Failure 429 {string} string "Too Many Requests - account locked"
// @Router /backoffice/auth/login [post]
func (api *BackofficeAuthAPI) login(w http.ResponseWriter, r *http.Request) {
	var req BackofficeLoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Email == "" || req.Password == "" {
		http.Error(w, "Email and password are required", http.StatusBadRequest)
		return
	}

	// Account lockout enforcement matches the tenant plane — same
	// rate-limiter instance, per-email key so a distributed brute-force
	// against the same email is throttled regardless of which plane it
	// targets.
	if api.rateLimiter != nil {
		locked, resetAt, err := api.rateLimiter.IsAccountLocked(r.Context(), req.Email)
		if err != nil {
			// Fail-open: don't make auth unavailable due to limiter
			// backend outages. Mirrors the tenant plane's stance.
			slog.Error("Failed to check backoffice account lockout", "error", err)
		} else if locked {
			retryAfter := max(int(time.Until(resetAt).Seconds()), 0)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			api.logAuth(r.Context(), backofficeActionLoginFailed, "", false, r, new("account locked"))
			http.Error(w, "Too many failed login attempts. Please try again later.", http.StatusTooManyRequests)
			return
		}
	}

	user, err := api.backofficeUserRegistry.GetByEmail(r.Context(), req.Email)
	if err != nil {
		// User-not-found and wrong-password share the same response so
		// an attacker can't enumerate emails. Failed login is recorded
		// against the email the client typed.
		slog.Warn("Backoffice login: user not found", "email", req.Email)
		api.maybeRecordFailedLogin(r.Context(), req.Email)
		api.logAuth(r.Context(), backofficeActionLoginFailed, "", false, r, new("user not found"))
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		slog.Warn("Backoffice login: invalid password", "email", req.Email, "user_id", user.ID)
		api.maybeRecordFailedLogin(r.Context(), req.Email)
		api.logAuth(r.Context(), backofficeActionLoginFailed, user.ID, false, r, new("invalid password"))
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if !user.IsActive {
		slog.Warn("Backoffice login: account disabled", "email", req.Email, "user_id", user.ID)
		api.logAuth(r.Context(), backofficeActionLoginFailed, user.ID, false, r, new("account disabled"))
		http.Error(w, "Account disabled", http.StatusForbidden)
		return
	}

	// Clear any prior failed-login counter on success — same as tenant
	// plane.
	if api.rateLimiter != nil {
		if err := api.rateLimiter.ClearFailedLogins(r.Context(), req.Email); err != nil {
			slog.Error("Failed to clear backoffice failed login counters", "error", err)
		}
	}

	// MFA gate placeholder (Phase 4). Today the flag is data-only on
	// BackofficeUser; the MFA challenge flow lands in Phase 4 which will
	// turn this branch into a step-1 mfa_token response identical in
	// shape to the tenant equivalent. Until then we let MFAEnforced=true
	// users in directly so Phase 2 is usable; Phase 4 flips the default.
	//
	// TODO(#1785 Phase 4): replace this no-op branch with a back-office
	// MFA challenge mint + return BackofficeMFARequiredResponse.
	if user.MFAEnforced {
		slog.Debug("Backoffice login: MFA enforced but not yet wired (Phase 4)", "user_id", user.ID)
	}

	if !api.mintAndRespondAfterAuth(w, r, user, backofficeActionLogin) {
		return
	}
}

// mintAndRespondAfterAuth persists a refresh-token row, mints an access
// token, sets the cookie, stamps last_login_at, audits the success, and
// writes the LoginResponse. Returns false when an internal step
// failed and an HTTP error has already been written.
//
// Extracted so the (future) MFA-step-2 handler in Phase 4 can call the
// same finalisation path without duplicating the persist/mint dance.
func (api *BackofficeAuthAPI) mintAndRespondAfterAuth(w http.ResponseWriter, r *http.Request, user *models.BackofficeUser, action string) bool {
	rti, rawRefreshToken, err := api.persistRefreshToken(r.Context(), r, user)
	if err != nil {
		slog.Error("Failed to issue backoffice refresh token", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return false
	}

	accessToken, _, err := api.issueAccessToken(user, rti)
	if err != nil {
		slog.Error("Failed to issue backoffice access token", "user_id", user.ID, "error", err)
		api.rollbackRefreshToken(r.Context(), user.ID, rti)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return false
	}

	api.setRefreshTokenCookie(w, r, rawRefreshToken)

	now := time.Now()
	if err := api.backofficeUserRegistry.UpdateLastLogin(r.Context(), user.ID, now); err != nil {
		// Best-effort — a failed last_login_at update must not fail
		// the login.
		slog.Error("Failed to stamp backoffice last_login_at", "user_id", user.ID, "error", err)
	} else {
		user.LastLoginAt = &now
	}

	api.logAuth(r.Context(), action, user.ID, true, r, nil)
	api.writeLoginResponse(w, accessToken, user)
	return true
}

// refresh issues a new access token using a valid back-office refresh
// cookie. Mirrors /auth/refresh but is strictly scoped to the
// back-office plane — it never reads `refresh_token`, only
// `backoffice_refresh_token`.
// @Summary Refresh back-office access token
// @Description Issue a new back-office access token using the `backoffice_refresh_token` cookie.
// @Tags backoffice-auth
// @Produce json
// @Success 200 {object} BackofficeLoginResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /backoffice/auth/refresh [post]
func (api *BackofficeAuthAPI) refresh(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(backofficeRefreshTokenCookieName)
	if err != nil {
		http.Error(w, "Refresh token required", http.StatusUnauthorized)
		return
	}

	tokenHash := hashBackofficeRefreshToken(cookie.Value)

	refreshToken, err := api.refreshTokenRegistry.GetByHash(r.Context(), tokenHash)
	if err != nil {
		slog.Warn("Backoffice refresh: token not found")
		clearBackofficeRefreshCookie(w, r)
		http.Error(w, "Invalid refresh token", http.StatusUnauthorized)
		return
	}

	if !refreshToken.IsValid() {
		slog.Warn("Backoffice refresh: token expired or revoked", "token_id", refreshToken.ID)
		clearBackofficeRefreshCookie(w, r)
		http.Error(w, "Refresh token expired or revoked", http.StatusUnauthorized)
		return
	}

	user, err := api.backofficeUserRegistry.Get(r.Context(), refreshToken.BackofficeUserID)
	if err != nil || !user.IsActive {
		slog.Warn("Backoffice refresh: user missing/inactive", "user_id", refreshToken.BackofficeUserID)
		clearBackofficeRefreshCookie(w, r)
		http.Error(w, "User not found or inactive", http.StatusUnauthorized)
		return
	}

	// Reject refresh if a user-level blacklist (set on password change)
	// predates the token. Mirrors the tenant flow.
	if api.blacklistService != nil {
		since, blacklisted, blErr := api.blacklistService.UserBlacklistedSince(r.Context(), backofficeBlacklistUserKey(user.ID))
		if blErr != nil {
			slog.Error("Failed to check backoffice user blacklist on refresh", "user_id", user.ID, "error", blErr)
		} else if blacklisted && refreshToken.CreatedAt.Unix() <= since.Unix() {
			slog.Warn("Backoffice refresh: blacklisted user attempted refresh", "user_id", user.ID)
			clearBackofficeRefreshCookie(w, r)
			http.Error(w, "User not found or inactive", http.StatusUnauthorized)
			return
		}
	}

	accessToken, _, err := api.issueAccessToken(user, refreshToken.ID)
	if err != nil {
		slog.Error("Failed to issue backoffice access token on refresh", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	refreshToken.LastUsedAt = &now
	if _, err := api.refreshTokenRegistry.Update(r.Context(), *refreshToken); err != nil {
		slog.Error("Failed to update backoffice refresh token last_used_at", "token_id", refreshToken.ID, "error", err)
	}

	api.logAuth(r.Context(), backofficeActionRefresh, user.ID, true, r, nil)
	api.writeLoginResponse(w, accessToken, user)
}

// logout revokes the current refresh token (matched by cookie) and
// blacklists the access token's `jti` until its `exp`.
// @Summary Back-office logout
// @Description Revoke the current back-office session's refresh token and clear the cookie.
// @Tags backoffice-auth
// @Produce json
// @Success 200 {object} BackofficeLogoutResponse "OK"
// @Router /backoffice/auth/logout [post]
func (api *BackofficeAuthAPI) logout(w http.ResponseWriter, r *http.Request) {
	var adminID string
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		api.blacklistAccessToken(r.Context(), authHeader)
		adminID, _ = api.adminIDFromAccessTokenHeader(authHeader)
	}

	if cookie, err := r.Cookie(backofficeRefreshTokenCookieName); err == nil {
		api.revokeRefreshTokenByRaw(r.Context(), cookie.Value)
	}

	if adminID != "" {
		api.logAuth(r.Context(), backofficeActionLogout, adminID, true, r, nil)
	}

	clearBackofficeRefreshCookie(w, r)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(BackofficeLogoutResponse{Message: "Logged out successfully"}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// handleGetCurrentUser returns the back-office user's profile.
// @Summary Get current back-office user
// @Description Return the currently authenticated back-office user's profile.
// @Tags backoffice-auth
// @Produce json
// @Success 200 {object} BackofficeProfile "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /backoffice/auth/me [get]
func (api *BackofficeAuthAPI) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := appctx.BackofficeUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(backofficeProfileFromUser(user)); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// -----------------------------------------------------------------------
// Internal helpers
// -----------------------------------------------------------------------

// issueAccessToken mints a `backoffice`-aud access token. Mirrors
// AuthAPI.issueAccessToken but stamps `admin_id` (not `user_id`) and
// `aud = "backoffice"` so the cross-plane guards on both middlewares
// catch any replay attempt.
func (api *BackofficeAuthAPI) issueAccessToken(user *models.BackofficeUser, rti string) (string, time.Time, error) {
	expiresAt := time.Now().Add(backofficeAccessTokenExpiration)
	claims := jwt.MapClaims{
		"jti":        uuid.New().String(),
		"admin_id":   user.ID,
		"role":       string(user.Role),
		"aud":        backofficeTokenAudience,
		"token_type": accessTokenType,
		"exp":        expiresAt.Unix(),
		"iat":        time.Now().Unix(),
	}
	if rti != "" {
		claims["rti"] = rti
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(api.jwtSecret)
	return signed, expiresAt, err
}

// persistRefreshToken stores a fresh refresh-token row, returning the
// row id (used as the `rti` claim) and the raw token string (cookie
// value). Mirrors AuthAPI.persistRefreshToken.
func (api *BackofficeAuthAPI) persistRefreshToken(ctx context.Context, r *http.Request, user *models.BackofficeUser) (id, rawToken string, err error) {
	if api.refreshTokenRegistry == nil {
		return "", "", nil
	}

	raw, tokenHash, err := generateBackofficeRefreshToken()
	if err != nil {
		return "", "", err
	}

	rt := models.BackofficeRefreshToken{
		BackofficeUserID: user.ID,
		TokenHash:        tokenHash,
		ExpiresAt:        time.Now().Add(backofficeRefreshTokenExpiration),
		IPAddress:        clientIPTruncated(r),
		UserAgent:        r.UserAgent(),
	}

	created, err := api.refreshTokenRegistry.Create(ctx, rt)
	if err != nil {
		return "", "", err
	}
	return created.ID, raw, nil
}

// setRefreshTokenCookie writes the back-office refresh cookie.
func (api *BackofficeAuthAPI) setRefreshTokenCookie(w http.ResponseWriter, r *http.Request, rawToken string) {
	if rawToken == "" {
		return
	}
	writeBackofficeRefreshCookie(w, r, rawToken, int(backofficeRefreshTokenExpiration.Seconds()))
}

// rollbackRefreshToken revokes a refresh-token row that was persisted
// in preparation for a login response that won't be sent (because the
// access mint failed in between).
func (api *BackofficeAuthAPI) rollbackRefreshToken(ctx context.Context, userID, rowID string) {
	if api.refreshTokenRegistry == nil || rowID == "" {
		return
	}
	if err := api.refreshTokenRegistry.Revoke(ctx, userID, rowID); err != nil {
		slog.Error("Failed to roll back backoffice refresh token after access mint failure",
			"user_id", userID, "row_id", rowID, "error", err)
	}
}

// revokeRefreshTokenByRaw looks up a row by raw cookie value and revokes it.
func (api *BackofficeAuthAPI) revokeRefreshTokenByRaw(ctx context.Context, rawToken string) {
	if api.refreshTokenRegistry == nil || rawToken == "" {
		return
	}
	tokenHash := hashBackofficeRefreshToken(rawToken)
	rt, err := api.refreshTokenRegistry.GetByHash(ctx, tokenHash)
	if err != nil {
		return
	}
	if err := api.refreshTokenRegistry.Revoke(ctx, rt.BackofficeUserID, rt.ID); err != nil {
		slog.Error("Failed to revoke backoffice refresh token", "token_id", rt.ID, "error", err)
	}
}

// adminIDFromAccessTokenHeader pulls the `admin_id` claim out of a
// (possibly expired) Bearer token. Used by logout so an expired-token
// logout still records the admin actor.
func (api *BackofficeAuthAPI) adminIDFromAccessTokenHeader(authHeader string) (string, bool) {
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return "", false
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return api.jwtSecret, nil
	})
	if token == nil {
		return "", false
	}
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
		return "", false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", false
	}
	adminID, ok := claims["admin_id"].(string)
	return adminID, ok && adminID != ""
}

// blacklistAccessToken extracts the JTI from a Bearer header and
// blacklists it until its `exp`. Capped at 2× backofficeAccessTokenExpiration
// as defence-in-depth against an artificially large exp.
func (api *BackofficeAuthAPI) blacklistAccessToken(ctx context.Context, authHeader string) {
	if api.blacklistService == nil {
		return
	}
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader {
		return
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return api.jwtSecret, nil
	})
	if token == nil {
		return
	}
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
	expiresAt := time.Unix(int64(exp), 0)
	maxExpiry := time.Now().Add(2 * backofficeAccessTokenExpiration)
	if expiresAt.After(maxExpiry) {
		expiresAt = maxExpiry
	}
	if err := api.blacklistService.BlacklistToken(ctx, jti, expiresAt); err != nil {
		slog.Error("Failed to blacklist backoffice access token", "error", err)
	}
}

// writeLoginResponse encodes the login/refresh JSON response.
func (api *BackofficeAuthAPI) writeLoginResponse(w http.ResponseWriter, accessToken string, user *models.BackofficeUser) {
	w.Header().Set("Content-Type", "application/json")
	resp := BackofficeLoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(backofficeAccessTokenExpiration.Seconds()),
		User:        backofficeProfileFromUser(user),
	}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
	}
}

// logAuth is a nil-safe wrapper around AuditLogger.LogAuth scoped to the
// back-office plane. userID is a string (not pointer) so the call site
// reads obvious; an empty userID is normalised to nil here.
func (api *BackofficeAuthAPI) logAuth(ctx context.Context, action, userID string, success bool, r *http.Request, errMsg *string) {
	if api.auditService == nil {
		return
	}
	var userIDPtr *string
	if userID != "" {
		uid := userID
		userIDPtr = &uid
	}
	api.auditService.LogAuth(ctx, services.AuthEvent{
		Action:  action,
		UserID:  userIDPtr,
		Success: success,
		Request: r,
		ErrMsg:  errMsg,
	})
}

// maybeRecordFailedLogin mirrors AuthAPI.maybeRecordFailedLogin —
// no-op when the limiter is nil.
func (api *BackofficeAuthAPI) maybeRecordFailedLogin(ctx context.Context, email string) {
	if api.rateLimiter == nil {
		return
	}
	if _, _, err := api.rateLimiter.RecordFailedLogin(ctx, email); err != nil {
		slog.Error("Failed to record backoffice failed login", "error", err)
	}
}

// -----------------------------------------------------------------------
// Free-function helpers
// -----------------------------------------------------------------------

// generateBackofficeRefreshToken mints a fresh cryptographically-random
// refresh token + its SHA-256 hash. Mirrors models.GenerateRefreshToken
// shape; kept package-local so the back-office plane never accidentally
// reaches for a tenant-side helper.
func generateBackofficeRefreshToken() (token, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	hash = base64.RawURLEncoding.EncodeToString(h[:])
	return token, hash, nil
}

// hashBackofficeRefreshToken computes the SHA-256 hash of a raw token
// (mirrors models.HashRefreshToken; see generateBackofficeRefreshToken
// for the package-locality reason).
func hashBackofficeRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// backofficeBlacklistUserKey produces the user-blacklist key for the
// back-office plane. Prefixed so it can never collide with a tenant
// user_id even if the two id spaces ever overlap.
func backofficeBlacklistUserKey(adminID string) string {
	return "backoffice:" + adminID
}

// writeBackofficeRefreshCookie writes the refresh cookie under the
// back-office-specific name + path. Secure mirrors the tenant logic —
// true on HTTPS, false on plain-HTTP local dev.
func writeBackofficeRefreshCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	secureCookie := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	// #nosec G124 -- HttpOnly + SameSiteStrict are set; Secure is true on HTTPS and intentionally false on plain-HTTP local dev.
	http.SetCookie(w, &http.Cookie{
		Name:     backofficeRefreshTokenCookieName,
		Value:    value,
		Path:     backofficeRefreshTokenCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearBackofficeRefreshCookie deletes the cookie by setting MaxAge=-1.
func clearBackofficeRefreshCookie(w http.ResponseWriter, r *http.Request) {
	secureCookie := r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	// #nosec G124 -- HttpOnly + SameSiteStrict are set; Secure is true on HTTPS and intentionally false on plain-HTTP local dev.
	http.SetCookie(w, &http.Cookie{
		Name:     backofficeRefreshTokenCookieName,
		Value:    "",
		Path:     backofficeRefreshTokenCookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteStrictMode,
	})
}
