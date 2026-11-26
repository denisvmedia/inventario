package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
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

// FileCategoryCounts is the per-category count payload for the three UI tiles.
// `All` is the sum across the three buckets, scoped to the same filters as the
// containing GET /files/category-counts request. `Bytes` carries the same
// shape but with per-category byte totals so the Files page can render the
// cumulative "{N} files · {Y} total" footer without a second round-trip.
//
// #1622 dropped the `invoices` bucket — "invoice" is now a conventional tag
// (`FileTagInvoice`) carried inside `documents`.
type FileCategoryCounts struct {
	Images    int               `json:"images" example:"3" format:"int64"`
	Documents int               `json:"documents" example:"6" format:"int64"`
	Other     int               `json:"other" example:"2" format:"int64"`
	All       int               `json:"all" example:"11" format:"int64"`
	Bytes     FileCategoryBytes `json:"bytes"`
}

// FileCategoryBytes is the per-category byte-total payload, mirroring
// FileCategoryCounts. `All` is the sum across the three buckets so the FE can
// pick the right number based on the active tile (and read All when the user
// is on the synthetic "All" filter).
type FileCategoryBytes struct {
	Images    int64 `json:"images" example:"1048576" format:"int64"`
	Documents int64 `json:"documents" example:"786432" format:"int64"`
	Other     int64 `json:"other" example:"131072" format:"int64"`
	All       int64 `json:"all" example:"1966080" format:"int64"`
}

// FileCategoryCountsResponse is the JSON:API envelope for category counts.
type FileCategoryCountsResponse struct {
	Data FileCategoryCounts `json:"data"`
}

// NewFileCategoryCountsResponse fills the four buckets from the registry maps
// and computes `All` (both count and bytes) so callers don't have to. Missing
// buckets are treated as zero, matching the registry contract.
func NewFileCategoryCountsResponse(counts map[models.FileCategory]int, bytes map[models.FileCategory]int64) *FileCategoryCountsResponse {
	data := FileCategoryCounts{
		Images:    counts[models.FileCategoryImages],
		Documents: counts[models.FileCategoryDocuments],
		Other:     counts[models.FileCategoryOther],
		Bytes: FileCategoryBytes{
			Images:    bytes[models.FileCategoryImages],
			Documents: bytes[models.FileCategoryDocuments],
			Other:     bytes[models.FileCategoryOther],
		},
	}
	data.All = data.Images + data.Documents + data.Other
	data.Bytes.All = data.Bytes.Images + data.Bytes.Documents + data.Bytes.Other
	return &FileCategoryCountsResponse{Data: data}
}

// Render renders the FileCategoryCountsResponse as an HTTP response.
func (fr *FileCategoryCountsResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
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

// FileRequestData contains the attributes for creating a file entity.
// It is metadata-only: the blob path is never accepted from the client.
// The physical blob key is populated server-side by the upload flow
// (/uploads/file). See updateFile for the separate update request struct.
// The new file is standalone; POST /files/{id}/links attaches it.
type FileRequestData struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
}

// FileAttributes is an alias for FileRequestData for backward compatibility with tests
type FileAttributes = FileRequestData

func (frd FileRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (frd FileRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		// Title is optional, at most 255 characters (not UTF-8 bytes).
		validation.Field(&frd.Title, validation.RuneLength(0, 255)),
		validation.Field(&frd.Description, validation.Length(0, 1000)),
		validation.Field(&frd.Tags, validation.Length(0, 100)), // Allow up to 100 tags
	)

	return validation.ValidateStructWithContext(ctx, &frd, fields...)
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
		// Attributes is a struct VALUE, and FileUpdateRequestFileData implements
		// ValidatableWithContext on a POINTER receiver. ozzo's Field() dereferences
		// the pointer to a value before type-asserting against
		// ValidatableWithContext, so the nested validator would be silently
		// skipped (issue #2033). Invoke it explicitly so the path rules actually
		// run.
		validation.Field(&fd.Attributes, validation.Required, validation.WithContext(
			func(ctx context.Context, _ any) error {
				return fd.Attributes.ValidateWithContext(ctx)
			},
		)),
	)
	return validation.ValidateStructWithContext(ctx, fd, fields...)
}

