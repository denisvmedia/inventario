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
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/apiserver/middleware"
	"github.com/denisvmedia/inventario/internal/errkit"
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
	FilePath string
	MIMEType string
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

func (api *uploadsAPI) handleImageUpload(w http.ResponseWriter, r *http.Request) {
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

	entityID := entityIDFromContext(r.Context())
	if entityID == "" {
		unprocessableEntityError(w, r, ErrEntityNotFound)
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
	originalPath := f.FilePath
	// Set Path to be the filename without extension
	pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

	// Create file entity instead of image
	now := time.Now()
	fileEntity := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		Title:            pathWithoutExt, // Use filename as title
		Description:      "",
		Type:             models.FileTypeImage,
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   entityID,
		LinkedEntityMeta: "images",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         pathWithoutExt,
			OriginalPath: originalPath,
			Ext:          ext,
			MIMEType:     f.MIMEType,
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
	originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(createdFile, user.ID)
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

func (api *uploadsAPI) handleManualUpload(w http.ResponseWriter, r *http.Request) {
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

	entityID := entityIDFromContext(r.Context())
	if entityID == "" {
		unprocessableEntityError(w, r, ErrEntityNotFound)
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
	originalPath := f.FilePath
	// Set Path to be the filename without extension
	pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

	// Auto-detect file type based on MIME type
	fileType := detectFileType(f.MIMEType)

	// Create file entity instead of manual
	now := time.Now()
	fileEntity := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		Title:            pathWithoutExt, // Use filename as title
		Description:      "",
		Type:             fileType, // Use auto-detected type
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   entityID,
		LinkedEntityMeta: "manuals",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         pathWithoutExt,
			OriginalPath: originalPath,
			Ext:          ext,
			MIMEType:     f.MIMEType,
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
	originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(createdFile, user.ID)
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

func (api *uploadsAPI) handleInvoiceUpload(w http.ResponseWriter, r *http.Request) {
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

	entityID := entityIDFromContext(r.Context())
	if entityID == "" {
		unprocessableEntityError(w, r, ErrEntityNotFound)
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
	originalPath := f.FilePath
	// Set Path to be the filename without extension
	pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

	// Auto-detect file type based on MIME type
	fileType := detectFileType(f.MIMEType)

	// Create file entity instead of invoice
	now := time.Now()
	fileEntity := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		Title:            pathWithoutExt, // Use filename as title
		Description:      "",
		Type:             fileType, // Use auto-detected type
		Tags:             []string{},
		LinkedEntityType: "commodity",
		LinkedEntityID:   entityID,
		LinkedEntityMeta: "invoices",
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         pathWithoutExt,
			OriginalPath: originalPath,
			Ext:          ext,
			MIMEType:     f.MIMEType,
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
	originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(createdFile, user.ID)
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
	originalPath := f.FilePath
	// Set Path to be the filename without extension
	pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

	// Auto-detect file type based on MIME type
	fileType := detectFileType(f.MIMEType)

	// Create file entity with auto-generated title from filename
	now := time.Now()
	fileEntity := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
			UserID:   user.ID,
		},
		Title:       pathWithoutExt, // Use filename as default title
		Description: "",             // Empty description
		Type:        fileType,
		Tags:        []string{}, // Empty tags
		CreatedAt:   now,
		UpdatedAt:   now,
		File: &models.File{
			Path:         pathWithoutExt,
			OriginalPath: originalPath,
			Ext:          ext,
			MIMEType:     f.MIMEType,
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
	originalURL, thumbnails, err := api.fileSigningService.GenerateSignedURLsWithThumbnails(createdFile, user.ID)
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

func (api *uploadsAPI) uploadFiles(allowedContentTypes ...string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Get the multipart reader from the request
			reader, err := r.MultipartReader()
			if err != nil {
				unprocessableEntityError(w, r, err)
				return
			}

			var uploadedFiles []uploadedFile
			fileCount := 0

		loop:
			for {
				// Read the next part (file) in the multipart stream
				part, err := reader.NextPart()
				switch err {
				case nil:
				case io.EOF:
					break loop
				default:
					internalServerError(w, r, errkit.Wrap(err, "unable to read part in multipart form"))
					return
				}

				// Skip if it's not a file part
				if part.FileName() == "" {
					continue
				}

				fileCount++
				// Only allow single file uploads
				if fileCount > 1 {
					unprocessableEntityError(w, r, errkit.WithStack(errors.New("only single file uploads are allowed")))
					return
				}

				// Generate the file path and open a new file
				filename := filekit.UploadFileName(part.FileName())
				mimeType, err := api.saveFile(r.Context(), filename, part, allowedContentTypes) // TODO: make sure that the file is not too big
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

			ctx := context.WithValue(r.Context(), uploadedFilesCtxKey, uploadedFiles)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (api *uploadsAPI) saveFile(ctx context.Context, filename string, src io.Reader, allowedContentTypes []string) (mimeType string, err error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return "", errkit.Wrap(err, "failed to open bucket") // TODO: we might want adding uploadLocation as a field, but it may contain sensitive data
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
		r.With(commodityCtx()).
			Route("/commodities/{commodityID}", func(r chi.Router) {
				// Single image upload with concurrent upload limiting
				imageMiddlewares := []func(http.Handler) http.Handler{
					middleware.SetUploadOperation("image_upload"),
					uploadLimiter,
					api.uploadFiles(mimekit.ImageContentTypes()...),
				}
				r.With(imageMiddlewares...).Post("/image", api.handleImageUpload)

				// Single manual upload with concurrent upload limiting
				manualMiddlewares := []func(http.Handler) http.Handler{
					middleware.SetUploadOperation("document_upload"),
					uploadLimiter,
					api.uploadFiles(mimekit.DocContentTypes()...),
				}
				r.With(manualMiddlewares...).Post("/manual", api.handleManualUpload)

				// Single invoice upload with concurrent upload limiting
				invoiceMiddlewares := []func(http.Handler) http.Handler{
					middleware.SetUploadOperation("document_upload"),
					uploadLimiter,
					api.uploadFiles(mimekit.DocContentTypes()...),
				}
				r.With(invoiceMiddlewares...).Post("/invoice", api.handleInvoiceUpload)
			})

		// Single file upload - allow all content types with concurrent upload limiting
		fileMiddlewares := []func(http.Handler) http.Handler{
			middleware.SetUploadOperation("file_upload"),
			uploadLimiter,
			api.uploadFiles(mimekit.AllContentTypes()...),
		}
		r.With(fileMiddlewares...).Post("/file", api.handleFileUpload)

		// Restore uploads - only allow XML files (no upload limiting for system operations)
		r.With(api.uploadFiles(mimekit.XMLContentTypes()...)).Post("/restores", api.handleRestoreUpload)
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
