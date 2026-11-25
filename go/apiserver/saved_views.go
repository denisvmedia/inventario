package apiserver

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/internal/errormarshal"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/commodityquery"
)

type savedViewsAPI struct {
	savedViewService *services.SavedViewService
	groupService     *services.GroupService
}

// SavedViews mounts the saved view routes of a group. They take no role gate
// of their own: every member manages their own personal views, and the
// service decides who may share and who may change a shared view.
func SavedViews(params Params, groupService *services.GroupService) func(r chi.Router) {
	api := &savedViewsAPI{
		savedViewService: services.NewSavedViewService(params.FactorySet),
		groupService:     groupService,
	}
	return func(r chi.Router) {
		r.Get("/", api.listSavedViews)
		r.Post("/", api.createSavedView)
		r.Get("/counts", api.countSavedViews)
		r.Post("/reorder", api.reorderSavedViews)
		r.Route("/{savedViewID}", func(r chi.Router) {
			r.Get("/", api.getSavedView)
			r.Patch("/", api.updateSavedView)
			r.Delete("/", api.deleteSavedView)
		})
	}
}

// actor resolves the requesting member and their role in the group of the
// request. It renders the error response and returns false when it cannot.
func (api *savedViewsAPI) actor(w http.ResponseWriter, r *http.Request) (services.SavedViewActor, bool) {
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return services.SavedViewActor{}, false
	}
	group := groupFromContext(r.Context())
	if group == nil {
		http.Error(w, "Group context required", http.StatusInternalServerError)
		return services.SavedViewActor{}, false
	}
	role, err := api.groupService.GetMembershipRole(r.Context(), group.ID, user.ID)
	if err != nil {
		if errors.Is(err, services.ErrNotGroupMember) {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return services.SavedViewActor{}, false
		}
		slog.Error("saved views: role lookup failed", "group_id", group.ID, "user_id", user.ID, "error", err)
		internalServerError(w, r, err)
		return services.SavedViewActor{}, false
	}
	return services.SavedViewActor{UserID: user.ID, Role: role}, true
}

func savedViewData(entry services.SavedViewEntry) jsonapi.SavedViewData {
	access := jsonapi.SavedViewAccess{
		ShowInSidebar:   entry.ShowInSidebar,
		ShowOnDashboard: entry.ShowOnDashboard,
		Owned:           entry.Owned,
		CanEdit:         entry.CanEdit,
		CanDelete:       entry.CanDelete,
	}
	if entry.QueryError != nil {
		access.QueryError = &jsonapi.SavedViewQueryError{
			Parameter: entry.QueryError.Param,
			Reason:    entry.QueryError.Reason,
		}
	}
	return jsonapi.NewSavedViewData(entry.View, access)
}

// renderSavedViewError maps the errors of the saved view service.
func renderSavedViewError(w http.ResponseWriter, r *http.Request, err error) {
	if qErr, ok := errors.AsType[*commodityquery.Error](err); ok {
		_ = render.Render(w, r, jsonapi.NewErrors(jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusUnprocessableEntity,
			StatusText:     "Unprocessable Entity",
			Code:           "query.invalid_parameter",
			Meta:           map[string]any{"parameter": qErr.Param, "reason": qErr.Reason},
		}))
		return
	}
	switch {
	case errors.Is(err, services.ErrSavedViewForbidden):
		_ = codedForbiddenError(w, r, err, "saved_view.forbidden")
	case errors.Is(err, services.ErrSavedViewLimitReached):
		_ = codedUnprocessableEntityError(w, r, err, "saved_view.limit_reached")
	default:
		if _, ok := errors.AsType[validation.Errors](err); ok {
			_ = unprocessableEntityError(w, r, err)
			return
		}
		if _, ok := errors.AsType[validation.Error](err); ok {
			_ = unprocessableEntityError(w, r, err)
			return
		}
		_ = renderEntityError(w, r, err)
	}
}

