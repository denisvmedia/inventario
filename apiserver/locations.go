package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/registry"
)

type locationsAPI struct {
	locationRegistry registry.LocationRegistry
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
	locations, _ := api.locationRegistry.List()

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
func (api *locationsAPI) getLocation(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	location := locationFromContext(r.Context())
	if location == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	respLocation := &jsonapi.Location{
		Location: location,
		Areas:    api.locationRegistry.GetAreas(location.ID),
	}

	if err := render.Render(w, r, jsonapi.NewLocationResponse(respLocation)); err != nil {
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
	location, err := api.locationRegistry.Create(*input.Data.Attributes)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	respLocation := &jsonapi.Location{
		Location: location,
		Areas:    api.locationRegistry.GetAreas(location.ID),
	}

	resp := jsonapi.NewLocationResponse(respLocation).WithStatusCode(http.StatusCreated)
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

	err := api.locationRegistry.Delete(location.ID)
	if err != nil {
		renderEntityError(w, r, err)
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

	if location.ID != input.Data.Attributes.ID {
		unprocessableEntityError(w, r, nil)
		return
	}

	newLocation, err := api.locationRegistry.Update(*input.Data.Attributes)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	respLocation := &jsonapi.Location{
		Location: newLocation,
		Areas:    api.locationRegistry.GetAreas(location.ID),
	}

	resp := jsonapi.NewLocationResponse(respLocation).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func Locations(locationRegistry registry.LocationRegistry) func(r chi.Router) {
	api := &locationsAPI{locationRegistry: locationRegistry}
	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listLocations) // GET /locations
		r.Route("/{locationID}", func(r chi.Router) {
			r.Use(locationCtx(locationRegistry))
			r.Get("/", api.getLocation)       // GET /locations/123
			r.Put("/", api.updateLocation)    // PUT /locations/123
			r.Delete("/", api.deleteLocation) // DELETE /locations/123
		})
		r.Post("/", api.createLocation) // POST /areas
	}
}
