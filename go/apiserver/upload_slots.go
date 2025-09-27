package apiserver

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// uploadSlotsAPI handles upload slot operations
type uploadSlotsAPI struct {
	concurrentUploadService services.ConcurrentUploadService
}

// checkUploadCapacity handles upload capacity check requests
// @Summary Check upload capacity
// @Description Check if user can start an upload for a specific operation
// @Tags upload-slots
// @Accept json-api
// @Produce json-api
// @Param operation query string true "Operation name" example(image_upload)
// @Success 200 {object} jsonapi.UploadStatusResponse "Upload capacity available"
// @Failure 400 {object} jsonapi.Errors "Bad request"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Failure 429 {object} jsonapi.Errors "Too many concurrent uploads"
// @Router /upload-slots/check [get]
func (api *uploadSlotsAPI) checkUploadCapacity(w http.ResponseWriter, r *http.Request) {
	// Get operation name from query parameter
	operationName := r.URL.Query().Get("operation")
	if operationName == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Get user from context
	user, err := appctx.RequireUserFromContext(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Get upload status
	status, err := api.concurrentUploadService.GetUploadStatus(r.Context(), user.ID, operationName)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Create response
	response := jsonapi.NewUploadStatusResponse(status)

	// Set status code based on availability
	if !status.CanStartUpload {
		response = response.WithStatusCode(http.StatusTooManyRequests)
	}

	// Render response
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getUploadStatus handles upload status requests
// @Summary Get upload status
// @Description Get current upload status for a specific operation
// @Tags upload-slots
// @Accept json-api
// @Produce json-api
// @Param operation query string true "Operation name" example(image_upload)
// @Success 200 {object} jsonapi.UploadStatusResponse "Upload status retrieved successfully"
// @Failure 400 {object} jsonapi.Errors "Bad request"
// @Failure 401 {object} jsonapi.Errors "Unauthorized"
// @Router /upload-slots/status [get]
func (api *uploadSlotsAPI) getUploadStatus(w http.ResponseWriter, r *http.Request) {
	// Get operation name from query parameter
	operationName := r.URL.Query().Get("operation")
	if operationName == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Get user from context
	user, err := appctx.RequireUserFromContext(r.Context())
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Get upload status
	status, err := api.concurrentUploadService.GetUploadStatus(r.Context(), user.ID, operationName)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Create response
	response := jsonapi.NewUploadStatusResponse(status)

	// Render response
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// UploadSlots sets up the upload slots API routes
func UploadSlots(factorySet *registry.FactorySet) func(r chi.Router) {
	// Create concurrent upload service
	config := services.LoadConcurrentUploadConfig()
	concurrentUploadService := services.NewConcurrentUploadService(config)

	api := &uploadSlotsAPI{
		concurrentUploadService: concurrentUploadService,
	}

	return func(r chi.Router) {
		r.Get("/check", api.checkUploadCapacity) // GET /upload-slots/check
		r.Get("/status", api.getUploadStatus)    // GET /upload-slots/status
	}
}
