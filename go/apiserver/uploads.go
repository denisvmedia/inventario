package apiserver

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/middleware"
	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/internal/mimekit"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// detectFileType auto-detects file type based on MIME type
func detectFileType(mimeType string) models.FileType {
	switch {
	case strings.HasPrefix(mimeType, "image/"):
		return models.FileTypeImage
	case strings.HasPrefix(mimeType, "video/"):
		return models.FileTypeVideo
	case strings.HasPrefix(mimeType, "audio/"):
		return models.FileTypeAudio
	case mimeType == "application/zip" || mimeType == "application/x-zip-compressed":
		return models.FileTypeArchive
	case mimeType == "application/pdf" ||
		mimeType == "text/plain" ||
		mimeType == "text/csv" ||
		strings.Contains(mimeType, "document") ||
		strings.Contains(mimeType, "spreadsheet") ||
		strings.Contains(mimeType, "presentation"):
		return models.FileTypeDocument
	default:
		return models.FileTypeOther
	}
}

type uploadedFile struct {
	// FilePath is the bucket-relative blob key the upload was written
	// to. Post-#1793 this is always tenant-prefixed
	// (`t/<tenant>/files/<basename>` or `t/<tenant>/restores/<basename>`).
	FilePath string
	// Basename is the sanitized filename from filekit.UploadFileName —
	// preserved separately so handlers can derive a human-readable
	// title from it without having to parse the tenant-prefixed key.
	Basename string
	MIMEType string
	Size     int64
}

const uploadedFilesCtxKey ctxValueKey = "uploadedFiles"

func uploadedFilesFromContext(ctx context.Context) []uploadedFile {
	uploadedFiles, ok := ctx.Value(uploadedFilesCtxKey).([]uploadedFile)
	if !ok {
		return nil
	}
	return uploadedFiles
}

type uploadsAPI struct {
	uploadLocation     string
	fileService        *services.FileService
	fileSigningService *services.FileSigningService
	factorySet         *registry.FactorySet
	thumbnailConfig    services.ThumbnailGenerationConfig
}

