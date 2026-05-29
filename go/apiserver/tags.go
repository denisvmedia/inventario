package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

const tagCtxKey ctxValueKey = "tag"

func tagFromContext(ctx context.Context) *models.Tag {
	tag, ok := ctx.Value(tagCtxKey).(*models.Tag)
	if !ok {
		return nil
	}
	return tag
}

func tagCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			tagID := chi.URLParam(r, "tagID")
			tag, err := regSet.TagRegistry.Get(r.Context(), tagID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), tagCtxKey, tag)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type tagsAPI struct {
	factorySet *registry.FactorySet
	tagService *services.TagService
}

// listTags lists tags with pagination, optional q-search, and sort.
// @Summary List tags
// @Description get tags of the given kind. Pass include=usage to attach a per-row meta.usage block. kind=commodity|file is required — item-tags and file-tags are separate entities.
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param kind query string true "Tag kind (commodity = item-tags, file = file-tags)" Enums(commodity,file)
// @Param q query string false "Search by label or slug"
// @Param sort query string false "Sort field (label, created_at, usage)" Enums(label,created_at,usage)
// @Param order query string false "Sort direction (asc, desc)" default(asc)
// @Param page query int false "Page number (1-based)" default(1)
// @Param per_page query int false "Items per page" default(50)
// @Param include query string false "Comma-separated extras. 'usage' attaches per-row meta.usage." Enums(usage)
// @Success 200 {object} jsonapi.TagsResponse "OK"
// @Failure 422 {object} jsonapi.Errors "Missing or invalid kind value"
// @Router /g/{groupSlug}/tags [get].
func (api *tagsAPI) listTags(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	kind, ok := parseTagKind(q.Get("kind"))
	if !ok {
		unprocessableEntityError(w, r, fmt.Errorf("invalid kind: %q (must be one of: commodity, file)", q.Get("kind")))
		return
	}

	opts := registry.TagListOptions{
		Search:    q.Get("q"),
		SortField: registry.TagSortField(q.Get("sort")),
		SortDesc:  q.Get("order") == "desc",
		Kind:      kind,
	}

	tags, total, err := regSet.TagRegistry.ListPaginated(r.Context(), offset, perPage, opts)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	var usageBySlug map[string]registry.TagUsage
	if includeHasToken(q.Get("include"), "usage") && len(tags) > 0 {
		slugs := make([]string, 0, len(tags))
		for _, t := range tags {
			slugs = append(slugs, t.Slug)
		}
		usageBySlug, err = regSet.TagRegistry.GetUsageBatch(r.Context(), kind, slugs)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
	}

	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, jsonapi.NewTagsResponseWithUsage(tags, total, usageBySlug)); err != nil {
		internalServerError(w, r, err)
	}
}

// getTagStats returns the group-wide tag adoption summary that backs the
// Tags page stats bar: total tags + tagged/untagged counts on commodities
// and files.
// @Summary Tag adoption stats
// @Description Returns total tags, plus tagged/untagged counts on commodities and files for the current group.
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.TagStatsResponse "OK"
// @Router /g/{groupSlug}/tags/stats [get].
func (api *tagsAPI) getTagStats(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	stats, err := regSet.TagRegistry.GetStats(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewTagStatsResponse(stats)); err != nil {
		internalServerError(w, r, err)
	}
}

// includeHasToken returns true when the `?include=` query value contains
// the requested token. The handler accepts comma-separated tokens — only
// "usage" is recognised today, but the helper keeps the parsing in one
// place so adding more (e.g. "stats") later is a one-liner.
func includeHasToken(raw, token string) bool {
	if raw == "" {
		return false
	}
	for part := range strings.SplitSeq(raw, ",") {
		if strings.TrimSpace(part) == token {
			return true
		}
	}
	return false
}

// autocompleteTags returns the top-N matching tags ranked by usage and recency.
// @Summary Autocomplete tag suggestions
// @Description Top-N tags of the given kind matching the query, ranked by usage desc + created_at desc.
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param kind query string true "Tag kind (commodity = item-tags, file = file-tags)" Enums(commodity,file)
// @Param q query string false "Substring match against label and slug"
// @Param limit query int false "Maximum suggestions returned" default(10)
// @Success 200 {object} jsonapi.TagAutocompleteResponse "OK"
// @Failure 422 {object} jsonapi.Errors "Missing or invalid kind value"
// @Router /g/{groupSlug}/tags/autocomplete [get].
func (api *tagsAPI) autocompleteTags(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query().Get("q")
	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 50 {
			limit = parsed
		}
	}
	kind, ok := parseTagKind(r.URL.Query().Get("kind"))
	if !ok {
		unprocessableEntityError(w, r, fmt.Errorf("invalid kind: %q (must be one of: commodity, file)", r.URL.Query().Get("kind")))
		return
	}

	tags, err := regSet.TagRegistry.Search(r.Context(), q, limit, kind)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewTagAutocompleteResponse(tags)); err != nil {
		internalServerError(w, r, err)
	}
}

