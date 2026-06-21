package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// Naming convention: every handler/type in this file is part of the
// TOTP/MFA flow added for #1645. Keeping the concern in a sibling
// file rather than expanding auth.go to ~1300 lines.

// MFA-flow JWT lifetime and claim layout. Short on purpose: a 5-minute
// window forces the user to complete step-2 in the same session and
// limits replay if the step-1 transcript leaks.
const (
	mfaTokenExpiration = 5 * time.Minute
	mfaTokenType       = "mfa_challenge"
)

// MFA login response shape, used by both step-1 (initial login that
// short-circuits before issuing the access token) and the step-2
// endpoint when it fails.
//
// `reason` mirrors the OAuth convention so the FE can branch on a
// well-known string instead of HTTP status alone.

// LoginMFARequiredResponse is the 200 body returned when the user has
// MFA enabled. The FE treats this as a virtual 401 — it hides
// credentials and presents the code-entry surface. We return 200
// rather than 401 because credentials were correct; the *step* of
// auth, not its outcome, is incomplete.
type LoginMFARequiredResponse struct {
	MFARequired bool   `json:"mfa_required"`
	MFAToken    string `json:"mfa_token"`
	ExpiresIn   int    `json:"expires_in"`
	// Email is echoed so the FE can render "Continue as X" without
	// re-asking. No other user fields are leaked at this stage.
	Email string `json:"email"`
}

// LoginMFARequest is the body for POST /auth/login/mfa.
type LoginMFARequest struct {
	MFAToken   string `json:"mfa_token"`
	TOTPCode   string `json:"totp_code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
}

// MFAState is the three-state enum for the user's MFA enrollment.
// Encoded as a single string field on MFAStatusResponse so the FE
// can switch on a discriminator instead of decoding a pair of bools.
type MFAState string

const (
	// MFAStateNone — no row exists. The user has never enrolled.
	MFAStateNone MFAState = "none"
	// MFAStatePending — row exists but EnabledAt is null. The user
	// called /auth/mfa/setup but never verified the first code.
	MFAStatePending MFAState = "pending"
	// MFAStateActive — row exists and is verified. Login is gated.
	MFAStateActive MFAState = "active"
)

// MFAStatusResponse describes the user's enrollment state. GET-only —
// driven by the SettingsPage Privacy & Security row's Active/Inactive
// badge. The `state` field is the canonical discriminator; downstream
// code that wants the older bool shape can derive it from `state`.
type MFAStatusResponse struct {
	State                MFAState   `json:"state"`
	EnabledAt            *time.Time `json:"enabled_at,omitempty"`
	LastUsedAt           *time.Time `json:"last_used_at,omitempty"`
	BackupCodesRemaining int        `json:"backup_codes_remaining"`
}

// MFASetupResponse returns the secret + provisioning URL to the FE.
// Both are *only* shown during setup; once verified the row's
// SecretEncrypted is never returned to any API surface again.
type MFASetupResponse struct {
	Secret          string `json:"secret"`
	ProvisioningURL string `json:"qr_code_url"`
}

// MFAVerifyRequest carries the code typed during enrollment.
type MFAVerifyRequest struct {
	Code string `json:"code"`
}

// MFAVerifyResponse returns the 10 backup codes — shown once.
type MFAVerifyResponse struct {
	BackupCodes []string `json:"backup_codes"`
}

// MFADisableRequest requires both a password and a fresh
// TOTP/backup code, mirroring the issue's "re-auth" requirement.
type MFADisableRequest struct {
	Password   string `json:"password"`
	TOTPCode   string `json:"totp_code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
}

// MFARegenerateResponse echoes a fresh set of backup codes.
type MFARegenerateResponse = MFAVerifyResponse

