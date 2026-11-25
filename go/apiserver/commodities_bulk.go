package apiserver

import (
	"context"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"strconv"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/internal/errormarshal"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/commodityquery"
)

// Bulk actions on items. Each one names its items as ids or as a query of
// the item query contract (jsonapi.CommodityBulkSelection), changes them one
// by one with the checks and history event of a single change, and answers
// 200 with the outcome per item (jsonapi.BulkResultResponse): an item that
// fails does not stop the others.

// bulkDeleteCommodities moves several items to Trash.
// @Summary Bulk-delete commodities
// @Description Move the selected items to Trash, each as DELETE /commodities/{id} does: with the files no other item, area or location uses, and a deleted event in its history.
// @Description The items are given as ids (at most 100) or as a query of the item query contract with the count and digest of the items it was seen to select (at most 500); see POST /commodities/bulk-update for the selection rules.
// @Description The response lists succeeded and failed ids. An item that went to Trash while moving its files there failed is succeeded, with a files_kept note.
// @Description A retry sends the failed ids as `ids`, in batches of at most 100.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.CommodityBulkDeleteRequest true "The items to delete"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 400 {object} jsonapi.Errors "The query is not a valid item query, or selects items in Trash"
// @Failure 409 {object} jsonapi.Errors "bulk.selection_changed: the query selects at most 500 items, but other ones than expected_count and expected_digest describe"
// @Failure 422 {object} jsonapi.Errors "Bad request body, or bulk.too_many: the query selects more than 500 items (checked before the 409)"
// @Router /g/{groupSlug}/commodities/bulk-delete [post].
func (api *commoditiesAPI) bulkDeleteCommodities(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.CommodityBulkDeleteRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	targets, ok := resolveCommoditySelection(w, r, regSet, input.Data.Attributes)
	if !ok {
		return
	}
	renderBulkOutcome(w, r, "commodities", api.bulkService.Delete(r.Context(), targets))
}

// bulkMoveCommodities puts several items into one area.
// @Summary Bulk-move commodities to a new area
// @Description Put the selected items into the area area_id. An item already in that area is reported as skipped. Each moved item gets a moved event in its history.
// @Description An area_id that is not an area of the group is refused with 422 before any item changes.
// @Description The items are given as ids (at most 100) or as a query of the item query contract with the count and digest of the items it was seen to select (at most 500); see POST /commodities/bulk-update for the selection rules.
// @Description A retry sends the same area_id with the failed ids as `ids`, in batches of at most 100.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkMoveRequest true "The items and the destination area_id"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 400 {object} jsonapi.Errors "The query is not a valid item query, or selects items in Trash"
// @Failure 409 {object} jsonapi.Errors "bulk.selection_changed: the query selects at most 500 items, but other ones than expected_count and expected_digest describe"
// @Failure 422 {object} jsonapi.Errors "Bad request body, an area_id that is not an area of the group, or bulk.too_many: the query selects more than 500 items (checked before the 409)"
// @Router /g/{groupSlug}/commodities/bulk-move [post].
func (api *commoditiesAPI) bulkMoveCommodities(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.BulkMoveRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Each moved item is validated as a single edit validates it, which
	// reads the group currency.
	rWithCurrency, err := requestWithGroupCurrency(r)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	// A missing area is one error for the request, not a failure per item.
	areaID := input.Data.Attributes.AreaID
	if _, err := regSet.AreaRegistry.Get(r.Context(), areaID); err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			unprocessableEntityError(w, r, validation.Errors{
				"area_id": validation.NewError("area_not_found", "the area does not exist"),
			})
			return
		}
		renderEntityError(w, r, err)
		return
	}

	targets, ok := resolveCommoditySelection(w, r, regSet, &input.Data.Attributes.CommodityBulkSelection)
	if !ok {
		return
	}
	renderBulkOutcome(w, r, "commodities", api.bulkService.Move(r.Context(), regSet.CommodityRegistry, targets, areaID))
}

