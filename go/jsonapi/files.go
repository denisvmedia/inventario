package jsonapi

import (
	"context"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

// FileResponse is an object that holds file information.
type FileResponse struct {
	HTTPStatusCode int `json:"-"` // HTTP response status code

	ID         string              `json:"id"`
	Type       string              `json:"type" example:"files" enums:"files"`
	Attributes models.FileEntity   `json:"attributes"`
}

// NewFileResponse creates a new FileResponse instance.
func NewFileResponse(file *models.FileEntity) *FileResponse {
	return &FileResponse{
		ID:         file.ID,
		Type:       "files",
		Attributes: *file,
	}
}

// WithStatusCode sets the HTTP response status code for the FileResponse.
func (fr *FileResponse) WithStatusCode(statusCode int) *FileResponse {
	tmp := *fr
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the FileResponse as an HTTP response.
func (fr *FileResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(fr.HTTPStatusCode, http.StatusOK))
	return nil
}

// FilesMeta is a meta information for FilesResponse.
type FilesMeta struct {
	Files int `json:"files" example:"10" format:"int64"`
	Total int `json:"total" example:"100" format:"int64"`
}

// FilesResponse is an object that holds a list of file information.
type FilesResponse struct {
	Data []*models.FileEntity `json:"data"`
	Meta FilesMeta            `json:"meta"`
}

// NewFilesResponse creates a new FilesResponse instance.
func NewFilesResponse(files []*models.FileEntity, total int) *FilesResponse {
	return &FilesResponse{
		Data: files,
		Meta: FilesMeta{Files: len(files), Total: total},
	}
}

// Render renders the FilesResponse as an HTTP response.
func (*FilesResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// FileRequest represents a request to create or update a file.
type FileRequest struct {
	Data struct {
		ID         string            `json:"id,omitempty"`
		Type       string            `json:"type"`
		Attributes FileRequestData   `json:"attributes"`
	} `json:"data"`
}

// FileRequestData contains the attributes for creating/updating a file.
type FileRequestData struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Type        models.FileType   `json:"type"`
	Tags        []string          `json:"tags"`
	Path        string            `json:"path,omitempty"` // Only for updates
}

// Bind validates the FileRequest.
func (fr *FileRequest) Bind(r *http.Request) error {
	return fr.ValidateWithContext(r.Context())
}

// ValidateWithContext validates the FileRequest with context.
func (fr *FileRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&fr.Data.Type, validation.Required, validation.In("files")),
		validation.Field(&fr.Data.Attributes.Title, validation.Required, validation.Length(1, 255)),
		validation.Field(&fr.Data.Attributes.Description, validation.Length(0, 1000)),
		validation.Field(&fr.Data.Attributes.Type, validation.Required, validation.In(
			models.FileTypeImage, models.FileTypeDocument, models.FileTypeVideo,
			models.FileTypeAudio, models.FileTypeArchive, models.FileTypeOther,
		)),
	)

	return validation.ValidateStructWithContext(ctx, fr, fields...)
}

// FileUpdateRequest represents a request to update a file's metadata.
type FileUpdateRequest struct {
	Data struct {
		ID         string                    `json:"id"`
		Type       string                    `json:"type"`
		Attributes FileUpdateRequestData     `json:"attributes"`
	} `json:"data"`
}

// FileUpdateRequestData contains the attributes for updating a file.
type FileUpdateRequestData struct {
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Type        models.FileType `json:"type"`
	Tags        []string        `json:"tags"`
	Path        string          `json:"path"` // User-editable filename
}

// Bind validates the FileUpdateRequest.
func (fur *FileUpdateRequest) Bind(r *http.Request) error {
	return fur.ValidateWithContext(r.Context())
}

// ValidateWithContext validates the FileUpdateRequest with context.
func (fur *FileUpdateRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&fur.Data.ID, validation.Required),
		validation.Field(&fur.Data.Type, validation.Required, validation.In("files")),
		validation.Field(&fur.Data.Attributes.Title, validation.Required, validation.Length(1, 255)),
		validation.Field(&fur.Data.Attributes.Description, validation.Length(0, 1000)),
		validation.Field(&fur.Data.Attributes.Type, validation.Required, validation.In(
			models.FileTypeImage, models.FileTypeDocument, models.FileTypeVideo,
			models.FileTypeAudio, models.FileTypeArchive, models.FileTypeOther,
		)),
		validation.Field(&fur.Data.Attributes.Path, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, fur, fields...)
}

// FileUploadRequest represents a request to upload a file.
type FileUploadRequest struct {
	Data struct {
		Type       string                      `json:"type"`
		Attributes FileUploadRequestData       `json:"attributes"`
	} `json:"data"`
}

// FileUploadRequestData contains the attributes for uploading a file.
type FileUploadRequestData struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Type        models.FileType   `json:"type"`
	Tags        []string          `json:"tags"`
}

// Bind validates the FileUploadRequest.
func (fur *FileUploadRequest) Bind(r *http.Request) error {
	return fur.ValidateWithContext(r.Context())
}

// ValidateWithContext validates the FileUploadRequest with context.
func (fur *FileUploadRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&fur.Data.Type, validation.Required, validation.In("files")),
		validation.Field(&fur.Data.Attributes.Title, validation.Required, validation.Length(1, 255)),
		validation.Field(&fur.Data.Attributes.Description, validation.Length(0, 1000)),
		validation.Field(&fur.Data.Attributes.Type, validation.Required, validation.In(
			models.FileTypeImage, models.FileTypeDocument, models.FileTypeVideo,
			models.FileTypeAudio, models.FileTypeArchive, models.FileTypeOther,
		)),
	)

	return validation.ValidateStructWithContext(ctx, fur, fields...)
}
