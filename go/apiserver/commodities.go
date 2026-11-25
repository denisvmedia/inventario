package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
	"go.5x5.cz/inventario/services/commodityquery"
)

type commoditiesAPI struct {
	entityService *services.EntityService
	tagService    *services.TagService
	coverService  *services.CommodityCoverService
	eventService  *services.CommodityEventService
	factorySet    *registry.FactorySet
}

// listCommodities lists the commodities a query selects, one page at a time.
// The filter parameters are the item query contract (services/commodityquery,
// devdocs/item-query-contract.md); an unknown parameter or an unreadable
// value is a 400, never a silently dropped condition.
// @Summary List commodities
// @Description get commodities
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Param type query []string false "Filter by commodity type; repeat to OR" collectionFormat(multi)
// @Param status query []string false "Filter by status (in_use, sold, lost, disposed, written_off); repeat to OR" collectionFormat(multi)
// @Param area_id query string false "Filter by exact area ID"
// @Param unassigned query bool false "Only commodities with no area (ignored when area_id is set)"
// @Param q query string false "Search text: space-separated terms, quotes keep a phrase; every term must match the name, short name, a serial or part number, a tag, a note or the asset ID. At most 200 characters and 10 terms"
// @Param tags query []string false "Only items with these tag slugs (comma-separated or repeated); see tag_mode" collectionFormat(multi)
// @Param tag_mode query string false "How tags combine: all (default) or any" Enums(all,any)
// @Param tags_exclude query []string false "Only items with none of these tag slugs (comma-separated or repeated)" collectionFormat(multi)
// @Param no_tags query bool false "Only items with no tags; cannot be combined with tags or tags_exclude"
// @Param include_inactive query bool false "Include drafts and non-in_use commodities (default true; false hides them)"
// @Param sort query string false "Sort field — name|registered_date|purchase_date|current_price|original_price|count, prefix with '-' for descending"
// @Param warranty_status query []string false "Filter by computed warranty status (active, expiring, expired, none); repeat to OR" collectionFormat(multi)
// @Param warranty_expires_before query string false "Restrict to commodities whose warranty expires strictly before YYYY-MM-DD"
// @Param lent_out query bool false "Filter by current loan state: true = only currently lent (open loan), false = only currently not-lent"
// @Success 200 {object} jsonapi.CommoditiesResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Unsupported parameter or invalid value"
// @Router /g/{groupSlug}/commodities [get].
func (api *commoditiesAPI) listCommodities(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	values, err := commodityquery.DecodeQuery(r.URL.RawQuery)
	if err != nil {
		queryParamError(w, r, err)
		return
	}
	page, perPage := parsePagination(values.Get("page"), values.Get("per_page"))
	offset := (page - 1) * perPage

	query, err := commodityquery.Parse(values, "page", "per_page")
	if err != nil {
		queryParamError(w, r, err)
		return
	}

	result, err := commodityquery.List(r.Context(), regSet, query, offset, perPage)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, result.Total)

	covers := api.resolveCoversForList(r, regSet.FileRegistry, result.Commodities)

	resp := jsonapi.NewCommoditiesResponseWithCovers(result.Commodities, result.Total, page, perPage, covers).
		WithMatches(result.Matches)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// resolveCoversForList wraps coverService.ResolveMany with the page of
// commodities and the per-request user, returning the JSON:API-shaped
// map that NewCommoditiesResponseWithCovers expects. Returns nil (not an
// empty map) when no covers resolve so the response shape stays clean.
func (api *commoditiesAPI) resolveCoversForList(r *http.Request, fileReg registry.FileRegistry, commodities []*models.Commodity) map[string]jsonapi.CommodityCover {
	if api.coverService == nil || len(commodities) == 0 {
		return nil
	}
	ctx := r.Context()
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return nil
	}
	resolved := api.coverService.ResolveMany(ctx, fileReg, commodities, user.ID, services.ExtractSessionBinding(r))
	if len(resolved) == 0 {
		return nil
	}
	out := make(map[string]jsonapi.CommodityCover, len(resolved))
	for id, cov := range resolved {
		out[id] = jsonapi.CommodityCover{
			FileID:     cov.FileID,
			Thumbnails: cov.Thumbnails,
			Source:     string(cov.Source),
		}
	}
	return out
}

