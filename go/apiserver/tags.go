package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

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
// @Description get tags with optional filtering
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param q query string false "Search by label or slug"
// @Param sort query string false "Sort field (label, created_at, usage)" Enums(label,created_at,usage)
// @Param order query string false "Sort direction (asc, desc)" default(asc)
// @Param page query int false "Page number (1-based)" default(1)
// @Param per_page query int false "Items per page" default(50)
// @Success 200 {object} jsonapi.TagsResponse "OK"
// @Router /tags [get].
func (api *tagsAPI) listTags(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	opts := registry.TagListOptions{
		Search:    q.Get("q"),
		SortField: registry.TagSortField(q.Get("sort")),
		SortDesc:  q.Get("order") == "desc",
	}

	tags, total, err := regSet.TagRegistry.ListPaginated(r.Context(), offset, perPage, opts)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, jsonapi.NewTagsResponse(tags, total)); err != nil {
		internalServerError(w, r, err)
	}
}

// autocompleteTags returns the top-N matching tags ranked by usage and recency.
// @Summary Autocomplete tag suggestions
// @Description Top-N tags matching the query, ranked by usage desc + created_at desc.
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param q query string false "Substring match against label and slug"
// @Param limit query int false "Maximum suggestions returned" default(10)
// @Success 200 {object} jsonapi.TagAutocompleteResponse "OK"
// @Router /tags/autocomplete [get].
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

	tags, err := regSet.TagRegistry.Search(r.Context(), q, limit)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewTagAutocompleteResponse(tags)); err != nil {
		internalServerError(w, r, err)
	}
}

// getTag returns a tag by id with usage breakdown.
// @Summary Get a tag
// @Description get tag by ID with usage breakdown
// @Tags tags
// @Accept json-api
// @Produce json-api
// @Param id path string true "Tag ID"
// @Success 200 {object} jsonapi.TagResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Tag not found"
// @Router /tags/{id} [get].
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

	usage, err := regSet.TagRegistry.GetUsage(r.Context(), tag.Slug)
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
// @Param tag body jsonapi.TagRequest true "Tag object"
// @Success 201 {object} jsonapi.TagResponse "Tag created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /tags [post].
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
// @Param id path string true "Tag ID"
// @Param tag body jsonapi.TagUpdateRequest true "Tag patch payload"
// @Success 200 {object} jsonapi.TagResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Tag not found"
// @Failure 409 {object} jsonapi.Errors "Slug already in use"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /tags/{id} [patch].
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
// @Param id path string true "Tag ID"
// @Param force query bool false "Strip JSONB references then delete"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Tag not found"
// @Failure 409 {object} jsonapi.Errors "Tag is in use; pass force=true to strip references"
// @Router /tags/{id} [delete].
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
		r.Route("/{tagID}", func(r chi.Router) {
			r.Use(tagCtx())
			r.Get("/", api.getTag)
			r.Patch("/", api.updateTag)
			r.Delete("/", api.deleteTag)
		})
	}
}
