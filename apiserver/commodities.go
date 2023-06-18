package apiserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type commoditiesAPI struct {
	commodityRegistry registry.CommodityRegistry
	imageRegistry     registry.ImageRegistry
	manualRegistry    registry.ManualRegistry
	invoiceRegistry   registry.InvoiceRegistry
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

	respCommodity := &jsonapi.Commodity{
		Commodity: commodity,
		CommodityExtra: jsonapi.CommodityExtra{
			Images:   api.commodityRegistry.GetImages(commodity.ID),
			Manuals:  api.commodityRegistry.GetManuals(commodity.ID),
			Invoices: api.commodityRegistry.GetInvoices(commodity.ID),
		},
	}

	resp := jsonapi.NewCommodityResponse(respCommodity)
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

	respCommodity := &jsonapi.Commodity{
		Commodity: commodity,
		CommodityExtra: jsonapi.CommodityExtra{
			Images:   api.commodityRegistry.GetImages(commodity.ID),
			Manuals:  api.commodityRegistry.GetManuals(commodity.ID),
			Invoices: api.commodityRegistry.GetInvoices(commodity.ID),
		},
	}

	resp := jsonapi.NewCommodityResponse(respCommodity).WithStatusCode(http.StatusCreated)
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

	respCommodity := &jsonapi.Commodity{
		Commodity: updatedCommodity,
		CommodityExtra: jsonapi.CommodityExtra{
			Images:   api.commodityRegistry.GetImages(updatedCommodity.ID),
			Manuals:  api.commodityRegistry.GetManuals(updatedCommodity.ID),
			Invoices: api.commodityRegistry.GetInvoices(updatedCommodity.ID),
		},
	}

	resp := jsonapi.NewCommodityResponse(respCommodity).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// listImages lists all images for a commodity.
// @Summary List images for a commodity
// @Description get images for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.ImagesResponse "OK"
// @Router /commodities/{commodityID}/images [get]
func (api *commoditiesAPI) listImages(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var images []*models.Image
	imageIDs := api.commodityRegistry.GetImages(commodity.ID)
	for _, id := range imageIDs {
		img, err := api.imageRegistry.Get(id)
		if err != nil {
			unprocessableEntityError(w, r, nil)
			return
		}
		images = append(images, img)
	}
	response := jsonapi.NewImagesResponse(images, len(imageIDs))

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// listInvoices lists all invoices for a commodity.
// @Summary List invoices for a commodity
// @Description get invoices for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.InvoicesResponse "OK"
// @Router /commodities/{commodityID}/invoices [get]
func (api *commoditiesAPI) listInvoices(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var invoices []*models.Invoice
	invoiceIDs := api.commodityRegistry.GetInvoices(commodity.ID)
	for _, id := range invoiceIDs {
		img, err := api.invoiceRegistry.Get(id)
		if err != nil {
			unprocessableEntityError(w, r, nil)
			return
		}
		invoices = append(invoices, img)
	}
	response := jsonapi.NewInvoicesResponse(invoices, len(invoiceIDs))

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// listManuals lists all manuals for a commodity.
// @Summary List manuals for a commodity
// @Description get manuals for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Success 200 {object} jsonapi.ManualsResponse "OK"
// @Router /commodities/{commodityID}/manuals [get]
func (api *commoditiesAPI) listManuals(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var manuals []*models.Manual
	manualIDs := api.commodityRegistry.GetManuals(commodity.ID)
	for _, id := range manualIDs {
		img, err := api.manualRegistry.Get(id)
		if err != nil {
			unprocessableEntityError(w, r, nil)
			return
		}
		manuals = append(manuals, img)
	}
	response := jsonapi.NewManualsResponse(manuals, len(manualIDs))

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteImage deletes an image for a commodity.
// @Summary Delete an image for a commodity
// @Description Delete an image for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param imageID path string true "Image ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Commodity or image not found"
// @Router /commodities/{commodityID}/images/{imageID} [delete]
func (api *commoditiesAPI) deleteImage(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	imageID := chi.URLParam(r, "imageID")
	image, err := api.imageRegistry.Get(imageID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if image.CommodityID != commodity.ID {
		unprocessableEntityError(w, r, errors.New("image does not belong to commodity"))
		return
	}

	err = api.imageRegistry.Delete(imageID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteInvoice deletes an invoice for a commodity.
// @Summary Delete an invoice for a commodity
// @Description Delete an invoice for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Commodity or invoice not found"
// @Router /commodities/{commodityID}/invoices/{invoiceID} [delete]
func (api *commoditiesAPI) deleteInvoice(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	invoiceID := chi.URLParam(r, "invoiceID")
	invoice, err := api.invoiceRegistry.Get(invoiceID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if invoice.CommodityID != commodity.ID {
		unprocessableEntityError(w, r, errors.New("invoice does not belong to commodity"))
		return
	}

	err = api.invoiceRegistry.Delete(invoiceID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// deleteManual deletes a manual for a commodity.
// @Summary Delete a manual for a commodity
// @Description Delete a manual for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param manualID path string true "Manual ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Commodity or manual not found"
// @Router /commodities/{commodityID}/manuals/{manualID} [delete]
func (api *commoditiesAPI) deleteManual(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	manualID := chi.URLParam(r, "manualID")
	manual, err := api.manualRegistry.Get(manualID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if manual.CommodityID != commodity.ID {
		unprocessableEntityError(w, r, errors.New("manual does not belong to commodity"))
		return
	}

	err = api.manualRegistry.Delete(manualID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func Commodities(params Params) func(r chi.Router) {
	api := &commoditiesAPI{
		commodityRegistry: params.CommodityRegistry,
		imageRegistry:     params.ImageRegistry,
		invoiceRegistry:   params.InvoiceRegistry,
		manualRegistry:    params.ManualRegistry,
	}
	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listCommodities) // GET /commodities
		r.Route("/{commodityID}", func(r chi.Router) {
			r.Use(commodityCtx(api.commodityRegistry))
			r.Get("/", api.getCommodity)       // GET /commodities/123
			r.Put("/", api.updateCommodity)    // PUT /commodities/123
			r.Delete("/", api.deleteCommodity) // DELETE /commodities/123

			r.Get("/images", api.listImages)               // GET /commodities/123/images
			r.Delete("/images/{imageID}", api.deleteImage) // DELETE /commodities/123/images/456

			r.Get("/invoices", api.listInvoices)                 // GET /commodities/123/invoices
			r.Delete("/invoices/{invoiceID}", api.deleteInvoice) // DELETE /commodities/123/invoices/789

			r.Get("/manuals", api.listManuals)                // GET /commodities/123/manuals
			r.Delete("/manuals/{manualID}", api.deleteManual) // DELETE /commodities/123/manuals/abc
		})
		r.Post("/", api.createCommodity) // POST /commodities
	}
}
