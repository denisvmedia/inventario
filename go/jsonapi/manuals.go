// jsonapi/manuals.go

package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/models"
)

// ManualResponse is an object that holds manual information.
type ManualResponse struct {
	HTTPStatusCode int `json:"-"` // HTTP response status code

	ID         string        `json:"id"`
	Type       string        `json:"type" example:"manuals" enums:"manuals"`
	Attributes models.Manual `json:"attributes"`
}

// NewManualResponse creates a new ManualResponse instance.
func NewManualResponse(manual *models.Manual) *ManualResponse {
	return &ManualResponse{
		ID:         manual.ID,
		Type:       "manuals",
		Attributes: *manual,
	}
}

// WithStatusCode sets the HTTP response status code for the ManualResponse.
func (mr *ManualResponse) WithStatusCode(statusCode int) *ManualResponse {
	tmp := *mr
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the ManualResponse as an HTTP response.
func (mr *ManualResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(mr.HTTPStatusCode, http.StatusOK))
	return nil
}

// ManualsMeta is a meta information for ManualsResponse.
type ManualsMeta struct {
	Manuals    int                `json:"manuals" example:"1" format:"int64"`
	SignedUrls map[string]URLData `json:"signed_urls,omitempty"` // Map of file ID to signed URLs and thumbnails
}

// ManualsResponse is an object that holds a list of manual information.
type ManualsResponse struct {
	Data []*models.Manual `json:"data"`
	Meta ManualsMeta      `json:"meta"`
}

// NewManualsResponse creates a new ManualsResponse instance.
func NewManualsResponse(manuals []*models.Manual, total int) *ManualsResponse {
	// Ensure Data is never nil to maintain consistent JSON output
	if manuals == nil {
		manuals = []*models.Manual{}
	}
	return &ManualsResponse{
		Data: manuals,
		Meta: ManualsMeta{Manuals: total},
	}
}

// NewManualsResponseWithSignedUrls creates a new ManualsResponse instance with signed URLs.
func NewManualsResponseWithSignedUrls(manuals []*models.Manual, total int, signedUrls map[string]URLData) *ManualsResponse {
	// Ensure Data is never nil to maintain consistent JSON output
	if manuals == nil {
		manuals = []*models.Manual{}
	}
	return &ManualsResponse{
		Data: manuals,
		Meta: ManualsMeta{
			Manuals:    total,
			SignedUrls: signedUrls,
		},
	}
}

// Render renders the ManualsResponse as an HTTP response.
func (*ManualsResponse) Render(_w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