// listSavedViews returns the saved views the requesting member sees: their
// own and every view shared with the group, in display order.
//
// @Summary List saved views
// @Description The member's own saved views and every view shared with the group, ordered by position, then name. meta.limits carries the limits the server keeps.
// @Tags saved_views
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.SavedViewsResponse "OK"
// @Router /g/{groupSlug}/saved-views [get].
func (api *savedViewsAPI) listSavedViews(w http.ResponseWriter, r *http.Request) {
	actor, ok := api.actor(w, r)
	if !ok {
		return
	}
	entries, err := api.savedViewService.List(r.Context(), actor)
	if err != nil {
		renderSavedViewError(w, r, err)
		return
	}
	data := make([]jsonapi.SavedViewData, 0, len(entries))
	for _, e := range entries {
		data = append(data, savedViewData(e))
	}
	canShare := actor.Role.AtLeast(models.GroupRoleUser)
	if err := render.Render(w, r, jsonapi.NewSavedViewsResponse(data, canShare)); err != nil {
		internalServerError(w, r, err)
	}
}

// countSavedViews counts the items of every saved view the member shows.
//
// @Summary Count the items of saved views
// @Description Per saved view ID, the number of items its query selects, for the views the member shows in their sidebar or on their dashboard.
// @Description Each count uses the filter of GET /commodities. A view whose query cannot run (query_error) has no entry.
// @Tags saved_views
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.SavedViewCountsResponse "OK"
// @Router /g/{groupSlug}/saved-views/counts [get].
func (api *savedViewsAPI) countSavedViews(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	counts, err := api.savedViewService.Counts(r.Context(), regSet)
	if err != nil {
		renderSavedViewError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSavedViewCountsResponse(counts)); err != nil {
		internalServerError(w, r, err)
	}
}

// getSavedView returns one saved view the member sees.
//
// @Summary Get a saved view
// @Description One saved view: the member's own, or one shared with the group. Another member's personal view is a 404.
// @Tags saved_views
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param savedViewID path string true "Saved view ID"
// @Success 200 {object} jsonapi.SavedViewResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Saved view not found"
// @Router /g/{groupSlug}/saved-views/{savedViewID} [get].
func (api *savedViewsAPI) getSavedView(w http.ResponseWriter, r *http.Request) {
	actor, ok := api.actor(w, r)
	if !ok {
		return
	}
	entry, err := api.savedViewService.Get(r.Context(), actor, chi.URLParam(r, "savedViewID"))
	if err != nil {
		renderSavedViewError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSavedViewResponse(savedViewData(entry))); err != nil {
		internalServerError(w, r, err)
	}
}

