package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// Naming convention: every handler/type in this file is part of the
// passwordless "magic link" sign-in flow. It lives on *AuthAPI so it can
// reuse the unexported session + MFA helpers (maybeIssueMFAChallenge,
// persistRefreshToken, issueAccessToken, setRefreshTokenCookie,
// generateCSRFTokenForUser, …) directly, without re-exporting them.

const (
	// magicLinkLoginExpiration is how long a magic-link sign-in token
	// remains valid. A bearer credential that grants a full session, so
	// much shorter than the 1h reset token, but longer than the 5-min MFA
	// window to tolerate mail-delivery latency.
	magicLinkLoginExpiration = 15 * time.Minute
)

// MagicLinkRequest is the body for POST /auth/magic-link/request.
type MagicLinkRequest struct {
	Email string `json:"email"`
}

// MagicLinkVerifyRequest is the body for POST /auth/magic-link/verify.
type MagicLinkVerifyRequest struct {
	Token string `json:"token"`
}

// requestMagicLink accepts an email and sends a one-time sign-in link.
// Always responds with a neutral success to prevent user-enumeration.
//
// @Summary Request a magic-link sign-in
// @Description Send a passwordless sign-in link to the given email address. Always returns 200 to prevent email enumeration. Returns 404 when magic-link login is disabled for this deployment.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body MagicLinkRequest true "Email address"
// @Success 200 {object} map[string]string "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 404 {string} string "Not Found - magic-link login disabled"
// @Failure 429 {string} string "Too Many Requests"
// @Router /auth/magic-link/request [post]
func (api *AuthAPI) requestMagicLink(w http.ResponseWriter, r *http.Request) {
	if !api.magicLinkLoginEnabled {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var req MagicLinkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}

	// Neutral response regardless of account existence or state.
	successMsg := "If that email address is registered you will receive a sign-in link shortly."

	tenantID := TenantIDFromContext(r.Context())

	user, err := api.userRegistry.GetByEmail(r.Context(), tenantID, req.Email)
	switch {
	case errors.Is(err, registry.ErrNotFound):
		api.logAuth(r.Context(), "magic_link_request_unknown", nil, &tenantID, false, r, nil)
		writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
		return
	case err != nil:
		// Real DB error — log it, but still return the neutral response.
		slog.ErrorContext(r.Context(), "failed to look up user by email for magic link",
			"email", req.Email, "error", err)
		api.logAuth(r.Context(), "magic_link_request_error", nil, &tenantID, false, r, nil)
		writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
		return
	case user == nil || !user.IsActive:
		// Defensive nil + inactive accounts get the same neutral 200.
		api.logAuth(r.Context(), "magic_link_request_inactive", nil, &tenantID, false, r, nil)
		writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
		return
	}

	api.sendMagicLink(r, user)
	api.logAuth(r.Context(), "magic_link_request", &user.ID, &user.TenantID, true, r, nil)
	writeJSON(w, http.StatusOK, map[string]string{"message": successMsg})
}

