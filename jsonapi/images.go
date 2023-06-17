// jsonapi/images.go

package jsonapi

import (
	"net/http"

	"github.com/go-chi/render"

	"github.com/denisvmedia/inventario/models"
)

// ImageResponse is an object that holds image information.
type ImageResponse struct {
	HTTPStatusCode int `json:"-"` // HTTP response status code

	ID         string       `json:"id"`
	Type       string       `json:"type" example:"images" enums:"images"`
	Attributes models.Image `json:"attributes"`
}

// NewImageResponse creates a new ImageResponse instance.
func NewImageResponse(image *models.Image) *ImageResponse {
	return &ImageResponse{
		ID:         image.ID,
		Type:       "images",
		Attributes: *image,
	}
}

// WithStatusCode sets the HTTP response status code for the ImageResponse.
func (ir *ImageResponse) WithStatusCode(statusCode int) *ImageResponse {
	tmp := *ir
	tmp.HTTPStatusCode = statusCode
	return &tmp
}

// Render renders the ImageResponse as an HTTP response.
func (ir *ImageResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(ir.HTTPStatusCode, http.StatusOK))
	return nil
}

// ImagesMeta is a meta information for ImagesResponse.
type ImagesMeta struct {
	Images int `json:"images" example:"1" format:"int64"`
}

// ImagesResponse is an object that holds a list of image information.
type ImagesResponse struct {
	Data []*models.Image `json:"data"`
	Meta ImagesMeta      `json:"meta"`
}

// NewImagesResponse creates a new ImagesResponse instance.
func NewImagesResponse(images []*models.Image, total int) *ImagesResponse {
	return &ImagesResponse{
		Data: images,
		Meta: ImagesMeta{Images: total},
	}
}

// Render renders the ImagesResponse as an HTTP response.
func (ir *ImagesResponse) Render(w http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
