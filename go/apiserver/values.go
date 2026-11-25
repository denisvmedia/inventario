package apiserver

import (
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"go.5x5.cz/inventario/internal/valuation"
	"go.5x5.cz/inventario/jsonapi"
)

type valuesAPI struct {
}

// getValues returns the total value of commodities.
// @Summary Get total value of commodities
// @Description Get the total value of commodities globally, by location, and by area.
// @Description Only in-use, non-draft items count. An item's value is its current price,
// @Description or its purchase price in the group currency when the current price is
// @Description unknown; a known zero counts. Items with no known value are not added as
// @Description zero: unvalued_count reports how many each total leaves out.
// @Tags commodities
// @Accept json
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Success 200 {object} jsonapi.ValueResponse "OK"
// @Router /g/{groupSlug}/commodities/values [get]
func (api *valuesAPI) getValues(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	// Create a valuator
	valuator := valuation.NewValuator(r.Context(), registrySet)

	totals, err := valuator.Calculate()
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Embed entity names so the dashboard can render top-N grouping cards
	// without a follow-up `/locations` + `/areas` walk (issue #1330 Copilot
	// review). The valuator owns its own dataset reads, but the registries
	// are cheap (already cached for this request) and the join is
	// O(locations) + O(areas).
	locations, err := registrySet.LocationRegistry.List(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	areas, err := registrySet.AreaRegistry.List(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	locationNames := make(map[string]string, len(locations))
	for _, l := range locations {
		locationNames[l.ID] = l.Name
	}
	areaNames := make(map[string]string, len(areas))
	for _, a := range areas {
		areaNames[a.ID] = a.Name
	}

	// Create response
	response := jsonapi.NewValueResponse(
		totals.Global.Value,
		totals.Global.ValuedCount,
		totals.Global.UnvaluedCount,
		buildNamedTotals(totals.ByLocation, locationNames),
		buildNamedTotals(totals.ByArea, areaNames),
	)

	// Render response
	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// buildNamedTotals zips a `id → total` map with a `id → name` lookup
// into the API's `[]NamedTotal` shape, sorted by descending value so
// the frontend can slice a top-N without a second sort. Entries with
// no matching name fall back to the empty string — the frontend will
// render them as "Unknown". An entry whose items all lack a known value
// is kept with a zero value, so its unvalued count still reaches the UI.
func buildNamedTotals(totals map[string]valuation.Total, names map[string]string) []jsonapi.NamedTotal {
	out := make([]jsonapi.NamedTotal, 0, len(totals))
	for id, total := range totals {
		out = append(out, jsonapi.NamedTotal{
			ID:            id,
			Name:          names[id],
			Value:         total.Value,
			UnvaluedCount: total.UnvaluedCount,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Value.Equal(out[j].Value) {
			return out[i].ID < out[j].ID
		}
		return out[i].Value.GreaterThan(out[j].Value)
	})
	return out
}

// Values returns a handler for commodity values.
func Values() func(r chi.Router) {
	api := &valuesAPI{}

	return func(r chi.Router) {
		r.Get("/", api.getValues) // GET /commodities/values
	}
}
