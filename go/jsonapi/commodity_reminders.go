package jsonapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
)

// CommodityReminderView is a reminder with the fields the server derives
// from it. NextOccurrence is the first occurrence on or after today; it is
// empty for a one-off reminder whose date has passed. DaysUntilNext is the
// whole days until NextOccurrence and is omitted with it.
type CommodityReminderView struct {
	*models.CommodityReminder
	NextOccurrence string `json:"next_occurrence,omitempty" format:"date" example:"2026-12-24"`
	DaysUntilNext  *int   `json:"days_until_next,omitempty"`
}

// NewCommodityReminderView derives the view fields of r at now.
func NewCommodityReminderView(r *models.CommodityReminder, now time.Time) *CommodityReminderView {
	view := &CommodityReminderView{CommodityReminder: r}
	if next, ok := r.NextOccurrence(now); ok {
		view.NextOccurrence = string(next)
		view.DaysUntilNext = new(models.DaysUntil(next, now))
	}
	return view
}

// CommodityReminderResponse is the JSON:API envelope for one reminder.
type CommodityReminderResponse struct {
	HTTPStatusCode int                            `json:"-"`
	Data           *CommodityReminderResponseData `json:"data"`
}

// CommodityReminderResponseData is the inner resource object.
type CommodityReminderResponseData struct {
	ID         string                `json:"id"`
	Type       string                `json:"type" example:"commodity_reminders" enums:"commodity_reminders"`
	Attributes CommodityReminderView `json:"attributes"`
}

func NewCommodityReminderResponse(r *models.CommodityReminder, now time.Time) *CommodityReminderResponse {
	return &CommodityReminderResponse{
		Data: &CommodityReminderResponseData{
			ID:         r.ID,
			Type:       "commodity_reminders",
			Attributes: *NewCommodityReminderView(r, now),
		},
	}
}

func (cr *CommodityReminderResponse) WithStatusCode(code int) *CommodityReminderResponse {
	tmp := *cr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (cr *CommodityReminderResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(cr.HTTPStatusCode, http.StatusOK))
	return nil
}

// CommodityRemindersMeta is the count block on a list response.
type CommodityRemindersMeta struct {
	Reminders int `json:"reminders" example:"3" format:"int64"`
}

// CommodityRemindersResponse lists the reminders of one item.
type CommodityRemindersResponse struct {
	Data []*CommodityReminderView `json:"data"`
	Meta CommodityRemindersMeta   `json:"meta"`
}

func NewCommodityRemindersResponse(reminders []*models.CommodityReminder, now time.Time) *CommodityRemindersResponse {
	views := make([]*CommodityReminderView, 0, len(reminders))
	for _, r := range reminders {
		views = append(views, NewCommodityReminderView(r, now))
	}
	return &CommodityRemindersResponse{
		Data: views,
		Meta: CommodityRemindersMeta{Reminders: len(views)},
	}
}

func (*CommodityRemindersResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CommodityReminderRequest is the JSON:API payload for
// POST .../commodities/{id}/reminders.
type CommodityReminderRequest struct {
	Data *CommodityReminderRequestDataWrapper `json:"data"`
}

type CommodityReminderRequestDataWrapper struct {
	ID         string                       `json:"id,omitempty"`
	Type       string                       `json:"type"`
	Attributes CommodityReminderRequestData `json:"attributes"`
}

// CommodityReminderRequestData carries the fields of a new reminder.
// RepeatUnit defaults to "none" and RepeatInterval to 1.
type CommodityReminderRequestData struct {
	Title            string                    `json:"title"`
	DueOn            models.Date               `json:"due_on"`
	RemindDaysBefore []int                     `json:"remind_days_before"`
	RepeatUnit       models.ReminderRepeatUnit `json:"repeat_unit,omitempty" enums:"none,day,week,month,year"`
	RepeatInterval   int                       `json:"repeat_interval,omitempty"`
}

func (d CommodityReminderRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (d CommodityReminderRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &d,
		validation.Field(&d.Title, validation.Required, validation.Length(1, models.ReminderTitleMaxLength)),
		validation.Field(&d.DueOn, validation.Required, models.DateFormat),
		validation.Field(&d.RemindDaysBefore, models.ReminderLeadTimesRule),
		validation.Field(&d.RepeatUnit, models.ReminderRepeatUnitRule),
		validation.Field(&d.RepeatInterval, validation.Min(0), validation.Max(models.ReminderMaxRepeatInterval)),
	)
}

func (w *CommodityReminderRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if w.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("commodity_reminders")),
		validation.Field(&w.Attributes, validation.Required),
	)
}

func (cr *CommodityReminderRequest) Bind(r *http.Request) error {
	return cr.ValidateWithContext(r.Context())
}

func (cr *CommodityReminderRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, cr,
		validation.Field(&cr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CommodityReminderRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityReminderRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityReminderRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = (*CommodityReminderRequestData)(nil)
)

// CommodityReminderUpdateRequest is the JSON:API payload for
// PATCH .../reminders/{id}. Every attribute is optional; an absent one is
// left unchanged. The merged reminder is validated as a whole.
type CommodityReminderUpdateRequest struct {
	Data *CommodityReminderUpdateRequestDataWrapper `json:"data"`
}

type CommodityReminderUpdateRequestDataWrapper struct {
	ID         string                             `json:"id"`
	Type       string                             `json:"type"`
	Attributes CommodityReminderUpdateRequestData `json:"attributes"`
}

type CommodityReminderUpdateRequestData struct {
	Title            *string                    `json:"title,omitempty"`
	DueOn            *models.Date               `json:"due_on,omitempty"`
	RemindDaysBefore []int                      `json:"remind_days_before,omitempty"`
	RepeatUnit       *models.ReminderRepeatUnit `json:"repeat_unit,omitempty" enums:"none,day,week,month,year"`
	RepeatInterval   *int                       `json:"repeat_interval,omitempty"`
}

func (w *CommodityReminderUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("commodity_reminders")),
	)
}

func (ur *CommodityReminderUpdateRequest) Bind(r *http.Request) error {
	return ur.ValidateWithContext(r.Context())
}

func (ur *CommodityReminderUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, ur,
		validation.Field(&ur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CommodityReminderUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityReminderUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*CommodityReminderUpdateRequestDataWrapper)(nil)
)
