package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/render"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// Audit actions of the Collection set (#1607). The entity is the group.
const (
	auditActionCollectionSetAdd    = "group_collection_set_add"
	auditActionCollectionSetRemove = "group_collection_set_remove"
)

// collectionSetService returns the Collection set service of group, with
// registries scoped to it, and the context to call it with. The /groups
// subtree carries no group scope of its own, so the scope is set here.
func (api *groupsAPI) collectionSetService(ctx context.Context, group *models.LocationGroup) (*services.ItemSetService, context.Context, error) {
	if api.factorySet == nil {
		return nil, nil, errors.New("registry factories are not configured")
	}
	ctx = appctx.WithGroup(ctx, group)
	itemTypes, err := api.factorySet.ItemTypeRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, nil, err
	}
	fields, err := api.factorySet.CustomFieldRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, nil, err
	}
	return services.NewItemSetService(models.CollectionSet, itemTypes, fields), ctx, nil
}

// renderItemSetError maps a failed add or remove: a group mid-deletion and
// a full group are user errors; the rest goes through renderEntityError.
func renderItemSetError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, registry.ErrItemTypeLimit):
		codedConflictError(w, r, fmt.Errorf("a group keeps at most %d item types", models.MaxItemTypesPerGroup),
			itemTypeLimitCode, map[string]any{"limit": models.MaxItemTypesPerGroup})
	case errors.Is(err, registry.ErrResourceLimitExceeded):
		jsErr := NewUnprocessableEntityError(errors.New("the group has the most custom fields it can have"))
		jsErr.Code = customFieldLimitCode
		jsErr.Meta = map[string]any{"limit": models.MaxCustomFieldsPerGroup}
		render.Render(w, r, jsonapi.NewErrors(jsErr))
	default:
		renderEntityError(w, r, err)
	}
}

// logCollectionSet writes the audit row of an add or a remove of the
// Collection set. Nil-safe, like the other group audit helpers.
func (api *groupsAPI) logCollectionSet(r *http.Request, action string, group *models.LocationGroup, err error) {
	if api.auditService == nil {
		return
	}
	user := GetUserFromRequest(r)
	if user == nil {
		return
	}
	var errMsg *string
	if err != nil {
		msg := err.Error()
		errMsg = &msg
	}
	entityType := "location_group"
	api.auditService.LogAuth(r.Context(), services.AuthEvent{
		Action:     action,
		UserID:     new(user.ID),
		TenantID:   new(user.TenantID),
		Success:    err == nil,
		Request:    r,
		ErrMsg:     errMsg,
		EntityType: &entityType,
		EntityID:   new(group.ID),
	})
}

// getCollectionSet shows how the group holds the Collection set.
// @Summary Get the group's Collection set
// @Description Lists the item types and custom fields of the Collection set (#1607) and, for each, whether the group has it:
// @Description missing (adding the set creates it), added (the set created it) or own (a member's type or field of the same name, which the set uses and never deletes).
// @Description Each one the group has carries its item counts and whether removing the set deletes it, or why it stays. Requires group membership.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.ItemSetResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group member"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Router /groups/{groupID}/collection-set [get].
func (api *groupsAPI) getCollectionSet(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	svc, ctx, err := api.collectionSetService(r.Context(), group)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	st, err := svc.Status(ctx)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewItemSetResponse(st)); err != nil {
		internalServerError(w, r, err)
	}
}

// addCollectionSet adds the Collection set to the group.
// @Summary Add the Collection set to the group
// @Description Creates the item types Art, Books, Coins, Stamps, Records, Watches and Cards and the custom fields Condition, Catalog reference, Edition or variant, Signed, Provenance,
// @Description Certificate of authenticity, Appraised value and Appraisal date, and puts every field on the template of every one of those types.
// @Description A type or field the group already has under that name, ignoring case, is used instead of a new one. Adding again creates only what is missing, so a failed add is finished by repeating it.
// @Description The names are English; members rename them like any other. meta counts what the request created. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.ItemSetResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Failure 409 {object} jsonapi.Errors "The group has 100 item types (item_type.limit)"
// @Failure 422 {object} jsonapi.Errors "The group is being deleted (group.not_active), or has 200 custom fields (custom_field.limit_reached)"
// @Router /groups/{groupID}/collection-set [post].
func (api *groupsAPI) addCollectionSet(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	if !group.IsActive() {
		renderEntityError(w, r, services.ErrGroupNotActive)
		return
	}
	svc, ctx, err := api.collectionSetService(r.Context(), group)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	res, err := svc.Add(ctx)
	api.logCollectionSet(r, auditActionCollectionSetAdd, group, err)
	if err != nil {
		renderItemSetError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewItemSetAddResponse(res)); err != nil {
		internalServerError(w, r, err)
	}
}

// removeCollectionSet removes the Collection set from the group.
// @Summary Remove the Collection set from the group
// @Description Deletes the item types and custom fields the Collection set created that nothing uses: no item, in Trash or not, of the type, and no item with a value of the field.
// @Description Types and fields a member made, and those items use, stay; so does the group's last type. Removing again deletes what has become unused since.
// @Description meta names what the request deleted; the attributes show what stays and why. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Success 200 {object} jsonapi.ItemSetResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Failure 422 {object} jsonapi.Errors "The group is being deleted (group.not_active)"
// @Router /groups/{groupID}/collection-set [delete].
func (api *groupsAPI) removeCollectionSet(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	if !group.IsActive() {
		renderEntityError(w, r, services.ErrGroupNotActive)
		return
	}
	svc, ctx, err := api.collectionSetService(r.Context(), group)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	res, err := svc.Remove(ctx)
	api.logCollectionSet(r, auditActionCollectionSetRemove, group, err)
	if err != nil {
		renderItemSetError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewItemSetRemoveResponse(res)); err != nil {
		internalServerError(w, r, err)
	}
}

// addCollectionSetToNewGroup adds the Collection set to a group the request
// has just created and reports how it went. The group exists either way: a
// failure is logged, audited and reported in the response meta, and adding
// the set from the group settings finishes it.
func (api *groupsAPI) addCollectionSetToNewGroup(r *http.Request, group *models.LocationGroup) *jsonapi.GroupCreateItemSetOutcome {
	outcome := &jsonapi.GroupCreateItemSetOutcome{}
	svc, ctx, err := api.collectionSetService(r.Context(), group)
	if err != nil {
		slog.Error("Collection set: cannot scope registries to the new group", "group_id", group.ID, "error", err)
		api.logCollectionSet(r, auditActionCollectionSetAdd, group, err)
		return outcome
	}
	res, err := svc.Add(ctx)
	api.logCollectionSet(r, auditActionCollectionSetAdd, group, err)
	if res != nil {
		outcome.CreatedItemTypes = res.CreatedItemTypes
		outcome.CreatedCustomFields = res.CreatedFields
	}
	if err != nil {
		slog.Error("Collection set: adding it to the new group failed", "group_id", group.ID, "error", err)
		return outcome
	}
	outcome.Added = true
	return outcome
}
