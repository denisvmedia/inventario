package apiserver

import (
	"context"
	"encoding/json"
	"errors"
	"maps"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/internal/errormarshal"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

const customFieldCtxKey ctxValueKey = "customField"

// JSON:API error codes of the custom field endpoints and of item writes
// that carry custom field values (#2803).
const (
	// customFieldNameTakenCode: another field of the group has this name.
	customFieldNameTakenCode = "custom_field.name_taken"
	// customFieldInUseCode: a delete without delete_values named a field
	// that items, in Trash or not, have a value of. Meta carries the counts.
	customFieldInUseCode = "custom_field.in_use"
	// customFieldLimitCode: the group has the most fields it can have.
	customFieldLimitCode = "custom_field.limit_reached"
	// customFieldInvalidValueCode: an item write carried a custom field
	// value its field refuses. The field errors are under custom_fields.<id>.
	customFieldInvalidValueCode = "custom_field.invalid_value"
)

func customFieldFromContext(ctx context.Context) *models.CustomFieldDefinition {
	d, ok := ctx.Value(customFieldCtxKey).(*models.CustomFieldDefinition)
	if !ok {
		return nil
	}
	return d
}

func customFieldCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			d, err := regSet.CustomFieldRegistry.Get(r.Context(), chi.URLParam(r, "customFieldID"))
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), customFieldCtxKey, d)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type customFieldsAPI struct{}

// renderCustomFieldWriteError maps a custom field write failure: field
// errors to 422, a duplicate name to 409, the field limit to 422, and the
// rest through renderEntityError.
func renderCustomFieldWriteError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case isValidationErrors(err):
		unprocessableEntityError(w, r, err)
	case errors.Is(err, registry.ErrAlreadyExists):
		codedConflictError(w, r, errors.New("a custom field with this name already exists"), customFieldNameTakenCode, nil)
	case errors.Is(err, registry.ErrResourceLimitExceeded):
		jsErr := NewUnprocessableEntityError(errors.New("the group has the most custom fields it can have"))
		jsErr.Code = customFieldLimitCode
		jsErr.Meta = map[string]any{"limit": models.MaxCustomFieldsPerGroup}
		render.Render(w, r, jsonapi.NewErrors(jsErr))
	case errors.Is(err, registry.ErrInvalidInput):
		unprocessableEntityError(w, r, validation.Errors{"templates": errors.New("names an item type the group does not have")})
	default:
		renderEntityError(w, r, err)
	}
}

func isValidationErrors(err error) bool {
	_, ok := errors.AsType[validation.Errors](err)
	return ok
}

// customFieldSentinelJSONAPIError maps the custom field errors that reach
// renderEntityError from other handlers. An item write whose custom field
// values are refused is a 422 with one field error per value under
// custom_fields.<field id>, so a form marks each input.
func customFieldSentinelJSONAPIError(err error) (jsonapi.Error, bool) {
	if valueErr, ok := errors.AsType[*registry.CustomFieldValueError](err); ok {
		fields := validation.Errors{}
		maps.Copy(fields, valueErr.Fields)
		jsErr := NewUnprocessableEntityError(validation.Errors{"custom_fields": fields})
		jsErr.Err = err
		jsErr.Code = customFieldInvalidValueCode
		return jsErr, true
	}
	if errors.Is(err, registry.ErrCustomFieldInUse) {
		return jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusConflict,
			StatusText:     http.StatusText(http.StatusConflict),
			Code:           customFieldInUseCode,
		}, true
	}
	return jsonapi.Error{}, false
}

