package apiserver

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/textutils"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type filesAPI struct {
	registrySet    *registry.Set
	uploadLocation string
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
		files, err = api.registrySet.FileRegistry.Search(r.Context(), searchParam, fileType, tags)
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
		files, total, err = api.registrySet.FileRegistry.ListPaginated(r.Context(), offset, limit, fileType)
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
	}

	response := jsonapi.NewFilesResponse(files, total)
	if err := render.Render(w, r, response); err != nil {
		internalServerError(w, r, err)
		return
	}
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
	var input jsonapi.FileRequest
	if err := render.Bind(r, &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	now := time.Now()
	fileEntity := models.FileEntity{
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

	createdFile, err := api.registrySet.FileRegistry.Create(r.Context(), fileEntity)
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
	fileID := chi.URLParam(r, "fileID")

	file, err := api.registrySet.FileRegistry.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	response := jsonapi.NewFileResponse(file)
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

	file, err := api.registrySet.FileRegistry.Get(r.Context(), fileID)
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

	updatedFile, err := api.registrySet.FileRegistry.Update(r.Context(), *file)
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

	// Get the file first to get the file path for deletion
	file, err := api.registrySet.FileRegistry.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Delete from database first
	err = api.registrySet.FileRegistry.Delete(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Delete the physical file
	if file.File != nil && file.OriginalPath != "" {
		if err := api.deletePhysicalFile(r.Context(), file.OriginalPath); err != nil {
			renderEntityError(w, r, err)
			return
		}
	}

	w.WriteHeader(http.StatusNoContent)
}

// downloadFile downloads a file.
// @Summary Download a file
// @Description download file content
// @Tags files
// @Param id path string true "File ID"
// @Param ext path string true "File extension"
// @Success 200 "File content"
// @Failure 404 {object} jsonapi.Errors "File not found"
// @Router /files/{id}.{ext} [get].
func (api *filesAPI) downloadFile(w http.ResponseWriter, r *http.Request) {
	fileID := chi.URLParam(r, "fileID")
	ext := chi.URLParam(r, "fileExt")

	file, err := api.registrySet.FileRegistry.Get(r.Context(), fileID)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	if file.File == nil {
		renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	// Validate extension matches
	expectedExt := strings.TrimPrefix(file.Ext, ".")
	if ext != expectedExt {
		renderEntityError(w, r, registry.ErrNotFound)
		return
	}

	// Get file attributes for Content-Length header
	attrs, err := downloadutils.GetFileAttributes(r.Context(), api.uploadLocation, file.OriginalPath)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	// Set streaming headers
	filename := file.Path + file.Ext
	downloadutils.SetStreamingHeaders(w, file.MIMEType, attrs.Size, filename)

	// Open and stream the file
	b, err := blob.OpenBucket(r.Context(), api.uploadLocation)
	if err != nil {
		internalServerError(w, r, errkit.Wrap(err, "failed to open bucket"))
		return
	}
	defer b.Close()

	reader, err := b.NewReader(r.Context(), file.OriginalPath, nil)
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

// deletePhysicalFile deletes the physical file from storage.
func (api *filesAPI) deletePhysicalFile(ctx context.Context, filePath string) error {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open bucket")
	}
	defer b.Close()

	err = b.Delete(ctx, filePath)
	if err != nil {
		return errkit.Wrap(err, "failed to delete file")
	}

	return nil
}

// Files sets up the files API routes.
func Files(params Params) func(r chi.Router) {
	api := &filesAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listFiles)   // GET /files
		r.Post("/", api.createFile) // POST /files
		r.Route("/{fileID}", func(r chi.Router) {
			r.Get("/", api.apiGetFile)    // GET /files/123
			r.Put("/", api.updateFile)    // PUT /files/123
			r.Delete("/", api.deleteFile) // DELETE /files/123
		})
		r.Get("/{fileID}.{fileExt}", api.downloadFile) // GET /files/123.pdf
	}
}
