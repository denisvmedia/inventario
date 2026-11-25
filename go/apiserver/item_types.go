package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/internal/errormarshal"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

const itemTypeCtxKey ctxValueKey = "itemType"

// JSON:API error codes of the item type endpoints (#2802).
const (
	// itemTypeNameTakenCode: another type of the group has this name.
	itemTypeNameTakenCode = "item_type.name_taken"
	// itemTypeInUseCode: a delete without reassign_to named a type that
	// items, in Trash or not, still have. Meta carries the counts.
	itemTypeInUseCode = "item_type.in_use"
	// itemTypeLastCode: a delete named the group's only type.
	itemTypeLastCode = "item_type.last"
	// itemTypeUnknownCode: an item write named a type the group does not
	// have.
	itemTypeUnknownCode = "item_type.unknown"
	// itemTypeLimitCode: a create in a group that already has
	// models.MaxItemTypesPerGroup types. Meta carries the limit.
	itemTypeLimitCode = "item_type.limit"
	// itemTypeUnknownFieldCode is the validation code on the type field of
	// that 422, which the frontend localizes (errors:validation.*). A bulk
	// patch with an unknown type carries the same code.
	itemTypeUnknownFieldCode = services.ItemTypeUnknownCode
)

func itemTypeFromContext(ctx context.Context) *models.ItemType {
	t, ok := ctx.Value(itemTypeCtxKey).(*models.ItemType)
	if !ok {
		return nil
	}
	return t
}

func itemTypeCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			t, err := regSet.ItemTypeRegistry.Get(r.Context(), chi.URLParam(r, "itemTypeID"))
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), itemTypeCtxKey, t)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type itemTypesAPI struct{}

// renderItemTypeWriteError maps an item type write failure: field errors to
// 422, a duplicate name and a full group to 409, and the rest through
// renderEntityError.
func renderItemTypeWriteError(w http.ResponseWriter, r *http.Request, err error) {
	if _, ok := errors.AsType[validation.Errors](err); ok {
		unprocessableEntityError(w, r, err)
		return
	}
	if errors.Is(err, registry.ErrAlreadyExists) {
		codedConflictError(w, r, errors.New("an item type with this name already exists"), itemTypeNameTakenCode, nil)
		return
	}
	if errors.Is(err, registry.ErrItemTypeLimit) {
		codedConflictError(w, r, fmt.Errorf("a group keeps at most %d item types", models.MaxItemTypesPerGroup),
			itemTypeLimitCode, map[string]any{"limit": models.MaxItemTypesPerGroup})
		return
	}
	renderEntityError(w, r, err)
}

// itemTypeSentinelJSONAPIError maps the item type sentinels that reach
// renderEntityError from other handlers: an item write that names a type
// the group does not have is a 422 with a field error on type, nested under
// data.attributes as a binding error is, so a form marks its type field;
// the delete refusals are 409s.
func itemTypeSentinelJSONAPIError(err error) (jsonapi.Error, bool) {
	var (
		status int
		code   string
	)
	switch {
	case errors.Is(err, registry.ErrUnknownItemType):
		jsErr := NewUnprocessableEntityError(validation.Errors{
			"data": validation.Errors{"attributes": validation.Errors{
				"type": validation.NewError(itemTypeUnknownFieldCode, "no item type with this slug in the group"),
			}},
		})
		jsErr.Code = itemTypeUnknownCode
		return jsErr, true
	case errors.Is(err, registry.ErrItemTypeInUse):
		status, code = http.StatusConflict, itemTypeInUseCode
	case errors.Is(err, registry.ErrLastItemType):
		status, code = http.StatusConflict, itemTypeLastCode
	default:
		return jsonapi.Error{}, false
	}
	return jsonapi.Error{
		Err:            err,
		UserError:      errormarshal.Marshal(err),
		HTTPStatusCode: status,
		StatusText:     http.StatusText(status),
		Code:           code,
	}, true
}

