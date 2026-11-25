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

const (
	savedViewsType       = "saved_views"
	savedViewReorderType = "saved_views_reorder"
)

// SavedViewQueryError explains why the stored query of a saved view cannot
// run. Parameter is empty when the whole query is affected.
type SavedViewQueryError struct {
	Parameter string `json:"parameter" example:"warranty_expires_before"`
	Reason    string `json:"reason" example:"is not available in a saved view"`
}

// SavedViewAttributes is a saved view as the requesting member sees it.
type SavedViewAttributes struct {
	Name string `json:"name" example:"Expiring warranties"`
	Icon string `json:"icon" example:"shield"`
	// Entity is the list the view filters; only "commodities" exists.
	Entity string `json:"entity" example:"commodities" enums:"commodities"`
	// Query is the canonical query string of the view in the parameter
	// names of GET /commodities, without the leading "?".
	Query        string `json:"query" example:"warranty_status=expiring&sort=name"`
	QueryVersion int    `json:"query_version" example:"1"`
	// Layout is grid, list, or empty for the viewer's default.
	Layout string `json:"layout" example:"list" enums:",grid,list"`
	Shared bool   `json:"shared"`
	// ShowInSidebar and ShowOnDashboard are where the requesting member
	// shows the view. Each member places the views they see on their own.
	ShowInSidebar   bool      `json:"show_in_sidebar"`
	ShowOnDashboard bool      `json:"show_on_dashboard"`
	SortOrder       int       `json:"sort_order" example:"0"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	// Owned is true for the views the requesting member created.
	Owned     bool `json:"owned"`
	CanEdit   bool `json:"can_edit"`
	CanDelete bool `json:"can_delete"`
	// QueryError is set when the stored query cannot run; such a view has
	// no count and should be edited before it is opened.
	QueryError *SavedViewQueryError `json:"query_error,omitempty"`
}

// SavedViewAccess is how the requesting member stands to one view: where
// they show it and what they may do with it. The service decides it.
type SavedViewAccess struct {
	ShowInSidebar   bool
	ShowOnDashboard bool
	Owned           bool
	CanEdit         bool
	CanDelete       bool
	QueryError      *SavedViewQueryError
}

// SavedViewData is one saved view resource object.
type SavedViewData struct {
	ID         string              `json:"id"`
	Type       string              `json:"type" example:"saved_views" enums:"saved_views"`
	Attributes SavedViewAttributes `json:"attributes"`
}

// NewSavedViewData builds the resource object of view for a member.
func NewSavedViewData(view *models.SavedView, access SavedViewAccess) SavedViewData {
	return SavedViewData{
		ID:   view.ID,
		Type: savedViewsType,
		Attributes: SavedViewAttributes{
			Name:            view.Name,
			Icon:            string(view.Icon),
			Entity:          string(view.Entity),
			Query:           view.Query,
			QueryVersion:    view.QueryVersion,
			Layout:          string(view.Layout),
			Shared:          view.Shared,
			ShowInSidebar:   access.ShowInSidebar,
			ShowOnDashboard: access.ShowOnDashboard,
			SortOrder:       view.SortOrder,
			CreatedAt:       view.CreatedAt,
			UpdatedAt:       view.UpdatedAt,
			Owned:           access.Owned,
			CanEdit:         access.CanEdit,
			CanDelete:       access.CanDelete,
			QueryError:      access.QueryError,
		},
	}
}

// SavedViewResponse is the single-resource envelope of a saved view.
type SavedViewResponse struct {
	HTTPStatusCode int           `json:"-"`
	Data           SavedViewData `json:"data"`
}

// NewSavedViewResponse wraps one resource object.
func NewSavedViewResponse(data SavedViewData) *SavedViewResponse {
	return &SavedViewResponse{Data: data}
}

// WithStatusCode sets the status the response renders with.
func (sr *SavedViewResponse) WithStatusCode(code int) *SavedViewResponse {
	tmp := *sr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (sr *SavedViewResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(sr.HTTPStatusCode, http.StatusOK))
	return nil
}

// SavedViewLimits tells a client the limits the server keeps, so it can
// disable a control before a request fails.
type SavedViewLimits struct {
	MaxNameLength int `json:"max_name_length" example:"60"`
	MaxPerUser    int `json:"max_per_user" example:"50"`
	MaxShared     int `json:"max_shared" example:"50"`
	// CanShare is true when the requesting member may share views.
	CanShare bool `json:"can_share"`
}

// SavedViewsMeta is the meta of the list response.
type SavedViewsMeta struct {
	Total  int             `json:"total" example:"3" format:"int64"`
	Limits SavedViewLimits `json:"limits"`
}

// SavedViewsResponse is the list envelope. Views come in display order.
type SavedViewsResponse struct {
	Data []SavedViewData `json:"data"`
	Meta SavedViewsMeta  `json:"meta"`
}

// NewSavedViewsResponse wraps the views a member sees.
func NewSavedViewsResponse(data []SavedViewData, canShare bool) *SavedViewsResponse {
	if data == nil {
		data = []SavedViewData{}
	}
	return &SavedViewsResponse{
		Data: data,
		Meta: SavedViewsMeta{
			Total: len(data),
			Limits: SavedViewLimits{
				MaxNameLength: models.MaxSavedViewNameRunes,
				MaxPerUser:    models.MaxSavedViewsPerUser,
				MaxShared:     models.MaxSharedSavedViews,
				CanShare:      canShare,
			},
		},
	}
}

func (*SavedViewsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// SavedViewCounts holds, per saved view ID, the number of items its query
// selects.
type SavedViewCounts struct {
	Counts map[string]int `json:"counts"`
}

// SavedViewCountsResponse is the payload of GET /saved-views/counts: a flat
// envelope, like GET /commodities/tag-counts.
type SavedViewCountsResponse struct {
	Data SavedViewCounts `json:"data"`
}

// NewSavedViewCountsResponse wraps the counts.
func NewSavedViewCountsResponse(counts map[string]int) *SavedViewCountsResponse {
	if counts == nil {
		counts = map[string]int{}
	}
	return &SavedViewCountsResponse{Data: SavedViewCounts{Counts: counts}}
}

func (*SavedViewCountsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// SavedViewRequest is the payload of POST /saved-views.
type SavedViewRequest struct {
	Data *SavedViewRequestDataWrapper `json:"data"`
}

type SavedViewRequestDataWrapper struct {
	ID         string               `json:"id,omitempty"`
	Type       string               `json:"type" example:"saved_views" enums:"saved_views"`
	Attributes SavedViewRequestData `json:"attributes"`
}

// SavedViewRequestData carries the fields a member sets on create. The query
// is validated against the item query contract by the service.
// ShowInSidebar and ShowOnDashboard are the creator's own placement.
type SavedViewRequestData struct {
	Name            string `json:"name" example:"Expiring warranties"`
	Icon            string `json:"icon,omitempty" example:"shield"`
	Query           string `json:"query" example:"warranty_status=expiring"`
	Layout          string `json:"layout,omitempty" example:"list" enums:",grid,list"`
	Shared          bool   `json:"shared,omitempty"`
	ShowInSidebar   bool   `json:"show_in_sidebar,omitempty"`
	ShowOnDashboard bool   `json:"show_on_dashboard,omitempty"`
}

func (SavedViewRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (d SavedViewRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &d,
		validation.Field(&d.Name, validation.Required, validation.RuneLength(1, models.MaxSavedViewNameRunes)),
		validation.Field(&d.Icon, validation.By(validSavedViewIcon)),
		validation.Field(&d.Query, validation.Length(0, models.MaxSavedViewQueryBytes)),
		validation.Field(&d.Layout, validation.By(validSavedViewLayout)),
	)
}

func (w *SavedViewRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if w.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In(savedViewsType)),
		validation.Field(&w.Attributes, validation.Required),
	)
}

func (sr *SavedViewRequest) Bind(r *http.Request) error {
	return sr.ValidateWithContext(r.Context())
}

func (sr *SavedViewRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sr,
		validation.Field(&sr.Data, validation.Required),
	)
}

// SavedViewUpdateRequest is the payload of PATCH /saved-views/{id}. A field
// left out is not changed.
type SavedViewUpdateRequest struct {
	Data *SavedViewUpdateRequestDataWrapper `json:"data"`
}

type SavedViewUpdateRequestDataWrapper struct {
	ID         string                     `json:"id,omitempty"`
	Type       string                     `json:"type" example:"saved_views" enums:"saved_views"`
	Attributes SavedViewUpdateRequestData `json:"attributes"`
}

// SavedViewUpdateRequestData carries the fields a PATCH changes.
// ShowInSidebar and ShowOnDashboard change where the requesting member shows
// the view, which any member who sees it may do; the other fields change the
// view and need the right to edit it.
type SavedViewUpdateRequestData struct {
	Name            *string `json:"name,omitempty"`
	Icon            *string `json:"icon,omitempty"`
	Query           *string `json:"query,omitempty"`
	Layout          *string `json:"layout,omitempty"`
	Shared          *bool   `json:"shared,omitempty"`
	ShowInSidebar   *bool   `json:"show_in_sidebar,omitempty"`
	ShowOnDashboard *bool   `json:"show_on_dashboard,omitempty"`
}

func (SavedViewUpdateRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (d SavedViewUpdateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0, 4)
	if d.Name != nil {
		fields = append(fields, validation.Field(&d.Name, validation.By(func(any) error {
			return validation.Validate(*d.Name, validation.Required, validation.RuneLength(1, models.MaxSavedViewNameRunes))
		})))
	}
	if d.Icon != nil {
		fields = append(fields, validation.Field(&d.Icon, validation.By(func(any) error {
			if *d.Icon == "" {
				return validation.NewError("validation_required", "cannot be blank")
			}
			return validSavedViewIcon(*d.Icon)
		})))
	}
	if d.Query != nil {
		fields = append(fields, validation.Field(&d.Query, validation.By(func(any) error {
			return validation.Validate(*d.Query, validation.Length(0, models.MaxSavedViewQueryBytes))
		})))
	}
	if d.Layout != nil {
		fields = append(fields, validation.Field(&d.Layout, validation.By(func(any) error {
			return validSavedViewLayout(*d.Layout)
		})))
	}
	return validation.ValidateStructWithContext(ctx, &d, fields...)
}

func (w *SavedViewUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In(savedViewsType)),
	)
}

func (sr *SavedViewUpdateRequest) Bind(r *http.Request) error {
	return sr.ValidateWithContext(r.Context())
}

func (sr *SavedViewUpdateRequest) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, sr,
		validation.Field(&sr.Data, validation.Required),
	); err != nil {
		return err
	}
	return sr.Data.Attributes.ValidateWithContext(ctx)
}

// SavedViewReorderRequest is the payload of POST /saved-views/reorder.
type SavedViewReorderRequest struct {
	Data *SavedViewReorderRequestData `json:"data"`
}

type SavedViewReorderRequestData struct {
	Type       string                            `json:"type" example:"saved_views_reorder" enums:"saved_views_reorder"`
	Attributes SavedViewReorderRequestAttributes `json:"attributes"`
}

// SavedViewReorderRequestAttributes lists views in their new order. The
// views keep the positions they hold between them; views left out do not
// move.
type SavedViewReorderRequestAttributes struct {
	IDs []string `json:"ids"`
}

func (SavedViewReorderRequestAttributes) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (a SavedViewReorderRequestAttributes) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &a,
		validation.Field(&a.IDs, validation.Required,
			validation.Length(1, models.MaxSavedViewsPerUser+models.MaxSharedSavedViews),
			validation.Each(validation.Required)),
	)
}

func (d *SavedViewReorderRequestData) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.Type, validation.Required, validation.In(savedViewReorderType)),
		validation.Field(&d.Attributes, validation.Required),
	)
}

func (sr *SavedViewReorderRequest) Bind(r *http.Request) error {
	return sr.ValidateWithContext(r.Context())
}

func (sr *SavedViewReorderRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, sr,
		validation.Field(&sr.Data, validation.Required),
	)
}

func validSavedViewIcon(value any) error {
	s, _ := value.(string)
	if s == "" {
		return nil
	}
	return models.SavedViewIcon(s).Validate()
}

func validSavedViewLayout(value any) error {
	s, _ := value.(string)
	return models.SavedViewLayout(s).Validate()
}

var (
	_ render.Binder                     = (*SavedViewRequest)(nil)
	_ render.Binder                     = (*SavedViewUpdateRequest)(nil)
	_ render.Binder                     = (*SavedViewReorderRequest)(nil)
	_ validation.ValidatableWithContext = (*SavedViewRequest)(nil)
	_ validation.ValidatableWithContext = (*SavedViewRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = SavedViewRequestData{}
	_ validation.ValidatableWithContext = (*SavedViewUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*SavedViewUpdateRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = SavedViewUpdateRequestData{}
	_ validation.ValidatableWithContext = (*SavedViewReorderRequest)(nil)
	_ validation.ValidatableWithContext = (*SavedViewReorderRequestData)(nil)
	_ validation.ValidatableWithContext = SavedViewReorderRequestAttributes{}
)
