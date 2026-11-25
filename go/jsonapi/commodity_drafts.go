package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
)

// CommodityDraftView is a draft with the fields the server derives from it.
// Title is the trimmed name from the values, empty while the form has none.
// FileCount is the number of live files attached to the draft; the files
// themselves are read through GET .../files?linked_entity_type=draft.
type CommodityDraftView struct {
	*models.CommodityDraft
	Title     string `json:"title" example:"Cordless drill"`
	FileCount int    `json:"file_count" example:"2"`
}

// NewCommodityDraftView builds the view of d.
func NewCommodityDraftView(d *models.CommodityDraft, fileCount int) *CommodityDraftView {
	return &CommodityDraftView{CommodityDraft: d, Title: d.Values.Title(), FileCount: fileCount}
}

// CommodityDraftResponse is the JSON:API envelope for one draft.
type CommodityDraftResponse struct {
	HTTPStatusCode int                         `json:"-"`
	Data           *CommodityDraftResponseData `json:"data"`
}

// CommodityDraftResponseData is the inner resource object.
type CommodityDraftResponseData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type" example:"commodity_drafts" enums:"commodity_drafts"`
	Attributes CommodityDraftView `json:"attributes"`
}

func NewCommodityDraftResponse(d *models.CommodityDraft, fileCount int) *CommodityDraftResponse {
	return &CommodityDraftResponse{
		Data: &CommodityDraftResponseData{
			ID:         d.ID,
			Type:       "commodity_drafts",
			Attributes: *NewCommodityDraftView(d, fileCount),
		},
	}
}

func (cr *CommodityDraftResponse) WithStatusCode(code int) *CommodityDraftResponse {
	tmp := *cr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (cr *CommodityDraftResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(cr.HTTPStatusCode, http.StatusOK))
	return nil
}

// CommodityDraftsMeta is the count block on a list response.
type CommodityDraftsMeta struct {
	Drafts int `json:"drafts" example:"3" format:"int64"`
}

// CommodityDraftsResponse lists the open drafts of a group.
type CommodityDraftsResponse struct {
	Data []*CommodityDraftView `json:"data"`
	Meta CommodityDraftsMeta   `json:"meta"`
}

// NewCommodityDraftsResponse builds a list page; total is the number of open
// drafts in the group and fileCounts the live files per draft id.
func NewCommodityDraftsResponse(drafts []*models.CommodityDraft, total int, fileCounts map[string]int) *CommodityDraftsResponse {
	views := make([]*CommodityDraftView, 0, len(drafts))
	for _, d := range drafts {
		views = append(views, NewCommodityDraftView(d, fileCounts[d.ID]))
	}
	return &CommodityDraftsResponse{
		Data: views,
		Meta: CommodityDraftsMeta{Drafts: total},
	}
}

func (*CommodityDraftsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityDraftCreateRequest is the JSON:API payload for POST .../drafts.
type CommodityDraftCreateRequest struct {
	Data *CommodityDraftCreateRequestData `json:"data"`
}

// CommodityDraftCreateRequestData wraps the attributes of a new draft.
type CommodityDraftCreateRequestData struct {
	ID         string                         `json:"id,omitempty"`
	Type       string                         `json:"type" example:"commodity_drafts" enums:"commodity_drafts"`
	Attributes CommodityDraftCreateAttributes `json:"attributes"`
}

// CommodityDraftCreateAttributes carries the client key and the initial
// values. The client key is the client's identity for the draft: a create
// with a key the group already has returns that draft (200) instead of making
// another one (201).
type CommodityDraftCreateAttributes struct {
	ClientKey string                      `json:"client_key" example:"5f0c8a7e-3f43-4f1e-9a43-6a1fbb8f2c11"`
	Values    models.CommodityDraftValues `json:"values" swaggertype:"object"`
}

func (a CommodityDraftCreateAttributes) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (a CommodityDraftCreateAttributes) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &a,
		validation.Field(&a.ClientKey, validation.Required, validation.Length(1, models.CommodityDraftClientKeyMaxLength)),
	)
}

func (d *CommodityDraftCreateRequestData) ValidateWithContext(ctx context.Context) error {
	if d.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.Type, validation.Required, validation.In("commodity_drafts")),
		validation.Field(&d.Attributes),
	)
}

func (cr *CommodityDraftCreateRequest) Bind(r *http.Request) error {
	return cr.ValidateWithContext(r.Context())
}

func (cr *CommodityDraftCreateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, cr,
		validation.Field(&cr.Data, validation.Required),
	)
}

// CommodityDraftSaveRequest is the JSON:API payload for PUT .../drafts/{id}.
type CommodityDraftSaveRequest struct {
	Data *CommodityDraftSaveRequestData `json:"data"`
}

// CommodityDraftSaveRequestData wraps the attributes of a save.
type CommodityDraftSaveRequestData struct {
	ID         string                       `json:"id,omitempty"`
	Type       string                       `json:"type" example:"commodity_drafts" enums:"commodity_drafts"`
	Attributes CommodityDraftSaveAttributes `json:"attributes"`
}

// CommodityDraftSaveAttributes carries the values and the revision they were
// based on. A save whose revision is not the draft's current one is refused
// with 409, so a stale tab cannot overwrite newer input.
type CommodityDraftSaveAttributes struct {
	Revision int                         `json:"revision" example:"3"`
	Values   models.CommodityDraftValues `json:"values" swaggertype:"object"`
}

func (a CommodityDraftSaveAttributes) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (a CommodityDraftSaveAttributes) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &a,
		validation.Field(&a.Revision, validation.Required, validation.Min(1)),
	)
}

func (d *CommodityDraftSaveRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.Type, validation.Required, validation.In("commodity_drafts")),
		validation.Field(&d.Attributes),
	)
}

func (cr *CommodityDraftSaveRequest) Bind(r *http.Request) error {
	return cr.ValidateWithContext(r.Context())
}

func (cr *CommodityDraftSaveRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, cr,
		validation.Field(&cr.Data, validation.Required),
	)
}

// CommodityDraftPromoteRequest is the JSON:API payload for
// POST .../drafts/{id}/promote: the finished item, as for
// POST .../commodities, and the revision of the draft it finishes.
type CommodityDraftPromoteRequest struct {
	Data *CommodityData            `json:"data"`
	Meta CommodityDraftPromoteMeta `json:"meta"`
}

// CommodityDraftPromoteMeta names the draft revision the item was based on.
type CommodityDraftPromoteMeta struct {
	Revision int `json:"revision" example:"3"`
}

func (m CommodityDraftPromoteMeta) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (m CommodityDraftPromoteMeta) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &m,
		validation.Field(&m.Revision, validation.Required, validation.Min(1)),
	)
}

func (cr *CommodityDraftPromoteRequest) Bind(r *http.Request) error {
	// The item is new: an id in the payload is refused as in a create.
	ctx := context.WithValue(r.Context(), httpMethodKey, http.MethodPost)
	return cr.ValidateWithContext(ctx)
}

func (cr *CommodityDraftPromoteRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, cr,
		validation.Field(&cr.Data, validation.Required),
		validation.Field(&cr.Meta),
	)
}
