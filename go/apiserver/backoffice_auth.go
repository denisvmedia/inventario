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
//  1. Different `aud` claim: back-office tokens carry `aud ==
//     "backoffice"`. Tenant tokens historically do not stamp an `aud`
//     claim — the back-office guard rejects anything whose `aud` is not
//     literally "backoffice" (including absent), so the asymmetry is
//     deliberate. JWTMiddleware (tenant) symmetrically rejects any token
//     whose `aud == "backoffice"`.
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
//
// COOKIE-PATH INVARIANT: every back-office HTTP surface MUST be mounted
// at a path starting with `/api/v1/backoffice` so the refresh cookie at
// `Path=/api/v1/backoffice` is delivered. Phase 3+ contributors adding
// admin endpoints under a sibling path (`/api/v1/admin/*`,
// `/api/v1/backoffice-something`, etc.) must EITHER remount them under
// `/api/v1/backoffice` OR widen this cookie path AND audit the
// cross-plane CSRF implications (a wider path means more endpoints see
// the cookie, which can break the cross-plane isolation if the wider
// region also accepts tenant credentials).
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
	// backofficeActionLoginMFACompleted is the audit action stamped on a
	// successful step-2 MFA login. Lets ops correlate the step-1 challenge
	// (`backoffice.login_mfa_required`) with the step-2 outcome on the
	// same admin row.
	backofficeActionLoginMFACompleted = "backoffice.login_mfa_completed"
	// backofficeActionLoginMFAFailed is the audit action stamped on a
	// step-2 MFA login that rejected the supplied TOTP / backup code.
	backofficeActionLoginMFAFailed = "backoffice.login_mfa_failed"
	// backofficeMFANotImplementedCode is the FE-facing JSON:API error
	// `code` returned with HTTP 501 when a back-office user has
	// `mfa_enforced=true` set in the database BUT no enrollment row
	// exists in backoffice_user_mfa_secrets. Phase 4 wired the challenge
	// flow for the happy path (row present, EnabledAt non-null); this
	// fail-closed branch survives so an operator who flips
	// `mfa_enforced=true` without running `inventario backoffice mfa setup`
	// gets a clear 501 + a stable error code instead of silently signing
	// in without MFA.
	backofficeMFANotImplementedCode = "backoffice.mfa_not_implemented"
	// backofficeMFATokenType is the `token_type` claim stamped on the
	// short-lived JWT issued by step-1 of the MFA login. The step-2
	// handler refuses any other token type so a tenant-plane MFA token
	// (mfa_challenge) can NEVER be replayed at the back-office step-2.
	backofficeMFATokenType = "backoffice_mfa_challenge"
	// backofficeMFATokenExpiration is how long the step-1 challenge token
	// stays valid. 5 minutes mirrors the tenant plane's MFA token TTL —
	// long enough for a human to switch to their authenticator app, short
	// enough to limit replay if the step-1 transcript leaks.
	backofficeMFATokenExpiration = 5 * time.Minute
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

// BackofficeMFARequiredResponse is the typed shape returned from POST
// /backoffice/auth/login when the user has `mfa_enforced=true`. Phase 4
// uses it in two distinct branches:
//
//   - 200 OK + `MFAToken` populated + `ExpiresIn` set: step-1 succeeded
//     (password verified, enrollment row present) and the FE must POST
//     the supplied token + a TOTP/backup code to /login/mfa to complete.
//   - 501 Not Implemented + `Code = backofficeMFANotImplementedCode` +
//     `MFAToken` empty: the user has `mfa_enforced=true` but no row in
//     backoffice_user_mfa_secrets. The operator must run
//     `inventario backoffice mfa setup --email <e>` before they can
//     sign in. Fail-closed by design.
//
// FE clients should branch on the HTTP status FIRST (501 → ops error)
// and on `Code` second; `MFAToken` is only present on the 200 path.
type BackofficeMFARequiredResponse struct {
	MFARequired bool   `json:"mfa_required"`
	Email       string `json:"email"`
	// MFAToken is the short-lived (5 min) step-1 token that the FE must
	// echo back to /login/mfa alongside the TOTP / backup code. Empty
	// on the 501 (not-implemented) branch.
	MFAToken string `json:"mfa_token,omitempty"`
	// ExpiresIn carries the MFAToken lifetime in seconds so the FE can
	// disable the code-entry surface when the token lapses. Zero on the
	// 501 branch.
	ExpiresIn int `json:"expires_in,omitempty"`
	// Code mirrors a stable identifier so FE clients can branch on it
	// instead of an HTTP status alone. Today only the 501 branch sets
	// `backofficeMFANotImplementedCode`; future codes can be added
	// without breaking older FE clients.
	Code string `json:"code,omitempty"`
}

