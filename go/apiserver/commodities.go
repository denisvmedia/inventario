package apiserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

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
	trashService  *services.TrashService
	tagService    *services.TagService
	coverService  *services.CommodityCoverService
	eventService  *services.CommodityEventService
	bulkService   *services.CommodityBulkService
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
// @Param status query []string false "Filter by status (in_use, sold, lost, disposed, written_off, donated, stolen); repeat to OR" collectionFormat(multi)
// @Param area_id query string false "Filter by exact area ID"
// @Param unassigned query bool false "Only commodities with no area (ignored when area_id is set)"
// @Param seller_id query []string false "Filter by seller ID; repeat for several (OR)" collectionFormat(multi)
// @Param q query string false "Search text: space-separated terms, quotes keep a phrase; every term must match the name, short name, brand, model, a serial or part number, a tag, a note or the asset ID. At most 200 characters and 10 terms"
// @Param tags query []string false "Only items with these tag slugs (comma-separated or repeated); see tag_mode" collectionFormat(multi)
// @Param tag_mode query string false "How tags combine: all (default) or any" Enums(all,any)
// @Param tags_exclude query []string false "Only items with none of these tag slugs (comma-separated or repeated)" collectionFormat(multi)
// @Param no_tags query bool false "Only items with no tags; cannot be combined with tags or tags_exclude"
// @Param no_photo query bool false "Only items without a photo; cannot be combined with trashed"
// @Param no_purchase_price query bool false "Only items whose purchase price is unknown; a known zero is a price"
// @Param no_purchase_date query bool false "Only items whose purchase date is unknown"
// @Param cf query []string false "Custom field condition as <field id>:<op>[:<value>], op one of set, unset, eq, contains, gte, lte, in; repeat for several (AND). A field the group does not have matches no item" collectionFormat(multi)
// @Param include_inactive query bool false "Include drafts and non-in_use commodities (default true; false hides them)"
// @Param sort query string false "Sort field — name|registered_date|purchase_date|current_price|original_price|count, or cf.<field id> for a custom field (items without a value last); prefix with '-' for descending"
// @Param warranty_status query []string false "Filter by computed warranty status (active, expiring, expired, none); repeat to OR" collectionFormat(multi)
// @Param warranty_expires_before query string false "Restrict to commodities whose warranty expires strictly before YYYY-MM-DD"
// @Param lent_out query bool false "Filter by current loan state: true = only currently lent (open loan), false = only currently not-lent"
// @Param trashed query bool false "true lists the items in Trash instead of the live ones (default false); cannot be combined with lent_out"
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
		queryRunError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, result.Total)

	covers := api.resolveCoversForList(r, regSet.FileRegistry, result.Commodities)

	digest := listSelectionDigest(r.Context(), regSet, query, offset, result)

	// The names of linked items are for the list's custom field columns; a
	// failed lookup shows those columns without names instead of failing
	// the list.
	linked, err := linkedItemSummaries(r.Context(), regSet, result.Commodities...)
	if err != nil {
		slog.WarnContext(r.Context(), "items list: linked item lookup failed; rendering without it", "err", err)
		linked = nil
	}

	resp := jsonapi.NewCommoditiesResponseWithCovers(result.Commodities, result.Total, page, perPage, covers).
		WithMatches(result.Matches).
		WithSelectionDigest(digest).
		WithLinkedItems(linked)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// listSelectionDigest returns the digest of the items a list query selects,
// on every page, for a bulk action on all of them (meta.selection_digest).
// It is empty when a bulk action cannot take the query (in Trash, no items,
// more than services.MaxBulkQueryItems), when the ids read do not add up to
// the total the list counted, and when reading them fails: the digest is
// optional, so a failed lookup is logged and the page renders without it.
func listSelectionDigest(ctx context.Context, regSet *registry.Set, query commodityquery.Query, offset int, result commodityquery.Page) string {
	if query.Trashed || result.Total == 0 || result.Total > services.MaxBulkQueryItems {
		return ""
	}
	if offset == 0 && len(result.Commodities) == result.Total {
		ids := make([]string, 0, len(result.Commodities))
		for _, c := range result.Commodities {
			ids = append(ids, c.ID)
		}
		return commodityquery.Digest(ids)
	}
	digest, ok, err := commodityquery.SelectionDigest(ctx, regSet, query, result.Total)
	if err != nil {
		slog.WarnContext(ctx, "items list: selection digest lookup failed; rendering without it", "err", err)
		return ""
	}
	if !ok {
		return ""
	}
	return digest
}

