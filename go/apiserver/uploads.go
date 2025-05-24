package apiserver

import (
	"context"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/internal/mimekit"
	"github.com/denisvmedia/inventario/jsonapi"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

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
	uploadLocation  string
	imageRegistry   registry.ImageRegistry
	manualRegistry  registry.ManualRegistry
	invoiceRegistry registry.InvoiceRegistry
}

func (api *uploadsAPI) handleImagesUpload(w http.ResponseWriter, r *http.Request) {
	uploadedFiles := uploadedFilesFromContext(r.Context())
	if len(uploadedFiles) == 0 {
		unprocessableEntityError(w, r, ErrNoFilesUploaded)
		return
	}

	entityID := entityIDFromContext(r.Context())
	if entityID == "" {
		unprocessableEntityError(w, r, ErrEntityNotFound)
		return
	}

	uploadData := jsonapi.UploadData{
		Type: "images",
	}

	for _, f := range uploadedFiles {
		// Get the extension from the MIME type
		ext := mimekit.ExtensionByMime(f.MIMEType)
		originalPath := f.FilePath
		// Set Path to be the filename without extension
		pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

		img, err := api.imageRegistry.Create(r.Context(), models.Image{
			CommodityID: entityID,
			File: &models.File{
				Path:         pathWithoutExt, // Just the filename without extension
				OriginalPath: originalPath,
				Ext:          ext,
				MIMEType:     f.MIMEType,
			},
		})
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
		uploadData.FileNames = append(uploadData.FileNames, img.Path)
	}

	resp := jsonapi.NewUploadResponse(entityID, uploadData).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func (api *uploadsAPI) handleManualsUpload(w http.ResponseWriter, r *http.Request) {
	uploadedFiles := uploadedFilesFromContext(r.Context())
	if len(uploadedFiles) == 0 {
		unprocessableEntityError(w, r, ErrNoFilesUploaded)
		return
	}

	entityID := entityIDFromContext(r.Context())
	if entityID == "" {
		unprocessableEntityError(w, r, ErrEntityNotFound)
		return
	}

	uploadData := jsonapi.UploadData{
		Type: "manuals",
	}

	for _, f := range uploadedFiles {
		// Get the extension from the MIME type
		ext := mimekit.ExtensionByMime(f.MIMEType)
		originalPath := f.FilePath
		// Set Path to be the filename without extension
		pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

		img, err := api.manualRegistry.Create(r.Context(), models.Manual{
			CommodityID: entityID,
			File: &models.File{
				Path:         pathWithoutExt, // Just the filename without extension
				OriginalPath: originalPath,
				Ext:          ext,
				MIMEType:     f.MIMEType,
			},
		})
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
		uploadData.FileNames = append(uploadData.FileNames, img.Path)
	}

	resp := jsonapi.NewUploadResponse(entityID, uploadData).WithStatusCode(http.StatusCreated)
	if err := render.Render(w, r, resp); err != nil {
		internalServerError(w, r, err)
		return
	}
}

func (api *uploadsAPI) handleInvoicesUpload(w http.ResponseWriter, r *http.Request) {
	uploadedFiles := uploadedFilesFromContext(r.Context())
	if len(uploadedFiles) == 0 {
		unprocessableEntityError(w, r, ErrNoFilesUploaded)
		return
	}

	entityID := entityIDFromContext(r.Context())
	if entityID == "" {
		unprocessableEntityError(w, r, ErrEntityNotFound)
		return
	}

	uploadData := jsonapi.UploadData{
		Type: "invoices",
	}

	for _, f := range uploadedFiles {
		// Get the extension from the MIME type
		ext := mimekit.ExtensionByMime(f.MIMEType)
		originalPath := f.FilePath
		// Set Path to be the filename without extension
		pathWithoutExt := strings.TrimSuffix(originalPath, filepath.Ext(originalPath))

		img, err := api.invoiceRegistry.Create(r.Context(), models.Invoice{
			CommodityID: entityID,
			File: &models.File{
				Path:         pathWithoutExt, // Just the filename without extension
				OriginalPath: originalPath,
				Ext:          ext,
				MIMEType:     f.MIMEType,
			},
		})
		if err != nil {
			renderEntityError(w, r, err)
			return
		}
		uploadData.FileNames = append(uploadData.FileNames, img.Path)
	}

	resp := jsonapi.NewUploadResponse(entityID, uploadData).WithStatusCode(http.StatusCreated)
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
		uploadLocation:  params.UploadLocation,
		imageRegistry:   params.RegistrySet.ImageRegistry,
		manualRegistry:  params.RegistrySet.ManualRegistry,
		invoiceRegistry: params.RegistrySet.InvoiceRegistry,
	}

	return func(r chi.Router) {
		r.With(commodityCtx(params.RegistrySet.CommodityRegistry)).
			Route("/commodities/{commodityID}", func(r chi.Router) {
				r.With(api.uploadFiles(mimekit.ImageContentTypes()...)).Post("/images", api.handleImagesUpload)
				r.With(api.uploadFiles(mimekit.DocContentTypes()...)).Post("/manuals", api.handleManualsUpload)
				r.With(api.uploadFiles(mimekit.DocContentTypes()...)).Post("/invoices", api.handleInvoicesUpload)
			})
	}
}
