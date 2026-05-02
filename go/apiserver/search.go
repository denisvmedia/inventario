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

	// Pull the user-aware registry from the request context (the same
	// pattern every other group-scoped handler uses). The previous
	// type-asserting `api.registrySet.(*registry.Set)` returned 500
	// because the constructor receives `params.EntityService`, not a
	// `*registry.Set`. The context-based lookup hits the right object
	// regardless of how the route is wired.
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	api.searchWithRegistry(w, r, registrySet, query, entityType, limit, offset, tags, tagOperator)
}

func (api *searchAPI) searchWithRegistry(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, query, entityType string, limit, offset int, tags []string, _tagOperator registry.TagOperator) {
	// Route every supported entity type through the same in-memory
	// fallback. The optimised registry full-text / tag-search variants
	// were stubbed out previously; the basic List+filter path is enough
	// for the Cmd+K command palette (#1330 PR 5.4) which only exercises
	// commodities + files. Areas / locations fall through to the
	// fallback's default branch and return a 400 — the frontend keeps
	// querying just commodities + files.
	api.searchWithBasicFallback(w, r, registrySet, query, entityType, limit, offset, tags)
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
		start := min(offset, len(filtered))
		end := min(start+limit, len(filtered))
		filtered = filtered[start:end]

		response := jsonapi.NewSearchResponse("commodities", filtered, len(filtered))
		if err := render.Render(w, r, response); err != nil {
			internalServerError(w, r, err)
		}

	case "files":
		files, err := registrySet.FileRegistry.Search(r.Context(), query, nil, nil, tags, nil, nil)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}

		// Apply pagination
		start := min(offset, len(files))
		end := min(start+limit, len(files))
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
