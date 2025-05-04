package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"
)

// UploadResponse is an object that holds upload information.
type UploadResponse struct {
	HTTPStatusCode int `json:"-"` // HTTP response status code

	ID         string     `json:"id"`
	Type       string     `json:"type" example:"uploads" enums:"uploads"`
	Attributes UploadData `json:"attributes"`
}

// UploadData is an object that holds upload data information.
type UploadData struct {
	Type      string   `json:"type" example:"images"`
	FileNames []string `json:"fileNames"`
}

// NewUploadResponse creates a new UploadResponse instance.
func NewUploadResponse(entityID string, uploadData UploadData) *UploadResponse {
	return &UploadResponse{
		ID:         entityID,
		Type:       "uploads",
		Attributes: uploadData,
	}
}

// WithStatusCode sets the HTTP response status code for the UploadResponse.
func (cr *UploadResponse) WithStatusCode(statusCode int) *UploadResponse {
	tmp := *cr
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the UploadResponse as an HTTP response.
func (cr *UploadResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(cr.HTTPStatusCode, http.StatusOK))
	return nil
}