// getCommodity gets a commodity by ID.
// @Summary Get a commodity
// @Description get commodity by ID
// @Tags commodities
// @Accept  json-api
// @Produce  json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Router /g/{groupSlug}/commodities/{commodityID} [get].
func (api *commoditiesAPI) getCommodity(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	resp := jsonapi.NewCommodityResponse(commodity).WithStatusCode(http.StatusOK)
	if cover := api.resolveCoverForOne(r, commodity); cover != nil {
		resp = resp.WithCover(cover)
	}

	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// resolveCoverForOne is the single-commodity counterpart to
// resolveCoversForList. Returns nil when the commodity has no usable
// photo, or the user context is missing, or signing fails — every path
// the FE handles via the type-emoji fallback.
func (api *commoditiesAPI) resolveCoverForOne(r *http.Request, commodity *models.Commodity) *jsonapi.CommodityCover {
	if api.coverService == nil || commodity == nil || commodity.ID == "" {
		return nil
	}
	ctx := r.Context()
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return nil
	}
	regSet := RegistrySetFromContext(ctx)
	if regSet == nil {
		return nil
	}
	cov, ok := api.coverService.ResolveOne(ctx, regSet.FileRegistry, commodity, user.ID, services.ExtractSessionBinding(r))
	if !ok {
		return nil
	}
	return &jsonapi.CommodityCover{
		FileID:     cov.FileID,
		Thumbnails: cov.Thumbnails,
		Source:     string(cov.Source),
	}
}

