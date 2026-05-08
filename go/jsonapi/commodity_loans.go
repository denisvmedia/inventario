package jsonapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

// CommodityLoanResponse is the JSON:API envelope for a single loan.
type CommodityLoanResponse struct {
	HTTPStatusCode int `json:"-"`

	ID         string               `json:"id"`
	Type       string               `json:"type" example:"commodity_loans" enums:"commodity_loans"`
	Attributes models.CommodityLoan `json:"attributes"`
}

func NewCommodityLoanResponse(loan *models.CommodityLoan) *CommodityLoanResponse {
	return &CommodityLoanResponse{ID: loan.ID, Type: "commodity_loans", Attributes: *loan}
}

func (lr *CommodityLoanResponse) WithStatusCode(code int) *CommodityLoanResponse {
	tmp := *lr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (lr *CommodityLoanResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(lr.HTTPStatusCode, http.StatusOK))
	return nil
}

// CommodityLoansMeta describes the pagination block on a list response.
type CommodityLoansMeta struct {
	Loans int `json:"loans" example:"10" format:"int64"`
	Total int `json:"total" example:"100" format:"int64"`
}

// CommodityLoansResponse is the paginated list shape.
type CommodityLoansResponse struct {
	Data []*models.CommodityLoan `json:"data"`
	Meta CommodityLoansMeta      `json:"meta"`
}

func NewCommodityLoansResponse(loans []*models.CommodityLoan, total int) *CommodityLoansResponse {
	return &CommodityLoansResponse{
		Data: loans,
		Meta: CommodityLoansMeta{Loans: len(loans), Total: total},
	}
}

func (*CommodityLoansResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityLoanRequest is the JSON:API payload for POST .../loans.
type CommodityLoanRequest struct {
	Data *CommodityLoanRequestDataWrapper `json:"data"`
}

type CommodityLoanRequestDataWrapper struct {
	ID         string                   `json:"id,omitempty"`
	Type       string                   `json:"type"`
	Attributes CommodityLoanRequestData `json:"attributes"`
}

// CommodityLoanRequestData carries the user-supplied fields on create.
type CommodityLoanRequestData struct {
	BorrowerName    string       `json:"borrower_name"`
	BorrowerContact string       `json:"borrower_contact,omitempty"`
	BorrowerNote    string       `json:"borrower_note,omitempty"`
	LentAt          models.Date  `json:"lent_at"`
	DueBackAt       models.PDate `json:"due_back_at,omitempty"`
}

func (lrd *CommodityLoanRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (lrd *CommodityLoanRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, lrd,
		validation.Field(&lrd.BorrowerName, validation.Required, validation.Length(1, 200)),
		validation.Field(&lrd.BorrowerContact, validation.Length(0, 200)),
		validation.Field(&lrd.BorrowerNote, validation.Length(0, 1000)),
		validation.Field(&lrd.LentAt, validation.Required),
	)
}

func (lrdw *CommodityLoanRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if lrdw.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, lrdw,
		validation.Field(&lrdw.Type, validation.Required, validation.In("commodity_loans")),
		validation.Field(&lrdw.Attributes, validation.Required),
	)
}

func (lr *CommodityLoanRequest) Bind(r *http.Request) error {
	return lr.ValidateWithContext(r.Context())
}

func (lr *CommodityLoanRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, lr,
		validation.Field(&lr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CommodityLoanRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityLoanRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityLoanRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = (*CommodityLoanRequestData)(nil)
)

// CommodityLoanUpdateRequest is the JSON:API payload for PATCH
// .../loans/{id}. All fields are optional; absent means
// "leave unchanged."
//
// `due_back_at` is tri-state on the wire (issue #1513):
//   - omitted — leave the value unchanged.
//   - null — clear the due date (open-ended loan).
//   - "YYYY-MM-DD" — replace the due date with the given value.
//
// Implementation note (BE-only, not part of the wire contract): a
// custom UnmarshalJSON populates a parallel ClearDueBackAt bool when
// the payload contains JSON null, since Go's encoding/json can't
// distinguish absent from null on a pointer field. Other nullable
// patch fields can fold into the same pattern when a clear path is
// needed; we don't pre-add tri-state on every field because the
// cost (a parallel bool per field) is real and the surface is small.
type CommodityLoanUpdateRequest struct {
	Data *CommodityLoanUpdateRequestDataWrapper `json:"data"`
}

type CommodityLoanUpdateRequestDataWrapper struct {
	ID         string                         `json:"id"`
	Type       string                         `json:"type"`
	Attributes CommodityLoanUpdateRequestData `json:"attributes"`
}

type CommodityLoanUpdateRequestData struct {
	BorrowerName    *string `json:"borrower_name,omitempty"`
	BorrowerContact *string `json:"borrower_contact,omitempty"`
	BorrowerNote    *string `json:"borrower_note,omitempty"`
	// DueBackAt: omitted leaves it unchanged; a "YYYY-MM-DD" string
	// replaces the value; an explicit JSON `null` clears the column
	// (open-ended loan).
	DueBackAt models.PDate `json:"due_back_at,omitempty" extensions:"x-nullable=true"`
	// ClearDueBackAt is filled by UnmarshalJSON when the wire payload
	// contained `"due_back_at": null`. Not a JSON field — the `-` tag
	// keeps it out of any response or OpenAPI schema generation.
	ClearDueBackAt bool `json:"-"`
}

// UnmarshalJSON decodes the patch attributes with presence detection
// for `due_back_at`. We buffer into a raw key map first so a literal
// JSON null on the wire flips ClearDueBackAt to true; an absent key
// leaves it false. Both cases produce a nil DueBackAt under the
// standard decode, which is why the parallel bool is necessary.
func (lurd *CommodityLoanUpdateRequestData) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	// type alias avoids infinite recursion on the structured decode.
	type alias CommodityLoanUpdateRequestData
	var a alias
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	*lurd = CommodityLoanUpdateRequestData(a)

	if v, ok := raw["due_back_at"]; ok && bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
		lurd.ClearDueBackAt = true
		lurd.DueBackAt = nil
	}
	return nil
}