// @Summary Upload a file
// @Description Upload a single file of any type. The file is stored and a file entity is created without a linked entity.
// @Tags uploads
// @Accept multipart/form-data
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param file formData file true "File to upload"
// @Success 201 {object} jsonapi.FileResponse "Created"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Failure 500 {object} jsonapi.Errors "Internal Server Error"
// @Router /g/{groupSlug}/uploads/file [post]
func (api *uploadsAPI) handleFileUpload(w http.ResponseWriter, r *http.Request) {
	// Get user-aware settings registry from context
	registrySet := RegistrySetFromContext(r.Context())
	if registrySet == nil {
		http.Error(w, "Registry set not found in context", http.StatusInternalServerError)
		return
	}

	uploadedFiles := uploadedFilesFromContext(r.Context())
	if len(uploadedFiles) != 1 {
		unprocessableEntityError(w, r, ErrNoFilesUploaded)
		return
	}

	// Extract user from authenticated request context
	user := GetUserFromRequest(r)
	if user == nil {
		http.Error(w, "User context required", http.StatusInternalServerError)
		return
	}

	fileReg := registrySet.FileRegistry
	f := uploadedFiles[0] // Single file only

	// Get the extension from the MIME type
	ext := mimekit.ExtensionByMime(f.MIMEType)
	// `OriginalPath` carries the tenant-prefixed blob key
	// (`t/<tenant>/files/<basename>`) — see `uploadFiles` middleware.
	originalPath := f.FilePath
	// `Path` / Title come from the human-readable basename, NOT the
	// blob key — otherwise the user would see `t/<tenant>/files/...`
	// as the title of every uploaded file.
	pathWithoutExt := strings.TrimSuffix(f.Basename, filepath.Ext(f.Basename))

	// Auto-detect file type based on MIME type
	fileType := detectFileType(f.MIMEType)

	// Create file entity with auto-generated title from filename
	now := time.Now()
	groupID := appctx.GroupIDFromContext(r.Context())
	if groupID == "" {
		http.Error(w, "Group context required", http.StatusInternalServerError)
		return
	}

	fileEntity := models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        user.TenantID,
			GroupID:         groupID,
			CreatedByUserID: user.ID,
		},
		Title:       pathWithoutExt, // Use filename as default title
		Description: "",             // Empty description
		Type:        fileType,
		Category:    models.FileCategoryFromMIME(f.MIMEType),
		Tags:        []string{}, // Empty tags
		CreatedAt:   now,
		UpdatedAt:   now,
		File: &models.File{
			Path:         pathWithoutExt,
			OriginalPath: originalPath,
			Ext:          ext,
			MIMEType:     f.MIMEType,
			SizeBytes:    f.Size,
		},
	}

	createdFile, err := fileReg.Create(r.Context(), fileEntity)
	if err != nil {
		renderEntityError(w, r, err)
		return
	}

	// Generate thumbnail inline for image files
	api.generateThumbnailInline(r.Context(), createdFile, user.ID)

	// Generate signed URLs with thumbnails for immediate use
	binding := services.ExtractSessionBinding(r)
	originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(createdFile, user.ID, binding)
	if err != nil {
		// Log error but don't fail the upload - signed URLs are optional
		slog.Error("Failed to generate signed URLs after upload", "error", err.Error(), "file_id", createdFile.ID)
		// Return response without signed URLs
		resp := jsonapi.NewFileResponse(createdFile).WithStatusCode(http.StatusCreated)
		if err := render.Render(w, r, resp); err != nil {
			internalServerError(w, r, err)
			return
		}
		return
	}

	// Create signed URLs map
	signedUrls := map[string]jsonapi.URLData{
		createdFile.ID: {
			URL:        originalURL,
			InlineURL:  bestEffortInlineURL(api.fileSigningService, createdFile, user.ID, binding),
			Thumbnails: thumbnails,
		},
	}

	// Return response with signed URLs and thumbnails
	resp := jsonapi.NewFileResponseWithSignedUrls(createdFile, signedUrls).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// handleRestoreUpload handles signed .inb backup file upload for restore operations.
