package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/internal/seeddata"
	"github.com/denisvmedia/inventario/registry"
)

type seedAPI struct {
	registrySet *registry.Set
}

// seedDatabase seeds the database with example data.
// @Summary Seed database
// @Description Seed the database with example data
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "OK"
// @Router /seed [post].
func (api *seedAPI) seedDatabase(w http.ResponseWriter, r *http.Request) {
	err := seeddata.SeedData(api.registrySet)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	render.JSON(w, r, map[string]string{"status": "success", "message": "Database seeded successfully"})
}

// Seed returns a handler for seeding the database.
func Seed(registrySet *registry.Set) func(r chi.Router) {
	api := &seedAPI{
		registrySet: registrySet,
	}

	return func(r chi.Router) {
		r.Post("/", api.seedDatabase) // POST /seed
	}
}
