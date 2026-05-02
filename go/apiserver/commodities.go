package apiserver

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

type commoditiesAPI struct {
	entityService *services.EntityService
}

// listCommodities lists all commodities with pagination, filters, and sort.
// @Summary List commodities
// @Description get commodities
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param page query int false "Page number (default 1)"
// @Param per_page query int false "Items per page (default 50, max 100)"
// @Param type query []string false "Filter by commodity type; repeat to OR" collectionFormat(multi)
// @Param status query []string false "Filter by status (in_use, sold, lost, disposed, written_off); repeat to OR" collectionFormat(multi)
// @Param area_id query string false "Filter by exact area ID"
// @Param q query string false "Case-insensitive substring match on name + short_name"
// @Param include_inactive query bool false "Include drafts and non-in_use commodities (default false hides them)"
// @Param sort query string false "Sort field — name|registered_date|purchase_date|current_price|original_price|count, prefix with '-' for descending"
// @Success 200 {object} jsonapi.CommoditiesResponse "OK"
// @Router /commodities [get].
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

	commodities, total, err := commodityReg.ListPaginated(r.Context(), offset, perPage, opts)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)

	if err := render.Render(w, r, jsonapi.NewCommoditiesResponse(commodities, total, page, perPage)); err != nil {
		internalServerError(w, r, err)
		return
	}
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
	return opts
}

