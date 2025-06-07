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
	"github.com/denisvmedia/inventario/internal/errkit"
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
	registrySet    *registry.Set
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
	exports, err := api.registrySet.ExportRegistry.List(r.Context())
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

	export := *request.Data.Attributes

	// Set status to pending if not set
	if export.Status == "" {
		export.Status = models.ExportStatusPending
	}

	// Enrich selected items with names from the database
	if export.Type == models.ExportTypeSelectedItems && len(export.SelectedItems) > 0 {
		if err := api.enrichSelectedItemsWithNames(r.Context(), &export); err != nil {
			internalServerError(w, r, err)
			return
		}
	}

	createdExport, err := api.registrySet.ExportRegistry.Create(r.Context(), export)
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

	err := api.registrySet.ExportRegistry.Delete(r.Context(), export.ID)
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

	// Check if export is deleted
	if export.IsDeleted() {
		http.NotFound(w, r)
		return
	}

	// Check if export is completed and has a file path
	if export.Status != models.ExportStatusCompleted || export.FilePath == "" {
		http.NotFound(w, r)
		return
	}

	// Get file attributes to set Content-Length and other headers
	attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, export.FilePath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	file, err := api.getDownloadFile(r.Context(), export.FilePath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	defer file.Close()

	// Generate filename based on export description and type
	filename := path.Base(export.FilePath)
	if filename == "" {
		filename = "export.xml"
	}

	// Set headers to optimize streaming and prevent browser preloading
	// downloadutils.SetStreamingHeaders(w, "application/xml", attrs.Size, filename)
	downloadutils.SetStreamingHeaders(w, "application/octet-stream", attrs.Size, filename)

	// Use chunked copying to prevent browser buffering
	if err := downloadutils.CopyFileInChunks(w, file); err != nil {
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

// enrichSelectedItemsWithNames fetches the names and relationships for selected items and adds them to the export
func (api *exportsAPI) enrichSelectedItemsWithNames(ctx context.Context, export *models.Export) error {
	for i, item := range export.SelectedItems {
		var name string
		var locationID, areaID string
		var err error

		switch item.Type {
		case models.ExportSelectedItemTypeLocation:
			location, getErr := api.registrySet.LocationRegistry.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Location " + item.ID + "]"
			} else {
				name = location.Name
			}
		case models.ExportSelectedItemTypeArea:
			area, getErr := api.registrySet.AreaRegistry.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Area " + item.ID + "]"
			} else {
				name = area.Name
				locationID = area.LocationID // Store the relationship
			}
		case models.ExportSelectedItemTypeCommodity:
			commodity, getErr := api.registrySet.CommodityRegistry.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Commodity " + item.ID + "]"
			} else {
				name = commodity.Name
				areaID = commodity.AreaID // Store the relationship
			}
		default:
			name = "[Unknown Item " + item.ID + "]"
		}

		if err != nil {
			return errkit.Wrap(err, "failed to fetch item name")
		}

		// Update the item with the name and relationships
		export.SelectedItems[i].Name = name
		export.SelectedItems[i].LocationID = locationID
		export.SelectedItems[i].AreaID = areaID
	}

	return nil
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
func Exports(params Params) func(r chi.Router) {
	api := &exportsAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listExports)
		r.Post("/", api.createExport)

		r.Route("/{id}", func(r chi.Router) {
			r.Use(exportCtx(params.RegistrySet))
			r.Get("/", api.getExport)
			r.Delete("/", api.deleteExport)
			r.Get("/download", api.downloadExport)
		})
	}
}
