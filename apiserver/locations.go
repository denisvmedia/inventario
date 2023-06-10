package apiserver

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const locationCtxKey ctxValueKey = "location"

func locationFromContext(ctx context.Context) *models.Location {
	location, ok := ctx.Value(locationCtxKey).(*models.Location)
	if !ok {
		return nil
	}
	return location
}

type locationsAPI struct {
	locationsRegistry registry.LocationRegistry
}

// listLocations lists all locations.
// @Summary List locations
// @Description get locations
// @Tags locations
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.LocationsResponse "OK"
// @Router /locations [get]
func (api *locationsAPI) listLocations(w http.ResponseWriter, r *http.Request) {
	locations, _ := api.locationsRegistry.List()

	if err := render.Render(w, r, jsonapi.NewLocationsResponse(locations, len(locations))); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getLocation gets a location by ID.
// @Summary Get a location
// @Description get location by ID
// @Tags locations
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Location ID"
// @Success 200 {object} jsonapi.LocationResponse "OK"
// @Router /locations/{id} [get]
func (api *locationsAPI) getLocation(w http.ResponseWriter, r *http.Request) {
	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	if err := render.Render(w, r, jsonapi.NewLocationResponse(location)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// Create a new location
// @Summary Create a new location
// @Description add by location data
// @Tags locations
// @Accept json-api
// @Produce json-api
// @Param location body jsonapi.LocationRequest true "Location object"
// @Success 201 {object} jsonapi.LocationResponse "Location created"
// @Failure 404 {object} jsonapi.Errors "Location not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /locations [post]
func (api *locationsAPI) createLocation(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.LocationRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}
	location, err := api.locationsRegistry.Create(*input.Data)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	resp := jsonapi.NewLocationResponse(location).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteLocation deletes a location by ID.
// @Summary Delete a location
// @Description Delete by location ID
// @Tags locations
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Location ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Location not found"
// @Router /locations/{id} [delete]
func (api *locationsAPI) deleteLocation(w http.ResponseWriter, r *http.Request) {
	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	err := api.locationsRegistry.Delete(location.ID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// updateLocation updates a location.
// @Summary Update a location
// @Description Update by location data
// @Tags locations
// @Accept json-api
// @Produce json-api
// @Param id path string true "Location ID"
// @Param location body jsonapi.LocationRequest true "Location object"
// @Success 200 {object} jsonapi.LocationResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Location not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /locations/{id} [put]
func (api *locationsAPI) updateLocation(w http.ResponseWriter, r *http.Request) {
	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.LocationRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if location.ID != input.Data.ID {
		unprocessableEntityError(w, r, nil)
		return
	}

	newLocation, err := api.locationsRegistry.Update(*input.Data)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	resp := jsonapi.NewLocationResponse(newLocation).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func (api *locationsAPI) locationCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		locationID := chi.URLParam(r, "locationID")
		location, err := api.locationsRegistry.Get(locationID)
		switch {
		case err == nil:
		case errors.Is(err, registry.ErrNotFound):
			notFoundError(w, r, err)
			return
		default:
			internalServerError(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), locationCtxKey, location)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Locations(locationsRegistry registry.LocationRegistry) func(r chi.Router) {
	api := &locationsAPI{locationsRegistry: locationsRegistry}
	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listLocations) // GET /locations
		r.Route("/{locationID}", func(r chi.Router) {
			r.Use(api.locationCtx)
			r.Get("/", api.getLocation)       // GET /locations/123
			r.Put("/", api.updateLocation)    // PUT /locations/123
			r.Delete("/", api.deleteLocation) // DELETE /locations/123
		})
		r.Post("/", api.createLocation) // POST /articles
	}
}