// createSavedView stores a filter of the items list under a name.
//
// @Summary Create a saved view
// @Description Store an item query (the query string of GET /commodities, limited to the parameters the items list edits) under a name. The query is validated and stored in canonical form. Sharing with the group takes the user role.
// @Description show_in_sidebar and show_on_dashboard place the view for the creator only; sharing places it for nobody else.
// @Tags saved_views
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param view body jsonapi.SavedViewRequest true "Saved view attributes"
// @Success 201 {object} jsonapi.SavedViewResponse "Created"
// @Failure 403 {object} jsonapi.Errors "Sharing needs the user role (saved_view.forbidden)"
// @Failure 422 {object} jsonapi.Errors "Invalid attributes, an invalid query (query.invalid_parameter) or a limit reached (saved_view.limit_reached)"
// @Router /g/{groupSlug}/saved-views [post].
func (api *savedViewsAPI) createSavedView(w http.ResponseWriter, r *http.Request) {
	actor, ok := api.actor(w, r)
	if !ok {
		return
	}
	var input jsonapi.SavedViewRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	entry, err := api.savedViewService.Create(r.Context(), actor, services.SavedViewInput{
		Name:            attrs.Name,
		Icon:            models.SavedViewIcon(attrs.Icon),
		Query:           attrs.Query,
		Layout:          models.SavedViewLayout(attrs.Layout),
		Shared:          attrs.Shared,
		ShowInSidebar:   attrs.ShowInSidebar,
		ShowOnDashboard: attrs.ShowOnDashboard,
	})
	if err != nil {
		renderSavedViewError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSavedViewResponse(savedViewData(entry)).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// updateSavedView changes a saved view, or where the member shows it.
//
// @Summary Update a saved view
// @Description Change the fields given; the others stay. A new query is validated and stored in canonical form under the current query version.
// @Description The member who created a view edits it; a shared view also takes the user role, and admins may edit any shared view.
// @Description show_in_sidebar and show_on_dashboard change where the requesting member shows the view; any member who sees it may set them.
// @Tags saved_views
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param savedViewID path string true "Saved view ID"
// @Param view body jsonapi.SavedViewUpdateRequest true "Saved view attributes to change"
// @Success 200 {object} jsonapi.SavedViewResponse "OK"
// @Failure 403 {object} jsonapi.Errors "Not allowed to change this view (saved_view.forbidden)"
// @Failure 404 {object} jsonapi.Errors "Saved view not found"
// @Failure 422 {object} jsonapi.Errors "Invalid attributes, an invalid query (query.invalid_parameter) or a limit reached (saved_view.limit_reached)"
// @Router /g/{groupSlug}/saved-views/{savedViewID} [patch].
func (api *savedViewsAPI) updateSavedView(w http.ResponseWriter, r *http.Request) {
	actor, ok := api.actor(w, r)
	if !ok {
		return
	}
	var input jsonapi.SavedViewUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	patch := services.SavedViewPatch{
		Name:            attrs.Name,
		Query:           attrs.Query,
		Shared:          attrs.Shared,
		ShowInSidebar:   attrs.ShowInSidebar,
		ShowOnDashboard: attrs.ShowOnDashboard,
	}
	if attrs.Icon != nil {
		patch.Icon = new(models.SavedViewIcon(*attrs.Icon))
	}
	if attrs.Layout != nil {
		patch.Layout = new(models.SavedViewLayout(*attrs.Layout))
	}
	entry, err := api.savedViewService.Update(r.Context(), actor, chi.URLParam(r, "savedViewID"), patch)
	if err != nil {
		renderSavedViewError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSavedViewResponse(savedViewData(entry))); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteSavedView removes a saved view.
//
// @Summary Delete a saved view
// @Description Remove a saved view. A member may delete the views they created; admins may delete any shared view.
// @Tags saved_views
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param savedViewID path string true "Saved view ID"
// @Success 204 "No content"
// @Failure 403 {object} jsonapi.Errors "Not allowed to delete this view (saved_view.forbidden)"
// @Failure 404 {object} jsonapi.Errors "Saved view not found"
// @Router /g/{groupSlug}/saved-views/{savedViewID} [delete].
func (api *savedViewsAPI) deleteSavedView(w http.ResponseWriter, r *http.Request) {
	actor, ok := api.actor(w, r)
	if !ok {
		return
	}
	if err := api.savedViewService.Delete(r.Context(), actor, chi.URLParam(r, "savedViewID")); err != nil {
		renderSavedViewError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// reorderSavedViews puts saved views in a new order.
//
// @Summary Reorder saved views
// @Description Put the listed views in the given order among the positions they hold now; views left out keep their positions. Every view must be one the member may edit.
// @Tags saved_views
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param order body jsonapi.SavedViewReorderRequest true "Saved view IDs in the new order"
// @Success 204 "No content"
// @Failure 403 {object} jsonapi.Errors "A listed view may not be changed (saved_view.forbidden)"
// @Failure 404 {object} jsonapi.Errors "A listed view was not found or is listed twice"
// @Failure 422 {object} jsonapi.Errors "Invalid payload"
// @Router /g/{groupSlug}/saved-views/reorder [post].
func (api *savedViewsAPI) reorderSavedViews(w http.ResponseWriter, r *http.Request) {
	actor, ok := api.actor(w, r)
	if !ok {
		return
	}
	var input jsonapi.SavedViewReorderRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	if err := api.savedViewService.Reorder(r.Context(), actor, input.Data.Attributes.IDs); err != nil {
		renderSavedViewError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