// parseTagKind validates and returns the required tag kind from the raw
// ?kind= query value. Item-tags and file-tags are separate entities, so
// kind is mandatory — there is no "all". Returns (_, false) for an empty
// or unknown value; the handler converts that to 422.
func parseTagKind(raw string) (models.TagKind, bool) {
	candidate := models.TagKind(strings.TrimSpace(raw))
	if !candidate.IsValid() {
		return models.TagKindAny, false
	}
	return candidate, true
}

// getTag returns a tag by id with usage breakdown.
// @Summary Get a tag
// @Description get tag by ID with usage breakdown
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param tagID path string true "Tag ID"
// @Success 200 {object} jsonapi.TagResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Tag not found"
// @Router /g/{groupSlug}/tags/{tagID} [get].
func (api *tagsAPI) getTag(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	tag := tagFromContext(r.Context())
	if tag == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	usage, err := regSet.TagRegistry.GetUsage(r.Context(), tag.Kind, tag.Slug)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewTagResponseWithUsage(tag, usage)); err != nil {
		internalServerError(w, r, err)
	}
}

// createTag creates a new tag.
// @Summary Create a new tag
// @Description add a tag with slug, label, color
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param tag body jsonapi.TagRequest true "Tag object"
// @Success 201 {object} jsonapi.TagResponse "Tag created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/tags [post].
func (api *tagsAPI) createTag(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.TagRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	tag := models.Tag{
		Kind:  input.Data.Attributes.Kind,
		Slug:  input.Data.Attributes.Slug,
		Label: input.Data.Attributes.Label,
		Color: input.Data.Attributes.Color,
	}
	created, err := regSet.TagRegistry.Create(r.Context(), tag)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewTagResponse(created).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// updateTag patches a tag's label/color/slug. Slug change rewrites JSONB
// references on commodities + files.
// @Summary Update a tag
// @Description Patch label/color/slug. Slug change rewrites JSONB references.
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param tagID path string true "Tag ID"
// @Param tag body jsonapi.TagUpdateRequest true "Tag patch payload"
// @Success 200 {object} jsonapi.TagResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Tag not found"
// @Failure 409 {object} jsonapi.Errors "Slug already in use"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/tags/{tagID} [patch].
func (api *tagsAPI) updateTag(w http.ResponseWriter, r *http.Request) {
	tag := tagFromContext(r.Context())
	if tag == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.TagUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	updated, err := api.tagService.RenameTag(
		r.Context(), tag.ID,
		input.Data.Attributes.Label,
		input.Data.Attributes.Slug,
		input.Data.Attributes.Color,
	)
	if err != nil {
		if errors.Is(err, registry.ErrAlreadyExists) {
			conflictError(w, r, err, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewTagResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteTag removes a tag. Returns 409 with usage breakdown when the tag
// has references and force=false. With ?force=true, references are stripped
// from JSONB arrays first.
// @Summary Delete a tag
// @Description Delete tag by ID. Returns 409 with usage breakdown when in use; pass ?force=true to strip references.
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param tagID path string true "Tag ID"
// @Param force query bool false "Strip JSONB references then delete"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Tag not found"
// @Failure 409 {object} jsonapi.Errors "Tag is in use; pass force=true to strip references"
// @Router /g/{groupSlug}/tags/{tagID} [delete].
func (api *tagsAPI) deleteTag(w http.ResponseWriter, r *http.Request) {
	tag := tagFromContext(r.Context())
	if tag == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	usage, err := api.tagService.DeleteTag(r.Context(), tag.ID, force)
	if err != nil {
		if errors.Is(err, services.ErrTagInUse) {
			conflictError(w, r,
				err,
				fmt.Errorf("tag is in use (commodities=%d, files=%d) — pass force=true to strip references",
					usage.Commodities, usage.Files),
			)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Tags returns the chi sub-router for /tags. The autocomplete sub-route is
// mounted before /{tagID} so chi routes the slug to the dedicated handler.
func Tags(params Params) func(r chi.Router) {
	api := &tagsAPI{
		factorySet: params.FactorySet,
		tagService: services.NewTagService(params.FactorySet),
	}
	return func(r chi.Router) {
		r.Get("/", api.listTags)
		r.Post("/", api.createTag)
		r.Get("/autocomplete", api.autocompleteTags)
		r.Get("/stats", api.getTagStats)
		r.Route("/{tagID}", func(r chi.Router) {
			r.Use(tagCtx())
			r.Get("/", api.getTag)
			r.Patch("/", api.updateTag)
			r.Delete("/", api.deleteTag)
		})
	}
}
