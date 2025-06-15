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

type exportRestoresAPI struct {
	registrySet   *registry.Set
	restoreWorker RestoreWorkerInterface
}

// listExportRestores lists all restore operations for an export.
// @Summary List export restore operations
// @Description get restore operations for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Success 200 {object} jsonapi.RestoreOperationsResponse "OK"
// @Router /exports/{id}/restores [get].
func (api *exportRestoresAPI) listExportRestores(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOperations, err := api.registrySet.RestoreOperationRegistry.ListByExport(r.Context(), exportID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewRestoreOperationsResponse(restoreOperations)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// apiGetExportRestore returns a specific restore operation for an export.
// @Summary Get export restore operation
// @Description get restore operation by ID for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Param restoreId path string true "Restore Operation ID"
// @Success 200 {object} jsonapi.RestoreOperationResponse "OK"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /exports/{id}/restores/{restoreId} [get].
func (api *exportRestoresAPI) apiGetExportRestore(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	restoreID := chi.URLParam(r, "restoreId")

	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	if restoreID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOperation, err := api.registrySet.RestoreOperationRegistry.Get(r.Context(), restoreID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify the restore operation belongs to this export
	if restoreOperation.ExportID != exportID {
		notFound(w, r)
		return
	}

	// Load steps for this restore operation
	steps, err := api.registrySet.RestoreStepRegistry.ListByRestoreOperation(r.Context(), restoreID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Convert steps to the format expected by the model
	restoreOperation.Steps = make([]models.RestoreStep, len(steps))
	for i, step := range steps {
		restoreOperation.Steps[i] = *step
	}

	if err := render.Render(w, r, jsonapi.NewRestoreOperationResponse(restoreOperation)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createExportRestore creates a new restore operation for an export.
// @Summary Create export restore operation
// @Description create a new restore operation for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Param request body jsonapi.RestoreOperationCreateRequest true "Restore operation data"
// @Success 201 {object} jsonapi.RestoreOperationResponse "Created"
// @Failure 400 {object} jsonapi.ErrorResponse "Bad Request"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /exports/{id}/restores [post].
func (api *exportRestoresAPI) createExportRestore(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists and is completed
	export, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if export.Status != models.ExportStatusCompleted {
		badRequest(w, r, ErrInvalidContentType)
		return
	}

	data := &jsonapi.RestoreOperationCreateRequest{}
	if err := render.Bind(r, data); err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Check if there are any running restore operations
	hasRunning, err := api.restoreWorker.HasRunningRestores(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	if hasRunning {
		// Return HTTP 409 Conflict if a restore is already in progress or pending
		err := errors.New("restore operation already in progress or pending")
		userErr := errors.New("A restore operation is already in progress or pending. Please wait for it to complete before starting a new one.")
		conflictError(w, r, err, userErr)
		return
	}

	restoreOperation := models.NewRestoreOperationFromUserInput(data.Data.Attributes)
	createdRestoreOperation, err := api.registrySet.RestoreOperationRegistry.Create(r.Context(), restoreOperation)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Return immediately with the created restore operation
	// The restore worker will pick up the pending operation and process it
	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewRestoreOperationResponse(createdRestoreOperation)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteExportRestore deletes a restore operation for an export.
// @Summary Delete export restore operation
// @Description delete a restore operation for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Param restoreId path string true "Restore Operation ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /exports/{id}/restores/{restoreId} [delete].
func (api *exportRestoresAPI) deleteExportRestore(w http.ResponseWriter, r *http.Request) {
	exportID := chi.URLParam(r, "id")
	restoreID := chi.URLParam(r, "restoreId")

	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	if restoreID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := api.registrySet.ExportRegistry.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify the restore operation exists and belongs to this export
	restoreOperation, err := api.registrySet.RestoreOperationRegistry.Get(r.Context(), restoreID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if restoreOperation.ExportID != exportID {
		notFound(w, r)
		return
	}

	// Don't allow deletion of running restore operations
	if restoreOperation.Status == models.RestoreStatusRunning {
		badRequest(w, r, ErrInvalidContentType)
		return
	}

	err = api.registrySet.RestoreOperationRegistry.Delete(r.Context(), restoreID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ExportRestores sets up the export restore API routes.
func ExportRestores(params Params, restoreWorker RestoreWorkerInterface) func(r chi.Router) {
	api := &exportRestoresAPI{
		registrySet:   params.RegistrySet,
		restoreWorker: restoreWorker,
	}

	return func(r chi.Router) {
		r.Get("/", api.listExportRestores)
		r.Post("/", api.createExportRestore)
		r.Get("/{restoreId}", api.apiGetExportRestore)
		r.Delete("/{restoreId}", api.deleteExportRestore)
	}
}
