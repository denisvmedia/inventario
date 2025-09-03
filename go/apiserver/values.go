package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

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
	valuator := valuation.NewValuator(registrySet)

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

	// Create response
	response := jsonapi.NewValueResponse(globalTotal, locationTotals, areaTotals)

	// Render response
	render.Status(r, http.StatusOK)
	render.Render(w, r, response)
}

// Values returns a handler for commodity values.
func Values() func(r chi.Router) {
	api := &valuesAPI{}

	return func(r chi.Router) {
		r.Get("/", api.getValues) // GET /commodities/values
	}
}
