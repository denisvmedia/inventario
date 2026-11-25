package jsonapi

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/go-chi/render"
)

// BulkIDsRequest is the body shape used by every "bulk action" endpoint
// that targets a list of entity IDs (#1330 PR 5.5: bulk delete on
// commodities + files). The wrapper mirrors json-api's `{data:{...}}`
// envelope so the request bodies are uniform with the rest of the API.
type BulkIDsRequest struct {
	Data *BulkIDsRequestData `json:"data"`
}

// BulkIDsRequestData is the inner body for a BulkIDsRequest.
type BulkIDsRequestData struct {
	// Type is the entity collection to act on (e.g. "commodities", "files").
	Type string `json:"type"`
	// Attributes carries the actual IDs payload.
	Attributes *BulkIDsAttributes `json:"attributes"`
}

// BulkIDsAttributes carries the identifiers a bulk action targets.
type BulkIDsAttributes struct {
	IDs []string `json:"ids"`
}

// MaxBulkIDs caps a single bulk action. Each id costs at least one
// round-trip, and the handlers loop without a deadline, so an unbounded list
// turned one request into an arbitrarily long blocking one (#2131). 100 is
// the page size the UI works in, so nothing a user can select is refused.
const MaxBulkIDs = 100

// Bind validates a BulkIDsRequest body. The implementation matches the
// other Bind() implementations in this package: do the cheap shape
// checks here so the handler can rely on a populated payload.
func (r *BulkIDsRequest) Bind(_ *http.Request) error {
	if r.Data == nil || r.Data.Attributes == nil {
		return errors.New("missing data.attributes")
	}
	if len(r.Data.Attributes.IDs) == 0 {
		return errors.New("ids must not be empty")
	}
	if len(r.Data.Attributes.IDs) > MaxBulkIDs {
		return fmt.Errorf("ids must not exceed %d entries", MaxBulkIDs)
	}
	return nil
}

// CommodityBulkSelection names the items a bulk action on items targets:
// the items listed in IDs, or every live item the query in Query selects.
// Exactly one of the two is given.
//
// Query is a query string of the item query contract
// (devdocs/item-query-contract.md), without pagination. ExpectedCount and
// ExpectedDigest are the number of items the caller saw the query select
// and the meta.selection_digest the items list returned for it, copied
// unchanged. When the query selects more than 500 items by the time the
// request runs, it fails with 422 bulk.too_many; when it selects other items
// within that limit, more, fewer or the same number with one swapped, it
// fails with 409. Either way nothing changes, so an action the user
// confirmed never reaches items they did not see.
type CommodityBulkSelection struct {
	IDs            []string `json:"ids,omitempty"`
	Query          *string  `json:"query,omitempty" example:"status=in_use&tags=garage&include_inactive=false"`
	ExpectedCount  *int     `json:"expected_count,omitempty" example:"37"`
	ExpectedDigest *string  `json:"expected_digest,omitempty" example:"1eb7c54d52831bbfe8942af0b1c56b7409523a59ed6ca99c1174fef7eb32c1b5"`
}

// validate checks the shape of the selection; resolving the query is the
// handler's job.
func (s *CommodityBulkSelection) validate() error {
	hasIDs := len(s.IDs) > 0
	hasQuery := s.Query != nil
	switch {
	case hasIDs && hasQuery:
		return errors.New("give either ids or query, not both")
	case !hasIDs && !hasQuery:
		return errors.New("ids must not be empty")
	case hasIDs && len(s.IDs) > MaxBulkIDs:
		return fmt.Errorf("ids must not exceed %d entries", MaxBulkIDs)
	case hasIDs && (s.ExpectedCount != nil || s.ExpectedDigest != nil):
		return errors.New("expected_count and expected_digest apply to query only")
	case hasQuery && s.ExpectedCount == nil:
		return errors.New("expected_count is required with query")
	case hasQuery && *s.ExpectedCount < 1:
		return errors.New("expected_count must be at least 1")
	case hasQuery && (s.ExpectedDigest == nil || *s.ExpectedDigest == ""):
		return errors.New("expected_digest is required with query")
	}
	return nil
}

// CommodityBulkDeleteRequest is the body of POST /commodities/bulk-delete.
type CommodityBulkDeleteRequest struct {
	Data *CommodityBulkDeleteRequestData `json:"data"`
}

// CommodityBulkDeleteRequestData is the inner body for a
// CommodityBulkDeleteRequest.
type CommodityBulkDeleteRequestData struct {
	Type       string                  `json:"type" example:"commodities"`
	Attributes *CommodityBulkSelection `json:"attributes"`
}

// Bind validates a CommodityBulkDeleteRequest body.
func (r *CommodityBulkDeleteRequest) Bind(_ *http.Request) error {
	if r.Data == nil || r.Data.Attributes == nil {
		return errors.New("missing data.attributes")
	}
	return r.Data.Attributes.validate()
}

