package apiserver

import (
	"context"
	"encoding/json"
	"errors"
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
	// impersonation (max 500 chars).
	Reason string `json:"reason,omitempty"`
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
	jwtSecret    []byte
	ttl          time.Duration
}

// clampImpersonationTTL resolves the effective impersonation TTL:
// zero/negative falls back to the 30-min default, and any value above
// the 30-min ceiling is clamped down so a misconfigured
// INVENTARIO_RUN_IMPERSONATION_TTL cannot widen the borrowed-identity
// window past what the #1750 spec allows.
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
// @Description refreshed and cannot start a nested impersonation.
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

	// The impersonation token is set as the active session via the body
	// (same LoginResponse shape the FE already handles). The httpOnly
	// refresh cookie is intentionally left untouched: impersonation
	// sessions are non-refreshable, and the cookie still belongs to the
	// admin's own session — `end` consumes it to restore the operator.
	writeImpersonationLoginResponse(w, tokenString, ttl, target)
}

// endImpersonation revokes the impersonation access token and restores
// the admin's original session.
//
// @Summary End an impersonation session (admin)
// @Description Ends the active impersonation session: blacklists the impersonation access token and restores the operator's own session. Must be called with the impersonation access token (the token carrying `imp=true`).
// @Description Returns 422 with `admin.impersonate.not_active` when the caller is not inside an impersonation session.
// @Tags admin
// @Produce json
// @Success 200 {object} LoginResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - no active impersonation session"
// @Router /admin/impersonation/end [post]
func (api *adminImpersonationAPI) endImpersonation(w http.ResponseWriter, r *http.Request) {
	claims := appctx.JWTClaimsFromContext(r.Context())
	if !claimsAreImpersonation(claims) {
		_ = renderEntityError(w, r, ErrNotImpersonating)
		return
	}

	jti, _ := claims["jti"].(string)
	adminID, _ := claims["impersonated_by"].(string)
	targetID, _ := claims["user_id"].(string)

	slot, err := api.store.Get(r.Context(), jti)
	if err != nil {
		// The token is a valid impersonation token but the slot is gone
		// (already ended, or the process restarted). Treat as "not
		// active" — the FE banner clears and the operator re-logs in.
		slog.Warn("Impersonation end: return slot not found", "jti", jti, "admin_id", adminID)
		_ = renderEntityError(w, r, ErrNotImpersonating)
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
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
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
		// Token says impersonation, but the slot is gone — report
		// inactive so the FE clears its banner.
		writeJSON(w, http.StatusOK, ImpersonationStateResponse{Active: false})
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
// captured one) and writes the LoginResponse. Extracted from
// endImpersonation to stay under the funlen budget.
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

	// The admin's refresh cookie was never touched by the impersonation
	// start, so it is still valid and still in the browser — no Set-Cookie
	// needed here. The FE swaps the in-memory access token back to the
	// admin's and the operator is whole again.
	writeImpersonationLoginResponse(w, tokenString, accessTokenExpiration, admin)
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

// refreshCookieValue returns the raw refresh-token cookie value, or ""
// when the request carries no refresh cookie (e.g. a pure-bearer client).
func refreshCookieValue(r *http.Request) string {
	cookie, err := r.Cookie(refreshTokenCookieName)
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

// writeImpersonationLoginResponse writes a LoginResponse with the given
// access token and TTL. Reuses the LoginResponse shape so the FE handles
// the impersonation-start, impersonation-end, and normal-login responses
// with one code path. CSRFToken is intentionally left empty — the
// impersonation flow does not rotate CSRF tokens; the FE recovers the
// CSRF token from the /auth/me header on the next navigation.
func writeImpersonationLoginResponse(w http.ResponseWriter, accessToken string, ttl time.Duration, user *models.User) {
	writeJSON(w, http.StatusOK, LoginResponse{
		AccessToken: accessToken,
		TokenType:   "Bearer",
		ExpiresIn:   int(ttl.Seconds()),
		User:        user,
	})
}
