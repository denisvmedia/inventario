package apiserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/services"
)

const groupCtxKey ctxValueKey = "group"

type groupsAPI struct {
	groupService *services.GroupService
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
				http.Error(w, "Group not found", http.StatusNotFound)
				return
			}

			if !group.IsActive() {
				http.Error(w, "Group is not available", http.StatusGone)
				return
			}

			if !groupService.IsGroupMember(r.Context(), group.ID, user.ID) {
				http.Error(w, "Group membership required", http.StatusForbidden)
				return
			}

			ctx := context.WithValue(r.Context(), groupCtxKey, group)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// groupCtx middleware loads a group by its ID from the URL parameter.
func groupCtx(groupService *services.GroupService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			groupID := chi.URLParam(r, "groupID")
			if groupID == "" {
				unprocessableEntityError(w, r, nil)
				return
			}

			group, err := groupService.GetGroup(r.Context(), groupID)
			if err != nil {
				renderEntityError(w, r, err)
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

// requireGroupAdmin middleware ensures the current user is an admin of the group in context.
func requireGroupAdmin(groupService *services.GroupService) func(http.Handler) http.Handler {
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

			if !groupService.IsGroupAdmin(r.Context(), group.ID, user.ID) {
				http.Error(w, "Group admin access required", http.StatusForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Groups returns the route handler for group management endpoints.
func Groups(params Params, groupService *services.GroupService) func(r chi.Router) {
	api := &groupsAPI{
		groupService: groupService,
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

			// Admin-only operations
			r.Group(func(r chi.Router) {
				r.Use(requireGroupAdmin(groupService))
				r.Patch("/", api.updateGroup)
				r.Delete("/", api.deleteGroup)
				r.Delete("/members/{memberUserID}", api.removeMember)
				r.Patch("/members/{memberUserID}", api.updateMemberRole)
				r.Post("/invites", api.createInvite)
				r.Get("/invites", api.listInvites)
				r.Delete("/invites/{inviteID}", api.revokeInvite)
			})
		})
	}
}

// Invites returns the route handler for invite acceptance (separate from group routes).
func Invites(groupService *services.GroupService) func(r chi.Router) {
	api := &groupsAPI{
		groupService: groupService,
	}
	return func(r chi.Router) {
		r.Get("/{token}", api.getInviteInfo)
		r.Post("/{token}/accept", api.acceptInvite)
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

	name := input.Data.Attributes.Name
	icon := input.Data.Attributes.Icon

	group, err := api.groupService.CreateGroup(r.Context(), user.TenantID, user.ID, name, icon)
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

	resp := jsonapi.NewLocationGroupResponse(group)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateGroup updates a group's metadata.
// @Summary Update group
// @Description Updates a location group's name and icon. Requires group admin role.
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

	updated, err := api.groupService.UpdateGroup(r.Context(), group.ID, input.Data.Attributes.Name, input.Data.Attributes.Icon)
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
// @Description Initiates async deletion of a location group. Requires typing the group name as confirmation. Requires group admin role.
// @Tags groups
// @Accept json
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param data body jsonapi.GroupDeleteRequest true "Deletion confirmation"
// @Success 204 "No Content"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Failure 422 {object} jsonapi.Errors "Invalid confirmation"
// @Router /groups/{groupID} [delete].
func (api *groupsAPI) deleteGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.GroupDeleteRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	err := api.groupService.InitiateGroupDeletion(r.Context(), group.ID, input.ConfirmWord, group.Name)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// listMembers lists all members of a group.
// @Summary List group members
// @Description Returns all members of a location group with their roles. Requires group membership.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.GroupMembershipsResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group member"
// @Router /groups/{groupID}/members [get].
func (api *groupsAPI) listMembers(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	members, err := api.groupService.ListMembers(r.Context(), group.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := jsonapi.NewGroupMembershipsResponse(members)
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

// createInvite generates a single-use invite link for a group.
// @Summary Create invite link
// @Description Generates a single-use invite link with a 24h default expiry. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 201 {object} jsonapi.GroupInviteResponse "Created"
// @Failure 403 {string} string "Forbidden - not a group admin"
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

	invite, err := api.groupService.CreateInvite(r.Context(), user.TenantID, group.ID, user.ID, 0)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := jsonapi.NewGroupInviteResponse(invite).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
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
	inviteID := chi.URLParam(r, "inviteID")
	if inviteID == "" {
		unprocessableEntityError(w, r, nil)
		return
	}

	err := api.groupService.RevokeInvite(r.Context(), inviteID)
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

	membership, err := api.groupService.AcceptInvite(r.Context(), token, user.ID)
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
