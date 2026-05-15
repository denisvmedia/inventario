package apiserver

import (
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

// MFAStatusResponse describes the user's enrollment state. GET-only —
// driven by the SettingsPage Privacy & Security row's Active/Inactive
// badge.
type MFAStatusResponse struct {
	Enabled        bool       `json:"enabled"`
	EnrollmentInProgress bool `json:"enrollment_in_progress"`
	EnabledAt      *time.Time `json:"enabled_at,omitempty"`
	LastUsedAt     *time.Time `json:"last_used_at,omitempty"`
	BackupCodesRemaining int  `json:"backup_codes_remaining"`
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
	resp := MFAStatusResponse{}
	if api.mfaRegistry != nil {
		row, err := api.mfaRegistry.GetByUser(r.Context(), user.TenantID, user.ID)
		switch {
		case errors.Is(err, registry.ErrNotFound):
			// no enrollment — defaults are fine
		case err != nil:
			slog.Error("MFA status lookup failed", "user_id", user.ID, "error", err)
			http.Error(w, "Failed to read MFA status", http.StatusInternalServerError)
			return
		default:
			resp.Enabled = row.IsEnabled()
			resp.EnrollmentInProgress = !row.IsEnabled()
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

	ok, err := api.mfaService.VerifyTOTP(*row, req.Code)
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

	ok, err := api.mfaService.VerifyTOTP(*row, req.Code)
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

	plain, hashes, err := api.mfaService.GenerateBackupCodes(services.MFABackupCodeCount)
	if err != nil {
		slog.Error("MFA regenerate: backup codes failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to regenerate codes", http.StatusInternalServerError)
		return
	}
	// Update LastUsedAt alongside the new code set — the user just
	// proved possession of a valid TOTP, so the row's "last used"
	// timestamp should reflect that, matching how loginMFA / mfa_disable
	// touch LastUsedAt on every successful verification (#1645 review).
	now := time.Now()
	row.BackupCodesHashed = models.ValuerSlice[string](hashes)
	row.LastUsedAt = &now
	if _, err := api.mfaRegistry.Update(r.Context(), *row); err != nil {
		slog.Error("MFA regenerate: persist failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to regenerate codes", http.StatusInternalServerError)
		return
	}
	api.logAuth(r.Context(), "mfa_regenerate", &user.ID, &user.TenantID, true, r, nil)
	writeJSON(w, http.StatusOK, MFAVerifyResponse{BackupCodes: plain})
}

// handleLoginMFA is the step-2 endpoint: validates the mfa_token from
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

	userID, tenantID, err := api.parseMFAToken(req.MFAToken)
	if err != nil {
		slog.Warn("MFA login: invalid mfa_token", "error", err)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	user, err := api.userRegistry.Get(r.Context(), userID)
	if err != nil || !user.IsActive || user.TenantID != tenantID {
		slog.Warn("MFA login: user lookup failed", "user_id", userID, "error", err)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	row, err := api.mfaRegistry.GetByUser(r.Context(), tenantID, userID)
	if err != nil || !row.IsEnabled() {
		// Token referenced a user whose MFA was disabled mid-flight,
		// or the row vanished. Treat as a generic challenge failure.
		errMsg := "mfa not enabled at completion"
		api.logAuth(r.Context(), "login_mfa", &userID, &tenantID, false, r, &errMsg)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	if !api.consumeAnyMFACode(r, user, row, req.TOTPCode, req.BackupCode, "login_mfa") {
		api.recordLoginEvent(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeBadMFA, r)
		http.Error(w, "Invalid code", http.StatusUnauthorized)
		return
	}

	accessToken, _, err := api.issueAccessToken(user)
	if err != nil {
		slog.Error("MFA login: access token failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}
	if err := api.issueRefreshTokenCookie(w, r, user); err != nil {
		slog.Error("MFA login: refresh token failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	now := time.Now()
	user.LastLoginAt = &now
	if _, err := api.userRegistry.Update(r.Context(), *user); err != nil {
		slog.Error("Failed to update user last login time", "user_id", user.ID, "error", err)
	}

	csrfToken := api.generateCSRFTokenForUser(r.Context(), user.ID)
	api.logAuth(r.Context(), "login_mfa", &user.ID, &user.TenantID, true, r, nil)
	api.recordLoginEvent(r.Context(), tenantID, user.Email, &user.ID, models.LoginOutcomeOK, r)
	writeLoginResponse(w, accessToken, csrfToken, user)
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
		ok, err := api.mfaService.VerifyTOTP(*row, totpCode)
		if err != nil {
			slog.Error("MFA verify path: totp check failed", "user_id", user.ID, "error", err)
			return false
		}
		if ok {
			now := time.Now()
			row.LastUsedAt = &now
			if _, err := api.mfaRegistry.Update(ctx, *row); err != nil {
				slog.Error("MFA verify path: persist last_used_at failed", "user_id", user.ID, "error", err)
			}
			return true
		}
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

// parseMFAToken decodes a token previously issued by issueMFAToken
// and returns the (user_id, tenant_id) it carries. Rejects tokens of
// the wrong type, expired, or with mismatched signing keys.
func (api *AuthAPI) parseMFAToken(tokenString string) (userID, tenantID string, err error) {
	parsed, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return api.jwtSecret, nil
	})
	if err != nil {
		return "", "", err
	}
	claims, ok := parsed.Claims.(jwt.MapClaims)
	if !ok || !parsed.Valid {
		return "", "", errors.New("invalid claims")
	}
	if ty, _ := claims["token_type"].(string); ty != mfaTokenType {
		return "", "", errors.New("wrong token type")
	}
	userID, _ = claims["user_id"].(string)
	tenantID, _ = claims["tenant_id"].(string)
	if userID == "" || tenantID == "" {
		return "", "", errors.New("missing identity claims")
	}
	return userID, tenantID, nil
}

