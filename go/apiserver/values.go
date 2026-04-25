package apiserver

import (
	"net/http"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/valuation"
	"github.com/denisvmedia/inventario/jsonapi"
)

type valuesAPI struct {
}

// getValues returns the total value of commodities.
// @Summary Get total value of commodities
// @Description Get the total value of commodities globally, by location, and by area
// @Tags commodities
// @Accept json
// @Produce json-api
// @Success 200 {object} jsonapi.ValueResponse "OK"
// @Router /commodities/values [get]
func (api *valuesAPI) getValues(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	// Create a valuator
	valuator := valuation.NewValuator(r.Context(), registrySet)

	// Calculate global total
	globalTotal, err := valuator.CalculateGlobalTotalValue()
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Calculate totals by location
	locationTotals, err := valuator.CalculateTotalValueByLocation()
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Calculate totals by area
	areaTotals, err := valuator.CalculateTotalValueByArea()
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
		globalTotal,
		buildNamedTotals(locationTotals, locationNames),
		buildNamedTotals(areaTotals, areaNames),
	)

	// Render response
	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// buildNamedTotals zips a `id → value` map with a `id → name` lookup
// into the API's `[]NamedTotal` shape, sorted by descending value so
// the frontend can slice a top-N without a second sort. Entries with
// no matching name fall back to the empty string — the frontend will
// render them as "Unknown".
func buildNamedTotals(totals map[string]decimal.Decimal, names map[string]string) []jsonapi.NamedTotal {
	out := make([]jsonapi.NamedTotal, 0, len(totals))
	for id, value := range totals {
		out = append(out, jsonapi.NamedTotal{
			ID:    id,
			Name:  names[id],
			Value: value,
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
