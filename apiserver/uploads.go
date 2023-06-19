package apiserver

import (
	"context"
	"errors"
	"io"
	"net/http"

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

const uploadedFilesCtxKey ctxValueKey = "uploadedFiles"

func uploadedFilesFromContext(ctx context.Context) [][]string {
	uploadedFiles, ok := ctx.Value(uploadedFilesCtxKey).([][]string)
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

	for _, v := range uploadedFiles {
		img, err := api.imageRegistry.Create(models.Image{
			Path:        v[0],
			Ext:         mimekit.ExtensionByMime(v[1]),
			CommodityID: entityID,
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

	for _, v := range uploadedFiles {
		img, err := api.manualRegistry.Create(models.Manual{
			Path:        v[0],
			Ext:         mimekit.ExtensionByMime(v[1]),
			CommodityID: entityID,
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

	for _, v := range uploadedFiles {
		img, err := api.invoiceRegistry.Create(models.Invoice{
			Path:        v[0],
			Ext:         mimekit.ExtensionByMime(v[1]),
			CommodityID: entityID,
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

			var filePaths [][]string // TODO: use a dedicated struct instead of a sub-slice
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
				filePaths = append(filePaths, []string{filename, mimeType})
			}

			ctx := context.WithValue(r.Context(), uploadedFilesCtxKey, filePaths)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func (api *uploadsAPI) saveFile(ctx context.Context, filename string, src io.Reader, allowedContentTypes []string) (mimeType string, err error) {
	b, err := blob.OpenBucket(ctx, api.uploadLocation)
	if err != nil {
		return "", errkit.Wrap(err, "failed to open bucket") // TODO: we might want adding uploadLocation as a field, but it may contain sensitive data
	}
	defer b.Close()

	fw, err := b.NewWriter(ctx, filename, nil)
	if err != nil {
		return "", errkit.Wrap(err, "failed to create a new writer")
	}
	defer fw.Close()

	wrappedSrc := mimekit.NewMIMEReader(src, allowedContentTypes)

	_, err = io.Copy(fw, wrappedSrc)
	if err != nil {
		return "", errkit.ChainWrap(err, "failed when saving the file").WithField("filename", filename)
	}

	return wrappedSrc.MIMEType(), nil
}

func Uploads(params Params) func(r chi.Router) {
	api := &uploadsAPI{
		uploadLocation:  params.UploadLocation,
		imageRegistry:   params.ImageRegistry,
		manualRegistry:  params.ManualRegistry,
		invoiceRegistry: params.InvoiceRegistry,
	}

	return func(r chi.Router) {
		r.With(commodityCtx(params.CommodityRegistry)).
			Route("/commodities/{commodityID}", func(r chi.Router) {
				r.With(api.uploadFiles(mimekit.ImageContentTypes()...)).Post("/images", api.handleImagesUpload)
				r.With(api.uploadFiles(mimekit.DocContentTypes()...)).Post("/manuals", api.handleManualsUpload)
				r.With(api.uploadFiles(mimekit.DocContentTypes()...)).Post("/invoices", api.handleInvoicesUpload)
			})
	}
}
