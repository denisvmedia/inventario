package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// Admin user block/unblock action names. Kept as constants so the audit
// trail uses the same literals as the swagger tags and the FE filter
// chips on the admin-activity page. Mirrors the "admin.<noun>_<verb>"
// pattern set by #1745 (admin.grant_system_admin / admin.revoke_system_admin).
const (
	// AuditActionAdminUserBlock is the audit-row Action emitted on a
	// successful block of another (non-admin) user. The matching
	// failure rows (self-block, admin-without-force) reuse the same
	// Action with Success=false so a single filter pulls the whole
	// attempt history.
	AuditActionAdminUserBlock = "admin.user_block"
	// AuditActionAdminUserBlockForce is the audit-row Action emitted
	// when an admin blocks ANOTHER system admin with force=true. The
	// distinct Action means audit-side severity tooling can pivot on
	// the literal without re-parsing the breadcrumb JSON. The row's
	// breadcrumb also carries `forced: true` so post-hoc analysis can
	// branch on either signal.
	AuditActionAdminUserBlockForce = "admin.user_block_force"
	// AuditActionAdminUserUnblock is the audit-row Action emitted on a
	// successful unblock. Unblock has no force variant — re-activating
	// an account is symmetric for admins and non-admins.
	AuditActionAdminUserUnblock = "admin.user_unblock"
)

// JSON:API error codes returned by the block / unblock endpoints. Kept
// as constants so the swagger annotations, the FE branch table, and the
// tests reference the same literals. Codes follow the "admin.block.*"
// family declared in the #1747 issue spec; the body-validation codes
// are shared across block and unblock since both DTOs carry the same
// reason field.
const (
	// AdminBlockSelfBlockedCode signals "the caller tried to block
	// their own account". Maps to a 422.
	AdminBlockSelfBlockedCode = "admin.block.self_blocked"
	// AdminBlockAdminRequiresForceCode signals "the target is another
	// system admin, send force=true to override". Maps to a 422.
	AdminBlockAdminRequiresForceCode = "admin.block.admin_requires_force"
	// AdminBlockReasonRequiredCode signals "the request body is missing
	// the required `reason` field (or it is blank)". Maps to a 422.
	// Shared between block and unblock — both DTOs require a reason.
	AdminBlockReasonRequiredCode = "admin.block.reason_required"
	// AdminBlockReasonTooLongCode signals "the supplied reason exceeds
	// the 500-char cap". Maps to a 422. Shared between block and
	// unblock.
	AdminBlockReasonTooLongCode = "admin.block.reason_too_long"
)

// adminBlockReasonMaxLen caps the reason string at 500 characters.
// Going higher serves no audit-readability goal and risks bloating the
// audit_logs.user_agent column we tunnel the breadcrumb through.
const adminBlockReasonMaxLen = 500

// AdminBlockRequest is the request body for POST /admin/users/{userID}/block.
//
// `reason` is required and is persisted into the audit log breadcrumb so
// every state transition carries an operator-supplied justification.
// `force` is the explicit override that allows blocking another system
// admin — without it, an admin-on-admin block returns 422 with code
// "admin.block.admin_requires_force".
type AdminBlockRequest struct {
	// Reason is the free-form justification for the block (max 500 chars).
	Reason string `json:"reason" validate:"required,max=500"`
	// Force overrides the "cannot block another system admin" guard.
	// Has no effect when blocking a non-admin user.
	Force bool `json:"force,omitempty"`
}

// AdminUnblockRequest is the request body for POST /admin/users/{userID}/unblock.
// Symmetric with AdminBlockRequest — `reason` is required so the audit
// breadcrumb carries it.
type AdminUnblockRequest struct {
	// Reason is the free-form justification for the unblock (max 500 chars).
	Reason string `json:"reason" validate:"required,max=500"`
}

// AdminUserView is the JSON:API-style attributes block returned by the
// block / unblock endpoints. Deliberately narrow: only the fields a
// system admin actually cares about for a block / unblock receipt
// (identity + state), not every user column. The FE renders a confirm
// toast off of this; richer admin user-detail surfaces ship in later
// #1744 issues.
type AdminUserView struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Name          string `json:"name"`
	IsActive      bool   `json:"is_active"`
	IsSystemAdmin bool   `json:"is_system_admin"`
	TenantID      string `json:"tenant_id"`
}

