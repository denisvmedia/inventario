package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/valuation"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/registry"
)

type valuesAPI struct {
	registrySet *registry.Set
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
	user, err := appctx.RequireUserFromContext(r.Context())
	if err != nil {
		unauthorizedError(w, r, err)
		return
	}

	// Create a valuator
	valuator := valuation.NewValuator(api.registrySet, user)

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
func Values(registrySet *registry.Set) func(r chi.Router) {
	api := &valuesAPI{
		registrySet: registrySet,
	}

	return func(r chi.Router) {
		r.Get("/", api.getValues) // GET /commodities/values
	}
}
