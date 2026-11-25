package apiserver

import (
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"go.5x5.cz/inventario/jsonapi"
	"go.5x5.cz/inventario/models"
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
// @Description Locations match on name and address and areas on name: every white-space separated term of q has to be found, ignoring case; results are sorted by name.
// @Description Tags (item and file tags) match when q is found in the label or the slug, ignoring case; results are sorted by label.
// @Tags search
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param q query string true "Search text; for commodities, white-space separated terms that must all match (see GET /commodities)"
// @Param type query string false "Entity type to search" Enums(commodities,files,areas,locations,tags)
// @Param limit query int false "Maximum number of results" default(20)
// @Param offset query int false "Number of results to skip" default(0)
// @Param tags query string false "Filter by tags (comma-separated); for commodities see tag_mode"
// @Param tag_mode query string false "Commodities only: how tags combine, all (default) or any" Enums(all,any)
// @Param tags_exclude query []string false "Commodities only: only items with none of these tag slugs (comma-separated or repeated)" collectionFormat(multi)
// @Param no_tags query bool false "Commodities only: only items with no tags; cannot be combined with tags or tags_exclude"
// @Param status query []string false "Commodities only: filter by status (in_use, sold, lost, disposed, written_off); repeat to OR" collectionFormat(multi)
// @Param area_id query string false "Commodities only: filter by exact area ID"
// @Param unassigned query bool false "Commodities only: only items with no area (ignored when area_id is set)"
// @Param include_inactive query bool false "Commodities only: include drafts and non-in_use items (default true; false hides them)"
// @Param warranty_status query []string false "Commodities only: filter by computed warranty status (active, expiring, expired, none); repeat to OR" collectionFormat(multi)
// @Param warranty_expires_before query string false "Commodities only: restrict to items whose warranty expires strictly before YYYY-MM-DD"
// @Param lent_out query bool false "Commodities only: true = only items currently lent, false = only items not lent"
// @Param trashed query bool false "Commodities only: true searches the items in Trash instead of the live ones (default false); cannot be combined with lent_out"
// @Param sort query string false "Commodities only: sort field — name|registered_date|purchase_date|current_price|original_price|count, prefix with '-' for descending"
// @Success 200 {object} jsonapi.SearchResponse "Search results"
// @Failure 400 {object} jsonapi.Errors "Missing q, unsupported parameter or invalid value"
// @Router /g/{groupSlug}/search [get]
func (api *searchAPI) search(w http.ResponseWriter, r *http.Request) {
	values, err := commodityquery.DecodeQuery(r.URL.RawQuery)
	if err != nil {
		queryParamError(w, r, err)
		return
	}
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
		api.searchCommodities(w, r, registrySet, values, limit, offset)
	case "files":
		api.searchFiles(w, r, registrySet, query, limit, offset)
	case "locations":
		api.searchLocations(w, r, registrySet, query, limit, offset)
	case "areas":
		api.searchAreas(w, r, registrySet, query, limit, offset)
	case "tags":
		api.searchTags(w, r, registrySet, query, limit, offset)
	default:
		http.Error(w, "unsupported entity type for basic search", http.StatusBadRequest)
	}
}

// searchCommodities answers /search?type=commodities through the item query
// contract, so the search page and the items list select the same items for
// the same query string.
func (*searchAPI) searchCommodities(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, values url.Values, limit, offset int) {
	query, err := commodityquery.Parse(values, "type", "limit", "offset")
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

// searchLocations answers /search?type=locations. A group holds few
// locations, so they are listed and matched here rather than in each backend.
func (*searchAPI) searchLocations(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, query string, limit, offset int) {
	locations, err := registrySet.LocationRegistry.List(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	terms := registry.SplitSearchTerms(query)
	matched := make([]*models.Location, 0, len(locations))
	for _, location := range locations {
		if matchesEveryTerm(terms, location.Name, location.Address) {
			matched = append(matched, location)
		}
	}
	slices.SortStableFunc(matched, func(a, b *models.Location) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	renderSearchPage(w, r, "locations", matched, limit, offset)
}

// searchAreas answers /search?type=areas, matched on the area name the same
// way as locations.
func (*searchAPI) searchAreas(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, query string, limit, offset int) {
	areas, err := registrySet.AreaRegistry.List(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	terms := registry.SplitSearchTerms(query)
	matched := make([]*models.Area, 0, len(areas))
	for _, area := range areas {
		if matchesEveryTerm(terms, area.Name) {
			matched = append(matched, area)
		}
	}
	slices.SortStableFunc(matched, func(a, b *models.Area) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	renderSearchPage(w, r, "areas", matched, limit, offset)
}

// searchTags answers /search?type=tags with the tags of both kinds whose
// label or slug contains q, as the tags list matches its own search box.
func (*searchAPI) searchTags(w http.ResponseWriter, r *http.Request, registrySet *registry.Set, query string, limit, offset int) {
	tags, total, err := registrySet.TagRegistry.ListPaginated(r.Context(), offset, limit, registry.TagListOptions{
		Search: strings.TrimSpace(query),
		Kind:   models.TagKindAny,
	})
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	response := jsonapi.NewSearchResponse("tags", tags, total)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
	}
}

// matchesEveryTerm reports whether every term is found, ignoring case, in at
// least one of the fields. No terms match everything.
func matchesEveryTerm(terms []string, fields ...string) bool {
	folded := make([]string, len(fields))
	for i, field := range fields {
		folded[i] = foldCase(field)
	}
	for _, term := range terms {
		needle := foldCase(term)
		if !slices.ContainsFunc(folded, func(field string) bool {
			return strings.Contains(field, needle)
		}) {
			return false
		}
	}
	return true
}

// foldCase maps every rune of s to the lowest rune of its Unicode case
// folding orbit (unicode.SimpleFold), so two strings that strings.EqualFold
// calls equal fold to the same string, and a substring search on folded
// strings ignores case. strings.ToLower is not enough: it keeps the final
// sigma ς apart from σ, the lower case of Σ.
func foldCase(s string) string {
	return strings.Map(func(r rune) rune {
		lowest := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			lowest = min(lowest, f)
		}
		return lowest
	}, s)
}

// renderSearchPage renders one page of rows, with the number of all rows as
// the total.
func renderSearchPage[T any](w http.ResponseWriter, r *http.Request, entityType string, rows []T, limit, offset int) {
	total := len(rows)
	start := min(offset, total)
	end := min(start+limit, total)
	response := jsonapi.NewSearchResponse(entityType, rows[start:end], total)
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