// listItemTypes lists the group's item types with their item counts.
// @Summary List item types
// @Description The item types of the group: the built-in ones first in their fixed order, then the others by name.
// @Description Each carries the number of items of that type and, separately, the number in Trash. The first request seeds the built-in types into a group that has none.
// @Tags item-types
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.ItemTypesResponse "OK"
// @Router /g/{groupSlug}/item-types [get].
func (*itemTypesAPI) listItemTypes(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	types, err := regSet.ItemTypeRegistry.List(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	counts, err := regSet.ItemTypeRegistry.ItemCounts(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewItemTypesResponse(types, counts)); err != nil {
		internalServerError(w, r, err)
	}
}

// getItemType returns one item type with its item counts.
// @Summary Get an item type
// @Tags item-types
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param itemTypeID path string true "Item type ID"
// @Success 200 {object} jsonapi.ItemTypeResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Item type not found"
// @Router /g/{groupSlug}/item-types/{itemTypeID} [get].
func (*itemTypesAPI) getItemType(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	t := itemTypeFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if t == nil || regSet == nil {
		http.Error(w, "Item type not found in context", http.StatusInternalServerError)
		return
	}
	counts, err := regSet.ItemTypeRegistry.ItemCounts(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewItemTypeResponse(t, counts[t.Slug])); err != nil {
		internalServerError(w, r, err)
	}
}

// createItemType adds an item type to the group.
// @Summary Create an item type
// @Description The name is required and unique in the group, ignoring case and runs of spaces; a built-in type that keeps its default name holds its English name.
// @Description The slug items store is made from the name and never changes. The icon, when set, is a Lucide icon name in kebab case.
// @Description A group keeps at most 100 item types; a create beyond that is refused with 409 item_type.limit.
// @Tags item-types
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param itemType body jsonapi.ItemTypeRequest true "Item type object"
// @Success 201 {object} jsonapi.ItemTypeResponse "Item type created"
// @Failure 409 {object} jsonapi.Errors "Another item type of the group has this name, or the group has 100 item types"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/item-types [post].
func (*itemTypesAPI) createItemType(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	var input jsonapi.ItemTypeRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	t := models.ItemType{Name: attrs.Name, Icon: attrs.Icon}
	t.Normalize()
	if err := t.ValidateFields(r.Context()); err != nil {
		renderItemTypeWriteError(w, r, err)
		return
	}
	created, err := regSet.ItemTypeRegistry.Create(r.Context(), t)
	if err != nil {
		renderItemTypeWriteError(w, r, err)
		return
	}
	resp := jsonapi.NewItemTypeResponse(created, registry.ItemTypeUsage{}).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// updateItemType renames an item type or changes its icon.
// @Summary Update an item type
// @Description Attributes left out keep their value. The slug never changes, so items, filters and saved views keep working after a rename. An empty name on a built-in type gives it back its default name.
// @Tags item-types
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param itemTypeID path string true "Item type ID"
// @Param itemType body jsonapi.ItemTypeUpdateRequest true "Item type patch"
// @Success 200 {object} jsonapi.ItemTypeResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Item type not found"
// @Failure 409 {object} jsonapi.Errors "Another item type of the group has this name"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/item-types/{itemTypeID} [patch].
func (*itemTypesAPI) updateItemType(w http.ResponseWriter, r *http.Request) {
	t := itemTypeFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if t == nil || regSet == nil {
		http.Error(w, "Item type not found in context", http.StatusInternalServerError)
		return
	}
	var input jsonapi.ItemTypeUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	if input.Data.ID != "" && input.Data.ID != t.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}
	merged := *t
	input.Data.Attributes.Apply(&merged)
	merged.Normalize()
	if err := merged.ValidateFields(r.Context()); err != nil {
		renderItemTypeWriteError(w, r, err)
		return
	}
	updated, err := regSet.ItemTypeRegistry.Update(r.Context(), merged)
	if err != nil {
		renderItemTypeWriteError(w, r, err)
		return
	}
	counts, err := regSet.ItemTypeRegistry.ItemCounts(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewItemTypeResponse(updated, counts[updated.Slug])); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteItemType removes an item type, moving its items to another type
// when told to.
// @Summary Delete an item type
// @Description Deletes the type. A type that items still have, in Trash included, is refused with 409 item_type.in_use unless reassign_to names another type of the group;
// @Description then every item of the deleted type gets that type first, in the same transaction. The group's last type cannot be deleted (409 item_type.last).
// @Tags item-types
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param itemTypeID path string true "Item type ID"
// @Param reassign_to query string false "ID of the item type the deleted type's items move to"
// @Success 200 {object} jsonapi.ItemTypeDeleteResponse "Deleted; meta.reassigned_items counts the items moved"
// @Failure 404 {object} jsonapi.Errors "Item type not found"
// @Failure 409 {object} jsonapi.Errors "The type is in use and no reassign_to was given, or it is the group's last type"
// @Failure 422 {object} jsonapi.Errors "reassign_to names no other type of the group"
// @Router /g/{groupSlug}/item-types/{itemTypeID} [delete].
func (*itemTypesAPI) deleteItemType(w http.ResponseWriter, r *http.Request) {
	t := itemTypeFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if t == nil || regSet == nil {
		http.Error(w, "Item type not found in context", http.StatusInternalServerError)
		return
	}
	reassignTo := r.URL.Query().Get("reassign_to")
	moved, err := regSet.ItemTypeRegistry.DeleteReassigning(r.Context(), t.ID, reassignTo)
	switch {
	case err == nil:
	case errors.Is(err, registry.ErrItemTypeInUse):
		var meta map[string]any
		if counts, cerr := regSet.ItemTypeRegistry.ItemCounts(r.Context()); cerr == nil {
			usage := counts[t.Slug]
			meta = map[string]any{"item_count": usage.Live, "trashed_item_count": usage.Trashed}
		}
		codedConflictError(w, r, errors.New("items still have this type; choose a type to move them to"), itemTypeInUseCode, meta)
		return
	case errors.Is(err, registry.ErrLastItemType):
		codedConflictError(w, r, errors.New("a group needs at least one item type"), itemTypeLastCode, nil)
		return
	case errors.Is(err, registry.ErrInvalidInput):
		unprocessableEntityError(w, r, validation.Errors{"reassign_to": errors.New("no other item type with this ID in the group")})
		return
	default:
		renderEntityError(w, r, err)
		return
	}
	resp := &jsonapi.ItemTypeDeleteResponse{Meta: jsonapi.ItemTypeDeleteMeta{ReassignedItems: moved}}
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// ItemTypes returns the chi sub-router for /item-types (#2802).
func ItemTypes() func(r chi.Router) {
	api := &itemTypesAPI{}
	return func(r chi.Router) {
		r.Get("/", api.listItemTypes)
		r.Post("/", api.createItemType)
		r.Route("/{itemTypeID}", func(r chi.Router) {
			r.Use(itemTypeCtx())
			r.Get("/", api.getItemType)
			r.Patch("/", api.updateItemType)
			r.Delete("/", api.deleteItemType)
		})
	}
}
