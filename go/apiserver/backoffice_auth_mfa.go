package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/models"
)

// Back-office MFA login completion (issue #1785, Phase 4).
//
// This is the step-2 sibling of POST /backoffice/auth/login. Step-1
// (in backoffice_auth.go) verifies the password and, when the user has
// `mfa_enforced=true` AND an enabled enrollment row, mints a short-lived
// challenge JWT and returns it in the 200 response body. Step-2
// (this file) validates the challenge token, validates a TOTP or backup
// code, and completes the login by minting the standard access + refresh
// tokens.
//
// The challenge token is signed with the same shared JWT secret as the
// access tokens — rotating the secret invalidates everything, which is
// the desired blast radius. The `aud` claim is what isolates the planes
// from each other: any back-office JWT carries `aud == "backoffice"`,
// and the step-2 handler additionally enforces a
// `token_type == "backoffice_mfa_challenge"` claim so a back-office
// ACCESS token can never be replayed at /login/mfa.

// loginMFA completes a back-office login with MFA.
// @Summary Complete back-office login with MFA
// @Description Exchange a short-lived MFA challenge token + TOTP/backup code for a back-office access token. Issues a `backoffice` aud access token in the body and sets a `backoffice_refresh_token` cookie at `/api/v1/backoffice`.
// @Tags backoffice-auth
// @Accept json
// @Produce json
// @Param data body BackofficeLoginMFARequest true "MFA challenge response"
// @Success 200 {object} BackofficeLoginResponse "OK"
// @Failure 400 {string} string "Bad Request"
// @Failure 401 {string} string "Unauthorized — invalid token or code"
// @Failure 429 {string} string "Too Many Requests — account locked"
// @Failure 501 {string} string "MFA not configured"
// @Router /backoffice/auth/login/mfa [post]
func (api *BackofficeAuthAPI) loginMFA(w http.ResponseWriter, r *http.Request) {
	var req BackofficeLoginMFARequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	if req.MFAToken == "" || (req.TOTPCode == "" && req.BackupCode == "") {
		http.Error(w, "mfa_token and one of totp_code or backup_code are required", http.StatusBadRequest)
		return
	}
	if api.mfaService == nil || api.mfaRegistry == nil {
		http.Error(w, "MFA not configured", http.StatusNotImplemented)
		return
	}

	claims, err := api.parseBackofficeMFAToken(req.MFAToken)
	if err != nil {
		slog.Warn("Backoffice login_mfa: invalid mfa_token", "error", err)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	user, err := api.backofficeUserRegistry.Get(r.Context(), claims.AdminID)
	if err != nil || !user.IsActive {
		slog.Warn("Backoffice login_mfa: user lookup failed", "admin_id", claims.AdminID, "error", err)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	// Account-lockout check on the user's email — same key the step-1
	// limiter uses, so step-1 + step-2 failures share a single budget.
	if api.rateLimiter != nil {
		locked, resetAt, lockErr := api.rateLimiter.IsAccountLocked(r.Context(), user.Email)
		if lockErr != nil {
			slog.Error("Backoffice login_mfa: rate-limiter lookup failed", "error", lockErr)
		} else if locked {
			retryAfter := max(int(time.Until(resetAt).Seconds()), 0)
			w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
			http.Error(w, "Too many failed login attempts. Please try again later.", http.StatusTooManyRequests)
			return
		}
	}

	row, err := api.mfaRegistry.Get(r.Context(), claims.AdminID)
	if err != nil || !row.IsEnabled() {
		// Enrollment vanished between step-1 and step-2 (admin tooling
		// wiped the row, etc.). Treat as a generic challenge failure.
		errMsg := "mfa enrollment missing at step-2"
		api.logAuth(r.Context(), backofficeActionLoginMFAFailed, user.ID, false, r, &errMsg)
		http.Error(w, "Invalid or expired MFA token", http.StatusUnauthorized)
		return
	}

	if !api.consumeBackofficeMFACode(r, user, row, req.TOTPCode, req.BackupCode) {
		api.maybeRecordFailedLogin(r.Context(), user.Email)
		errMsg := "invalid mfa code"
		api.logAuth(r.Context(), backofficeActionLoginMFAFailed, user.ID, false, r, &errMsg)
		http.Error(w, "Invalid code", http.StatusUnauthorized)
		return
	}

	// Clear the failed-login counter and blacklist the consumed
	// challenge token's jti so it can never be replayed within its
	// 5-minute TTL.
	api.afterBackofficeMFASuccess(r.Context(), user.Email, &claims)

	if !api.mintAndRespondAfterAuth(w, r, user, backofficeActionLoginMFACompleted) {
		return
	}
}

// consumeBackofficeMFACode tries the TOTP code first, then the backup
// code. On a successful TOTP match it bumps LastUsedAt (the backup-code
// path does that internally via ConsumeBackupCodeAtomic). Returns true
// iff one of the codes verified.
func (api *BackofficeAuthAPI) consumeBackofficeMFACode(r *http.Request, user *models.BackofficeUser, row *models.BackofficeUserMFASecret, totpCode, backupCode string) bool {
	ctx := r.Context()
	if totpCode != "" {
		// Reuse the existing MFAService verifier — the TOTP secret
		// envelope is identical to the tenant-plane secret, so the
		// same VerifyTOTP path applies. We construct a thin shim
		// models.UserMFASecret that carries just the encrypted secret;
		// the verifier only reads SecretEncrypted.
		shim := models.UserMFASecret{SecretEncrypted: row.SecretEncrypted}
		ok, err := api.mfaService.VerifyTOTP(shim, totpCode)
		if err != nil {
			slog.Error("Backoffice login_mfa: TOTP verify failed", "user_id", user.ID, "error", err)
			return false
		}
		if ok {
			if bumpErr := api.mfaRegistry.BumpLastUsedAt(ctx, user.ID, time.Now()); bumpErr != nil {
				// Non-fatal — the user still authenticated.
				slog.Error("Backoffice login_mfa: failed to bump last_used_at", "user_id", user.ID, "error", bumpErr)
			}
			return true
		}
	}
	if backupCode != "" {
		matcher := api.mfaService.MatchBackupCode(backupCode)
		if matcher != nil {
			ok, err := api.mfaRegistry.ConsumeBackupCodeAtomic(ctx, user.ID, time.Now(), matcher)
			if err != nil {
				slog.Error("Backoffice login_mfa: atomic backup-code consume failed", "user_id", user.ID, "error", err)
				return false
			}
			if ok {
				return true
			}
		}
	}
	return false
}

// afterBackofficeMFASuccess clears the step-2 failed-attempt counter
// and blacklists the consumed mfa_token's jti so the same challenge
// can't be replayed within its 5-minute TTL.
func (api *BackofficeAuthAPI) afterBackofficeMFASuccess(ctx context.Context, email string, claims *backofficeMFATokenClaims) {
	if api.rateLimiter != nil {
		if err := api.rateLimiter.ClearFailedLogins(ctx, email); err != nil {
			slog.Error("Backoffice login_mfa: clear failed-login counter", "error", err)
		}
	}
	if api.blacklistService != nil && claims.JTI != "" && !claims.ExpiresAt.IsZero() {
		if err := api.blacklistService.BlacklistToken(ctx, claims.JTI, claims.ExpiresAt); err != nil {
			// Not fatal — session still issues. Replay risk is scoped
			// to <5 min, single-use code already consumed.
			slog.Error("Backoffice login_mfa: failed to blacklist mfa_token jti", "error", err)
		}
	}
}

// issueBackofficeMFAToken signs a short-lived JWT that authorizes the
// step-2 endpoint AND ONLY that endpoint. The token carries:
//
//   - aud = "backoffice" — so the tenant-plane MFA endpoint (which
//     enforces aud != backoffice via parseMFAToken's token_type check)
//     can never accidentally honour it.
//   - token_type = "backoffice_mfa_challenge" — distinct from the
//     tenant plane's `mfa_challenge`, so an attacker can't reuse a
//     stolen tenant token here even if they manipulate the aud claim.
//   - admin_id — points at backoffice_users.id (NOT user_id, which
//     would collide with the tenant identity namespace).
//
// The JWT secret is shared with the access-token path on purpose:
// rotating it invalidates everything in one move.
func (api *BackofficeAuthAPI) issueBackofficeMFAToken(user *models.BackofficeUser) (string, time.Time, error) {
	expiresAt := time.Now().Add(backofficeMFATokenExpiration)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"jti":        uuid.New().String(),
		"admin_id":   user.ID,
		"aud":        backofficeTokenAudience,
		"token_type": backofficeMFATokenType,
		"exp":        expiresAt.Unix(),
		"iat":        time.Now().Unix(),
	})
	signed, err := token.SignedString(api.jwtSecret)
	return signed, expiresAt, err
}