// BackofficeLogoutResponse is the trivial body returned by logout.
type BackofficeLogoutResponse struct {
	Message string `json:"message"`
}

// BackofficeLoginMFARequest is the body for POST /backoffice/auth/login/mfa
// (step-2 of the MFA login dance). The client supplies the short-lived
// `mfa_token` from step-1's 200 response alongside EITHER a TOTP code
// OR a backup code — never both. The handler verifies the token, looks
// up the back-office user, validates the code, and mints the standard
// access + refresh tokens on success.
type BackofficeLoginMFARequest struct {
	MFAToken   string `json:"mfa_token"`
	TOTPCode   string `json:"totp_code,omitempty"`
	BackupCode string `json:"backup_code,omitempty"`
}

// BackofficeAuthAPI handles the back-office auth endpoints.
type BackofficeAuthAPI struct {
	backofficeUserRegistry registry.BackofficeUserRegistry
	refreshTokenRegistry   registry.BackofficeRefreshTokenRegistry
	mfaRegistry            registry.BackofficeUserMFASecretRegistry
	mfaService             *services.MFAService
	blacklistService       services.TokenBlacklister
	rateLimiter            services.AuthRateLimiter
	auditService           services.AuditLogger
	jwtSecret              []byte
}

// BackofficeAuthParams holds the wiring for the back-office auth router.
type BackofficeAuthParams struct {
	BackofficeUserRegistry          registry.BackofficeUserRegistry
	BackofficeRefreshTokenRegistry  registry.BackofficeRefreshTokenRegistry
	BackofficeUserMFASecretRegistry registry.BackofficeUserMFASecretRegistry
	MFAService                      *services.MFAService
	BlacklistService                services.TokenBlacklister
	RateLimiter                     services.AuthRateLimiter
	AuditService                    services.AuditLogger
	JWTSecret                       []byte
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
		mfaRegistry:            params.BackofficeUserMFASecretRegistry,
		mfaService:             params.MFAService,
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
		// Step-2 of the MFA login dance (issue #1785, Phase 4). Same
		// per-IP limiter as step-1 so an attacker can't bypass the
		// account-lockout budget by hammering step-2 alone.
		r.With(AuthLoginRateLimitMiddleware(params.RateLimiter)).Post("/login/mfa", api.loginMFA)
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
// @Failure 429 {string} string "Too Many Requests - account locked"
// @Failure 501 {object} BackofficeMFARequiredResponse "Not Implemented - MFA enforced but not yet wired (Phase 4)"
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
		// an attacker can't enumerate emails. To keep the timing channel
		// shut as well, run a bcrypt comparison against a fixed dummy
		// hash so the user-not-found path costs the same as a
		// wrong-password path for an existing user.
		_ = bcrypt.CompareHashAndPassword(backofficeDummyBcryptHash, []byte(req.Password))
		slog.Warn("Backoffice login: user not found", "email", req.Email)
		api.maybeRecordFailedLogin(r.Context(), req.Email)
		api.logAuth(r.Context(), backofficeActionLoginFailed, "", false, r, new("user not found"))
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check is_active BEFORE bcrypt so a disabled account does not get a
	// 300ms bcrypt round before the response. Run a fixed dummy bcrypt
	// to keep timing aligned with the active+wrong-password path and
	// surface the same 401 "Invalid credentials" body — collapsing
	// inactive into the wrong-password response stops attackers from
	// confirming that a particular operator email exists. An operator
	// whose account has been disabled is notified out-of-band; the API
	// surface intentionally leaks nothing.
	if !user.IsActive {
		_ = bcrypt.CompareHashAndPassword(backofficeDummyBcryptHash, []byte(req.Password))
		slog.Warn("Backoffice login: account disabled", "email", req.Email, "user_id", user.ID)
		// NOTE: maybeRecordFailedLogin intentionally NOT called here —
		// a disabled account cannot be unlocked by exhausting the
		// per-email budget, so attempts against it must not count
		// toward the legitimate-user lockout window. ErrBackoffice
		// AccountDisabled is the sentinel a future admin UI would use
		// to render a different status; the wire response stays 401.
		api.logAuth(r.Context(), backofficeActionLoginFailed, user.ID, false, r, new(ErrBackofficeAccountDisabled.Error()))
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

	// MFA gate (issue #1785, Phase 4). Two branches:
	//
	//   1. MFAEnforced=true AND an enabled enrollment row exists →
	//      mint a short-lived step-1 challenge token and return
	//      200 + mfa_required:true + mfa_token. The FE then POSTs to
	//      /login/mfa with the token + a TOTP / backup code.
	//
	//   2. MFAEnforced=true AND NO enrollment row (or row not yet
	//      enabled) → fail closed with 501 + a typed body. An operator
	//      must run `inventario backoffice mfa setup` before the
	//      account can sign in. Same behaviour as Phase 2's placeholder
	//      so existing FE clients on the 501 branch keep working.
	if user.MFAEnforced {
		if !api.handleBackofficeMFAGate(w, r, user) {
			return
		}
	}

	// Clear any prior failed-login counter on success — same as tenant
	// plane.
	if api.rateLimiter != nil {
		if err := api.rateLimiter.ClearFailedLogins(r.Context(), req.Email); err != nil {
			slog.Error("Failed to clear backoffice failed login counters", "error", err)
		}
	}

	if !api.mintAndRespondAfterAuth(w, r, user, backofficeActionLogin) {
		return
	}
}

// handleBackofficeMFAGate inspects the user's MFA enrollment and either
// (a) writes a 200 + step-1 challenge response and returns false, or
// (b) writes a 501 fail-closed response (no enrollment row, or row not
// yet enabled, or MFA service not configured) and returns false. Returns
// true ONLY when no MFA action is required — which today is unreachable
// because the caller only invokes this when MFAEnforced=true. Returning
// false signals the caller to abandon the login flow; the response has
// already been written.
//
// The dual return is the cleanest way to keep the caller's control flow
// readable: `if !api.handleBackofficeMFAGate(...) { return }` — same
// shape as the rate-limiter check above.
func (api *BackofficeAuthAPI) handleBackofficeMFAGate(w http.ResponseWriter, r *http.Request, user *models.BackofficeUser) bool {
	// No MFA service / registry configured → fail closed. This branch
	// fires when an operator deploys a build without wiring the
	// back-office MFA dependencies (memory-only test harness, etc.).
	// Treating "not configured" identically to "no enrollment row"
	// keeps the FE response shape consistent for the ops contract.
	if api.mfaService == nil || api.mfaRegistry == nil {
		slog.Warn("Backoffice login: MFAEnforced=true but MFA service not configured", "user_id", user.ID)
		api.logAuth(r.Context(), backofficeActionMFARequired, user.ID, false, r, new("mfa not configured"))
		api.writeMFANotImplementedResponse(w, user.Email)
		return false
	}

	row, err := api.mfaRegistry.Get(r.Context(), user.ID)
	if err != nil {
		if errors.Is(err, registry.ErrBackofficeMFASecretNotFound) {
			slog.Warn("Backoffice login: MFAEnforced=true but no enrollment row", "user_id", user.ID)
			api.logAuth(r.Context(), backofficeActionMFARequired, user.ID, false, r, new("mfa enrollment missing"))
			api.writeMFANotImplementedResponse(w, user.Email)
			return false
		}
		slog.Error("Backoffice login: MFA lookup failed", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to check MFA enrollment", http.StatusInternalServerError)
		return false
	}
	if !row.IsEnabled() {
		// A row with EnabledAt=null shouldn't happen in practice (the
		// CLI stamps it atomically with insert), but if it does we
		// treat it like a missing row: fail closed.
		slog.Warn("Backoffice login: MFAEnforced=true but enrollment row not enabled", "user_id", user.ID)
		api.logAuth(r.Context(), backofficeActionMFARequired, user.ID, false, r, new("mfa enrollment not enabled"))
		api.writeMFANotImplementedResponse(w, user.Email)
		return false
	}

	// Happy path: mint the step-1 challenge token and return 200 so the
	// FE pivots to the code-entry surface.
	token, expiresAt, err := api.issueBackofficeMFAToken(user)
	if err != nil {
		slog.Error("Backoffice login: failed to mint MFA challenge token", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to issue MFA challenge", http.StatusInternalServerError)
		return false
	}
	api.logAuth(r.Context(), backofficeActionMFARequired, user.ID, true, r, nil)
	api.writeMFAChallengeResponse(w, user.Email, token, int(time.Until(expiresAt).Seconds()))
	return false
}

// writeMFANotImplementedResponse writes the typed 501 response returned
// from login when a back-office user has MFAEnforced=true but no
// enrollment row exists (or the row is not yet enabled). The FE branches
// on `code == backoffice.mfa_not_implemented` to render an explicit "MFA
// is required on your account — contact platform admin" message.
func (api *BackofficeAuthAPI) writeMFANotImplementedResponse(w http.ResponseWriter, email string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotImplemented)
	if err := json.NewEncoder(w).Encode(BackofficeMFARequiredResponse{
		MFARequired: true,
		Email:       email,
		Code:        backofficeMFANotImplementedCode,
	}); err != nil {
		slog.Error("Failed to encode backoffice MFA-not-implemented response", "error", err)
	}
}

// writeMFAChallengeResponse writes the 200 step-1 response carrying the
// short-lived MFA token. The FE pivots to the code-entry surface and
// POSTs the token + a TOTP/backup code back to /login/mfa.
func (api *BackofficeAuthAPI) writeMFAChallengeResponse(w http.ResponseWriter, email, token string, expiresInSeconds int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(BackofficeMFARequiredResponse{
		MFARequired: true,
		Email:       email,
		MFAToken:    token,
		ExpiresIn:   expiresInSeconds,
	}); err != nil {
		slog.Error("Failed to encode backoffice MFA challenge response", "error", err)
	}
}

// backofficeDummyBcryptHash is a fixed bcrypt(DefaultCost) hash used to
// neutralise the timing difference between the user-not-found / account-
// disabled paths and the wrong-password path. The plaintext does not
// matter — the only invariant is that CompareHashAndPassword runs to
// completion (~300ms at DefaultCost) so the failing paths cost the
// same as an existing-user wrong-password call. Generated once at
// package init from a fixed string so the cost stays deterministic
// across binaries.
var backofficeDummyBcryptHash = mustGenerateDummyBcryptHash()

func mustGenerateDummyBcryptHash() []byte {
	// The plaintext "backoffice-dummy-password" is irrelevant — only
	// the cost factor matters. bcrypt at DefaultCost takes a fixed
	// amount of work regardless of input.
	h, err := bcrypt.GenerateFromPassword([]byte("backoffice-dummy-password"), bcrypt.DefaultCost)
	if err != nil {
		// bcrypt.GenerateFromPassword only fails on cost out of range,
		// which DefaultCost never is. Panic at package init so a
		// regression is impossible to miss.
		panic(fmt.Sprintf("failed to generate backoffice dummy bcrypt hash: %v", err))
	}
	return h
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
// cookie AND rotates the refresh token. Mirrors /auth/refresh but is
// strictly scoped to the back-office plane — it never reads
// `refresh_token`, only `backoffice_refresh_token`.
//
// REFRESH-TOKEN ROTATION: on every successful refresh, the consumed
// refresh-token row is revoked and a NEW row is inserted with a fresh
// 30-day TTL; the response's Set-Cookie carries the new value. A stolen
// cookie therefore stays valid only until the legitimate operator next
// refreshes — at which point the attacker's cookie hashes to a revoked
// row and the next replay returns 401. The back-office plane is the
// highest-value identity surface in the system and so diverges from the
// tenant-side non-rotating behaviour deliberately.
// @Summary Refresh back-office access token
// @Description Issue a new back-office access token using the `backoffice_refresh_token` cookie. Rotates the refresh token: the consumed cookie value is revoked and a new value is set.
// @Tags backoffice-auth
// @Produce json
// @Success 200 {object} BackofficeLoginResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Failure 501 {string} string "Refresh tokens not supported (registry not configured)"
// @Router /backoffice/auth/refresh [post]
func (api *BackofficeAuthAPI) refresh(w http.ResponseWriter, r *http.Request) {
	// Guard against a nil registry the same way the tenant /auth/refresh
	// does: a misconfiguration / test wiring with no refresh-token
	// registry must return 501, not nil-deref.
	if api.refreshTokenRegistry == nil {
		http.Error(w, "Refresh tokens not supported", http.StatusNotImplemented)
		return
	}

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

	// Rotate: persist a new refresh-token row BEFORE revoking the old
	// one so a transient registry error leaves the operator's existing
	// session usable (vs. half-rotated and signed out). The new row's
	// id becomes the `rti` claim of the minted access token so the
	// linkage points at the new row from the moment of mint.
	newRTI, newRawRefreshToken, err := api.persistRefreshToken(r.Context(), r, user)
	if err != nil {
		slog.Error("Failed to issue rotated backoffice refresh token", "user_id", user.ID, "error", err)
		http.Error(w, "Failed to rotate session", http.StatusInternalServerError)
		return
	}

	accessToken, _, err := api.issueAccessToken(user, newRTI)
	if err != nil {
		slog.Error("Failed to issue backoffice access token on refresh", "user_id", user.ID, "error", err)
		api.rollbackRefreshToken(r.Context(), user.ID, newRTI)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	// Mark the consumed row as revoked. If revocation fails (transient
	// DB error), the access token is already minted and the new cookie
	// will overwrite the old one in the browser — but log loudly so
	// operators see the partial rotation. The old token cannot be
	// concurrently refreshed (we hold the only legitimate copy of
	// `cookie.Value`); a replay would race the next refresh and lose,
	// at which point IsValid() rejects it on the now-revoked row.
	if err := api.refreshTokenRegistry.Revoke(r.Context(), user.ID, refreshToken.ID); err != nil {
		slog.Error("Failed to revoke consumed backoffice refresh token (rotation partially completed)",
			"old_token_id", refreshToken.ID, "new_token_id", newRTI, "user_id", user.ID, "error", err)
	}

	// Stamp last_used_at on the NEW row so an operator inspecting the
	// active session list sees the most recent usage. Best-effort: a
	// failure here doesn't break the response.
	now := time.Now()
	if err := api.refreshTokenRegistry.BumpLastUsedAt(r.Context(), user.ID, newRTI, now); err != nil {
		slog.Error("Failed to bump backoffice refresh token last_used_at", "token_id", newRTI, "error", err)
	}

	api.setRefreshTokenCookie(w, r, newRawRefreshToken)
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

// parseBackofficeAccessTokenClaims parses (and verifies the signature on)
// a Bearer access token from authHeader and returns the claims map ONLY
// if the token presents as a back-office token: `aud == "backoffice"`
// AND a non-empty `admin_id` claim. Expired tokens are accepted so that
// logout can still extract the admin actor + JTI after the access TTL
// lapses. Returns (nil, false) for any of:
//
//   - missing/malformed Authorization header,
//   - signature failure or non-HMAC alg,
//   - missing/empty `admin_id`,
//   - `aud` != "backoffice" (a tenant JWT replayed at a back-office
//     endpoint must NEVER let us write into the shared jti blacklist
//     or stamp a back-office audit row).
//
// The Bearer scheme match is case-insensitive (RFC 7235 §2.1), aligned
// with the tenant-side parseBearerToken added in PR #1812.
func (api *BackofficeAuthAPI) parseBackofficeAccessTokenClaims(authHeader string) (jwt.MapClaims, bool) {
	scheme, tokenString, hasSpace := strings.Cut(authHeader, " ")
	if !hasSpace || !strings.EqualFold(scheme, "Bearer") {
		return nil, false
	}
	tokenString = strings.TrimSpace(tokenString)
	if tokenString == "" {
		return nil, false
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return api.jwtSecret, nil
	})
	if token == nil {
		return nil, false
	}
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
		return nil, false
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, false
	}

	// Plane guard #1: `aud` MUST be "backoffice". A tenant JWT (which
	// historically carries no `aud` at all) MUST never reach the
	// blacklist or audit-log writers below — otherwise an attacker who
	// holds a tenant token can POST it at /backoffice/auth/logout and
	// forcibly invalidate the victim's tenant session via a different
	// plane, bypassing the cross-plane boundary. Mirrors the guard
	// `validateBackofficeJWT` enforces on authenticated routes.
	if aud, _ := claims["aud"].(string); aud != backofficeTokenAudience {
		return nil, false
	}

	// Plane guard #2: `admin_id` MUST be present. A token with the
	// right `aud` but no admin_id is malformed — refuse to act on it.
	if adminID, _ := claims["admin_id"].(string); adminID == "" {
		return nil, false
	}

	return claims, true
}

// adminIDFromAccessTokenHeader pulls the `admin_id` claim out of a
// (possibly expired) Bearer token, returning ("", false) for any token
// that is not a back-office token (wrong aud, missing admin_id, bad
// signature). Used by logout so an expired-token logout still records
// the admin actor — but ONLY when the token is genuinely ours.
func (api *BackofficeAuthAPI) adminIDFromAccessTokenHeader(authHeader string) (string, bool) {
	claims, ok := api.parseBackofficeAccessTokenClaims(authHeader)
	if !ok {
		return "", false
	}
	adminID, _ := claims["admin_id"].(string)
	return adminID, adminID != ""
}

// blacklistAccessToken extracts the JTI from a Bearer header and
// blacklists it until its `exp`. Capped at 2× backofficeAccessTokenExpiration
// as defence-in-depth against an artificially large exp.
//
// The header MUST present as a back-office token (aud="backoffice",
// non-empty admin_id) — otherwise the call is a no-op. The blacklister
// keyspace is shared with the tenant plane, so writing a tenant JWT's
// jti into it from this handler would let an unauthenticated attacker
// invalidate any tenant session by POSTing a captured tenant JWT at
// /backoffice/auth/logout. The guard collapses that into a silent
// decline so the cross-plane boundary holds.
func (api *BackofficeAuthAPI) blacklistAccessToken(ctx context.Context, authHeader string) {
	if api.blacklistService == nil {
		return
	}
	claims, ok := api.parseBackofficeAccessTokenClaims(authHeader)
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
