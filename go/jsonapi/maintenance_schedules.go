package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
)

// MaintenanceScheduleResponse is the JSON:API envelope for a single
// schedule. Mirrors CommodityLoanResponse — wraps the resource in
// `{data: {id, type, attributes}}`.
type MaintenanceScheduleResponse struct {
	HTTPStatusCode int                              `json:"-"`
	Data           *MaintenanceScheduleResponseData `json:"data"`
}

// MaintenanceScheduleResponseData is the inner resource object.
type MaintenanceScheduleResponseData struct {
	ID         string                     `json:"id"`
	Type       string                     `json:"type" example:"maintenance_schedules" enums:"maintenance_schedules"`
	Attributes models.MaintenanceSchedule `json:"attributes"`
}

func NewMaintenanceScheduleResponse(schedule *models.MaintenanceSchedule) *MaintenanceScheduleResponse {
	return &MaintenanceScheduleResponse{
		Data: &MaintenanceScheduleResponseData{
			ID:         schedule.ID,
			Type:       "maintenance_schedules",
			Attributes: *schedule,
		},
	}
}

func (sr *MaintenanceScheduleResponse) WithStatusCode(code int) *MaintenanceScheduleResponse {
	tmp := *sr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (sr *MaintenanceScheduleResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(sr.HTTPStatusCode, http.StatusOK))
	return nil
}

// MaintenanceSchedulesMeta is the pagination block on a list response.
type MaintenanceSchedulesMeta struct {
	Schedules int `json:"schedules" example:"10" format:"int64"`
	Total     int `json:"total" example:"100" format:"int64"`
}

// MaintenanceSchedulesResponse is the per-commodity list shape (no
// commodity ref needed — the commodity is implicit in the URL).
type MaintenanceSchedulesResponse struct {
	Data []*models.MaintenanceSchedule `json:"data"`
	Meta MaintenanceSchedulesMeta      `json:"meta"`
}

func NewMaintenanceSchedulesResponse(schedules []*models.MaintenanceSchedule, total int) *MaintenanceSchedulesResponse {
	return &MaintenanceSchedulesResponse{
		Data: schedules,
		Meta: MaintenanceSchedulesMeta{Schedules: len(schedules), Total: total},
	}
}

func (*MaintenanceSchedulesResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// MaintenanceCommodityRef is a tiny, denormalised view of a commodity
// — name + short_name — returned alongside schedule rows in group-wide
// list endpoints so the FE can render "name → next due" without a
// second round-trip per row.
type MaintenanceCommodityRef struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	ShortName string `json:"short_name,omitempty"`
}

// MaintenanceScheduleListItem is a single row in a paginated,
// group-wide list of schedules with the optional per-row commodity
// block. Mirrors CommodityLoanListItem.
type MaintenanceScheduleListItem struct {
	*models.MaintenanceSchedule
	Commodity *MaintenanceCommodityRef `json:"commodity,omitempty"`
}

// MaintenanceScheduleListResponse is the group-wide paginated list
// shape used by `/g/{slug}/maintenance`.
type MaintenanceScheduleListResponse struct {
	Data []*MaintenanceScheduleListItem `json:"data"`
	Meta MaintenanceSchedulesMeta       `json:"meta"`
}

func NewMaintenanceScheduleListResponse(schedules []*models.MaintenanceSchedule, total int, commoditiesByID map[string]*models.Commodity) *MaintenanceScheduleListResponse {
	items := make([]*MaintenanceScheduleListItem, 0, len(schedules))
	for _, s := range schedules {
		item := &MaintenanceScheduleListItem{MaintenanceSchedule: s}
		if c, ok := commoditiesByID[s.CommodityID]; ok && c != nil {
			item.Commodity = &MaintenanceCommodityRef{
				ID:        c.ID,
				Name:      c.Name,
				ShortName: c.ShortName,
			}
		}
		items = append(items, item)
	}
	return &MaintenanceScheduleListResponse{
		Data: items,
		Meta: MaintenanceSchedulesMeta{Schedules: len(items), Total: total},
	}
}

