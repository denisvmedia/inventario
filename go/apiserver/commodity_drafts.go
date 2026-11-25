package apiserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// JSON:API error codes of the draft endpoints (#2812).
const (
	// DraftRevisionConflictCode: the draft was saved by another tab or
	// device after the client loaded it. Meta carries the current revision.
	DraftRevisionConflictCode = "draft.revision_conflict"
	// DraftPromotedCode: the draft has already become an item. Meta carries
	// the item id when it is known.
	DraftPromotedCode = "draft.promoted"
)

// commodityDraftsAPI serves the drafts of a group: unfinished items saved on
// the server. Every member can read them; saving, discarding and promoting
// need a role that can edit items (the route's write gate).
type commodityDraftsAPI struct {
	service    *services.CommodityDraftService
	tagService *services.TagService
}

// Drafts mounts /g/{groupSlug}/drafts.
func Drafts(params Params) func(r chi.Router) {
	api := &commodityDraftsAPI{
		service:    services.NewCommodityDraftService(params.FactorySet),
		tagService: services.NewTagService(params.FactorySet),
	}
	return func(r chi.Router) {
		r.Get("/", api.list)
		r.Post("/", api.create)
		r.Route("/{draftID}", func(r chi.Router) {
			r.Get("/", api.get)
			r.Put("/", api.save)
			r.Delete("/", api.discard)
			r.Post("/promote", api.promote)
		})
	}
}

// renderDraftError maps the draft lifecycle errors. A stale revision and an
// already promoted draft are 409s with a code and the facts the client needs
// to recover; invalid values are 422s.
func renderDraftError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, registry.ErrDraftRevisionConflict):
		meta := map[string]any{}
		if regSet := RegistrySetFromContext(r.Context()); regSet != nil {
			if current, getErr := regSet.CommodityDraftRegistry.Get(r.Context(), chi.URLParam(r, "draftID")); getErr == nil {
				meta["revision"] = current.Revision
			}
		}
		_ = codedConflictError(w, r, err, DraftRevisionConflictCode, meta)
	case errors.Is(err, registry.ErrDraftPromoted):
		meta := map[string]any{}
		if regSet := RegistrySetFromContext(r.Context()); regSet != nil {
			if current, getErr := regSet.CommodityDraftRegistry.Get(r.Context(), chi.URLParam(r, "draftID")); getErr == nil && current.PromotedCommodityID != nil {
				meta["commodity_id"] = *current.PromotedCommodityID
			}
		}
		_ = codedConflictError(w, r, err, DraftPromotedCode, meta)
	default:
		if _, ok := errors.AsType[validation.Errors](err); ok {
			_ = unprocessableEntityError(w, r, err)
			return
		}
		_ = renderEntityError(w, r, err)
	}
}

// fileCount returns the number of live files attached to a draft.
func (api *commodityDraftsAPI) fileCount(r *http.Request, id string) (int, error) {
	files, err := api.service.Files(r.Context(), id)
	if err != nil {
		return 0, err
	}
	return len(files), nil
}

// list returns the open drafts of the group.
//
// @Summary List drafts
// @Description The group's open drafts, most recently saved first. A draft is an unfinished
// @Description item saved on the server; it is not an item and appears in no item list, count,
// @Description search or export.
// @Tags drafts
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param page query int false "Page number"
// @Param per_page query int false "Drafts per page (at most 100)"
// @Success 200 {object} jsonapi.CommodityDraftsResponse "OK"
// @Router /g/{groupSlug}/drafts [get].
func (api *commodityDraftsAPI) list(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	page, perPage := parsePagination(r.URL.Query().Get("page"), r.URL.Query().Get("per_page"))
	drafts, total, err := regSet.CommodityDraftRegistry.ListOpen(r.Context(), (page-1)*perPage, perPage)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	ids := make([]string, 0, len(drafts))
	for _, d := range drafts {
		ids = append(ids, d.ID)
	}
	counts, err := api.service.FileCounts(r.Context(), ids)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, jsonapi.NewCommodityDraftsResponse(drafts, total, counts)); err != nil {
		internalServerError(w, r, err)
	}
}

