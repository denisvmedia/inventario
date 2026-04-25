package jsonapi

import (
	"errors"
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
	return nil
}

// BulkMoveRequest carries the IDs to move plus the destination area.
// Used by POST /commodities/bulk-move (#1330 PR 5.5).
type BulkMoveRequest struct {
	Data *BulkMoveRequestData `json:"data"`
}

// BulkMoveRequestData is the inner body for a BulkMoveRequest.
type BulkMoveRequestData struct {
	Type       string              `json:"type"`
	Attributes *BulkMoveAttributes `json:"attributes"`
}

// BulkMoveAttributes carries the bulk-move payload.
type BulkMoveAttributes struct {
	IDs    []string `json:"ids"`
	AreaID string   `json:"area_id"`
}

// Bind validates a BulkMoveRequest body.
func (r *BulkMoveRequest) Bind(_ *http.Request) error {
	if r.Data == nil || r.Data.Attributes == nil {
		return errors.New("missing data.attributes")
	}
	if len(r.Data.Attributes.IDs) == 0 {
		return errors.New("ids must not be empty")
	}
	if r.Data.Attributes.AreaID == "" {
		return errors.New("area_id is required")
	}
	return nil
}

// BulkResultResponse describes the outcome of a bulk action: which IDs
// succeeded and which failed (with reason). The endpoint always returns
// 200 with this body so the caller can render partial-failure UX
// without parsing per-id HTTP statuses.
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
	Succeeded []string         `json:"succeeded"`
	Failed    []BulkResultFail `json:"failed"`
}

// BulkResultFail describes one failure inside a bulk action result.
type BulkResultFail struct {
	ID    string `json:"id"`
	Error string `json:"error"`
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

// Helper to silence the linter when the render.Render result is
// ignored — handlers route errors through internalServerError() but
// some call-sites just want the side-effect.
var _ render.Renderer = (*BulkResultResponse)(nil)