// backofficeMFATokenClaims is the unpacked form of a successfully-
// validated back-office mfa_token. Mirrors mfaTokenClaims in
// auth_mfa.go but keys on `admin_id` instead of `user_id`.
type backofficeMFATokenClaims struct {
	AdminID   string
	JTI       string
	ExpiresAt time.Time
}

// parseBackofficeMFAToken decodes a token previously issued by
// issueBackofficeMFAToken. Rejects tokens with the wrong aud, wrong
// token_type, missing/expired exp, missing admin_id, or a non-HMAC
// signing alg. Mirrors auth_mfa.go::parseMFAToken's defence-in-depth
// checks so the back-office step-2 path holds the same invariants.
func (api *BackofficeAuthAPI) parseBackofficeMFAToken(tokenString string) (backofficeMFATokenClaims, error) {
	var out backofficeMFATokenClaims
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
	// Explicit exp validation — same defence as auth_mfa.go: the JWT
	// lib will accept a token with no exp claim, which is not the
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
	if aud, _ := claims["aud"].(string); aud != backofficeTokenAudience {
		return out, errors.New("wrong audience")
	}
	if ty, _ := claims["token_type"].(string); ty != backofficeMFATokenType {
		return out, errors.New("wrong token type")
	}
	out.AdminID, _ = claims["admin_id"].(string)
	if out.AdminID == "" {
		return out, errors.New("missing admin_id claim")
	}
	out.JTI, _ = claims["jti"].(string)
	out.ExpiresAt = time.Unix(int64(expFloat), 0)
	return out, nil
}