// AdminUserEnvelope is the JSON:API envelope returned by block /
// unblock. Single-resource shape ({"data": {...}}) matches what the
// rest of the protected admin surface will return as #1744 fills out.
type AdminUserEnvelope struct {
	Data AdminUserResource `json:"data"`
}

// AdminUserResource is the JSON:API resource block carried inside
// AdminUserEnvelope. Keeps `type` + `id` at the top level and pushes
// the rest under `attributes`, matching the project's other JSON:API
// responses.
type AdminUserResource struct {
	Type       string        `json:"type"`
	ID         string        `json:"id"`
	Attributes AdminUserView `json:"attributes"`
}

// adminUsersAPI backs the /admin/users/* and /admin/tenants/{tenantID}/users
// routes. Holds the FactorySet directly (not the per-request user-aware
// Set) for the same cross-tenant reason adminTenantsAPI does — admin
// endpoints intentionally cross tenants and the user-aware registries
// would constrain the admin to their own tenant, defeating the surface.
// The blacklist dependency is the #1747 block-cascade addition: needed
// to bump the JWT-iat staleness threshold so live access tokens minted
// before the block are rejected on next use.
type adminUsersAPI struct {
	factorySet   *registry.FactorySet
	blacklist    services.TokenBlacklister
	auditService services.AuditLogger
}

