package apiserver

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

type commoditiesAPI struct {
	entityService *services.EntityService
	tagService    *services.TagService
	coverService  *services.CommodityCoverService
	eventService  *services.CommodityEventService
	factorySet    *registry.FactorySet
}

// listCommodities lists all commodities with pagination, filters, and sort.
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
// @Param q query string false "Case-insensitive substring match on name + short_name"
// @Param include_inactive query bool false "Include drafts and non-in_use commodities (default false hides them)"
// @Param sort query string false "Sort field — name|registered_date|purchase_date|current_price|original_price|count, prefix with '-' for descending"
// @Param warranty_status query []string false "Filter by computed warranty status (active, expiring, expired, none); repeat to OR" collectionFormat(multi)
// @Param warranty_expires_before query string false "Restrict to commodities whose warranty expires strictly before YYYY-MM-DD"
// @Param lent_out query bool false "Filter by current loan state: true = only currently lent (open loan), false = only currently not-lent"
// @Success 200 {object} jsonapi.CommoditiesResponse "OK"
// @Router /g/{groupSlug}/commodities [get].
func (api *commoditiesAPI) listCommodities(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	commodityReg := regSet.CommodityRegistry

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	opts := parseCommodityListOptions(q)

	// Pre-resolve the open-loan commodity ID set for the lent_out filter
	// — but ONLY for backends that don't already resolve LentOut natively
	// (postgres joins commodity_loans inline via an EXISTS subquery and
	// implements registry.NativeLentOutFilterer). Skipping the pre-fetch
	// on postgres saves the extra count+list queries on every filtered
	// commodities request; the memory backend (without the marker) still
	// needs the pre-resolved set to evaluate membership without reaching
	// back into CommodityLoanRegistry.
	if opts.LentOut != nil && regSet.CommodityLoanRegistry != nil {
		if _, native := commodityReg.(registry.NativeLentOutFilterer); !native {
			ids, err := listOpenLoanCommodityIDs(r.Context(), regSet.CommodityLoanRegistry)
			if err != nil {
				internalServerError(w, r, err)
				return
			}
			opts.OpenLoanCommodityIDs = ids
		}
	}

	commodities, total, err := commodityReg.ListPaginated(r.Context(), offset, perPage, opts)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)

	covers := api.resolveCoversForList(r.Context(), regSet.FileRegistry, commodities)

	if err := render.Render(w, r, jsonapi.NewCommoditiesResponseWithCovers(commodities, total, page, perPage, covers)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// resolveCoversForList wraps coverService.ResolveMany with the page of
// commodities and the per-request user, returning the JSON:API-shaped
// map that NewCommoditiesResponseWithCovers expects. Returns nil (not an
// empty map) when no covers resolve so the response shape stays clean.
func (api *commoditiesAPI) resolveCoversForList(ctx context.Context, fileReg registry.FileRegistry, commodities []*models.Commodity) map[string]jsonapi.CommodityCover {
	if api.coverService == nil || len(commodities) == 0 {
		return nil
	}
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return nil
	}
	resolved := api.coverService.ResolveMany(ctx, fileReg, commodities, user.ID)
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

// parseCommodityListOptions extracts filter/sort args from the query
// string. Unknown sort fields fall back silently to name-ascending —
// see registry.CommoditySortField.IsValid for why we don't 4xx.
//
// `IncludeInactive` defaults to true (= no implicit filter) so legacy
// FE clients keep seeing all commodities. The new React list page opts
// IN to the active-only view by sending `include_inactive=false`.
func parseCommodityListOptions(q url.Values) registry.CommodityListOptions {
	opts := registry.CommodityListOptions{
		AreaID:          strings.TrimSpace(q.Get("area_id")),
		Search:          strings.TrimSpace(q.Get("q")),
		IncludeInactive: true,
	}
	if v := strings.TrimSpace(q.Get("include_inactive")); v != "" {
		opts.IncludeInactive = v == "1" || strings.EqualFold(v, "true")
	}
	for _, t := range q["type"] {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		opts.Types = append(opts.Types, models.CommodityType(t))
	}
	for _, s := range q["status"] {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		opts.Statuses = append(opts.Statuses, models.CommodityStatus(s))
	}
	if sort := strings.TrimSpace(q.Get("sort")); sort != "" {
		desc := strings.HasPrefix(sort, "-")
		field := strings.TrimPrefix(sort, "-")
		opts.SortField = registry.CommoditySortField(field)
		opts.SortDesc = desc
	}
	for _, s := range q["warranty_status"] {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		filter := registry.WarrantyStatusFilter(s)
		if !filter.IsValid() {
			// Unknown values silently dropped — stay consistent with the
			// CommoditySortField precedent (FE multi-version rollout).
			continue
		}
		opts.WarrantyStatuses = append(opts.WarrantyStatuses, filter)
	}
	if v := strings.TrimSpace(q.Get("warranty_expires_before")); v != "" {
		opts.WarrantyExpiresBefore = v
	}
	if v := strings.TrimSpace(q.Get("lent_out")); v != "" {
		// Mirror include_inactive's parser ("1" or case-insensitive "true"
		// is true; everything else is false). Presence alone activates the
		// filter — empty/missing leaves opts.LentOut nil (no filter).
		b := v == "1" || strings.EqualFold(v, "true")
		opts.LentOut = &b
	}
	return opts
}

// listOpenLoanCommodityIDs collects every commodity ID in the current
// group that has at least one open loan (a commodity_loans row with
// `returned_at IS NULL`). Pages through CommodityLoanRegistry until the
// total is exhausted so the filter stays correct even when a group
// crosses a single page worth of open loans. The partial index
// `idx_commodity_loans_active` keeps each page cheap.
func listOpenLoanCommodityIDs(ctx context.Context, loanReg registry.CommodityLoanRegistry) ([]string, error) {
	const pageSize = 1000
	seen := make(map[string]struct{})
	var ids []string
	offset := 0
	for {
		loans, total, err := loanReg.ListPaginated(ctx, offset, pageSize, registry.LoanListOptions{State: registry.LoanStateOpen})
		if err != nil {
			return nil, err
		}
		for _, l := range loans {
			if l == nil {
				continue
			}
			if _, dup := seen[l.CommodityID]; dup {
				continue
			}
			seen[l.CommodityID] = struct{}{}
			ids = append(ids, l.CommodityID)
		}
		offset += len(loans)
		// Defensive break: an empty page or reaching the reported total
		// both stop the loop. The empty-page guard avoids a hot loop if
		// a backend ever returns total > rows-available (would only
		// happen with a buggy registry, but the cost of the guard is
		// nothing and the cost of an infinite loop is everything).
		if len(loans) == 0 || offset >= total {
			break
		}
	}
	return ids, nil
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
	if cover := api.resolveCoverForOne(r.Context(), commodity); cover != nil {
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
func (api *commoditiesAPI) resolveCoverForOne(ctx context.Context, commodity *models.Commodity) *jsonapi.CommodityCover {
	if api.coverService == nil || commodity == nil || commodity.ID == "" {
		return nil
	}
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return nil
	}
	regSet := RegistrySetFromContext(ctx)
	if regSet == nil {
		return nil
	}
	cov, ok := api.coverService.ResolveOne(ctx, regSet.FileRegistry, commodity, user.ID)
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
		slugs, terr := api.tagService.NormalizeAndEnsureSlugs(r.Context(), []string(commodity.Tags))
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
		slugs, terr := api.tagService.NormalizeAndEnsureSlugs(r.Context(), []string(updateData.Tags))
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
//   - a non-empty string — must reference a file that already belongs to
//     this commodity (`linked_entity_type=commodity` /
//     `linked_entity_id=<this>`) and is an image. The handler validates
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
			var verrs validation.Errors
			if errors.As(err, &verrs) {
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
	if cover := api.resolveCoverForOne(r.Context(), updated); cover != nil {
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
	if file.LinkedEntityType != "commodity" || file.LinkedEntityID != commodityID {
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
// then `PUT /files/{id}` to set `linked_entity_type` / `linked_entity_id`
// on the resulting row — `/uploads/file` itself does not parse those
// query params. The FE was migrated in #1476. The legacy
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
		commodity.AreaID = input.Data.Attributes.AreaID
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
			r.Get("/", api.getCommodity)                       // GET /commodities/123
			r.Put("/", api.updateCommodity)                    // PUT /commodities/123
			r.Delete("/", api.deleteCommodity)                 // DELETE /commodities/123
			r.Patch("/cover", api.setCommodityCover)           // PATCH /commodities/123/cover
			r.Route("/loans", CommodityLoans(params))          // /commodities/123/loans (#1452)
			r.Route("/services", CommodityServices(params))    // /commodities/123/services (#1508)
			r.Route("/supplies", CommoditySupplyLinks(params)) // /commodities/123/supplies (#1369)
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
