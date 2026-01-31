package apiserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/assets"
	"github.com/denisvmedia/inventario/internal/textutils"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

type filesAPI struct {
	uploadLocation     string
	fileService        *services.FileService
	fileSigningService *services.FileSigningService
	factorySet         *registry.FactorySet
	thumbnailConfig    services.ThumbnailGenerationConfig
}

// listFiles lists all files with optional filtering and pagination.
// @Summary List files
// @Description get files with optional filtering
// @Tags files
// @Accept json-api
// @Produce json-api
// @Param type query string false "Filter by file type" Enums(image,document,video,audio,archive,other)
// @Param search query string false "Search in title, description, and file paths"
// @Param tags query string false "Filter by tags (comma-separated)"
// @Param page query int false "Page number (1-based)" default(1)
// @Param limit query int false "Items per page" default(20)
// @Success 200 {object} jsonapi.FilesResponse "OK"
// @Router /files [get].
func (api *filesAPI) listFiles(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	// Parse query parameters
	typeParam := r.URL.Query().Get("type")
	searchParam := r.URL.Query().Get("search")
	tagsParam := r.URL.Query().Get("tags")
	pageParam := r.URL.Query().Get("page")
	limitParam := r.URL.Query().Get("limit")

	// Parse pagination
	page := 1
	if pageParam != "" {
		if p, err := strconv.Atoi(pageParam); err == nil && p > 0 {
			page = p
		}
	}

	limit := 20
	if limitParam != "" {
		if l, err := strconv.Atoi(limitParam); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	offset := (page - 1) * limit

	var fileType *models.FileType
	if typeParam != "" {
		ft := models.FileType(typeParam)
		fileType = &ft
	}

	var tags []string
	if tagsParam != "" {
		tags = strings.Split(tagsParam, ",")
		for i, tag := range tags {
			tags[i] = strings.TrimSpace(tag)
		}
	}

	var files []*models.FileEntity
	var total int
	var err error

	if searchParam != "" || len(tags) > 0 {
		// Use search if search query or tags are provided
		files, err = fileReg.Search(r.Context(), searchParam, fileType, tags)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
		total = len(files)

		// Apply pagination manually for search results
		start := offset
		if start > total {
			start = total
		}
		end := start + limit
		if end > total {
			end = total
		}
		files = files[start:end]
	} else {
		// Use paginated list for simple queries
		files, total, err = fileReg.ListPaginated(r.Context(), offset, limit, fileType)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
	}

	// Generate signed URLs for all files
	signedUrls := api.generateSignedURLsForFiles(r.Context(), files)

	response := jsonapi.NewFilesResponseWithSignedUrls(files, total, signedUrls)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// generateSignedURLsForFiles generates signed URLs for a list of files.
// Returns a map of file ID to URLData with signed URLs and thumbnails. Missing URLs indicate generation failures.
func (api *filesAPI) generateSignedURLsForFiles(ctx context.Context, files []*models.FileEntity) map[string]jsonapi.URLData {
	signedUrls := make(map[string]jsonapi.URLData)
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return signedUrls
	}

	for _, file := range files {
		// Generate signed URLs for file and thumbnails
		originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(file, user.ID)
		if err != nil {
			// Log error but don't fail the entire request
			// The frontend can handle missing URLs gracefully
			continue
		}

		signedUrls[file.ID] = jsonapi.URLData{
			URL:        originalURL,
			Thumbnails: thumbnails,
		}
	}

	return signedUrls
}

// createFile creates a new file entity (metadata only, file must be uploaded via /uploads/files first).
// @Summary Create a file entity
// @Description create a new file entity with metadata (file upload handled separately)
// @Tags files
// @Accept json-api
// @Produce json-api
// @Param file body jsonapi.FileRequest true "File metadata"
// @Success 201 {object} jsonapi.FileResponse "Created"
// @Failure 422 {object} jsonapi.Errors "Validation error"
// @Router /files [post].
func (api *filesAPI) createFile(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	var input jsonapi.FileRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	// Extract user from authenticated request context
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User context required", http.StatusInternalServerError)
		return
	}

	now := time.Now()
	fileEntity := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		Title:            input.Data.Attributes.Title,
		Description:      input.Data.Attributes.Description,
		Type:             models.FileTypeOther, // Default type, should be updated when file is uploaded
		Tags:             input.Data.Attributes.Tags,
		LinkedEntityType: input.Data.Attributes.LinkedEntityType,
		LinkedEntityID:   input.Data.Attributes.LinkedEntityID,
		LinkedEntityMeta: input.Data.Attributes.LinkedEntityMeta,
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         input.Data.Attributes.Path,
			OriginalPath: input.Data.Attributes.Path,
			Ext:          "",
			MIMEType:     "",
		},
	}

	createdFile, err := fileReg.Create(r.Context(), fileEntity)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	response := jsonapi.NewFileResponse(createdFile).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// apiGetFile gets a file by ID.
