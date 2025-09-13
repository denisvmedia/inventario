package jsonapi

import (
	"context"
	"errors"
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
	Meta       *FileMeta         `json:"meta,omitempty"` // Optional meta field for signed URLs
}

// FileMeta represents the meta part of a file response.
type FileMeta struct {
	SignedUrls map[string]URLData `json:"signed_urls,omitempty"` // Map of file ID to signed URLs and thumbnails
}

// NewFileResponse creates a new FileResponse instance.
func NewFileResponse(file *models.FileEntity) *FileResponse {
	return &FileResponse{
		ID:         file.ID,
		Type:       "files",
		Attributes: *file,
	}
}

// NewFileResponseWithSignedUrls creates a new FileResponse instance with signed URLs.
func NewFileResponseWithSignedUrls(file *models.FileEntity, signedUrls map[string]URLData) *FileResponse {
	return &FileResponse{
		ID:         file.ID,
		Type:       "files",
		Attributes: *file,
		Meta: &FileMeta{
			SignedUrls: signedUrls,
		},
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
	Files      int                `json:"files" example:"10" format:"int64"`
	Total      int                `json:"total" example:"100" format:"int64"`
	SignedUrls map[string]URLData `json:"signed_urls,omitempty"` // Map of file ID to signed URLs and thumbnails
}

// FilesResponse is an object that holds a list of file information.
type FilesResponse struct {
	Data []*models.FileEntity `json:"data"`
	Meta FilesMeta            `json:"meta"`
}

// NewFilesResponse creates a new FilesResponse instance.
func NewFilesResponse(files []*models.FileEntity, total int) *FilesResponse {
	// Ensure Data is never nil to maintain consistent JSON output
	if files == nil {
		files = []*models.FileEntity{}
	}
	return &FilesResponse{
		Data: files,
		Meta: FilesMeta{Files: len(files), Total: total},
	}
}

// NewFilesResponseWithSignedUrls creates a new FilesResponse instance with signed URLs.
func NewFilesResponseWithSignedUrls(files []*models.FileEntity, total int, signedUrls map[string]URLData) *FilesResponse {
	// Ensure Data is never nil to maintain consistent JSON output
	if files == nil {
		files = []*models.FileEntity{}
	}
	return &FilesResponse{
		Data: files,
		Meta: FilesMeta{
			Files:      len(files),
			Total:      total,
			SignedUrls: signedUrls,
		},
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
	// Prevent user-specified IDs in create requests
	if frdw.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}

	// Prevent manual creation of export-linked files
	if frdw.Attributes.LinkedEntityType == "export" {
		return errors.New("export files cannot be manually created")
	}

	// Validate commodity linking
	if frdw.Attributes.LinkedEntityType == "commodity" {
		if frdw.Attributes.LinkedEntityID == "" {
			return errors.New("linked entity ID is required for commodity files")
		}
		if frdw.Attributes.LinkedEntityMeta == "" {
			return errors.New("linked entity meta is required for commodity files")
		}
		if frdw.Attributes.LinkedEntityMeta != "images" &&
			frdw.Attributes.LinkedEntityMeta != "invoices" &&
			frdw.Attributes.LinkedEntityMeta != "manuals" {
			return errors.New("linked entity meta must be one of: images, invoices, manuals")
		}
	}

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
	Path             string   `json:"path,omitempty"`               // Only for updates
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
		validation.Field(&frd.Tags, validation.Length(0, 100)),                  // Allow up to 100 tags
		validation.Field(&frd.LinkedEntityType, validation.In("", "commodity")), // Only allow commodity for manual creation
		validation.Field(&frd.LinkedEntityID, validation.Length(0, 255)),
		validation.Field(&frd.LinkedEntityMeta, validation.Length(0, 255)),
	)

	// If linked entity type is specified, validate the linked entity ID and meta
	if frd.LinkedEntityType == "commodity" {
		fields = append(fields,
			validation.Field(&frd.LinkedEntityID, validation.Required),
			validation.Field(&frd.LinkedEntityMeta, validation.Required, validation.In("images", "invoices", "manuals")),
		)
	}

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
		validation.Field(&fur.Tags, validation.Length(0, 100)),                            // Allow up to 100 tags
		validation.Field(&fur.LinkedEntityType, validation.In("", "commodity", "export")), // Allow export for existing files
		validation.Field(&fur.LinkedEntityID, validation.Length(0, 255)),
		validation.Field(&fur.LinkedEntityMeta, validation.Length(0, 255)),
	)

	// If linked entity type is specified, validate the linked entity ID and meta
	switch fur.LinkedEntityType {
	case "commodity":
		fields = append(fields,
			validation.Field(&fur.LinkedEntityID, validation.Required),
			validation.Field(&fur.LinkedEntityMeta, validation.Required, validation.In("images", "invoices", "manuals")),
		)
	case "export":
		fields = append(fields,
			validation.Field(&fur.LinkedEntityID, validation.Required),
			validation.Field(&fur.LinkedEntityMeta, validation.Required, validation.In("xml-1.0")),
		)
	}

	return validation.ValidateStructWithContext(ctx, fur, fields...)
}

// SearchResponse represents a generic search response
type SearchResponse struct {
	Data any        `json:"data"`
	Meta SearchMeta `json:"meta"`
}

// SearchMeta contains metadata about search results
type SearchMeta struct {
	EntityType string `json:"entity_type"`
	Total      int    `json:"total"`
	Query      string `json:"query,omitempty"`
}

// NewSearchResponse creates a new search response
func NewSearchResponse(entityType string, data any, total int) *SearchResponse {
	return &SearchResponse{
		Data: data,
		Meta: SearchMeta{
			EntityType: entityType,
			Total:      total,
		},
	}
}

// Render renders the SearchResponse as an HTTP response
func (*SearchResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// SignedFileURLResponse is an object that holds signed file URL information.
type SignedFileURLResponse struct {
	HTTPStatusCode int `json:"-"` // HTTP response status code

	ID         string  `json:"id"`                               // file id
	Type       string  `json:"type" example:"urls" enums:"urls"` // resource type
	Attributes URLData `json:"attributes"`                       // URL data
}

// URLData is an object that holds URL data information.
type URLData struct {
	URL        string            `json:"url"`                  // signed URL for file access
	Thumbnails map[string]string `json:"thumbnails,omitempty"` // map of thumbnail size to signed URL
}

// NewSignedFileURLResponse creates a new SignedFileURLResponse instance.
func NewSignedFileURLResponse(fileID, signedURL string) *SignedFileURLResponse {
	return &SignedFileURLResponse{
		ID:   fileID,
		Type: "urls",
		Attributes: URLData{
			URL: signedURL,
		},
	}
}

// NewSignedFileURLResponseWithThumbnails creates a new SignedFileURLResponse instance with thumbnails.
func NewSignedFileURLResponseWithThumbnails(fileID, signedURL string, thumbnails map[string]string) *SignedFileURLResponse {
	return &SignedFileURLResponse{
		ID:   fileID,
		Type: "urls",
		Attributes: URLData{
			URL:        signedURL,
			Thumbnails: thumbnails,
		},
	}
}

// WithStatusCode sets the HTTP response status code for the SignedFileURLResponse.
func (sfur *SignedFileURLResponse) WithStatusCode(statusCode int) *SignedFileURLResponse {
	tmp := *sfur
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the SignedFileURLResponse as an HTTP response.
func (sfur *SignedFileURLResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(sfur.HTTPStatusCode, http.StatusOK))
	return nil
}
