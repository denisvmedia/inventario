package jsonapi

import (
	"errors"
	"net/http"
)

// CommodityBulkUpdateRequest is the body of POST /commodities/bulk-update:
// the items to change, as ids or a query (CommodityBulkSelection), and the
// change to make to each of them.
type CommodityBulkUpdateRequest struct {
	Data *CommodityBulkUpdateRequestData `json:"data"`
}

// CommodityBulkUpdateRequestData is the inner body for a
// CommodityBulkUpdateRequest.
type CommodityBulkUpdateRequestData struct {
	Type       string                         `json:"type" example:"commodities"`
	Attributes *CommodityBulkUpdateAttributes `json:"attributes"`
}

// CommodityBulkUpdateAttributes carries the selection and the patch.
type CommodityBulkUpdateAttributes struct {
	CommodityBulkSelection
	Patch *CommodityBulkPatch `json:"patch"`
}

// CommodityBulkPatch is the change a bulk update makes to every selected
// item. A member that is absent leaves that part of the items as it is.
// Tags are added and removed, never replaced, so tags the patch does not
// name stay on each item.
type CommodityBulkPatch struct {
	Tags *CommodityBulkTagsPatch `json:"tags,omitempty"`
	// Type sets the item type.
	Type *string `json:"type,omitempty" example:"electronics"`
	// Status changes the status, with the date and note a single status
	// change records.
	Status *CommodityBulkStatusPatch `json:"status,omitempty"`
}

// CommodityBulkTagsPatch adds and removes tags. Add takes tag slugs or the
// text of a new tag, which is created as on a single item; Remove takes
// slugs. A tag cannot be in both lists.
type CommodityBulkTagsPatch struct {
	Add    []string `json:"add,omitempty"`
	Remove []string `json:"remove,omitempty"`
}

// CommodityBulkStatusPatch sets the status. Value in_use clears the status
// date and note. Any other status but sold needs Date (YYYY-MM-DD) and takes
// an optional Note of at most 1024 characters. sold is refused: it needs a
// sale price of its own for each item.
type CommodityBulkStatusPatch struct {
	Value string `json:"value" example:"disposed" enums:"in_use,lost,disposed,written_off"`
	Date  string `json:"date,omitempty" example:"2026-11-25"`
	Note  string `json:"note,omitempty"`
}

// Bind validates the shape of a CommodityBulkUpdateRequest body. The values
// in the patch are checked by the bulk service, which knows the item rules.
func (r *CommodityBulkUpdateRequest) Bind(_ *http.Request) error {
	if r.Data == nil || r.Data.Attributes == nil {
		return errors.New("missing data.attributes")
	}
	if err := r.Data.Attributes.validate(); err != nil {
		return err
	}
	p := r.Data.Attributes.Patch
	if p == nil {
		return errors.New("patch is required")
	}
	if p.Tags == nil && p.Type == nil && p.Status == nil {
		return errors.New("patch must change tags, type or status")
	}
	if p.Tags != nil && len(p.Tags.Add) == 0 && len(p.Tags.Remove) == 0 {
		return errors.New("patch.tags must add or remove at least one tag")
	}
	return nil
}
