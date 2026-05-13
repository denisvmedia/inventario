package apiserver

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

const groupCtxKey ctxValueKey = "group"

// createInviteMaxBodyBytes caps the JSON envelope on POST /invites.
// The body only ever carries `{email, role}`; 4 KiB is comfortably
// above the realistic ceiling (255-char email + role + envelope
// overhead) and small enough that a malicious caller can't DoS the
// handler by streaming a multi-megabyte payload into io.ReadAll.
const createInviteMaxBodyBytes int64 = 4 * 1024

type groupsAPI struct {
	groupService *services.GroupService
	auditService services.AuditLogger
	// emailService is used by createInvite / resendInvite to dispatch
	// the email-flow invitation message. Nil-safe: the handlers skip
	// the send when not set, which keeps targeted unit tests and the
	// invite-info / accept-invite public paths working without wiring
	// EmailService through every test factory.
	emailService services.EmailService
	// publicBaseURL is the operator-configured external origin used to
	// build the absolute /invite/{token} URL in the dispatched email.
	// When empty, the handler falls back to the request's
	// scheme + host (which honours X-Forwarded-* and so requires the
	// deployment to terminate spoofable proxy headers upstream). Same
	// source-of-truth password-reset / verification emails use.
	publicBaseURL string
}

func groupFromContext(ctx context.Context) *models.LocationGroup {
	group, ok := ctx.Value(groupCtxKey).(*models.LocationGroup)
	if !ok {
		return nil
	}
	return group
}