// verifyMagicLink claims a sign-in token and completes login by minting the
// same access/refresh/CSRF tokens the password path issues — or hands off to
// the existing MFA challenge when the user has TOTP enrolled.
//
// @Summary Verify a magic-link sign-in
// @Description Exchange a one-time sign-in token for a session. Returns the same shapes as /auth/login (LoginResponse, or LoginMFARequiredResponse when MFA is enabled). Returns 404 when magic-link login is disabled.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body MagicLinkVerifyRequest true "Sign-in token"
// @Success 200 {object} LoginResponse "OK"
// @Failure 400 {string} string "Bad Request - missing, invalid, or expired token"
// @Failure 403 {string} string "Forbidden - account disabled"
// @Failure 404 {string} string "Not Found - magic-link login disabled"
// @Failure 429 {string} string "Too Many Requests - account locked"
// @Router /auth/magic-link/verify [post]
func (api *AuthAPI) verifyMagicLink(w http.ResponseWriter, r *http.Request) {
	if !api.magicLinkLoginEnabled {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	var req MagicLinkVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		http.Error(w, "Sign-in token is required", http.StatusBadRequest)
		return
	}

	tenantID := TenantIDFromContext(r.Context())
	if tenantID == "" {
		slog.Error("Magic-link verify attempted without tenant context in request")
		http.Error(w, "Tenant context not established", http.StatusInternalServerError)
		return
	}

	// Claim first (race-safe): MarkClaimed atomically flips claimed_at only
	// for a still-unclaimed, non-expired row. A false result covers unknown /
	// replayed / expired / lost-race in one branch.
	claimed, err := api.magicLinkRegistry.MarkClaimed(r.Context(), req.Token)
	if err != nil {
		slog.Error("Failed to claim magic-link token", "error", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if !claimed {
		http.Error(w, "Invalid or expired sign-in link", http.StatusBadRequest)
		return
	}

	mlt, err := api.magicLinkRegistry.GetByToken(r.Context(), req.Token)
	if err != nil {
		slog.Error("Failed to load claimed magic-link token", "error", err)
		http.Error(w, "Invalid or expired sign-in link", http.StatusBadRequest)
		return
	}
	// Belt-and-braces: MarkClaimed already folds in the expiry check, but
	// re-assert it after the read in case the row's clock skewed.
	if mlt.IsExpired() {
		http.Error(w, "Invalid or expired sign-in link", http.StatusBadRequest)
		return
	}
	// Tenant guard: the token must belong to the resolved tenant (mirrors
	// the loginMFA claims check).
	if mlt.TenantID != tenantID {
		slog.Warn("Magic-link verify: tenant mismatch", "user_id", mlt.UserID)
		http.Error(w, "Invalid or expired sign-in link", http.StatusBadRequest)
		return
	}

	user, err := api.userRegistry.Get(r.Context(), mlt.UserID)
	if err != nil {
		slog.Error("Failed to look up user for magic-link verify", "user_id", mlt.UserID, "error", err)
		http.Error(w, "Invalid or expired sign-in link", http.StatusBadRequest)
		return
	}
	// Blocked/disabled accounts are refused at verify even with a valid token
	// (mirrors login()).
	if !user.IsActive {
		slog.Warn("Magic-link verify: account disabled", "user_id", user.ID)
		http.Error(w, "User account disabled", http.StatusForbidden)
		return
	}

	// Account-lockout enforced at verify (requesting a link while locked is
	// still allowed — refusing would leak lockout state).
	if !api.checkMFALoginLockout(w, r, user.Email) {
		return
	}

	// Clear any stale failed-login counters now that the link has verified,
	// mirroring the success branch of login(): otherwise a prior password
	// typo's count survives and the next typo could immediately re-lock an
	// account that just proved control of the inbox.
	if api.rateLimiter != nil {
		if err := api.rateLimiter.ClearFailedLogins(r.Context(), user.Email); err != nil {
			slog.Error("Failed to clear failed login counters after magic-link auth", "user_id", user.ID, "error", err)
		}
	}

	// Invalidate any other outstanding links for this user.
	if delErr := api.magicLinkRegistry.DeleteByUserID(r.Context(), user.ID); delErr != nil {
		slog.Warn("Failed to clean up magic-link tokens", "user_id", user.ID, "error", delErr)
	}

	// MFA hand-off (load-bearing): if the user has TOTP enrolled, return the
	// short-lived mfa_token and let the client finish via the existing
	// POST /auth/login/mfa. Magic link is never an MFA bypass.
	if api.maybeIssueMFAChallenge(w, r, user, tenantID) {
		return
	}

	api.issueMagicLinkSession(w, r, user, tenantID)
}

// sendMagicLink generates a token, stores it, and sends the sign-in email.
// Mirrors PasswordResetAPI.sendPasswordReset.
func (api *AuthAPI) sendMagicLink(r *http.Request, user *models.User) {
	if api.emailService == nil {
		return
	}
	// Supersede any existing pending links before issuing a new one.
	if err := api.magicLinkRegistry.DeleteByUserID(r.Context(), user.ID); err != nil {
		slog.Warn("Failed to delete old magic-link tokens", "user_id", user.ID, "error", err)
	}

	token, err := models.GenerateMagicLinkToken()
	if err != nil {
		slog.Error("Failed to generate magic-link token", "user_id", user.ID, "error", err)
		return
	}
	mlt := models.MagicLinkToken{
		UserID:    user.ID,
		TenantID:  TenantIDFromContext(r.Context()),
		Email:     user.Email,
		Token:     token,
		ExpiresAt: time.Now().Add(magicLinkLoginExpiration),
	}
	if _, err := api.magicLinkRegistry.Create(r.Context(), mlt); err != nil {
		slog.Error("Failed to store magic-link record", "user_id", user.ID, "error", err)
		return
	}

	signInQuery := url.Values{"token": {token}}
	signInURL := "/magic-link?" + signInQuery.Encode()
	if api.publicBaseURL == "" {
		slog.Warn("Public base URL is not configured; using relative magic-link URL", "user_id", user.ID)
	} else {
		signInURL, err = buildPublicURL(api.publicBaseURL, "/magic-link", signInQuery)
		if err != nil {
			slog.Error("Failed to build magic-link URL", "user_id", user.ID, "error", err)
			return
		}
	}

	// Never pass plain r.Context() directly to this async email send.
	// The request may already be cancelled while the sign-in email still must
	// be sent, so use context.WithoutCancel(r.Context()) to preserve
	// request-scoped values without inheriting cancellation.
	emailCtx := context.WithoutCancel(r.Context())
	go func() {
		ctx, cancel := context.WithTimeout(emailCtx, detachedAuthEmailTimeout)
		defer cancel()
		if err := api.emailService.SendMagicLinkEmail(ctx, user.Email, user.Name, signInURL); err != nil {
			slog.Error("Failed to send magic-link email", "user_id", user.ID, "error", err)
		}
	}()
}

// issueMagicLinkSession mints the access + refresh + CSRF tokens, updates
// last-login, and writes the LoginResponse — the non-MFA completion path for a
// verified magic link. Mirrors issueMFALoginSession, sourcing the tenant from
// the request context rather than mfa_token claims and stamping a
// magic_link_login audit verb.
func (api *AuthAPI) issueMagicLinkSession(w http.ResponseWriter, r *http.Request, user *models.User, tenantID string) {
	rti, rawRefreshToken, err := api.persistRefreshToken(r.Context(), r, user)
	if err != nil {
		slog.Error("Magic-link login: refresh token failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	accessToken, _, err := api.issueAccessToken(r.Context(), user, rti)
	if err != nil {
		slog.Error("Magic-link login: access token failed", "user_id", user.ID, "error", err)
		api.rollbackRefreshToken(r.Context(), user.ID, rti)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	api.setRefreshTokenCookie(w, r, rawRefreshToken)

	now := time.Now()
	user.LastLoginAt = &now
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to update user last login time", "user_id", user.ID, "error", err)
	}

	csrfToken := api.generateCSRFTokenForUser(r.Context(), user.ID)
	slog.Info("Successful magic-link login", "email", user.Email, "user_id", user.ID)
	api.logAuth(r.Context(), "magic_link_login", &user.ID, &user.TenantID, true, r, nil)
	userID := user.ID
	api.recordLoginEventWithMethod(r.Context(), tenantID, user.Email, &userID, models.LoginOutcomeOK, models.LoginMethodMagicLink, r)

	// Stamp the wire-only is_system_admin advisory flag (#1784).
	populateUserSystemAdminFlag(r.Context(), api.systemAdminGrantRegistry, user)

	writeLoginResponse(w, accessToken, csrfToken, user)
}
