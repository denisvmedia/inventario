package models

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// tagSlugPattern matches the canonical slug shape: lowercase alphanumerics
// optionally separated by single hyphens. Mirror of input.IsValidSlug,
// inlined here because models/ cannot import the internal cmd package.
var tagSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// IsValidTagSlug reports whether s is a canonical tag slug.
func IsValidTagSlug(s string) bool {
	return tagSlugPattern.MatchString(s)
}

// TagColor is the curated set of colors a tag can be displayed in. The set is
// closed and validated at app level (no DB CHECK constraint — the migrator
// helper does not emit them and we already gate writes through the registry).
// FE mirrors the same six values; both sides must change together.
type TagColor string

const (
	TagColorAmber  TagColor = "amber"
	TagColorGreen  TagColor = "green"
	TagColorBlue   TagColor = "blue"
	TagColorOrange TagColor = "orange"
	TagColorRed    TagColor = "red"
	TagColorMuted  TagColor = "muted"
)

// ValidTagColors is the closed set accepted by validation. Order also drives
// the order in the FE picker; keep stable.
var ValidTagColors = []TagColor{
	TagColorAmber,
	TagColorGreen,
	TagColorBlue,
	TagColorOrange,
	TagColorRed,
	TagColorMuted,
}

// DefaultTagColor is assigned to tags that are auto-created on first
// reference (e.g. when a commodity write includes a slug not yet in the
// `tags` table). Picked from ValidTagColors so the row always passes
// validation; the user can re-color via PATCH later.
const DefaultTagColor = TagColorMuted

var validTagColorSet = func() map[TagColor]struct{} {
	set := make(map[TagColor]struct{}, len(ValidTagColors))
	for _, c := range ValidTagColors {
		set[c] = struct{}{}
	}
	return set
}()

// IsValidTagColor reports whether the given string is one of the curated
// tag colors. Empty strings are NOT valid here; the validator handles the
// "color is required" case separately.
func (c TagColor) IsValid() bool {
	_, ok := validTagColorSet[c]
	return ok
}

// Validate makes TagColor a validation.Validatable so it can be referenced
// directly by validation.Field rules.
func (c TagColor) Validate() error {
	if !c.IsValid() {
		return validation.NewError("invalid_tag_color", "invalid tag color")
	}
	return nil
}

// NormalizeTagSlug coerces a free-form user-typed string into the canonical
// slug shape accepted by IsValidSlug:
//
//   - lowercased
//   - non-alphanumeric runs collapsed to a single '-'
//   - leading and trailing '-' trimmed
//
// The result is empty when the input contains no usable characters
// (e.g. "   ", "###"). Callers should treat empty as a validation error.
func NormalizeTagSlug(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevDash := true // start as if previous was dash to swallow leading separators
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.TrimRight(b.String(), "-")
}

var (
	_ validation.Validatable            = (*Tag)(nil)
	_ validation.ValidatableWithContext = (*Tag)(nil)
	_ TenantGroupAwareIDable            = (*Tag)(nil)
)

// Tag is a first-class, group-scoped catalogue entry for the free-form tag
// strings stored as JSONB on commodities and files. Promoting tags off the
// JSONB columns gives them stable identity (slug+id), a curated color, and
// usage counts; the JSONB associations stay in place so RLS is unchanged.
//
// Auto-created on first reference: a commodity/file write that mentions an
// unknown slug will provision a Tag row with DefaultTagColor instead of
// erroring — preserves the legacy "anyone can type any tag" UX.
//
// Enable RLS for multi-tenant isolation
//
//migrator:schema:rls:enable table="tags" comment="Enable RLS for multi-tenant tag isolation"
//migrator:schema:rls:policy name="tag_isolation" table="tags" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures tags can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="tag_background_worker_access" table="tags" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all tags for processing"
//migrator:schema:table name="tags"
type Tag struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// Slug is the kebab-cased identifier referenced from
	// commodities.tags / files.tags JSONB arrays. Unique per group.
	//migrator:schema:field name="slug" type="TEXT" not_null="true"
	Slug string `json:"slug" db:"slug"`

	// Label is the human-readable display name.
	//migrator:schema:field name="label" type="TEXT" not_null="true"
	Label string `json:"label" db:"label"`

	// Color is one of the curated TagColor values.
	//migrator:schema:field name="color" type="TEXT" not_null="true" default="muted"
	Color TagColor `json:"color" db:"color"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TagIndexes defines the postgres indexes / uniqueness constraints for tags.
type TagIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//migrator:schema:index name="idx_tags_uuid" fields="uuid" unique="true" table="tags"
	_ int

	// Index for tenant-based queries (audit, cross-group analytics).
	//migrator:schema:index name="idx_tags_tenant_id" fields="tenant_id" table="tags"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//migrator:schema:index name="idx_tags_tenant_group" fields="tenant_id,group_id" table="tags"
	_ int

	// Per-group slug uniqueness — backs the autocomplete lookup and
	// prevents duplicate "kitchen" / "Kitchen" tags within the same group.
	//migrator:schema:index name="idx_tags_group_slug" fields="group_id,slug" unique="true" table="tags"
	_ int

	// Trigram similarity for label search (autocomplete / ?q=).
	//migrator:schema:index name="tags_label_trgm_idx" fields="label" type="GIN" ops="gin_trgm_ops" table="tags"
	_ int
}

func (*Tag) Validate() error {
	return ErrMustUseValidateWithContext
}

func (t *Tag) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, t,
		validation.Field(&t.Slug, rules.NotEmpty, validation.By(func(value any) error {
			s, _ := value.(string)
			if !IsValidTagSlug(s) {
				return validation.NewError("invalid_slug", "slug must be lowercase, kebab-cased")
			}
			return nil
		})),
		validation.Field(&t.Label, rules.NotEmpty, validation.Length(1, 64)),
		validation.Field(&t.Color, validation.Required),
	)
}
