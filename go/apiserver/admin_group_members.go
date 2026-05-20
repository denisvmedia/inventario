package apiserver

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// Admin group-membership audit action names (#1749). Kept as constants
// so the audit trail, the swagger annotations, and the tests reference
// the same literals. Mirrors the "admin.<noun>_<verb>" family set by
// #1745 (admin.grant_system_admin) and #1747 (admin.user_block).
const (
	// AuditActionAdminMemberAdd is the audit-row Action emitted when a
	// system admin adds a user to a group. Failed attempts (tenant
	// mismatch, cap hit, duplicate) reuse the same Action with
	// Success=false so a single filter pulls the whole attempt history.
	AuditActionAdminMemberAdd = "admin.group_member_add"
	// AuditActionAdminMemberRemove is the audit-row Action emitted when
	// a system admin removes a member from a group.
	AuditActionAdminMemberRemove = "admin.group_member_remove"
	// AuditActionAdminMemberRoleChange is the audit-row Action emitted
	// when a system admin changes a member's role.
	AuditActionAdminMemberRoleChange = "admin.group_member_role_change"
)

// JSON:API error codes returned by the admin group-membership
// endpoints. Kept as constants so the swagger annotations, the FE
// branch table, and the tests reference the same literals. Codes follow
// the "admin.member.*" family — distinct from the per-group "group.*"
// codes the non-admin members surface uses so the admin FE can branch
// without colliding.
const (
	// adminMemberTenantMismatchCode signals "the target user belongs to
	// a different tenant than the group". Maps to a 422. Referenced by
	// toJSONAPIError in errors.go.
	adminMemberTenantMismatchCode = "admin.member.tenant_mismatch"
	// adminMemberInvalidRoleCode signals "the request body's `role`
	// field is missing or not one of viewer|user|admin|owner". Maps to
	// a 422.
	adminMemberInvalidRoleCode = "admin.member.invalid_role"
	// adminMemberUserRequiredCode signals "the add-member body is
	// missing the required `userID` field". Maps to a 422.
	adminMemberUserRequiredCode = "admin.member.user_required"
)

// AdminAddMemberRequest is the request body for
// POST /admin/groups/{groupID}/members.
//
// `userID` is the user to admit; `role` is the role they are admitted
// with. Both are required — there is no invite-token flow here, an
// admin add is direct, so the role cannot be deferred to an acceptance
// step.
type AdminAddMemberRequest struct {
	// UserID is the ID of the user being added to the group.
	UserID string `json:"userID" validate:"required"`
	// Role is the group role granted to the user: viewer|user|admin|owner.
	Role models.GroupRole `json:"role" validate:"required"`
}

// AdminUpdateMemberRoleRequest is the request body for
// PATCH /admin/groups/{groupID}/members/{userID}. Only the role is
// mutable — moving a membership to a different group or user is a
// remove + add, not a patch.
type AdminUpdateMemberRoleRequest struct {
	// Role is the new group role: viewer|user|admin|owner.
	Role models.GroupRole `json:"role" validate:"required"`
}

// AdminMemberView is the JSON:API-style attributes block returned by
// the add / role-change endpoints. Narrow on purpose: the membership
// identity (group, user, role) plus the synthetic joined_at. Richer
// member detail (avatar / email) lives on the per-group members surface.
type AdminMemberView struct {
	GroupID      string           `json:"group_id"`
	MemberUserID string           `json:"member_user_id"`
	Role         models.GroupRole `json:"role"`
	TenantID     string           `json:"tenant_id"`
	JoinedAt     string           `json:"joined_at"`
}

// AdminMemberEnvelope is the JSON:API envelope returned by add /
// role-change. Single-resource shape ({"data": {...}}) matches the rest
// of the admin surface (AdminUserEnvelope).
type AdminMemberEnvelope struct {
	Data AdminMemberResource `json:"data"`
}

// AdminMemberResource is the JSON:API resource block carried inside
// AdminMemberEnvelope.
type AdminMemberResource struct {
	Type       string          `json:"type"`
	ID         string          `json:"id"`
	Attributes AdminMemberView `json:"attributes"`
}