// listCustomFields lists the group's custom fields with their usage.
// @Summary List custom fields
// @Description The custom fields of the group, by name. Each carries the number of items with a value of it and, separately, the number in Trash;
// @Description each option of a select field carries the number of items whose value it is. meta.limit is the most fields a group can have.
// @Tags custom-fields
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.CustomFieldsResponse "OK"
// @Router /g/{groupSlug}/custom-fields [get].
func (*customFieldsAPI) listCustomFields(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	defs, err := regSet.CustomFieldRegistry.List(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	usage, err := regSet.CustomFieldRegistry.Usage(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCustomFieldsResponse(defs, usage)); err != nil {
		internalServerError(w, r, err)
	}
}

// getCustomField returns one custom field with its usage.
// @Summary Get a custom field
// @Tags custom-fields
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param customFieldID path string true "Custom field ID"
// @Success 200 {object} jsonapi.CustomFieldResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Custom field not found"
// @Router /g/{groupSlug}/custom-fields/{customFieldID} [get].
func (*customFieldsAPI) getCustomField(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	d := customFieldFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if d == nil || regSet == nil {
		http.Error(w, "Custom field not found in context", http.StatusInternalServerError)
		return
	}
	usage, err := regSet.CustomFieldRegistry.Usage(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCustomFieldResponse(d, usage[d.ID])); err != nil {
		internalServerError(w, r, err)
	}
}

// createCustomField adds a custom field to the group.
// @Summary Create a custom field
// @Description The name is required and unique in the group, ignoring case and runs of spaces. The type is fixed once the field exists.
// @Description A select field needs at least one option; the server gives each option its ID. templates lists the item types whose form shows the field, each with an optional default value.
// @Tags custom-fields
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param customField body jsonapi.CustomFieldRequest true "Custom field object"
// @Success 201 {object} jsonapi.CustomFieldResponse "Custom field created"
// @Failure 409 {object} jsonapi.Errors "Another custom field of the group has this name"
// @Failure 422 {object} jsonapi.Errors "User-side request problem, or the group has the most fields it can have"
// @Router /g/{groupSlug}/custom-fields [post].
func (*customFieldsAPI) createCustomField(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	var input jsonapi.CustomFieldRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	d := input.Data.Attributes.Definition()
	d.Normalize()
	if err := d.ValidateFields(r.Context()); err != nil {
		renderCustomFieldWriteError(w, r, err)
		return
	}
	created, err := regSet.CustomFieldRegistry.Create(r.Context(), d)
	if err != nil {
		renderCustomFieldWriteError(w, r, err)
		return
	}
	resp := jsonapi.NewCustomFieldResponse(created, registry.CustomFieldUsage{}).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// updateCustomField renames a custom field, changes its options or its
// templates.
// @Summary Update a custom field
// @Description Attributes left out keep their value. Values, filters and list columns name the field by ID, so they keep working after a rename.
// @Description options replaces the list: an entry with an existing option's ID keeps that option and its values, an entry without an ID adds one, and an option left out is removed with its values.
// @Description templates replaces the template entries. The type cannot change: a different type is a 422 on type.
// @Tags custom-fields
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param customFieldID path string true "Custom field ID"
// @Param customField body jsonapi.CustomFieldUpdateRequest true "Custom field patch"
// @Success 200 {object} jsonapi.CustomFieldResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Custom field not found"
// @Failure 409 {object} jsonapi.Errors "Another custom field of the group has this name"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/custom-fields/{customFieldID} [patch].
func (*customFieldsAPI) updateCustomField(w http.ResponseWriter, r *http.Request) {
	d := customFieldFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if d == nil || regSet == nil {
		http.Error(w, "Custom field not found in context", http.StatusInternalServerError)
		return
	}
	var input jsonapi.CustomFieldUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	if input.Data.ID != "" && input.Data.ID != d.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}
	merged := *d
	if err := input.Data.Attributes.Apply(&merged); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	merged.Normalize()
	if err := merged.ValidateFields(r.Context()); err != nil {
		renderCustomFieldWriteError(w, r, err)
		return
	}
	updated, err := regSet.CustomFieldRegistry.Update(r.Context(), merged)
	if err != nil {
		renderCustomFieldWriteError(w, r, err)
		return
	}
	usage, err := regSet.CustomFieldRegistry.Usage(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCustomFieldResponse(updated, usage[updated.ID])); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteCustomField removes a custom field and, when told to, its values.
// @Summary Delete a custom field
// @Description Deletes the field. A field that items have a value of, in Trash included, is refused with 409 custom_field.in_use and the counts in meta,
// @Description unless delete_values=true; then its values are deleted with it, in the same transaction.
// @Tags custom-fields
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param customFieldID path string true "Custom field ID"
// @Param delete_values query bool false "Delete the field's values along with it"
// @Success 200 {object} jsonapi.CustomFieldDeleteResponse "Deleted; meta.deleted_values counts the values removed"
// @Failure 404 {object} jsonapi.Errors "Custom field not found"
// @Failure 409 {object} jsonapi.Errors "Items have a value of the field and delete_values was not true"
// @Failure 422 {object} jsonapi.Errors "delete_values is not a boolean"
// @Router /g/{groupSlug}/custom-fields/{customFieldID} [delete].
func (*customFieldsAPI) deleteCustomField(w http.ResponseWriter, r *http.Request) {
	d := customFieldFromContext(r.Context())
	regSet := RegistrySetFromContext(r.Context())
	if d == nil || regSet == nil {
		http.Error(w, "Custom field not found in context", http.StatusInternalServerError)
		return
	}
	deleteValues := false
	if raw := r.URL.Query().Get("delete_values"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			unprocessableEntityError(w, r, validation.Errors{"delete_values": errors.New("must be true or false")})
			return
		}
		deleteValues = v
	}
	n := 0
	var err error
	if deleteValues {
		n, err = regSet.CustomFieldRegistry.DeleteWithValues(r.Context(), d.ID)
	} else {
		err = regSet.CustomFieldRegistry.Delete(r.Context(), d.ID)
	}
	switch {
	case err == nil:
	case errors.Is(err, registry.ErrCustomFieldInUse):
		var meta map[string]any
		if usage, uerr := regSet.CustomFieldRegistry.Usage(r.Context()); uerr == nil {
			u := usage[d.ID]
			meta = map[string]any{"item_count": u.Live, "trashed_item_count": u.Trashed}
		}
		codedConflictError(w, r, errors.New("items have a value of this field; confirm to delete the values too"), customFieldInUseCode, meta)
		return
	default:
		renderEntityError(w, r, err)
		return
	}
	resp := &jsonapi.CustomFieldDeleteResponse{Meta: jsonapi.CustomFieldDeleteMeta{DeletedValues: n}}
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// linkedItemSummaries names the items the item link values of commodities
// point at: the live ones in one query, a trashed one by its own lookup. A
// link always names an item of the group, because purging the item deletes
// the value.
func linkedItemSummaries(ctx context.Context, set *registry.Set, commodities ...*models.Commodity) (map[string]jsonapi.LinkedItemSummary, error) {
	if set == nil || set.CustomFieldRegistry == nil || !anyCustomFieldValues(commodities) {
		return nil, nil
	}
	defs, err := set.CustomFieldRegistry.List(ctx)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, d := range defs {
		if d.Type != models.CustomFieldTypeItemLink {
			continue
		}
		for _, commodity := range commodities {
			raw, ok := commodity.CustomFields[d.ID]
			if !ok {
				continue
			}
			var id string
			if json.Unmarshal(raw, &id) == nil && id != "" && !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	if len(ids) == 0 {
		return nil, nil
	}
	live, err := set.CommodityRegistry.GetMany(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make(map[string]jsonapi.LinkedItemSummary, len(ids))
	for _, c := range live {
		out[c.ID] = jsonapi.LinkedItemSummary{Name: c.Name}
	}
	for _, id := range ids {
		if _, ok := out[id]; ok {
			continue
		}
		trashed, err := set.CommodityRegistry.GetTrashed(ctx, id)
		switch {
		case errors.Is(err, registry.ErrNotFound):
			continue
		case err != nil:
			return nil, err
		}
		out[id] = jsonapi.LinkedItemSummary{Name: trashed.Name, Trashed: true}
	}
	return out, nil
}

// CustomFields returns the chi sub-router for /custom-fields (#2803).
func CustomFields() func(r chi.Router) {
	api := &customFieldsAPI{}
	return func(r chi.Router) {
		r.Get("/", api.listCustomFields)
		r.Post("/", api.createCustomField)
		r.Route("/{customFieldID}", func(r chi.Router) {
			r.Use(customFieldCtx())
			r.Get("/", api.getCustomField)
			r.Patch("/", api.updateCustomField)
			r.Delete("/", api.deleteCustomField)
		})
	}
}

// anyCustomFieldValues reports whether one of commodities has a custom field
// value.
func anyCustomFieldValues(commodities []*models.Commodity) bool {
	for _, c := range commodities {
		if c != nil && len(c.CustomFields) > 0 {
			return true
		}
	}
	return false
}
