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

// itemSetChangedCode is the error code of an add that found the group's
// types or fields changing under it twice in a row.
const itemSetChangedCode = "item_set.changed"

// renderItemSetError maps a failed add or remove: a group mid-deletion, a
// full group and rows changing under the add are user errors; the rest goes
// through renderEntityError.
func renderItemSetError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, services.ErrItemSetChanged):
		codedConflictError(w, r, errors.New("the group's item types or custom fields changed while the set was being added; add it again"),
			itemSetChangedCode, nil)
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
// Collection set, with what it did in extra. Nil-safe, like the other group
// audit helpers.
func (api *groupsAPI) logCollectionSet(ctx context.Context, r *http.Request, action string, group *models.LocationGroup, extra map[string]any, err error) {
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
	api.auditService.LogAuth(ctx, services.AuthEvent{
		Action:     action,
		UserID:     new(user.ID),
		TenantID:   new(user.TenantID),
		Success:    err == nil,
		Request:    r,
		ErrMsg:     errMsg,
		EntityType: new("location_group"),
		EntityID:   new(group.ID),
		Extra:      extra,
	})
}

// addAuditExtra is what an add records: what it created.
func addAuditExtra(res *services.ItemSetAddResult) map[string]any {
	if res == nil {
		return nil
	}
	return map[string]any{
		"created_item_types":    res.CreatedItemTypes,
		"created_custom_fields": res.CreatedFields,
		"added_templates":       res.AddedTemplates,
	}
}

// removeAuditExtra is what a remove records: the rows it deleted and those
// the request listed that stayed, by name.
func removeAuditExtra(res *services.ItemSetRemoveResult) map[string]any {
	if res == nil {
		return nil
	}
	return map[string]any{
		"deleted_item_types":    res.DeletedItemTypes,
		"deleted_custom_fields": res.DeletedFields,
		"kept_item_types":       res.KeptItemTypes,
		"kept_custom_fields":    res.KeptFields,
	}
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
// @Failure 409 {object} jsonapi.Errors "The group has 100 item types (item_type.limit), or its types or fields changed during the add twice (item_set.changed)"
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
	api.logCollectionSet(r.Context(), r, auditActionCollectionSetAdd, group, addAuditExtra(res), err)
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
// @Description Deletes the item types and custom fields the body lists, as the confirmation showed them, of those the Collection set created that nothing uses: no item, in Trash or not, of the type, and no item with a value of the field.
// @Description A listed type or field that stopped being removable since stays and is named in meta.kept_item_types or meta.kept_custom_fields; one not listed is never deleted.
// @Description Types and fields a member made, and those items use, stay; so does the group's last type. Removing again deletes what has become unused since.
// @Description meta names what the request deleted and kept; the attributes show what stays and why. The template entries the set put on a member's own types and fields stay. Requires group admin role.
// @Tags groups
// @Accept json-api
// @Produce json-api
// @Param groupID path string true "Group ID"
// @Param data body jsonapi.ItemSetRemoveRequest true "The item types and custom fields to delete, by ID"
// @Success 200 {object} jsonapi.ItemSetResponse "OK"
// @Failure 403 {string} string "Forbidden - not a group admin"
// @Failure 404 {object} jsonapi.Errors "Group not found"
// @Failure 422 {object} jsonapi.Errors "The body is missing or invalid, or the group is being deleted (group.not_active)"
// @Router /groups/{groupID}/collection-set [delete].
func (api *groupsAPI) removeCollectionSet(w http.ResponseWriter, r *http.Request) {
	group := groupFromContext(r.Context())
	if group == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	var input jsonapi.ItemSetRemoveRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
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
	res, err := svc.Remove(ctx, services.ItemSetRemoval{
		ItemTypeIDs: input.Data.Attributes.ItemTypeIDs,
		FieldIDs:    input.Data.Attributes.CustomFieldIDs,
	})
	api.logCollectionSet(r.Context(), r, auditActionCollectionSetRemove, group, removeAuditExtra(res), err)
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
// the set from the group settings finishes it. The group is committed, so
// the add runs to its end even when the client goes away meanwhile.
func (api *groupsAPI) addCollectionSetToNewGroup(r *http.Request, group *models.LocationGroup) *jsonapi.GroupCreateItemSetOutcome {
	outcome := &jsonapi.GroupCreateItemSetOutcome{}
	base := context.WithoutCancel(r.Context())
	svc, ctx, err := api.collectionSetService(base, group)
	if err != nil {
		slog.ErrorContext(base, "Failed to scope registries to the new group for the Collection set", "group_id", group.ID, "error", err)
		api.logCollectionSet(base, r, auditActionCollectionSetAdd, group, nil, err)
		return outcome
	}
	res, err := svc.Add(ctx)
	api.logCollectionSet(base, r, auditActionCollectionSetAdd, group, addAuditExtra(res), err)
	if res != nil {
		outcome.CreatedItemTypes = res.CreatedItemTypes
		outcome.CreatedCustomFields = res.CreatedFields
	}
	if err != nil {
		slog.ErrorContext(base, "Failed to add the Collection set to the new group", "group_id", group.ID, "error", err)
		return outcome
	}
	outcome.Added = true
	return outcome
}
