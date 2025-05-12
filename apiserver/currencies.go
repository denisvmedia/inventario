package apiserver

import (
	"encoding/json"
	"net/http"

	"github.com/bojanz/currency"
	"github.com/go-chi/chi/v5"
)

type currenciesAPI struct{}

// getCurrencies returns a list of supported currencies.
// @Summary Get supported currencies
// @Description get list of supported currencies
// @Tags currencies
// @Accept  json-api
// @Produce  json
// @Success 200 {array} string "OK"
// @Router /currencies [get].
func (api *currenciesAPI) getCurrencies(w http.ResponseWriter, r *http.Request) {
	// Get all supported currency codes
	currencyCodes := currency.GetCurrencyCodes()

	// Set the content type to application/json
	w.Header().Set("Content-Type", "application/json")

	// Return the array of currency codes directly
	if err := json.NewEncoder(w).Encode(currencyCodes); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// Currencies returns a handler for currencies.
func Currencies() func(r chi.Router) {
	api := &currenciesAPI{}

	return func(r chi.Router) {
		r.Get("/", api.getCurrencies) // GET /currencies
	}
}
