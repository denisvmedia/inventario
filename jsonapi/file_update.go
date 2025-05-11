// jsonapi/file_update.go

package jsonapi

import (
	"errors"
	"net/http"
)

// FileUpdateRequest is the request body for updating a file's path.
type FileUpdateRequest struct {
	Data struct {
		ID         string           `json:"id"`
		Type       string           `json:"type" example:"images" enums:"images,manuals,invoices"`
		Attributes FileUpdateParams `json:"attributes"`
	} `json:"data"`
}

// FileUpdateParams contains the parameters for updating a file.
type FileUpdateParams struct {
	Path string `json:"path"` // Only the Path field can be updated
}

// Bind validates and binds the request.
func (fr *FileUpdateRequest) Bind(r *http.Request) error {
	if fr.Data.ID == "" {
		return errors.New("missing id")
	}

	if fr.Data.Type == "" {
		return errors.New("missing type")
	}

	if fr.Data.Type != "images" && fr.Data.Type != "manuals" && fr.Data.Type != "invoices" {
		return errors.New("invalid type")
	}

	if fr.Data.Attributes.Path == "" {
		return errors.New("path cannot be empty")
	}

	return nil
}
