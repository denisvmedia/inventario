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

func (api *locationsAPI) listLocations(w http.ResponseWriter, r *http.Request) {
	locations, _ := api.locationsRegistry.List()

	if err := render.Render(w, r, jsonapi.NewLocationsResponse(locations, len(locations))); err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
		return
	}
}

func (api *locationsAPI) getLocation(w http.ResponseWriter, r *http.Request) {
	location := locationFromContext(r.Context())
	if location == nil {
		render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(nil)))
		return
	}
	if err := render.Render(w, r, jsonapi.NewLocationResponse(location)); err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
		return
	}
}

func (api *locationsAPI) createLocation(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.LocationRequest
	if err := render.Bind(r, &input); err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(err)))
		return
	}
	location, err := api.locationsRegistry.Create(*input.Data)
	if err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
		return
	}
	resp := jsonapi.NewLocationResponse(location).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
		return
	}
}

func (api *locationsAPI) deleteLocation(w http.ResponseWriter, r *http.Request) {
	location := locationFromContext(r.Context())
	if location == nil {
		render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(nil)))
		return
	}

	err := api.locationsRegistry.Delete(location.ID)
	if err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (api *locationsAPI) updateLocation(w http.ResponseWriter, r *http.Request) {
	location := locationFromContext(r.Context())
	if location == nil {
		render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(nil)))
		return
	}

	var input jsonapi.LocationRequest
	if err := render.Bind(r, &input); err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(err)))
		return
	}

	if location.ID != input.Data.ID {
		render.Render(w, r, jsonapi.NewErrors(NewUnprocessableEntityError(nil)))
		return
	}

	newLocation, err := api.locationsRegistry.Update(*input.Data)
	if err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
		return
	}
	resp := jsonapi.NewLocationResponse(newLocation).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
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
			render.Render(w, r, jsonapi.NewErrors(NewNotFoundError(err)))
			return
		default:
			render.Render(w, r, jsonapi.NewErrors(NewInternalServerError(err)))
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
