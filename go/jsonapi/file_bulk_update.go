package jsonapi

import (
	"errors"
	"fmt"
	"net/http"
)

// FileBulkUpdateRequest is the body of POST /files/bulk-update: the files to
// change and the change to make to each of them.
type FileBulkUpdateRequest struct {
	Data *FileBulkUpdateRequestData `json:"data"`
}

// FileBulkUpdateRequestData is the inner body for a FileBulkUpdateRequest.
type FileBulkUpdateRequestData struct {
	Type       string                    `json:"type" example:"files"`
	Attributes *FileBulkUpdateAttributes `json:"attributes"`
}

// FileBulkUpdateAttributes carries the file ids (at most 100) and the patch.
type FileBulkUpdateAttributes struct {
	IDs   []string       `json:"ids"`
	Patch *FileBulkPatch `json:"patch"`
}

// FileBulkPatch is the change a bulk update makes to every selected file.
// Tags are added and removed, never replaced, so tags the patch does not
// name stay on each file.
type FileBulkPatch struct {
	Tags *BulkTagsPatch `json:"tags"`
}

// Bind validates the shape of a FileBulkUpdateRequest body. The tags are
// checked by the bulk service.
func (r *FileBulkUpdateRequest) Bind(_ *http.Request) error {
	if r.Data == nil || r.Data.Attributes == nil {
		return errors.New("missing data.attributes")
	}
	a := r.Data.Attributes
	if len(a.IDs) == 0 {
		return errors.New("ids must not be empty")
	}
	if len(a.IDs) > MaxBulkIDs {
		return fmt.Errorf("ids must not exceed %d entries", MaxBulkIDs)
	}
	if a.Patch == nil || a.Patch.Tags == nil {
		return errors.New("patch.tags is required")
	}
	return nil
}