// @Summary Upload restore file
// @Description Upload a signed `.inb` backup file to be used for a restore operation.
// @Tags uploads
// @Accept multipart/form-data
// @Produce json-api
// @Param groupSlug path string true "Group slug"
// @Param file formData file true "Signed .inb backup file to upload"
// @Success 200 {object} jsonapi.UploadResponse "OK"
// @Failure 422 {object} jsonapi.Errors "Unprocessable Entity"
// @Failure 500 {object} jsonapi.Errors "Internal Server Error"
// @Router /g/{groupSlug}/uploads/restores [post]
func (api *uploadsAPI) handleRestoreUpload(w http.ResponseWriter, r *http.Request) {
	uploadedFiles := uploadedFilesFromContext(r.Context())
	if len(uploadedFiles) == 0 {
		unprocessableEntityError(w, r, ErrNoFilesUploaded)
		return
	}

	uploadData := jsonapi.UploadData{
		Type: "restores",
	}

	for _, f := range uploadedFiles {
		uploadData.FileNames = append(uploadData.FileNames, f.FilePath)
	}

	resp := jsonapi.NewUploadResponse("", uploadData).WithStatusCode(http.StatusOK)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

// uploadKind selects the per-tenant subfolder that the upload middleware
// writes to. Restore uploads land under `restores/` so the importer can
// list them separately from user-facing files; the default `file_upload`
// flow writes under `files/`.
type uploadKind int

const (
	uploadKindFile uploadKind = iota
	uploadKindRestore
)

func (api *uploadsAPI) uploadFiles(kind uploadKind, allowedContentTypes ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Resolve the authenticated tenant up-front so the blob is
			// written under the per-tenant namespace from the very first
			// byte (issue #1793). The middleware runs after JWT + tenant
			// middlewares so the user must be present; an unauthenticated
			// request here is a wiring bug, not a recoverable client error.
			user := appctx.UserFromContext(r.Context())
			if user == nil || user.TenantID == "" {
				internalServerError(w, r, errxtrace.Classify(errx.NewDisplayable("upload requires authenticated tenant context")))
				return
			}

			uploadedFiles, err := api.readUploadedFiles(r, user.TenantID, kind, allowedContentTypes)
			if err != nil {
				renderUploadError(w, r, err)
				return
			}
			ctx := context.WithValue(r.Context(), uploadedFilesCtxKey, uploadedFiles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// uploadHTTPError is the per-part error envelope returned from
// readUploadedFiles. Carries the HTTP status the handler should render
// so the middleware itself stays flat (cognitive-complexity budget).
type uploadHTTPError struct {
	status int
	err    error
}

func (e *uploadHTTPError) Error() string { return e.err.Error() }
func (e *uploadHTTPError) Unwrap() error { return e.err }

func renderUploadError(w http.ResponseWriter, r *http.Request, err error) {
	if ue, ok := errors.AsType[*uploadHTTPError](err); ok {
		switch ue.status {
		case http.StatusUnprocessableEntity:
			unprocessableEntityError(w, r, ue.err)
		default:
			internalServerError(w, r, ue.err)
		}
		return
	}
	internalServerError(w, r, err)
}

// readUploadedFiles walks the request's multipart stream and writes
// each part to the bucket under the tenant-prefixed namespace. Returns
// the parsed-out uploadedFile list (one entry per accepted part) or
// an uploadHTTPError carrying the status the caller should render.
func (api *uploadsAPI) readUploadedFiles(r *http.Request, tenantID string, kind uploadKind, allowedContentTypes []string) ([]uploadedFile, error) {
	reader, err := r.MultipartReader()
	if err != nil {
		return nil, &uploadHTTPError{status: http.StatusUnprocessableEntity, err: err}
	}

	var uploadedFiles []uploadedFile
	fileCount := 0
	for {
		part, err := reader.NextPart()
		switch err {
		case nil:
			// fallthrough to body
		case io.EOF:
			return uploadedFiles, nil
		default:
			return nil, errxtrace.Wrap("unable to read part in multipart form", err)
		}

		rawName := part.FileName()
		if rawName == "" {
			continue
		}

		fileCount++
		if fileCount > 1 {
			return nil, &uploadHTTPError{
				status: http.StatusUnprocessableEntity,
				err:    errxtrace.Classify(errx.NewDisplayable("only single file uploads are allowed")),
			}
		}

		basename := filekit.UploadFileName(rawName)
		uf, err := api.saveOnePart(r.Context(), part, basename, tenantID, kind, allowedContentTypes)
		if err != nil {
			return nil, err
		}
		uploadedFiles = append(uploadedFiles, uf)
	}
}

// saveOnePart computes the tenant-prefixed blob key from the
// caller-supplied (already-sanitised) basename and writes the part
// bytes to the bucket. Returns either a populated uploadedFile or an
// error (wrapped in uploadHTTPError for the unprocessable-entity-bound
// MIME mismatch).
func (api *uploadsAPI) saveOnePart(ctx context.Context, part io.Reader, basename, tenantID string, kind uploadKind, allowedContentTypes []string) (uploadedFile, error) {
	var blobKey string
	switch kind {
	case uploadKindRestore:
		blobKey = blobkeys.BuildRestoreUploadKey(tenantID, basename)
	default:
		blobKey = blobkeys.BuildFileUploadKey(tenantID, basename)
	}
	mimeType, size, err := api.saveFile(ctx, blobKey, part, allowedContentTypes)
	switch {
	case errors.Is(err, mimekit.ErrInvalidContentType):
		return uploadedFile{}, &uploadHTTPError{
			status: http.StatusUnprocessableEntity,
			err:    errxtrace.Wrap("unsupported content type", err),
		}
	case err != nil:
		return uploadedFile{}, errxtrace.Wrap("unable to save file", err)
	}
	return uploadedFile{FilePath: blobKey, Basename: basename, MIMEType: mimeType, Size: size}, nil
}

func (api *uploadsAPI) saveFile(ctx context.Context, filename string, src io.Reader, allowedContentTypes []string) (mimeType string, size int64, err error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return "", 0, errxtrace.Wrap("failed to open bucket", err) // TODO: we might want adding uploadLocation as a field, but it may contain sensitive data
	}
	defer func() {
		err = errors.Join(err, b.Close())
	}()

	fw, err := b.NewWriter(ctx, filename, nil)
	if err != nil {
		return "", 0, errxtrace.Wrap("failed to create a new writer", err)
	}
	defer func() {
		err = errors.Join(err, fw.Close())
	}()

	wrappedSrc := mimekit.NewMIMEReader(src, allowedContentTypes)

	written, err := io.Copy(fw, wrappedSrc)
	if err != nil {
		return "", 0, errxtrace.Wrap("failed when saving the file", err, errx.Attrs("filename", filename))
	}

	return wrappedSrc.MIMEType(), written, nil
}

func Uploads(params Params) func(r chi.Router) {
	api := &uploadsAPI{
		uploadLocation:     params.UploadLocation,
		fileService:        services.NewFileService(params.FactorySet, params.UploadLocation),
		fileSigningService: services.NewFileSigningService(params.FileSigningKey, params.FileURLExpiration),
		factorySet:         params.FactorySet,
		thumbnailConfig:    params.ThumbnailConfig,
	}

	// Create concurrent upload service for upload limiting
	config := services.LoadConcurrentUploadConfig()
	concurrentUploadService := services.NewConcurrentUploadService(config)
	uploadLimiter := middleware.UploadLimiter(concurrentUploadService)

	return func(r chi.Router) {
		// Legacy commodity-/location-scoped upload routes
		// (`/uploads/{commodities,locations}/{id}/{image,manual,invoice,file}`)
		// were removed under #1421. Clients now POST a multipart file to
		// `/uploads/file` (creates an unlinked FileEntity) and then
		// PUT `/files/{id}` with `linked_entity_type` / `linked_entity_id`
		// set in the JSON:API attributes to attach the row to a
		// commodity / location. The unified `/uploads/file` handler
		// itself does NOT read linked-entity fields off the multipart
		// form — see `handleFileUpload` below.

		// Single file upload - allow all content types with concurrent upload limiting
		fileMiddlewares := []func(http.Handler) http.Handler{
			middleware.SetUploadOperation("file_upload"),
			uploadLimiter,
			api.uploadFiles(uploadKindFile, mimekit.AllContentTypes()...),
		}
		r.With(fileMiddlewares...).Post("/file", api.handleFileUpload)

		// Restore uploads - only allow signed .inb backup archives (#534); no
		// upload limiting for system operations.
		r.With(api.uploadFiles(uploadKindRestore, mimekit.INBContentTypes()...)).Post("/restores", api.handleRestoreUpload)
	}
}

// generateThumbnailInline generates thumbnails inline during upload for image files
func (api *uploadsAPI) generateThumbnailInline(ctx context.Context, file *models.FileEntity, userID string) {
	// Only generate for supported image types
	if !mimekit.IsImage(file.MIMEType) {
		return // Not an image, skip thumbnail generation
	}

	// Only support JPEG and PNG for thumbnail generation
	if !strings.HasPrefix(file.MIMEType, "image/jpeg") && !strings.HasPrefix(file.MIMEType, "image/png") {
		return // Skip unsupported image formats
	}

	// Generate thumbnail inline using file service directly
	err := api.fileService.GenerateThumbnails(ctx, file)
	if err != nil {
		// Log error but don't fail the upload - thumbnails are optional
		slog.Error("Failed to generate thumbnail inline",
			"error", err.Error(),
			"original_path", file.OriginalPath,
			"mime_type", file.MIMEType,
			"file_id", file.ID,
			"user_id", userID)
	} else {
		slog.Info("Thumbnail generated inline successfully",
			"original_path", file.OriginalPath,
			"mime_type", file.MIMEType,
			"file_id", file.ID)
	}
}
