// jsonapi/file_update.go

package jsonapi

import (
	"errors"
	"net/http"
)

// CommodityFileUpdateRequest is the request body for updating a commodity file's path.
type CommodityFileUpdateRequest struct {
	Data struct {
		ID         string                    `json:"id"`
		Type       string                    `json:"type" example:"images" enums:"images,manuals,invoices"`
		Attributes CommodityFileUpdateParams `json:"attributes"`
	} `json:"data"`
}

// CommodityFileUpdateParams contains the parameters for updating a commodity file.
type CommodityFileUpdateParams struct {
	Path string `json:"path"` // Only the Path field can be updated
}

// Bind validates and binds the request.
func (cfur *CommodityFileUpdateRequest) Bind(_r *http.Request) error {
	if cfur.Data.ID == "" {
		return errors.New("missing id")
	}

	if cfur.Data.Type == "" {
		return errors.New("missing type")
	}

	if cfur.Data.Type != "images" && cfur.Data.Type != "manuals" && cfur.Data.Type != "invoices" {
		return errors.New("invalid type")
	}

	if cfur.Data.Attributes.Path == "" {
		return errors.New("path cannot be empty")
	}

	return nil
}
