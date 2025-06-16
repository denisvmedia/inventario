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

	ID         string            `json:"id"`
	Type       string            `json:"type" example:"files" enums:"files"`
	Attributes models.FileEntity `json:"attributes"`
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
	Data *FileRequestDataWrapper `json:"data"`
}

// FileRequestDataWrapper wraps the file request data
type FileRequestDataWrapper struct {
	ID         string          `json:"id,omitempty"`
	Type       string          `json:"type"`
	Attributes FileRequestData `json:"attributes"`
}

func (frdw *FileRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&frdw.Type, validation.Required, validation.In("files")),
		validation.Field(&frdw.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, frdw, fields...)
}

var _ render.Binder = (*FileRequest)(nil)
var _ validation.ValidatableWithContext = (*FileRequest)(nil)
var _ validation.ValidatableWithContext = (*FileRequestDataWrapper)(nil)

// Bind validates the FileRequest.
func (fr *FileRequest) Bind(r *http.Request) error {
	return fr.ValidateWithContext(r.Context())
}

// ValidateWithContext validates the FileRequest with context.
func (fr *FileRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&fr.Data, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, fr, fields...)
}

// FileRequestData contains the attributes for creating/updating a file.
type FileRequestData struct {
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	Path             string   `json:"path,omitempty"`              // Only for updates
	LinkedEntityType string   `json:"linked_entity_type,omitempty"` // commodity, export, or empty
	LinkedEntityID   string   `json:"linked_entity_id,omitempty"`   // ID of linked entity
	LinkedEntityMeta string   `json:"linked_entity_meta,omitempty"` // metadata about the link
}

// FileAttributes is an alias for FileRequestData for backward compatibility with tests
type FileAttributes = FileRequestData

func (frd *FileRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (frd *FileRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&frd.Title, validation.Length(0, 255)), // Title is now optional
		validation.Field(&frd.Description, validation.Length(0, 1000)),
		validation.Field(&frd.Tags, validation.Length(0, 100)), // Allow up to 100 tags
		validation.Field(&frd.LinkedEntityType, validation.In("", "commodity", "export")),
		validation.Field(&frd.LinkedEntityID, validation.Length(0, 255)),
		validation.Field(&frd.LinkedEntityMeta, validation.Length(0, 255)),
	)

	return validation.ValidateStructWithContext(ctx, frd, fields...)
}

var _ render.Binder = (*FileUpdateRequest)(nil)
var _ validation.ValidatableWithContext = (*FileUpdateRequest)(nil)
var _ validation.ValidatableWithContext = (*FileRequestData)(nil)

// FileUpdateRequest represents a request to update a file's metadata.
type FileUpdateRequest struct {
	Data *FileUpdateRequestData `json:"data"`
}

// Bind binds the commodity data from the request to the FileUpdateRequest object.
func (fr *FileUpdateRequest) Bind(r *http.Request) error {
	return fr.ValidateWithContext(r.Context())
}

func (fr *FileUpdateRequest) Validate() error {
	return models.ErrMustUseValidateWithContext
}

// ValidateWithContext validates the commodity request data.
func (fr *FileUpdateRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&fr.Data, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, fr, fields...)
}

// FileUpdateRequestData contains the attributes for updating a file.
type FileUpdateRequestData struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type" example:"files" enums:"files"`
	Attributes FileUpdateRequestFileData `json:"attributes"`
}

func (fd *FileUpdateRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (fd *FileUpdateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&fd.Type, validation.Required, validation.In("files")),
		validation.Field(&fd.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, fd, fields...)
}

// FileUpdateRequestFileData contains the attributes for updating a file.
type FileUpdateRequestFileData struct {
	Title            string   `json:"title"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	Path             string   `json:"path"`                         // User-editable filename
	LinkedEntityType string   `json:"linked_entity_type,omitempty"` // commodity, export, or empty
	LinkedEntityID   string   `json:"linked_entity_id,omitempty"`   // ID of linked entity
	LinkedEntityMeta string   `json:"linked_entity_meta,omitempty"` // metadata about the link
}

func (fur *FileUpdateRequestFileData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

// ValidateWithContext validates the FileUpdateRequest with context.
func (fur *FileUpdateRequestFileData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&fur.Title, validation.Length(0, 255)), // Title is now optional
		validation.Field(&fur.Description, validation.Length(0, 1000)),
		validation.Field(&fur.Path, validation.Required),
		validation.Field(&fur.Tags, validation.Length(0, 100)), // Allow up to 100 tags
		validation.Field(&fur.LinkedEntityType, validation.In("", "commodity", "export")),
		validation.Field(&fur.LinkedEntityID, validation.Length(0, 255)),
		validation.Field(&fur.LinkedEntityMeta, validation.Length(0, 255)),
	)

	return validation.ValidateStructWithContext(ctx, fur, fields...)
}