// BulkMoveRequest carries the items to move plus the destination area.
// Used by POST /commodities/bulk-move (#1330 PR 5.5).
type BulkMoveRequest struct {
	Data *BulkMoveRequestData `json:"data"`
}

// BulkMoveRequestData is the inner body for a BulkMoveRequest.
type BulkMoveRequestData struct {
	Type       string              `json:"type"`
	Attributes *BulkMoveAttributes `json:"attributes"`
}

// BulkMoveAttributes carries the bulk-move payload: the items, as ids or a
// query, and the destination area.
type BulkMoveAttributes struct {
	CommodityBulkSelection
	AreaID string `json:"area_id"`
}

// Bind validates a BulkMoveRequest body.
func (r *BulkMoveRequest) Bind(_ *http.Request) error {
	if r.Data == nil || r.Data.Attributes == nil {
		return errors.New("missing data.attributes")
	}
	if err := r.Data.Attributes.validate(); err != nil {
		return err
	}
	if r.Data.Attributes.AreaID == "" {
		return errors.New("area_id is required")
	}
	return nil
}

// BulkResultResponse describes the outcome of a bulk action: which IDs
// succeeded, which were left as they were and which failed (with reason).
// The endpoint always returns 200 with this body so the caller can render
// partial-failure UX without parsing per-id HTTP statuses.
type BulkResultResponse struct {
	Data *BulkResultData `json:"data"`
}

// BulkResultData carries the per-id outcome of a bulk action.
type BulkResultData struct {
	Type       string           `json:"type"`
	ID         string           `json:"id"`
	Attributes *BulkResultAttrs `json:"attributes"`
}

// BulkResultAttrs is the attributes payload of BulkResultResponse.
type BulkResultAttrs struct {
	Succeeded []string `json:"succeeded"`
	// Skipped lists the ids the action left as they were because there was
	// nothing to change. Absent when there are none.
	Skipped []BulkResultSkip `json:"skipped,omitempty"`
	Failed  []BulkResultFail `json:"failed"`
	// Notes adds to succeeded ids something the caller should know. Absent
	// when there is nothing to add.
	Notes []BulkResultNote `json:"notes,omitempty"`
}

// BulkResultFail describes one failure inside a bulk action result.
type BulkResultFail struct {
	ID string `json:"id"`
	// Name is the item's name when the item could be read.
	Name  string `json:"name,omitempty"`
	Error string `json:"error"`
	// Code classifies the failure: not_found (the item does not exist or is
	// in Trash), invalid (the change does not pass the item's validation;
	// Error says why) or failed (anything else).
	Code string `json:"code,omitempty" enums:"not_found,invalid,failed"`
	// Field and Reason come with invalid: the field the validation refused
	// and the validator's code for why (for example short_name and
	// validation_required), so a client can say it in the user's language.
	// Error is the English message.
	Field  string `json:"field,omitempty" example:"short_name"`
	Reason string `json:"reason,omitempty" example:"validation_required"`
}

// BulkResultNote is something to know about an id the action changed.
type BulkResultNote struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// Code is files_kept: the item went to Trash, but moving its files there
	// failed, so some of them stayed live.
	Code string `json:"code" enums:"files_kept"`
}

// BulkResultSkip describes one item a bulk action left unchanged.
type BulkResultSkip struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
	// Reason is unchanged: the item already had what the action sets.
	Reason string `json:"reason" enums:"unchanged"`
}

// Render satisfies the render.Renderer interface.
func (*BulkResultResponse) Render(_w http.ResponseWriter, _r *http.Request) error {
	return nil
}

// NewBulkResultResponse wraps a {Succeeded, Failed} pair into the
// JSON:API envelope. Compile-time guarantee that handlers use the
// canonical envelope shape.
func NewBulkResultResponse(entityType string, succeeded []string, failed []BulkResultFail) *BulkResultResponse {
	if succeeded == nil {
		succeeded = []string{}
	}
	if failed == nil {
		failed = []BulkResultFail{}
	}
	return &BulkResultResponse{
		Data: &BulkResultData{
			Type: entityType + ".bulk_result",
			ID:   "bulk-result",
			Attributes: &BulkResultAttrs{
				Succeeded: succeeded,
				Failed:    failed,
			},
		},
	}
}

// WithSkipped sets the ids the action left unchanged.
func (r *BulkResultResponse) WithSkipped(skipped []BulkResultSkip) *BulkResultResponse {
	r.Data.Attributes.Skipped = skipped
	return r
}

// WithNotes sets the notes on succeeded ids.
func (r *BulkResultResponse) WithNotes(notes []BulkResultNote) *BulkResultResponse {
	r.Data.Attributes.Notes = notes
	return r
}

// Helper to silence the linter when the render.Render result is
// ignored — handlers route errors through internalServerError() but
// some call-sites just want the side-effect.
var _ render.Renderer = (*BulkResultResponse)(nil)
