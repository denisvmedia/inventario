package apiserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// searchAPI handles search-related endpoints
type searchAPI struct {
	registrySet any
}

// Search provides advanced search capabilities across different entities
// @Summary Advanced search
// @Description Perform advanced search across commodities, files, and other entities
// @Tags search
// @Accept json-api
// @Produce json-api
// @Param q query string true "Search query"
// @Param type query string false "Entity type to search" Enums(commodities,files,areas,locations)
// @Param limit query int false "Maximum number of results" default(20)
// @Param offset query int false "Number of results to skip" default(0)
// @Param tags query string false "Filter by tags (comma-separated)"
// @Param operator query string false "Tag operator" Enums(AND,OR) default(OR)
// @Success 200 {object} jsonapi.SearchResponse "Search results"
// @Router /search [get]
func (api *searchAPI) search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	entityType := r.URL.Query().Get("type")
	if entityType == "" {
		entityType = "commodities" // Default to commodities
	}

	// Parse pagination parameters
	limit := 20
	if limitParam := r.URL.Query().Get("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetParam := r.URL.Query().Get("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse tags
	var tags []string
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	// Parse tag operator
	tagOperator := registry.TagOperatorOR
	if operatorParam := r.URL.Query().Get("operator"); operatorParam == "AND" {
		tagOperator = registry.TagOperatorAND
	}

	// Use the registry set directly (PostgreSQL-first approach)
	if registrySet, ok := api.registrySet.(*registry.Set); ok {
		api.searchWithRegistry(w, r, registrySet, query, entityType, limit, offset, tags, tagOperator)
		return
	}

	http.Error(w, "unsupported registry type", http.StatusInternalServerError)
}

func (api *searchAPI) searchWithRegistry(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, query, entityType string, limit, offset int, tags []string, tagOperator registry.TagOperator) {
	searchOptions := []registry.SearchOption{
		registry.WithLimit(limit),
		registry.WithOffset(offset),
	}

	switch entityType {
	case "commodities":
		var commodities []*models.Commodity
		var err error

		if len(tags) > 0 {
			// Search by tags using the enhanced methods now available in the base interface
			commodities, err = registrySet.CommodityRegistry.SearchByTags(r.Context(), tags, tagOperator)
		} else {
			// Full-text search using the enhanced methods now available in the base interface
			commodities, err = registrySet.CommodityRegistry.FullTextSearch(r.Context(), query, searchOptions...)
		}

		if err != nil {
			renderEntityError(w, r, err)
			return
		}

		response := jsonapi.NewSearchResponse("commodities", commodities, len(commodities))
		if err := render.Render(w, r, response); err != nil {
			internalServerError(w, r, err)
		}

	case "files":
		files, err := registrySet.FileRegistry.FullTextSearch(r.Context(), query, nil, searchOptions...)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}

		response := jsonapi.NewSearchResponse("files", files, len(files))
		if err := render.Render(w, r, response); err != nil {
			internalServerError(w, r, err)
		}

	case "areas":
		areas, err := registrySet.AreaRegistry.SearchByName(r.Context(), query)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}

		response := jsonapi.NewSearchResponse("areas", areas, len(areas))
		if err := render.Render(w, r, response); err != nil {
			internalServerError(w, r, err)
		}

	case "locations":
		locations, err := registrySet.LocationRegistry.SearchByName(r.Context(), query)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}

		response := jsonapi.NewSearchResponse("locations", locations, len(locations))
		if err := render.Render(w, r, response); err != nil {
			internalServerError(w, r, err)
		}

	default:
		// Fallback to basic search for unsupported entity types
		api.searchWithBasicFallback(w, r, registrySet, query, entityType, limit, offset, tags)
	}
}

func (api *searchAPI) searchWithBasicFallback(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, query, entityType string, limit, offset int, tags []string) {
	switch entityType {
	case "commodities":
		// Fallback to basic commodity search
		commodities, err := registrySet.CommodityRegistry.List(r.Context())
		if err != nil {
			renderEntityError(w, r, err)
			return
		}

		// Simple in-memory filtering
		query = strings.ToLower(query)
		var filtered []*models.Commodity
		for _, commodity := range commodities {
			if strings.Contains(strings.ToLower(commodity.Name), query) ||
				strings.Contains(strings.ToLower(commodity.ShortName), query) ||
				strings.Contains(strings.ToLower(commodity.Comments), query) {
				filtered = append(filtered, commodity)
			}
		}

		// Apply pagination
		start := offset
		if start > len(filtered) {
			start = len(filtered)
		}
		end := start + limit
		if end > len(filtered) {
			end = len(filtered)
		}
		filtered = filtered[start:end]

		response := jsonapi.NewSearchResponse("commodities", filtered, len(filtered))
		if err := render.Render(w, r, response); err != nil {
			internalServerError(w, r, err)
		}

	case "files":
		files, err := registrySet.FileRegistry.Search(r.Context(), query, nil, tags)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}

		// Apply pagination
		start := offset
		if start > len(files) {
			start = len(files)
		}
		end := start + limit
		if end > len(files) {
			end = len(files)
		}
		files = files[start:end]

		response := jsonapi.NewSearchResponse("files", files, len(files))
		if err := render.Render(w, r, response); err != nil {
			internalServerError(w, r, err)
		}

	default:
		http.Error(w, "unsupported entity type for basic search", http.StatusBadRequest)
	}
}

// Search creates the search router
func Search(registrySet any) func(r chi.Router) {
	api := &searchAPI{
		registrySet: registrySet,
	}

	return func(r chi.Router) {
		r.Get("/", api.search) // GET /search
	}
}
