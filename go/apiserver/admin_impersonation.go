package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/csrf"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// defaultImpersonationTTL is the fallback impersonation-session lifetime
// used when AdminParams.ImpersonationTTL is unset (zero). 30 min matches
// the #1750 spec default and the hard upper bound enforced by
// clampImpersonationTTL — operators tune it via INVENTARIO_RUN_IMPERSONATION_TTL.
const defaultImpersonationTTL = 30 * time.Minute

// maxImpersonationTTL is the hard ceiling on an impersonation session.
// The #1750 spec mandates "lifetime ≤ 30 min" — a longer configured
// value is clamped down to this rather than rejected so a fat-fingered
// `2h` cannot silently widen the borrowed-identity window.
const maxImpersonationTTL = 30 * time.Minute

// impersonationReasonMaxLen caps the optional reason string. Mirrors
// adminBlockReasonMaxLen so the audit-breadcrumb column stays bounded.
const impersonationReasonMaxLen = 500

// Impersonation audit action names. Kept as constants so the audit
// trail uses the same literals as the swagger tags and the FE filter
// chips. Mirrors the "admin.<verb>" pattern set by #1745/#1747.
const (
	// AuditActionAdminImpersonateStart is the audit-row Action emitted
	// when an admin opens an impersonation session. Failure attempts
	// (target-admin, target-blocked, nested, rate-limited) reuse the
	// same Action with Success=false so one filter pulls the whole
	// attempt history.
	AuditActionAdminImpersonateStart = "admin.impersonate_start"
	// AuditActionAdminImpersonateEnd is the audit-row Action emitted
	// when an impersonation session is ended via POST /impersonation/end.
	AuditActionAdminImpersonateEnd = "admin.impersonate_end"
)

// ImpersonateRequest is the (optional) request body for
// POST /admin/users/{userID}/impersonate. The whole body may be omitted;
// when present, `reason` is the operator's free-form justification and
// is persisted into the audit-log breadcrumb.
type ImpersonateRequest struct {
	// Reason is the optional free-form justification for the
	// impersonation (max 500 chars). The maxLength struct tag surfaces
	// the cap into the generated OpenAPI schema; the handler enforces
	// the same bound at decode time (impersonationReasonMaxLen).
	Reason string `json:"reason,omitempty" maxLength:"500"`
}

