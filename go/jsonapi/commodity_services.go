package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
)

// CommodityServiceResponse is the JSON:API envelope for a single service row.
type CommodityServiceResponse struct {
	HTTPStatusCode int `json:"-"`

	ID         string                  `json:"id"`
	Type       string                  `json:"type" example:"commodity_services" enums:"commodity_services"`
	Attributes models.CommodityService `json:"attributes"`
}

func NewCommodityServiceResponse(svc *models.CommodityService) *CommodityServiceResponse {
	return &CommodityServiceResponse{ID: svc.ID, Type: "commodity_services", Attributes: *svc}
}

func (sr *CommodityServiceResponse) WithStatusCode(code int) *CommodityServiceResponse {
	tmp := *sr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (sr *CommodityServiceResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(sr.HTTPStatusCode, http.StatusOK))
	return nil
}

// CommodityServicesMeta describes the pagination block on a list response.
type CommodityServicesMeta struct {
	Services int `json:"services" example:"10" format:"int64"`
	Total    int `json:"total" example:"100" format:"int64"`
}

// CommodityServicesResponse is the per-commodity list shape (no commodity ref).
type CommodityServicesResponse struct {
	Data []*models.CommodityService `json:"data"`
	Meta CommodityServicesMeta      `json:"meta"`
}

func NewCommodityServicesResponse(services []*models.CommodityService, total int) *CommodityServicesResponse {
	return &CommodityServicesResponse{
		Data: services,
		Meta: CommodityServicesMeta{Services: len(services), Total: total},
	}
}

func (*CommodityServicesResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityServiceRequest is the JSON:API payload for POST .../services.
type CommodityServiceRequest struct {
	Data *CommodityServiceRequestDataWrapper `json:"data"`
}

type CommodityServiceRequestDataWrapper struct {
	ID         string                      `json:"id,omitempty"`
	Type       string                      `json:"type"`
	Attributes CommodityServiceRequestData `json:"attributes"`
}

// CommodityServiceRequestData carries the user-supplied fields on create.
type CommodityServiceRequestData struct {
	ProviderName     string           `json:"provider_name"`
	ProviderContact  string           `json:"provider_contact,omitempty"`
	Reason           string           `json:"reason,omitempty"`
	SentAt           models.Date      `json:"sent_at"`
	ExpectedReturnAt models.PDate     `json:"expected_return_at,omitempty"`
	CostAmount       *decimal.Decimal `json:"cost_amount,omitempty"`
	CostCurrency     string           `json:"cost_currency,omitempty"`
}

func (srd *CommodityServiceRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (srd *CommodityServiceRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, srd,
		validation.Field(&srd.ProviderName, validation.Required, validation.Length(1, 200)),
		validation.Field(&srd.ProviderContact, validation.Length(0, 200)),
		validation.Field(&srd.Reason, validation.Length(0, 1000)),
		validation.Field(&srd.SentAt, validation.Required),
		validation.Field(&srd.CostCurrency, validation.By(func(any) error {
			amountSet := srd.CostAmount != nil && !srd.CostAmount.IsZero()
			currencySet := srd.CostCurrency != ""
			if amountSet != currencySet {
				return validation.NewError("cost_currency_pair_required",
					"cost_amount and cost_currency must be set together")
			}
			return nil
		})),
	)
}

func (srdw *CommodityServiceRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if srdw.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, srdw,
		validation.Field(&srdw.Type, validation.Required, validation.In("commodity_services")),
		validation.Field(&srdw.Attributes, validation.Required),
	)
}

func (sr *CommodityServiceRequest) Bind(r *http.Request) error {
	return sr.ValidateWithContext(r.Context())
}

