package apiserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

const commodityServiceCtxKey ctxValueKey = "commodity_service"

func serviceFromContext(ctx context.Context) *models.CommodityService {
	svc, ok := ctx.Value(commodityServiceCtxKey).(*models.CommodityService)
	if !ok {
		return nil
	}
	return svc
}

// serviceCtx loads the service row referenced by the {serviceID} URL
// param into the request context. Mirrors loanCtx.
func serviceCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			serviceID := chi.URLParam(r, "serviceID")
			svc, err := regSet.CommodityServiceRegistry.Get(r.Context(), serviceID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			// Defence-in-depth: when mounted under
			// /commodities/{commodityID}/services/{serviceID}, the row
			// must belong to that commodity. Mismatch → 404 (don't leak
			// existence of the cross-commodity row).
			if commodityID := chi.URLParam(r, "commodityID"); commodityID != "" && svc.CommodityID != commodityID {
				renderEntityError(w, r, registry.ErrNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), commodityServiceCtxKey, svc)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type commodityServicesAPI struct {
	factorySet     *registry.FactorySet
	serviceService *services.CommodityServiceService
}

// listCommodityServices returns all service rows (open + completed) for
// the commodity in the URL path, most-recent-first.
//
// @Summary List service rows for a commodity
// @Description All service rows (open + completed) for the commodity, most-recent-first.
// @Tags commodity_services
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.CommodityServicesResponse "OK"
// @Router /g/{groupSlug}/commodities/{commodityID}/services [get].
func (api *commodityServicesAPI) listCommodityServices(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	commodityID := chi.URLParam(r, "commodityID")
	rows, err := regSet.CommodityServiceRegistry.ListByCommodity(r.Context(), commodityID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityServicesResponse(rows, len(rows))); err != nil {
		internalServerError(w, r, err)
	}
}

// createCommodityService opens a new service row for the commodity in
// the URL. Returns 409 when an open row already exists or when the
// commodity has an open loan (cross-kind invariant from #1508).
//
// @Summary Send a commodity for service
// @Description Open a new service row. Returns 409 if one is already open or the commodity is currently lent out.
// @Tags commodity_services
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param service body jsonapi.CommodityServiceRequest true "Service attributes"
// @Success 201 {object} jsonapi.CommodityServiceResponse "Service created"
// @Failure 409 {object} jsonapi.Errors "Commodity already has an open holding"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/services [post].
func (api *commodityServicesAPI) createCommodityService(w http.ResponseWriter, r *http.Request) {
	commodityID := chi.URLParam(r, "commodityID")

	var input jsonapi.CommodityServiceRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	svc := models.CommodityService{
		CommodityID:      commodityID,
		ProviderName:     input.Data.Attributes.ProviderName,
		ProviderContact:  input.Data.Attributes.ProviderContact,
		Reason:           input.Data.Attributes.Reason,
		SentAt:           input.Data.Attributes.SentAt,
		ExpectedReturnAt: input.Data.Attributes.ExpectedReturnAt,
		CostCurrency:     input.Data.Attributes.CostCurrency,
	}
	if input.Data.Attributes.CostAmount != nil {
		svc.CostAmount = *input.Data.Attributes.CostAmount
	}

	created, existing, crossHolding, err := api.serviceService.StartService(r.Context(), svc)
	if err != nil {
		if errors.Is(err, services.ErrServiceAlreadyOpen) {
			conflictError(w, r,
				err,
				fmt.Errorf("commodity already has an open service (service_id=%s)", existing.ID),
			)
			return
		}
		if errors.Is(err, services.ErrCommodityAlreadyOut) && crossHolding != nil {
			conflictError(w, r,
				err,
				fmt.Errorf("commodity is already out (kind=%s, id=%s, party=%s)", crossHolding.Kind, crossHolding.ID, crossHolding.PartyName),
			)
			return
		}
		// Model validation (cost-pair / ISO 4217 / length caps) routes to
		// 422 with the offending field path. Any other error stays in the
		// generic renderEntityError path.
		var verrs validation.Errors
		if errors.As(err, &verrs) {
			unprocessableEntityError(w, r, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewCommodityServiceResponse(created).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// updateCommodityService patches a service row's mutable fields.
//
// @Summary Update a service row
// @Description Patch provider contact / reason / expected return / cost.
// @Tags commodity_services
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param serviceID path string true "Service ID"
// @Param service body jsonapi.CommodityServiceUpdateRequest true "Service patch payload"
// @Success 200 {object} jsonapi.CommodityServiceResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Service not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/services/{serviceID} [patch].
func (api *commodityServicesAPI) updateCommodityService(w http.ResponseWriter, r *http.Request) {
	svc := serviceFromContext(r.Context())
	if svc == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.CommodityServiceUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	patch := services.ServiceUpdate{
		ProviderName:     input.Data.Attributes.ProviderName,
		ProviderContact:  input.Data.Attributes.ProviderContact,
		Reason:           input.Data.Attributes.Reason,
		ExpectedReturnAt: input.Data.Attributes.ExpectedReturnAt,
		CostAmount:       input.Data.Attributes.CostAmount,
		CostCurrency:     input.Data.Attributes.CostCurrency,
	}

	updated, err := api.serviceService.UpdateService(r.Context(), svc.ID, patch)
	if err != nil {
		var verrs validation.Errors
		if errors.As(err, &verrs) {
			unprocessableEntityError(w, r, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewCommodityServiceResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// returnCommodityService closes out a service row. Empty body → today
// (server clock). Optional body lets the caller record final cost.
//
// @Summary Mark a service as returned
// @Description Close a service row. Defaults returned_at to today. 409 if already returned.
// @Tags commodity_services
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param serviceID path string true "Service ID"
// @Param payload body jsonapi.CommodityServiceReturnRequest false "Optional explicit returned_at and final cost"
// @Success 200 {object} jsonapi.CommodityServiceResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Service not found"
// @Failure 409 {object} jsonapi.Errors "Service already returned"
// @Router /g/{groupSlug}/commodities/{commodityID}/services/{serviceID}/return [post].
func (api *commodityServicesAPI) returnCommodityService(w http.ResponseWriter, r *http.Request) {
	svc := serviceFromContext(r.Context())
	if svc == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.CommodityServiceReturnRequest
	if r.Body != nil && r.Body != http.NoBody && r.ContentLength != 0 {
		if err := render.Bind(r, &input); err != nil {
			unprocessableEntityError(w, r, err)
			return
		}
	}
	var (
		returnedAt    models.PDate
		finalCost     *decimal.Decimal
		finalCurrency *string
	)
	if input.Data != nil {
		returnedAt = input.Data.Attributes.ReturnedAt
		finalCost = input.Data.Attributes.CostAmount
		finalCurrency = input.Data.Attributes.CostCurrency
	}

	updated, err := api.serviceService.MarkReturned(r.Context(), svc.ID, returnedAt, finalCost, finalCurrency)
	if err != nil {
		if errors.Is(err, services.ErrServiceAlreadyReturned) {
			conflictError(w, r, err, err)
			return
		}
		var verrs validation.Errors
		if errors.As(err, &verrs) {
			unprocessableEntityError(w, r, err)
			return
		}
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewCommodityServiceResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteCommodityService permanently removes a service row.
//
// @Summary Delete a service row
// @Description Hard-delete a service row.
// @Tags commodity_services
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param serviceID path string true "Service ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Service not found"
// @Router /g/{groupSlug}/commodities/{commodityID}/services/{serviceID} [delete].
func (api *commodityServicesAPI) deleteCommodityService(w http.ResponseWriter, r *http.Request) {
	svc := serviceFromContext(r.Context())
	if svc == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	if err := regSet.CommodityServiceRegistry.Delete(r.Context(), svc.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// listGroupServices returns the group-wide service list. Drives the
// dedicated /in-service page.
//
// @Summary List group-wide services
// @Description List service rows across the current group with optional state filter.
// @Tags commodity_services
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param state query string false "Filter by state" Enums(all,open,overdue,completed) default(all)
// @Param page query int false "Page number (1-based)" default(1)
// @Param per_page query int false "Items per page" default(50)
// @Success 200 {object} jsonapi.CommodityServiceListResponse "OK"
// @Router /g/{groupSlug}/services [get].
func (api *commodityServicesAPI) listGroupServices(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	q := r.URL.Query()
	page, perPage := parsePagination(q.Get("page"), q.Get("per_page"))
	offset := (page - 1) * perPage

	state := registry.ServiceState(q.Get("state"))
	if !state.IsValid() {
		state = registry.ServiceStateAll
	}

	rows, total, err := regSet.CommodityServiceRegistry.ListPaginated(r.Context(), offset, perPage, registry.ServiceListOptions{
		State: state,
	})
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	commoditiesByID := make(map[string]*models.Commodity, len(rows))
	for _, s := range rows {
		if _, ok := commoditiesByID[s.CommodityID]; ok {
			continue
		}
		c, cerr := regSet.CommodityRegistry.Get(r.Context(), s.CommodityID)
		if cerr != nil {
			if errors.Is(cerr, registry.ErrNotFound) {
				commoditiesByID[s.CommodityID] = nil
				continue
			}
			renderEntityError(w, r, cerr)
			return
		}
		commoditiesByID[s.CommodityID] = c
	}

	setPaginationHeaders(w, page, perPage, total)
	if err := render.Render(w, r, jsonapi.NewCommodityServiceListResponse(rows, total, commoditiesByID)); err != nil {
		internalServerError(w, r, err)
	}
}

// getGroupServiceCounts returns per-commodity open-service counts for a
// list of commodity ids. Backs the list-page "in service" badge.
//
// @Summary Get open-service counts by commodity
// @Description Map of commodity_id → open-service count for a list of commodities.
// @Tags commodity_services
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodity_id query []string true "Repeatable commodity IDs to look up" collectionFormat(multi)
// @Success 200 {object} jsonapi.CommodityServiceCountsResponse "OK"
// @Router /g/{groupSlug}/services/counts [get].
func (api *commodityServicesAPI) getGroupServiceCounts(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	ids := r.URL.Query()["commodity_id"]
	counts, err := regSet.CommodityServiceRegistry.CountOpenByCommodity(r.Context(), ids)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewCommodityServiceCountsResponse(counts)); err != nil {
		internalServerError(w, r, err)
	}
}

// CommodityServices returns the chi sub-router mounted under the per-
// commodity prefix `/commodities/{commodityID}/services`.
func CommodityServices(params Params) func(r chi.Router) {
	api := &commodityServicesAPI{
		factorySet:     params.FactorySet,
		serviceService: services.NewCommodityServiceService(params.FactorySet),
	}
	return func(r chi.Router) {
		r.Get("/", api.listCommodityServices)
		r.Post("/", api.createCommodityService)
		r.Route("/{serviceID}", func(r chi.Router) {
			r.Use(serviceCtx())
			r.Patch("/", api.updateCommodityService)
			r.Delete("/", api.deleteCommodityService)
			r.Post("/return", api.returnCommodityService)
		})
	}
}

// GroupServices returns the chi sub-router for the group-wide /services
// surface — list + bulk counts.
func GroupServices(params Params) func(r chi.Router) {
	api := &commodityServicesAPI{
		factorySet:     params.FactorySet,
		serviceService: services.NewCommodityServiceService(params.FactorySet),
	}
	return func(r chi.Router) {
		r.Get("/", api.listGroupServices)
		r.Get("/counts", api.getGroupServiceCounts)
	}
}
