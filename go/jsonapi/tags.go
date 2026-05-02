package jsonapi

import (
	"context"
	"errors"
	"net/http"

	"github.com/go-chi/render"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// TagMeta carries the per-tag usage breakdown returned alongside detail
// responses (GET /tags/{id}). Empty on list responses to keep the payload
// small — usage on lists is folded into the meta on FilesMeta-equivalent
// shape if needed later.
type TagMeta struct {
	Usage *registry.TagUsage `json:"usage,omitempty"`
}

// TagResponse is the JSON:API envelope for a single tag.
type TagResponse struct {
	HTTPStatusCode int `json:"-"`

	ID         string     `json:"id"`
	Type       string     `json:"type" example:"tags" enums:"tags"`
	Attributes models.Tag `json:"attributes"`
	Meta       *TagMeta   `json:"meta,omitempty"`
}

// NewTagResponse builds a TagResponse without usage details (list/detail-lite).
func NewTagResponse(tag *models.Tag) *TagResponse {
	return &TagResponse{ID: tag.ID, Type: "tags", Attributes: *tag}
}

// NewTagResponseWithUsage attaches a usage breakdown to the meta.
func NewTagResponseWithUsage(tag *models.Tag, usage registry.TagUsage) *TagResponse {
	return &TagResponse{
		ID:         tag.ID,
		Type:       "tags",
		Attributes: *tag,
		Meta:       &TagMeta{Usage: &usage},
	}
}

func (tr *TagResponse) WithStatusCode(code int) *TagResponse {
	tmp := *tr
	tmp.HTTPStatusCode = code
	return &tmp
}

func (tr *TagResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, statusCodeDef(tr.HTTPStatusCode, http.StatusOK))
	return nil
}

// TagsMeta is the meta block for a paginated tag list.
type TagsMeta struct {
	Tags  int `json:"tags" example:"10" format:"int64"`
	Total int `json:"total" example:"100" format:"int64"`
}

// TagsResponse is a paginated list of tags.
type TagsResponse struct {
	Data []*models.Tag `json:"data"`
	Meta TagsMeta      `json:"meta"`
}

func NewTagsResponse(tags []*models.Tag, total int) *TagsResponse {
	if tags == nil {
		tags = []*models.Tag{}
	}
	return &TagsResponse{
		Data: tags,
		Meta: TagsMeta{Tags: len(tags), Total: total},
	}
}

func (*TagsResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// TagAutocompleteEntry is the lightweight shape returned by GET
// /tags/autocomplete — strictly what the FE input needs to render a chip.
type TagAutocompleteEntry struct {
	ID    string          `json:"id"`
	Slug  string          `json:"slug"`
	Label string          `json:"label"`
	Color models.TagColor `json:"color" example:"muted" enums:"amber,green,blue,orange,red,muted"`
}

// TagAutocompleteResponse is a flat list of tag entries — no pagination
// envelope because the endpoint always caps at a small limit.
type TagAutocompleteResponse struct {
	Data []TagAutocompleteEntry `json:"data"`
}

// NewTagAutocompleteResponse converts a list of tags into the lightweight
// autocomplete shape.
func NewTagAutocompleteResponse(tags []*models.Tag) *TagAutocompleteResponse {
	entries := make([]TagAutocompleteEntry, 0, len(tags))
	for _, t := range tags {
		entries = append(entries, TagAutocompleteEntry{
			ID:    t.ID,
			Slug:  t.Slug,
			Label: t.Label,
			Color: t.Color,
		})
	}
	return &TagAutocompleteResponse{Data: entries}
}

func (*TagAutocompleteResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusOK)
	return nil
}

// TagRequest is the JSON:API payload for POST /tags.
type TagRequest struct {
	Data *TagRequestDataWrapper `json:"data"`
}

// TagRequestDataWrapper wraps the create attributes.
type TagRequestDataWrapper struct {
	ID         string         `json:"id,omitempty"`
	Type       string         `json:"type"`
	Attributes TagRequestData `json:"attributes"`
}

// TagRequestData carries the user-supplied fields on create.
type TagRequestData struct {
	Slug  string          `json:"slug"`
	Label string          `json:"label"`
	Color models.TagColor `json:"color" example:"muted" enums:"amber,green,blue,orange,red,muted"`
}

func (trd *TagRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (trd *TagRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&trd.Slug, validation.Required, validation.By(func(value any) error {
			s, _ := value.(string)
			if !models.IsValidTagSlug(s) {
				return validation.NewError("invalid_slug", "slug must be lowercase, kebab-cased")
			}
			return nil
		})),
		validation.Field(&trd.Label, validation.Required, validation.Length(1, 64)),
		validation.Field(&trd.Color, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, trd, fields...)
}

func (trdw *TagRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	if trdw.ID != "" {
		return errors.New("ID field not allowed in create requests")
	}
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&trdw.Type, validation.Required, validation.In("tags")),
		validation.Field(&trdw.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, trdw, fields...)
}

func (tr *TagRequest) Bind(r *http.Request) error {
	return tr.ValidateWithContext(r.Context())
}

func (tr *TagRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&tr.Data, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, tr, fields...)
}

var (
	_ render.Binder                     = (*TagRequest)(nil)
	_ validation.ValidatableWithContext = (*TagRequest)(nil)
	_ validation.ValidatableWithContext = (*TagRequestDataWrapper)(nil)
	_ validation.ValidatableWithContext = (*TagRequestData)(nil)
)

// TagUpdateRequest is the JSON:API payload for PATCH /tags/{id}.
// All attribute fields are optional; the handler treats empty/zero values
// as "leave unchanged" rather than "clear".
type TagUpdateRequest struct {
	Data *TagUpdateRequestDataWrapper `json:"data"`
}

type TagUpdateRequestDataWrapper struct {
	ID         string               `json:"id"`
	Type       string               `json:"type"`
	Attributes TagUpdateRequestData `json:"attributes"`
}

type TagUpdateRequestData struct {
	Slug  string          `json:"slug,omitempty"`
	Label string          `json:"label,omitempty"`
	Color models.TagColor `json:"color,omitempty" example:"muted" enums:"amber,green,blue,orange,red,muted"`
}

func (turd *TagUpdateRequestData) Validate() error {
	return models.ErrMustUseValidateWithContext
}

func (turd *TagUpdateRequestData) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	if turd.Slug != "" {
		fields = append(fields,
			validation.Field(&turd.Slug, validation.By(func(value any) error {
				s, _ := value.(string)
				if !models.IsValidTagSlug(s) {
					return validation.NewError("invalid_slug", "slug must be lowercase, kebab-cased")
				}
				return nil
			})),
		)
	}
	if turd.Label != "" {
		fields = append(fields, validation.Field(&turd.Label, validation.Length(1, 64)))
	}
	// Color zero value is "" — only validate if set.
	if turd.Color != "" {
		fields = append(fields, validation.Field(&turd.Color))
	}
	return validation.ValidateStructWithContext(ctx, turd, fields...)
}

func (tudw *TagUpdateRequestDataWrapper) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&tudw.Type, validation.Required, validation.In("tags")),
		validation.Field(&tudw.Attributes, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, tudw, fields...)
}

func (tur *TagUpdateRequest) Bind(r *http.Request) error {
	return tur.ValidateWithContext(r.Context())
}

func (tur *TagUpdateRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)
	fields = append(fields,
		validation.Field(&tur.Data, validation.Required),
	)
	return validation.ValidateStructWithContext(ctx, tur, fields...)
}

var (
	_ render.Binder                     = (*TagUpdateRequest)(nil)
	_ validation.ValidatableWithContext = (*TagUpdateRequest)(nil)
)