func (*MaintenanceScheduleListResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// MaintenanceScheduleRequest is the JSON:API payload for POST
// .../commodities/{id}/maintenance.
type MaintenanceScheduleRequest struct {
	Data *MaintenanceScheduleRequestDataWrapper `json:"data"`
}

type MaintenanceScheduleRequestDataWrapper struct {
	ID         string                         `json:"id,omitempty"`
	Type       string                         `json:"type"`
	Attributes MaintenanceScheduleRequestData `json:"attributes"`
}

// MaintenanceScheduleRequestData carries the user-supplied fields on
// create. NextDueAt is optional — when omitted the service defaults it
// to `today + interval_days` so the user can just say "every 90 days"
// without picking a start date.
type MaintenanceScheduleRequestData struct {
	Title        string       `json:"title"`
	IntervalDays int          `json:"interval_days"`
	NextDueAt    models.Date  `json:"next_due_at,omitempty"`
	LastDoneAt   models.PDate `json:"last_done_at,omitempty"`
	Notes        string       `json:"notes,omitempty"`
	// Enabled defaults to true at the BE when omitted (the model
	// has a `default="true"` migrator tag); the request field is a
	// pointer so the BE can tell "user explicitly chose true" from
	// "user omitted the field". The handler folds nil → true.
	Enabled *bool `json:"enabled,omitempty"`
}

func (mrd *MaintenanceScheduleRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (mrd *MaintenanceScheduleRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, mrd,
		validation.Field(&mrd.Title, validation.Required, validation.Length(1, 200)),
		validation.Field(&mrd.IntervalDays, validation.Required, validation.Min(1), validation.Max(36500)),
		validation.Field(&mrd.NextDueAt),
		validation.Field(&mrd.LastDoneAt),
		validation.Field(&mrd.Notes, validation.Length(0, 1000)),
	)
}

func (mrdw *MaintenanceScheduleRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if mrdw.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, mrdw,
		validation.Field(&mrdw.Type, validation.Required, validation.In("maintenance_schedules")),
		validation.Field(&mrdw.Attributes, validation.Required),
	)
}

func (mr *MaintenanceScheduleRequest) Bind(r *http.Request) error {
	return mr.ValidateWithContext(r.Context())
}

func (mr *MaintenanceScheduleRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, mr,
		validation.Field(&mr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*MaintenanceScheduleRequest)(nil)
	_ validation.ValidatableWithContext = (*MaintenanceScheduleRequest)(nil)
	_ validation.ValidatableWithContext = (*MaintenanceScheduleRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = (*MaintenanceScheduleRequestData)(nil)
)

// MaintenanceScheduleUpdateRequest is the JSON:API payload for PATCH
// .../maintenance/{id}. All fields are optional; absent means
// "leave unchanged."
type MaintenanceScheduleUpdateRequest struct {
	Data *MaintenanceScheduleUpdateRequestDataWrapper `json:"data"`
}

type MaintenanceScheduleUpdateRequestDataWrapper struct {
	ID         string                               `json:"id"`
	Type       string                               `json:"type"`
	Attributes MaintenanceScheduleUpdateRequestData `json:"attributes"`
}

type MaintenanceScheduleUpdateRequestData struct {
	Title        *string      `json:"title,omitempty"`
	IntervalDays *int         `json:"interval_days,omitempty"`
	NextDueAt    *models.Date `json:"next_due_at,omitempty"`
	LastDoneAt   models.PDate `json:"last_done_at,omitempty"`
	Notes        *string      `json:"notes,omitempty"`
	Enabled      *bool        `json:"enabled,omitempty"`
}

func (murd *MaintenanceScheduleUpdateRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (murd *MaintenanceScheduleUpdateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0, 4)
	if murd.Title != nil {
		fields = append(fields, validation.Field(murd.Title, validation.Length(1, 200)))
	}
	if murd.IntervalDays != nil {
		fields = append(fields, validation.Field(murd.IntervalDays, validation.Min(1), validation.Max(36500)))
	}
	if murd.Notes != nil {
		fields = append(fields, validation.Field(murd.Notes, validation.Length(0, 1000)))
	}
	return validation.ValidateStructWithContext(ctx, murd, fields...)
}

func (mudw *MaintenanceScheduleUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, mudw,
		validation.Field(&mudw.Type, validation.Required, validation.In("maintenance_schedules")),
		validation.Field(&mudw.Attributes, validation.Required),
	)
}

func (mur *MaintenanceScheduleUpdateRequest) Bind(r *http.Request) error {
	return mur.ValidateWithContext(r.Context())
}

func (mur *MaintenanceScheduleUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, mur,
		validation.Field(&mur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*MaintenanceScheduleUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*MaintenanceScheduleUpdateRequest)(nil)
)

// MaintenanceScheduleDoneRequest is the JSON:API payload for POST
// .../maintenance/{id}/done. DoneAt is optional; absent means "today
// (server clock)".
type MaintenanceScheduleDoneRequest struct {
	Data *MaintenanceScheduleDoneRequestDataWrapper `json:"data,omitempty"`
}

type MaintenanceScheduleDoneRequestDataWrapper struct {
	Type       string                             `json:"type"`
	Attributes MaintenanceScheduleDoneRequestData `json:"attributes"`
}

type MaintenanceScheduleDoneRequestData struct {
	DoneAt models.PDate `json:"done_at,omitempty"`
}

func (mdr *MaintenanceScheduleDoneRequest) Bind(r *http.Request) error {
	if mdr.Data == nil {
		// Empty body is allowed — the handler will default to today.
		return nil
	}
	return validation.ValidateStructWithContext(r.Context(), mdr.Data,
		validation.Field(&mdr.Data.Type, validation.Required, validation.In("maintenance_schedules")),
	)
}

var _ render.Binder = (*MaintenanceScheduleDoneRequest)(nil)
