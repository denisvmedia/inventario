package apiserver

import (
	"context"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/mimekit"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const exportCtxKey ctxValueKey = "export"

func exportFromContext(ctx context.Context) *models.Export {
	export, ok := ctx.Value(exportCtxKey).(*models.Export)
	if !ok {
		return nil
	}
	return export
}

type exportsAPI struct {
	exportRegistry registry.ExportRegistry
	uploadLocation string
}

// listExports lists all exports.
// @Summary List exports
// @Description get exports
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Success 200 {object} jsonapi.ExportsResponse "OK"
// @Router /exports [get].
func (api *exportsAPI) listExports(w http.ResponseWriter, r *http.Request) {
	exports, err := api.exportRegistry.List(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewExportsResponse(exports, len(exports))); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// getExport gets an export by ID.
// @Summary Get an export
// @Description get export by ID
// @Tags exports
// @Accept  json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Success 200 {object} jsonapi.ExportResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /exports/{id} [get].
//
//nolint:revive // getExport is an HTTP handler, not a getter function
func (api *exportsAPI) getExport(w http.ResponseWriter, r *http.Request) {
	export := exportFromContext(r.Context())
	if export == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	if err := render.Render(w, r, jsonapi.NewExportResponse(export)); err != nil {
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

	export := request.Data.ToModel()

	// Set status to pending if not set
	if export.Status == "" {
		export.Status = models.ExportStatusPending
	}

	createdExport, err := api.exportRegistry.Create(r.Context(), export)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewExportResponse(createdExport)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateExport updates an export.
// @Summary Update an export
// @Description update an export
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param id path string true "Export ID"
// @Param export body jsonapi.ExportUpdateRequest true "Export"
// @Success 200 {object} jsonapi.ExportResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Router /exports/{id} [patch].
func (api *exportsAPI) updateExport(w http.ResponseWriter, r *http.Request) {
	export := exportFromContext(r.Context())
	if export == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	var request jsonapi.ExportUpdateRequest
	if err := render.Bind(r, &request); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Update export fields
	updatedExport := request.Data.ToModel()
	updatedExport.ID = export.ID

	result, err := api.exportRegistry.Update(r.Context(), updatedExport)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if err := render.Render(w, r, jsonapi.NewExportResponse(result)); err != nil {
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
	export := exportFromContext(r.Context())
	if export == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	err := api.exportRegistry.Delete(r.Context(), export.ID)
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
	export := exportFromContext(r.Context())
	if export == nil {
		unprocessableEntityError(w, r, nil)
		return
	}

	// Check if export is completed and has a file path
	if export.Status != models.ExportStatusCompleted || export.FilePath == "" {
		http.NotFound(w, r)
		return
	}

	file, err := api.getDownloadFile(r.Context(), export.FilePath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/xml")
	// Generate filename based on export description and type
	filename := "export.xml"
	if export.Description != "" {
		filename = export.Description + ".xml"
	}
	attachmentHeader := mimekit.FormatContentDisposition(filename)
	w.Header().Set("Content-Disposition", attachmentHeader)

	if _, err := io.Copy(w, file); err != nil {
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

func exportCtx(exportRegistry registry.ExportRegistry) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			exportID := chi.URLParam(r, "id")
			if exportID == "" {
				next.ServeHTTP(w, r)
				return
			}

			export, err := exportRegistry.Get(r.Context(), exportID)
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
func Exports(exportRegistry registry.ExportRegistry, uploadLocation string) func(r chi.Router) {
	api := &exportsAPI{
		exportRegistry: exportRegistry,
		uploadLocation: uploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listExports)
		r.Post("/", api.createExport)

		r.Route("/{id}", func(r chi.Router) {
			r.Use(exportCtx(exportRegistry))
			r.Get("/", api.getExport)
			r.Patch("/", api.updateExport)
			r.Delete("/", api.deleteExport)
			r.Get("/download", api.downloadExport)
		})
	}
}
