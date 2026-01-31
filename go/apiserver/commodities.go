package apiserver

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/textutils"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

type commoditiesAPI struct {
	uploadLocation     string
	entityService      *services.EntityService
	fileService        *services.FileService
	fileSigningService *services.FileSigningService
}

// generateSignedURLsForFiles generates signed URLs for a list of files.
// Returns a map of file ID to URLData with signed URLs and thumbnails. Missing URLs indicate generation failures.
func (api *commoditiesAPI) generateSignedURLsForFiles(ctx context.Context, files []*models.FileEntity) map[string]jsonapi.URLData {
	signedUrls := make(map[string]jsonapi.URLData)
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return signedUrls
	}

	for _, file := range files {
		// Generate signed URLs for file and thumbnails
		originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(file, user.ID)
		if err != nil {
			// Log error but don't fail the entire request
			// The frontend can handle missing URLs gracefully
			continue
		}

		signedUrls[file.ID] = jsonapi.URLData{
			URL:        originalURL,
			Thumbnails: thumbnails,
		}
	}

	return signedUrls
}

// listCommodities lists all commodities.
// @Summary List commodities
// @Description get commodities
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.CommoditiesResponse "OK"
// @Router /commodities [get].
func (api *commoditiesAPI) listCommodities(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	commodityReg := regSet.CommodityRegistry

	commodities, _ := commodityReg.List(r.Context())

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
// @Router /commodities/{id} [get].
func (api *commoditiesAPI) getCommodity(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	regSet := RegistrySetFromContext(r.Context())
	if regSet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}
	comReg := regSet.CommodityRegistry

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	var imagesError string
	images, err := comReg.GetImages(r.Context(), commodity.ID)
	if err != nil {
		imagesError = err.Error()
	}

	var manualsError string
	manuals, err := comReg.GetManuals(r.Context(), commodity.ID)
	if err != nil {
		manualsError = err.Error()
	}

	var invoicesError string
	invoices, err := comReg.GetInvoices(r.Context(), commodity.ID)
	if err != nil {
		invoicesError = err.Error()
	}

	resp := jsonapi.NewCommodityResponse(commodity, &jsonapi.CommodityMeta{
		Images:        images,
		ImagesError:   imagesError,
		Manuals:       manuals,
		ManualsError:  manualsError,
		Invoices:      invoices,
		InvoicesError: invoicesError,
	}).WithStatusCode(http.StatusOK)

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
// @Router /commodities [post].
func (api *commoditiesAPI) createCommodity(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	var input jsonapi.CommodityRequest

	rWithCurrency, err := requestWithMainCurrency(r, registrySet.SettingsRegistry)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Use standardized security validation
	user, ok := RequireUserContext(w, r)
	if !ok {
		return // Error already handled by RequireUserContext
	}

	// Validate input
	if secErr := ValidateInputSanitization(r, input.Data.Attributes); secErr != nil {
		HandleSecurityError(w, r, secErr)
		return
	}

	commodity := *input.Data.Attributes
	if commodity.TenantID == "" {
		commodity.TenantID = user.TenantID
	}

	// Use standardized registry access
	commodityReg := registrySet.CommodityRegistry

	createdCommodity, err := commodityReg.Create(r.Context(), commodity)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	var imagesError string
	images, err := registrySet.CommodityRegistry.GetImages(r.Context(), createdCommodity.ID)
	if err != nil {
		imagesError = err.Error()
	}

	var manualsError string
	manuals, err := registrySet.CommodityRegistry.GetManuals(r.Context(), createdCommodity.ID)
	if err != nil {
		manualsError = err.Error()
	}

	var invoicesError string
	invoices, err := registrySet.CommodityRegistry.GetInvoices(r.Context(), createdCommodity.ID)
	if err != nil {
		invoicesError = err.Error()
	}

	resp := jsonapi.NewCommodityResponse(createdCommodity, &jsonapi.CommodityMeta{
		Images:        images,
		ImagesError:   imagesError,
		Manuals:       manuals,
		ManualsError:  manualsError,
		Invoices:      invoices,
		InvoicesError: invoicesError,
	}).WithStatusCode(http.StatusCreated)

	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteCommodity deletes a commodity by ID.
// @Summary Delete a commodity
// @Description Delete a commodity by ID and all its linked files
// @Tags commodities
// @Accept  json-api
// @Produce  json-api
// @Param id path string true "Commodity ID"
// @Success 204 "No content"
// @Failure 404 {object} jsonapi.Errors "Commodity not found"
// @Router /commodities/{id} [delete].
func (api *commoditiesAPI) deleteCommodity(w http.ResponseWriter, r *http.Request) {
	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	err := api.entityService.DeleteCommodityRecursive(r.Context(), commodity.ID)
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
// @Router /commodities/{id} [put].
func (api *commoditiesAPI) updateCommodity(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	rWithCurrency, err := requestWithMainCurrency(r, registrySet.SettingsRegistry)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}
	r = rWithCurrency

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	var input jsonapi.CommodityRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if commodity.ID != input.Data.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}

	input.Data.Attributes.ID = input.Data.ID

	// Preserve tenant_id and user_id from the existing commodity
	// This ensures the foreign key constraints are satisfied during updates
	updateData := *input.Data.Attributes
	if updateData.TenantID == "" {
		updateData.TenantID = commodity.TenantID
	}

	// Use UpdateWithUser to ensure proper user context and validation
	ctx := r.Context()
	commodityReg := registrySet.CommodityRegistry
	updatedCommodity, err := commodityReg.Update(ctx, updateData)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	var imagesError string
	images, err := registrySet.CommodityRegistry.GetImages(r.Context(), commodity.ID)
	if err != nil {
		imagesError = err.Error()
	}

	var manualsError string
	manuals, err := registrySet.CommodityRegistry.GetManuals(r.Context(), commodity.ID)
	if err != nil {
		manualsError = err.Error()
	}

	var invoicesError string
	invoices, err := registrySet.CommodityRegistry.GetInvoices(r.Context(), commodity.ID)
	if err != nil {
		invoicesError = err.Error()
	}

	resp := jsonapi.NewCommodityResponse(updatedCommodity, &jsonapi.CommodityMeta{
		Images:        images,
		ImagesError:   imagesError,
		Manuals:       manuals,
		ManualsError:  manualsError,
		Invoices:      invoices,
		InvoicesError: invoicesError,
	}).WithStatusCode(http.StatusOK)

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
// @Success 200 {object} jsonapi.FilesResponse "OK"
// @Router /commodities/{commodityID}/images [get].
func (api *commoditiesAPI) listImages(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	fileReg := registrySet.FileRegistry

	// Get file entities linked to this commodity with "images" meta
	files, err := fileReg.ListByLinkedEntityAndMeta(r.Context(), "commodity", commodity.ID, "images")
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Generate signed URLs with thumbnails for all files
	signedUrls := api.generateSignedURLsForFiles(r.Context(), files)

	// Use the new FilesResponse format with signed URLs and thumbnails
	response := jsonapi.NewFilesResponseWithSignedUrls(files, len(files), signedUrls)

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
// @Router /commodities/{commodityID}/invoices [get].
func (api *commoditiesAPI) listInvoices(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	fileReg := registrySet.FileRegistry

	// Get file entities linked to this commodity with "invoices" meta
	files, err := fileReg.ListByLinkedEntityAndMeta(r.Context(), "commodity", commodity.ID, "invoices")
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Generate signed URLs for all files
	signedUrls := api.generateSignedURLsForFiles(r.Context(), files)

	// Convert file entities to legacy invoice format for compatibility
	var invoices []*models.Invoice
	for _, file := range files {
		invoice := &models.Invoice{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: file.ID}, TenantID: "default-tenant"},
			CommodityID:         commodity.ID,
			File:                file.File,
		}
		invoices = append(invoices, invoice)
	}
	response := jsonapi.NewInvoicesResponseWithSignedUrls(invoices, len(files), signedUrls)

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
// @Router /commodities/{commodityID}/manuals [get].
func (api *commoditiesAPI) listManuals(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	fileReg := registrySet.FileRegistry

	// Get file entities linked to this commodity with "manuals" meta
	files, err := fileReg.ListByLinkedEntityAndMeta(r.Context(), "commodity", commodity.ID, "manuals")
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Generate signed URLs for all files
	signedUrls := api.generateSignedURLsForFiles(r.Context(), files)

	// Convert file entities to legacy manual format for compatibility
	var manuals []*models.Manual
	for _, file := range files {
		manual := &models.Manual{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: file.ID}, TenantID: "default-tenant"},
			CommodityID:         commodity.ID,
			File:                file.File,
		}
		manuals = append(manuals, manual)
	}
	response := jsonapi.NewManualsResponseWithSignedUrls(manuals, len(files), signedUrls)

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
// @Router /commodities/{commodityID}/images/{imageID} [delete].
func (api *commoditiesAPI) deleteImage(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	fileReg := registrySet.FileRegistry

	imageID := chi.URLParam(r, "imageID")

	// Get the file entity
	file, err := fileReg.Get(r.Context(), imageID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it belongs to this commodity and is an image
	if file.LinkedEntityType != "commodity" || file.LinkedEntityID != commodity.ID || file.LinkedEntityMeta != "images" {
		unprocessableEntityError(w, r, errors.New("file does not belong to commodity or is not an image"))
		return
	}

	err = api.fileService.DeleteFileWithPhysical(r.Context(), imageID)
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
// @Router /commodities/{commodityID}/invoices/{invoiceID} [delete].
func (api *commoditiesAPI) deleteInvoice(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	fileReg := registrySet.FileRegistry

	invoiceID := chi.URLParam(r, "invoiceID")

	// Get the file entity
	file, err := fileReg.Get(r.Context(), invoiceID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it belongs to this commodity and is an invoice
	if file.LinkedEntityType != "commodity" || file.LinkedEntityID != commodity.ID || file.LinkedEntityMeta != "invoices" {
		unprocessableEntityError(w, r, errors.New("file does not belong to commodity or is not an invoice"))
		return
	}

	err = api.fileService.DeleteFileWithPhysical(r.Context(), invoiceID)
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
// @Router /commodities/{commodityID}/manuals/{manualID} [delete].
func (api *commoditiesAPI) deleteManual(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	fileReg := registrySet.FileRegistry

	manualID := chi.URLParam(r, "manualID")

	// Get the file entity
	file, err := fileReg.Get(r.Context(), manualID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it belongs to this commodity and is a manual
	if file.LinkedEntityType != "commodity" || file.LinkedEntityID != commodity.ID || file.LinkedEntityMeta != "manuals" {
		unprocessableEntityError(w, r, errors.New("file does not belong to commodity or is not a manual"))
		return
	}

	err = api.fileService.DeleteFileWithPhysical(r.Context(), manualID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// downloadImage downloads an image file for a commodity.
// @Summary Download an image file for a commodity
// @Description Download an image file for a commodity
// @Tags commodities
// @Accept octet-stream
// @Produce octet-stream
// @Param commodityID path string true "Commodity ID"
// @Param imageID path string true "Image ID"
// @Param imageExt path string true "Image Extension"
// @Success 200 "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or image not found"
// @Router /commodities/{commodityID}/images/{imageID}.{imageExt} [get].
func (api *commoditiesAPI) downloadImage(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	imageReg := registrySet.ImageRegistry

	imageID := chi.URLParam(r, "imageID")
	image, err := imageReg.Get(r.Context(), imageID)
	if err != nil || image.CommodityID != commodity.ID {
		http.NotFound(w, r)
		return
	}

	// Get file attributes to set Content-Length and other headers
	attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, image.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	file, err := api.getDownloadFile(r.Context(), image.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	defer file.Close()

	// Use Path + Ext for the downloaded filename
	filename := image.Path + image.Ext
	contentType := mime.TypeByExtension(image.Ext)

	// Set headers to optimize streaming and prevent browser preloading
	downloadutils.SetStreamingHeaders(w, contentType, attrs.Size, filename)

	// Use chunked copying to prevent browser buffering
	if err := downloadutils.CopyFileInChunks(w, file); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// downloadInvoice downloads an invoice file for a commodity.
// @Summary Download an invoice file for a commodity
// @Description Download an invoice file for a commodity
// @Tags commodities
// @Accept octet-stream
// @Produce octet-stream
// @Param commodityID path string true "Commodity ID"
// @Param invoiceID path string true "Invoice ID"
// @Param invoiceExt path string true "Invoice Extension"
// @Success 200 "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or invoice not found"
// @Router /commodities/{commodityID}/invoices/{invoiceID}.{invoiceExt} [get].
func (api *commoditiesAPI) downloadInvoice(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	invoiceReg := registrySet.InvoiceRegistry

	invoiceID := chi.URLParam(r, "invoiceID")
	invoice, err := invoiceReg.Get(r.Context(), invoiceID)
	if err != nil || invoice.CommodityID != commodity.ID {
		http.NotFound(w, r)
		return
	}

	// Get file attributes to set Content-Length and other headers
	attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, invoice.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	file, err := api.getDownloadFile(r.Context(), invoice.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	defer file.Close()

	// Use Path + Ext for the downloaded filename
	filename := invoice.Path + invoice.Ext
	contentType := mime.TypeByExtension(invoice.Ext)

	// Set headers to optimize streaming and prevent browser preloading
	downloadutils.SetStreamingHeaders(w, contentType, attrs.Size, filename)

	// Use chunked copying to prevent browser buffering
	if err := downloadutils.CopyFileInChunks(w, file); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// downloadManual downloads a manual file for a commodity.
// @Summary Download a manual file for a commodity
// @Description Download a manual file for a commodity
// @Tags commodities
// @Accept octet-stream
// @Produce octet-stream
// @Param commodityID path string true "Commodity ID"
// @Param manualID path string true "Manual ID"
// @Param manualExt path string true "Manual Extension"
// @Success 200 "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or manual not found"
// @Router /commodities/{commodityID}/manuals/{manualID}.{manualExt} [get].
func (api *commoditiesAPI) downloadManual(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	commodity := commodityFromContext(r.Context())
	if commodity == nil {
		unprocessableEntityError(w, r, errors.New("commodity not found in context"))
		return
	}

	manualReg := registrySet.ManualRegistry

	manualID := chi.URLParam(r, "manualID")
	manual, err := manualReg.Get(r.Context(), manualID)
	if err != nil || manual.CommodityID != commodity.ID {
		http.NotFound(w, r)
		return
	}

	// Get file attributes to set Content-Length and other headers
	attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, manual.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	file, err := api.getDownloadFile(r.Context(), manual.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	defer file.Close()

	// Use Path + Ext for the downloaded filename
	filename := manual.Path + manual.Ext
	contentType := mime.TypeByExtension(manual.Ext)

	// Set headers to optimize streaming and prevent browser preloading
	downloadutils.SetStreamingHeaders(w, contentType, attrs.Size, filename)

	// Use chunked copying to prevent browser buffering
	if err := downloadutils.CopyFileInChunks(w, file); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func (api *commoditiesAPI) getDownloadFile(ctx context.Context, originalPath string) (io.ReadCloser, error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return nil, stacktrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	// Use the original path for file retrieval
	return b.NewReader(context.Background(), originalPath, nil)
}

// getImageData retrieves data of an image for a commodity.
// @Summary Get image data
// @Description get data of an image for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param imageID path string true "Image ID"
// @Success 200 {object} jsonapi.ImageResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or image not found"
// @Router /commodities/{commodityID}/images/{imageID} [get].
func (api *commoditiesAPI) getImageData(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	imageID := chi.URLParam(r, "imageID")

	// Get the file entity
	file, err := fileReg.Get(r.Context(), imageID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it's a commodity image
	if file.LinkedEntityType != "commodity" || file.LinkedEntityMeta != "images" {
		unprocessableEntityError(w, r, errors.New("file is not a commodity image"))
		return
	}

	// Convert to legacy image format for compatibility
	image := &models.Image{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: file.ID}, TenantID: "default-tenant"},
		CommodityID:         file.LinkedEntityID,
		File:                file.File,
	}

	response := jsonapi.NewImageResponse(image)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getInvoiceData retrieves data of an invoice for a commodity.
// @Summary Get invoice data
// @Description get data of an invoice for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param invoiceID path string true "Invoice ID"
// @Success 200 {object} jsonapi.InvoiceResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or invoice not found"
// @Router /commodities/{commodityID}/invoices/{invoiceID} [get].
func (api *commoditiesAPI) getInvoiceData(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	invoiceID := chi.URLParam(r, "invoiceID")

	// Get the file entity
	file, err := fileReg.Get(r.Context(), invoiceID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it's a commodity invoice
	if file.LinkedEntityType != "commodity" || file.LinkedEntityMeta != "invoices" {
		unprocessableEntityError(w, r, errors.New("file is not a commodity invoice"))
		return
	}

	// Convert to legacy invoice format for compatibility
	invoice := &models.Invoice{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: file.ID}, TenantID: "default-tenant"},
		CommodityID:         file.LinkedEntityID,
		File:                file.File,
	}

	response := jsonapi.NewInvoiceResponse(invoice)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getManualsData retrieves data of a manual for a commodity.
// @Summary Get manual data
// @Description get data of a manual for a commodity
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param manualID path string true "Manual ID"
// @Success 200 {object} jsonapi.ManualResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or manual not found"
// @Router /commodities/{commodityID}/manuals/{manualID} [get].
func (api *commoditiesAPI) getManualsData(w http.ResponseWriter, r *http.Request) { //revive:disable-line:get-return
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	manualID := chi.URLParam(r, "manualID")

	// Get the file entity
	file, err := fileReg.Get(r.Context(), manualID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it's a commodity manual
	if file.LinkedEntityType != "commodity" || file.LinkedEntityMeta != "manuals" {
		unprocessableEntityError(w, r, errors.New("file is not a commodity manual"))
		return
	}

	// Convert to legacy manual format for compatibility
	manual := &models.Manual{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: file.ID}, TenantID: "default-tenant"},
		CommodityID:         file.LinkedEntityID,
		File:                file.File,
	}

	response := jsonapi.NewManualResponse(manual)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateImage updates an image's path.
// @Summary Update an image
// @Description update an image's path
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param imageID path string true "Image ID"
// @Param request body jsonapi.CommodityFileUpdateRequest true "Update request"
// @Success 200 {object} jsonapi.ImageResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or image not found"
// @Router /commodities/{commodityID}/images/{imageID} [put].
func (api *commoditiesAPI) updateImage(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	imageID := chi.URLParam(r, "imageID")

	var input jsonapi.CommodityFileUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if imageID != input.Data.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}

	// Get the file entity
	file, err := fileReg.Get(r.Context(), imageID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it's a commodity image
	if file.LinkedEntityType != "commodity" || file.LinkedEntityMeta != "images" {
		unprocessableEntityError(w, r, errors.New("file is not a commodity image"))
		return
	}

	// Update the file entity
	file.Path = textutils.CleanFilename(input.Data.Attributes.Path)
	file.UpdatedAt = time.Now()

	updatedFile, err := fileReg.Update(r.Context(), *file)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Convert back to legacy image format for compatibility
	updatedImage := &models.Image{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: updatedFile.ID}, TenantID: "default-tenant"},
		CommodityID:         updatedFile.LinkedEntityID,
		File:                updatedFile.File,
	}

	response := jsonapi.NewImageResponse(updatedImage)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateInvoice updates an invoice's path.
// @Summary Update an invoice
// @Description update an invoice's path
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param invoiceID path string true "Invoice ID"
// @Param request body jsonapi.CommodityFileUpdateRequest true "Update request"
// @Success 200 {object} jsonapi.InvoiceResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or invoice not found"
// @Router /commodities/{commodityID}/invoices/{invoiceID} [put].
func (api *commoditiesAPI) updateInvoice(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	invoiceID := chi.URLParam(r, "invoiceID")

	var input jsonapi.CommodityFileUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if invoiceID != input.Data.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}

	// Get the file entity
	file, err := fileReg.Get(r.Context(), invoiceID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it's a commodity invoice
	if file.LinkedEntityType != "commodity" || file.LinkedEntityMeta != "invoices" {
		unprocessableEntityError(w, r, errors.New("file is not a commodity invoice"))
		return
	}

	// Update the file entity
	file.Path = textutils.CleanFilename(input.Data.Attributes.Path)
	file.UpdatedAt = time.Now()

	updatedFile, err := fileReg.Update(r.Context(), *file)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Convert back to legacy invoice format for compatibility
	updatedInvoice := &models.Invoice{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: updatedFile.ID}, TenantID: "default-tenant"},
		CommodityID:         updatedFile.LinkedEntityID,
		File:                updatedFile.File,
	}

	response := jsonapi.NewInvoiceResponse(updatedInvoice)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateManual updates a manual's path.
// @Summary Update a manual
// @Description update a manual's path
// @Tags commodities
// @Accept json-api
// @Produce json-api
// @Param commodityID path string true "Commodity ID"
// @Param manualID path string true "Manual ID"
// @Param request body jsonapi.CommodityFileUpdateRequest true "Update request"
// @Success 200 {object} jsonapi.ManualResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Commodity or manual not found"
// @Router /commodities/{commodityID}/manuals/{manualID} [put].
func (api *commoditiesAPI) updateManual(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	manualID := chi.URLParam(r, "manualID")

	var input jsonapi.CommodityFileUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if manualID != input.Data.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}

	// Get the file entity
	file, err := fileReg.Get(r.Context(), manualID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify it's a commodity manual
	if file.LinkedEntityType != "commodity" || file.LinkedEntityMeta != "manuals" {
		unprocessableEntityError(w, r, errors.New("file is not a commodity manual"))
		return
	}

	// Update the file entity
	file.Path = textutils.CleanFilename(input.Data.Attributes.Path)
	file.UpdatedAt = time.Now()

	updatedFile, err := fileReg.Update(r.Context(), *file)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Convert back to legacy manual format for compatibility
	updatedManual := &models.Manual{
		TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: updatedFile.ID}, TenantID: "default-tenant"},
		CommodityID:         updatedFile.LinkedEntityID,
		File:                updatedFile.File,
	}

	response := jsonapi.NewManualResponse(updatedManual)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func Commodities(params Params) func(r chi.Router) {
	api := &commoditiesAPI{
		uploadLocation:     params.UploadLocation,
		entityService:      params.EntityService,
		fileService:        services.NewFileService(params.FactorySet, params.UploadLocation),
		fileSigningService: services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
	}

	return func(r chi.Router) {
		r.With(paginate).Get("/", api.listCommodities) // GET /commodities
		r.Route("/{commodityID}", func(r chi.Router) {
			r.Use(commodityCtx())
			r.Get("/", api.getCommodity)       // GET /commodities/123
			r.Put("/", api.updateCommodity)    // PUT /commodities/123
			r.Delete("/", api.deleteCommodity) // DELETE /commodities/123

			r.Get("/images", api.listImages)               // GET /commodities/123/images
			r.Delete("/images/{imageID}", api.deleteImage) // DELETE /commodities/123/images/456
			r.Put("/images/{imageID}", api.updateImage)    // PUT /commodities/123/images/456

			r.Get("/invoices", api.listInvoices)                 // GET /commodities/123/invoices
			r.Delete("/invoices/{invoiceID}", api.deleteInvoice) // DELETE /commodities/123/invoices/789
			r.Put("/invoices/{invoiceID}", api.updateInvoice)    // PUT /commodities/123/invoices/789

			r.Get("/manuals", api.listManuals)                // GET /commodities/123/manuals
			r.Delete("/manuals/{manualID}", api.deleteManual) // DELETE /commodities/123/manuals/abc
			r.Put("/manuals/{manualID}", api.updateManual)    // PUT /commodities/123/manuals/abc

			r.Get("/images/{imageID}.{imageExt}", api.downloadImage)         // GET /commodities/123/images/456.png
			r.Get("/invoices/{invoiceID}.{invoiceExt}", api.downloadInvoice) // GET /commodities/123/invoices/789.pdf
			r.Get("/manuals/{manualID}.{manualExt}", api.downloadManual)     // GET /commodities/123/manuals/abc.pdf

			r.Get("/images/{imageID}", api.getImageData)       // GET /commodities/123/images/456
			r.Get("/invoices/{invoiceID}", api.getInvoiceData) // GET /commodities/123/invoices/789
			r.Get("/manuals/{manualID}", api.getManualsData)   // GET /commodities/123/manuals/abc
		})
		r.Post("/", api.createCommodity) // POST /commodities
	}
}

func requestWithMainCurrency(r *http.Request, userSettingsRegistry registry.SettingsRegistry) (*http.Request, error) {
	settings, err := userSettingsRegistry.Get(r.Context())
	if err != nil {
		return nil, err
	}

	if settings.MainCurrency == nil {
		return nil, registry.ErrMainCurrencyNotSet
	}

	ctx := validationctx.WithMainCurrency(r.Context(), *settings.MainCurrency)

	return r.WithContext(ctx), nil
}
