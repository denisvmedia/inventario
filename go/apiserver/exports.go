package apiserver

import (
	"context"
	"io"
	"net/http"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
	"github.com/denisvmedia/inventario/backup/export"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

const exportCtxKey ctxValueKey = "export"

func exportFromContext(ctx context.Context) *models.Export {
	exp, ok := ctx.Value(exportCtxKey).(*models.Export)
	if !ok {
		return nil
	}
	return exp
}

type exportsAPI struct {
	registrySet    *registry.Set
	uploadLocation string
	entityService  *services.EntityService
}

// listExports lists all exports.
// @Summary List exports
// @Description get exports
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param include_deleted query bool false "Include deleted exports"
// @Success 200 {object} jsonapi.ExportsResponse "OK"
// @Router /exports [get].
func (api *exportsAPI) listExports(w http.ResponseWriter, r *http.Request) {
	// Check if we should include deleted exports
	includeDeleted := r.URL.Query().Get("include_deleted") == "true"

	var exports []*models.Export
	var err error

	if includeDeleted {
		exports, err = api.registrySet.ExportRegistry.ListWithDeleted(r.Context())
	} else {
		exports, err = api.registrySet.ExportRegistry.List(r.Context())
	}

	if err != nil {
		internalServerError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewExportsResponse(exports, len(exports))); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// apiGetExport gets an export by ID.
// @Summary Get an export
// @Description get export by ID
// @Tags exports
// @Accept  json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Success 200 {object} jsonapi.ExportResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /exports/{id} [get].
func (api *exportsAPI) apiGetExport(w http.ResponseWriter, r *http.Request) {
	exp := exportFromContext(r.Context())
	if exp == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	if err := render.Render(w, r, jsonapi.NewExportResponse(exp)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// createExport creates a new export.
// @Summary Create an export
// @Description create a new export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param export body jsonapi.ExportCreateRequest true "Export"
// @Success 201 {object} jsonapi.ExportResponse "Created"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Router /exports [post].
func (api *exportsAPI) createExport(w http.ResponseWriter, r *http.Request) {
	var request jsonapi.ExportCreateRequest
	if err := render.Bind(r, &request); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	svc := export.NewExportService(api.registrySet, api.uploadLocation)
	// Create an export to be later processed by the export worker
	createdExport, err := svc.CreateExportFromUserInput(r.Context(), request.Data.Attributes)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewExportResponse(&createdExport)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteExport deletes an export.
// @Summary Delete an export
// @Description delete an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /exports/{id} [delete].
func (api *exportsAPI) deleteExport(w http.ResponseWriter, r *http.Request) {
	exp := exportFromContext(r.Context())
	if exp == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	// Use entity service to properly handle export and file deletion
	err := api.entityService.DeleteExportWithFile(r.Context(), exp.ID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// downloadExport downloads an export file.
// @Summary Download an export file
// @Description Download an export XML file
// @Tags exports
// @Accept octet-stream
// @Produce octet-stream
// @Param id path string true "Export ID"
// @Success 200 {file} application/xml "Export file"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /exports/{id}/download [get].
func (api *exportsAPI) downloadExport(w http.ResponseWriter, r *http.Request) {
	exp := exportFromContext(r.Context())
	if exp == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	// Check if export is deleted
	if exp.IsDeleted() {
		http.NotFound(w, r)
		return
	}

	// Check if export is completed and has a file entity
	if exp.Status != models.ExportStatusCompleted {
		http.NotFound(w, r)
		return
	}

	// Get the file entity for the export
	var fileEntity *models.FileEntity
	var err error

	switch {
	case exp.FileID != "":
		// Use new file entity system
		fileEntity, err = api.registrySet.FileRegistry.Get(r.Context(), exp.FileID)
		if err != nil {
			internalServerError(w, r, err)
			return
		}
	case exp.FilePath != "":
		// Fallback to old file path system for backward compatibility
		attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, exp.FilePath)
		if err != nil {
			internalServerError(w, r, err)
			return
		}

		file, err := api.getDownloadFile(r.Context(), exp.FilePath)
		if err != nil {
			internalServerError(w, r, err)
			return
		}
		defer file.Close()

		filename := path.Base(exp.FilePath)
		if filename == "" {
			filename = "export.xml"
		}

		downloadutils.SetStreamingHeaders(w, "application/xml", attrs.Size, filename)
		if err := downloadutils.CopyFileInChunks(w, file); err != nil {
			internalServerError(w, r, err)
			return
		}
		return
	default:
		http.NotFound(w, r)
		return
	}

	// Use file entity download
	if fileEntity.File == nil {
		http.NotFound(w, r)
		return
	}

	// Get file attributes for Content-Length header
	attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, fileEntity.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Set streaming headers
	filename := fileEntity.Path + fileEntity.Ext
	downloadutils.SetStreamingHeaders(w, fileEntity.MIMEType, attrs.Size, filename)

	// Open and stream the file
	b, err := blob.OpenBucket(r.Context(), api.uploadLocation)
	if err != nil {
		internalServerError(w, r, errkit.Wrap(err, "failed to open bucket"))
		return
	}
	defer b.Close()

	reader, err := b.NewReader(r.Context(), fileEntity.OriginalPath, nil)
	if err != nil {
		internalServerError(w, r, errkit.Wrap(err, "failed to open file"))
		return
	}
	defer reader.Close()

	// Stream the file in chunks
	if err := downloadutils.CopyFileInChunks(w, reader); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// importExport imports an XML export file and creates an export record
// @Summary Import XML export
// @Description Import an uploaded XML export file and create an export record
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param data body jsonapi.ImportExportRequest true "Import request data"
// @Success 201 {object} jsonapi.ExportResponse "Created"
// @Failure 400 {object} jsonapi.Errors "Bad Request"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Router /exports/import [post].
func (api *exportsAPI) importExport(w http.ResponseWriter, r *http.Request) {
	var data jsonapi.ImportExportRequest
	if err := render.Bind(r, &data); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Create export record with pending status and source file path
	importedExport := models.NewImportedExport(data.Data.Attributes.Description, data.Data.Attributes.SourceFilePath)

	createdExport, err := api.registrySet.ExportRegistry.Create(r.Context(), importedExport)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Import worker will pick up this pending import and process it in background
	// Return immediately with the created export
	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewExportResponse(createdExport)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func (api *exportsAPI) getDownloadFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to open bucket")
	}
	defer b.Close()

	return b.NewReader(context.Background(), filePath, nil)
}

func exportCtx(registrySet *registry.Set) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			exportID := chi.URLParam(r, "id")
			if exportID == "" {
				next.ServeHTTP(w, r)
				return
			}

			export, err := registrySet.ExportRegistry.Get(r.Context(), exportID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}

			ctx := context.WithValue(r.Context(), exportCtxKey, export)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Exports sets up the exports API routes.
func Exports(params Params, restoreWorker RestoreWorkerInterface) func(r chi.Router) {
	api := &exportsAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
		entityService:  params.EntityService,
	}

	return func(r chi.Router) {
		r.Get("/", api.listExports)
		r.Post("/", api.createExport)
		r.Post("/import", api.importExport)

		r.Route("/{id}", func(r chi.Router) {
			r.Use(exportCtx(params.RegistrySet))
			r.Get("/", api.apiGetExport)
			r.Delete("/", api.deleteExport)
			r.Get("/download", api.downloadExport)
			r.Route("/restores", ExportRestores(params, restoreWorker))
		})
	}
}


