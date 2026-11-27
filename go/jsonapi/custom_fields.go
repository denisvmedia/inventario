package jsonapi

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// CustomFieldOptionView is a select option with the number of items, in
// Trash included, whose value it is, so a client can say what removing it
// clears.
type CustomFieldOptionView struct {
	ID        string `json:"id" example:"a1b2c3d4"`
	Label     string `json:"label" example:"Petrol"`
	ItemCount int    `json:"item_count" example:"2" format:"int64"`
}

// CustomFieldView is a custom field as the API shows it.
type CustomFieldView struct {
	Name string `json:"name" example:"License plate"`
	// Type is one of text, long_text, number, money, date, boolean, select,
	// url, item_link. It never changes.
	Type      string                       `json:"type" example:"text"`
	Options   []CustomFieldOptionView      `json:"options"`
	Templates []models.CustomFieldTemplate `json:"templates"`
	// ItemCount and TrashedItemCount are the items with a value of the
	// field. Deleting the field deletes those values.
	ItemCount        int `json:"item_count" example:"3" format:"int64"`
	TrashedItemCount int `json:"trashed_item_count" example:"0" format:"int64"`
	// DraftCount is the number of unfinished drafts with a value of the
	// field. Deleting the field takes the value out of them too.
	DraftCount int       `json:"draft_count" example:"0" format:"int64"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func newCustomFieldView(d *models.CustomFieldDefinition, usage registry.CustomFieldUsage) CustomFieldView {
	options := make([]CustomFieldOptionView, 0, len(d.Options))
	for _, o := range d.Options {
		options = append(options, CustomFieldOptionView{ID: o.ID, Label: o.Label, ItemCount: usage.Options[o.ID]})
	}
	templates := d.Templates
	if templates == nil {
		templates = []models.CustomFieldTemplate{}
	}
	return CustomFieldView{
		Name:             d.Name,
		Type:             string(d.Type),
		Options:          options,
		Templates:        templates,
		ItemCount:        usage.Live,
		TrashedItemCount: usage.Trashed,
		DraftCount:       usage.Drafts,
		CreatedAt:        d.CreatedAt,
		UpdatedAt:        d.UpdatedAt,
	}
}

// CustomFieldResponse is the JSON:API envelope for one custom field.
type CustomFieldResponse struct {
	HTTPStatusCode int                      `json:"-"`
	Data           *CustomFieldResponseData `json:"data"`
}

// CustomFieldResponseData is the inner resource object.
type CustomFieldResponseData struct {
	ID         string          `json:"id"`
	Type       string          `json:"type" example:"custom_fields" enums:"custom_fields"`
	Attributes CustomFieldView `json:"attributes"`
}

func newCustomFieldResponseData(d *models.CustomFieldDefinition, usage registry.CustomFieldUsage) *CustomFieldResponseData {
	return &CustomFieldResponseData{ID: d.ID, Type: "custom_fields", Attributes: newCustomFieldView(d, usage)}
}

func NewCustomFieldResponse(d *models.CustomFieldDefinition, usage registry.CustomFieldUsage) *CustomFieldResponse {
	return &CustomFieldResponse{Data: newCustomFieldResponseData(d, usage)}
}

func (cr *CustomFieldResponse) WithStatusCode(code int) *CustomFieldResponse {
	tmp := *cr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (cr *CustomFieldResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(cr.HTTPStatusCode, http.StatusOK))
	return nil
}

// CustomFieldsMeta is the count block on a list response. Limit is the most
// fields a group can have.
type CustomFieldsMeta struct {
	CustomFields int `json:"custom_fields" example:"4" format:"int64"`
	Limit        int `json:"limit" example:"200" format:"int64"`
}

// CustomFieldsResponse lists the custom fields of a group by name.
type CustomFieldsResponse struct {
	Data []*CustomFieldResponseData `json:"data"`
	Meta CustomFieldsMeta           `json:"meta"`
}

// NewCustomFieldsResponse wraps defs; usage maps a field ID to its counts,
// and a field missing from it has no values.
func NewCustomFieldsResponse(defs []*models.CustomFieldDefinition, usage map[string]registry.CustomFieldUsage) *CustomFieldsResponse {
	data := make([]*CustomFieldResponseData, 0, len(defs))
	for _, d := range defs {
		data = append(data, newCustomFieldResponseData(d, usage[d.ID]))
	}
	return &CustomFieldsResponse{Data: data, Meta: CustomFieldsMeta{CustomFields: len(data), Limit: models.MaxCustomFieldsPerGroup}}
}

func (*CustomFieldsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// CustomFieldRequest is the JSON:API payload for POST /custom-fields.
type CustomFieldRequest struct {
	Data *CustomFieldRequestDataWrapper `json:"data"`
}

type CustomFieldRequestDataWrapper struct {
	ID         string                 `json:"id,omitempty"`
	Type       string                 `json:"type" example:"custom_fields" enums:"custom_fields"`
	Attributes CustomFieldRequestData `json:"attributes"`
}

// CustomFieldRequestData carries the fields of a new custom field. Option
// IDs are made by the server. The handler normalizes and validates the
// fields with models.CustomFieldDefinition.ValidateFields.
type CustomFieldRequestData struct {
	Name      string                       `json:"name" example:"License plate"`
	Type      string                       `json:"type" example:"text"`
	Options   []models.CustomFieldOption   `json:"options,omitempty"`
	Templates []models.CustomFieldTemplate `json:"templates,omitempty"`
}

// Definition returns the new field d describes.
func (d CustomFieldRequestData) Definition() models.CustomFieldDefinition {
	options := make(models.ValuerSlice[models.CustomFieldOption], 0, len(d.Options))
	for _, o := range d.Options {
		options = append(options, models.CustomFieldOption{Label: o.Label})
	}
	return models.CustomFieldDefinition{
		Name:      d.Name,
		Type:      models.CustomFieldType(d.Type),
		Options:   options,
		Templates: d.Templates,
	}
}

func (w *CustomFieldRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if w.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("custom_fields")),
	)
}

func (cr *CustomFieldRequest) Bind(r *http.Request) error {
	return cr.ValidateWithContext(r.Context())
}

func (cr *CustomFieldRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, cr,
		validation.Field(&cr.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CustomFieldRequest)(nil)
	_ validation.ValidatableWithContext = (*CustomFieldRequest)(nil)
	_ validation.ValidatableWithContext = (*CustomFieldRequestDataWrapper)(nil)
)

// CustomFieldUpdateRequest is the JSON:API payload for PATCH
// /custom-fields/{id}. An absent attribute is left unchanged.
type CustomFieldUpdateRequest struct {
	Data *CustomFieldUpdateRequestDataWrapper `json:"data"`
}

type CustomFieldUpdateRequestDataWrapper struct {
	ID         string                       `json:"id"`
	Type       string                       `json:"type" example:"custom_fields" enums:"custom_fields"`
	Attributes CustomFieldUpdateRequestData `json:"attributes"`
}

// CustomFieldUpdateRequestData carries the fields to change.
//
// Options replaces the option list: an entry with the ID of an existing
// option keeps it (and its values) under the new label and position, an
// entry without an ID adds an option, and an option left out is removed
// together with its values. Templates replaces the template entries. Type
// may be sent only with the stored type; another one is a field error.
type CustomFieldUpdateRequestData struct {
	Name      *string                       `json:"name,omitempty"`
	Type      *string                       `json:"type,omitempty"`
	Options   *[]models.CustomFieldOption   `json:"options,omitempty"`
	Templates *[]models.CustomFieldTemplate `json:"templates,omitempty"`
}

// Apply copies the attributes present in d onto def, which holds the stored
// field. It returns models.ErrCustomFieldTypeChanged under "type" when d
// names another type.
func (d CustomFieldUpdateRequestData) Apply(def *models.CustomFieldDefinition) error {
	if d.Type != nil && models.CustomFieldType(*d.Type) != def.Type {
		return validation.Errors{"type": models.ErrCustomFieldTypeChanged}
	}
	if d.Name != nil {
		def.Name = *d.Name
	}
	if d.Options != nil {
		def.Options = models.ValuerSlice[models.CustomFieldOption](*d.Options)
	}
	def.Templates = nil
	if d.Templates != nil {
		def.Templates = *d.Templates
		if def.Templates == nil {
			def.Templates = []models.CustomFieldTemplate{}
		}
	}
	return nil
}

func (w *CustomFieldUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.Type, validation.Required, validation.In("custom_fields")),
	)
}

func (ur *CustomFieldUpdateRequest) Bind(r *http.Request) error {
	return ur.ValidateWithContext(r.Context())
}

func (ur *CustomFieldUpdateRequest) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, ur,
		validation.Field(&ur.Data, validation.Required),
	)
}

var (
	_ render.Binder                     = (*CustomFieldUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*CustomFieldUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*CustomFieldUpdateRequestDataWrapper)(nil)
)

// CustomFieldDeleteResponse reports how many values a delete removed.
type CustomFieldDeleteResponse struct {
	Meta CustomFieldDeleteMeta `json:"meta"`
}

type CustomFieldDeleteMeta struct {
	DeletedValues int `json:"deleted_values" example:"4" format:"int64"`
}

func (*CustomFieldDeleteResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}