// listTenantUsers returns a paginated, filtered, sorted listing of
// every user in the target tenant. The endpoint crosses tenants — the
// admin caller may not be a member of the target tenant — and the
// registry layer bypasses RLS via `SET LOCAL row_security = off` for
// defense-in-depth.
//
// @Summary List users for a tenant (admin)
// @Description Returns users in the target tenant with computed group_membership_count. Tri-state ?is_active. Pagination via ?page&per_page; ?q matches email/name (ILIKE); ?sort=<field> with `-` prefix or ?order=asc|desc.
// @Tags admin
// @Produce json-api
// @Param tenantID path string true "Tenant ID"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Param q query string false "Search term — ILIKE match on email/name"
// @Param is_active query boolean false "Tri-state active-flag filter: true (active only), false (inactive only), or omit the param entirely for no filter. Unknown values are ignored."
// @Param sort query string false "Sort field: email|name|created_at|last_login_at|is_active (prefix with - for desc)"
// @Param order query string false "Sort direction override: asc|desc (wins over `-` prefix)"
// @Success 200 {object} jsonapi.AdminUsersResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Tenant not found"
// @Router /admin/tenants/{tenantID}/users [get]
func (api *adminUsersAPI) listTenantUsers(w http.ResponseWriter, r *http.Request) {
	tenantID := chi.URLParam(r, "tenantID")
	if tenantID == "" {
		api.auditListUsers(r, tenantID, registry.ErrNotFound)
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	// Probe the tenant first so a typo in the URL surfaces as 404
	// rather than an empty listing. The probe and the listing share a
	// single audit row keyed on `admin.list_tenant_users`.
	if _, err := api.factorySet.TenantRegistry.Get(r.Context(), tenantID); err != nil {
		api.auditListUsers(r, tenantID, err)
		_ = renderEntityError(w, r, err)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	sortField, sortDesc := parseAdminSortAndOrder(q.Get("sort"), q.Get("order"))

	opts := registry.AdminUserListOptions{
		Page:      page,
		PerPage:   perPage,
		Query:     q.Get("q"),
		IsActive:  parseTriStateBool(q.Get("is_active")),
		SortField: registry.AdminUserSortField(sortField),
		SortDesc:  sortDesc,
	}

	items, total, err := api.factorySet.UserRegistry.ListAdminByTenant(r.Context(), tenantID, opts)
	if err != nil {
		api.auditListUsers(r, tenantID, err)
		_ = renderEntityError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)
	// Audit AFTER render so a JSON-encoding / response-writer failure
	// turns into a Success=false row instead of claiming the client
	// received their data successfully.
	renderErr := render.Render(w, r, jsonapi.NewAdminUsersResponse(items, page, perPage, total))
	api.auditListUsers(r, tenantID, renderErr)
	if renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// getUser returns the full user detail row: identity fields,
// is_active, last_login_at, the resolved group memberships, and the
// active_session_count from refresh_tokens. The handler resolves
// memberships by joining via the GroupMembership registry and
// fetching the matching LocationGroup rows in a small batched lookup
// so the FE doesn't N+1 the group registry per row.
//
// @Summary Get user (admin)
// @Description Returns the user detail across tenants: identity, is_active, last_login_at, group memberships (group_id, group_slug, group_name, role, joined_at), and active_session_count from unrevoked refresh tokens. No password hash.
// @Tags admin
// @Produce json-api
// @Param userID path string true "User ID"
// @Success 200 {object} jsonapi.AdminUserResponse "OK"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "User not found"
// @Router /admin/users/{userID} [get]
func (api *adminUsersAPI) getUser(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userID")
	if userID == "" {
		api.auditGetUser(r, userID, "", registry.ErrNotFound)
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	user, err := api.factorySet.UserRegistry.Get(r.Context(), userID)
	if err != nil {
		api.auditGetUser(r, userID, "", err)
		_ = renderEntityError(w, r, err)
		return
	}

	// Memberships are part of the resource identity (the FE renders
	// group access on this page), so a registry failure here is a 500.
	// Contrast with the session-count path below, which degrades
	// silently because active_session_count is derived metadata and
	// the user-detail row is still useful without it.
	memberships, mlErr := api.factorySet.GroupMembershipRegistry.ListByUser(r.Context(), user.TenantID, user.ID)
	if mlErr != nil && !errors.Is(mlErr, registry.ErrNotFound) {
		api.auditGetUser(r, userID, user.TenantID, mlErr)
		_ = renderEntityError(w, r, mlErr)
		return
	}

	// Resolve each membership to (group_id, group_slug, group_name).
	// The membership rows carry only the group_id; the slug + name
	// live on LocationGroup, fetched per row. Tenant-scoped lookup
	// (ListByTenant) is one round-trip with an in-memory join.
	groupsByID := map[string]*models.LocationGroup{}
	if user.TenantID != "" {
		groups, glErr := api.factorySet.LocationGroupRegistry.ListByTenant(r.Context(), user.TenantID)
		if glErr != nil {
			api.auditGetUser(r, userID, user.TenantID, glErr)
			_ = renderEntityError(w, r, glErr)
			return
		}
		for _, g := range groups {
			groupsByID[g.ID] = g
		}
	}

	mems := make([]jsonapi.AdminUserGroupMembership, 0, len(memberships))
	for _, m := range memberships {
		row := jsonapi.AdminUserGroupMembership{
			GroupID:  m.GroupID,
			Role:     m.Role,
			JoinedAt: m.JoinedAt,
		}
		if g, ok := groupsByID[m.GroupID]; ok {
			row.GroupSlug = g.Slug
			row.GroupName = g.Name
		}
		mems = append(mems, row)
	}

	sessionCount, sErr := api.factorySet.UserRegistry.CountSessionsByUser(r.Context(), user.ID)
	if sErr != nil {
		// Count failures degrade to zero rather than 500 so a transient
		// registry hiccup doesn't hide the user detail. The audit row
		// still flags the failure via a separate `admin.get_user_sessions`
		// action so operators can spot a pattern of session-count
		// outages without taking down the primary endpoint — note the
		// primary `admin.get_user` row reports Success: true, so audit
		// readers correlating on ActorID + timestamp see both rows.
		api.auditSessionCountFailure(r, user.ID, user.TenantID, sErr)
		sessionCount = 0
	}

	// Audit AFTER render — a JSON-encoding / writer failure should
	// land in the audit trail as Success=false rather than the
	// previous "say success then 500" pattern. The session-count
	// degradation audit above is a separate row keyed on
	// `admin.get_user_sessions` and is unaffected by render outcome.
	renderErr := render.Render(w, r, jsonapi.NewAdminUserResponse(jsonapi.AdminUserResponseInput{
		User:               user,
		Memberships:        mems,
		ActiveSessionCount: sessionCount,
	}))
	api.auditGetUser(r, userID, user.TenantID, renderErr)
	if renderErr != nil {
		_ = internalServerError(w, r, renderErr)
	}
}

// blockUser deactivates a user account and tears down their live
// sessions: refresh tokens revoked, access tokens iat-stale-rejected
// via the user-level blacklist entry. Guards prevent self-lockout and
// silent admin-on-admin demotion (an explicit `force: true` is
// required for the latter).
// @Summary Block (deactivate) a user account
// @Description Sets the user's `is_active` flag to false, revokes every refresh token, and bumps the JWT-blacklist staleness threshold so live access tokens are rejected on next use.
// @Description Returns 422 with `admin.block.self_blocked` when the caller targets their own account, and 422 with `admin.block.admin_requires_force` when targeting another system admin without `force=true`.
// @Description Body-validation rejections surface as 422 with `admin.block.reason_required` (missing or blank `reason`) or `admin.block.reason_too_long` (reason exceeds 500 characters).
// @Tags admin
// @Accept json
// @Produce json
// @Param userID path string true "Target user ID"
// @Param data body AdminBlockRequest true "Block request"
// @Success 200 {object} AdminUserEnvelope "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown user"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - self-block or admin-on-admin without force"
// @Router /admin/users/{userID}/block [post]
func (api *adminUsersAPI) blockUser(w http.ResponseWriter, r *http.Request) {
	actor := appctx.UserFromContext(r.Context())
	if actor == nil {
		// Defence-in-depth: RequireSystemAdmin should have caught this
		// already. Reaching here means the middleware chain is wired
		// wrong; render the same shape RequireSystemAdmin would.
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}

	userID := chi.URLParam(r, "userID")
	if strings.TrimSpace(userID) == "" {
		_ = badRequest(w, r, errors.New("missing user id"))
		return
	}

	req, ok := api.decodeBlockRequest(w, r)
	if !ok {
		return
	}

	target, err := api.factorySet.UserRegistry.Get(r.Context(), userID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			api.logBlockOutcome(r, actor, userID, "", req, false, err.Error(), false)
			_ = renderEntityError(w, r, err)
			return
		}
		slog.Error("admin block: failed to load user", "user_id", userID, "error", err)
		api.logBlockOutcome(r, actor, userID, "", req, false, err.Error(), false)
		_ = internalServerError(w, r, err)
		return
	}

	// Self-block guard. Under an impersonation session (#1750) the
	// JWT's user_id is the impersonated user; the operator-of-record
	// lives in the `impersonated_by` claim. Resolve the real operator
	// via services.ImpersonatorIDFromContext so an operator can't
	// self-block by acting through an impersonated identity. The guard
	// runs above the idempotency check so an operator who tries to
	// block themselves always sees the typed error, even if their
	// account is already inactive.
	if target.ID == operatorIDFromContext(r.Context(), actor) {
		api.logBlockOutcome(r, actor, target.ID, target.TenantID, req, false, ErrAdminCannotBlockSelf.Error(), false)
		_ = renderEntityError(w, r, ErrAdminCannotBlockSelf)
		return
	}

	if !target.IsActive {
		// Idempotent: the user is already blocked. Re-run the cascade
		// anyway — the blacklist ring has a 30-min TTL, so a re-block
		// after the ring expired (or after a freshly-issued token
		// landed in the window between block attempts) needs the
		// cascade to invalidate whatever's live now. The cascade is
		// itself idempotent. Record `forced=false`: a no-op block did
		// not override the admin-on-admin guard, even when req.Force
		// was set — nothing was "forced". The cascade then runs
		// regardless of req.Force so the iat-staleness bump still
		// fires for peer-admin re-blocks. Runs above the
		// admin-without-force guard so re-blocking an already-blocked
		// peer admin is a 200 no-op rather than a surprising 422.
		cascadeErrMsg := api.applyBlockCascade(r, target.ID)
		api.logBlockOutcome(r, actor, target.ID, target.TenantID, req, cascadeErrMsg == "", cascadeErrMsg, false)
		api.writeUserEnvelope(w, http.StatusOK, target)
		return
	}

	if target.IsSystemAdmin && !req.Force {
		api.logBlockOutcome(r, actor, target.ID, target.TenantID, req, false, ErrAdminCannotBlockAdminWithoutForce.Error(), false)
		_ = renderEntityError(w, r, ErrAdminCannotBlockAdminWithoutForce)
		return
	}

	// Forced flag is true only when the operator explicitly overrode
	// the admin-on-admin guard AND the block actually fires (target
	// was active above). Surface it on the audit row so post-hoc
	// analysis can pivot on "operator blocked a live peer admin"
	// without re-parsing the breadcrumb JSON.
	forced := target.IsSystemAdmin && req.Force

	updated := *target
	updated.IsActive = false
	saved, err := api.factorySet.UserRegistry.Update(r.Context(), updated)
	if err != nil {
		slog.Error("admin block: failed to deactivate user", "user_id", target.ID, "error", err)
		api.logBlockOutcome(r, actor, target.ID, target.TenantID, req, false, err.Error(), forced)
		_ = internalServerError(w, r, err)
		return
	}

	// Best-effort cascade: revoke refresh tokens and bump the iat
	// staleness threshold so live access tokens are rejected on next
	// use. Failures here don't roll back the is_active flip — the
	// account is already deactivated, so the request has met its main
	// invariant; the cascade failures get logged for operator
	// awareness and the audit row records ErrMsg if either step
	// blipped.
	cascadeErrMsg := api.applyBlockCascade(r, target.ID)
	api.logBlockOutcome(r, actor, target.ID, target.TenantID, req, cascadeErrMsg == "", cascadeErrMsg, forced)
	api.writeUserEnvelope(w, http.StatusOK, saved)
}

// operatorIDFromContext returns the ID of the user who actually
// initiated the request: the impersonator if an impersonation session
// is active (#1750 — `imp` true with non-empty `impersonated_by`),
// otherwise actor.ID. Used by the self-block guard so an operator
// cannot deactivate their own account by routing the block through an
// impersonated identity.
func operatorIDFromContext(ctx context.Context, actor *models.User) string {
	if imp := services.ImpersonatorIDFromContext(ctx); imp != nil && *imp != "" {
		return *imp
	}
	return actor.ID
}

// unblockUser flips IsActive back to true. Does NOT re-issue tokens —
// the user has to log in again — and does NOT clear the blacklist
// entry: the iat-staleness ring expires on its own and stale tokens
// from before the block stay rejected even after unblock (#1747 spec).
// @Summary Unblock (reactivate) a user account
// @Description Sets the user's `is_active` flag back to true. Does NOT re-issue tokens — the user must log in again.
// @Description Does NOT clear the JWT-blacklist staleness threshold either, so any access tokens that were issued before the block stay rejected until the iat-staleness ring expires.
// @Description Body-validation rejections surface as 422 with `admin.block.reason_required` (missing or blank `reason`) or `admin.block.reason_too_long` (reason exceeds 500 characters); the codes are shared with the block endpoint.
// @Tags admin
// @Accept json
// @Produce json
// @Param userID path string true "Target user ID"
// @Param data body AdminUnblockRequest true "Unblock request"
// @Success 200 {object} AdminUserEnvelope "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown user"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - invalid reason"
// @Router /admin/users/{userID}/unblock [post]
func (api *adminUsersAPI) unblockUser(w http.ResponseWriter, r *http.Request) {
	actor := appctx.UserFromContext(r.Context())
	if actor == nil {
		_ = unauthorizedError(w, r, ErrMissingUserContext)
		return
	}

	userID := chi.URLParam(r, "userID")
	if strings.TrimSpace(userID) == "" {
		_ = badRequest(w, r, errors.New("missing user id"))
		return
	}

	req, ok := api.decodeUnblockRequest(w, r)
	if !ok {
		return
	}

	target, err := api.factorySet.UserRegistry.Get(r.Context(), userID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			api.logUnblockOutcome(r, actor, userID, "", req, false, err.Error())
			_ = renderEntityError(w, r, err)
			return
		}
		slog.Error("admin unblock: failed to load user", "user_id", userID, "error", err)
		api.logUnblockOutcome(r, actor, userID, "", req, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	if target.IsActive {
		// Idempotent: the user is already active. Audit and return
		// 200 — the alternative would be a 422 that the FE doesn't
		// actually need to branch on.
		api.logUnblockOutcome(r, actor, target.ID, target.TenantID, req, true, "")
		api.writeUserEnvelope(w, http.StatusOK, target)
		return
	}

	updated := *target
	updated.IsActive = true
	saved, err := api.factorySet.UserRegistry.Update(r.Context(), updated)
	if err != nil {
		slog.Error("admin unblock: failed to reactivate user", "user_id", target.ID, "error", err)
		api.logUnblockOutcome(r, actor, target.ID, target.TenantID, req, false, err.Error())
		_ = internalServerError(w, r, err)
		return
	}

	api.logUnblockOutcome(r, actor, target.ID, target.TenantID, req, true, "")
	api.writeUserEnvelope(w, http.StatusOK, saved)
}

// decodeBlockRequest parses + validates the POST /block body. Writes
// the right error response and returns ok=false on failure so the
// caller can early-return without sprinkling decode-error noise.
func (api *adminUsersAPI) decodeBlockRequest(w http.ResponseWriter, r *http.Request) (AdminBlockRequest, bool) {
	var req AdminBlockRequest
	if r.Body == nil {
		_ = badRequest(w, r, errors.New("missing request body"))
		return req, false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		_ = badRequest(w, r, err)
		return req, false
	}
	if dec.More() {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return req, false
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is required"), AdminBlockReasonRequiredCode)
		return req, false
	}
	if len(req.Reason) > adminBlockReasonMaxLen {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is too long"), AdminBlockReasonTooLongCode)
		return req, false
	}
	return req, true
}

// decodeUnblockRequest mirrors decodeBlockRequest for the unblock
// shape. Kept as a separate helper so the swagger DTO stays narrow
// (no `force` field on unblock) and so a future expansion of either
// body type doesn't entangle the two.
func (api *adminUsersAPI) decodeUnblockRequest(w http.ResponseWriter, r *http.Request) (AdminUnblockRequest, bool) {
	var req AdminUnblockRequest
	if r.Body == nil {
		_ = badRequest(w, r, errors.New("missing request body"))
		return req, false
	}
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		_ = badRequest(w, r, err)
		return req, false
	}
	if dec.More() {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return req, false
	}
	req.Reason = strings.TrimSpace(req.Reason)
	if req.Reason == "" {
		// Reuses the admin.block.reason_required code because both DTOs
		// share the same reason field semantics — having a parallel
		// admin.unblock.* family would just duplicate the FE branch.
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is required"), AdminBlockReasonRequiredCode)
		return req, false
	}
	if len(req.Reason) > adminBlockReasonMaxLen {
		_ = codedUnprocessableEntityError(w, r, errors.New("reason is too long"), AdminBlockReasonTooLongCode)
		return req, false
	}
	return req, true
}

// applyBlockCascade tears down the user's live sessions after the
// is_active flip has committed. Returns the concatenated error message
// (or "" on success) so the audit row records the cause when either
// step blipped; both steps run regardless of the other's outcome so a
// transient refresh-token failure doesn't keep stale access tokens
// alive.
func (api *adminUsersAPI) applyBlockCascade(r *http.Request, userID string) string {
	var msgs []string
	if api.factorySet != nil && api.factorySet.RefreshTokenRegistry != nil {
		if err := api.factorySet.RefreshTokenRegistry.RevokeByUserID(r.Context(), userID); err != nil {
			slog.Warn("admin block: failed to revoke refresh tokens", "user_id", userID, "error", err)
			msgs = append(msgs, "revoke_refresh_tokens: "+err.Error())
		}
	}
	if api.blacklist != nil {
		// Use 2× accessTokenExpiration to match the password-reset /
		// MFA-reset cascade so live access tokens (15 min lifetime)
		// are reliably rejected via the iat-staleness check. The
		// blacklist entry is independent of is_active so unblocking
		// the user does NOT clear it — stale tokens from before the
		// block stay rejected until the ring expires (#1747 spec).
		//
		// Race window: BlacklistUserTokens stamps `since` at second
		// precision (time.Unix(now.Unix(), 0)), and the JWT iat claim
		// is also second-precision. checkUserBlacklistIat uses strict
		// iat.Before(since), so a token whose iat falls in the same
		// wall-clock second as the blacklist call survives. The
		// alternatives (sub-second precision, bumping `since` by +1s)
		// either need a blacklister API change or risk false-positive
		// invalidation of unrelated tokens; the one-second window is
		// short enough that an operator can re-block to close it.
		if err := api.blacklist.BlacklistUserTokens(r.Context(), userID, 2*accessTokenExpiration); err != nil {
			slog.Warn("admin block: failed to blacklist user tokens", "user_id", userID, "error", err)
			msgs = append(msgs, "blacklist_user_tokens: "+err.Error())
		}
	}
	return strings.Join(msgs, "; ")
}

// logBlockOutcome writes the admin.user_block audit row (or its
// _force variant). Best-effort: nil-safe wrapper for the case where
// AuditService was not wired in. `forced` selects the audit Action
// (block vs block_force) and is recorded in the breadcrumb — it is a
// data tag carried through to the persisted row, not a control flag
// branching the work the function does.
//
//revive:disable-next-line:flag-parameter
func (api *adminUsersAPI) logBlockOutcome(
	r *http.Request,
	actor *models.User,
	subjectID, subjectTenantID string,
	req AdminBlockRequest,
	success bool,
	errMsg string,
	forced bool,
) {
	if api.auditService == nil {
		return
	}
	action := AuditActionAdminUserBlock
	if forced {
		action = AuditActionAdminUserBlockForce
	}
	ev := services.AdminEvent{
		Action:      action,
		ActorID:     new(actor.ID),
		TenantID:    nullableString(subjectTenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(subjectID),
		Success:     success,
		Request:     r,
		Reason:      req.Reason,
		Forced:      forced,
	}
	if errMsg != "" {
		ev.ErrMsg = new(errMsg)
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// logUnblockOutcome writes the admin.user_unblock audit row. Mirrors
// logBlockOutcome but with no force variant — unblock is symmetric.
func (api *adminUsersAPI) logUnblockOutcome(
	r *http.Request,
	actor *models.User,
	subjectID, subjectTenantID string,
	req AdminUnblockRequest,
	success bool,
	errMsg string,
) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      AuditActionAdminUserUnblock,
		ActorID:     new(actor.ID),
		TenantID:    nullableString(subjectTenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(subjectID),
		Success:     success,
		Request:     r,
		Reason:      req.Reason,
	}
	if errMsg != "" {
		ev.ErrMsg = new(errMsg)
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// writeUserEnvelope encodes a *models.User into the AdminUserEnvelope
// JSON:API shape and writes it on the response.
func (api *adminUsersAPI) writeUserEnvelope(w http.ResponseWriter, status int, u *models.User) {
	envelope := AdminUserEnvelope{
		Data: AdminUserResource{
			Type: "users",
			ID:   u.ID,
			Attributes: AdminUserView{
				ID:            u.ID,
				Email:         u.Email,
				Name:          u.Name,
				IsActive:      u.IsActive,
				IsSystemAdmin: u.IsSystemAdmin,
				TenantID:      u.TenantID,
			},
		},
	}
	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope); err != nil {
		// Headers (and the status) have already been flushed; we
		// cannot recover the response, but logging the encode failure
		// keeps the operator-side trail honest instead of silently
		// dropping it.
		slog.Error("admin user envelope: failed to encode response", "user_id", u.ID, "error", err)
	}
}

// auditSessionCountFailure records the secondary `admin.get_user_sessions`
// audit row when CountSessionsByUser fails. The primary
// `admin.get_user` row still reports Success: true because the user
// detail itself rendered correctly; this row exists so audit consumers
// can spot a pattern of session-count outages.
func (api *adminUsersAPI) auditSessionCountFailure(r *http.Request, userID, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	api.auditService.LogAdmin(r.Context(), services.AdminEvent{
		Action:      "admin.get_user_sessions",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(userID),
		Success:     false,
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	})
}

// auditListUsers records an `admin.list_tenant_users` audit row keyed
// to the target tenant.
func (api *adminUsersAPI) auditListUsers(r *http.Request, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      "admin.list_tenant_users",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("tenant"),
		SubjectID:   nullableString(tenantID),
		Success:     opErr == nil && !errors.Is(opErr, registry.ErrNotFound),
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// auditGetUser records an `admin.get_user` audit row keyed to the
// target user and (when resolved) the user's tenant.
func (api *adminUsersAPI) auditGetUser(r *http.Request, userID, tenantID string, opErr error) {
	if api.auditService == nil {
		return
	}
	ev := services.AdminEvent{
		Action:      "admin.get_user",
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(userID),
		Success:     opErr == nil && !errors.Is(opErr, registry.ErrNotFound),
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
	}
	api.auditService.LogAdmin(r.Context(), ev)
}

// nullableString returns nil for empty strings, &s otherwise. Used by
// the admin audit helpers where TenantID/SubjectID columns are
// nullable on the audit_logs schema and we want "missing" represented
// as SQL NULL rather than an empty string.
func nullableString(s string) *string {
	if s == "" {
		return nil
	}
	v := s
	return &v
}

func stringPtr(s string) *string {
	v := s
	return &v
}

func strPtrFromErr(err error) *string {
	if err == nil {
		return nil
	}
	msg := err.Error()
	return &msg
}