func (lurd *CommodityLoanUpdateRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (lurd *CommodityLoanUpdateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0, 3)
	if lurd.BorrowerName != nil {
		fields = append(fields, validation.Field(lurd.BorrowerName, validation.Length(1, 200)))
	}
	if lurd.BorrowerContact != nil {
		fields = append(fields, validation.Field(lurd.BorrowerContact, validation.Length(0, 200)))
	}
	if lurd.BorrowerNote != nil {
		fields = append(fields, validation.Field(lurd.BorrowerNote, validation.Length(0, 1000)))
	}
	return validation.ValidateStructWithContext(ctx, lurd, fields...)
}

func (ludw *CommodityLoanUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, ludw,
		validation.Field(&ludw.Type, validation.Required, validation.In("commodity_loans")),
		validation.Field(&ludw.Attributes, validation.Required),
	)
}

func (lur *CommodityLoanUpdateRequest) Bind(r *http.Request) error {
	return lur.ValidateWithContext(r.Context())
}

func (lur *CommodityLoanUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, lur,
		validation.Field(&lur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CommodityLoanUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityLoanUpdateRequest)(nil)
)

// CommodityLoanReturnRequest is the JSON:API payload for POST
// .../loans/{id}/return. ReturnedAt is optional; absent means "today
// (server clock)".
type CommodityLoanReturnRequest struct {
	Data *CommodityLoanReturnRequestDataWrapper `json:"data,omitempty"`
}

type CommodityLoanReturnRequestDataWrapper struct {
	Type       string                         `json:"type"`
	Attributes CommodityLoanReturnRequestData `json:"attributes"`
}

type CommodityLoanReturnRequestData struct {
	ReturnedAt models.PDate `json:"returned_at,omitempty"`
}

func (lrr *CommodityLoanReturnRequest) Bind(r *http.Request) error {
	if lrr.Data == nil {
		// Empty body is allowed — the handler will default to "today".
		return nil
	}
	return validation.ValidateStructWithContext(r.Context(), lrr.Data,
		validation.Field(&lrr.Data.Type, validation.Required, validation.In("commodity_loans")),
	)
}

var _ render.Binder = (*CommodityLoanReturnRequest)(nil)

// LoanCommodityRef is a tiny, denormalised view of a commodity's
// public-facing identifiers — name + short_name + cover thumbnail —
// returned alongside loan rows in group-wide list endpoints so the FE
// can render a row "name → borrower" without a second round-trip per
// loan. Cover URL is intentionally omitted on the simple list to keep
// the payload light; clients that need it call the per-commodity
// endpoint.
type LoanCommodityRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name,omitempty"`
}

// CommodityLoanListItem is a single row in a paginated, group-wide loan
// list. It mirrors the project's "FLAT in data" envelope (loan fields
// hoisted to the top level via embedding) and adds an optional
// per-resource `commodity` block that summarises the lent item.
type CommodityLoanListItem struct {
	*models.CommodityLoan
	Commodity *LoanCommodityRef `json:"commodity,omitempty"`
}

// CommodityLoanListResponse is the paginated list shape with the
// per-row commodity ref attached. Used by the dedicated /loans
// endpoint; the per-commodity endpoint uses CommodityLoansResponse
// instead because the commodity is implicit.
type CommodityLoanListResponse struct {
	Data []*CommodityLoanListItem `json:"data"`
	Meta CommodityLoansMeta       `json:"meta"`
}

func NewCommodityLoanListResponse(loans []*models.CommodityLoan, total int, commoditiesByID map[string]*models.Commodity) *CommodityLoanListResponse {
	items := make([]*CommodityLoanListItem, 0, len(loans))
	for _, l := range loans {
		item := &CommodityLoanListItem{CommodityLoan: l}
		if c, ok := commoditiesByID[l.CommodityID]; ok && c != nil {
			item.Commodity = &LoanCommodityRef{
				ID:        c.ID,
				Name:      c.Name,
				ShortName: c.ShortName,
			}
		}
		items = append(items, item)
	}
	return &CommodityLoanListResponse{
		Data: items,
		Meta: CommodityLoansMeta{Loans: len(items), Total: total},
	}
}

func (*CommodityLoanListResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityLoanCountsResponse is the lightweight payload that backs the
// list-page "lent out" badge: a map of commodity_id → open-loan count.
// Empty input → empty map. Returned as a flat object so the FE can
// resolve `counts[id] ?? 0` without iterating an array.
type CommodityLoanCountsResponse struct {
	Data map[string]int `json:"data"`
}

func NewCommodityLoanCountsResponse(counts map[string]int) *CommodityLoanCountsResponse {
	return &CommodityLoanCountsResponse{Data: counts}
}

func (*CommodityLoanCountsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
