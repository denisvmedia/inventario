package apiserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func areaFromContext(ctx context.Context) *models.Area {
	area, ok := ctx.Value(areaCtxKey).(*models.Area)
	if !ok {
		return nil
	}
	return area
}

type areasAPI struct {
	areaRegistry registry.AreaRegistry
}

// listAreas lists all areas.
// @Summary List areas
// @Description get areas
// @Tags areas
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.AreasResponse "OK"
// @Router /areas [get].
func (api *areasAPI) listAreas(w http.ResponseWriter, r *http.Request) {
	areaRegistry, err := api.areaRegistry.WithCurrentUser(r.Context())
	if err != nil {
		unauthorizedError(w, r, err)
		return
	}
	areas, err := areaRegistry.List(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewAreasResponse(areas, len(areas))); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getArea gets an area by ID.
// @Summary Get an area
// @Description get area by ID
// @Tags areas
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Area ID"
// @Success 200 {object} jsonapi.AreaResponse "OK"
// @Router /areas/{id} [get].
func (*areasAPI) getArea(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	area := areaFromContext(r.Context())
	if area == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	resp := jsonapi.NewAreaResponse(area)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createArea creates a new area.
// @Summary Create a new area
// @Description add by area data
// @Tags areas
// @Accept json-api
// @Produce json-api
// @Param area body jsonapi.AreaRequest true "Area object"
// @Success 201 {object} jsonapi.AreaResponse "Area created"
// @Failure 404 {object} jsonapi.Errors "Area not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /areas [post].
func (api *areasAPI) createArea(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.AreaRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Extract user from authenticated request context
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User context required", http.StatusInternalServerError)
		return
	}

	area := *input.Data.Attributes
	if area.TenantID == "" {
		area.TenantID = user.TenantID
	}

	// Use CreateWithUser to ensure proper user context and validation
	ctx := registry.WithUserContext(r.Context(), user.ID)
	areaReg, err := api.areaRegistry.WithCurrentUser(ctx)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	createdArea, err := areaReg.Create(ctx, area)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewAreaResponse(createdArea).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteArea deletes an area by ID.
// @Summary Delete an area
// @Description Delete by area ID
// @Tags areas
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Area ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Area not found"
// @Router /areas/{id} [delete].
func (api *areasAPI) deleteArea(w http.ResponseWriter, r *http.Request) {
	area := areaFromContext(r.Context())
	if area == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	ctx := r.Context()
	areaReg, err := api.areaRegistry.WithCurrentUser(ctx)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	err = areaReg.Delete(ctx, area.ID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// updateArea updates a area.
// @Summary Update a area
// @Description Update by area data
// @Tags areas
// @Accept json-api
// @Produce json-api
// @Param id path string true "Area ID"
// @Param area body jsonapi.AreaRequest true "Area object"
// @Success 200 {object} jsonapi.AreaResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Area not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /areas/{id} [put].
func (api *areasAPI) updateArea(w http.ResponseWriter, r *http.Request) {
	area := areaFromContext(r.Context())
	if area == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.AreaRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if area.ID != input.Data.ID {
		unprocessableEntityError(w, r, nil)
		return
	}

	// Preserve tenant_id and user_id from the existing area
	// This ensures the foreign key constraints are satisfied during updates
	updateData := *input.Data.Attributes
	if updateData.TenantID == "" {
		updateData.TenantID = area.TenantID
	}

	// Use UpdateWithUser to ensure proper user context and validation
	ctx := r.Context()
	areaReg, err := api.areaRegistry.WithCurrentUser(ctx)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	newArea, err := areaReg.Update(ctx, updateData)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewAreaResponse(newArea).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func Areas(areaRegistry registry.AreaRegistry) func(r chi.Router) {
	api := &areasAPI{
		areaRegistry: areaRegistry,
	}
	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listAreas) // GET /areas
		r.Route("/{areaID}", func(r chi.Router) {
			r.Use(areaCtx(areaRegistry))
			r.Get("/", api.getArea)       // GET /areas/123
			r.Put("/", api.updateArea)    // PUT /areas/123
			r.Delete("/", api.deleteArea) // DELETE /areas/123
		})
		r.Post("/", api.createArea) // POST /areas
	}
}
