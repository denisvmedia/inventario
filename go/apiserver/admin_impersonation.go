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
	errxtrace "github.com/go-extras/errx/stacktrace"
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

// impersonatorTypeBackofficeUser is the wire value of the
// `impersonator_type` JWT claim stamped on an impersonation access
// token whose operator-of-record is a back-office user (#1785 Phase 5).
// Mirrors services.ImpersonationOperatorBackoffice — kept duplicated
// at the apiserver boundary so the wire-side string is grep-friendly
// without a services-package round-trip.
const impersonatorTypeBackofficeUser = "backoffice_user"

// Impersonation audit action names. Kept as constants so the audit
// trail uses the same literals as the swagger tags and the FE filter
// chips. Mirrors the "admin.<verb>" pattern set by #1745/#1747.
const (
	// AuditActionAdminImpersonateStart is the audit-row Action emitted
	// when a back-office operator opens an impersonation session.
	// Failure attempts (target-admin, target-blocked, nested,
	// rate-limited) reuse the same Action with Success=false so one
	// filter pulls the whole attempt history.
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
	// TargetUser is the impersonated tenant user — nil when Active is false.
	TargetUser *ImpersonationUserView `json:"target_user,omitempty"`
	// Operator is the back-office operator who initiated the session —
	// nil when Active is false. Renamed from `admin_user` at #1785
	// Phase 5 to reflect that the operator-of-record now lives in the
	// back-office identity plane.
	Operator *ImpersonationOperatorView `json:"operator,omitempty"`
	// StartedAt / ExpiresAt bound the session — zero values when inactive.
	StartedAt *time.Time `json:"started_at,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

// ImpersonationUserView is the narrow tenant-user projection embedded
// in the impersonation responses. Deliberately minimal: identity only,
// no password hash, no group memberships.
type ImpersonationUserView struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	TenantID string `json:"tenant_id"`
}

// ImpersonationOperatorView is the narrow back-office-operator
// projection embedded in the impersonation-state response. Distinct
// from ImpersonationUserView so a FE consumer cannot accidentally
// expect a `tenant_id` field on the operator — back-office identities
// are tenant-agnostic (#1785 Phase 5).
type ImpersonationOperatorView struct {
	ID    string `json:"id"`
	Email string `json:"email"`
	Name  string `json:"name"`
	Role  string `json:"role"`
}

// adminImpersonationAPI backs the /admin/users/{id}/impersonate and
// /admin/impersonation/* routes. Holds the FactorySet directly (not the
// per-request user-aware Set) for the same cross-tenant reason the other
// admin APIs do — the impersonation target may live in a different
// tenant than the operator.
//
// Phase 5 of issue #1785 cuts the start path over to the back-office
// auth plane: the operator-of-record is a *models.BackofficeUser, the
// captured refresh cookie is the operator's `backoffice_refresh_token`,
// and `end` mints a back-office access token rather than a tenant one.
type adminImpersonationAPI struct {
	factorySet   *registry.FactorySet
	store        services.ImpersonationStore
	rateLimiter  services.AuthRateLimiter
	blacklist    services.TokenBlacklister
	auditService services.AuditLogger
	// csrfService mints a CSRF token for the impersonated tenant user
	// across the identity swap at start (#1750): the impersonated
	// session's SPA still talks to /api/v1/g/... tenant endpoints
	// which enforce CSRF, so without a rotated token the SPA's first
	// mutating request under the target identity would 403. On `end`
	// no CSRF token is minted — the restored session is a back-office
	// session, and the back-office plane does not currently CSRF-protect
	// its surface. May be nil (isolated unit tests); the start response
	// then carries an empty token.
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
// the target tenant user on behalf of the calling back-office operator
// and records the server-side return slot needed to restore the
// operator's back-office session.
//
// @Summary Start an impersonation session (back-office operator)
// @Description Issues a short-lived impersonation access token for the target tenant user on behalf of the calling back-office operator.
// @Description The token carries `imp=true`, `impersonator_id=<backoffice user id>`, `impersonator_type=backoffice_user`, and
// @Description `is_system_admin=false`; it cannot be refreshed and cannot start a nested impersonation. The operator's own
// @Description `backoffice_refresh_token` cookie is captured server-side at start and restored by POST /admin/impersonation/end.
// @Description Returns 422 with `admin.impersonate.target_is_admin` when the target is a system admin,
// @Description `admin.impersonate.target_blocked` when the target account is blocked, and `admin.impersonate.nested`
// @Description when the caller is already impersonating. Returns 429 with `admin.impersonate.rate_limited` when the
// @Description per-operator start rate limit (10/hour) is exceeded.
// @Tags admin
// @Accept json
// @Produce json
// @Param userID path string true "Target tenant user ID"
// @Param data body ImpersonateRequest false "Optional impersonation reason"
// @Success 200 {object} LoginResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - platform_admin role required"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown tenant user"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - target is admin / blocked / nested impersonation / reason too long"
// @Failure 429 {object} jsonapi.Errors "Too Many Requests - per-operator rate limit"
// @Router /admin/users/{userID}/impersonate [post]
func (api *adminImpersonationAPI) startImpersonation(w http.ResponseWriter, r *http.Request) {
	operator := appctx.BackofficeUserFromContext(r.Context())
	if operator == nil {
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}

	// Nested-impersonation guard: an impersonation access token carries
	// `imp=true`. The RequireBackofficeAuth gate already rejects tenant
	// (and impersonation) tokens, so reaching here with an active
	// impersonation context would mean the gate was bypassed — but the
	// guard is kept for defence-in-depth and audit consistency.
	if isImpersonatedRequest(r.Context()) {
		api.auditStart(r, operator.ID, "", "", "", false, ErrNestedImpersonation.Error())
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

	// Per-operator rate limit (#1750): 10 starts/hour. Checked before
	// the target lookup so a runaway script cannot hammer the user
	// registry.
	if !api.checkRateLimit(w, r, operator.ID) {
		return
	}

	target, err := api.factorySet.UserRegistry.Get(r.Context(), userID)
	if err != nil {
		api.auditStart(r, operator.ID, "", userID, "", false, err.Error())
		_ = renderEntityError(w, r, err)
		return
	}

	if guardErr := impersonationTargetGuard(r.Context(), api.factorySet.SystemAdminGrantRegistry, target); guardErr != nil {
		api.auditStart(r, operator.ID, "", target.ID, target.TenantID, false, guardErr.Error())
		_ = renderEntityError(w, r, guardErr)
		return
	}

	api.issueAndRespond(w, r, operator, target, req.Reason)
}

// issueAndRespond mints the impersonation token, records the return
// slot, and writes the session response. Extracted from
// startImpersonation to keep that handler under the funlen budget.
func (api *adminImpersonationAPI) issueAndRespond(w http.ResponseWriter, r *http.Request, operator *models.BackofficeUser, target *models.User, reason string) {
	startedAt := time.Now()
	ttl := clampImpersonationTTL(api.ttl)
	expiresAt := startedAt.Add(ttl)
	jti := uuid.New().String()

	tokenString, err := api.signImpersonationToken(target, operator.ID, jti, startedAt, expiresAt)
	if err != nil {
		slog.Error("Failed to sign impersonation token", "operator_id", operator.ID, "target_id", target.ID, "error", err)
		api.auditStart(r, operator.ID, "", target.ID, target.TenantID, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	// Record the return slot BEFORE writing the response so the `end`
	// endpoint can always resolve it. The slot captures the operator's
	// current `backoffice_refresh_token` cookie raw value (when present)
	// so `end` can re-plant the cookie on the operator's browser; an
	// operator who authenticated with bearer-only (no refresh cookie)
	// gets a fresh access token on `end` without a refresh cookie —
	// matching the pure-bearer caller's pre-impersonation shape.
	slot := services.ImpersonationSlot{
		JTI:                     jti,
		OperatorKind:            services.ImpersonationOperatorBackoffice,
		OperatorUserID:          operator.ID,
		OperatorRefreshTokenRaw: backofficeRefreshCookieValue(r),
		TargetUserID:            target.ID,
		TargetTenantID:          target.TenantID,
		Reason:                  reason,
		StartedAt:               startedAt,
		ExpiresAt:               expiresAt,
	}
	if err := api.store.Put(r.Context(), slot); err != nil {
		slog.Error("Failed to record impersonation return slot", "operator_id", operator.ID, "target_id", target.ID, "error", err)
		api.auditStart(r, operator.ID, "", target.ID, target.TenantID, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	api.auditStart(r, operator.ID, reason, target.ID, target.TenantID, true, "")
	slog.Info("Impersonation session started", "operator_id", operator.ID, "target_id", target.ID, "jti", jti)

	// Cross-plane impersonation (#1785 Phase 5) does NOT touch the
	// tenant `refresh_token` cookie at start: the operator was never
	// authenticated on the tenant plane, so there is nothing to mask
	// or replace there. The browser-bound proof that previously rode
	// in an `imp:<jti>` marker cookie is gone — its job is taken over
	// by the JTI-keyed server-side slot, whose OperatorUserID must
	// match the impersonation token's `impersonator_id` claim on `end`.
	// The token's signature gives integrity; the slot gives binding.

	// Rotate the CSRF token to the TARGET tenant user: the impersonated
	// session continues to hit /api/v1/g/... tenant endpoints whose
	// CSRF validation is per-user, so the response must hand the SPA a
	// token minted for the target or its first mutating request 403s.
	// Mirrors how login / refresh return a freshly-minted token.
	csrfToken := generateCSRFToken(r.Context(), api.csrfService, target.ID)

	// Stamp the wire-only is_system_admin advisory flag (#1784). Computed
	// from the grants table just like everywhere else — the target guard
	// already refuses to impersonate a system admin, so this will always
	// be false in practice, but the rule "this field is set from grants,
	// nowhere else" stays single-sourced.
	populateUserSystemAdminFlag(r.Context(), api.factorySet.SystemAdminGrantRegistry, target)

	// The impersonation token is set as the active tenant session via
	// the body (the FE swaps it into its in-memory access-token slot
	// and continues against the tenant endpoints under the target's
	// identity). LoginResponse keeps the existing wire shape so the FE
	// can reuse one code path for login / refresh / impersonation start.
	writeImpersonationLoginResponse(w, tokenString, csrfToken, ttl, target)
}

// parseImpersonationEndToken extracts and verifies the impersonation
// access token from the request's Authorization header for the `end`
// endpoint. It is the self-validation step that lets `end` run WITHOUT
// the standard middleware chain (see Admin() for why the route is
// mounted bare):
//
//   - The HS256 signature is ALWAYS verified against the same jwtSecret
//     the rest of the apiserver uses — a forged or garbage token is
//     rejected here exactly as JWTMiddleware would reject it.
//   - ONLY an expired `exp` is tolerated (jwt.ErrTokenExpired). Every
//     other validation error — bad signature, wrong algorithm, malformed
//     token — still fails. This is the single, deliberate relaxation:
//     it lets an operator end a session whose impersonation token lapsed
//     while idle instead of being forced into a full re-login.
//   - The token MUST be an impersonation token (`imp=true` + a non-empty
//     `impersonator_id`). An expired NON-impersonation token is therefore
//     NOT admitted — claimsAreImpersonation returns false for it.
//
// The authoritative authorization still happens in endImpersonation:
// the jti-keyed server-side return slot must exist AND its
// OperatorUserID must equal the token's `impersonator_id` AND the
// slot's OperatorKind must be ImpersonationOperatorBackoffice. The
// token here only identifies WHICH slot; the slot is the proof.
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
	tokenString, ok := parseBearerToken(authHeader)
	if !ok {
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
// the operator's back-office session.
//
// @Summary End an impersonation session (back-office operator)
// @Description Ends the active impersonation session: blacklists the impersonation access token, mints a fresh back-office
// @Description access token, and re-plants the operator's `backoffice_refresh_token` cookie (when one was captured at
// @Description start). Must be called with the impersonation access token (the token carrying `imp=true`). The token's
// @Description signature is always verified; an expired impersonation token is still accepted so an operator can end an
// @Description idle session without re-logging in.
// @Description This endpoint is mounted WITHOUT the JWT middleware and self-validates the impersonation token off the
// @Description Authorization header. Returns 401 when the token is missing, malformed, forged, or not an impersonation
// @Description token (an authentication failure). Returns 422 with `admin.impersonate.not_active` when the token is a
// @Description validly-signed impersonation token but no active session backs it — the return slot is missing or its
// @Description operator-of-record disagrees with the token. A 500 is returned only on a genuine store or registry fault.
// @Tags admin
// @Produce json
// @Success 200 {object} BackofficeLoginResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized - missing, malformed, forged, or non-impersonation token"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - no active impersonation session (missing/mismatched return slot)"
// @Failure 500 {object} jsonapi.Errors "Internal Server Error - return-slot store or registry fault"
// @Router /admin/impersonation/end [post]
func (api *adminImpersonationAPI) endImpersonation(w http.ResponseWriter, r *http.Request) {
	// `end` is mounted WITHOUT any auth middleware (see Admin()), so the
	// token is validated here: the signature is verified and `imp=true`
	// is required; only an expired `exp` is tolerated. A missing /
	// malformed / forged / non-impersonation token is an AUTHENTICATION
	// failure → 401, distinct from the 422 reserved for "validly-signed
	// token, no active session".
	claims, perr := api.parseImpersonationEndToken(r.Header.Get("Authorization"))
	if perr != nil {
		_ = unauthorizedError(w, r, perr)
		return
	}

	// `end` runs without the standard middleware chain, so the validated
	// claims are not in context. Plant them so auditEnd's LogAdmin can
	// auto-fill the audit row's ImpersonatedBy column from the
	// `imp`/`impersonator_id` claims exactly as it does on every
	// JWTMiddleware-backed route — the audit semantics stay single-sourced
	// rather than forking a special end-only path.
	r = r.WithContext(appctx.WithJWTClaims(r.Context(), claims))

	jti, _ := claims["jti"].(string)
	operatorID, _ := claims["impersonator_id"].(string)
	targetID, _ := claims["user_id"].(string)

	// A genuine impersonation token always carries a UUID jti (see
	// signImpersonationToken). An empty jti would make the slot lookup
	// fail vacuously, so reject it up front as malformed — an
	// authentication failure (401).
	if jti == "" {
		slog.Warn("Impersonation end: token missing jti claim", "operator_id", operatorID)
		_ = unauthorizedError(w, r, ErrImpersonationTokenInvalid)
		return
	}

	slot, err := api.store.Get(r.Context(), jti)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			// The token is a valid impersonation token but the slot is
			// gone (already ended, or the process restarted). Treat as
			// "not active" — the FE banner clears and the operator
			// re-logs in.
			slog.Warn("Impersonation end: return slot not found", "jti", jti, "operator_id", operatorID)
			_ = renderEntityError(w, r, ErrNotImpersonating)
			return
		}
		// A non-not-found error is a genuine store fault (e.g. a
		// Redis-backed store outage), not a missing slot — surface it
		// as a 500 rather than masking it as "not active".
		slog.Error("Impersonation end: failed to read return slot", "jti", jti, "operator_id", operatorID, "error", err)
		_ = internalServerError(w, r, err)
		return
	}

	// Defence-in-depth: the slot is jti-keyed and server-mutable, while
	// `impersonator_id` comes from the signed (authoritative) token. If
	// the two disagree, the slot belongs to a different operator — refuse
	// rather than restore the wrong session.
	if slot.OperatorUserID != operatorID {
		slog.Error("Impersonation end: slot/token operator mismatch",
			"jti", jti, "slot_operator", slot.OperatorUserID, "token_operator", operatorID)
		_ = renderEntityError(w, r, ErrNotImpersonating)
		return
	}
	// Cross-plane discriminator: refuse a slot whose OperatorKind is
	// anything other than backoffice. A wire mismatch here means
	// either a legacy slot (pre-#1785 Phase 5) survived a rolling
	// upgrade or the slot was written under a future second plane
	// without `end` learning to handle it. Failing loudly is safer
	// than restoring the wrong cookie shape.
	if slot.OperatorKind != services.ImpersonationOperatorBackoffice {
		slog.Error("Impersonation end: unsupported slot operator kind",
			"jti", jti, "operator_kind", string(slot.OperatorKind))
		_ = renderEntityError(w, r, ErrNotImpersonating)
		return
	}

	// Blacklist the impersonation token so it cannot be reused after
	// the session is ended, and drop the return slot.
	//
	// NOTE: impersonation-token revocation is durable only with a
	// Redis-backed token blacklist. With the default in-memory blacklist
	// a process restart loses this entry, so an already-ended
	// impersonation token is accepted again by JWTMiddleware until its
	// (≤30-min TTL) ceiling expires. Operators running a single
	// in-memory instance accept that ≤30-min window; multi-instance /
	// production deployments should configure
	// INVENTARIO_RUN_TOKEN_BLACKLIST_REDIS_URL.
	api.blacklistImpersonationToken(r.Context(), jti, slot.ExpiresAt)
	if delErr := api.store.Delete(r.Context(), jti); delErr != nil {
		slog.Warn("Impersonation end: failed to delete return slot", "jti", jti, "error", delErr)
	}

	operator, err := api.factorySet.BackofficeUserRegistry.Get(r.Context(), slot.OperatorUserID)
	if err != nil {
		slog.Error("Impersonation end: failed to reload back-office operator", "operator_id", slot.OperatorUserID, "error", err)
		api.auditEnd(r, slot, targetID, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	api.auditEnd(r, slot, targetID, true, "")
	slog.Info("Impersonation session ended", "operator_id", operator.ID, "target_id", slot.TargetUserID, "jti", jti)

	api.restoreBackofficeSession(w, r, operator, slot)
}

// currentImpersonation reports the active impersonation session for
// the FE banner.
//
// @Summary Read the active impersonation session
// @Description Convenience read for the FE impersonation banner. Returns `active=false` with no other fields when the
// @Description caller is not inside an impersonation session, and the target/operator/started_at/expires_at quartet when
// @Description it is. Reachable from EITHER the operator's back-office session OR the impersonated tenant session.
// @Tags admin
// @Produce json
// @Success 200 {object} ImpersonationStateResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
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

	// A target (or operator) that cannot be resolved means the session
	// is effectively broken — a vanished target is not a meaningful
	// "active" impersonation. Report inactive so the FE clears its
	// banner rather than rendering active=true with a nil user.
	target, terr := api.factorySet.UserRegistry.Get(r.Context(), slot.TargetUserID)
	if terr != nil {
		slog.Warn("Impersonation current: failed to resolve target user, reporting inactive",
			"jti", jti, "target_id", slot.TargetUserID, "error", terr)
		writeJSON(w, http.StatusOK, ImpersonationStateResponse{Active: false})
		return
	}
	operator, oerr := api.factorySet.BackofficeUserRegistry.Get(r.Context(), slot.OperatorUserID)
	if oerr != nil {
		slog.Warn("Impersonation current: failed to resolve back-office operator, reporting inactive",
			"jti", jti, "operator_id", slot.OperatorUserID, "error", oerr)
		writeJSON(w, http.StatusOK, ImpersonationStateResponse{Active: false})
		return
	}

	writeJSON(w, http.StatusOK, ImpersonationStateResponse{
		Active:     true,
		StartedAt:  &slot.StartedAt,
		ExpiresAt:  &slot.ExpiresAt,
		TargetUser: impersonationUserView(target),
		Operator:   impersonationOperatorView(operator),
	})
}

// restoreBackofficeSession re-issues the operator's back-office session
// after an impersonation session ends: it mints a fresh back-office
// access token bound to the operator's existing back-office refresh
// token row (when the return slot captured one) and re-plants the
// `backoffice_refresh_token` cookie. Extracted from endImpersonation to
// stay under the funlen budget.
func (api *adminImpersonationAPI) restoreBackofficeSession(w http.ResponseWriter, r *http.Request, operator *models.BackofficeUser, slot services.ImpersonationSlot) {
	// No IsActive re-check on the operator here on purpose:
	// RequireBackofficeAuth's per-request load rejects a disabled
	// account on the very next request, so minting the token for a
	// since-disabled operator is not an escalation — the middleware is
	// the real gate.
	//
	// Resolve the rti claim from the operator's captured refresh token
	// so back-office session-listing surfaces can flag the operator's
	// own session. An empty rti is fine — signBackofficeAccessTokenForEnd
	// omits the claim.
	rti := api.resolveBackofficeRefreshTokenID(r.Context(), slot.OperatorRefreshTokenRaw)

	tokenString, err := api.signBackofficeAccessTokenForEnd(operator, rti)
	if err != nil {
		slog.Error("Impersonation end: failed to mint back-office access token", "operator_id", operator.ID, "error", err)
		_ = internalServerError(w, r, err)
		return
	}

	// Restore the operator's full back-office session. The start
	// handler did not touch the operator's cookie, but re-planting it
	// here is defence-in-depth (in case the cookie was rotated or
	// cleared elsewhere) AND covers a paranoid future where start
	// captured + cleared the cookie.
	//
	//  - The operator had a back-office refresh cookie at start-time:
	//    re-plant it (still valid in the DB — impersonation never
	//    revoked it) so the operator can transparently refresh again
	//    afterwards.
	//  - The operator had no refresh cookie (a pure-bearer caller,
	//    e.g. a test or API client): there is nothing to restore, so
	//    no cookie is written. The operator continues on the freshly-
	//    minted access token until it expires, then logs in again —
	//    the same behaviour a pure-bearer client had before
	//    impersonation.
	if slot.OperatorRefreshTokenRaw != "" {
		writeBackofficeRefreshCookie(w, r, slot.OperatorRefreshTokenRaw, int(backofficeRefreshTokenExpiration.Seconds()))
	}

	// The FE swaps the in-memory access token back to the operator's
	// back-office token and the operator is whole again. Returning the
	// back-office LoginResponse shape signals the plane change to the
	// FE so it routes the response to the back-office state slice. The
	// operator is a *models.BackofficeUser (no is_system_admin field —
	// the back-office plane uses role-based authorization, not the
	// tenant-side grants table).
	writeBackofficeImpersonationEndResponse(w, tokenString, operator)
}

// resolveBackofficeRefreshTokenID looks up the row id of the
// operator's captured back-office refresh token so the restored access
// token can carry it as `rti`. Returns "" when no token was captured,
// the token is unknown, or the lookup fails — the `rti` claim is
// informational and a missing value only means session-listing
// surfaces cannot self-flag the row.
func (api *adminImpersonationAPI) resolveBackofficeRefreshTokenID(ctx context.Context, rawToken string) string {
	if rawToken == "" || api.factorySet.BackofficeRefreshTokenRegistry == nil {
		return ""
	}
	rt, err := api.factorySet.BackofficeRefreshTokenRegistry.GetByHash(ctx, hashBackofficeRefreshToken(rawToken))
	if err != nil {
		return ""
	}
	return rt.ID
}

// signImpersonationToken builds and signs the impersonation access
// token. Claims:
//   - user_id = target tenant user id (so the standard tenant
//     JWTMiddleware loads the target on the impersonated session's
//     requests),
//   - tenant_id = target's tenant,
//   - impersonator_id = the back-office operator's id (#1785 Phase 5;
//     replaces the legacy `impersonated_by` claim),
//   - impersonator_type = "backoffice_user" — pinned so a future
//     second operator-plane never silently aliases this slot,
//   - imp = true,
//   - is_system_admin = false (an impersonated session never carries
//     platform-admin authority; pinned defence-in-depth even though
//     impersonationTargetGuard already rejects admin targets).
//
// The token is consumed on the standard tenant access path by
// JWTMiddleware, so it carries token_type=access (#1778). The audit
// helper impersonatorFromContext reads `imp` + `impersonator_id` to
// auto-fill the audit-log `impersonated_by` column on every action
// taken under the impersonated session.
func (api *adminImpersonationAPI) signImpersonationToken(target *models.User, operatorID, jti string, issuedAt, expiresAt time.Time) (string, error) {
	claims := jwt.MapClaims{
		"jti":               jti,
		"user_id":           target.ID,
		"tenant_id":         target.TenantID,
		"impersonator_id":   operatorID,
		"impersonator_type": impersonatorTypeBackofficeUser,
		"imp":               true,
		"is_system_admin":   false,
		"token_type":        accessTokenType,
		"iat":               issuedAt.Unix(),
		"exp":               expiresAt.Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(api.jwtSecret)
}

// signBackofficeAccessTokenForEnd mints a normal (non-impersonation)
// back-office access token for the operator when an impersonation
// session ends. Mirrors BackofficeAuthAPI.issueAccessToken so the
// restored session is indistinguishable from a fresh
// /backoffice/auth/login or /backoffice/auth/refresh response —
// including `aud="backoffice"`, `admin_id`, and `role`.
func (api *adminImpersonationAPI) signBackofficeAccessTokenForEnd(operator *models.BackofficeUser, rti string) (string, error) {
	now := time.Now()
	claims := jwt.MapClaims{
		"jti":        uuid.New().String(),
		"admin_id":   operator.ID,
		"role":       string(operator.Role),
		"aud":        backofficeTokenAudience,
		"token_type": accessTokenType,
		"iat":        now.Unix(),
		"exp":        now.Add(backofficeAccessTokenExpiration).Unix(),
	}
	if rti != "" {
		claims["rti"] = rti
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(api.jwtSecret)
}

// blacklistImpersonationToken adds the impersonation token's JTI to the
// blacklist so it is rejected on any further use after `end`.
// Best-effort: a blacklist outage is logged but does not block the end
// flow — the token expires on its own at the (≤ 30 min) TTL regardless.
// The blacklister is the same service the rest of the apiserver uses,
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

// checkRateLimit enforces the per-operator impersonation-start rate
// limit. Returns true when the request may proceed. On a limiter
// backend error it fails open (consistent with the other rate limiters
// in this package). On a genuine limit hit it writes the 429 + code
// response and returns false.
func (api *adminImpersonationAPI) checkRateLimit(w http.ResponseWriter, r *http.Request, operatorID string) bool {
	if api.rateLimiter == nil {
		return true
	}
	res, err := api.rateLimiter.CheckImpersonationAttempt(r.Context(), operatorID)
	if err != nil {
		slog.Error("Impersonation rate limiter error", "operator_id", operatorID, "error", err)
		return true
	}
	if !res.Allowed {
		retryAfter := max(int(time.Until(res.ResetAt).Seconds()), 0)
		w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
		api.auditStart(r, operatorID, "", "", "", false, "rate limit exceeded")
		_ = codedTooManyRequestsError(w, r, services.ErrRateLimitExceeded, AdminImpersonateRateLimitedCode,
			map[string]any{"retry_after_seconds": retryAfter})
		return false
	}
	return true
}

// auditStart writes the admin.impersonate_start audit row. The
// impersonator column is NOT set from context here (the start request
// runs as the back-office operator, not an impersonated session);
// LogAdmin still reads the breadcrumb reason. Best-effort like the
// other admin audit helpers.
func (api *adminImpersonationAPI) auditStart(r *http.Request, operatorID, reason, targetID, targetTenantID string, success bool, errMsg string) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      AuditActionAdminImpersonateStart,
		ActorID:     nullableString(operatorID),
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

// auditEnd writes the admin.impersonate_end audit row. The request
// runs under the impersonation token, so the actor-of-record is the
// impersonated target (ActorID → UserID = targetID) and LogAdmin
// auto-fills the ImpersonatedBy column from the `imp` /
// `impersonator_id` claims — the row records "operator acting as
// target" without the call site doing anything special.
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
// valid impersonation subject. System-admin status is resolved via the
// dedicated grants registry (#1784) — the legacy IsSystemAdmin column
// on users is gone.
func impersonationTargetGuard(ctx context.Context, grants registry.SystemAdminGrantRegistry, target *models.User) error {
	if grants != nil {
		ok, err := grants.Exists(ctx, target.ID)
		if err != nil {
			return errxtrace.Wrap("impersonation guard: grant lookup failed", err)
		}
		if ok {
			return ErrCannotImpersonateAdmin
		}
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

// claimsAreImpersonation reports whether the given JWT claims belong
// to an impersonation token: `imp` is true AND `impersonator_id` is a
// non-empty string. Both conditions are required so a token with a
// stray `imp` claim but no operator-of-record is not mistaken for a
// genuine impersonation session.
//
// Phase 5 of issue #1785 renames the operator-of-record claim from
// `impersonated_by` to `impersonator_id`; the audit-log column stays
// named `impersonated_by` because the column tracks "who was the
// impersonator?" — the rename only applies to the wire-side claim.
func claimsAreImpersonation(claims jwt.MapClaims) bool {
	if claims == nil {
		return false
	}
	imp, _ := claims["imp"].(bool)
	if !imp {
		return false
	}
	by, ok := claims["impersonator_id"].(string)
	return ok && by != ""
}

// backofficeRefreshCookieValue returns the raw
// `backoffice_refresh_token` cookie value carried by the request, or
// "" when no such cookie is present. Used by impersonation-start to
// capture the operator's back-office refresh token into the return
// slot so `end` can re-plant it. Distinct from refreshCookieValue
// (tenant refresh cookie) — the two cookies live at different paths
// and never collide.
func backofficeRefreshCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(backofficeRefreshTokenCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
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

// impersonationOperatorView projects a *models.BackofficeUser into the
// narrow identity-only view embedded in the impersonation-state
// response (#1785 Phase 5). Distinct from impersonationUserView so
// the operator's tenant-agnostic shape is preserved on the wire.
func impersonationOperatorView(u *models.BackofficeUser) *ImpersonationOperatorView {
	return &ImpersonationOperatorView{
		ID:    u.ID,
		Email: u.Email,
		Name:  u.Name,
		Role:  string(u.Role),
	}
}

// writeImpersonationLoginResponse writes a LoginResponse with the given
// access token, TTL, and CSRF token. Reuses the LoginResponse shape so
// the FE handles the impersonation-start and normal-login responses
// with one code path on the tenant side.
//
// csrfToken MUST be a token freshly minted for the response's effective
// tenant user (the target on start): CSRF validation is per-user, so
// after the identity swap the SPA cannot reuse the operator's previous
// token — its first mutating request would 403. The handler also
// mirrors the token into the X-CSRF-Token response header, matching
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

// writeBackofficeImpersonationEndResponse writes the
// BackofficeLoginResponse returned by POST /admin/impersonation/end.
// Reuses the wire shape /backoffice/auth/login and /backoffice/auth/refresh
// use so the FE handles the end response through the same code path it
// already handles a refresh response with — restoring the operator's
// back-office identity slice and discarding the tenant slice.
//
// CSRF is NOT minted here: the back-office plane does not currently
// CSRF-protect its surface, so the response carries no CSRF token.
func writeBackofficeImpersonationEndResponse(w http.ResponseWriter, accessToken string, operator *models.BackofficeUser) {
	writeJSON(w, http.StatusOK, BackofficeLoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(backofficeAccessTokenExpiration.Seconds()),
		User:        backofficeProfileFromUser(operator),
	})
}
