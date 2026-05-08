package apiserver

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
)

type exportRestoresAPI struct {
	restoreStatus RestoreStatusQuerier
	// currencyMigrationLockEnabled mirrors Params.FeatureCurrencyMigration.
	// When true, createExportRestore queries InFlightForGroup before
	// inserting and rejects with HTTP 423 if a migration is pending or
	// running on the group (issue #202 §3.2 cross-op lock). The
	// commodity-write side of the lock is enforced by the
	// requireGroupNotMigrating middleware in apiserver.go; this is the
	// in-handler symmetric guard for the restore-create endpoint.
	currencyMigrationLockEnabled bool
}

// listExportRestores lists all restore operations for an export.
// @Summary List export restore operations
// @Description get restore operations for an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Success 200 {object} jsonapi.RestoreOperationsResponse "OK"
// @Router /g/{groupSlug}/exports/{id}/restores [get].
func (api *exportRestoresAPI) listExportRestores(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	expReg := registrySet.ExportRegistry

	exportID := chi.URLParam(r, "id")
	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := expReg.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOpReg := registrySet.RestoreOperationRegistry

	restoreOperations, err := restoreOpReg.ListByExport(r.Context(), exportID)
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
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Param restoreId path string true "Restore Operation ID"
// @Success 200 {object} jsonapi.RestoreOperationResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Not found"
// @Router /g/{groupSlug}/exports/{id}/restores/{restoreId} [get].
func (api *exportRestoresAPI) apiGetExportRestore(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	exportID := chi.URLParam(r, "id")
	restoreID := chi.URLParam(r, "restoreId")

	expReg := registrySet.ExportRegistry
	restoreOpReg := registrySet.RestoreOperationRegistry
	stepReg := registrySet.RestoreStepRegistry

	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	if restoreID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := expReg.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOperation, err := restoreOpReg.Get(r.Context(), restoreID)
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
	steps, err := stepReg.ListByRestoreOperation(r.Context(), restoreID)
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
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Param request body jsonapi.RestoreOperationCreateRequest true "Restore operation data"
// @Success 201 {object} jsonapi.RestoreOperationResponse "Created"
// @Failure 400 {object} jsonapi.Errors "Bad request"
// @Failure 404 {object} jsonapi.Errors "Not found"
// @Router /g/{groupSlug}/exports/{id}/restores [post].
func (api *exportRestoresAPI) createExportRestore(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	exportID := chi.URLParam(r, "id")
	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	expReg := registrySet.ExportRegistry

	// Verify export exists and is completed
	export, err := expReg.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	restoreOpReg := registrySet.RestoreOperationRegistry

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
	hasRunning, err := api.restoreStatus.HasRunningRestores(r.Context())
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

	// Cross-op lock with the currency-migration system (issue #202 §3.2).
	// A restore mid-migration would rewrite commodities while the worker
	// is rewriting prices, corrupting totals; reject with 423 and let
	// the FE surface a friendly toast.
	if api.currencyMigrationLockEnabled {
		registrySet := RegistrySetFromContext(r.Context())
		group := groupFromContext(r.Context())
		if registrySet != nil && group != nil && registrySet.CurrencyMigrationRegistry != nil {
			if inFlight, qerr := registrySet.CurrencyMigrationRegistry.InFlightForGroup(r.Context(), group.ID); qerr != nil {
				_ = internalServerError(w, r, qerr)
				return
			} else if inFlight != nil {
				_ = lockedError(w, r,
					errors.New("group is locked while a currency migration is in progress"),
					codeCurrencyMigrationLocked,
					map[string]any{
						"migration_id": inFlight.ID,
						"status":       string(inFlight.Status),
					},
				)
				return
			}
		}
	}

	restoreOperation := models.NewRestoreOperationFromUserInput(data.Data.Attributes)
	// The restore is scoped to the export from the URL — the FE only
	// sends description+options on the body, so populate ExportID
	// server-side before the registry's Create validates the row.
	// Without this, the row gets persisted with an empty export_id and
	// ListByExport(exportID) returns []; the FE never sees the restore
	// it just created.
	restoreOperation.ExportID = exportID
	createdRestoreOperation, err := restoreOpReg.Create(r.Context(), restoreOperation)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Return immediately with the created restore operation; the restore
	// worker will pick up the pending operation and process it.
	// WithStatusCode is required because RestoreOperationResponse.Render
	// would otherwise reset the status to 200 — see exports.go createExport.
	if err := render.Render(w, r, jsonapi.NewRestoreOperationResponse(createdRestoreOperation).WithStatusCode(http.StatusCreated)); err != nil {
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
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Param restoreId path string true "Restore Operation ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Not found"
// @Router /g/{groupSlug}/exports/{id}/restores/{restoreId} [delete].
func (api *exportRestoresAPI) deleteExportRestore(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	exportID := chi.URLParam(r, "id")
	restoreID := chi.URLParam(r, "restoreId")

	restoreOpReg := registrySet.RestoreOperationRegistry
	expReg := registrySet.ExportRegistry

	if exportID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	if restoreID == "" {
		badRequest(w, r, ErrEntityNotFound)
		return
	}

	// Verify export exists
	_, err := expReg.Get(r.Context(), exportID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Verify the restore operation exists and belongs to this export
	restoreOperation, err := restoreOpReg.Get(r.Context(), restoreID)
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

	err = restoreOpReg.Delete(r.Context(), restoreID)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ExportRestores sets up the export restore API routes.
//
// currencyMigrationLockEnabled toggles the cross-op lock check inside
// createExportRestore (issue #202 §3.2): when an in-flight currency
// migration exists for the group, the restore-create endpoint
// returns HTTP 423 instead of 201. The flag mirrors
// Params.FeatureCurrencyMigration so the codepath is dead when the
// feature is off.
func ExportRestores(restoreStatus RestoreStatusQuerier, currencyMigrationLockEnabled bool) func(r chi.Router) {
	api := &exportRestoresAPI{
		restoreStatus:                restoreStatus,
		currencyMigrationLockEnabled: currencyMigrationLockEnabled,
	}

	return func(r chi.Router) {
		r.Get("/", api.listExportRestores)
		r.Post("/", api.createExportRestore)
		r.Get("/{restoreId}", api.apiGetExportRestore)
		r.Delete("/{restoreId}", api.deleteExportRestore)
	}
}