// GroupSlugResolverMiddleware resolves a group slug from the URL path ({groupSlug}),
// verifies the authenticated user is a member, and places the group in the request context.
// Used on all /api/v1/g/{groupSlug}/... routes.
func GroupSlugResolverMiddleware(groupService *services.GroupService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "groupSlug")
			if slug == "" {
				http.Error(w, "Group slug is required", http.StatusBadRequest)
				return
			}

			user := GetUserFromRequest(r)
			if user == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			group, err := groupService.GetGroupBySlug(r.Context(), user.TenantID, slug)
			if err != nil {
				// Distinguish a legitimately missing group (→ 404) from an
				// unexpected infrastructure error (→ 500). Masking the latter
				// as 404 hides incidents and can make a broken dependency look
				// like broken client input.
				if errors.Is(err, registry.ErrNotFound) {
					http.Error(w, "Group not found", http.StatusNotFound)
					return
				}
				slog.Error("GroupSlugResolverMiddleware: GetGroupBySlug failed", "slug", slug, "tenant_id", user.TenantID, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}

			if !group.IsActive() {
				http.Error(w, "Group is not available", http.StatusGone)
				return
			}

			isMember, err := groupService.CheckGroupMembership(r.Context(), group.ID, user.ID)
			if err != nil {
				slog.Error("GroupSlugResolverMiddleware: CheckGroupMembership failed", "group_id", group.ID, "user_id", user.ID, "error", err)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if !isMember {
				http.Error(w, "Group membership required", http.StatusForbidden)
				return
			}

			// Store the group in both the apiserver-local key (used by
			// group handlers) and the appctx key (read by registry factories
			// at RegistrySetMiddleware time to wire group_id into transactions).
			ctx := context.WithValue(r.Context(), groupCtxKey, group)
			ctx = appctx.WithGroup(ctx, group)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// groupCtx middleware loads a group by its ID from the URL parameter.
// The backing registry is tenant-scoped but NOT RLS-filtered, so a raw ID
// from the URL could otherwise resolve a group belonging to a different
// tenant. Reject that up front with 404 — "exists in another tenant" and
// "does not exist" are indistinguishable to the caller by design.
func groupCtx(groupService *services.GroupService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			groupID := chi.URLParam(r, "groupID")
			if groupID == "" {
				unprocessableEntityError(w, r, nil)
				return
			}

			user := GetUserFromRequest(r)
			if user == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			group, err := groupService.GetGroup(r.Context(), groupID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}

			if group.TenantID != user.TenantID {
				renderEntityError(w, r, registry.ErrNotFound)
				return
			}

			ctx := context.WithValue(r.Context(), groupCtxKey, group)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// requireGroupMember middleware ensures the current user is a member of the group in context.
func requireGroupMember(groupService *services.GroupService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			group := groupFromContext(r.Context())
			if group == nil {
				http.Error(w, "Group context required", http.StatusInternalServerError)
				return
			}

			user := GetUserFromRequest(r)
			if user == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			if !groupService.IsGroupMember(r.Context(), group.ID, user.ID) {
				http.Error(w, "Group membership required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// requireGroupAdmin middleware ensures the current user has admin
// privileges in the group in context. After the role-taxonomy expansion
// of #1533, "admin" means role >= admin (admin or owner). For
// owner-specific gates (delete-group) use requireGroupOwner instead.
func requireGroupAdmin(groupService *services.GroupService) func(http.Handler) http.Handler {
	return requireGroupRole(groupService, models.GroupRoleAdmin)
}

// requireGroupOwner middleware ensures the current user is the owner
// of the group in context. Reserved for delete-group; other admin-tier
// mutations stay admin+ so co-admins can run them without owner
// intervention.
func requireGroupOwner(groupService *services.GroupService) func(http.Handler) http.Handler {
	return requireGroupRole(groupService, models.GroupRoleOwner)
}

// requireGroupRole returns a middleware that ensures the current user
// has role >= minRole in the group in context. Failures map to:
//
//   - 500 when the group context is missing (handler-wiring bug) or
//     the membership lookup itself fails with a registry / infra
//     error — that mirrors GroupSlugResolverMiddleware so DB outages
//     surface as 5xx instead of being silently hidden behind a 403.
//   - 401 when the request is unauthenticated.
//   - 403 only when authentication is fine but the caller is not a
//     member of the group, or is a member with role < minRole. The
//     body carries a minRole-specific hint so the client can render
//     an appropriate error message.
func requireGroupRole(groupService *services.GroupService, minRole models.GroupRole) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			group := groupFromContext(r.Context())
			if group == nil {
				http.Error(w, "Group context required", http.StatusInternalServerError)
				return
			}

			user := GetUserFromRequest(r)
			if user == nil {
				http.Error(w, "Authentication required", http.StatusUnauthorized)
				return
			}

			ok, _, err := groupService.HasRoleAtLeast(r.Context(), group.ID, user.ID, minRole)
			if err != nil {
				// Treat infrastructure failures as 500 — same posture as
				// GroupSlugResolverMiddleware. Masking a DB outage as a
				// 403 hides incidents and confuses oncall debugging.
				slog.Error("requireGroupRole: role lookup failed",
					"group_id", group.ID,
					"user_id", user.ID,
					"min_role", minRole,
					"error", err,
				)
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			if !ok {
				http.Error(w, forbiddenMessageForRole(minRole), http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// requireGroupRoleForWrite gates non-GET requests to a route subtree
// behind requireGroupRole(minRole). GET / HEAD / OPTIONS fall through
// to `next` unchanged — reads remain available to any group member
// (handled by the GroupSlugResolverMiddleware membership check
// upstream). Used at /api/v1/g/{groupSlug}/* mounts so the resource
// router files (locations.go, commodities.go, …) don't have to
// duplicate the gating per HTTP method.
func requireGroupRoleForWrite(groupService *services.GroupService, minRole models.GroupRole) func(http.Handler) http.Handler {
	role := requireGroupRole(groupService, minRole)
	return func(next http.Handler) http.Handler {
		gated := role(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions:
				next.ServeHTTP(w, r)
			default:
				gated.ServeHTTP(w, r)
			}
		})
	}
}

func forbiddenMessageForRole(minRole models.GroupRole) string {
	switch minRole {
	case models.GroupRoleOwner:
		return "Group owner access required"
	case models.GroupRoleAdmin:
		return "Group admin access required"
	case models.GroupRoleUser:
		return "Group write access required"
	case models.GroupRoleViewer:
		return "Group membership required"
	default:
		return "Insufficient group role"
	}
}

// Groups returns the route handler for group management endpoints.
func Groups(params Params, groupService *services.GroupService, auditService services.AuditLogger) func(r chi.Router) {
	api := &groupsAPI{
		groupService:  groupService,
		auditService:  auditService,
		emailService:  params.EmailService,
		publicBaseURL: strings.TrimSpace(params.PublicURL),
	}
	return func(r chi.Router) {
		r.Get("/", api.listGroups)
		r.Post("/", api.createGroup)

		r.Route("/{groupID}", func(r chi.Router) {
			r.Use(groupCtx(groupService))
			r.Use(requireGroupMember(groupService))

			r.Get("/", api.getGroup)
			r.Get("/members", api.listMembers)
			r.Post("/leave", api.leaveGroup)

			// Admin-or-owner operations: member management, invites,
			// group rename. Admins can run these without owner sign-off
			// so multi-admin setups don't bottleneck on one person.
			r.Group(func(r chi.Router) {
				r.Use(requireGroupAdmin(groupService))
				r.Patch("/", api.updateGroup)
				r.Delete("/members/{memberUserID}", api.removeMember)
				r.Patch("/members/{memberUserID}", api.updateMemberRole)
				r.Post("/invites", api.createInvite)
				r.Get("/invites", api.listInvites)
				r.Delete("/invites/{inviteID}", api.revokeInvite)
				r.Post("/invites/{inviteID}/resend", api.resendInvite)
			})

			// Owner-only operations: deleting the group is the one
			// action with no recovery, so it stays scoped to owners.
			r.Group(func(r chi.Router) {
				r.Use(requireGroupOwner(groupService))
				r.Delete("/", api.deleteGroup)
			})
		})
	}
}

// Invites returns the route handler for invite info and acceptance.
// GET /{token} is public (the invitee is typically unauthenticated at first);
// POST /{token}/accept requires authentication and is wrapped with the caller's
// user-aware middleware chain so the request has the user / CSRF / RLS context
// the acceptInvite handler relies on.
func Invites(groupService *services.GroupService, authMiddlewares []func(http.Handler) http.Handler) func(r chi.Router) {
	api := &groupsAPI{
		groupService: groupService,
	}
	return func(r chi.Router) {
		r.Get("/{token}", api.getInviteInfo)
		r.With(authMiddlewares...).Post("/{token}/accept", api.acceptInvite)
	}
}

// listGroups lists all groups the current user belongs to.
// @Summary List user's groups
// @Description Returns all active location groups the authenticated user is a member of
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.LocationGroupsResponse "OK"
// @Failure 401 {string} string "Unauthorized"
// @Router /groups [get].
func (api *groupsAPI) listGroups(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	groups, err := api.groupService.ListUserGroups(r.Context(), user.TenantID, user.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Populate LocationGroup.MembersCount with one aggregate round-trip
	// so the sidebar GroupSelector and any other client can render
	// `N member(s)` without fetching the full members list per group
	// (issue #1650). Failures here are not fatal — the wider /groups
	// response is still useful — but logging via internalServerError
	// would mask the primary list; keep it strict (5xx) so a regression
	// in the count path doesn't silently ship empty counts.
	if err := api.groupService.AttachMembersCounts(r.Context(), groups); err != nil {
		internalServerError(w, r, err)
		return
	}

	total := len(groups)
	resp := jsonapi.NewLocationGroupsResponse(groups, total, 1, total)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createGroup creates a new location group.
// @Summary Create a group
// @Description Creates a new location group with a random non-guessable slug. The creator becomes the group admin.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param data body jsonapi.LocationGroupRequest true "Group data"
// @Success 201 {object} jsonapi.LocationGroupResponse "Created"
// @Failure 401 {string} string "Unauthorized"
// @Failure 422 {object} jsonapi.Errors "Validation error"
// @Router /groups [post].
func (api *groupsAPI) createGroup(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var input jsonapi.LocationGroupRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	attrs := input.Data.Attributes
	var groupCurrency models.Currency
	if attrs.GroupCurrency != nil {
		if !attrs.GroupCurrency.IsValid() {
			badRequest(w, r, fmt.Errorf("invalid group_currency: %q", *attrs.GroupCurrency))
			return
		}
		groupCurrency = *attrs.GroupCurrency
	}

	group, err := api.groupService.CreateGroup(r.Context(), user.TenantID, user.ID, attrs.Name, attrs.Icon, groupCurrency)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewLocationGroupResponse(group).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getGroup returns a single group's details.
// @Summary Get group details
// @Description Returns details of a location group. Requires group membership.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.LocationGroupResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group member"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Router /groups/{groupID} [get].
func (api *groupsAPI) getGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	// Populate LocationGroup.MembersCount via an aggregate query so the
	// detail response carries the same field as the list response
	// (issue #1650). The group came from groupCtx, which loaded it
	// without the count — enrich here right before render.
	if err := api.groupService.AttachMembersCount(r.Context(), group); err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := jsonapi.NewLocationGroupResponse(group)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateGroup updates a group's metadata.
// @Summary Update group
// @Description Updates a location group's name and icon. The group_currency field is set once at creation and cannot be changed here — see issue #202 for the currency-migration tool. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param data body jsonapi.LocationGroupRequest true "Group data"
// @Success 200 {object} jsonapi.LocationGroupResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Failure 422 {object} jsonapi.Errors "Validation error"
// @Router /groups/{groupID} [patch].
func (api *groupsAPI) updateGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.LocationGroupRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	attrs := input.Data.Attributes

	// group_currency is immutable after creation. A fully-featured
	// currency-migration tool (including a reprice of the group's
	// commodities) is tracked under #202; until it lands, reject
	// change attempts loudly instead of silently dropping them.
	if attrs.GroupCurrency != nil && *attrs.GroupCurrency != group.GroupCurrency {
		unprocessableEntityError(w, r, errors.New("group_currency is immutable after group creation (see #202)"))
		return
	}

	updated, err := api.groupService.UpdateGroup(r.Context(), group.ID, attrs.Name, attrs.Icon)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewLocationGroupResponse(updated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteGroup initiates async group deletion.
// @Summary Delete group
// @Description Initiates async deletion of a location group. Requires typing the group name AND the caller's current password. Requires group admin role. Failed attempts are audit-logged.
// @Tags groups
// @Accept json
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param data body jsonapi.GroupDeleteRequest true "Deletion confirmation (confirm_word + password)"
// @Success 204 "No Content"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Failure 422 {object} jsonapi.Errors "Invalid confirmation word or password"
// @Router /groups/{groupID} [delete].
func (api *groupsAPI) deleteGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	user := GetUserFromRequest(r)
	if user == nil {
		unauthorizedError(w, r, nil)
		return
	}

	var input jsonapi.GroupDeleteRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Verify the caller's password before touching the group. Distinct
	// from the confirm-word check so the frontend can show a specific
	// "wrong password" error (see spec #1219 §12). Every attempt is
	// audit-logged — a failed delete-group is worth tracing because it
	// signals either fumbled input or a hijacked session.
	if !user.CheckPassword(input.Password) {
		api.logGroupDeletion(r, user, group, false, "invalid_password")
		renderEntityError(w, r, services.ErrInvalidPassword)
		return
	}

	if err := api.groupService.InitiateGroupDeletion(r.Context(), group.ID, input.ConfirmWord, group.Name); err != nil {
		reason := "service_error"
		if errors.Is(err, services.ErrInvalidConfirmation) {
			reason = "invalid_confirmation"
		}
		api.logGroupDeletion(r, user, group, false, reason)
		renderEntityError(w, r, err)
		return
	}

	api.logGroupDeletion(r, user, group, true, "")
	w.WriteHeader(http.StatusNoContent)
}

// logGroupDeletion is a nil-safe audit log helper for group deletion
// attempts. Action name "group_delete" is used consistently across success
// and failure so an oncall can grep one string to find every attempt on a
// given group.
func (api *groupsAPI) logGroupDeletion(r *http.Request, user *models.User, group *models.LocationGroup, success bool, errReason string) {
	if api.auditService == nil {
		return
	}
	var errPtr *string
	if errReason != "" {
		msg := errReason + " on group " + group.ID
		errPtr = &msg
	}
	tenantID := user.TenantID
	api.auditService.LogAuth(r.Context(), "group_delete", &user.ID, &tenantID, success, r, errPtr)
}

// listMembers lists all members of a group with their resolved user data.
// @Summary List group members
// @Description Returns all members of a location group with their roles and joined user data (id / name / email). Requires group membership.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.MembershipsWithUsersResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group member"
// @Router /groups/{groupID}/members [get].
func (api *groupsAPI) listMembers(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	rows, err := api.groupService.ListMembersWithUsers(r.Context(), group.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := jsonapi.NewMembershipsWithUsersResponse(rows)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// removeMember removes a member from a group.
// @Summary Remove group member
// @Description Removes a user from a location group. Cannot remove the last admin. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param memberUserID path string true "User ID of the member to remove"
// @Success 204 "No Content"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 422 {object} jsonapi.Errors "Cannot remove the last admin"
// @Router /groups/{groupID}/members/{memberUserID} [delete].
func (api *groupsAPI) removeMember(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	memberUserID := chi.URLParam(r, "memberUserID")
	if memberUserID == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	err := api.groupService.RemoveMember(r.Context(), group.ID, memberUserID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// updateMemberRole changes a member's role within a group.
// @Summary Update member role
// @Description Changes a member's role (admin/user) within a location group. Cannot demote the last admin. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param memberUserID path string true "User ID of the member"
// @Param data body jsonapi.GroupMembershipRoleRequest true "New role"
// @Success 200 {object} jsonapi.GroupMembershipResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 422 {object} jsonapi.Errors "Cannot demote the last admin"
// @Router /groups/{groupID}/members/{memberUserID} [patch].
func (api *groupsAPI) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	memberUserID := chi.URLParam(r, "memberUserID")
	if memberUserID == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.GroupMembershipRoleRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	membership, err := api.groupService.UpdateMemberRole(r.Context(), group.ID, memberUserID, input.Data.Attributes.Role)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewGroupMembershipResponse(membership)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// leaveGroup allows the current user to leave a group.
// @Summary Leave group
// @Description Removes the current user from a location group. Cannot leave if you are the last admin.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Forbidden - not a group member"
// @Failure 422 {object} jsonapi.Errors "Cannot leave as the last admin"
// @Router /groups/{groupID}/leave [post].
func (api *groupsAPI) leaveGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	err := api.groupService.LeaveGroup(r.Context(), group.ID, user.ID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// createInvite generates an invite link for a group, optionally sending
// the invitation by email.
// @Summary Create invite
// @Description Creates an invite. When the request body carries `email`, the BE persists invitee_email on
// @Description the row and dispatches an email via EmailService; when empty, the invite remains a copy-paste
// @Description token. `role` (viewer / user / admin) defaults to "user"; owner-by-invite is rejected — owner
// @Description is a transfer-of-ownership operation. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param data body jsonapi.GroupInviteCreateRequest false "Optional invitee email + role"
// @Success 201 {object} jsonapi.GroupInviteResponse "Created"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 422 {object} jsonapi.Errors "Invalid email / role"
// @Router /groups/{groupID}/invites [post].
func (api *groupsAPI) createInvite(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	// The body is optional. An empty / missing body is the legacy
	// token-only flow with the default user role. Read once via
	// MaxBytesReader (so chunked requests where ContentLength == -1
	// still work but an oversized payload is rejected with 413), then
	// only Unmarshal + Bind when there's something to parse — that way
	// a deliberate `{}` body, a chunked body, and a truly empty body
	// all behave identically.
	var input jsonapi.GroupInviteCreateRequest
	r.Body = http.MaxBytesReader(w, r.Body, createInviteMaxBodyBytes)
	bodyBytes, readErr := io.ReadAll(r.Body)
	if readErr != nil {
		// MaxBytesReader returns *http.MaxBytesError on overflow — map
		// that to 413; anything else (transient network read failure)
		// stays a 422 body-parse error.
		var maxErr *http.MaxBytesError
		if errors.As(readErr, &maxErr) {
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		unprocessableEntityError(w, r, readErr)
		return
	}
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		if err := json.Unmarshal(bodyBytes, &input); err != nil {
			unprocessableEntityError(w, r, err)
			return
		}
		if err := input.Bind(r); err != nil {
			unprocessableEntityError(w, r, err)
			return
		}
	}

	var (
		email *string
		role  models.GroupRole
	)
	if input.Data != nil && input.Data.Attributes != nil {
		role = input.Data.Attributes.Role
		if input.Data.Attributes.Email != "" {
			e := input.Data.Attributes.Email
			email = &e
		}
	}

	invite, err := api.groupService.CreateInviteWithEmail(r.Context(), user.TenantID, group.ID, user.ID, role, email, 0)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Dispatch the invitation email when the caller passed an
	// invitee_email. The send is best-effort — failures are logged
	// but do NOT roll back the invite, matching the fire-and-forget
	// pattern used by password-reset / verification flows. Admins
	// can still copy the token URL from the response and share it
	// out-of-band if delivery is suspect. Build the absolute URL on
	// the request goroutine so the email points at the same origin
	// the admin is currently using.
	if email != nil && api.emailService != nil {
		inviteURL := api.buildInviteURL(r, invite.Token)
		go api.sendInviteEmailBestEffort(r.Context(), invite, group, user, *email, inviteURL)
	}

	resp := jsonapi.NewGroupInviteResponse(invite).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// resendInvite refreshes an email-flow invite's token + expiry and
// dispatches a fresh email to the captured invitee_email.
// @Summary Resend invite
// @Description Mints a new token and expiry on an email-flow invite, then dispatches a fresh email. Legacy token-only invites cannot be resent — create a new invite or recopy the URL. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param inviteID path string true "Invite ID"
// @Success 200 {object} jsonapi.GroupInviteResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 404 {object} jsonapi.Errors "Invite not found"
// @Failure 422 {object} jsonapi.Errors "Invite already used, belongs to another group, or has no invitee email"
// @Router /groups/{groupID}/invites/{inviteID}/resend [post].
func (api *groupsAPI) resendInvite(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	inviteID := chi.URLParam(r, "inviteID")
	if inviteID == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	invite, err := api.groupService.ResendInvite(r.Context(), group.ID, inviteID, 0)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if invite.InviteeEmail != nil && api.emailService != nil {
		inviteURL := api.buildInviteURL(r, invite.Token)
		go api.sendInviteEmailBestEffort(r.Context(), invite, group, user, *invite.InviteeEmail, inviteURL)
	}

	resp := jsonapi.NewGroupInviteResponse(invite)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// buildInviteURL constructs the absolute /invite/{token} URL the
// recipient will receive in the invitation email. When the operator
// has configured `params.PublicURL`, that origin is the canonical
// source — same path password-reset / verification emails take. The
// request-derived fallback (TLS + r.Host + X-Forwarded-*) only kicks
// in when no PublicURL is set; deployments that haven't configured one
// must terminate spoofable proxy headers upstream of this handler.
// Must be called on the request goroutine; *http.Request isn't safe
// to share with the detached email-send goroutine.
func (api *groupsAPI) buildInviteURL(r *http.Request, token string) string {
	if api.publicBaseURL != "" {
		built, err := buildPublicURL(api.publicBaseURL, "/invite/"+url.PathEscape(token), nil)
		if err == nil {
			return built
		}
		// buildPublicURL already logs the misconfiguration. Fall
		// through to the request-derived path so a bad operator config
		// doesn't break the feature outright.
		slog.Warn("Falling back to request-derived invite URL due to invalid public_url",
			"public_url", api.publicBaseURL,
			"error", err,
		)
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	if fwd := strings.ToLower(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))); fwd != "" {
		// Pick the first proto when the header lists multiple hops.
		if i := strings.Index(fwd, ","); i >= 0 {
			fwd = strings.TrimSpace(fwd[:i])
		}
		if fwd == "http" || fwd == "https" {
			scheme = fwd
		}
	}
	host := r.Host
	if fwdHost := strings.TrimSpace(r.Header.Get("X-Forwarded-Host")); fwdHost != "" {
		if i := strings.Index(fwdHost, ","); i >= 0 {
			fwdHost = strings.TrimSpace(fwdHost[:i])
		}
		host = fwdHost
	}
	return fmt.Sprintf("%s://%s/invite/%s", scheme, host, url.PathEscape(token))
}

// sendInviteEmailBestEffort dispatches the group-invite email via the
// existing async EmailService. The send is async-best-effort: we log
// failures (so they're observable in ops) but never propagate them to
// the caller — the invite is already persisted, and the admin can
// always resend or copy the link manually. The caller must build
// inviteURL with api.buildInviteURL on the request goroutine; passing
// it in avoids smuggling *http.Request into the detached goroutine.
// A bounded WithTimeout wraps the enqueue so a hung queue backend
// can't leak goroutines, mirroring the password-reset / verification
// detached-send pattern.
func (api *groupsAPI) sendInviteEmailBestEffort(ctx context.Context, invite *models.GroupInvite, group *models.LocationGroup, inviter *models.User, to, inviteURL string) {
	if invite == nil || api.emailService == nil {
		return
	}
	// Detached context preserves request-scoped values (tenant, RLS
	// hints) without inheriting the request's cancellation — the email
	// dispatch must outlive the HTTP response. The explicit timeout
	// then caps the goroutine's lifetime so we don't leak forever if
	// the queue backend hangs.
	bgCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), detachedAuthEmailTimeout)
	defer cancel()

	if err := api.emailService.SendGroupInviteEmail(
		bgCtx,
		to,
		inviter.Name,
		group.Name,
		invite.Role.Label(),
		inviteURL,
		invite.ExpiresAt,
	); err != nil {
		slog.Warn("Failed to enqueue group invite email",
			"invite_id", invite.ID,
			"group_id", group.ID,
			"to", to,
			"error", err,
		)
	}
}

// listInvites lists active invite links for a group.
// @Summary List active invites
// @Description Returns all non-expired, unused invite links for a group. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.GroupInvitesResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Router /groups/{groupID}/invites [get].
func (api *groupsAPI) listInvites(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	invites, err := api.groupService.ListActiveInvites(r.Context(), group.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := jsonapi.NewGroupInvitesResponse(invites)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// revokeInvite revokes an unused invite link.
// @Summary Revoke invite
// @Description Deletes an unused invite link. Cannot revoke an already-used invite. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param inviteID path string true "Invite ID"
// @Success 204 "No Content"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 422 {object} jsonapi.Errors "Cannot revoke a used invite"
// @Router /groups/{groupID}/invites/{inviteID} [delete].
func (api *groupsAPI) revokeInvite(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	inviteID := chi.URLParam(r, "inviteID")
	if inviteID == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	err := api.groupService.RevokeInviteForGroup(r.Context(), group.ID, inviteID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getInviteInfo returns public info about an invite (group name, status).
// @Summary Get invite info
// @Description Returns public information about an invite link including group name and whether it has expired or been used. Does not require authentication.
// @Tags invites
// @Produce json-api
// @Param token path string true "Invite token"
// @Success 200 {object} jsonapi.InviteInfoResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Invite not found"
// @Router /invites/{token} [get].
func (api *groupsAPI) getInviteInfo(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	invite, group, err := api.groupService.GetInviteInfo(r.Context(), token)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewInviteInfoResponse(group, invite)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// acceptInvite accepts an invite and joins the group.
// @Summary Accept invite
// @Description Accepts a single-use invite link and joins the group as a user. Requires authentication.
// @Tags invites
// @Accept json-api
// @Produce json-api
// @Param token path string true "Invite token"
// @Success 201 {object} jsonapi.GroupMembershipResponse "Created"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {object} jsonapi.Errors "Invite not found"
// @Failure 422 {object} jsonapi.Errors "Invite expired, already used, or user already a member"
// @Router /invites/{token}/accept [post].
func (api *groupsAPI) acceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	membership, err := api.groupService.AcceptInvite(r.Context(), token, user.ID, user.TenantID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewGroupMembershipResponse(membership).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}
