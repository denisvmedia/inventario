package apiserver

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services/commodityquery"
)

// searchAPI handles search-related endpoints
type searchAPI struct {
	registrySet any
}

// Search provides advanced search capabilities across different entities
// @Summary Advanced search
// @Description Search one entity type. For commodities, the query string is the item query contract shared with GET /commodities (services/commodityquery).
// @Description Every filter parameter of that endpoint except `type` (taken here by the entity type) is accepted, and an unknown parameter or an unreadable value is a 400.
// @Description Drafts and inactive items are included unless include_inactive=false.
// @Tags search
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param q query string true "Search text; for commodities, white-space separated terms that must all match (see GET /commodities)"
// @Param type query string false "Entity type to search" Enums(commodities,files,areas,locations)
// @Param limit query int false "Maximum number of results" default(20)
// @Param offset query int false "Number of results to skip" default(0)
// @Param tags query string false "Filter by tags (comma-separated); for commodities see tag_mode"
// @Param tag_mode query string false "Commodities only: how tags combine, all (default) or any" Enums(all,any)
// @Success 200 {object} jsonapi.SearchResponse "Search results"
// @Failure 400 {object} jsonapi.Errors "Missing q, unsupported parameter or invalid value"
// @Router /g/{groupSlug}/search [get]
func (api *searchAPI) search(w http.ResponseWriter, r *http.Request) {
	values := r.URL.Query()
	query := values.Get("q")
	if strings.TrimSpace(query) == "" {
		http.Error(w, "query parameter 'q' is required", http.StatusBadRequest)
		return
	}

	entityType := values.Get("type")
	if entityType == "" {
		entityType = "commodities" // Default to commodities
	}

	limit := 20
	if limitParam := values.Get("limit"); limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := 0
	if offsetParam := values.Get("offset"); offsetParam != "" {
		if o, err := strconv.Atoi(offsetParam); err == nil && o >= 0 {
			offset = o
		}
	}

	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	switch entityType {
	case "commodities":
		api.searchCommodities(w, r, registrySet, limit, offset)
	case "files":
		api.searchFiles(w, r, registrySet, query, limit, offset)
	default:
		http.Error(w, "unsupported entity type for basic search", http.StatusBadRequest)
	}
}

// searchCommodities answers /search?type=commodities through the item query
// contract, so the search page and the items list select the same items for
// the same query string.
func (*searchAPI) searchCommodities(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, limit, offset int) {
	query, err := commodityquery.Parse(r.URL.Query(), "type", "limit", "offset")
	if err != nil {
		queryParamError(w, r, err)
		return
	}
	result, err := commodityquery.List(r.Context(), registrySet, query, offset, limit)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	response := jsonapi.NewSearchResponse("commodities", result.Commodities, result.Total).WithMatches(result.Matches)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
	}
}

func (*searchAPI) searchFiles(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, query string, limit, offset int) {
	var tags []string
	if tagsParam := r.URL.Query().Get("tags"); tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	files, err := registrySet.FileRegistry.Search(r.Context(), query, nil, nil, tags, nil, nil)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Count before slicing: the total is the match count, not the page
	// size, or nothing past the first page is reachable (#2129).
	total := len(files)
	start := min(offset, total)
	end := min(start+limit, total)
	files = files[start:end]

	response := jsonapi.NewSearchResponse("files", files, total)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
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
