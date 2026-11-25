package models

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/jellydator/validation"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/unicode/norm"

	"go.5x5.cz/inventario/models/rules"
)

// MaxTagSlugRunes and MaxTagLabelRunes cap a tag's slug and label. They count
// runes, not bytes, so a Cyrillic or CJK label gets the same 64 characters as
// a Latin one. The frontend counts code points against the same limits.
const (
	MaxTagSlugRunes  = 64
	MaxTagLabelRunes = 64
)

// tagSlugPattern matches the shape of a slug: runs of Unicode letters,
// numbers and combining marks separated by single hyphens, each run starting
// with a letter or a number. The frontend's slugPattern is the same
// expression.
var tagSlugPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}\p{M}]*(?:-[\p{L}\p{N}][\p{L}\p{N}\p{M}]*)*$`)

// IsValidTagSlug reports whether s is a canonical tag slug: it has the shape
// tagSlugPattern describes and NormalizeTagSlug leaves it unchanged, which
// also requires it to be lowercase and in Unicode NFC. Defining validity
// through the normalizer keeps the two rules from drifting apart.
func IsValidTagSlug(s string) bool {
	return tagSlugPattern.MatchString(s) && NormalizeTagSlug(s) == s
}

// TagSlugRule is the validation.By rule for a tag slug field. The model and
// the JSON:API request types share it so they reject the same slugs with the
// same message.
func TagSlugRule(value any) error {
	s, _ := value.(string)
	if !IsValidTagSlug(s) {
		return validation.NewError("invalid_slug", "slug must be lowercase letters and digits separated by single hyphens")
	}
	return nil
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
// `tags` table). Declared as `var` rather than `const` so swag does not
// fold it into the OpenAPI enum entries for `TagColor` (which would emit
// "muted" twice — once for TagColorMuted and once for the alias).
//
//nolint:gochecknoglobals // intentional: see comment above; const would leak into swag enum.
var DefaultTagColor = TagColorMuted

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

// TagKind is the entity type a tag belongs to. Item-tags (commodity) and
// file-tags (file) are *separate* entities that merely share the tags table:
// the same slug can exist once as a commodity tag and once as a file tag
// (uniqueness is per (group, kind, slug)). The kind is stored on the row —
// it is no longer derived from usage.
//
// Names are part of the public API surface (FE sends them as ?kind=).
type TagKind string

const (
	// TagKindCommodity is the kind for tags attached to commodities (items).
	TagKindCommodity TagKind = "commodity"
	// TagKindFile is the kind for tags attached to files.
	TagKindFile TagKind = "file"
)

// TagKindAny is the empty-string filter sentinel meaning "no kind filter".
// Declared as a `var` (not a `const` in the enum block) so swag does NOT
// fold "" into the public models.TagKind OpenAPI enum — the exported
// contract / generated client types are commodity|file only. Same technique
// as DefaultTagColor above. Never a valid *stored* value: IsValid rejects it
// and the model validator requires a concrete kind; it exists purely as an
// internal "no filter" sentinel.
//
//nolint:gochecknoglobals // intentional: keep "" out of the swag enum; see comment above.
var TagKindAny TagKind = ""

// IsValid reports whether k is a valid *stored* kind (commodity or file).
// The empty string (TagKindAny) is a filter-only sentinel and is NOT valid
// here — mirrors TagColor.IsValid rejecting "".
func (k TagKind) IsValid() bool {
	switch k {
	case TagKindCommodity, TagKindFile:
		return true
	}
	return false
}

// Validate makes TagKind a validation.Validatable so it can be referenced
// directly by validation.Field rules (mirrors TagColor.Validate).
func (k TagKind) Validate() error {
	if !k.IsValid() {
		return validation.NewError("invalid_tag_kind", "invalid tag kind")
	}
	return nil
}

// NormalizeTagSlug coerces a free-form user-typed string into the canonical
// slug shape accepted by IsValidTagSlug. Letters and digits of every script
// are kept:
//
//   - the input is put into Unicode NFC, so precomposed and decomposed
//     spellings of the same text ("ě" as one code point or as "e" plus a
//     combining caron) give the same slug;
//   - it is lowercased with the full Unicode mapping (Greek final sigma
//     included), which is what JavaScript's String.prototype.toLowerCase
//     does;
//   - letters (\p{L}) and numbers (\p{N}) are kept, and so are combining
//     marks (\p{M}) that follow one of them, so scripts such as Devanagari
//     keep their vowel signs;
//   - every other run of characters (spaces, punctuation, symbols, emoji)
//     becomes a single '-', and leading and trailing '-' are dropped.
//
// frontend/src/features/tags/slug.ts normaliseSlug implements the same
// steps and is tested against the same table of cases; change both together.
//
// The result is empty when the input contains no usable characters
// (e.g. "   ", "###"). Callers should treat empty as a validation error.
func NormalizeTagSlug(s string) string {
	// A Caser keeps state between calls and must not be shared across
	// goroutines, so each call builds its own.
	lowered := norm.NFC.String(cases.Lower(language.Und).String(norm.NFC.String(s)))

	var b strings.Builder
	b.Grow(len(lowered))
	inRun := false       // the last rune written belongs to a letter/number run
	pendingDash := false // a separator was seen after at least one written run
	for _, r := range lowered {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if pendingDash {
				b.WriteByte('-')
				pendingDash = false
			}
			b.WriteRune(r)
			inRun = true
		case inRun && unicode.IsMark(r):
			b.WriteRune(r)
		default:
			// A mark with nothing to attach to counts as a separator too.
			inRun = false
			pendingDash = b.Len() > 0
		}
	}
	return b.String()
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
//ptah:schema:rls:enable table="tags" comment="Enable RLS for multi-tenant tag isolation"
//ptah:schema:rls:policy name="tag_isolation" table="tags" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures tags can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="tag_background_worker_access" table="tags" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all tags for processing"
//ptah:schema:table name="tags"
type Tag struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// Kind separates item-tags (commodity) from file-tags (file). Two tags
	// with the same slug but different kind are distinct entities. Existing
	// rows default to "commodity" on migration; new rows always carry an
	// explicit kind from the write path.
	//ptah:schema:field name="kind" type="TEXT" not_null="true" default="commodity"
	Kind TagKind `json:"kind" db:"kind"`

	// Slug is the identifier referenced from commodities.tags / files.tags
	// JSONB arrays: lowercase letters and digits of any script separated by
	// single hyphens, in Unicode NFC. Unique per (group, kind).
	//ptah:schema:field name="slug" type="TEXT" not_null="true"
	Slug string `json:"slug" db:"slug"`

	// Label is the human-readable display name.
	//ptah:schema:field name="label" type="TEXT" not_null="true"
	Label string `json:"label" db:"label"`

	// Color is one of the curated TagColor values.
	//ptah:schema:field name="color" type="TEXT" not_null="true" default="muted"
	Color TagColor `json:"color" db:"color"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// TagIndexes defines the postgres indexes / uniqueness constraints for tags.
type TagIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//ptah:schema:index name="idx_tags_uuid" fields="uuid" unique="true" table="tags"
	_ int

	// Index for tenant-based queries (audit, cross-group analytics).
	//ptah:schema:index name="idx_tags_tenant_id" fields="tenant_id" table="tags"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//ptah:schema:index name="idx_tags_tenant_group" fields="tenant_id,group_id" table="tags"
	_ int

	// Per-group, per-kind slug uniqueness — backs the autocomplete lookup
	// and prevents duplicate "kitchen" / "Kitchen" tags within the same
	// group AND kind, while allowing the same slug to exist once as a
	// commodity tag and once as a file tag.
	//ptah:schema:index name="idx_tags_group_kind_slug" fields="group_id,kind,slug" unique="true" table="tags"
	_ int

	// Trigram similarity for label search (autocomplete / ?q=).
	//ptah:schema:index name="tags_label_trgm_idx" fields="label" type="GIN" ops="gin_trgm_ops" table="tags"
	_ int
}

func (*Tag) Validate() error {
	return ErrMustUseValidateWithContext
}

func (t *Tag) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, t,
		validation.Field(&t.Kind, validation.Required),
		validation.Field(&t.Slug, rules.NotEmpty, validation.RuneLength(1, MaxTagSlugRunes), validation.By(TagSlugRule)),
		validation.Field(&t.Label, rules.NotEmpty, validation.RuneLength(1, MaxTagLabelRunes)),
		validation.Field(&t.Color, validation.Required),
	)
}
