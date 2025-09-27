package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/models"
)

// UploadStatusResponse represents the response for upload status
type UploadStatusResponse struct {
	HTTPStatusCode int               `json:"-"` // HTTP response status code
	Data           *UploadStatusData `json:"data"`
}

// UploadStatusData represents the data part of upload status response
type UploadStatusData struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type" example:"upload-status" enums:"upload-status"`
	Attributes *UploadStatusAttributes `json:"attributes"`
}

// UploadStatusAttributes represents the attributes of upload status
type UploadStatusAttributes struct {
	OperationName     string `json:"operation_name"`
	ActiveUploads     int    `json:"active_uploads"`
	MaxUploads        int    `json:"max_uploads"`
	AvailableUploads  int    `json:"available_uploads"`
	CanStartUpload    bool   `json:"can_start_upload"`
	RetryAfterSeconds *int   `json:"retry_after_seconds,omitempty"`
}

// NewUploadStatusResponse creates a new upload status response
func NewUploadStatusResponse(status *models.UploadStatus) *UploadStatusResponse {
	return &UploadStatusResponse{
		Data: &UploadStatusData{
			ID:   status.OperationName, // Use operation name as ID
			Type: "upload-status",
			Attributes: &UploadStatusAttributes{
				OperationName:     status.OperationName,
				ActiveUploads:     status.ActiveUploads,
				MaxUploads:        status.MaxUploads,
				AvailableUploads:  status.AvailableUploads,
				CanStartUpload:    status.CanStartUpload,
				RetryAfterSeconds: status.RetryAfterSeconds,
			},
		},
	}
}

// Render renders the UploadStatusResponse as an HTTP response
func (r *UploadStatusResponse) Render(_w http.ResponseWriter, req *http.Request) error {
	render.Status(req, statusCodeDef(r.HTTPStatusCode, http.StatusOK))
	return nil
}

// WithStatusCode sets the HTTP status code for the response
func (r *UploadStatusResponse) WithStatusCode(code int) *UploadStatusResponse {
	r.HTTPStatusCode = code
	return r
}