// countCommodityTags counts, per tag, the items a query selects with its tag
// filter set aside (commodityquery.TagCounts), for a tag picker that shows
// how many items each tag would list.
// @Summary Count commodity tags
// @Description Per item tag slug, the number of items the query selects when its own tag filter (tags, tag_mode, tags_exclude, no_tags) is set aside, plus the number without tags.
// @Description Takes the filter parameters of GET /commodities (item query contract); sort is accepted and ignored. An unknown parameter or an unreadable value is a 400.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param q query string false "Search text, as for GET /commodities"
// @Param type query []string false "Filter by commodity type, as for GET /commodities" collectionFormat(multi)
// @Param status query []string false "Filter by status, as for GET /commodities" collectionFormat(multi)
// @Param area_id query string false "Filter by exact area ID"
// @Param unassigned query bool false "Only commodities with no area"
// @Param seller_id query []string false "Filter by seller ID; repeat for several (OR)" collectionFormat(multi)
// @Param include_inactive query bool false "Include drafts and non-in_use commodities (default true)"
// @Param warranty_status query []string false "Filter by computed warranty status" collectionFormat(multi)
// @Param warranty_expires_before query string false "Warranty expires strictly before YYYY-MM-DD"
// @Param lent_out query bool false "Filter by current loan state"
// @Param trashed query bool false "Count the items in Trash instead of the live ones (default false); cannot be combined with lent_out"
// @Param tags query []string false "Validated as for GET /commodities, then set aside for the counts" collectionFormat(multi)
// @Param tag_mode query string false "Validated as for GET /commodities, then set aside for the counts" Enums(all,any)
// @Param tags_exclude query []string false "Validated as for GET /commodities, then set aside for the counts" collectionFormat(multi)
// @Param no_tags query bool false "Validated as for GET /commodities, then set aside for the counts"
// @Param no_photo query bool false "Only items without a photo; cannot be combined with trashed"
// @Param no_purchase_price query bool false "Only items whose purchase price is unknown"
// @Param no_purchase_date query bool false "Only items whose purchase date is unknown"
// @Param cf query []string false "Custom field condition, as for GET /commodities" collectionFormat(multi)
// @Param sort query string false "Accepted and ignored"
// @Success 200 {object} jsonapi.CommodityTagCountsResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Unsupported parameter or invalid value"
// @Router /g/{groupSlug}/commodities/tag-counts [get].
func (*commoditiesAPI) countCommodityTags(w http.ResponseWriter, r *http.Request) {
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
	query, err := commodityquery.Parse(values)
	if err != nil {
		queryParamError(w, r, err)
		return
	}
	counts, err := commodityquery.TagCounts(r.Context(), regSet, query)
	if err != nil {
		queryRunError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityTagCountsResponse(counts)); err != nil {
		internalServerError(w, r, err)
	}
}

// Brand suggestions: the default and the most a request may ask for.
const (
	defaultCommodityBrandsLimit = 20
	maxCommodityBrandsLimit     = 100
)