// ImpersonationStateResponse is returned by GET /admin/impersonation/current.
// It is a convenience read for the FE's "you are impersonating X" banner.
type ImpersonationStateResponse struct {
	// Active reports whether the caller is inside an impersonation session.
	Active bool `json:"active"`
	// TargetUser is the impersonated user — nil when Active is false.
	TargetUser *ImpersonationUserView `json:"target_user,omitempty"`
	// AdminUser is the operator who initiated the session — nil when
	// Active is false.
	AdminUser *ImpersonationUserView `json:"admin_user,omitempty"`
	// StartedAt / ExpiresAt bound the session — zero values when inactive.
	StartedAt *time.Time `json:"started_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// ImpersonationUserView is the narrow user projection embedded in the
// impersonation responses. Deliberately minimal: identity only, no
// password hash, no group memberships.
type ImpersonationUserView struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
}

// adminImpersonationAPI backs the /admin/users/{id}/impersonate and
// /admin/impersonation/* routes. Holds the FactorySet directly (not the
// per-request user-aware Set) for the same cross-tenant reason the other
// admin APIs do — the impersonation target may live in a different
// tenant than the operator.
type adminImpersonationAPI struct {
	factorySet   *registry.FactorySet
	store        services.ImpersonationStore
	rateLimiter  services.AuthRateLimiter
	blacklist    services.TokenBlacklister
	auditService services.AuditLogger
	// csrfService mints a CSRF token for the new effective user across an
	// identity swap (#1750): admin→target on start, target→admin on end.
	// CSRF validation is per-user, so without a rotated token the SPA's
	// first mutating request under the swapped identity would 403. May be
	// nil (isolated unit tests); the response then carries an empty token.
	csrfService csrf.Service
	jwtSecret   []byte
	ttl         time.Duration
}

// clampImpersonationTTL resolves the effective impersonation TTL:
// zero falls back to the 30-min default, and any value above the 30-min
// ceiling is clamped down so a misconfigured
// INVENTARIO_RUN_IMPERSONATION_TTL cannot widen the borrowed-identity
// window past what the #1750 spec allows.
//
// A negative TTL never reaches here in practice: Params.Validate()
// rejects a negative ImpersonationTTL at startup. The `<= 0` branch
// below still treats a negative value as "fall back to the default" —
// defensive depth for any caller that bypasses Params.Validate().
func clampImpersonationTTL(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return defaultImpersonationTTL
	}
	if ttl > maxImpersonationTTL {
		return maxImpersonationTTL
	}
	return ttl
}

// startImpersonation issues a short-lived impersonation access token for
// the target user and records the server-side return slot needed to
// restore the admin's session.
//
// @Summary Start an impersonation session (admin)
// @Description Issues a short-lived impersonation access token for the target user and sets it as the active session.
// @Description The token carries `imp=true`, `impersonated_by=<adminID>`, and `is_system_admin=false`; it cannot be
// @Description refreshed and cannot start a nested impersonation. The admin's own refresh-token cookie is replaced
// @Description with a non-refreshable impersonation marker for the duration of the session; POST /admin/impersonation/end
// @Description restores the operator's original refresh cookie.
// @Description Returns 422 with `admin.impersonate.target_is_admin` when the target is a system admin,
// @Description `admin.impersonate.target_blocked` when the target account is blocked, and `admin.impersonate.nested`
// @Description when the caller is already impersonating. Returns 429 with `admin.impersonate.rate_limited` when the
// @Description per-admin start rate limit (10/hour) is exceeded.
// @Tags admin
// @Accept json
// @Produce json
// @Param userID path string true "Target user ID"
// @Param data body ImpersonateRequest false "Optional impersonation reason"
// @Success 200 {object} LoginResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown user"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - target is admin / blocked / nested impersonation / reason too long"
// @Failure 429 {object} jsonapi.Errors "Too Many Requests - per-admin rate limit"
// @Router /admin/users/{userID}/impersonate [post]
func (api *adminImpersonationAPI) startImpersonation(w http.ResponseWriter, r *http.Request) {
	admin := appctx.UserFromContext(r.Context())
	if admin == nil {
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}

	// Nested-impersonation guard: an impersonation access token carries
	// `imp=true`. Refuse to open a second session from inside the first
	// so the audit chain stays unambiguous about the operator-of-record.
	if isImpersonatedRequest(r.Context()) {
		api.auditStart(r, admin.ID, "", "", "", false, ErrNestedImpersonation.Error())
		_ = renderEntityError(w, r, ErrNestedImpersonation)
		return
	}

	userID := chi.URLParam(r, "userID")
	if strings.TrimSpace(userID) == "" {
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	req, ok := api.decodeImpersonateRequest(w, r)
	if !ok {
		return
	}

	// Per-admin rate limit (#1750): 10 starts/hour. Checked before the
	// target lookup so a runaway script cannot hammer the user registry.
	if !api.checkRateLimit(w, r, admin.ID) {
		return
	}

	target, err := api.factorySet.UserRegistry.Get(r.Context(), userID)
	if err != nil {
		api.auditStart(r, admin.ID, "", userID, "", false, err.Error())
		_ = renderEntityError(w, r, err)
		return
	}

	if guardErr := impersonationTargetGuard(target); guardErr != nil {
		api.auditStart(r, admin.ID, "", target.ID, target.TenantID, false, guardErr.Error())
		_ = renderEntityError(w, r, guardErr)
		return
	}

	api.issueAndRespond(w, r, admin, target, req.Reason)
}

// issueAndRespond mints the impersonation token, records the return
// slot, and writes the session response. Extracted from
// startImpersonation to keep that handler under the funlen budget.
func (api *adminImpersonationAPI) issueAndRespond(w http.ResponseWriter, r *http.Request, admin, target *models.User, reason string) {
	startedAt := time.Now()
	ttl := clampImpersonationTTL(api.ttl)
	expiresAt := startedAt.Add(ttl)
	jti := uuid.New().String()

	tokenString, err := api.signImpersonationToken(target, admin.ID, jti, startedAt, expiresAt)
	if err != nil {
		slog.Error("Failed to sign impersonation token", "admin_id", admin.ID, "target_id", target.ID, "error", err)
		api.auditStart(r, admin.ID, "", target.ID, target.TenantID, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	// Record the return slot BEFORE writing the response so the `end`
	// endpoint can always resolve it. The slot holds the admin's current
	// refresh-token raw value (read from the cookie) so `end` can restore
	// the admin's session; an admin without a refresh cookie (pure-bearer
	// client) gets a brand-new session on `end` instead.
	slot := services.ImpersonationSlot{
		JTI:                  jti,
		AdminUserID:          admin.ID,
		AdminTenantID:        admin.TenantID,
		AdminRefreshTokenRaw: refreshCookieValue(r),
		TargetUserID:         target.ID,
		TargetTenantID:       target.TenantID,
		Reason:               reason,
		StartedAt:            startedAt,
		ExpiresAt:            expiresAt,
	}
	if err := api.store.Put(r.Context(), slot); err != nil {
		slog.Error("Failed to record impersonation return slot", "admin_id", admin.ID, "target_id", target.ID, "error", err)
		api.auditStart(r, admin.ID, "", target.ID, target.TenantID, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	api.auditStart(r, admin.ID, reason, target.ID, target.TenantID, true, "")
	slog.Info("Impersonation session started", "admin_id", admin.ID, "target_id", target.ID, "jti", jti)

	// Replace the FULL active session, not just the access token. The
	// admin's own refresh cookie must NOT survive into the impersonation
	// session: if it did, the FE refresh interceptor — which posts to
	// /auth/refresh with the cookie but no Authorization header — would
	// silently mint a fresh *admin* access token the moment the
	// (short-lived) impersonation token 401s, ending the session outside
	// /admin/impersonation/end and orphaning this return slot.
	//
	// So overwrite the refresh cookie with the impersonation marker
	// (impersonationRefreshCookieMarker + jti). /auth/refresh detects the
	// marker and rejects the refresh on the cookie path. The admin's
	// genuine refresh token is preserved server-side in slot.AdminRefreshTokenRaw
	// and restored on `end`. The marker cookie's max-age matches the
	// impersonation TTL.
	//
	// An operator who lets the impersonation access token expire (idle)
	// is NOT stranded: POST /admin/impersonation/end self-validates the
	// expired token and still restores the admin from the return slot
	// (the slot outlives the access token up to the same TTL). The
	// genuine admin refresh token also stays recoverable until then —
	// either via that `end` call or, if the operator logs out instead,
	// via the marker-aware logout path. Only an operator who neither ends
	// nor logs out before the TTL elapses falls back to a fresh login,
	// at which point the slot has been TTL-pruned anyway.
	writeRefreshCookie(w, r, impersonationRefreshCookieMarker+jti, int(ttl.Seconds()))

	// Rotate the CSRF token to the TARGET user: CSRF validation is
	// per-user, so the impersonated session must carry a token minted for
	// the target or its first mutating request 403s. Mirrors how login /
	// refresh return a freshly-minted token.
	csrfToken := generateCSRFToken(r.Context(), api.csrfService, target.ID)

	// The impersonation token is set as the active session via the body
	// (same LoginResponse shape the FE already handles).
	writeImpersonationLoginResponse(w, tokenString, csrfToken, ttl, target)
}

// parseImpersonationEndToken extracts and verifies the impersonation
// access token from the request's Authorization header for the `end`
// endpoint. It is the self-validation step that lets `end` run WITHOUT
// JWTMiddleware (see Admin() for why the route is mounted bare):
//
//   - The HS256 signature is ALWAYS verified against the same jwtSecret
//     the rest of the apiserver uses — a forged or garbage token is
//     rejected here exactly as JWTMiddleware would reject it.
//   - ONLY an expired `exp` is tolerated (jwt.ErrTokenExpired). Every
//     other validation error — bad signature, wrong algorithm, malformed
//     token — still fails. This is the single, deliberate relaxation:
//     it lets an operator end a session whose impersonation token lapsed
//     while idle instead of being forced into a full re-login.
//   - The token MUST be an impersonation token (`imp=true` +
//     non-empty `impersonated_by`). An expired NON-impersonation token is
//     therefore NOT admitted — claimsAreImpersonation returns false for it.
//
// The authoritative authorization still happens in endImpersonation: the
// jti-keyed server-side return slot must exist AND its AdminUserID must
// equal the token's `impersonated_by`. The token here only identifies
// WHICH slot; the slot is the proof.
//
// Returns (claims, nil) on success. On failure it returns a non-nil
// error so the caller can pick the right HTTP status:
//   - ErrImpersonationTokenInvalid — the header is absent/malformed, the
//     signature is bad, or the token is not an impersonation token. This
//     is an AUTHENTICATION failure → endImpersonation maps it to 401.
//
// A validly-signed impersonation token whose server-side slot is missing
// is NOT an error here — that case yields (claims, nil) and is handled
// downstream as the 422 "not active" business-rule outcome.
func (api *adminImpersonationAPI) parseImpersonationEndToken(authHeader string) (jwt.MapClaims, error) {
	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == authHeader || tokenString == "" {
		return nil, ErrImpersonationTokenInvalid
	}
	token, err := jwt.Parse(tokenString, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return api.jwtSecret, nil
	})
	if token == nil {
		return nil, ErrImpersonationTokenInvalid
	}
	// Tolerate ONLY expiry — every other error (bad signature, wrong alg,
	// malformed) means the token cannot be trusted and `end` must reject it.
	if err != nil && !errors.Is(err, jwt.ErrTokenExpired) {
		return nil, ErrImpersonationTokenInvalid
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrImpersonationTokenInvalid
	}
	if !claimsAreImpersonation(claims) {
		return nil, ErrImpersonationTokenInvalid
	}
	return claims, nil
}

// endImpersonation revokes the impersonation access token and restores
// the admin's original session.
//
// @Summary End an impersonation session (admin)
// @Description Ends the active impersonation session: blacklists the impersonation access token, restores the operator's
// @Description own refresh-token cookie, and mints a fresh admin access token. Must be called with the impersonation
// @Description access token (the token carrying `imp=true`). The token's signature is always verified; an expired
// @Description impersonation token is still accepted here so an operator can end an idle session without re-logging in.
// @Description The request must also carry the httpOnly `refresh_token` cookie holding the `imp:<jti>` marker that
// @Description impersonation-start planted — proof the call comes from the operator's own browser. A stolen bearer
// @Description token alone, without that cookie, cannot be redeemed for admin credentials.
// @Description This endpoint is mounted WITHOUT the JWT middleware and self-validates the impersonation token off the
// @Description Authorization header. Returns 401 when the token is missing, malformed, forged, or not an impersonation
// @Description token (an authentication failure). Returns 422 with `admin.impersonate.not_active` when the token is a
// @Description validly-signed impersonation token but no active session backs it — the return slot is missing, the
// @Description slot's operator disagrees with the token, or the operator's marker refresh cookie is absent/mismatched.
// @Description A 500 is returned only on a genuine store or registry fault.
// @Tags admin
// @Produce json
// @Success 200 {object} LoginResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - missing, malformed, forged, or non-impersonation token"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - no active impersonation session (missing/mismatched return slot or marker cookie)"
// @Failure 500 {object} jsonapi.Errors "Internal Server Error - return-slot store or registry fault"
// @Router /admin/impersonation/end [post]
func (api *adminImpersonationAPI) endImpersonation(w http.ResponseWriter, r *http.Request) {
	// `end` is mounted WITHOUT JWTMiddleware (see Admin()), so the token is
	// validated here: the signature is verified and `imp=true` is required;
	// only an expired `exp` is tolerated. A missing/malformed/forged/
	// non-impersonation token is an AUTHENTICATION failure → 401, distinct
	// from the 422 reserved for "validly-signed token, no active session".
	claims, perr := api.parseImpersonationEndToken(r.Header.Get("Authorization"))
	if perr != nil {
		_ = unauthorizedError(w, r, perr)
		return
	}

	// `end` runs without JWTMiddleware, so the validated claims are not in
	// context. Plant them so auditEnd's LogAdmin can auto-fill the audit
	// row's ImpersonatedBy column from `imp`/`impersonated_by` exactly as
	// it does on every JWTMiddleware-backed route — the audit semantics
	// stay single-sourced rather than forking a special end-only path.
	r = r.WithContext(appctx.WithJWTClaims(r.Context(), claims))

	jti, _ := claims["jti"].(string)
	adminID, _ := claims["impersonated_by"].(string)
	targetID, _ := claims["user_id"].(string)

	// A genuine impersonation token always carries a UUID jti (see
	// signImpersonationToken). An empty jti would make the marker-cookie
	// comparison below pass vacuously against an absent cookie, so reject
	// it up front as malformed — an authentication failure (401).
	if jti == "" {
		slog.Warn("Impersonation end: token missing jti claim", "admin_id", adminID)
		_ = unauthorizedError(w, r, ErrImpersonationTokenInvalid)
		return
	}

	// SECURITY (#1750 / PR #1771 review): a valid impersonation bearer
	// token is NOT sufficient to redeem `end` for admin credentials. The
	// operator's browser holds the httpOnly `refresh_token` cookie that
	// impersonation-start planted as `imp:<jti>`; require it here and
	// require its jti to match the token's. A leaked bearer token without
	// that browser-bound cookie then cannot be exchanged for the admin
	// session. The marker cookie's max-age == the impersonation TTL, so it
	// is still present on the only-just-expired-token `end` path.
	if cookieJTI := markerCookieJTI(r); cookieJTI != jti {
		slog.Warn("Impersonation end: marker refresh cookie absent or mismatched",
			"jti", jti, "cookie_jti", cookieJTI, "admin_id", adminID)
		// Audit the rejected attempt. The return slot is not yet loaded, so
		// only the claim-derived fields are available — enough for the
		// audit trail to record "an end was attempted and refused".
		api.auditEnd(r, services.ImpersonationSlot{}, targetID, false,
			"marker refresh cookie absent or mismatched")
		_ = renderEntityError(w, r, ErrNotImpersonating)
		return
	}

	slot, err := api.store.Get(r.Context(), jti)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// The token is a valid impersonation token but the slot is
			// gone (already ended, or the process restarted). Treat as
			// "not active" — the FE banner clears and the operator
			// re-logs in.
			slog.Warn("Impersonation end: return slot not found", "jti", jti, "admin_id", adminID)
			_ = renderEntityError(w, r, ErrNotImpersonating)
			return
		}
		// A non-not-found error is a genuine store fault (e.g. a
		// Redis-backed store outage), not a missing slot — surface it as
		// a 500 rather than masking it as "not active".
		slog.Error("Impersonation end: failed to read return slot", "jti", jti, "admin_id", adminID, "error", err)
		_ = internalServerError(w, r, err)
		return
	}

	// Defence-in-depth: the slot is jti-keyed and server-mutable, while
	// `impersonated_by` comes from the signed (authoritative) token. If
	// the two disagree, the slot belongs to a different operator — refuse
	// rather than restore the wrong admin's session.
	if slot.AdminUserID != adminID {
		slog.Error("Impersonation end: slot/token admin mismatch",
			"jti", jti, "slot_admin", slot.AdminUserID, "token_admin", adminID)
		_ = renderEntityError(w, r, ErrNotImpersonating)
		return
	}

	// Blacklist the impersonation token so it cannot be reused after the
	// session is ended, and drop the return slot.
	//
	// NOTE: impersonation-token revocation is durable only with a
	// Redis-backed token blacklist. With the default in-memory blacklist a
	// process restart loses this entry, so an already-ended impersonation
	// token is accepted again by JWTMiddleware until its (≤30-min TTL)
	// ceiling expires. Operators running a single in-memory instance
	// accept that ≤30-min window; multi-instance / production deployments
	// should configure INVENTARIO_RUN_TOKEN_BLACKLIST_REDIS_URL.
	api.blacklistImpersonationToken(r.Context(), jti, slot.ExpiresAt)
	if delErr := api.store.Delete(r.Context(), jti); delErr != nil {
		slog.Warn("Impersonation end: failed to delete return slot", "jti", jti, "error", delErr)
	}

	admin, err := api.factorySet.UserRegistry.Get(r.Context(), slot.AdminUserID)
	if err != nil {
		slog.Error("Impersonation end: failed to reload admin user", "admin_id", slot.AdminUserID, "error", err)
		api.auditEnd(r, slot, targetID, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	api.auditEnd(r, slot, targetID, true, "")
	slog.Info("Impersonation session ended", "admin_id", admin.ID, "target_id", slot.TargetUserID, "jti", jti)

	api.restoreAdminSession(w, r, admin, slot)
}

// currentImpersonation reports the active impersonation session for the
// FE banner.
//
// @Summary Read the active impersonation session (admin)
// @Description Convenience read for the FE impersonation banner. Returns `active=false` with no other fields when the caller is not inside an impersonation session, and the target/admin/started_at/expires_at quartet when it is.
// @Tags admin
// @Produce json
// @Success 200 {object} ImpersonationStateResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin or active impersonation session required"
// @Router /admin/impersonation/current [get]
func (api *adminImpersonationAPI) currentImpersonation(w http.ResponseWriter, r *http.Request) {
	claims := appctx.JWTClaimsFromContext(r.Context())
	if !claimsAreImpersonation(claims) {
		writeJSON(w, http.StatusOK, ImpersonationStateResponse{Active: false})
		return
	}

	jti, _ := claims["jti"].(string)
	slot, err := api.store.Get(r.Context(), jti)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// Token says impersonation, but the slot is gone — report
			// inactive so the FE clears its banner.
			writeJSON(w, http.StatusOK, ImpersonationStateResponse{Active: false})
			return
		}
		// A non-not-found error is a genuine store fault, not a missing
		// slot — surface it as a 500 rather than reporting a misleading
		// inactive banner that masks a backend outage.
		slog.Error("Impersonation current: failed to read return slot", "jti", jti, "error", err)
		_ = internalServerError(w, r, err)
		return
	}

	// A target (or admin) that cannot be resolved means the session is
	// effectively broken — a vanished target is not a meaningful "active"
	// impersonation. Report inactive so the FE clears its banner rather
	// than rendering active=true with a nil user.
	target, terr := api.factorySet.UserRegistry.Get(r.Context(), slot.TargetUserID)
	if terr != nil {
		slog.Warn("Impersonation current: failed to resolve target user, reporting inactive",
			"jti", jti, "target_id", slot.TargetUserID, "error", terr)
		writeJSON(w, http.StatusOK, ImpersonationStateResponse{Active: false})
		return
	}
	admin, aerr := api.factorySet.UserRegistry.Get(r.Context(), slot.AdminUserID)
	if aerr != nil {
		slog.Warn("Impersonation current: failed to resolve admin user, reporting inactive",
			"jti", jti, "admin_id", slot.AdminUserID, "error", aerr)
		writeJSON(w, http.StatusOK, ImpersonationStateResponse{Active: false})
		return
	}

	writeJSON(w, http.StatusOK, ImpersonationStateResponse{
		Active:     true,
		StartedAt:  &slot.StartedAt,
		ExpiresAt:  &slot.ExpiresAt,
		TargetUser: impersonationUserView(target),
		AdminUser:  impersonationUserView(admin),
	})
}

// restoreAdminSession re-issues the operator's own session after an
// impersonation session ends: it mints a fresh admin access token bound
// to the admin's existing refresh-token row (when the return slot
// captured one) and restores the admin's original refresh cookie, which
// the impersonation-start handler had overwritten with the impersonation
// marker. Extracted from endImpersonation to stay under the funlen
// budget.
func (api *adminImpersonationAPI) restoreAdminSession(w http.ResponseWriter, r *http.Request, admin *models.User, slot services.ImpersonationSlot) {
	// No IsActive re-check on the admin here on purpose: JWTMiddleware's
	// validateUser rejects a blocked (!IsActive) user on the very next
	// request, so minting the token for a since-blocked admin is not an
	// escalation — the middleware is the real gate.
	//
	// Resolve the rti claim from the admin's captured refresh token so
	// /users/me/sessions can still flag the operator's own session. An
	// empty rti is fine — issueAdminAccessToken omits the claim.
	rti := api.resolveAdminRefreshTokenID(r.Context(), slot.AdminRefreshTokenRaw)

	tokenString, err := api.signAdminAccessToken(admin, rti)
	if err != nil {
		slog.Error("Impersonation end: failed to mint admin access token", "admin_id", admin.ID, "error", err)
		_ = internalServerError(w, r, err)
		return
	}

	// Restore the admin's full session. The impersonation-start handler
	// overwrote the refresh cookie with the impersonation marker, so it
	// must be put back here or the admin would be left unable to refresh.
	//
	//  - The admin had a refresh token at start-time: re-plant it (still
	//    valid in the DB — impersonation never revoked it) so the operator
	//    can transparently refresh again afterwards.
	//  - The admin had no refresh cookie (a pure-bearer client, e.g. a
	//    test or API caller): there is nothing to restore, so delete the
	//    marker cookie. The operator continues on the freshly-minted
	//    access token until it expires, then logs in again — the same
	//    behaviour a pure-bearer client had before impersonation.
	if slot.AdminRefreshTokenRaw != "" {
		writeRefreshCookie(w, r, slot.AdminRefreshTokenRaw, int(refreshTokenExpiration.Seconds()))
	} else {
		clearRefreshCookie(w, r)
	}

	// Rotate the CSRF token back to the ADMIN user: the identity swaps
	// from target back to operator, and CSRF validation is per-user, so
	// the restored admin session must carry a token minted for the admin
	// or its first mutating request 403s. Mirrors login / refresh.
	csrfToken := generateCSRFToken(r.Context(), api.csrfService, admin.ID)

	// The FE swaps the in-memory access token back to the admin's and the
	// operator is whole again.
	writeImpersonationLoginResponse(w, tokenString, csrfToken, accessTokenExpiration, admin)
}

// resolveAdminRefreshTokenID looks up the row id of the admin's captured
// refresh token so the restored access token can carry it as `rti`.
// Returns "" when no token was captured, the token is unknown, or the
// lookup fails — the `rti` claim is informational and a missing value
// only means /users/me/sessions cannot self-flag the row.
func (api *adminImpersonationAPI) resolveAdminRefreshTokenID(ctx context.Context, rawToken string) string {
	if rawToken == "" || api.factorySet.RefreshTokenRegistry == nil {
		return ""
	}
	rt, err := api.factorySet.RefreshTokenRegistry.GetByTokenHash(ctx, models.HashRefreshToken(rawToken))
	if err != nil {
		return ""
	}
	return rt.ID
}

// signImpersonationToken builds and signs the impersonation access
// token. Claims: user_id=<target> so JWTMiddleware loads the target;
// tenant_id=<target tenant>; impersonated_by=<admin>; imp=true;
// is_system_admin=false (an impersonated session never carries platform
// admin authority, even when the target somehow had the flag — the
// target guard rejects admins, but the claim is pinned false as
// defence-in-depth).
func (api *adminImpersonationAPI) signImpersonationToken(target *models.User, adminID, jti string, issuedAt, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"jti":             jti,
		"user_id":         target.ID,
		"tenant_id":       target.TenantID,
		"impersonated_by": adminID,
		"imp":             true,
		"is_system_admin": false,
		"iat":             issuedAt.Unix(),
		"exp":             expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(api.jwtSecret)
}

// signAdminAccessToken mints a normal (non-impersonation) access token
// for the operator when an impersonation session ends. Mirrors
// AuthAPI.issueAccessToken so the restored session is indistinguishable
// from a fresh login/refresh — including the is_system_admin claim.
func (api *adminImpersonationAPI) signAdminAccessToken(admin *models.User, rti string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"jti":             uuid.New().String(),
		"user_id":         admin.ID,
		"is_system_admin": admin.IsSystemAdmin,
		"iat":             now.Unix(),
		"exp":             now.Add(accessTokenExpiration).Unix(),
	}
	if rti != "" {
		claims["rti"] = rti
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(api.jwtSecret)
}

// blacklistImpersonationToken adds the impersonation token's JTI to the
// blacklist so it is rejected on any further use after `end`. Best-effort:
// a blacklist outage is logged but does not block the end flow — the
// token expires on its own at the (≤ 30 min) TTL regardless. The
// blacklister is the same service the rest of the apiserver uses,
// plumbed through AdminParams.Blacklist.
func (api *adminImpersonationAPI) blacklistImpersonationToken(ctx context.Context, jti string, expiresAt time.Time) {
	if api.blacklist == nil || jti == "" {
		return
	}
	if err := api.blacklist.BlacklistToken(ctx, jti, expiresAt); err != nil {
		slog.Warn("Impersonation end: failed to blacklist token", "jti", jti, "error", err)
	}
}

// decodeImpersonateRequest parses the optional POST body. An empty body
// is valid (the whole request body is optional) and yields a zero-value
// request. A present-but-malformed body is a 400; an over-long reason is
// a 422 with code admin.impersonate.reason_too_long — matching the admin
// block handler's reason-length contract exactly.
func (api *adminImpersonationAPI) decodeImpersonateRequest(w http.ResponseWriter, r *http.Request) (ImpersonateRequest, bool) {
	var req ImpersonateRequest
	if r.Body == nil {
		return req, true
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		if errors.Is(err, io.EOF) {
			// Empty body — the request body is optional.
			return ImpersonateRequest{}, true
		}
		_ = badRequest(w, r, err)
		return req, false
	}
	if !decoderAtEOF(dec) {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return req, false
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if utf8.RuneCountInString(req.Reason) > impersonationReasonMaxLen {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is too long"), AdminImpersonateReasonTooLongCode)
		return req, false
	}
	return req, true
}

// checkRateLimit enforces the per-admin impersonation-start rate limit.
// Returns true when the request may proceed. On a limiter backend error
// it fails open (consistent with the other rate limiters in this
// package). On a genuine limit hit it writes the 429 + code response and
// returns false.
func (api *adminImpersonationAPI) checkRateLimit(w http.ResponseWriter, r *http.Request, adminID string) bool {
	if api.rateLimiter == nil {
		return true
	}
	res, err := api.rateLimiter.CheckImpersonationAttempt(r.Context(), adminID)
	if err != nil {
		slog.Error("Impersonation rate limiter error", "admin_id", adminID, "error", err)
		return true
	}
	if !res.Allowed {
		retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		api.auditStart(r, adminID, "", "", "", false, "rate limit exceeded")
		_ = codedTooManyRequestsError(w, r, services.ErrRateLimitExceeded, AdminImpersonateRateLimitedCode,
			map[string]any{"retry_after_seconds": retryAfter})
		return false
	}
	return true
}

// auditStart writes the admin.impersonate_start audit row. The
// impersonator column is NOT set from context here (the start request
// runs as the admin, not an impersonated session); LogAdmin still reads
// the breadcrumb reason. Best-effort like the other admin audit helpers.
func (api *adminImpersonationAPI) auditStart(r *http.Request, adminID, reason, targetID, targetTenantID string, success bool, errMsg string) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      AuditActionAdminImpersonateStart,
		ActorID:     nullableString(adminID),
		TenantID:    nullableString(targetTenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(targetID),
		Success:     success,
		Request:     r,
		Reason:      reason,
	}
	if errMsg != "" {
		ev.ErrMsg = stringPtr(errMsg)
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// auditEnd writes the admin.impersonate_end audit row. The request runs
// under the impersonation token, so the actor-of-record is the
// impersonated target (ActorID → UserID = targetID) and LogAdmin
// auto-fills the ImpersonatedBy column from the `imp`/`impersonated_by`
// claims — the row records "admin acting as target" without the call
// site doing anything special.
func (api *adminImpersonationAPI) auditEnd(r *http.Request, slot services.ImpersonationSlot, targetID string, success bool, errMsg string) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      AuditActionAdminImpersonateEnd,
		ActorID:     nullableString(targetID),
		TenantID:    nullableString(slot.TargetTenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(targetID),
		Success:     success,
		Request:     r,
		Reason:      slot.Reason,
	}
	if errMsg != "" {
		ev.ErrMsg = stringPtr(errMsg)
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// impersonationTargetGuard rejects targets that may not be impersonated:
// a system admin (privilege-escalation footgun) or a blocked account
// (resurrecting a torn-down session). Returns nil when the target is a
// valid impersonation subject.
func impersonationTargetGuard(target *models.User) error {
	if target.IsSystemAdmin {
		return ErrCannotImpersonateAdmin
	}
	if !target.IsActive {
		return ErrTargetBlocked
	}
	return nil
}

// isImpersonatedRequest reports whether the current request is already
// running inside an impersonation session (the access token carries
// `imp=true`). Drives the nested-impersonation guard.
func isImpersonatedRequest(ctx context.Context) bool {
	return claimsAreImpersonation(appctx.JWTClaimsFromContext(ctx))
}

// claimsAreImpersonation reports whether the given JWT claims belong to
// an impersonation token: `imp` is true AND `impersonated_by` is a
// non-empty string. Both conditions are required so a token with a
// stray `imp` claim but no operator-of-record is not mistaken for a
// genuine impersonation session.
func claimsAreImpersonation(claims jwt.MapClaims) bool {
	if claims == nil {
		return false
	}
	imp, _ := claims["imp"].(bool)
	if !imp {
		return false
	}
	by, ok := claims["impersonated_by"].(string)
	return ok && by != ""
}

// refreshCookieValue returns the raw refresh-token cookie value to capture
// into the impersonation return slot, or "" when there is nothing genuine
// to capture — the request carries no refresh cookie (a pure-bearer
// client), or the cookie already holds an impersonation marker rather
// than a real token. The marker case should be unreachable (the
// nested-impersonation guard and RequireSystemAdmin both block starting
// an impersonation from inside one), but treating a marker as "no token"
// is defence-in-depth: it stops a marker value from being mistaken for
// the admin's real refresh token and re-planted on `end`.
func refreshCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(refreshTokenCookieName)
	if err != nil {
		return ""
	}
	if strings.HasPrefix(cookie.Value, impersonationRefreshCookieMarker) {
		return ""
	}
	return cookie.Value
}

// markerCookieJTI returns the jti carried by the request's impersonation
// marker refresh cookie, or "" when the request has no `refresh_token`
// cookie or its value is not an `imp:<jti>` marker.
//
// It is the browser-bound proof endImpersonation requires (#1750 / PR
// #1771 review): impersonation-start planted `imp:<jti>` into the
// operator's httpOnly refresh cookie, so a genuine `end` call from that
// same browser carries it. A stolen impersonation bearer token presented
// from anywhere else has no such cookie, so markerCookieJTI returns ""
// and the jti comparison in endImpersonation fails — the stolen token
// cannot be exchanged for admin credentials. The marker cookie's max-age
// equals the impersonation TTL, so the cookie is still present on the
// legitimate only-just-expired-token `end` path.
func markerCookieJTI(r *http.Request) string {
	cookie, err := r.Cookie(refreshTokenCookieName)
	if err != nil {
		return ""
	}
	if !strings.HasPrefix(cookie.Value, impersonationRefreshCookieMarker) {
		return ""
	}
	return strings.TrimPrefix(cookie.Value, impersonationRefreshCookieMarker)
}

// impersonationUserView projects a *models.User into the narrow
// identity-only view embedded in the impersonation responses.
func impersonationUserView(u *models.User) *ImpersonationUserView {
	return &ImpersonationUserView{
		ID:       u.ID,
		Email:    u.Email,
		Name:     u.Name,
		TenantID: u.TenantID,
	}
}

// writeImpersonationLoginResponse writes a LoginResponse with the given
// access token, TTL, and CSRF token. Reuses the LoginResponse shape so
// the FE handles the impersonation-start, impersonation-end, and
// normal-login responses with one code path.
//
// csrfToken MUST be a token freshly minted for the response's effective
// user (the target on start, the admin on end): CSRF validation is
// per-user, so after the identity swap the SPA cannot reuse the previous
// identity's token — its first mutating request would 403. The handlers
// also mirror the token into the X-CSRF-Token response header, matching
// how /auth/me exposes it, so an FE that reads either source recovers.
func writeImpersonationLoginResponse(w http.ResponseWriter, accessToken, csrfToken string, ttl time.Duration, user *models.User) {
	if csrfToken != "" {
		w.Header().Set(csrfHeaderName, csrfToken)
	}
	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(ttl.Seconds()),
		CSRFToken:   csrfToken,
		User:        user,
	})
}