// get returns one draft.
//
// @Summary Get a draft
// @Description One draft with its values and revision. A promoted draft is returned too, with
// @Description its values cleared and the id of the item it became. Its files are listed by GET
// @Description /g/{groupSlug}/files?linked_entity_type=draft&linked_entity_id={draftID}.
// @Tags drafts
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draftID path string true "Draft ID"
// @Success 200 {object} jsonapi.CommodityDraftResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Draft not found"
// @Router /g/{groupSlug}/drafts/{draftID} [get].
func (api *commodityDraftsAPI) get(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	draft, err := regSet.CommodityDraftRegistry.Get(r.Context(), chi.URLParam(r, "draftID"))
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	count, err := api.fileCount(r, draft.ID)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityDraftResponse(draft, count)); err != nil {
		internalServerError(w, r, err)
	}
}

// create saves a new draft.
//
// @Summary Create a draft
// @Description Save unfinished form values as a new draft. The values are stored as given and
// @Description are not validated as an item. client_key is the client's identity for the draft:
// @Description a create with a key the group already has returns that draft with 200 instead of
// @Description making another one, so a retry after a lost response is safe.
// @Tags drafts
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draft body jsonapi.CommodityDraftCreateRequest true "Draft"
// @Success 201 {object} jsonapi.CommodityDraftResponse "Draft created"
// @Success 200 {object} jsonapi.CommodityDraftResponse "A draft with this client key already exists"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/drafts [post].
func (api *commodityDraftsAPI) create(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.CommodityDraftCreateRequest
	if err := render.Bind(r, &input); err != nil {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	draft, created, err := api.service.Create(r.Context(), attrs.ClientKey, attrs.Values)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	count, err := api.fileCount(r, draft.ID)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	if err := render.Render(w, r, jsonapi.NewCommodityDraftResponse(draft, count).WithStatusCode(status)); err != nil {
		internalServerError(w, r, err)
	}
}

// save replaces the values of a draft.
//
// @Summary Save a draft
// @Description Replace the values of an open draft. revision is the revision the values were
// @Description based on; the response carries the next one. A save based on an older revision
// @Description is refused with 409 draft.revision_conflict and the current revision in meta, so
// @Description a stale tab cannot overwrite newer input. A promoted draft answers 409
// @Description draft.promoted.
// @Tags drafts
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draftID path string true "Draft ID"
// @Param draft body jsonapi.CommodityDraftSaveRequest true "Values and revision"
// @Success 200 {object} jsonapi.CommodityDraftResponse "Saved"
// @Failure 404 {object} jsonapi.Errors "Draft not found"
// @Failure 409 {object} jsonapi.Errors "Stale revision or promoted draft"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/drafts/{draftID} [put].
func (api *commodityDraftsAPI) save(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.CommodityDraftSaveRequest
	if err := render.Bind(r, &input); err != nil {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	attrs := input.Data.Attributes
	draft, err := api.service.Save(r.Context(), chi.URLParam(r, "draftID"), attrs.Revision, attrs.Values)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	count, err := api.fileCount(r, draft.ID)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityDraftResponse(draft, count)); err != nil {
		internalServerError(w, r, err)
	}
}

// discard deletes a draft.
//
// @Summary Discard a draft
// @Description Delete an open draft. Its files that nothing else uses go to Trash, where they
// @Description can be restored as standalone files until the Trash retention runs out; files
// @Description attached elsewhere only lose the link to the draft. A promoted draft answers 409
// @Description draft.promoted.
// @Tags drafts
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draftID path string true "Draft ID"
// @Success 204 "Discarded"
// @Failure 404 {object} jsonapi.Errors "Draft not found"
// @Failure 409 {object} jsonapi.Errors "Promoted draft"
// @Router /g/{groupSlug}/drafts/{draftID} [delete].
func (api *commodityDraftsAPI) discard(w http.ResponseWriter, r *http.Request) {
	if err := api.service.Discard(r.Context(), chi.URLParam(r, "draftID")); err != nil {
		renderDraftError(w, r, err)
		return
	}
	render.NoContent(w, r)
}

// validateDraftSeller refuses, on the seller_id field, a seller the group
// does not have, as POST /commodities does. The seller may have been deleted
// while the draft was open: the draft's values keep its ID, since they have
// no foreign key. A draft that is already promoted creates nothing, so the
// retry of a promotion is not checked.
func validateDraftSeller(r *http.Request, sellerID *string) error {
	if sellerID == nil || *sellerID == "" {
		return nil
	}
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		return errors.New("registry set not found in context")
	}
	draft, err := regSet.CommodityDraftRegistry.Get(r.Context(), chi.URLParam(r, "draftID"))
	if err != nil {
		return err
	}
	if !draft.IsOpen() {
		return nil
	}
	return validateCommoditySeller(r.Context(), regSet.SellerRegistry, sellerID)
}

