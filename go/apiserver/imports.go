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

	importpkg "github.com/denisvmedia/inventario/import"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type importsAPI struct {
	registrySet    *registry.Set
	uploadLocation string
}

func newImportsAPI(registrySet *registry.Set, uploadLocation string) *importsAPI {
	return &importsAPI{
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
	}
}

// listImports lists all imports.
// @Summary List imports
// @Description get imports
// @Tags imports
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.ImportsResponse "OK"
// @Router /imports [get].
func (api *importsAPI) listImports(w http.ResponseWriter, r *http.Request) {
	imports, err := api.registrySet.ImportRegistry.List(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewImportsResponse(imports)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getImport returns a specific import.
// @Summary Get import
// @Description get import by ID
// @Tags imports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Import ID"
// @Success 200 {object} jsonapi.ImportResponse "OK"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /imports/{id} [get].
func (api *importsAPI) getImport(w http.ResponseWriter, r *http.Request) {
	import_ := r.Context().Value(ctxValueKey("import")).(*models.Import)

	if err := render.Render(w, r, jsonapi.NewImportResponse(import_)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createImport creates a new import.
// @Summary Create import
// @Description create import
// @Tags imports
// @Accept json-api
// @Produce json-api
// @Param import body jsonapi.ImportCreateRequest true "Import request"
// @Success 201 {object} jsonapi.ImportResponse "Created"
// @Failure 400 {object} jsonapi.ErrorResponse "Bad Request"
// @Router /imports [post].
func (api *importsAPI) createImport(w http.ResponseWriter, r *http.Request) {
	data := &jsonapi.ImportCreateRequest{}
	if err := render.Bind(r, data); err != nil {
		renderEntityError(w, r, err)
		return
	}

	import_ := *data.Data.Attributes

	// Set created date (we do not accept it from the client)
	import_.CreatedDate = models.PNow()

	createdImport, err := api.registrySet.ImportRegistry.Create(r.Context(), import_)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Start import processing in background
	go api.processImport(context.Background(), createdImport.ID)

	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewImportResponse(createdImport)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// uploadImportFile uploads an XML file for import.
// @Summary Upload import file
// @Description upload XML file for import
// @Tags imports
// @Accept multipart/form-data
// @Produce json-api
// @Param file formData file true "XML file to import"
// @Success 200 {object} jsonapi.UploadResponse "OK"
// @Failure 400 {object} jsonapi.ErrorResponse "Bad Request"
// @Router /imports/upload [post].
func (api *importsAPI) uploadImportFile(w http.ResponseWriter, r *http.Request) {
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
		Type:      "imports",
		FileNames: []string{filename},
	}

	response := jsonapi.NewUploadResponse("", uploadData).WithStatusCode(http.StatusOK)

	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteImport deletes an import.
// @Summary Delete import
// @Description delete import
// @Tags imports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Import ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.ErrorResponse "Not Found"
// @Router /imports/{id} [delete].
func (api *importsAPI) deleteImport(w http.ResponseWriter, r *http.Request) {
	import_ := r.Context().Value(ctxValueKey("import")).(*models.Import)

	// Delete the source file if it exists
	if import_.SourceFilePath != "" {
		if err := api.deleteSourceFile(r.Context(), import_.SourceFilePath); err != nil {
			// Log error but don't fail the delete operation
			fmt.Printf("Warning: failed to delete source file %s: %v\n", import_.SourceFilePath, err)
		}
	}

	// Delete import record
	if err := api.registrySet.ImportRegistry.Delete(r.Context(), import_.ID); err != nil {
		internalServerError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// processImport processes an import operation in the background
func (api *importsAPI) processImport(ctx context.Context, importID string) {
	// Get import record
	import_, err := api.registrySet.ImportRegistry.Get(ctx, importID)
	if err != nil {
		fmt.Printf("Error: failed to get import %s: %v\n", importID, err)
		return
	}

	// Update status to running
	import_.Status = models.ImportStatusRunning
	import_.StartedDate = models.PNow()
	if _, err := api.registrySet.ImportRegistry.Update(ctx, *import_); err != nil {
		fmt.Printf("Error: failed to update import status %s: %v\n", importID, err)
		return
	}

	// Open source file
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		api.markImportFailed(ctx, importID, fmt.Sprintf("failed to open blob bucket: %v", err))
		return
	}
	defer b.Close()

	reader, err := b.NewReader(ctx, import_.SourceFilePath, nil)
	if err != nil {
		api.markImportFailed(ctx, importID, fmt.Sprintf("failed to open source file: %v", err))
		return
	}
	defer reader.Close()

	// Create import service and process
	service := importpkg.NewImportService(api.registrySet, api.uploadLocation)
	stats, err := service.ImportFromXML(ctx, reader)
	if err != nil {
		api.markImportFailed(ctx, importID, fmt.Sprintf("import failed: %v", err))
		return
	}

	// Update import record with results
	import_.Status = models.ImportStatusCompleted
	import_.CompletedDate = models.PNow()
	import_.LocationCount = stats.LocationCount
	import_.AreaCount = stats.AreaCount
	import_.CommodityCount = stats.CommodityCount
	import_.ImageCount = stats.ImageCount
	import_.InvoiceCount = stats.InvoiceCount
	import_.ManualCount = stats.ManualCount
	import_.BinaryDataSize = stats.BinaryDataSize
	import_.ErrorCount = stats.ErrorCount
	import_.Errors = models.ValuerSlice[string](stats.Errors)

	if _, err := api.registrySet.ImportRegistry.Update(ctx, *import_); err != nil {
		fmt.Printf("Error: failed to update import results %s: %v\n", importID, err)
	}
}

// markImportFailed marks an import as failed with an error message
func (api *importsAPI) markImportFailed(ctx context.Context, importID, errorMessage string) {
	import_, err := api.registrySet.ImportRegistry.Get(ctx, importID)
	if err != nil {
		fmt.Printf("Error: failed to get import %s for failure update: %v\n", importID, err)
		return
	}

	import_.Status = models.ImportStatusFailed
	import_.CompletedDate = models.PNow()
	import_.ErrorMessage = errorMessage

	if _, err := api.registrySet.ImportRegistry.Update(ctx, *import_); err != nil {
		fmt.Printf("Error: failed to mark import as failed %s: %v\n", importID, err)
	}
}

// deleteSourceFile deletes a source file from blob storage
func (api *importsAPI) deleteSourceFile(ctx context.Context, filename string) error {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open blob bucket")
	}
	defer b.Close()

	return b.Delete(ctx, filename)
}

// importCtx middleware is used to load an Import object from
// the URL parameters passed through as the request. In case
// the Import could not be found, we stop here and return a 404.
func importCtx(importRegistry registry.ImportRegistry) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var import_ *models.Import
			var err error

			if importID := chi.URLParam(r, "id"); importID != "" {
				import_, err = importRegistry.Get(r.Context(), importID)
			} else {
				notFound(w, r)
				return
			}
			if err != nil {
				notFound(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ctxValueKey("import"), import_)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Imports sets up the imports API routes.
func Imports(params Params) func(r chi.Router) {
	api := &importsAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listImports)
		r.Post("/", api.createImport)
		r.Post("/upload", api.uploadImportFile)

		r.Route("/{id}", func(r chi.Router) {
			r.Use(importCtx(params.RegistrySet.ImportRegistry))
			r.Get("/", api.getImport)
			r.Delete("/", api.deleteImport)
		})
	}
}