// FileUpdateRequestFileData contains the attributes for updating a file. The
// file's attachments are not part of it: POST /files/{id}/links and
// DELETE /files/{id}/links/{linkID} change them.
type FileUpdateRequestFileData struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Tags        []string `json:"tags"`
	Path        string   `json:"path"` // User-editable filename
}

func (fur FileUpdateRequestFileData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

// ValidateWithContext validates the FileUpdateRequest with context. `path` is
// not required: a client that edits only the title or tags omits it, and the
// handler keeps the persisted path. Only its length is bounded.
func (fur FileUpdateRequestFileData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &fur,
		// Title is optional. Its limit counts characters, not UTF-8 bytes: a
		// Czech or Russian title takes two bytes for most letters.
		validation.Field(&fur.Title, validation.RuneLength(0, 255)),
		validation.Field(&fur.Description, validation.Length(0, 1000)),
		validation.Field(&fur.Path, validation.Length(0, 1024)),
		validation.Field(&fur.Tags, validation.Length(0, 100)), // Allow up to 100 tags
	)
}

// FileLinkRequest is the body of POST /files/{id}/links: the entity to attach
// the file to.
type FileLinkRequest struct {
	Data *FileLinkRequestData `json:"data"`
}

var _ render.Binder = (*FileLinkRequest)(nil)

// Bind validates the FileLinkRequest.
func (fr *FileLinkRequest) Bind(r *http.Request) error {
	return fr.ValidateWithContext(r.Context())
}

func (fr *FileLinkRequest) Validate() error {
	return models.ErrMustUseValidateWithContext
}

// ValidateWithContext validates the FileLinkRequest.
func (fr *FileLinkRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, fr,
		validation.Field(&fr.Data, validation.Required),
	)
}

// FileLinkRequestData wraps the attributes of a FileLinkRequest.
type FileLinkRequestData struct {
	Type       string                    `json:"type" example:"file-links" enums:"file-links"`
	Attributes FileLinkRequestAttributes `json:"attributes"`
}

func (fd *FileLinkRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

// ValidateWithContext validates the type and, explicitly, the attributes (a
// struct value, see FileUpdateRequestData for why the call is spelled out).
func (fd *FileLinkRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, fd,
		validation.Field(&fd.Type, validation.Required, validation.In("file-links")),
		validation.Field(&fd.Attributes, validation.Required, validation.WithContext(
			func(ctx context.Context, _ any) error {
				return fd.Attributes.ValidateWithContext(ctx)
			},
		)),
	)
}

// FileLinkRequestAttributes names the entity to attach the file to. Meta is the
// bucket on the entity's Files tab; when empty, the server picks "images" for
// an image and the entity's document bucket for anything else. The first
// attachment into a bucket given here sets the file's category from it.
type FileLinkRequestAttributes struct {
	EntityType string `json:"entity_type" example:"commodity" enums:"commodity,location,area,draft"`
	EntityID   string `json:"entity_id"`
	Meta       string `json:"meta,omitempty" example:"documents" enums:"images,documents,files"`
}

func (fa FileLinkRequestAttributes) Validate() error {
	return models.ErrMustUseValidateWithContext
}