// @Summary Get a file
// @Description get file by ID
// @Tags files
// @Accept json-api
// @Produce json-api
// @Param id path string true "File ID"
// @Success 200 {object} jsonapi.FileResponse "OK"
// @Failure 404 {object} jsonapi.Errors "File not found"
// @Router /files/{id} [get].
func (api *filesAPI) apiGetFile(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	fileID := chi.URLParam(r, "fileID")

	file, err := fileReg.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Generate signed URLs for the file
	signedUrls := api.generateSignedURLsForFiles(r.Context(), []*models.FileEntity{file})

	response := jsonapi.NewFileResponseWithSignedUrls(file, signedUrls)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// updateFile updates a file's metadata.
// @Summary Update a file
// @Description update file metadata
// @Tags files
// @Accept json-api
// @Produce json-api
// @Param id path string true "File ID"
// @Param file body jsonapi.FileUpdateRequest true "File update data"
// @Success 200 {object} jsonapi.FileResponse "OK"
// @Failure 404 {object} jsonapi.Errors "File not found"
// @Failure 422 {object} jsonapi.Errors "Validation error"
// @Router /files/{id} [put].
func (api *filesAPI) updateFile(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry

	fileID := chi.URLParam(r, "fileID")

	var input jsonapi.FileUpdateRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if fileID != input.Data.ID {
		unprocessableEntityError(w, r, errors.New("ID in URL does not match ID in request body"))
		return
	}

	file, err := registrySet.FileRegistry.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Check if this is an export file and prevent changing entity linking
	if file.LinkedEntityType == "export" {
		// For export files, only allow updating title, description, tags, and path
		// Entity linking fields cannot be changed
		if input.Data.Attributes.LinkedEntityType != file.LinkedEntityType ||
			input.Data.Attributes.LinkedEntityID != file.LinkedEntityID ||
			input.Data.Attributes.LinkedEntityMeta != file.LinkedEntityMeta {
			err := errors.New("export file entity linking cannot be changed")
			unprocessableEntityError(w, r, err)
			return
		}
	}

	// Update the editable fields (file type is auto-detected from MIME type and cannot be changed manually)
	file.Title = input.Data.Attributes.Title
	file.Description = input.Data.Attributes.Description
	file.Tags = input.Data.Attributes.Tags
	file.Path = textutils.CleanFilename(input.Data.Attributes.Path)

	// Only update entity linking for non-export files or if values haven't changed
	if file.LinkedEntityType != "export" {
		file.LinkedEntityType = input.Data.Attributes.LinkedEntityType
		file.LinkedEntityID = input.Data.Attributes.LinkedEntityID
		file.LinkedEntityMeta = input.Data.Attributes.LinkedEntityMeta
	}

	file.UpdatedAt = time.Now() // Set updated timestamp

	// Auto-detect file type from MIME type if available
	if file.File != nil && file.MIMEType != "" {
		file.Type = models.FileTypeFromMIME(file.MIMEType)
	}

	updatedFile, err := fileReg.Update(r.Context(), *file)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	response := jsonapi.NewFileResponse(updatedFile)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// deleteFile deletes a file.
// @Summary Delete a file
// @Description delete file and its associated file
// @Tags files
// @Accept json-api
// @Produce json-api
// @Param id path string true "File ID"
// @Success 204 "No Content"
// @Failure 404 {object} jsonapi.Errors "File not found"
// @Router /files/{id} [delete].
func (api *filesAPI) deleteFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")

	// Use file service to delete both physical file and database record
	err := api.fileService.DeleteFileWithPhysical(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// generateSignedURL generates a signed URL for file download.
// @Summary Generate signed URL for file download
// @Description Generate a secure signed URL for downloading a file
// @Tags files
// @Param id path string true "File ID"
// @Success 200 {object} jsonapi.SignedFileURLResponse "Signed URL"
// @Failure 404 {object} jsonapi.Errors "File not found"
// @Router /files/{id}/signed-url [post].
func (api *filesAPI) generateSignedURL(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	// Get user from context
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User not found in context", http.StatusUnauthorized)
		return
	}

	fileReg := registrySet.FileRegistry
	fileID := chi.URLParam(r, "fileID")

	// Get the file to validate it exists and user has access
	file, err := fileReg.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if file.File == nil {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// Generate signed URLs for file and thumbnails
	signedURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(file, user.ID)
	if err != nil {
		internalServerError(w, r, stacktrace.Wrap("failed to generate signed URL", err))
		return
	}

	// Return the signed URL using JSON:API format with thumbnails
	response := jsonapi.NewSignedFileURLResponseWithThumbnails(fileID, signedURL, thumbnails)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, stacktrace.Wrap("failed to render response", err))
		return
	}
}

// Files sets up the files API routes.
func Files(params Params) func(r chi.Router) {
	fileSigningService := services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration)
	api := &filesAPI{
		uploadLocation:     params.UploadLocation,
		fileService:        services.NewFileService(params.FactorySet, params.UploadLocation),
		fileSigningService: fileSigningService,
		factorySet:         params.FactorySet,
		thumbnailConfig:    params.ThumbnailConfig,
	}

	return func(r chi.Router) {
		r.Get("/", api.listFiles)   // GET /files
		r.Post("/", api.createFile) // POST /files
		r.Route("/{fileID}", func(r chi.Router) {
			r.Get("/", api.apiGetFile)                   // GET /files/123
			r.Put("/", api.updateFile)                   // PUT /files/123
			r.Delete("/", api.deleteFile)                // DELETE /files/123
			r.Post("/signed-url", api.generateSignedURL) // POST /files/123/signed-url
		})
		// File downloads moved to signed URL routes for security
	}
}

