package apiserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/internal/mimekit"
	"github.com/denisvmedia/inventario/internal/textutils"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/apiserver/internal/downloadutils"
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

// createFile creates a new file with upload.
// @Summary Create a file
// @Description create a new file with file upload
// @Tags files
// @Accept multipart/form-data
// @Produce json-api
// @Param metadata formData string true "JSON metadata for the file"
// @Param file formData file true "File to upload"
// @Success 201 {object} jsonapi.FileResponse "Created"
// @Failure 422 {object} jsonapi.Errors "Validation error"
// @Router /files [post].
func (api *filesAPI) createFile(w http.ResponseWriter, r *http.Request) {
	uploadedFiles := uploadedFilesFromContext(r.Context())
	if len(uploadedFiles) == 0 {
		unprocessableEntityError(w, r, ErrNoFilesUploaded)
		return
	}

	if len(uploadedFiles) > 1 {
		unprocessableEntityError(w, r, errors.New("only one file can be uploaded at a time"))
		return
	}

	// Parse metadata from form
	metadataJSON := r.FormValue("metadata")
	if metadataJSON == "" {
		unprocessableEntityError(w, r, errors.New("metadata is required"))
		return
	}

	var input jsonapi.FileUploadRequest
	if err := render.DecodeJSON(strings.NewReader(metadataJSON), &input); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	if err := input.ValidateWithContext(r.Context()); err != nil {
		unprocessableEntityError(w, r, err)
		return
	}

	uploadedFile := uploadedFiles[0]

	// Get the extension from the MIME type
	ext := mimekit.ExtensionByMime(uploadedFile.MIMEType)
	originalPath := uploadedFile.FilePath
	// Set Path to be the filename without extension
	pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

	fileEntity := models.FileEntity{
		Title:       input.Data.Attributes.Title,
		Description: input.Data.Attributes.Description,
		Type:        input.Data.Attributes.Type,
		Tags:        input.Data.Attributes.Tags,
		File: &models.File{
			Path:         pathWithoutExt,
			OriginalPath: originalPath,
			Ext:          ext,
			MIMEType:     uploadedFile.MIMEType,
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

// getFile gets a file by ID.
// @Summary Get a file
// @Description get file by ID
// @Tags files
// @Accept json-api
// @Produce json-api
// @Param id path string true "File ID"
// @Success 200 {object} jsonapi.FileResponse "OK"
// @Failure 404 {object} jsonapi.Errors "File not found"
// @Router /files/{id} [get].
func (api *filesAPI) getFile(w http.ResponseWriter, r *http.Request) {
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

	// Update the editable fields
	file.Title = input.Data.Attributes.Title
	file.Description = input.Data.Attributes.Description
	file.Type = input.Data.Attributes.Type
	file.Tags = input.Data.Attributes.Tags
	file.Path = textutils.CleanFilename(input.Data.Attributes.Path)

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
			// Log the error but don't fail the request since the database record is already deleted
			// TODO: Add proper logging
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
		// Log error but don't send response as headers are already sent
		// TODO: Add proper logging
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

// uploadFiles middleware for handling file uploads.
func (api *filesAPI) uploadFiles(allowedContentTypes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}

			err := r.ParseMultipartForm(32 << 20) // 32 MB max memory
			if err != nil {
				unprocessableEntityError(w, r, errkit.Wrap(err, "failed to parse multipart form"))
				return
			}

			var uploadedFiles []uploadedFile

			if r.MultipartForm != nil && r.MultipartForm.File != nil {
				for _, fileHeaders := range r.MultipartForm.File {
					for _, fileHeader := range fileHeaders {
						part, err := fileHeader.Open()
						if err != nil {
							unprocessableEntityError(w, r, errkit.Wrap(err, "failed to open uploaded file"))
							return
						}
						defer part.Close()

						// Generate the file path and save file
						filename := filekit.UploadFileName(fileHeader.Filename)
						mimeType, err := api.saveFile(r.Context(), filename, part, allowedContentTypes)
						switch {
						case errors.Is(err, mimekit.ErrInvalidContentType):
							unprocessableEntityError(w, r, errkit.Wrap(err, "unsupported content type"))
							return
						case err != nil:
							internalServerError(w, r, errkit.Wrap(err, "unable to save file"))
							return
						}
						uploadedFiles = append(uploadedFiles, uploadedFile{FilePath: filename, MIMEType: mimeType})
					}
				}
			}

			ctx := context.WithValue(r.Context(), uploadedFilesCtxKey, uploadedFiles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// saveFile saves an uploaded file to storage.
func (api *filesAPI) saveFile(ctx context.Context, filename string, src io.Reader, allowedContentTypes []string) (mimeType string, err error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return "", errkit.Wrap(err, "failed to open bucket")
	}
	defer func() {
		err = errors.Join(err, b.Close())
	}()

	fw, err := b.NewWriter(ctx, filename, nil)
	if err != nil {
		return "", errkit.Wrap(err, "failed to create a new writer")
	}
	defer func() {
		err = errors.Join(err, fw.Close())
	}()

	wrappedSrc := mimekit.NewMIMEReader(src, allowedContentTypes)

	_, err = io.Copy(fw, wrappedSrc)
	if err != nil {
		return "", errkit.Wrap(err, "failed when saving the file").WithField("filename", filename)
	}

	return wrappedSrc.MIMEType(), nil
}

// Files sets up the files API routes.
func Files(params Params) func(r chi.Router) {
	api := &filesAPI{
		registrySet:    params.RegistrySet,
		uploadLocation: params.UploadLocation,
	}

	return func(r chi.Router) {
		r.Get("/", api.listFiles)                                                                              // GET /files
		r.With(api.uploadFiles(mimekit.AllContentTypes()...)).Post("/", api.createFile)                       // POST /files
		r.Route("/{fileID}", func(r chi.Router) {
			r.Get("/", api.getFile)                                                                            // GET /files/123
			r.Put("/", api.updateFile)                                                                         // PUT /files/123
			r.Delete("/", api.deleteFile)                                                                      // DELETE /files/123
		})
		r.Get("/{fileID}.{fileExt}", api.downloadFile)                                                         // GET /files/123.pdf
	}
}
