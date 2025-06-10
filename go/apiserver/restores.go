package apiserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/restore"
)

type restoresAPI struct {
	registrySet    *registry.Set
	uploadLocation string
}

func newRestoresAPI(registrySet *registry.Set, uploadLocation string) *restoresAPI {
	return &restoresAPI{
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
	}
}

// listRestores lists all restore operations.
// @Summary List restores
// @Description get restores
// @Tags restores
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.RestoresResponse "OK"
// @Router /restores [get].
func (api *restoresAPI) listRestores(w http.ResponseWriter, r *http.Request) {
	imports, err := api.registrySet.ImportRegistry.List(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Filter for restore operations (we'll use the same Import model but with different type)
	var restores []*models.Import
	for _, imp := range imports {
		if imp.Type == models.ImportTypeXMLBackup {
			restores = append(restores, imp)
		}
	}

	if err := render.Render(w, r, jsonapi.NewRestoresResponse(restores)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getRestore returns a specific restore operation.
// @Summary Get restore
// @Description get restore by ID
// @Tags restores
// @Accept json-api
// @Produce json-api
// @Param id path string true "Restore ID"
// @Success 200 {object} jsonapi.RestoreResponse "OK"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /restores/{id} [get].
func (api *restoresAPI) getRestore(w http.ResponseWriter, r *http.Request) {
	restore := r.Context().Value(ctxValueKey("restore")).(*models.Import)

	if err := render.Render(w, r, jsonapi.NewRestoreResponse(restore)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createRestore creates a new restore operation.
// @Summary Create restore
// @Description create restore operation
// @Tags restores
// @Accept json-api
// @Produce json-api
// @Param restore body jsonapi.RestoreCreateRequest true "Restore request"
// @Success 201 {object} jsonapi.RestoreResponse "Created"
// @Failure 400 {object} jsonapi.ErrorResponse "Bad Request"
// @Router /restores [post].
func (api *restoresAPI) createRestore(w http.ResponseWriter, r *http.Request) {
	data := &jsonapi.RestoreCreateRequest{}
	if err := render.Bind(r, data); err != nil {
		renderEntityError(w, r, err)
		return
	}

	restore := *data.Data.Attributes

	// Set created date (we do not accept it from the client)
	restore.CreatedDate = models.PNow()

	createdRestore, err := api.registrySet.ImportRegistry.Create(r.Context(), restore)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Start restore processing in background
	go api.processRestore(context.Background(), createdRestore.ID, data.Options)

	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewRestoreResponse(createdRestore)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// uploadRestoreFile uploads an XML file for restore.
// @Summary Upload restore file
// @Description upload XML file for restore
// @Tags restores
// @Accept multipart/form-data
// @Produce json-api
// @Param file formData file true "XML file to restore"
// @Success 200 {object} jsonapi.UploadResponse "OK"
// @Failure 400 {object} jsonapi.ErrorResponse "Bad Request"
// @Router /restores/upload [post].
func (api *restoresAPI) uploadRestoreFile(w http.ResponseWriter, r *http.Request) {
	// Get uploaded file
	file, header, err := r.FormFile("file")
	if err != nil {
		badRequest(w, r, errkit.Wrap(err, "failed to get uploaded file"))
		return
	}
	defer file.Close()

	// Validate file type
	if filepath.Ext(header.Filename) != ".xml" {
		badRequest(w, r, errkit.WithMessage(nil, "only XML files are allowed"))
		return
	}

	// Generate unique filename
	filename := filekit.UploadFileName(header.Filename)

	// Save file to blob storage
	b, err := blob.OpenBucket(r.Context(), api.uploadLocation)
	if err != nil {
		internalServerError(w, r, errkit.Wrap(err, "failed to open blob bucket"))
		return
	}
	defer b.Close()

	writer, err := b.NewWriter(r.Context(), filename, nil)
	if err != nil {
		internalServerError(w, r, errkit.Wrap(err, "failed to create blob writer"))
		return
	}
	defer writer.Close()

	// Copy file content
	if _, err := io.Copy(writer, file); err != nil {
		internalServerError(w, r, errkit.Wrap(err, "failed to save file"))
		return
	}

	uploadData := jsonapi.UploadData{
		Type:      "restores",
		FileNames: []string{filename},
	}

	response := jsonapi.NewUploadResponse("", uploadData).WithStatusCode(http.StatusOK)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteRestore deletes a restore operation.
// @Summary Delete restore
// @Description delete restore operation
// @Tags restores
// @Accept json-api
// @Produce json-api
// @Param id path string true "Restore ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /restores/{id} [delete].
func (api *restoresAPI) deleteRestore(w http.ResponseWriter, r *http.Request) {
	restore := r.Context().Value(ctxValueKey("restore")).(*models.Import)

	// Delete the source file if it exists
	if restore.SourceFilePath != "" {
		if err := api.deleteSourceFile(r.Context(), restore.SourceFilePath); err != nil {
			// Log error but don't fail the delete operation
			fmt.Printf("Warning: failed to delete source file %s: %v\n", restore.SourceFilePath, err)
		}
	}

	// Delete restore record
	if err := api.registrySet.ImportRegistry.Delete(r.Context(), restore.ID); err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// processRestore processes a restore operation in the background
func (api *restoresAPI) processRestore(ctx context.Context, restoreID string, options restore.RestoreOptions) {
	// Get restore record
	restoreRecord, err := api.registrySet.ImportRegistry.Get(ctx, restoreID)
	if err != nil {
		fmt.Printf("Error: failed to get restore %s: %v\n", restoreID, err)
		return
	}

	// Update status to running
	restoreRecord.Status = models.ImportStatusRunning
	restoreRecord.StartedDate = models.PNow()
	if _, err := api.registrySet.ImportRegistry.Update(ctx, *restoreRecord); err != nil {
		fmt.Printf("Error: failed to update restore status %s: %v\n", restoreID, err)
		return
	}

	// Open source file
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		api.markRestoreFailed(ctx, restoreID, fmt.Sprintf("failed to open blob bucket: %v", err))
		return
	}
	defer b.Close()

	reader, err := b.NewReader(ctx, restoreRecord.SourceFilePath, nil)
	if err != nil {
		api.markRestoreFailed(ctx, restoreID, fmt.Sprintf("failed to open source file: %v", err))
		return
	}
	defer reader.Close()

	// Create restore service and process
	service := restore.NewRestoreService(api.registrySet, api.uploadLocation)
	stats, err := service.RestoreFromXML(ctx, reader, options)
	if err != nil {
		api.markRestoreFailed(ctx, restoreID, fmt.Sprintf("restore failed: %v", err))
		return
	}

	// Update restore record with results
	restoreRecord.Status = models.ImportStatusCompleted
	restoreRecord.CompletedDate = models.PNow()
	restoreRecord.LocationCount = stats.LocationCount
	restoreRecord.AreaCount = stats.AreaCount
	restoreRecord.CommodityCount = stats.CommodityCount
	restoreRecord.ImageCount = stats.ImageCount
	restoreRecord.InvoiceCount = stats.InvoiceCount
	restoreRecord.ManualCount = stats.ManualCount
	restoreRecord.BinaryDataSize = stats.BinaryDataSize
	restoreRecord.ErrorCount = stats.ErrorCount
	restoreRecord.Errors = models.ValuerSlice[string](stats.Errors)

	if _, err := api.registrySet.ImportRegistry.Update(ctx, *restoreRecord); err != nil {
		fmt.Printf("Error: failed to update restore results %s: %v\n", restoreID, err)
	}
}

// markRestoreFailed marks a restore as failed with an error message
func (api *restoresAPI) markRestoreFailed(ctx context.Context, restoreID, errorMessage string) {
	restoreRecord, err := api.registrySet.ImportRegistry.Get(ctx, restoreID)
	if err != nil {
		fmt.Printf("Error: failed to get restore %s for failure update: %v\n", restoreID, err)
		return
	}

	restoreRecord.Status = models.ImportStatusFailed
	restoreRecord.CompletedDate = models.PNow()
	restoreRecord.ErrorMessage = errorMessage

	if _, err := api.registrySet.ImportRegistry.Update(ctx, *restoreRecord); err != nil {
		fmt.Printf("Error: failed to mark restore as failed %s: %v\n", restoreID, err)
	}
}

// deleteSourceFile deletes a source file from blob storage
func (api *restoresAPI) deleteSourceFile(ctx context.Context, filename string) error {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open blob bucket")
	}
	defer b.Close()

	return b.Delete(ctx, filename)
}

// restoreCtx middleware is used to load a Restore object from
// the URL parameters passed through as the request. In case
// the Restore could not be found, we stop here and return a 404.
func restoreCtx(importRegistry registry.ImportRegistry) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var restore *models.Import
			var err error

			if restoreID := chi.URLParam(r, "id"); restoreID != "" {
				restore, err = importRegistry.Get(r.Context(), restoreID)
			} else {
				notFound(w, r)
				return
			}
			if err != nil {
				notFound(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ctxValueKey("restore"), restore)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Restores sets up the restores API routes.
func Restores(params Params) func(r chi.Router) {
	api := &restoresAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listRestores)
		r.Post("/", api.createRestore)
		r.Post("/upload", api.uploadRestoreFile)

		r.Route("/{id}", func(r chi.Router) {
			r.Use(restoreCtx(params.RegistrySet.ImportRegistry))
			r.Get("/", api.getRestore)
			r.Delete("/", api.deleteRestore)
		})
	}
}
