package apiserver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
	"github.com/denisvmedia/inventario/export"
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

	// Set created date (we do not accept it from the client)
	export.CreatedDate = models.PNow()

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

// importExport imports an XML export file and creates an export record
// @Summary Import XML export
// @Description Import an uploaded XML export file and create an export record
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param data body jsonapi.ImportExportRequest true "Import request data"
// @Success 201 {object} jsonapi.ExportResponse "Created"
// @Failure 400 {object} jsonapi.ErrorResponse "Bad Request"
// @Failure 422 {object} jsonapi.ErrorResponse "Unprocessable Entity"
// @Router /exports/import [post].
func (api *exportsAPI) importExport(w http.ResponseWriter, r *http.Request) {
	var data jsonapi.ImportExportRequest
	if err := render.Bind(r, &data); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Create export record with pending status first
	export := models.Export{
		Description: data.Data.Attributes.Description,
		Type:        models.ExportTypeImported,
		Status:      models.ExportStatusPending,
		CreatedDate: models.PNow(),
	}

	createdExport, err := api.registrySet.ExportRegistry.Create(r.Context(), export)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Start import processing in background
	go func() {
		// Create a new context for the background operation
		bgCtx := context.Background()
		api.processImportInBackground(bgCtx, createdExport.ID, data.Data.Attributes.SourceFilePath)
	}()

	// Return immediately with the created export
	w.WriteHeader(http.StatusCreated)
	if err := render.Render(w, r, jsonapi.NewExportResponse(createdExport)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// processImportInBackground processes an XML import in the background
func (api *exportsAPI) processImportInBackground(ctx context.Context, exportID, sourceFilePath string) {
	// Get the export record
	exportRecord, err := api.registrySet.ExportRegistry.Get(ctx, exportID)
	if err != nil {
		api.markImportFailed(ctx, exportID, fmt.Sprintf("failed to get export record: %v", err))
		return
	}

	// Update status to in progress
	exportRecord.Status = models.ExportStatusInProgress
	_, err = api.registrySet.ExportRegistry.Update(ctx, *exportRecord)
	if err != nil {
		api.markImportFailed(ctx, exportID, fmt.Sprintf("failed to update export status: %v", err))
		return
	}

	// Create export service
	exportService := export.NewExportService(api.registrySet, api.uploadLocation)

	// Open blob bucket to read the XML file
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		api.markImportFailed(ctx, exportID, fmt.Sprintf("failed to open blob bucket: %v", err))
		return
	}
	defer b.Close()

	// Open the uploaded XML file
	reader, err := b.NewReader(ctx, sourceFilePath, nil)
	if err != nil {
		api.markImportFailed(ctx, exportID, fmt.Sprintf("failed to open uploaded XML file: %v", err))
		return
	}
	defer reader.Close()

	// Get file size
	attrs, err := b.Attributes(ctx, sourceFilePath)
	if err != nil {
		api.markImportFailed(ctx, exportID, fmt.Sprintf("failed to get file attributes: %v", err))
		return
	}

	// Parse XML to extract metadata and statistics (without creating a new record)
	stats, _, err := exportService.ParseXMLMetadata(ctx, reader)
	if err != nil {
		api.markImportFailed(ctx, exportID, fmt.Sprintf("failed to parse XML metadata: %v", err))
		return
	}

	// Update the original export record with the parsed data
	exportRecord.Status = models.ExportStatusCompleted
	exportRecord.CompletedDate = models.PNow()
	exportRecord.FilePath = sourceFilePath
	exportRecord.FileSize = attrs.Size
	exportRecord.LocationCount = stats.LocationCount
	exportRecord.AreaCount = stats.AreaCount
	exportRecord.CommodityCount = stats.CommodityCount
	exportRecord.ImageCount = stats.ImageCount
	exportRecord.InvoiceCount = stats.InvoiceCount
	exportRecord.ManualCount = stats.ManualCount
	exportRecord.BinaryDataSize = stats.BinaryDataSize
	exportRecord.IncludeFileData = stats.BinaryDataSize > 0

	_, err = api.registrySet.ExportRegistry.Update(ctx, *exportRecord)
	if err != nil {
		// Log error but don't fail the import since it actually succeeded
		fmt.Printf("Failed to update export with final stats: %v\n", err)
	}
}

// markImportFailed marks an import operation as failed with an error message
func (api *exportsAPI) markImportFailed(ctx context.Context, exportID, errorMessage string) {
	exportRecord, err := api.registrySet.ExportRegistry.Get(ctx, exportID)
	if err != nil {
		fmt.Printf("Failed to get export for error marking: %v\n", err)
		return
	}

	exportRecord.Status = models.ExportStatusFailed
	exportRecord.CompletedDate = models.PNow()
	exportRecord.ErrorMessage = errorMessage

	_, err = api.registrySet.ExportRegistry.Update(ctx, *exportRecord)
	if err != nil {
		fmt.Printf("Failed to update export with error: %v\n", err)
	}
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
func Exports(params Params, restoreWorker RestoreWorkerInterface) func(r chi.Router) {
	api := &exportsAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listExports)
		r.Post("/", api.createExport)
		r.Post("/import", api.importExport)

		r.Route("/{id}", func(r chi.Router) {
			r.Use(exportCtx(params.RegistrySet))
			r.Get("/", api.getExport)
			r.Delete("/", api.deleteExport)
			r.Get("/download", api.downloadExport)
			r.Route("/restores", ExportRestores(params, restoreWorker))
		})
	}
}
