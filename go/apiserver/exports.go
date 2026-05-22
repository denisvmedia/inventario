package apiserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/export"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
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
	uploadLocation     string
	entityService      *services.EntityService
	fileSigningService *services.FileSigningService
}

// listExports lists all exports.
// @Summary List exports
// @Description get exports
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param include_deleted query bool false "Include deleted exports"
// @Success 200 {object} jsonapi.ExportsResponse "OK"
// @Router /g/{groupSlug}/exports [get].
func (api *exportsAPI) listExports(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	expReg := registrySet.ExportRegistry

	// Check if we should include deleted exports
	includeDeleted := r.URL.Query().Get("include_deleted") == "true"

	var exports []*models.Export
	var err error

	if includeDeleted {
		exports, err = expReg.ListWithDeleted(r.Context())
	} else {
		exports, err = expReg.List(r.Context())
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
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Success 200 {object} jsonapi.ExportResponse "OK"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /g/{groupSlug}/exports/{id} [get].
func (api *exportsAPI) apiGetExport(w http.ResponseWriter, r *http.Request) {
	err := appctx.ValidateUserContext(r.Context())
	if err != nil {
		unauthorizedError(w, r, err)
		return
	}

	exp := exportFromContext(r.Context())
	if exp == nil {
		unprocessableEntityError(w, r, errors.New("export not found in context"))
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
// @Param groupSlug path string true "Group slug"
// @Param export body jsonapi.ExportCreateRequest true "Export"
// @Success 201 {object} jsonapi.ExportResponse "Created"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Router /g/{groupSlug}/exports [post].
func (api *exportsAPI) createExport(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	err := appctx.ValidateUserContext(r.Context())
	if err != nil {
		unauthorizedError(w, r, err)
		return
	}

	var request jsonapi.ExportCreateRequest
	if err := render.Bind(r, &request); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Create an export to be later processed by the export worker
	createdExport, err := export.CreateExportFromUserInput(r.Context(), registrySet, request.Data.Attributes)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// `WithStatusCode` is required because ExportResponse.Render
	// unconditionally calls render.Status(r, statusCodeDef(...,
	// StatusOK)) — without setting HTTPStatusCode the renderer
	// overwrites a handler-level render.Status back to 200.
	if err := render.Render(w, r, jsonapi.NewExportResponse(&createdExport).WithStatusCode(http.StatusCreated)); err != nil {
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
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /g/{groupSlug}/exports/{id} [delete].
func (api *exportsAPI) deleteExport(w http.ResponseWriter, r *http.Request) {
	err := appctx.ValidateUserContext(r.Context())
	if err != nil {
		unauthorizedError(w, r, err)
		return
	}

	exp := exportFromContext(r.Context())
	if exp == nil {
		unprocessableEntityError(w, r, errors.New("export not found in context"))
		return
	}

	// Use entity service to properly handle export and file deletion
	err = api.entityService.DeleteExportWithFile(r.Context(), exp.ID)
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
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Success 200 {file} application/xml "Export file"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /g/{groupSlug}/exports/{id}/download [get].
func (api *exportsAPI) downloadExport(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	exp := exportFromContext(r.Context())
	if exp == nil {
		unprocessableEntityError(w, r, errors.New("export not found in context"))
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
	case exp.FileID != nil && *exp.FileID != "":
		// Use new file entity system
		fileEntity, err = fileReg.Get(r.Context(), *exp.FileID)
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
		internalServerError(w, r, errxtrace.Wrap("failed to open bucket", err))
		return
	}
	defer b.Close()

	reader, err := b.NewReader(r.Context(), fileEntity.OriginalPath, nil)
	if err != nil {
		internalServerError(w, r, errxtrace.Wrap("failed to open file", err))
		return
	}
	defer reader.Close()

	// Stream the file in chunks
	if err := downloadutils.CopyFileInChunks(w, reader); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// generateExportSignedURL returns a signed URL for downloading an export file.
// @Summary Get signed URL for export download
// @Description Return a secure HMAC-signed URL for downloading a completed
// @Description export file without putting a JWT in the URL. Minting the URL
// @Description is side-effect-free, so this is a GET available to any group
// @Description member (the same audience as the export download route). The
// @Description signed URL targets the file-download route and is consumed by
// @Description the frontend export-download CTA.
// @Tags exports
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param id path string true "Export ID"
// @Success 200 {object} jsonapi.SignedFileURLResponse "Signed URL"
// @Failure 404 {object} jsonapi.Errors "Not Found"
// @Router /g/{groupSlug}/exports/{id}/signed-url [get].
func (api *exportsAPI) generateExportSignedURL(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	exp := exportFromContext(r.Context())
	if exp == nil {
		unprocessableEntityError(w, r, errors.New("export not found in context"))
		return
	}

	// Check if export is deleted. Render a JSON:API 404 (not http.NotFound's
	// plain-text body) so the frontend's JSON http wrapper parses a
	// structured error instead of throwing on a non-JSON response.
	if exp.IsDeleted() {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// Only completed exports can be downloaded
	if exp.Status != models.ExportStatusCompleted {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// Legacy FilePath-only exports have no FileEntity to sign. Completed
	// exports produced by the current export service always set FileID
	// (see go/backup/export/service.go), so a nil FileID here is an old
	// record that simply cannot be served via a signed URL.
	if exp.FileID == nil || *exp.FileID == "" {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User context required", http.StatusInternalServerError)
		return
	}

	// Load the file entity backing the export
	file, err := registrySet.FileRegistry.Get(r.Context(), *exp.FileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if file.File == nil {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// GenerateSignedURL requires a non-empty extension purely as a sanity
	// check — it never appears in the signed path, which keys on the file
	// ID. Fall back to the original path's extension, then to "xml" (export
	// artifacts are XML), so minting never 500s on a FileEntity whose Ext
	// happens to be empty.
	fileExt := strings.TrimPrefix(file.Ext, ".")
	if fileExt == "" {
		fileExt = strings.TrimPrefix(path.Ext(file.OriginalPath), ".")
	}
	if fileExt == "" {
		fileExt = "xml"
	}

	signedURL, err := api.fileSigningService.GenerateSignedURL(file.ID, fileExt, user.ID)
	if err != nil {
		internalServerError(w, r, errxtrace.Wrap("failed to generate signed URL", err))
		return
	}

	if err := render.Render(w, r, jsonapi.NewSignedFileURLResponse(file.ID, signedURL)); err != nil {
		internalServerError(w, r, errxtrace.Wrap("failed to render response", err))
		return
	}
}

// importExport imports an XML export file and creates an export record
// @Summary Import XML export
// @Description Import an uploaded XML export file and create an export record
// @Tags exports
// @Accept json-api
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param data body jsonapi.ImportExportRequest true "Import request data"
// @Success 201 {object} jsonapi.ExportResponse "Created"
// @Failure 400 {object} jsonapi.Errors "Bad Request"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Router /g/{groupSlug}/exports/import [post].
func (api *exportsAPI) importExport(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	expReg := registrySet.ExportRegistry

	var data jsonapi.ImportExportRequest
	if err := render.Bind(r, &data); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Create export record with pending status and source file path
	importedExport := models.NewImportedExport(data.Data.Attributes.Description, data.Data.Attributes.SourceFilePath)

	// Extract user from authenticated request context
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User context required", http.StatusInternalServerError)
		return
	}

	if importedExport.TenantID == "" {
		importedExport.TenantID = user.TenantID
	}

	createdExport, err := expReg.Create(r.Context(), importedExport)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Import worker will pick up this pending import and process it in background.
	// Return immediately with the created export. WithStatusCode is required
	// because ExportResponse.Render overwrites render.Status — see createExport.
	if err := render.Render(w, r, jsonapi.NewExportResponse(createdExport).WithStatusCode(http.StatusCreated)); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func (api *exportsAPI) getDownloadFile(ctx context.Context, filePath string) (io.ReadCloser, error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return nil, errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	return b.NewReader(context.Background(), filePath, nil)
}

func exportCtx() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get user-aware settings registry from context
			registrySet := RegistrySetFromContext(r.Context())
			if registrySet == nil {
				http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
				return
			}

			exportID := chi.URLParam(r, "id")
			if exportID == "" {
				next.ServeHTTP(w, r)
				return
			}

			expReg := registrySet.ExportRegistry

			exp, err := expReg.Get(r.Context(), exportID)
			if err != nil {
				renderEntityError(w, r, err)
				return
			}

			ctx := context.WithValue(r.Context(), exportCtxKey, exp)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// Exports sets up the exports API routes.
func Exports(params Params, restoreStatus RestoreStatusQuerier) func(r chi.Router) {
	api := &exportsAPI{
		uploadLocation:     params.UploadLocation,
		entityService:      params.EntityService,
		fileSigningService: services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
	}

	return func(r chi.Router) {
		r.Get("/", api.listExports)
		r.Post("/", api.createExport)
		r.Post("/import", api.importExport)

		r.Route("/{id}", func(r chi.Router) {
			r.Use(exportCtx())
			r.Get("/", api.apiGetExport)
			r.Delete("/", api.deleteExport)
			r.Get("/download", api.downloadExport)
			r.Get("/signed-url", api.generateExportSignedURL)
			r.Route("/restores", ExportRestores(restoreStatus, params.FeatureCurrencyMigration))
		})
	}
}
