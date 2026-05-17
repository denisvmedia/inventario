package apiserver

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

const supplyLinkCtxKey ctxValueKey = "commodity_supply_link"

func supplyLinkFromContext(ctx context.Context) *models.SupplyLink {
	link, ok := ctx.Value(supplyLinkCtxKey).(*models.SupplyLink)
	if !ok {
		return nil
	}
	return link
}

// supplyLinkCtx loads the supply link referenced by {supplyID} into
// the request context. Mirrors loanCtx — including the per-commodity
// defence-in-depth that surfaces a cross-commodity id as 404 instead
// of leaking its existence.
func supplyLinkCtx() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			regSet := RegistrySetFromContext(r.Context())
			if regSet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}
			supplyID := chi.URLParam(r, "supplyID")
			link, err := regSet.SupplyLinkRegistry.Get(r.Context(), supplyID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}
			if commodityID := chi.URLParam(r, "commodityID"); commodityID != "" && link.CommodityID != commodityID {
				renderEntityError(w, r, registry.ErrNotFound)
				return
			}
			ctx := context.WithValue(r.Context(), supplyLinkCtxKey, link)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

type commoditySupplyLinksAPI struct {
	factorySet        *registry.FactorySet
	supplyLinkService *services.SupplyLinkService
}

// listSupplyLinks returns every supply link for the commodity in the
// URL path, ordered by sort_order ASC, created_at ASC.
//
// @Summary List supply links for a commodity
// @Description All supply links for the commodity in the URL, sorted by sort_order ASC.
// @Tags commodity_supply_links
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.SupplyLinksResponse "OK"
// @Router /g/{groupSlug}/commodities/{commodityID}/supplies [get].
func (api *commoditySupplyLinksAPI) listSupplyLinks(w http.ResponseWriter, r *http.Request) {
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	commodityID := chi.URLParam(r, "commodityID")
	links, err := regSet.SupplyLinkRegistry.ListByCommodity(r.Context(), commodityID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSupplyLinksResponse(links, len(links))); err != nil {
		internalServerError(w, r, err)
	}
}

// createSupplyLink attaches a new supply link to the commodity. The
// commodity_id is taken from the URL — the body cannot override it.
//
// @Summary Create a supply link
// @Description Attach a new supply link to the commodity. commodity_id is taken from the URL.
// @Tags commodity_supply_links
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param link body jsonapi.SupplyLinkRequest true "Supply link attributes"
// @Success 201 {object} jsonapi.SupplyLinkResponse "Created"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/supplies [post].
func (api *commoditySupplyLinksAPI) createSupplyLink(w http.ResponseWriter, r *http.Request) {
	commodityID := chi.URLParam(r, "commodityID")

	var input jsonapi.SupplyLinkRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	link := models.SupplyLink{
		CommodityID: commodityID,
		Label:       input.Data.Attributes.Label,
		URL:         input.Data.Attributes.URL,
		Notes:       input.Data.Attributes.Notes,
	}
	if err := link.ValidateWithContext(r.Context()); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	created, err := api.supplyLinkService.Create(r.Context(), link)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewSupplyLinkResponse(created).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
	}
}

// updateSupplyLink patches a supply link's label/url/notes.
//
// @Summary Update a supply link
// @Description Patch label/url/notes on a supply link. Omitting a key leaves it unchanged.
// @Tags commodity_supply_links
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param supplyID path string true "Supply link ID"
// @Param link body jsonapi.SupplyLinkUpdateRequest true "Supply link patch"
// @Success 200 {object} jsonapi.SupplyLinkResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Supply link not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/supplies/{supplyID} [patch].
func (api *commoditySupplyLinksAPI) updateSupplyLink(w http.ResponseWriter, r *http.Request) {
	link := supplyLinkFromContext(r.Context())
	if link == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var input jsonapi.SupplyLinkUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	updated, err := api.supplyLinkService.Update(r.Context(), link.ID, services.SupplyLinkPatch{
		Label: input.Data.Attributes.Label,
		URL:   input.Data.Attributes.URL,
		Notes: input.Data.Attributes.Notes,
	})
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewSupplyLinkResponse(updated)); err != nil {
		internalServerError(w, r, err)
	}
}

// deleteSupplyLink permanently removes a supply link row.
//
// @Summary Delete a supply link
// @Description Hard-delete a supply link.
// @Tags commodity_supply_links
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param supplyID path string true "Supply link ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Supply link not found"
// @Router /g/{groupSlug}/commodities/{commodityID}/supplies/{supplyID} [delete].
func (api *commoditySupplyLinksAPI) deleteSupplyLink(w http.ResponseWriter, r *http.Request) {
	link := supplyLinkFromContext(r.Context())
	if link == nil {
		unprocessableEntityError(w, r, nil)
		return
	}
	if err := api.supplyLinkService.Delete(r.Context(), link.ID); err != nil {
		renderEntityError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// reorderSupplyLinks applies a new visible order to the commodity's
// supply links. The ID list IS the new order; BE renumbers 0..N-1.
//
// @Summary Reorder supply links
// @Description Renumber sort_order for the commodity's supply links per the supplied id list.
// @Tags commodity_supply_links
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param commodityID path string true "Commodity ID"
// @Param payload body jsonapi.SupplyLinkReorderRequest true "Ordered supply link ids"
// @Success 200 {object} jsonapi.SupplyLinksResponse "Updated list, sort_order applied"
// @Failure 404 {object} jsonapi.Errors "Commodity or supply link not found"
// @Failure 422 {object} jsonapi.Errors "User-side request problem"
// @Router /g/{groupSlug}/commodities/{commodityID}/supplies/reorder [post].
func (api *commoditySupplyLinksAPI) reorderSupplyLinks(w http.ResponseWriter, r *http.Request) {
	commodityID := chi.URLParam(r, "commodityID")
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.SupplyLinkReorderRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if err := api.supplyLinkService.Reorder(r.Context(), commodityID, input.Data.Attributes.IDs); err != nil {
		renderEntityError(w, r, err)
		return
	}

	links, err := regSet.SupplyLinkRegistry.ListByCommodity(r.Context(), commodityID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	if err := render.Render(w, r, jsonapi.NewSupplyLinksResponse(links, len(links))); err != nil {
		internalServerError(w, r, err)
	}
}

// CommoditySupplyLinks returns the chi sub-router mounted under the
// per-commodity prefix `/commodities/{commodityID}/supplies`.
func CommoditySupplyLinks(params Params) func(r chi.Router) {
	api := &commoditySupplyLinksAPI{
		factorySet:        params.FactorySet,
		supplyLinkService: services.NewSupplyLinkService(params.FactorySet),
	}
	return func(r chi.Router) {
		r.Get("/", api.listSupplyLinks)
		r.Post("/", api.createSupplyLink)
		r.Post("/reorder", api.reorderSupplyLinks)
		r.Route("/{supplyID}", func(r chi.Router) {
			r.Use(supplyLinkCtx())
			r.Patch("/", api.updateSupplyLink)
			r.Delete("/", api.deleteSupplyLink)
		})
	}
}
