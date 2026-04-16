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

// groupCtx middleware loads a group by its ID from the URL parameter.
func groupCtx(groupService *services.GroupService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			groupID := chi.URLParam(r, "groupID")
			if groupID == "" {
				unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
				return
			}

			group, err := groupService.GetGroup(r.Context(), groupID)
			if err != nil {
				renderEntityError(w, r, err) //nolint:errcheck // render error
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

func (api *groupsAPI) listGroups(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	groups, err := api.groupService.ListUserGroups(r.Context(), user.TenantID, user.ID)
	if err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}

	total := len(groups)
	resp := jsonapi.NewLocationGroupsResponse(groups, total, 1, total)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) createGroup(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	var input jsonapi.LocationGroupRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	name := input.Data.Attributes.Name
	icon := input.Data.Attributes.Icon

	group, err := api.groupService.CreateGroup(r.Context(), user.TenantID, user.ID, name, icon)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewLocationGroupResponse(group).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) getGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewLocationGroupResponse(group)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) updateGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	var input jsonapi.LocationGroupRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	updated, err := api.groupService.UpdateGroup(r.Context(), group.ID, input.Data.Attributes.Name, input.Data.Attributes.Icon)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewLocationGroupResponse(updated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) deleteGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	var input jsonapi.GroupDeleteRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	err := api.groupService.InitiateGroupDeletion(r.Context(), group.ID, input.ConfirmWord, group.Name)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *groupsAPI) listMembers(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	members, err := api.groupService.ListMembers(r.Context(), group.ID)
	if err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewGroupMembershipsResponse(members)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) removeMember(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	memberUserID := chi.URLParam(r, "memberUserID")
	if memberUserID == "" {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	err := api.groupService.RemoveMember(r.Context(), group.ID, memberUserID)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *groupsAPI) updateMemberRole(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	memberUserID := chi.URLParam(r, "memberUserID")
	if memberUserID == "" {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	var input jsonapi.GroupMembershipRoleRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	membership, err := api.groupService.UpdateMemberRole(r.Context(), group.ID, memberUserID, input.Data.Attributes.Role)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewGroupMembershipResponse(membership)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) leaveGroup(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	err := api.groupService.LeaveGroup(r.Context(), group.ID, user.ID)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (api *groupsAPI) createInvite(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	invite, err := api.groupService.CreateInvite(r.Context(), user.TenantID, group.ID, user.ID, 0)
	if err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewGroupInviteResponse(invite).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) listInvites(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	invites, err := api.groupService.ListActiveInvites(r.Context(), group.ID)
	if err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewGroupInvitesResponse(invites)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

func (api *groupsAPI) revokeInvite(w http.ResponseWriter, r *http.Request) {
	inviteID := chi.URLParam(r, "inviteID")
	if inviteID == "" {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	err := api.groupService.RevokeInvite(r.Context(), inviteID)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// getInviteInfo returns public info about an invite (group name, status).
// This endpoint does not require authentication.
func (api *groupsAPI) getInviteInfo(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	invite, group, err := api.groupService.GetInviteInfo(r.Context(), token)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewInviteInfoResponse(group, invite)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}

// acceptInvite accepts an invite and joins the group. Requires authentication.
func (api *groupsAPI) acceptInvite(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		unprocessableEntityError(w, r, nil) //nolint:errcheck // render error
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	membership, err := api.groupService.AcceptInvite(r.Context(), token, user.ID)
	if err != nil {
		renderEntityError(w, r, err) //nolint:errcheck // render error
		return
	}

	resp := jsonapi.NewGroupMembershipResponse(membership).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err) //nolint:errcheck // render error
		return
	}
}