// bulkUpdateCommodities changes the tags, type or status of several items.
// @Summary Bulk-update commodities
// @Description Apply one patch to the selected items. Each item is changed and validated as PUT /commodities/{id} changes it, and gets the history events that edit would give it:
// @Description a status change event when its status changes, and an updated event when only its tags or type change. As for PUT, a status change event stands for the whole change,
// @Description so a type or tag change in the same patch gets no event of its own.
// @Description
// @Description The patch: `tags.add` and `tags.remove` add and remove tags without touching the others
// @Description (`add` takes slugs or the text of a new tag, at most 50 per list); `type` sets the type;
// @Description `status` sets the status with `date` (required for every status but in_use) and an optional `note`.
// @Description `in_use` takes neither: send it without `date` and `note` (a non-empty one is refused with 422), and it clears
// @Description the status date, note and sale price. `sold` is refused, since it needs a sale price for each item.
// @Description An item that already has the status keeps its status date and note: the patch does not change them,
// @Description even when it sends another date or note. The other parts of the patch still apply to that item.
// @Description A part left out of the patch is left unchanged.
// @Description
// @Description The items are given as `ids` (at most 100) or as `query`, a query string of the item query contract
// @Description (the filter parameters of GET /commodities, without page and per_page), with `expected_count` and
// @Description `expected_digest`: the number of items the caller saw it select and the `meta.selection_digest`
// @Description GET /commodities returned for it. The query is run when the request arrives and may select no items
// @Description in Trash. When it now selects more than 500 items, the request fails with 422 bulk.too_many; this
// @Description check comes first, so a set that grew past 500 is a 422, not a 409. When it selects other items
// @Description within that limit (more, fewer, or as many with one swapped), the request fails with 409
// @Description bulk.selection_changed. Either way nothing changes.
// @Description
// @Description The response lists, per item, whether it changed (succeeded), no part of the patch changed it (skipped) or it could not be changed (failed, with a code and the reason).
// @Description To retry, send the same patch again with the failed ids as `ids`, in batches of at most 100, not the query: after the first run it may select other items, and it also selects the ones that succeeded.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.CommodityBulkUpdateRequest true "The items and the patch"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 400 {object} jsonapi.Errors "The query is not a valid item query, or selects items in Trash"
// @Failure 409 {object} jsonapi.Errors "bulk.selection_changed: the query selects at most 500 items, but other ones than expected_count and expected_digest describe"
// @Failure 422 {object} jsonapi.Errors "Bad request body or patch, or bulk.too_many: the query selects more than 500 items (checked before the 409)"
// @Router /g/{groupSlug}/commodities/bulk-update [post].
func (api *commoditiesAPI) bulkUpdateCommodities(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	// The item validation each change runs reads the group currency.
	rWithCurrency, err := requestWithGroupCurrency(r)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	var input jsonapi.CommodityBulkUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes

	patch, err := services.ParseCommodityPatch(r.Context(), commodityPatchInput(attrs.Patch))
	if err != nil {
		if isValidationError(err) {
			unprocessableEntityError(w, r, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	targets, ok := resolveCommoditySelection(w, r, regSet, &attrs.CommodityBulkSelection)
	if !ok {
		return
	}
	if err := api.bulkService.EnsureTags(r.Context(), &patch); err != nil {
		renderEntityError(w, r, err)
		return
	}
	renderBulkOutcome(w, r, "commodities", api.bulkService.Update(r.Context(), regSet.CommodityRegistry, targets, patch))
}

func commodityPatchInput(p *jsonapi.CommodityBulkPatch) services.CommodityPatchInput {
	in := services.CommodityPatchInput{Type: p.Type}
	if p.Tags != nil {
		in.AddTags = p.Tags.Add
		in.RemoveTags = p.Tags.Remove
	}
	if p.Status != nil {
		in.Status = &services.CommodityStatusInput{
			Value: p.Status.Value,
			Date:  p.Status.Date,
			Note:  p.Status.Note,
		}
	}
	return in
}

// resolveCommoditySelection returns the items sel names. When it cannot, it
// writes the error response and returns false.
func resolveCommoditySelection(w http.ResponseWriter, r *http.Request, regSet *registry.Set, sel *jsonapi.CommodityBulkSelection) ([]services.BulkTarget, bool) {
	selection := services.BulkSelection{IDs: sel.IDs}
	if sel.Query != nil {
		values, err := commodityquery.DecodeQuery(*sel.Query)
		if err != nil {
			queryParamError(w, r, err)
			return nil, false
		}
		query, err := commodityquery.Parse(values)
		if err != nil {
			queryParamError(w, r, err)
			return nil, false
		}
		selection.Query = &query
		selection.ExpectedCount = *sel.ExpectedCount
		selection.ExpectedDigest = *sel.ExpectedDigest
	}

	targets, err := services.ResolveBulkSelection(r.Context(), regSet, selection)
	if err == nil {
		return targets, true
	}
	if _, ok := errors.AsType[*commodityquery.Error](err); ok {
		queryParamError(w, r, err)
		return nil, false
	}
	if changed, ok := errors.AsType[*services.BulkSelectionChangedError](err); ok {
		codedConflictError(w, r, err, "bulk.selection_changed", map[string]any{
			"expected": strconv.Itoa(changed.Expected),
			"matched":  strconv.Itoa(changed.Matched),
		})
		return nil, false
	}
	if tooMany, ok := errors.AsType[*services.BulkTooManyError](err); ok {
		render.Render(w, r, jsonapi.NewErrors(jsonapi.Error{
			Err:            err,
			UserError:      errormarshal.Marshal(err),
			HTTPStatusCode: http.StatusUnprocessableEntity,
			StatusText:     "Unprocessable Entity",
			Code:           "bulk.too_many",
			Meta: map[string]any{
				"matched": strconv.Itoa(tooMany.Matched),
				"max":     strconv.Itoa(tooMany.Max),
			},
		}))
		return nil, false
	}
	renderEntityError(w, r, err)
	return nil, false
}

// renderBulkOutcome answers a bulk action on resource (commodities, files)
// with its outcome per entity.
func renderBulkOutcome(w http.ResponseWriter, r *http.Request, resource string, outcome services.BulkOutcome) {
	failed := make([]jsonapi.BulkResultFail, 0, len(outcome.Failed))
	for _, f := range outcome.Failed {
		failed = append(failed, bulkResultFail(r.Context(), resource, f))
	}
	var skipped []jsonapi.BulkResultSkip
	for _, s := range outcome.Skipped {
		skipped = append(skipped, jsonapi.BulkResultSkip{ID: s.ID, Name: s.Name, Reason: "unchanged"})
	}
	var notes []jsonapi.BulkResultNote
	for _, n := range outcome.Notes {
		notes = append(notes, jsonapi.BulkResultNote{ID: n.ID, Name: n.Name, Code: n.Code})
	}
	resp := jsonapi.NewBulkResultResponse(resource, outcome.Succeeded, failed).WithSkipped(skipped).WithNotes(notes)
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// bulkResultFail says why one entity of a bulk action failed. A validation
// message is meant for the user and is passed on; any other error is logged
// and reported without its internals.
func bulkResultFail(ctx context.Context, resource string, f services.BulkFailure) jsonapi.BulkResultFail {
	out := jsonapi.BulkResultFail{ID: f.ID, Name: f.Name}
	switch {
	case errors.Is(f.Err, registry.ErrNotFound):
		out.Code, out.Error = "not_found", "not found"
	case isValidationError(f.Err):
		out.Code, out.Error = "invalid", f.Err.Error()
		out.Field, out.Reason = validationFieldReason(f.Err)
	default:
		out.Code, out.Error = "failed", "could not be changed"
		slog.WarnContext(ctx, "bulk action: entity failed", "resource", resource, "id", f.ID, "err", f.Err)
	}
	return out
}

// validationFieldReason returns the first field a validation error names, in
// field order, and the validator's code for it. A nested validation.Errors
// gives its first field joined with a dot (for example extra.serial).
func validationFieldReason(err error) (field, reason string) {
	if errs, ok := errors.AsType[validation.Errors](err); ok {
		keys := slices.Sorted(maps.Keys(errs))
		for _, k := range keys {
			if errs[k] == nil {
				continue
			}
			sub, subReason := validationFieldReason(errs[k])
			if sub != "" {
				return k + "." + sub, subReason
			}
			return k, subReason
		}
		return "", ""
	}
	if ve, ok := errors.AsType[validation.Error](err); ok {
		return "", ve.Code()
	}
	return "", ""
}

func isValidationError(err error) bool {
	if _, ok := errors.AsType[validation.Errors](err); ok {
		return true
	}
	_, ok := errors.AsType[validation.Error](err)
	return ok
}