// listCommodityBrands lists the brands the group's items carry, for the
// brand input's suggestions.
// @Summary List commodity brands
// @Description The distinct brands of the group's items outside Trash, drafts and inactive items included, with the number of items carrying each, most used first. Brands are compared exactly, so "Bosch" and "bosch" are two entries.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param q query string false "Only brands that contain this text, without regard to case"
// @Param limit query int false "At most this many brands (1 to 100)" default(20)
// @Success 200 {object} jsonapi.CommodityBrandsResponse "OK"
// @Failure 400 {object} jsonapi.Errors "Invalid limit"
// @Router /g/{groupSlug}/commodities/brands [get].
func (*commoditiesAPI) listCommodityBrands(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	limit := defaultCommodityBrandsLimit
	if v := r.URL.Query().Get("limit"); v != "" {
		parsed, err := strconv.Atoi(v)
		if err != nil || parsed < 1 || parsed > maxCommodityBrandsLimit {
			badRequest(w, r, fmt.Errorf("limit must be a whole number from 1 to %d", maxCommodityBrandsLimit))
			return
		}
		limit = parsed
	}

	brands, err := regSet.CommodityRegistry.ListBrands(r.Context(), r.URL.Query().Get("q"), limit)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityBrandsResponse(brands)); err != nil {
		internalServerError(w, r, err)
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
	linked, err := linkedItemSummaries(r.Context(), RegistrySetFromContext(r.Context()), commodity)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	resp = resp.WithLinkedItems(linked)

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

	if err := validateCommoditySeller(r.Context(), registrySet.SellerRegistry, commodity.SellerID); err != nil {
		renderSellerWriteError(w, r, err)
		return
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

// deleteCommodity moves a commodity to Trash.
// @Summary Delete a commodity
// @Description Move the commodity to Trash with the files no other item,
// @Description area or location uses. It can be restored from Trash until
// @Description the retention runs out.
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

	// TrashCommodity records the "deleted" event in the item's history; it
	// stays there, so a restored item shows when it was deleted.
	_, err := api.trashService.TrashCommodity(r.Context(), commodity.ID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
	if err := services.ValidateCommodityStatusTransition(commodity.Status, updateData.Status, updateData.StatusDate, updateData.SalePrice); err != nil {
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

	if err := validateCommoditySeller(r.Context(), registrySet.SellerRegistry, updateData.SellerID); err != nil {
		renderSellerWriteError(w, r, err)
		return
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
//   - a non-empty string — must reference a file that is attached to this
//     commodity among its photos and is an image. The handler validates
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
// @Description file must already be attached to this commodity among its
// @Description photos and be an image.
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
	// Only the cover changes; the custom field values read with the item
	// would overwrite one written since.
	updateData.CustomFields = nil
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

// setCommodityDraftMark sets or clears the draft mark of an item and leaves
// the rest of it as it is. A full PUT built from an earlier read would write
// back every other field as that read had it, undoing an edit that landed in
// between; this write changes the mark alone, under the item's row lock. The
// item as it will be stored must pass the checks a full save runs, so an
// item that is not complete enough to be reviewed keeps its mark.
//
// @Summary Mark an item as draft or as reviewed
// @Description Sets (`true`) or clears (`false`) the draft mark of an item
// @Description and changes no other field. The item with the new mark must
// @Description pass the validation a full update runs.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param body body jsonapi.CommodityDraftMarkRequest true "The draft mark"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/draft [patch].
func (api *commoditiesAPI) setCommodityDraftMark(w http.ResponseWriter, r *http.Request) {
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

	var input jsonapi.CommodityDraftMarkRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	ctx := r.Context()
	check := func(next models.Commodity) error {
		// Checked as a full save would see it: a legacy zero price that
		// save echoes back is stored as unknown.
		stored := next
		next.UnsetEchoedLegacyZeros(&stored)
		err := next.ValidateWithContext(ctx)
		if verrs, ok := errors.AsType[validation.Errors](err); ok {
			// Nested as the body of a full update nests them, so a client
			// reads the refused fields the same way from either write.
			return validation.Errors{"data": validation.Errors{"attributes": verrs}}
		}
		return err
	}
	// The event compares the item as the write found it under its lock, so
	// a change that landed after the middleware's read is not recorded as
	// part of this one.
	before, updated, err := registrySet.CommodityRegistry.SetDraft(ctx, commodity.ID, *input.Data.Attributes.Draft, check)
	if err != nil {
		if _, ok := errors.AsType[validation.Errors](err); ok {
			unprocessableEntityError(w, r, err)
			return
		}
		if _, ok := errors.AsType[validation.Error](err); ok {
			unprocessableEntityError(w, r, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	api.eventService.EmitUpdated(ctx, before, updated)

	resp := jsonapi.NewCommodityResponse(updated).WithStatusCode(http.StatusOK)
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
	link := file.LinkTo(models.FileLinkCommodity, commodityID)
	if link == nil {
		return validationError("file_id", "file is not attached to this commodity")
	}
	// A cover is one of the item's photos: the attachments in its images
	// bucket, which the No photo filter counts as well.
	if link.Meta != models.FileLinkMetaImages {
		return validationError("file_id", "file is not attached to this commodity as a photo")
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

func Commodities(params Params) func(r chi.Router) {
	fileSigningService := services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration)
	api := &commoditiesAPI{
		entityService: params.EntityService,
		trashService:  services.NewTrashService(params.FactorySet, params.UploadLocation),
		tagService:    services.NewTagService(params.FactorySet),
		coverService:  services.NewCommodityCoverService(fileSigningService),
		eventService:  services.NewCommodityEventService(params.FactorySet),
		factorySet:    params.FactorySet,
	}
	api.bulkService = services.NewCommodityBulkService(api.tagService, api.eventService, api.trashService)

	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listCommodities) // GET /commodities
		// Bulk endpoints. Mounted before `/{commodityID}` so chi's
		// static-vs-param-route matcher routes `/bulk-*` here rather than
		// treating those slugs as a commodity id.
		r.Post("/bulk-delete", api.bulkDeleteCommodities) // POST /commodities/bulk-delete
		r.Post("/bulk-move", api.bulkMoveCommodities)     // POST /commodities/bulk-move
		r.Post("/bulk-update", api.bulkUpdateCommodities) // POST /commodities/bulk-update
		r.Get("/tag-counts", api.countCommodityTags)      // GET /commodities/tag-counts
		r.Get("/brands", api.listCommodityBrands)         // GET /commodities/brands
		r.Route("/{commodityID}", func(r chi.Router) {
			r.Use(commodityCtx())
			r.Get("/", api.getCommodity)                          // GET /commodities/123
			r.Put("/", api.updateCommodity)                       // PUT /commodities/123
			r.Delete("/", api.deleteCommodity)                    // DELETE /commodities/123
			r.Patch("/cover", api.setCommodityCover)              // PATCH /commodities/123/cover
			r.Patch("/draft", api.setCommodityDraftMark)          // PATCH /commodities/123/draft
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