// downloadOriginalFile downloads an original file using the file entity's stored metadata
func (api *filesAPI) downloadOriginalFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	if fileID == "" {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// Get the file entity to access stored metadata
	fileReg, err := api.factorySet.FileRegistryFactory.CreateUserRegistry(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	file, err := fileReg.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Use the stored MIME type and original path
	filePath := file.OriginalPath
	mimeType := file.MIMEType
	if mimeType == "" {
		// Fallback to extension-based detection only if MIME type is not stored
		ext := filepath.Ext(filePath)
		mimeType = mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
	}

	api.streamFileFromStorage(w, r, filePath, mimeType, file.Path+file.Ext)
}

// downloadThumbnail downloads a thumbnail file (always JPEG) with deferred generation support
func (api *filesAPI) downloadThumbnail(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	size := chi.URLParam(r, "size")

	if fileID == "" || size == "" {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// Validate size parameter
	if size != "small" && size != "medium" {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// Generate thumbnail path using the new structure
	thumbnailPath := fmt.Sprintf("thumbnails/%s_%s.jpg", fileID, size)

	// Check if thumbnail exists using file service
	exists, err := api.fileService.ThumbnailExists(r.Context(), fileID, size)
	if err != nil {
		internalServerError(w, r, stacktrace.Wrap("failed to check thumbnail existence", err))
		return
	}

	if !exists {
		// Thumbnail doesn't exist - serve placeholder and trigger generation
		api.servePlaceholderThumbnail(w, r, fileID, size)
		return
	}

	// All thumbnails are JPEG files
	mimeType := "image/jpeg"
	filename := fmt.Sprintf("%s_%s.jpg", fileID, size)

	api.streamFileFromStorage(w, r, thumbnailPath, mimeType, filename)
}

// servePlaceholderThumbnail serves a placeholder image while triggering thumbnail generation
func (api *filesAPI) servePlaceholderThumbnail(w http.ResponseWriter, r *http.Request, fileID, size string) {
	// Check if there's a thumbnail generation job for this file
	thumbnailService := services.NewThumbnailGenerationService(api.factorySet, api.uploadLocation, api.thumbnailConfig)

	job, err := thumbnailService.GetJobByFileID(r.Context(), fileID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		internalServerError(w, r, stacktrace.Wrap("failed to check thumbnail generation status", err))
		return
	}

	// If no job exists or job failed, request thumbnail generation
	if job == nil || job.Status == models.ThumbnailStatusFailed {
		_, err := thumbnailService.RequestThumbnailGeneration(r.Context(), fileID)
		if err != nil {
			// Log error but still serve placeholder
			slog.Error("Failed to request thumbnail generation", "error", err, "file_id", fileID)
		}
	}

	// Serve the placeholder image
	api.servePlaceholderImage(w, r, size)
}

// servePlaceholderImage serves a static placeholder image from embedded assets
func (api *filesAPI) servePlaceholderImage(w http.ResponseWriter, r *http.Request, size string) {
	// Set appropriate headers
	w.Header().Set("Content-Type", "image/gif")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Get placeholder from embedded assets
	filename := fmt.Sprintf("generating_%s.gif", size)
	data, err := assets.GetPlaceholderFile(filename)
	if err != nil {
		slog.Error("Failed to load placeholder image", "filename", filename, "error", err)
		http.Error(w, "Placeholder not found", http.StatusNotFound)
		return
	}

	// Set content length
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))

	// Write the image data
	if _, err := w.Write(data); err != nil {
		slog.Error("Failed to write placeholder image", "filename", filename, "error", err)
	}
}

// streamFileFromStorage is a helper function to stream files from storage
func (api *filesAPI) streamFileFromStorage(w http.ResponseWriter, r *http.Request, filePath, mimeType, filename string) {
	// Open the file from storage
	b, err := blob.OpenBucket(r.Context(), api.uploadLocation)
	if err != nil {
		internalServerError(w, r, stacktrace.Wrap("failed to open bucket", err))
		return
	}
	defer b.Close()

	// Check if file exists
	exists, err := b.Exists(r.Context(), filePath)
	if err != nil {
		internalServerError(w, r, stacktrace.Wrap("failed to check file existence", err))
		return
	}
	if !exists {
		renderEntityError(w, r, ErrNotFound)
		return
	}

	// Get file attributes for size
	attrs, err := b.Attributes(r.Context(), filePath)
	if err != nil {
		internalServerError(w, r, stacktrace.Wrap("failed to get file attributes", err))
		return
	}

	// Open file reader
	reader, err := b.NewReader(r.Context(), filePath, nil)
	if err != nil {
		internalServerError(w, r, stacktrace.Wrap("failed to open file reader", err))
		return
	}
	defer reader.Close()

	// Set headers and stream the file
	downloadutils.SetStreamingHeaders(w, mimeType, attrs.Size, filename)
	if _, err := io.Copy(w, reader); err != nil {
		// Log error but don't send response as headers are already sent
		slog.Error("Failed to stream file", "error", err, "file_path", filePath)
	}
}

// SignedFiles sets up the signed file download routes that use signed URL validation
func SignedFiles(params Params) func(r chi.Router) {
	api := &filesAPI{
		uploadLocation:  params.UploadLocation,
		fileService:     services.NewFileService(params.FactorySet, params.UploadLocation),
		factorySet:      params.FactorySet,
		thumbnailConfig: params.ThumbnailConfig,
	}

	return func(r chi.Router) {
		// Separate routes for original files vs thumbnails
		r.Get("/files/{fileID}", api.downloadOriginalFile)          // GET /files/download/files/123 (original file)
		r.Get("/thumbnails/{fileID}/{size}", api.downloadThumbnail) // GET /files/download/thumbnails/123/medium (thumbnail)
	}
}
