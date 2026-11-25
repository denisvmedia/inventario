package apiserver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
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
	// DraftDiscardedCode: the draft's discard has begun; it can no longer be
	// saved, promoted or given files.
	DraftDiscardedCode = "draft.discarded"
)

// commodityDraftsAPI serves the drafts of a group: unfinished items saved on
// the server. Every member can read them; saving, discarding and promoting
// need a role that can edit items (the route's write gate).
type commodityDraftsAPI struct {
	service    *services.CommodityDraftService
	tagService *services.TagService
	stale      staleDiscards
}

// newCommodityDraftService builds the draft service of the draft and inbox
// routes.
func newCommodityDraftService(params Params) *services.CommodityDraftService {
	return services.NewCommodityDraftService(params.FactorySet, params.CommodityDraftServiceOptions...)
}

// Drafts mounts /g/{groupSlug}/drafts.
func Drafts(params Params, groupService *services.GroupService) func(r chi.Router) {
	drafts := newCommodityDraftService(params)
	api := &commodityDraftsAPI{
		service:    drafts,
		tagService: services.NewTagService(params.FactorySet),
		stale:      staleDiscards{drafts: drafts, groups: groupService},
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
	renderDraftErrorFor(w, r, err, chi.URLParam(r, "draftID"))
}

// renderDraftErrorFor is renderDraftError for the draft draftID, for routes
// that name the draft elsewhere than in the path.
func renderDraftErrorFor(w http.ResponseWriter, r *http.Request, err error, draftID string) {
	switch {
	case errors.Is(err, registry.ErrDraftRevisionConflict):
		meta := map[string]any{}
		if regSet := RegistrySetFromContext(r.Context()); regSet != nil {
			if current, getErr := regSet.CommodityDraftRegistry.Get(r.Context(), draftID); getErr == nil {
				meta["revision"] = current.Revision
			}
		}
		_ = codedConflictError(w, r, err, DraftRevisionConflictCode, meta)
	case errors.Is(err, registry.ErrDraftPromoted):
		meta := map[string]any{}
		if regSet := RegistrySetFromContext(r.Context()); regSet != nil {
			if current, getErr := regSet.CommodityDraftRegistry.Get(r.Context(), draftID); getErr == nil && current.PromotedCommodityID != nil {
				meta["commodity_id"] = *current.PromotedCommodityID
			}
		}
		_ = codedConflictError(w, r, err, DraftPromotedCode, meta)
	case errors.Is(err, registry.ErrDraftDiscarded):
		_ = codedConflictError(w, r, err, DraftDiscardedCode, nil)
	default:
		if _, ok := errors.AsType[validation.Errors](err); ok {
			_ = unprocessableEntityError(w, r, err)
			return
		}
		_ = renderEntityError(w, r, err)
	}
}

// asAttributesError places the draft service's validation errors, which
// name the draft's fields, under data.attributes, where the request has
// them.
func asAttributesError(err error) error {
	if verrs, ok := errors.AsType[validation.Errors](err); ok {
		return validation.Errors{"data": validation.Errors{"attributes": verrs}}
	}
	return err
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
// @Description item saved on the server; it is not an item and appears in no item list, count
// @Description or search. A whole-group export in the .inb format (3.5 and later) carries the
// @Description open drafts; a selected-items export and the legacy XML export do not. When a
// @Description member who can edit items lists the drafts, the discards that stopped more than
// @Description ten minutes ago are finished first. That is best effort: at most 10 per request,
// @Description and a failure is logged, not returned. A 200 therefore does not mean that every
// @Description stopped discard finished or that the client_key of a discarded draft is free.
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
	api.stale.finish(r)
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
// @Description its values cleared and the id of the item it became. A draft being discarded
// @Description answers 404. Its files are listed by GET
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
	if err == nil && draft.IsDiscarding() {
		err = errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity_draft", "entity_id", draft.ID))
	}
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
// @Description making another one, so a retry after a lost response is safe. A key whose draft
// @Description is being discarded answers 409 draft.discarded. Known fields of the item form
// @Description must have the type the form uses (text, a number or numeric text, a list of
// @Description text, true or false); a field of another type is refused with 422 on
// @Description values.<field>. A type in values.type must be one of the group's item types
// @Description (422 item_type_unknown on values.type otherwise).
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
		renderDraftError(w, r, asAttributesError(err))
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
// @Description draft.promoted, a draft being discarded 409 draft.discarded. The values are
// @Description checked as on create.
// @Tags drafts
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draftID path string true "Draft ID"
// @Param draft body jsonapi.CommodityDraftSaveRequest true "Values and revision"
// @Success 200 {object} jsonapi.CommodityDraftResponse "Saved"
// @Failure 404 {object} jsonapi.Errors "Draft not found"
// @Failure 409 {object} jsonapi.Errors "Stale revision, promoted draft, or draft being discarded"
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
		renderDraftError(w, r, asAttributesError(err))
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
// @Description attached elsewhere only lose the link to the draft. A file attached only to the
// @Description draft and to an item that is already in Trash goes to Trash with that item:
// @Description restoring the item brings the file back. A promoted draft answers 409
// @Description draft.promoted. Once the discard has begun the draft can no longer be saved,
// @Description promoted or given files; repeating a discard that did not finish completes it.
// @Tags drafts
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draftID path string true "Draft ID"
// @Success 204 "Discarded"
// @Failure 404 {object} jsonapi.Errors "Draft not found"
// @Failure 409 {object} jsonapi.Errors "Promoted draft"
// @Router /g/{groupSlug}/drafts/{draftID} [delete].
func (api *commodityDraftsAPI) discard(w http.ResponseWriter, r *http.Request) {
	// A client that goes away must not stop the discard half way: the draft
	// would stay hidden with its files and its client key until
	// staleDiscards takes it up.
	if err := api.service.Discard(context.WithoutCancel(r.Context()), chi.URLParam(r, "draftID")); err != nil {
		renderDraftError(w, r, err)
		return
	}
	render.NoContent(w, r)
}

// staleDiscards finishes the draft discards of a group that stopped half way
// (a server that died, a database error). Such a draft is hidden but keeps
// its files and its client key until something finishes it. The routes that
// list drafts call it: GET /drafts and the inbox.
type staleDiscards struct {
	drafts *services.CommodityDraftService
	groups *services.GroupService
}

// finish finishes the stopped discards of the request's group when the
// member may discard drafts (user and up). A failure is logged and the
// request is served anyway.
func (s staleDiscards) finish(r *http.Request) {
	group := groupFromContext(r.Context())
	user := GetUserFromRequest(r)
	if group == nil || user == nil || s.drafts == nil || s.groups == nil {
		return
	}
	canWrite, _, err := s.groups.HasRoleAtLeast(r.Context(), group.ID, user.ID, models.GroupRoleUser)
	if err != nil {
		slog.Error("Failed to check the role for finishing stopped draft discards", "group_id", group.ID, "user_id", user.ID, "error", err)
		return
	}
	if !canWrite {
		return
	}
	finished, err := s.drafts.FinishStaleDiscards(context.WithoutCancel(r.Context()))
	if err != nil {
		slog.Error("Failed to finish stopped draft discards", "group_id", group.ID, "error", err)
	}
	if finished > 0 {
		slog.Info("Finished stopped draft discards", "group_id", group.ID, "count", finished)
	}
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
// @Description draft.revision_conflict when it is stale). meta.promotion_key is required: a key
// @Description the client makes for this promotion (a UUID) and sends again on every retry until
// @Description one succeeds. A repeat with the key of the promotion that happened creates nothing
// @Description and returns the item with 200, so a retry after a lost response is safe; any other
// @Description key, or the same key once that item no longer exists, answers 409 draft.promoted
// @Description with meta.commodity_id. A draft being discarded answers 409 draft.discarded.
// @Tags drafts
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param draftID path string true "Draft ID"
// @Param commodity body jsonapi.CommodityDraftPromoteRequest true "Finished item and draft revision"
// @Success 201 {object} jsonapi.CommodityResponse "Item created"
// @Success 200 {object} jsonapi.CommodityResponse "The draft was already promoted to this item"
// @Failure 404 {object} jsonapi.Errors "Draft not found"
// @Failure 409 {object} jsonapi.Errors "Stale revision, draft promoted by another request, or draft being discarded"
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

	item, created, err := api.service.Promote(r.Context(), chi.URLParam(r, "draftID"), input.Meta.Revision, input.Meta.PromotionKey, commodity)
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
