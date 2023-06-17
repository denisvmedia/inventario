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

const commodityCtxKey ctxValueKey = "commodity"

func commodityFromContext(ctx context.Context) *models.Commodity {
	commodity, ok := ctx.Value(commodityCtxKey).(*models.Commodity)
	if !ok {
		return nil
	}
	return commodity
}

type commoditiesAPI struct {
	commodityRegistry registry.CommodityRegistry
}

// listCommodities lists all commodities.
// @Summary List commodities
// @Description get commodities
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.CommoditiesResponse "OK"
// @Router /commodities [get]
func (api *commoditiesAPI) listCommodities(w http.ResponseWriter, r *http.Request) {
	commodities, _ := api.commodityRegistry.List()

	if err := render.Render(w, r, jsonapi.NewCommoditiesResponse(commodities, len(commodities))); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getCommodity gets a commodity by ID.
// @Summary Get a commodity
// @Description get commodity by ID
// @Tags commodities
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Commodity ID"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Router /commodities/{id} [get]
func (api *commoditiesAPI) getCommodity(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	resp := jsonapi.NewCommodityResponse(commodity)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createCommodity creates a new commodity.
// @Summary Create a new commodity
// @Description Add a new commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodity body jsonapi.CommodityRequest true "Commodity object"
// @Success 201 {object} jsonapi.CommodityResponse "Commodity created"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /commodities [post]
func (api *commoditiesAPI) createCommodity(w http.ResponseWriter, r *http.Request) {
	var input jsonapi.CommodityRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	commodity, err := api.commodityRegistry.Create(*input.Data)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewCommodityResponse(commodity).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteCommodity deletes a commodity by ID.
// @Summary Delete a commodity
// @Description Delete a commodity by ID
// @Tags commodities
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Commodity ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Router /commodities/{id} [delete]
func (api *commoditiesAPI) deleteCommodity(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	err := api.commodityRegistry.Delete(commodity.ID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// updateCommodity updates a commodity.
// @Summary Update a commodity
// @Description Update a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param id path string true "Commodity ID"
// @Param commodity body jsonapi.CommodityRequest true "Commodity object"
// @Success 200 {object} jsonapi.CommodityResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /commodities/{id} [put]
func (api *commoditiesAPI) updateCommodity(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.CommodityRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if commodity.ID != input.Data.ID {
		unprocessableEntityError(w, r, nil)
		return
	}

	updatedCommodity, err := api.commodityRegistry.Update(*input.Data)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	resp := jsonapi.NewCommodityResponse(updatedCommodity).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func (api *commoditiesAPI) commodityCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		commodityID := chi.URLParam(r, "commodityID")
		commodity, err := api.commodityRegistry.Get(commodityID)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), commodityCtxKey, commodity)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func Commodities(commodityRegistry registry.CommodityRegistry) func(r chi.Router) {
	api := &commoditiesAPI{
		commodityRegistry: commodityRegistry,
	}
	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listCommodities) // GET /commodities
		r.Route("/{commodityID}", func(r chi.Router) {
			r.Use(api.commodityCtx)
			r.Get("/", api.getCommodity)       // GET /commodities/123
			r.Put("/", api.updateCommodity)    // PUT /commodities/123
			r.Delete("/", api.deleteCommodity) // DELETE /commodities/123
		})
		r.Post("/", api.createCommodity) // POST /commodities
	}
}
