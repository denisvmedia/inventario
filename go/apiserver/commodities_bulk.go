package apiserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
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
// @Description The items are given as ids (at most 100) or as a query of the item query contract with the number of items it was seen to select (at most 500); see POST /commodities/bulk-update for the selection rules.
// @Description The response lists succeeded and failed ids.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.CommodityBulkDeleteRequest true "The items to delete"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 400 {object} jsonapi.Errors "The query is not a valid item query, or selects items in Trash"
// @Failure 409 {object} jsonapi.Errors "bulk.selection_changed: the query selects a different number of items than expected_count"
// @Failure 422 {object} jsonapi.Errors "Bad request body, or bulk.too_many: the query selects more than 500 items"
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

	ids, ok := resolveCommoditySelection(w, r, regSet, input.Data.Attributes)
	if !ok {
		return
	}
	renderBulkOutcome(w, r, api.bulkService.Delete(r.Context(), ids))
}

// bulkMoveCommodities puts several items into one area.
// @Summary Bulk-move commodities to a new area
// @Description Put the selected items into the area area_id. An item already in that area is reported as skipped. Each moved item gets a moved event in its history.
// @Description The items are given as ids (at most 100) or as a query of the item query contract with the number of items it was seen to select (at most 500); see POST /commodities/bulk-update for the selection rules.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkMoveRequest true "The items and the destination area_id"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 400 {object} jsonapi.Errors "The query is not a valid item query, or selects items in Trash"
// @Failure 409 {object} jsonapi.Errors "bulk.selection_changed: the query selects a different number of items than expected_count"
// @Failure 422 {object} jsonapi.Errors "Bad request body, or bulk.too_many: the query selects more than 500 items"
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

	ids, ok := resolveCommoditySelection(w, r, regSet, &input.Data.Attributes.CommodityBulkSelection)
	if !ok {
		return
	}
	outcome := api.bulkService.Move(r.Context(), regSet.CommodityRegistry, ids, input.Data.Attributes.AreaID)
	renderBulkOutcome(w, r, outcome)
}

// bulkUpdateCommodities changes the tags, type or status of several items.
// @Summary Bulk-update commodities
// @Description Apply one patch to the selected items. Each item is changed and validated as PUT /commodities/{id} changes it, and gets the same history events (a status change event for a status, an updated event for tags and type).
// @Description
// @Description The patch: `tags.add` and `tags.remove` add and remove tags without touching the others
// @Description (`add` takes slugs or the text of a new tag, at most 50 per list); `type` sets the type;
// @Description `status` sets the status with `date` (required for every status but in_use) and an optional `note`.
// @Description `in_use` clears the status date and note. `sold` is refused, since it needs a sale price for each item.
// @Description A part left out of the patch is left unchanged.
// @Description
// @Description The items are given as `ids` (at most 100) or as `query`, a query string of the item query contract
// @Description (the filter parameters of GET /commodities, without page and per_page), with `expected_count`,
// @Description the number of items the caller saw it select. The query is run when the request arrives; when it
// @Description selects a different number of items the request fails with 409 and changes nothing, and it may
// @Description select at most 500 items and no items in Trash.
// @Description
// @Description The response lists, per item, whether it changed (succeeded), already had what the patch sets (skipped) or could not be changed (failed, with a code and the reason). A retry can send the failed ids alone.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.CommodityBulkUpdateRequest true "The items and the patch"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 400 {object} jsonapi.Errors "The query is not a valid item query, or selects items in Trash"
// @Failure 409 {object} jsonapi.Errors "bulk.selection_changed: the query selects a different number of items than expected_count"
// @Failure 422 {object} jsonapi.Errors "Bad request body or patch, or bulk.too_many: the query selects more than 500 items"
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

	ids, ok := resolveCommoditySelection(w, r, regSet, &attrs.CommodityBulkSelection)
	if !ok {
		return
	}
	if err := api.bulkService.EnsureTags(r.Context(), &patch); err != nil {
		renderEntityError(w, r, err)
		return
	}
	renderBulkOutcome(w, r, api.bulkService.Update(r.Context(), regSet.CommodityRegistry, ids, patch))
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

// resolveCommoditySelection returns the ids sel names. When it cannot, it
// writes the error response and returns false.
func resolveCommoditySelection(w http.ResponseWriter, r *http.Request, regSet *registry.Set, sel *jsonapi.CommodityBulkSelection) ([]string, bool) {
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
	}

	ids, err := services.ResolveBulkSelection(r.Context(), regSet, selection)
	if err == nil {
		return ids, true
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

func renderBulkOutcome(w http.ResponseWriter, r *http.Request, outcome services.BulkOutcome) {
	failed := make([]jsonapi.BulkResultFail, 0, len(outcome.Failed))
	for _, f := range outcome.Failed {
		failed = append(failed, bulkResultFail(r.Context(), f))
	}
	var skipped []jsonapi.BulkResultSkip
	for _, s := range outcome.Skipped {
		skipped = append(skipped, jsonapi.BulkResultSkip{ID: s.ID, Name: s.Name, Reason: "unchanged"})
	}
	resp := jsonapi.NewBulkResultResponse("commodities", outcome.Succeeded, failed).WithSkipped(skipped)
	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
	}
}

// bulkResultFail says why one item of a bulk action failed. A validation
// message is meant for the user and is passed on; any other error is logged
// and reported without its internals.
func bulkResultFail(ctx context.Context, f services.BulkFailure) jsonapi.BulkResultFail {
	out := jsonapi.BulkResultFail{ID: f.ID, Name: f.Name}
	switch {
	case errors.Is(f.Err, registry.ErrNotFound):
		out.Code, out.Error = "not_found", "not found"
	case isValidationError(f.Err):
		out.Code, out.Error = "invalid", f.Err.Error()
	default:
		out.Code, out.Error = "failed", "could not be changed"
		slog.WarnContext(ctx, "bulk action: item failed", "commodity_id", f.ID, "err", f.Err)
	}
	return out
}

func isValidationError(err error) bool {
	if _, ok := errors.AsType[validation.Errors](err); ok {
		return true
	}
	_, ok := errors.AsType[validation.Error](err)
	return ok
}