// promote turns a draft into an item.
//
// @Summary Finish a draft
// @Description Create the item from the finished form and move the draft's files to it, in one
// @Description step. The body is the item as for POST /g/{groupSlug}/commodities, validated the
// @Description same way, and meta.revision is the draft revision it was based on (409
// @Description draft.revision_conflict when it is stale). Promoting a draft that is already
// @Description promoted creates nothing and returns the item it became with 200, so a retry
// @Description after a lost response is safe; when that item no longer exists the answer is 409
// @Description draft.promoted.
// @Tags drafts
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draftID path string true "Draft ID"
// @Param commodity body jsonapi.CommodityDraftPromoteRequest true "Finished item and draft revision"
// @Success 201 {object} jsonapi.CommodityResponse "Item created"
// @Success 200 {object} jsonapi.CommodityResponse "The draft was already promoted to this item"
// @Failure 404 {object} jsonapi.Errors "Draft not found"
// @Failure 409 {object} jsonapi.Errors "Stale revision, or the item the draft became is gone"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/drafts/{draftID}/promote [post].
func (api *commodityDraftsAPI) promote(w http.ResponseWriter, r *http.Request) {
	rWithCurrency, err := requestWithGroupCurrency(r)
	if err != nil {
		_ = renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	var input jsonapi.CommodityDraftPromoteRequest
	if err := render.Bind(r, &input); err != nil {
		_ = unprocessableEntityError(w, r, err)
		return
	}
	user, ok := RequireUserContext(w, r)
	if !ok {
		return
	}
	if secErr := ValidateInputSanitization(r, input.Data.Attributes); secErr != nil {
		HandleSecurityError(w, r, secErr)
		return
	}

	commodity := *input.Data.Attributes
	if commodity.TenantID == "" {
		commodity.TenantID = user.TenantID
	}
	// Tag rows are created before the promotion transaction, as on
	// POST /commodities; the transaction then locks them.
	if len(commodity.Tags) > 0 {
		slugs, err := api.tagService.NormalizeAndEnsureSlugs(r.Context(), models.TagKindCommodity, []string(commodity.Tags))
		if err != nil {
			_ = renderEntityError(w, r, err)
			return
		}
		commodity.Tags = models.ValuerSlice[string](slugs)
	}

	if err := validateDraftSeller(r, commodity.SellerID); err != nil {
		renderDraftError(w, r, err)
		return
	}

	item, created, err := api.service.Promote(r.Context(), chi.URLParam(r, "draftID"), input.Meta.Revision, commodity)
	if err != nil {
		renderDraftError(w, r, err)
		return
	}
	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	if err := render.Render(w, r, jsonapi.NewCommodityResponse(item).WithStatusCode(status)); err != nil {
		internalServerError(w, r, err)
	}
}