func (sr *CommodityServiceRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sr,
		validation.Field(&sr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CommodityServiceRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityServiceRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityServiceRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = (*CommodityServiceRequestData)(nil)
)

// CommodityServiceUpdateRequest is the JSON:API payload for PATCH
// .../services/{id}. All fields are optional; nil pointer / absent means
// "leave unchanged." Same expected_return_at "cannot clear via PATCH"
// caveat as CommodityLoanUpdateRequest — see that type for the reasoning.
//
// Cost fields are paired: caller must set BOTH or NEITHER on a single
// patch call.
type CommodityServiceUpdateRequest struct {
	Data *CommodityServiceUpdateRequestDataWrapper `json:"data"`
}

type CommodityServiceUpdateRequestDataWrapper struct {
	ID         string                            `json:"id"`
	Type       string                            `json:"type"`
	Attributes CommodityServiceUpdateRequestData `json:"attributes"`
}

type CommodityServiceUpdateRequestData struct {
	ProviderName     *string          `json:"provider_name,omitempty"`
	ProviderContact  *string          `json:"provider_contact,omitempty"`
	Reason           *string          `json:"reason,omitempty"`
	ExpectedReturnAt models.PDate     `json:"expected_return_at,omitempty"`
	CostAmount       *decimal.Decimal `json:"cost_amount,omitempty"`
	CostCurrency     *string          `json:"cost_currency,omitempty"`
}

func (surd *CommodityServiceUpdateRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (surd *CommodityServiceUpdateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0, 5)
	if surd.ProviderName != nil {
		fields = append(fields, validation.Field(surd.ProviderName, validation.Length(1, 200)))
	}
	if surd.ProviderContact != nil {
		fields = append(fields, validation.Field(surd.ProviderContact, validation.Length(0, 200)))
	}
	if surd.Reason != nil {
		fields = append(fields, validation.Field(surd.Reason, validation.Length(0, 1000)))
	}
	// Pair gate for cost: BOTH or NEITHER on this PATCH.
	fields = append(fields, validation.Field(&surd.CostCurrency, validation.By(func(any) error {
		amountSet := surd.CostAmount != nil
		currencySet := surd.CostCurrency != nil
		if amountSet != currencySet {
			return validation.NewError("cost_currency_pair_required",
				"cost_amount and cost_currency must be patched together")
		}
		return nil
	})))
	return validation.ValidateStructWithContext(ctx, surd, fields...)
}

func (sudw *CommodityServiceUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sudw,
		validation.Field(&sudw.Type, validation.Required, validation.In("commodity_services")),
		validation.Field(&sudw.Attributes, validation.Required),
	)
}

func (sur *CommodityServiceUpdateRequest) Bind(r *http.Request) error {
	return sur.ValidateWithContext(r.Context())
}

func (sur *CommodityServiceUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sur,
		validation.Field(&sur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CommodityServiceUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityServiceUpdateRequest)(nil)
)

// CommodityServiceReturnRequest is the JSON:API payload for POST
// .../services/{id}/return. ReturnedAt is optional (defaults to today,
// server clock). Optional final-cost fields let the caller record the
// repair bill on the same call.
type CommodityServiceReturnRequest struct {
	Data *CommodityServiceReturnRequestDataWrapper `json:"data,omitempty"`
}

type CommodityServiceReturnRequestDataWrapper struct {
	Type       string                            `json:"type"`
	Attributes CommodityServiceReturnRequestData `json:"attributes"`
}

type CommodityServiceReturnRequestData struct {
	ReturnedAt   models.PDate     `json:"returned_at,omitempty"`
	CostAmount   *decimal.Decimal `json:"cost_amount,omitempty"`
	CostCurrency *string          `json:"cost_currency,omitempty"`
}

func (srr *CommodityServiceReturnRequest) Bind(r *http.Request) error {
	if srr.Data == nil {
		// Empty body is allowed — the handler defaults returned_at to today.
		return nil
	}
	return validation.ValidateStructWithContext(r.Context(), srr.Data,
		validation.Field(&srr.Data.Type, validation.Required, validation.In("commodity_services")),
	)
}

var _ render.Binder = (*CommodityServiceReturnRequest)(nil)

// ServiceCommodityRef is the denormalised commodity-summary block
// returned alongside service rows on the dedicated /services list.
// Mirrors LoanCommodityRef.
type ServiceCommodityRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name,omitempty"`
}

// CommodityServiceListItem is a single row in a paginated, group-wide
// service list. Mirrors CommodityLoanListItem.
type CommodityServiceListItem struct {
	*models.CommodityService
	Commodity *ServiceCommodityRef `json:"commodity,omitempty"`
}

// CommodityServiceListResponse is the paginated group-wide list shape.
type CommodityServiceListResponse struct {
	Data []*CommodityServiceListItem `json:"data"`
	Meta CommodityServicesMeta       `json:"meta"`
}

func NewCommodityServiceListResponse(services []*models.CommodityService, total int, commoditiesByID map[string]*models.Commodity) *CommodityServiceListResponse {
	items := make([]*CommodityServiceListItem, 0, len(services))
	for _, s := range services {
		item := &CommodityServiceListItem{CommodityService: s}
		if c, ok := commoditiesByID[s.CommodityID]; ok && c != nil {
			item.Commodity = &ServiceCommodityRef{
				ID:        c.ID,
				Name:      c.Name,
				ShortName: c.ShortName,
			}
		}
		items = append(items, item)
	}
	return &CommodityServiceListResponse{
		Data: items,
		Meta: CommodityServicesMeta{Services: len(items), Total: total},
	}
}

func (*CommodityServiceListResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityServiceCountsResponse is the lightweight per-commodity
// open-service-count payload that backs the list-page "in service" badge.
type CommodityServiceCountsResponse struct {
	Data map[string]int `json:"data"`
}

func NewCommodityServiceCountsResponse(counts map[string]int) *CommodityServiceCountsResponse {
	return &CommodityServiceCountsResponse{Data: counts}
}

func (*CommodityServiceCountsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