// handleMFAStatus reports the user's enrollment state.
// @Summary Get MFA status
// @Description Return whether the authenticated user has TOTP enrolled and enabled.
// @Tags auth
// @Produce json
// @Success 200 {object} MFAStatusResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/mfa/status [get]
func (api *AuthAPI) handleMFAStatus(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	resp := MFAStatusResponse{State: MFAStateNone}
	if api.mfaRegistry != nil {
		row, err := api.mfaRegistry.GetByUser(r.Context(), user.TenantID, user.ID)
		switch {
		case errors.Is(err, registry.ErrNotFound):
			// no enrollment — default state ("none") is correct.
		case err != nil:
			slog.Error("MFA status lookup failed", "user_id", user.ID, "error", err)
			http.Error(w, "Failed to read MFA status", http.StatusInternalServerError)
			return
		default:
			if row.IsEnabled() {
				resp.State = MFAStateActive
			} else {
				resp.State = MFAStatePending
			}
			resp.EnabledAt = row.EnabledAt
			resp.LastUsedAt = row.LastUsedAt
			resp.BackupCodesRemaining = len(row.BackupCodesHashed)
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleMFASetup mints a new TOTP secret and returns the QR URL. Does
// NOT enable MFA — the user must call /auth/mfa/verify with a valid
// code to flip EnabledAt. Calling setup twice rotates the secret —
// any in-progress enrollment from a stale device is discarded.
// @Summary Begin MFA enrollment
// @Description Generate a TOTP secret for the authenticated user and return the QR provisioning URL. Does not enable MFA yet — a follow-up call to /auth/mfa/verify is required.
// @Tags auth
// @Produce json
// @Success 200 {object} MFASetupResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/mfa/setup [post]
func (api *AuthAPI) handleMFASetup(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.mfaService == nil || api.mfaRegistry == nil {
		http.Error(w, "MFA not configured", http.StatusServiceUnavailable)
		return
	}

	// If the user already has MFA fully enabled, refuse to re-issue a
	// secret. Disable first, then re-enroll — this prevents a stolen
	// session from silently replacing the secret while leaving the
	// user thinking their device is still bound.
	existing, err := api.mfaRegistry.GetByUser(r.Context(), user.TenantID, user.ID)
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		slog.Error("MFA setup precheck failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to read MFA state", http.StatusInternalServerError)
		return
	}
	if existing != nil && existing.IsEnabled() {
		http.Error(w, "MFA already enabled — disable before re-enrolling", http.StatusConflict)
		return
	}

	enrollment, err := api.mfaService.GenerateEnrollment(user.Email)
	if err != nil {
		slog.Error("MFA setup: generate enrollment failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to start MFA setup", http.StatusInternalServerError)
		return
	}
	encrypted, err := api.mfaService.EncryptSecret(enrollment.Secret)
	if err != nil {
		slog.Error("MFA setup: encrypt secret failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to store MFA secret", http.StatusInternalServerError)
		return
	}

	// Re-create the row to ensure a clean slate: previous in-progress
	// secrets, partially consumed backup codes, or last_used timestamps
	// all get cleared.
	if existing != nil {
		if err := api.mfaRegistry.DeleteByUser(r.Context(), user.TenantID, user.ID); err != nil {
			slog.Error("MFA setup: failed to clear previous enrollment", "user_id", user.ID, "error", err)
			http.Error(w, "Failed to start MFA setup", http.StatusInternalServerError)
			return
		}
	}

	row := models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		SecretEncrypted:   encrypted,
		BackupCodesHashed: models.ValuerSlice[string]{},
	}
	if _, err := api.mfaRegistry.Create(r.Context(), row); err != nil {
		slog.Error("MFA setup: persist row failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to start MFA setup", http.StatusInternalServerError)
		return
	}

	api.logAuth(r.Context(), "mfa_setup_started", &user.ID, &user.TenantID, true, r, nil)
	writeJSON(w, http.StatusOK, MFASetupResponse{
		Secret:          enrollment.Secret,
		ProvisioningURL: enrollment.ProvisioningURL,
	})
}

// handleMFAVerify confirms the first valid code. Flips EnabledAt and
// returns the freshly-minted 10 backup codes (shown once).
// @Summary Complete MFA enrollment
// @Description Verify the first TOTP code, enable MFA, and return single-use backup codes (shown ONCE).
// @Tags auth
// @Accept json
// @Produce json
// @Param data body MFAVerifyRequest true "Verification code"
// @Success 200 {object} MFAVerifyResponse "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized — invalid code"
// @Router /auth/mfa/verify [post]
func (api *AuthAPI) handleMFAVerify(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.mfaService == nil || api.mfaRegistry == nil {
		http.Error(w, "MFA not configured", http.StatusServiceUnavailable)
		return
	}

	var req MFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Code) == "" {
		http.Error(w, "Code is required", http.StatusBadRequest)
		return
	}

	row, err := api.mfaRegistry.GetByUser(r.Context(), user.TenantID, user.ID)
	if errors.Is(err, registry.ErrNotFound) {
		http.Error(w, "Call /auth/mfa/setup first", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("MFA verify: load row failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to verify MFA", http.StatusInternalServerError)
		return
	}

	_, ok, err := api.mfaService.VerifyTOTPStep(*row, req.Code)
	if err != nil {
		slog.Error("MFA verify: totp check failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to verify MFA", http.StatusInternalServerError)
		return
	}
	if !ok {
		errMsg := "invalid mfa code"
		api.logAuth(r.Context(), "mfa_verify", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Invalid code", http.StatusUnauthorized)
		return
	}

	// Mint backup codes BEFORE flipping EnabledAt — if hashing fails we
	// don't want a half-enrolled row.
	plain, hashes, err := api.mfaService.GenerateBackupCodes(services.MFABackupCodeCount)
	if err != nil {
		slog.Error("MFA verify: backup code generation failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to verify MFA", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	row.EnabledAt = &now
	row.LastUsedAt = &now
	// Enrollment intentionally does NOT advance last_used_step. The replay
	// guard (#2124) covers the endpoints where replaying a sniffed code has
	// value — login, disable, regenerate — which share the monotonic step
	// ledger. Enrollment is a one-time, already-authenticated, idempotent
	// pending→enabled confirmation; bumping it here would only block a
	// legitimate user who manages MFA within the same 30s step, for no real
	// gain (an enrollment-code→login replay also needs the victim's password).
	row.BackupCodesHashed = models.ValuerSlice[string](hashes)
	if _, err := api.mfaRegistry.Update(r.Context(), *row); err != nil {
		slog.Error("MFA verify: persist row failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to verify MFA", http.StatusInternalServerError)
		return
	}

	api.logAuth(r.Context(), "mfa_verify", &user.ID, &user.TenantID, true, r, nil)
	writeJSON(w, http.StatusOK, MFAVerifyResponse{BackupCodes: plain})
}

// handleMFADisable removes the user's MFA row after re-authenticating
// with password + (TOTP or backup code). Refreshing tokens is NOT
// invalidated — the user's session remains valid.
// @Summary Disable MFA
// @Description Remove the user's MFA enrollment. Requires password + a current TOTP code or unused backup code.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body MFADisableRequest true "Disable request"
// @Success 200 {object} map[string]string "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/mfa/disable [post]
func (api *AuthAPI) handleMFADisable(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.mfaService == nil || api.mfaRegistry == nil {
		http.Error(w, "MFA not configured", http.StatusServiceUnavailable)
		return
	}

	var req MFADisableRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if !services.VerifyPassword(user, req.Password) {
		errMsg := "wrong password during mfa disable"
		api.logAuth(r.Context(), "mfa_disable", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	row, err := api.mfaRegistry.GetByUser(r.Context(), user.TenantID, user.ID)
	if errors.Is(err, registry.ErrNotFound) || (err == nil && !row.IsEnabled()) {
		// Idempotent: disabling an already-disabled account is a no-op.
		writeJSON(w, http.StatusOK, map[string]string{"message": "MFA disabled"})
		return
	}
	if err != nil {
		slog.Error("MFA disable: load row failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to disable MFA", http.StatusInternalServerError)
		return
	}

	if !api.consumeAnyMFACode(r, user, row, req.TOTPCode, req.BackupCode, "mfa_disable") {
		http.Error(w, "Invalid code", http.StatusUnauthorized)
		return
	}

	if err := api.mfaRegistry.DeleteByUser(r.Context(), user.TenantID, user.ID); err != nil {
		slog.Error("MFA disable: delete row failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to disable MFA", http.StatusInternalServerError)
		return
	}

	// Invalidate every existing session for the user — same shape as
	// change-password (auth.go::handleChangePassword). The threat
	// model: if disable was triggered because the account was
	// compromised and an attacker has a stolen access/refresh token,
	// leaving those live would extend the breach. The user just
	// re-authed (password + TOTP/backup) so re-logging in afterward
	// is a one-step ritual, not a silent foot-gun.
	if api.refreshTokenRegistry != nil {
		if revErr := api.refreshTokenRegistry.RevokeByUserID(r.Context(), user.ID); revErr != nil {
			slog.Error("MFA disable: failed to revoke refresh tokens", "user_id", user.ID, "error", revErr)
		}
	}
	if authHeader := r.Header.Get("Authorization"); authHeader != "" {
		api.blacklistAccessToken(r.Context(), authHeader)
	}
	if api.blacklistService != nil {
		if blErr := api.blacklistService.BlacklistUserTokens(r.Context(), user.ID, 2*accessTokenExpiration); blErr != nil {
			slog.Error("MFA disable: failed to blacklist user tokens", "user_id", user.ID, "error", blErr)
		}
	}

	api.logAuth(r.Context(), "mfa_disable", &user.ID, &user.TenantID, true, r, nil)
	writeJSON(w, http.StatusOK, map[string]string{"message": "MFA disabled"})
}

// handleMFARegenerateBackupCodes mints a fresh set of backup codes,
// invalidating any previously-issued unused codes. Requires the user
// to prove possession of a current TOTP code so a stolen session
// can't quietly replace codes the legitimate user is relying on.
// @Summary Regenerate MFA backup codes
// @Description Invalidate the existing backup codes and return a fresh set. Requires a current TOTP code.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body MFAVerifyRequest true "Current TOTP code"
// @Success 200 {object} MFAVerifyResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/mfa/regenerate-backup-codes [post]
func (api *AuthAPI) handleMFARegenerateBackupCodes(w http.ResponseWriter, r *http.Request) {
	user := appctx.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}
	if api.mfaService == nil || api.mfaRegistry == nil {
		http.Error(w, "MFA not configured", http.StatusServiceUnavailable)
		return
	}

	var req MFAVerifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	row, err := api.mfaRegistry.GetByUser(r.Context(), user.TenantID, user.ID)
	if errors.Is(err, registry.ErrNotFound) || (err == nil && !row.IsEnabled()) {
		http.Error(w, "MFA not enabled", http.StatusBadRequest)
		return
	}
	if err != nil {
		slog.Error("MFA regenerate: load row failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to regenerate codes", http.StatusInternalServerError)
		return
	}

	matchedStep, ok, err := api.mfaService.VerifyTOTPStep(*row, req.Code)
	if err != nil {
		slog.Error("MFA regenerate: totp check failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to regenerate codes", http.StatusInternalServerError)
		return
	}
	if !ok {
		errMsg := "invalid code during regenerate-backup-codes"
		api.logAuth(r.Context(), "mfa_regenerate", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Invalid code", http.StatusUnauthorized)
		return
	}
	// Replay guard (RFC 6238 §5.2, #2124): commit the matched step via the
	// atomic CAS before regenerating codes. A replayed TOTP (step already
	// consumed) loses the CAS and is rejected like a wrong code, so a
	// stolen session can't reuse a single sniffed code to churn the
	// legitimate user's backup codes.
	now := time.Now()
	won, casErr := api.mfaRegistry.MarkTOTPStepUsedAtomic(r.Context(), user.TenantID, user.ID, matchedStep, now)
	if casErr != nil {
		slog.Error("MFA regenerate: totp step CAS failed", "user_id", user.ID, "error", casErr)
		http.Error(w, "Failed to regenerate codes", http.StatusInternalServerError)
		return
	}
	if !won {
		errMsg := "replayed code during regenerate-backup-codes"
		api.logAuth(r.Context(), "mfa_regenerate", &user.ID, &user.TenantID, false, r, &errMsg)
		http.Error(w, "Invalid code", http.StatusUnauthorized)
		return
	}

	plain, hashes, err := api.mfaService.GenerateBackupCodes(services.MFABackupCodeCount)
	if err != nil {
		slog.Error("MFA regenerate: backup codes failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to regenerate codes", http.StatusInternalServerError)
		return
	}
	// Carry the step + timestamp the CAS just committed into the in-memory
	// row so the full-row Update below doesn't revert last_used_step /
	// last_used_at to their pre-CAS values.
	row.BackupCodesHashed = models.ValuerSlice[string](hashes)
	row.LastUsedAt = &now
	row.LastUsedStep = matchedStep
	if _, err := api.mfaRegistry.Update(r.Context(), *row); err != nil {
		slog.Error("MFA regenerate: persist failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to regenerate codes", http.StatusInternalServerError)
		return
	}
	api.logAuth(r.Context(), "mfa_regenerate", &user.ID, &user.TenantID, true, r, nil)
	writeJSON(w, http.StatusOK, MFAVerifyResponse{BackupCodes: plain})
}

// loginMFA is the step-2 endpoint: validates the mfa_token from
// step-1, validates a TOTP code OR a backup code, and completes login
// by minting the same access/refresh/CSRF tokens the password-only
// path issues.
// @Summary Complete login with MFA
// @Description Exchange a short-lived mfa_token + TOTP/backup code for an access token.
// @Tags auth
// @Accept json
// @Produce json
// @Param data body LoginMFARequest true "MFA challenge response"
// @Success 200 {object} LoginResponse "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized"
// @Router /auth/login/mfa [post]
func (api *AuthAPI) loginMFA(w http.ResponseWriter, r *http.Request) {
	var req LoginMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.MFAToken == "" || (req.TOTPCode == "" && req.BackupCode == "") {
		http.Error(w, "mfa_token and one of totp_code or backup_code are required", http.StatusBadRequest)
		return
	}
	if api.mfaService == nil || api.mfaRegistry == nil {
		http.Error(w, "MFA not configured", http.StatusServiceUnavailable)
		return
	}

	claims, err := api.parseMFAToken(req.MFAToken)
	if err != nil {
		slog.Warn("MFA login: invalid mfa_token", "error", err)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	user, err := api.userRegistry.Get(r.Context(), claims.UserID)
	if err != nil || !user.IsActive || user.TenantID != claims.TenantID {
		slog.Warn("MFA login: user lookup failed", "user_id", claims.UserID, "error", err)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	if !api.checkMFALoginLockout(w, r, user.Email) {
		return
	}

	row, err := api.mfaRegistry.GetByUser(r.Context(), claims.TenantID, claims.UserID)
	if err != nil || !row.IsEnabled() {
		// Token referenced a user whose MFA was disabled mid-flight,
		// or the row vanished. Treat as a generic challenge failure.
		errMsg := "mfa not enabled at completion"
		api.logAuth(r.Context(), "login_mfa", &claims.UserID, &claims.TenantID, false, r, &errMsg)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	if !api.consumeAnyMFACode(r, user, row, req.TOTPCode, req.BackupCode, "login_mfa") {
		api.maybeRecordFailedLogin(r.Context(), user.Email)
		api.recordLoginEvent(r.Context(), claims.TenantID, user.Email, &user.ID, models.LoginOutcomeBadMFA, r)
		http.Error(w, "Invalid code", http.StatusUnauthorized)
		return
	}

	api.afterMFALoginSuccess(r.Context(), user.Email, user.ID, &claims)

	if !api.issueMFALoginSession(w, r, user, &claims) {
		return
	}
}

// checkMFALoginLockout returns false (and writes the 429 response)
// when the step-2 limiter says we should reject this attempt outright.
// Per-account lockout shares the same Account_locked path as /auth/login
// because the middleware can't extract user_id from the short-lived
// mfa_token; keying on email keeps the two windows additive.
func (api *AuthAPI) checkMFALoginLockout(w http.ResponseWriter, r *http.Request, email string) bool {
	if api.rateLimiter == nil {
		return true
	}
	locked, resetAt, lockErr := api.rateLimiter.IsAccountLocked(r.Context(), email)
	if lockErr != nil {
		slog.Error("MFA login: rate-limiter lookup failed", "error", lockErr)
		return true
	}
	if !locked {
		return true
	}
	retryAfter := max(int(time.Until(resetAt).Seconds()), 0)
	w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	http.Error(w, "Too many failed login attempts. Please try again later.", http.StatusTooManyRequests)
	return false
}

// afterMFALoginSuccess clears the step-2 failed-attempt counter and
// blacklists the consumed mfa_token's jti so the same challenge
// can't be replayed within its 5-minute TTL.
func (api *AuthAPI) afterMFALoginSuccess(ctx context.Context, email, userID string, claims *mfaTokenClaims) {
	if api.rateLimiter != nil {
		if clrErr := api.rateLimiter.ClearFailedLogins(ctx, email); clrErr != nil {
			slog.Error("MFA login: clear failed-login counter", "error", clrErr)
		}
	}
	if api.blacklistService != nil && claims.JTI != "" && !claims.ExpiresAt.IsZero() {
		if blErr := api.blacklistService.BlacklistToken(ctx, claims.JTI, claims.ExpiresAt); blErr != nil {
			// Not fatal — session still issues. Replay risk is
			// scoped to <5 min, single-use code already consumed.
			slog.Error("MFA login: failed to blacklist mfa_token jti", "user_id", userID, "error", blErr)
		}
	}
}

// issueMFALoginSession mints the access + refresh tokens, updates
// last-login, and writes the LoginResponse. Returns false if any of
// the token mints failed (caller has already written the error).
func (api *AuthAPI) issueMFALoginSession(w http.ResponseWriter, r *http.Request, user *models.User, claims *mfaTokenClaims) bool {
	// Persist refresh row first so issueAccessToken can pin "rti" — same
	// reasoning as login(): /users/me/sessions can't read the refresh
	// cookie, so the access token carries the row id explicitly. Cookie
	// is set AFTER the access token mints successfully to avoid leaving
	// the client with a valid session cookie if mint fails.
	rti, rawRefreshToken, err := api.persistRefreshToken(r.Context(), r, user)
	if err != nil {
		slog.Error("MFA login: refresh token failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return false
	}
	accessToken, _, err := api.issueAccessToken(r.Context(), user, rti)
	if err != nil {
		slog.Error("MFA login: access token failed", "user_id", user.ID, "error", err)
		api.rollbackRefreshToken(r.Context(), user.ID, rti)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return false
	}
	api.setRefreshTokenCookie(w, r, rawRefreshToken)
	now := time.Now()
	user.LastLoginAt = &now
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to update user last login time", "user_id", user.ID, "error", err)
	}

	csrfToken := api.generateCSRFTokenForUser(r.Context(), user.ID)
	api.logAuth(r.Context(), "login_mfa", &user.ID, &user.TenantID, true, r, nil)
	api.recordLoginEvent(r.Context(), claims.TenantID, user.Email, &user.ID, models.LoginOutcomeOK, r)

	// Stamp the wire-only is_system_admin advisory flag (#1784).
	populateUserSystemAdminFlag(r.Context(), api.systemAdminGrantRegistry, user)

	writeLoginResponse(w, accessToken, csrfToken, user)
	return true
}

// consumeAnyMFACode tries the TOTP code first, then the backup code,
// and on a successful backup-code match persists the updated
// remaining-codes slice. Returns true iff one of the codes verified.
//
// The *http.Request is threaded through so failed attempts keep IP +
// User-Agent in the audit log (#1645 review). Backup-code consumption
// goes through ConsumeBackupCodeAtomic — a row-level lock around the
// "find matching hash → rewrite slice" sequence so two concurrent
// requests racing on the same code can never both succeed.
func (api *AuthAPI) consumeAnyMFACode(r *http.Request, user *models.User, row *models.UserMFASecret, totpCode, backupCode, action string) bool {
	ctx := r.Context()
	if totpCode != "" {
		consumed, abort := api.consumeTOTPCode(ctx, user, row, totpCode)
		if abort {
			return false
		}
		if consumed {
			return true
		}
		// A non-consumed TOTP — a wrong code, or a replayed code that lost
		// the CAS — falls through to the backup-code path and the shared
		// invalid-code audit/return below, so a replay is logged and
		// rejected identically to a wrong code.
	}
	if backupCode != "" {
		matcher := api.mfaService.MatchBackupCode(backupCode)
		if matcher != nil {
			ok, err := api.mfaRegistry.ConsumeBackupCodeAtomic(
				ctx, user.TenantID, user.ID, time.Now(), matcher,
			)
			if err != nil {
				slog.Error("MFA verify path: atomic backup-code consume failed", "user_id", user.ID, "error", err)
				return false
			}
			if ok {
				return true
			}
		}
	}
	errMsg := "invalid mfa code"
	api.logAuth(ctx, action, &user.ID, &user.TenantID, false, r, &errMsg)
	return false
}

// consumeTOTPCode verifies totpCode against the user's secret and, on a
// match, commits the matched time-step via the atomic replay-guard CAS
// (#2124, RFC 6238 §5.2). consumed is true only when the code both
// verified and WON the CAS — i.e. a fresh, non-replayed code. A replayed
// code (the same step re-presented within the ±1-step skew window, or a
// concurrent racer with an identical code) loses the CAS and returns
// consumed=false. abort is true on an already-logged infrastructure
// failure, signalling the caller to reject the request. The CAS also
// stamps last_used_at, so no separate bump is needed.
func (api *AuthAPI) consumeTOTPCode(ctx context.Context, user *models.User, row *models.UserMFASecret, totpCode string) (consumed, abort bool) {
	matchedStep, ok, err := api.mfaService.VerifyTOTPStep(*row, totpCode)
	if err != nil {
		slog.Error("MFA verify path: totp check failed", "user_id", user.ID, "error", err)
		return false, true
	}
	if !ok {
		return false, false
	}
	won, casErr := api.mfaRegistry.MarkTOTPStepUsedAtomic(ctx, user.TenantID, user.ID, matchedStep, time.Now())
	if casErr != nil {
		slog.Error("MFA verify path: totp step CAS failed", "user_id", user.ID, "error", casErr)
		return false, true
	}
	return won, false
}

// issueMFAToken signs a short-lived JWT that authorizes the step-2
// endpoint and *only* that endpoint. We piggyback on api.jwtSecret
// because rotating that already invalidates all access tokens — so
// using it here doesn't expand the rotation blast radius.
func (api *AuthAPI) issueMFAToken(user *models.User) (string, time.Time, error) {
	expiresAt := time.Now().Add(mfaTokenExpiration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        uuid.New().String(),
		"user_id":    user.ID,
		"tenant_id":  user.TenantID,
		"token_type": mfaTokenType,
		"exp":        expiresAt.Unix(),
		"iat":        time.Now().Unix(),
	})
	signed, err := token.SignedString(api.jwtSecret)
	return signed, expiresAt, err
}

// mfaTokenClaims is the unpacked form of a successfully-validated
// mfa_token. The fields are read by loginMFA to (a) look up the user,
// (b) blacklist the jti after step-2 success so the same token can't
// be replayed within its TTL, and (c) feed the user's email into the
// rate limiter for per-account brute-force protection on /auth/login/mfa.
type mfaTokenClaims struct {
	UserID    string
	TenantID  string
	JTI       string
	ExpiresAt time.Time
}

// parseMFAToken decodes a token previously issued by issueMFAToken
// and returns the claims it carries. Rejects tokens of the wrong
// type, expired, missing the exp claim, or with mismatched signing
// keys. Mirrors validateJWTToken in jwt_middleware.go which requires
// the exp claim to be present even if the JWT library would otherwise
// accept a tokenless one.
func (api *AuthAPI) parseMFAToken(tokenString string) (mfaTokenClaims, error) {
	var out mfaTokenClaims
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return api.jwtSecret, nil
	})
	if err != nil {
		return out, err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return out, errors.New("invalid claims")
	}
	// Explicit exp validation — same defence as jwt_middleware.go: the
	// JWT lib will accept a token with no exp claim, which is not the
	// "short-lived" contract this token type promises.
	exp, ok := claims["exp"]
	if !ok {
		return out, errors.New("token missing expiration claim")
	}
	expFloat, ok := exp.(float64)
	if !ok {
		return out, errors.New("invalid expiration claim format")
	}
	if int64(expFloat) <= time.Now().Unix() {
		return out, errors.New("token expired")
	}
	if ty, _ := claims["token_type"].(string); ty != mfaTokenType {
		return out, errors.New("wrong token type")
	}
	out.UserID, _ = claims["user_id"].(string)
	out.TenantID, _ = claims["tenant_id"].(string)
	if out.UserID == "" || out.TenantID == "" {
		return out, errors.New("missing identity claims")
	}
	out.JTI, _ = claims["jti"].(string)
	out.ExpiresAt = time.Unix(int64(expFloat), 0)
	return out, nil
}