// adminGroupMembersAPI backs the /admin/groups/{groupID}/members
// routes. Holds the FactorySet directly for the cross-tenant lookups
// the add path needs (resolve group + user from any tenant) and the
// GroupService so the membership writes route through the same
// invariant-bearing methods the per-group members surface uses.
type adminGroupMembersAPI struct {
	factorySet   *registry.FactorySet
	groupService *services.GroupService
	auditService services.AuditLogger
}

// addMember admits a user to a group on behalf of a system
// administrator, bypassing the per-group requireGroupAdmin middleware
// but still enforcing the per-user membership cap (the registry's
// atomic CreateUnderCap path) and the cross-tenant safety check.
//
// @Summary Add a member to a group (admin)
// @Description Adds a user to a group with the given role, bypassing the per-group admin check. The membership cap is still enforced — a user already at the cap is rejected with 422.
// @Description Returns 422 with `admin.member.tenant_mismatch` when the target user's tenant differs from the group's tenant, `admin.member.invalid_role` for a missing or invalid role, and `admin.member.user_required` for a missing userID.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param data body AdminAddMemberRequest true "Add-member request"
// @Success 201 {object} AdminMemberEnvelope "Created"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Not Found - unknown group or user"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - tenant mismatch, cap reached, duplicate, or invalid role"
// @Router /admin/groups/{groupID}/members [post]
func (api *adminGroupMembersAPI) addMember(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	if strings.TrimSpace(groupID) == "" {
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	req, ok := api.decodeAddMemberRequest(w, r)
	if !ok {
		return
	}

	// Resolve the group first so a typo in the URL surfaces as 404
	// rather than a confusing tenant-mismatch or membership error.
	group, err := api.factorySet.LocationGroupRegistry.Get(r.Context(), groupID)
	if err != nil {
		// The group lookup failed, so its tenant is unknown — pass an
		// empty tenant for the failed-add audit row but keep the
		// already-validated target user ID so the row records who the
		// admin tried to add.
		api.auditAdd(r, "", groupID, req.UserID, req.Role, err)
		_ = renderEntityError(w, r, err)
		return
	}

	// Resolve the target user across tenants — the admin caller is not
	// scoped to the group's tenant, so the user may live anywhere.
	target, err := api.factorySet.UserRegistry.Get(r.Context(), req.UserID)
	if err != nil {
		api.auditAdd(r, group.TenantID, groupID, req.UserID, req.Role, err)
		_ = renderEntityError(w, r, err)
		return
	}

	membership, err := api.groupService.AdminAddMember(
		r.Context(), group.ID, group.TenantID, target.ID, target.TenantID, req.Role,
	)
	if err != nil {
		api.auditAdd(r, group.TenantID, group.ID, target.ID, req.Role, err)
		_ = renderEntityError(w, r, err)
		return
	}

	api.auditAdd(r, group.TenantID, group.ID, target.ID, req.Role, nil)
	api.writeMemberEnvelope(w, http.StatusCreated, membership)
}

// removeMember removes a member from a group on behalf of a system
// administrator. Wraps GroupService.RemoveMember, so the ≥1-owner and
// ≥1-member invariants (ErrLastOwner / ErrLastMember) are enforced
// unchanged — the same business rule the per-group leave path obeys.
//
// @Summary Remove a member from a group (admin)
// @Description Removes a user from a group, bypassing the per-group admin check. The ≥1-owner and ≥1-member invariants still apply — removing the last owner returns 422, removing the last member returns 422 with `group.last_member`.
// @Tags admin
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param userID path string true "User ID of the member to remove"
// @Success 204 "No Content"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Not Found - user is not a member of the group"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - removing the last owner or last member"
// @Router /admin/groups/{groupID}/members/{userID} [delete]
func (api *adminGroupMembersAPI) removeMember(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	userID := chi.URLParam(r, "userID")
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(userID) == "" {
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	// Resolve the group up front so the audit row carries the group's
	// tenant ID and a typo in the URL surfaces as a clean 404.
	group, err := api.factorySet.LocationGroupRegistry.Get(r.Context(), groupID)
	if err != nil {
		api.auditRemove(r, "", groupID, userID, err)
		_ = renderEntityError(w, r, err)
		return
	}

	if err := api.groupService.RemoveMember(r.Context(), group.ID, userID); err != nil {
		api.auditRemove(r, group.TenantID, group.ID, userID, err)
		// ErrNotGroupMember means "no such membership" — surface it as a
		// 404 so the admin FE renders a "not a member" message rather
		// than a generic 500. Other sentinels (ErrLastOwner /
		// ErrLastMember) flow through toJSONAPIError as 422.
		if errors.Is(err, services.ErrNotGroupMember) {
			_ = renderEntityError(w, r, registry.ErrNotFound)
			return
		}
		_ = renderEntityError(w, r, err)
		return
	}

	api.auditRemove(r, group.TenantID, group.ID, userID, nil)
	w.WriteHeader(http.StatusNoContent)
}

// updateMemberRole changes a member's role on behalf of a system
// administrator. Wraps GroupService.UpdateMemberRole, so the ≥1-owner
// invariant (ErrLastOwner — demoting the sole owner) is enforced under
// the same per-group lock the per-group role-change path uses.
//
// @Summary Change a member's role (admin)
// @Description Changes a group member's role, bypassing the per-group admin check. Demoting the sole owner is rejected with 422 (`ErrLastOwner`). Returns 422 with `admin.member.invalid_role` for a missing or invalid role.
// @Tags admin
// @Accept json
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param userID path string true "User ID of the member"
// @Param data body AdminUpdateMemberRoleRequest true "Role-change request"
// @Success 200 {object} AdminMemberEnvelope "OK"
// @Failure 400 {object} jsonapi.Errors "Bad Request - invalid body"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 403 {object} jsonapi.Errors "Forbidden - system-admin required"
// @Failure 404 {object} jsonapi.Errors "Not Found - user is not a member of the group"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity - demoting the last owner or invalid role"
// @Router /admin/groups/{groupID}/members/{userID} [patch]
func (api *adminGroupMembersAPI) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	groupID := chi.URLParam(r, "groupID")
	userID := chi.URLParam(r, "userID")
	if strings.TrimSpace(groupID) == "" || strings.TrimSpace(userID) == "" {
		_ = renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	req, ok := api.decodeUpdateRoleRequest(w, r)
	if !ok {
		return
	}

	// Resolve the group up front so the audit row carries the group's
	// tenant ID and a typo in the URL surfaces as a clean 404.
	group, err := api.factorySet.LocationGroupRegistry.Get(r.Context(), groupID)
	if err != nil {
		api.auditRoleChange(r, "", groupID, userID, req.Role, err)
		_ = renderEntityError(w, r, err)
		return
	}

	membership, err := api.groupService.UpdateMemberRole(r.Context(), group.ID, userID, req.Role)
	if err != nil {
		api.auditRoleChange(r, group.TenantID, group.ID, userID, req.Role, err)
		if errors.Is(err, services.ErrNotGroupMember) {
			_ = renderEntityError(w, r, registry.ErrNotFound)
			return
		}
		_ = renderEntityError(w, r, err)
		return
	}

	api.auditRoleChange(r, group.TenantID, group.ID, userID, req.Role, nil)
	api.writeMemberEnvelope(w, http.StatusOK, membership)
}

// decodeAddMemberRequest parses + validates the POST body. Writes the
// right error response and returns ok=false on failure so the caller
// can early-return without decode-error noise.
func (api *adminGroupMembersAPI) decodeAddMemberRequest(w http.ResponseWriter, r *http.Request) (AdminAddMemberRequest, bool) {
	var req AdminAddMemberRequest
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
	if !decoderAtEOF(dec) {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return req, false
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if req.UserID == "" {
		_ = codedUnprocessableEntityError(w, r, errors.New("userID is required"), adminMemberUserRequiredCode)
		return req, false
	}
	if err := req.Role.Validate(); err != nil {
		_ = codedUnprocessableEntityError(w, r, err, adminMemberInvalidRoleCode)
		return req, false
	}
	return req, true
}

// decodeUpdateRoleRequest mirrors decodeAddMemberRequest for the PATCH
// shape. Kept separate so the two swagger DTOs stay narrow and a future
// expansion of either body doesn't entangle them.
func (api *adminGroupMembersAPI) decodeUpdateRoleRequest(w http.ResponseWriter, r *http.Request) (AdminUpdateMemberRoleRequest, bool) {
	var req AdminUpdateMemberRoleRequest
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
	if !decoderAtEOF(dec) {
		_ = badRequest(w, r, errors.New("invalid JSON body — trailing tokens"))
		return req, false
	}
	if err := req.Role.Validate(); err != nil {
		_ = codedUnprocessableEntityError(w, r, err, adminMemberInvalidRoleCode)
		return req, false
	}
	return req, true
}

// writeMemberEnvelope encodes a *models.GroupMembership into the
// AdminMemberEnvelope JSON:API shape and writes it on the response.
func (api *adminGroupMembersAPI) writeMemberEnvelope(w http.ResponseWriter, status int, m *models.GroupMembership) {
	envelope := AdminMemberEnvelope{
		Data: AdminMemberResource{
			Type: "group_memberships",
			ID:   m.ID,
			Attributes: AdminMemberView{
				GroupID:      m.GroupID,
				MemberUserID: m.MemberUserID,
				Role:         m.Role,
				TenantID:     m.TenantID,
				JoinedAt:     m.JoinedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
			},
		},
	}
	w.Header().Set("Content-Type", "application/vnd.api+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope); err != nil {
		// Headers (and the status) have already been flushed; logging
		// the encode failure keeps the operator-side trail honest
		// instead of silently dropping it.
		slog.Error("admin member envelope: failed to encode response", "membership_id", m.ID, "error", err)
	}
}

// auditAdd records an admin.group_member_add audit row. The group's
// tenant is the audit TenantID; the subject is the membership target
// user. Best-effort — nil-safe for the case where AuditService was not
// wired in. `tenantID` is the group's tenant ("" when the group lookup
// failed and the tenant is therefore unknown).
func (api *adminGroupMembersAPI) auditAdd(r *http.Request, tenantID, groupID, userID string, role models.GroupRole, opErr error) {
	api.logMemberEvent(r, AuditActionAdminMemberAdd, tenantID, groupID, userID, role, opErr)
}

// auditRemove records an admin.group_member_remove audit row.
func (api *adminGroupMembersAPI) auditRemove(r *http.Request, tenantID, groupID, userID string, opErr error) {
	api.logMemberEvent(r, AuditActionAdminMemberRemove, tenantID, groupID, userID, "", opErr)
}

// auditRoleChange records an admin.group_member_role_change audit row.
func (api *adminGroupMembersAPI) auditRoleChange(r *http.Request, tenantID, groupID, userID string, role models.GroupRole, opErr error) {
	api.logMemberEvent(r, AuditActionAdminMemberRoleChange, tenantID, groupID, userID, role, opErr)
}

// logMemberEvent is the shared audit-row writer for the three
// group-membership admin actions. The subject is the target user; the
// group's tenant lands in the audit TenantID column; the group ID and
// (when present) the role land in the Extra breadcrumb so audit
// consumers can correlate "who changed whose membership in which group"
// without re-parsing free text.
func (api *adminGroupMembersAPI) logMemberEvent(
	r *http.Request,
	action, tenantID, groupID, userID string,
	role models.GroupRole,
	opErr error,
) {
	if api.auditService == nil {
		return
	}
	extra := map[string]any{"group_id": groupID}
	if role != "" {
		extra["role"] = string(role)
	}
	ev := services.AdminEvent{
		Action:      action,
		ActorID:     actorIDFromRequest(r),
		TenantID:    nullableString(tenantID),
		SubjectType: stringPtr("user"),
		SubjectID:   nullableString(userID),
		Success:     opErr == nil,
		Request:     r,
		ErrMsg:      strPtrFromErr(opErr),
		Extra:       extra,
	}
	api.auditService.LogAdmin(r.Context(), ev)
}