// ValidateWithContext checks the entity type and, when a meta is given, that it
// belongs to that type.
func (fa FileLinkRequestAttributes) ValidateWithContext(ctx context.Context) error {
	metas := []any{""}
	for _, m := range models.FileLinkMetaValues(fa.EntityType) {
		metas = append(metas, m)
	}
	return validation.ValidateStructWithContext(ctx, &fa,
		validation.Field(&fa.EntityType, validation.Required, validation.In(
			models.FileLinkCommodity, models.FileLinkLocation, models.FileLinkArea, models.FileLinkDraft,
		)),
		validation.Field(&fa.EntityID, validation.Required, validation.Length(1, 255)),
		validation.Field(&fa.Meta, validation.In(metas...)),
	)
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
	// Matches maps result id → the fields the query was found in. Only
	// the commodities search fills it.
	Matches map[string][]registry.CommoditySearchMatch `json:"matches,omitempty"`
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

// WithMatches sets meta.matches; an empty map leaves it out.
func (sr *SearchResponse) WithMatches(matches map[string][]registry.CommoditySearchMatch) *SearchResponse {
	if len(matches) > 0 {
		sr.Meta.Matches = matches
	}
	return sr
}

// Render renders the SearchResponse as an HTTP response
func (*SearchResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// ThumbnailStatusData represents thumbnail generation status data
type ThumbnailStatusData struct {
	ID         string                    `json:"id"`
	Type       string                    `json:"type" example:"thumbnail-status" enums:"thumbnail-status"`
	Attributes ThumbnailStatusAttributes `json:"attributes"`
}

// ThumbnailStatusAttributes contains the thumbnail status information
type ThumbnailStatusAttributes struct {
	FileID                string          `json:"file_id"`
	Status                string          `json:"status"`
	ThumbnailsExist       map[string]bool `json:"thumbnails_exist"`
	JobID                 *string         `json:"job_id,omitempty"`
	AttemptCount          *int            `json:"attempt_count,omitempty"`
	MaxAttempts           *int            `json:"max_attempts,omitempty"`
	CreatedAt             *string         `json:"created_at,omitempty"`
	UpdatedAt             *string         `json:"updated_at,omitempty"`
	ProcessingStartedAt   *string         `json:"processing_started_at,omitempty"`
	ProcessingCompletedAt *string         `json:"processing_completed_at,omitempty"`
	ErrorMessage          *string         `json:"error_message,omitempty"`
	Message               string          `json:"message"`
}

// ThumbnailStatusResponse represents a thumbnail status response
type ThumbnailStatusResponse struct {
	HTTPStatusCode int                 `json:"-"` // HTTP response status code
	Data           ThumbnailStatusData `json:"data"`
}

// NewThumbnailStatusResponse creates a new thumbnail status response following JSON:API patterns
func NewThumbnailStatusResponse(fileID string, job *models.ThumbnailGenerationJob, thumbnailsExist map[string]bool) *ThumbnailStatusResponse {
	attrs := ThumbnailStatusAttributes{
		FileID:          fileID,
		ThumbnailsExist: thumbnailsExist,
	}

	if job == nil {
		attrs.Status = "no_job"
		attrs.Message = "No thumbnail generation job exists"
	} else {
		attrs.Status = string(job.Status)
		attrs.JobID = &job.ID
		attrs.AttemptCount = &job.AttemptCount
		attrs.MaxAttempts = &job.MaxAttempts

		createdAt := job.CreatedAt.Format("2006-01-02T15:04:05Z07:00")
		updatedAt := job.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")
		attrs.CreatedAt = &createdAt
		attrs.UpdatedAt = &updatedAt

		if job.ProcessingStartedAt != nil {
			startedAt := job.ProcessingStartedAt.Format("2006-01-02T15:04:05Z07:00")
			attrs.ProcessingStartedAt = &startedAt
		}
		if job.ProcessingCompletedAt != nil {
			completedAt := job.ProcessingCompletedAt.Format("2006-01-02T15:04:05Z07:00")
			attrs.ProcessingCompletedAt = &completedAt
		}
		if job.ErrorMessage != "" {
			attrs.ErrorMessage = &job.ErrorMessage
		}

		// Set appropriate message based on status
		switch job.Status {
		case models.ThumbnailStatusPending:
			attrs.Message = "Thumbnail generation is pending"
		case models.ThumbnailStatusProcessing:
			attrs.Message = "Thumbnail generation is in progress"
		case models.ThumbnailStatusCompleted:
			attrs.Message = "Thumbnail generation completed successfully"
		case models.ThumbnailStatusFailed:
			attrs.Message = "Thumbnail generation failed"
		default:
			attrs.Message = "Unknown thumbnail generation status"
		}
	}

	return &ThumbnailStatusResponse{
		Data: ThumbnailStatusData{
			ID:         fileID,
			Type:       "thumbnail-status",
			Attributes: attrs,
		},
	}
}

// WithStatusCode sets the HTTP response status code for the ThumbnailStatusResponse.
func (tsr *ThumbnailStatusResponse) WithStatusCode(statusCode int) *ThumbnailStatusResponse {
	tmp := *tsr
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the ThumbnailStatusResponse as an HTTP response
func (tsr *ThumbnailStatusResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(tsr.HTTPStatusCode, http.StatusOK))
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
	URL        string            `json:"url"`                  // signed URL for file access (attachment / download)
	InlineURL  string            `json:"inline_url,omitempty"` // signed URL that serves the file inline for in-browser viewing ("Open in new tab", #1962); empty when not available
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