// getCommodity gets a commodity by ID.
// @Summary Get a commodity
// @Description get commodity by ID
// @Tags commodities
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Commodity ID"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Router /commodities/{id} [get].
func (api *commoditiesAPI) getCommodity(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	comReg := regSet.CommodityRegistry

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	var imagesError string
	images, err := comReg.GetImages(r.Context(), commodity.ID)
	if err != nil {
		imagesError = err.Error()
	}

	var manualsError string
	manuals, err := comReg.GetManuals(r.Context(), commodity.ID)
	if err != nil {
		manualsError = err.Error()
	}

	var invoicesError string
	invoices, err := comReg.GetInvoices(r.Context(), commodity.ID)
	if err != nil {
		invoicesError = err.Error()
	}

	resp := jsonapi.NewCommodityResponse(commodity, &jsonapi.CommodityMeta{
		Images:        images,
		ImagesError:   imagesError,
		Manuals:       manuals,
		ManualsError:  manualsError,
		Invoices:      invoices,
		InvoicesError: invoicesError,
	}).WithStatusCode(http.StatusOK)

	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createCommodity creates a new commodity.
// @Summary Create a new commodity
// @Description Add a new commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodity body jsonapi.CommodityRequest true "Commodity object"
// @Success 201 {object} jsonapi.CommodityResponse "Commodity created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /commodities [post].
func (api *commoditiesAPI) createCommodity(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.CommodityRequest

	rWithCurrency, err := requestWithMainCurrency(r)
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

	// Use standardized registry access
	commodityReg := registrySet.CommodityRegistry

	createdCommodity, err := commodityReg.Create(r.Context(), commodity)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	var imagesError string
	images, err := registrySet.CommodityRegistry.GetImages(r.Context(), createdCommodity.ID)
	if err != nil {
		imagesError = err.Error()
	}

	var manualsError string
	manuals, err := registrySet.CommodityRegistry.GetManuals(r.Context(), createdCommodity.ID)
	if err != nil {
		manualsError = err.Error()
	}

	var invoicesError string
	invoices, err := registrySet.CommodityRegistry.GetInvoices(r.Context(), createdCommodity.ID)
	if err != nil {
		invoicesError = err.Error()
	}

	resp := jsonapi.NewCommodityResponse(createdCommodity, &jsonapi.CommodityMeta{
		Images:        images,
		ImagesError:   imagesError,
		Manuals:       manuals,
		ManualsError:  manualsError,
		Invoices:      invoices,
		InvoicesError: invoicesError,
	}).WithStatusCode(http.StatusCreated)

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
// @Param id path string true "Commodity ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Router /commodities/{id} [delete].
func (api *commoditiesAPI) deleteCommodity(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	err := api.entityService.DeleteCommodityRecursive(r.Context(), commodity.ID)
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
// @Param id path string true "Commodity ID"
// @Param commodity body jsonapi.CommodityRequest true "Commodity object"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /commodities/{id} [put].
func (api *commoditiesAPI) updateCommodity(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	rWithCurrency, err := requestWithMainCurrency(r)
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

	// Use UpdateWithUser to ensure proper user context and validation
	ctx := r.Context()
	commodityReg := registrySet.CommodityRegistry
	updatedCommodity, err := commodityReg.Update(ctx, updateData)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	var imagesError string
	images, err := registrySet.CommodityRegistry.GetImages(r.Context(), commodity.ID)
	if err != nil {
		imagesError = err.Error()
	}

	var manualsError string
	manuals, err := registrySet.CommodityRegistry.GetManuals(r.Context(), commodity.ID)
	if err != nil {
		manualsError = err.Error()
	}

	var invoicesError string
	invoices, err := registrySet.CommodityRegistry.GetInvoices(r.Context(), commodity.ID)
	if err != nil {
		invoicesError = err.Error()
	}

	resp := jsonapi.NewCommodityResponse(updatedCommodity, &jsonapi.CommodityMeta{
		Images:        images,
		ImagesError:   imagesError,
		Manuals:       manuals,
		ManualsError:  manualsError,
		Invoices:      invoices,
		InvoicesError: invoicesError,
	}).WithStatusCode(http.StatusOK)

	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// Legacy commodity-scoped file routes (`/commodities/{id}/{images,invoices,manuals}*`)
// were removed under #1421. The unified `/files` surface (#1411) covers
// every read via `?linked_entity_type=commodity&linked_entity_id={id}`,
// detail / update / delete via `/files/{id}`, and uploads via
// `/uploads/file` with the same query. The FE was migrated in #1476.
// The legacy `images` / `invoices` / `manuals` SQL tables stay until
// ops run the #1399 backfill in production; that drop is a separate
// follow-up.

// bulkDeleteCommodities deletes a list of commodities in a single request.
// @Summary Bulk-delete commodities
// @Description Delete every commodity whose id appears in the body. The
// @Description response lists succeeded vs. failed ids so the frontend
// @Description can render partial-failure UX without parsing per-id HTTP
// @Description statuses.
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param body body jsonapi.BulkIDsRequest true "List of commodity IDs to delete"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /commodities/bulk-delete [post].
func (api *commoditiesAPI) bulkDeleteCommodities(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.BulkIDsRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	succeeded := make([]string, 0, len(input.Data.Attributes.IDs))
	failed := make([]jsonapi.BulkResultFail, 0)
	for _, id := range input.Data.Attributes.IDs {
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
// @Param body body jsonapi.BulkMoveRequest true "List of commodity IDs and the destination area_id"
// @Success 200 {object} jsonapi.BulkResultResponse "Per-id outcome"
// @Failure 422 {object} jsonapi.Errors "Bad request body"
// @Router /commodities/bulk-move [post].
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
		commodity.AreaID = input.Data.Attributes.AreaID
		if _, err := registrySet.CommodityRegistry.Update(r.Context(), *commodity); err != nil {
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

func Commodities(params Params) func(r chi.Router) {
	api := &commoditiesAPI{
		entityService: params.EntityService,
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
			r.Get("/", api.getCommodity)       // GET /commodities/123
			r.Put("/", api.updateCommodity)    // PUT /commodities/123
			r.Delete("/", api.deleteCommodity) // DELETE /commodities/123

			// Legacy commodity-scoped file routes were removed under
			// #1421. Use `/files?linked_entity_type=commodity&linked_entity_id=…`
			// for read, `/files/{id}` for detail/update/delete, and
			// `/uploads/file` (with the same query) for new uploads.
		})
		r.Post("/", api.createCommodity) // POST /commodities
	}
}

func requestWithMainCurrency(r *http.Request) (*http.Request, error) {
	group := appctx.GroupFromContext(r.Context())
	if group == nil || group.MainCurrency == "" {
		return nil, registry.ErrMainCurrencyNotSet
	}

	ctx := validationctx.WithMainCurrency(r.Context(), string(group.MainCurrency))

	return r.WithContext(ctx), nil
}