// createCommodity creates a new commodity.
// @Summary Create a new commodity
// @Description Add a new commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodity body jsonapi.CommodityRequest true "Commodity object"
// @Success 201 {object} jsonapi.CommodityResponse "Commodity created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities [post].
func (api *commoditiesAPI) createCommodity(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.CommodityRequest

	rWithCurrency, err := requestWithGroupCurrency(r)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Use standardized security validation
	user, ok := RequireUserContext(w, r)
	if !ok {
		return // Error already handled by RequireUserContext
	}

	// Validate input
	if secErr := ValidateInputSanitization(r, input.Data.Attributes); secErr != nil {
		HandleSecurityError(w, r, secErr)
		return
	}

	commodity := *input.Data.Attributes
	if commodity.TenantID == "" {
		commodity.TenantID = user.TenantID
	}

	// Auto-create any tags the user typed but that don't yet exist as
	// first-class rows; replace the JSONB list with the canonical slugs.
	if len(commodity.Tags) > 0 {
		slugs, terr := api.tagService.NormalizeAndEnsureSlugs(r.Context(), models.TagKindCommodity, []string(commodity.Tags))
		if terr != nil {
			renderEntityError(w, r, terr)
			return
		}
		commodity.Tags = models.ValuerSlice[string](slugs)
	}

	// Use standardized registry access
	commodityReg := registrySet.CommodityRegistry

	createdCommodity, err := commodityReg.Create(r.Context(), commodity)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// #1450: append-only audit row. Errors are logged inside the service —
	// a failed event must not 500 a successful create.
	api.eventService.EmitCreated(r.Context(), createdCommodity)

	resp := jsonapi.NewCommodityResponse(createdCommodity).WithStatusCode(http.StatusCreated)

	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteCommodity deletes a commodity by ID.
// @Summary Delete a commodity
// @Description Delete a commodity by ID and all its linked files
// @Tags commodities
// @Accept  json-api
// @Produce  json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Router /g/{groupSlug}/commodities/{commodityID} [delete].
func (api *commoditiesAPI) deleteCommodity(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	// #1450: emit the audit row BEFORE the delete so the FK CASCADE on
	// commodity_events doesn't drop the new event the same instant we
	// write it. Within the same request the timeline is observable; the
	// CASCADE then cleans up alongside the commodity row.
	//
	// Tradeoff: DeleteCommodityRecursive also clears blob storage and
	// can fail mid-flight after the audit row commits. In that rare
	// case we'd be left with a "deleted" event for a still-existing
	// commodity. Wrapping both into a single transaction is impractical
	// (the file delete touches non-transactional blob backends), and
	// emitting after the delete reintroduces the FK CASCADE problem
	// above. The next call attempt will produce a fresh "deleted" event
	// and the FK CASCADE wipes both on the eventual successful delete —
	// the timeline self-heals on retry.
	api.eventService.EmitDeleted(r.Context(), commodity)

	err := api.entityService.DeleteCommodityRecursive(r.Context(), commodity.ID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// validateForwardStatusTransition enforces the #1611 cross-field rule
// that needs the previous row's status (which the model layer doesn't
// carry): leaving in_use requires status_date, and marking as sold
// additionally requires sale_price. Returns nil when the transition is
// not "leaving in_use" or all required fields are present. Per-row
// invariants (sale_price only when sold, status_date only when not
// in_use) stay at the model layer via ValidateWithContext.
func validateForwardStatusTransition(prev, next models.CommodityStatus, statusDate models.PDate, salePrice *decimal.Decimal) error {
	if prev != models.CommodityStatusInUse || next == models.CommodityStatusInUse {
		return nil
	}
	if statusDate == nil || string(*statusDate) == "" {
		return validation.Errors{
			"status_date": validation.NewError("status_date_required_on_transition",
				"status date is required when leaving in_use"),
		}
	}
	if next == models.CommodityStatusSold && salePrice == nil {
		return validation.Errors{
			"sale_price": validation.NewError("sale_price_required_for_sold",
				"sale price is required when marking as sold"),
		}
	}
	return nil
}

// updateCommodity updates a commodity.
// @Summary Update a commodity
// @Description Update a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param commodity body jsonapi.CommodityRequest true "Commodity object"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID} [put].
func (api *commoditiesAPI) updateCommodity(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	rWithCurrency, err := requestWithGroupCurrency(r)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	var input jsonapi.CommodityRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if commodity.ID != input.Data.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}

	input.Data.Attributes.ID = input.Data.ID

	// Preserve tenant_id and user_id from the existing commodity
	// This ensures the foreign key constraints are satisfied during updates
	updateData := *input.Data.Attributes
	if updateData.TenantID == "" {
		updateData.TenantID = commodity.TenantID
	}

	if len(updateData.Tags) > 0 {
		slugs, terr := api.tagService.NormalizeAndEnsureSlugs(r.Context(), models.TagKindCommodity, []string(updateData.Tags))
		if terr != nil {
			renderEntityError(w, r, terr)
			return
		}
		updateData.Tags = models.ValuerSlice[string](slugs)
	}

	// #1611: terminal-status metadata. The cross-field rule "status_date
	// is required whenever the transition leaves in_use" depends on the
	// previous row's status — which the model layer doesn't carry — so
	// it's enforced here. Per-row invariants (sale_price only when
	// status=sold, status_date only when status != in_use) are enforced
	// at the model layer via ValidateWithContext, and the FE is
	// responsible for sending a clean payload on revert (clearing the
	// three metadata fields when flipping back to in_use). Pre-existing
	// terminal rows with NULL metadata stay valid on edits that don't
	// change the status.
	if err := validateForwardStatusTransition(commodity.Status, updateData.Status, updateData.StatusDate, updateData.SalePrice); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// #1554: a 1 → >1 quantity bump must be blocked when the row still
	// carries per-instance state (warranty fields / open loan / open
	// service). The model-level validation handles a fresh count>1 row
	// with warranty fields, but the cross-table loan/service check
	// needs registry access — done here. Returns a multi-error 422 so
	// the FE can list every blocker on the same submit.
	if commodity.Count == 1 && updateData.Count > 1 {
		blockers, berr := services.CheckQuantityBumpBlockers(r.Context(), api.factorySet, commodity)
		if berr != nil {
			renderEntityError(w, r, berr)
			return
		}
		if len(blockers) > 0 {
			renderQuantityBumpBlockers(w, r, blockers)
			return
		}
	}

	// Use UpdateWithUser to ensure proper user context and validation
	ctx := r.Context()
	commodityReg := registrySet.CommodityRegistry
	updatedCommodity, err := commodityReg.Update(ctx, updateData)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// #1450: emit one or more audit rows depending on what changed.
	// Multiple events emit when status / area / price / cover shift in the
	// same write — see CommodityEventService.EmitUpdated for the diff
	// rules.
	api.eventService.EmitUpdated(ctx, commodity, updatedCommodity)

	resp := jsonapi.NewCommodityResponse(updatedCommodity).WithStatusCode(http.StatusOK)

	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// setCommodityCover sets or clears the explicit cover-photo override
// for a commodity (issue #1451 option B). The body's `attributes.file_id`
// is either:
//
//   - a non-empty string — must reference a file that is linked to this
//     commodity and is an image. The handler validates
//     and persists `commodities.cover_file_id`.
//   - `null` or empty — clears the override. The first-photo path takes
//     over on the next read.
//
// The success response is the updated commodity envelope with
// `meta.cover` recomputed (so the FE doesn't have to re-fetch).
//
// @Summary Set or clear the commodity cover photo
// @Description Sets the explicit cover-photo override (issue #1451 option B)
// @Description or clears it when `attributes.file_id` is null/empty. The
// @Description file must already be attached to this commodity and be an
// @Description image.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param body body jsonapi.CommodityCoverRequest true "Cover photo file id (null to clear)"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or file not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/cover [patch].
func (api *commoditiesAPI) setCommodityCover(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	rWithCurrency, err := requestWithGroupCurrency(r)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	var input jsonapi.CommodityCoverRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// `null` and "" both clear the override. Treat any all-whitespace
	// input as empty, too — the BE validator on the model expects exact
	// matches and the FE never sends padded ids on purpose.
	desired := ""
	if input.Data.Attributes.FileID != nil {
		desired = strings.TrimSpace(*input.Data.Attributes.FileID)
	}

	if desired != "" {
		if err := api.validateCoverFile(r.Context(), registrySet.FileRegistry, commodity.ID, desired); err != nil {
			// Two-track error mapping (Copilot review on #1504):
			//
			//   - `validation.Errors` → 422 (user-facing input issue;
			//     `renderEntityError` would otherwise drop these into
			//     `toJSONAPIError`'s default branch and surface 500).
			//   - Anything else (registry.ErrNotFound, transient registry
			//     failures, …) → renderEntityError, which maps NotFound to
			//     404 per the swagger contract and unknown errors to 500
			//     instead of masking them as user input problems.
			if _, ok := errors.AsType[validation.Errors](err); ok {
				unprocessableEntityError(w, r, err)
				return
			}
			renderEntityError(w, r, err)
			return
		}
	}

	updateData := *commodity
	if desired == "" {
		updateData.CoverFileID = nil
	} else {
		v := desired
		updateData.CoverFileID = &v
	}

	updated, err := registrySet.CommodityRegistry.Update(r.Context(), updateData)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// #1450: cover override change is a distinct event kind so the timeline
	// can render "set / cleared cover photo" without aliasing it to the
	// generic "updated" row.
	api.eventService.EmitUpdated(r.Context(), commodity, updated)

	resp := jsonapi.NewCommodityResponse(updated).WithStatusCode(http.StatusOK)
	if cover := api.resolveCoverForOne(r, updated); cover != nil {
		resp = resp.WithCover(cover)
	}

	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// validateCoverFile rejects file ids that don't satisfy the cover
// invariants: must exist, must already belong to this commodity, must
// be an image. Mirrors the same checks the resolver applies on read so
// a stale override never lands in the DB in the first place.
func (api *commoditiesAPI) validateCoverFile(ctx context.Context, fileReg registry.FileRegistry, commodityID, fileID string) error {
	file, err := fileReg.Get(ctx, fileID)
	if err != nil {
		// Surface registry errors as-is so the PATCH handler can map
		// `ErrNotFound` to 404 (per the swagger contract) and any
		// other registry failure to 500 via renderEntityError. A
		// blanket `registry.ErrNotFound` would mask transient errors.
		return err
	}
	if file == nil || file.File == nil {
		return registry.ErrNotFound
	}
	if file.LinkTo(models.FileLinkCommodity, commodityID) == nil {
		return validationError("file_id", "file is not attached to this commodity")
	}
	// Cover photos live in the `images` bucket only — both `Type=image`
	// (MIME-derived, drives thumbnailing) and `Category=images` (user-
	// meaningful classification) must hold. Without the category check
	// a JPEG mis-uploaded as `category=invoices` could be set as the
	// cover (Copilot review on PR #1504).
	if file.Type != models.FileTypeImage {
		return validationError("file_id", "file is not an image")
	}
	if file.Category != models.FileCategoryImages {
		return validationError("file_id", "file is not categorised as an image")
	}
	return nil
}

// validationError builds a jellydator-validation error for a single
// field. Matches the error shape every other commodity handler returns
// so renderEntityError surfaces a 422 with consistent JSON-API output.
func validationError(field, msg string) error {
	return validation.Errors{field: validation.NewError("invalid", msg)}
}

// Legacy commodity-scoped file routes (`/commodities/{id}/{images,invoices,manuals}*`)
// were removed under #1421. The unified `/files` surface (#1411) covers
// every read via `?linked_entity_type=commodity&linked_entity_id={id}`
// and detail / update / delete via `/files/{id}`. New uploads go through
// `POST /uploads/file` (multipart, creates an unlinked FileEntity) and
// then `POST /files/{id}/links` to attach the resulting file to the item —
// `/uploads/file` itself does not parse link query params. The FE was migrated in #1476. The legacy
// `images` / `invoices` / `manuals` SQL tables stay until ops run the
// #1399 backfill in production; that drop is a separate follow-up.

// bulkDeleteCommodities deletes a list of commodities in a single request.
// @Summary Bulk-delete commodities
// @Description Delete every commodity whose id appears in the body. The
// @Description response lists succeeded vs. failed ids so the frontend
// @Description can render partial-failure UX without parsing per-id HTTP
// @Description statuses.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkIDsRequest true "List of commodity IDs to delete"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /g/{groupSlug}/commodities/bulk-delete [post].
func (api *commoditiesAPI) bulkDeleteCommodities(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.BulkIDsRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	succeeded := make([]string, 0, len(input.Data.Attributes.IDs))
	failed := make([]jsonapi.BulkResultFail, 0)
	for _, id := range input.Data.Attributes.IDs {
		// #1450: emit the audit row BEFORE the delete. Two reasons:
		//   - commodity_events.commodity_id has FK ON DELETE CASCADE.
		//     Writing the row after the commodity is gone would either
		//     fail with a FK violation (if the FK is verified at insert
		//     time) or create an orphan reference. Inserting first
		//     keeps the FK happy: the row is observable for the rest
		//     of the request, and the cascade wipes it when the
		//     subsequent commodity DELETE commits.
		//   - Mirrors the single-delete handler so the timeline shape
		//     is identical for "delete one" vs "delete N at once".
		// Get-failure is non-fatal: if the row is already gone, the
		// delete below will surface the canonical error and we just
		// skip the audit row for that id.
		if before, getErr := registrySet.CommodityRegistry.Get(r.Context(), id); getErr == nil {
			api.eventService.EmitDeleted(r.Context(), before)
		}
		if err := api.entityService.DeleteCommodityRecursive(r.Context(), id); err != nil {
			failed = append(failed, jsonapi.BulkResultFail{ID: id, Error: err.Error()})
			continue
		}
		succeeded = append(succeeded, id)
	}

	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, jsonapi.NewBulkResultResponse("commodities", succeeded, failed)); err != nil {
		internalServerError(w, r, err)
	}
}

// bulkMoveCommodities reassigns a list of commodities to a new area in
// a single request.
// @Summary Bulk-move commodities to a new area
// @Description Update every commodity whose id appears in the body to
// @Description belong to the supplied area_id.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param body body jsonapi.BulkMoveRequest true "List of commodity IDs and the destination area_id"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /g/{groupSlug}/commodities/bulk-move [post].
func (api *commoditiesAPI) bulkMoveCommodities(w http.ResponseWriter, r *http.Request) {
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.BulkMoveRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	succeeded := make([]string, 0, len(input.Data.Attributes.IDs))
	failed := make([]jsonapi.BulkResultFail, 0)
	for _, id := range input.Data.Attributes.IDs {
		commodity, err := registrySet.CommodityRegistry.Get(r.Context(), id)
		if err != nil {
			failed = append(failed, jsonapi.BulkResultFail{ID: id, Error: err.Error()})
			continue
		}
		before := *commodity
		// BulkMove always targets a concrete area (required & non-empty,
		// validated at jsonapi/bulk.go). AreaID is now a *string on the
		// model, so take the address of the wire value.
		aid := input.Data.Attributes.AreaID
		commodity.AreaID = &aid
		updated, err := registrySet.CommodityRegistry.Update(r.Context(), *commodity)
		if err != nil {
			failed = append(failed, jsonapi.BulkResultFail{ID: id, Error: err.Error()})
			continue
		}
		// #1450: emit one event per affected commodity (per the issue's
		// "Bulk-move / bulk-delete emit one event per affected commodity"
		// requirement). EmitUpdated detects the area change and produces
		// a `moved` row.
		api.eventService.EmitUpdated(r.Context(), &before, updated)
		succeeded = append(succeeded, id)
	}

	render.Status(r, http.StatusOK)
	if err := render.Render(w, r, jsonapi.NewBulkResultResponse("commodities", succeeded, failed)); err != nil {
		internalServerError(w, r, err)
	}
}

func Commodities(params Params) func(r chi.Router) {
	fileSigningService := services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration)
	api := &commoditiesAPI{
		entityService: params.EntityService,
		tagService:    services.NewTagService(params.FactorySet),
		coverService:  services.NewCommodityCoverService(fileSigningService),
		eventService:  services.NewCommodityEventService(params.FactorySet),
		factorySet:    params.FactorySet,
	}

	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listCommodities) // GET /commodities
		// Bulk endpoints (#1330 PR 5.5). Mounted before `/{commodityID}`
		// so chi's static-vs-param-route matcher routes `/bulk-delete`
		// and `/bulk-move` here rather than treating those slugs as a
		// commodity id.
		r.Post("/bulk-delete", api.bulkDeleteCommodities) // POST /commodities/bulk-delete
		r.Post("/bulk-move", api.bulkMoveCommodities)     // POST /commodities/bulk-move
		r.Route("/{commodityID}", func(r chi.Router) {
			r.Use(commodityCtx())
			r.Get("/", api.getCommodity)                          // GET /commodities/123
			r.Put("/", api.updateCommodity)                       // PUT /commodities/123
			r.Delete("/", api.deleteCommodity)                    // DELETE /commodities/123
			r.Patch("/cover", api.setCommodityCover)              // PATCH /commodities/123/cover
			r.Route("/loans", CommodityLoans(params))             // /commodities/123/loans (#1452)
			r.Route("/services", CommodityServices(params))       // /commodities/123/services (#1508)
			r.Route("/supplies", CommoditySupplyLinks(params))    // /commodities/123/supplies (#1369)
			r.Route("/maintenance", CommodityMaintenance(params)) // /commodities/123/maintenance (#1368)
			r.Route("/reminders", CommodityReminders(params))     // /commodities/123/reminders (#2805)
			// #1450: append-only audit timeline.
			r.With(paginate).Get("/events", api.listCommodityEvents) // GET /commodities/123/events

			// Legacy commodity-scoped file routes were removed under
			// #1421. Use `/files?linked_entity_type=commodity&linked_entity_id=…`
			// for read, `/files/{id}` for detail / update / delete, and
			// `POST /uploads/file` (multipart, unlinked) followed by
			// `PUT /files/{id}` to set `linked_entity_*` for new uploads.
		})
		r.Post("/", api.createCommodity) // POST /commodities
	}
}

func requestWithGroupCurrency(r *http.Request) (*http.Request, error) {
	group := appctx.GroupFromContext(r.Context())
	if group == nil || group.GroupCurrency == "" {
		return nil, registry.ErrGroupCurrencyNotSet
	}

	ctx := validationctx.WithGroupCurrency(r.Context(), string(group.GroupCurrency))

	return r.WithContext(ctx), nil
}
