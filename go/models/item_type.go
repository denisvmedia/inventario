package models

import (
	"context"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jellydator/validation"
)

// Limits on an item type's fields. They count runes, so a Cyrillic or CJK
// name gets the same room as a Latin one. The frontend checks the same
// limits.
const (
	MaxItemTypeNameRunes = 50
	MaxItemTypeSlugRunes = 64
	MaxItemTypeIconRunes = 40
)

// itemTypeSlugPattern is the shape of an item type slug: runs of Unicode
// letters, numbers and combining marks separated by single hyphens or
// underscores. The built-in slugs use underscores ("white_goods"); a slug
// made from a name uses hyphens, as a tag slug does (NormalizeTagSlug).
var itemTypeSlugPattern = regexp.MustCompile(`^[\p{L}\p{N}][\p{L}\p{N}\p{M}]*(?:[-_][\p{L}\p{N}][\p{L}\p{N}\p{M}]*)*$`)

// itemTypeIconPattern is the shape of an icon key: lowercase ASCII words
// joined by hyphens ("refrigerator", "washing-machine"). The frontend maps
// the key to an icon and shows a generic one for a key it does not know.
var itemTypeIconPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// BuiltinItemType is one of the item types every group starts with.
type BuiltinItemType struct {
	Slug CommodityType
	// Name is the English name. The frontend shows its own translation of
	// the slug instead, until a member renames the type.
	Name string
	Icon string
}

// BuiltinItemTypes are seeded in every group, in this order.
var BuiltinItemTypes = []BuiltinItemType{
	{Slug: CommodityTypeWhiteGoods, Name: "Appliance", Icon: "refrigerator"},
	{Slug: CommodityTypeElectronics, Name: "Electronics", Icon: "laptop"},
	{Slug: CommodityTypeEquipment, Name: "Equipment", Icon: "wrench"},
	{Slug: CommodityTypeFurniture, Name: "Furniture", Icon: "armchair"},
	{Slug: CommodityTypeClothes, Name: "Clothes", Icon: "shirt"},
	{Slug: CommodityTypeOther, Name: "Other", Icon: "package"},
}

var builtinItemTypeBySlug = func() map[CommodityType]BuiltinItemType {
	m := make(map[CommodityType]BuiltinItemType, len(BuiltinItemTypes))
	for _, b := range BuiltinItemTypes {
		m[b.Slug] = b
	}
	return m
}()

// BuiltinItemTypeFor returns the built-in item type with the given slug.
func BuiltinItemTypeFor(slug CommodityType) (BuiltinItemType, bool) {
	b, ok := builtinItemTypeBySlug[slug]
	return b, ok
}

var (
	_ validation.Validatable            = (*ItemType)(nil)
	_ validation.ValidatableWithContext = (*ItemType)(nil)
	_ TenantGroupAwareIDable            = (*ItemType)(nil)
)

// ItemType is a kind of item a group keeps (#2802): a name and an icon.
// Every group starts with the six BuiltinItemTypes; members add, rename and
// delete types. An item names its type by Slug (Commodity.Type), which never
// changes, so renaming a type leaves its items, filters and saved views as
// they are.
//
// A built-in type keeps an empty Name until a member renames it; the
// frontend shows its translated default name meanwhile. Names are unique in
// the group regardless of case, Unicode normalization and runs of spaces:
// NameKey holds the folded name, the English default for a built-in type
// with no name of its own, and carries the unique index.
//
// A group always has at least one type, so a group with none has never been
// seeded. The registries seed the built-in types on the first access to such
// a group, and a startup backfill seeds the groups nobody has opened yet.
//
//ptah:schema:rls:enable table="item_types" comment="Enable RLS for multi-tenant item type isolation"
//ptah:schema:rls:policy name="item_type_isolation" table="item_types" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures item types can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="item_type_background_worker_access" table="item_types" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all item types for processing"
//ptah:schema:table name="item_types"
type ItemType struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// Slug identifies the type inside its group and is what items store.
	// The registry sets it on create and keeps it on every update.
	//ptah:schema:field name="slug" type="TEXT" not_null="true"
	Slug CommodityType `json:"slug" db:"slug" swaggertype:"string" example:"vehicle"`

	// Name is the type's name as a member typed it, or empty for a built-in
	// type that keeps its default name.
	//ptah:schema:field name="name" type="TEXT" not_null="true" default=""
	Name string `json:"name" db:"name" example:"Vehicle"`

	// NameKey is ItemTypeNameKey(DisplayName()). The registries set it on
	// every write; it is not part of the API.
	//ptah:schema:field name="name_key" type="TEXT" not_null="true"
	NameKey string `json:"-" db:"name_key" userinput:"false"`

	// Icon is the key of the icon the frontend shows for the type, or empty
	// for the generic one.
	//ptah:schema:field name="icon" type="TEXT" not_null="true" default=""
	Icon string `json:"icon" db:"icon" example:"car"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// ItemTypeIndexes defines the indexes of the item_types table.
type ItemTypeIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//ptah:schema:index name="idx_item_types_uuid" fields="uuid" unique="true" table="item_types"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//ptah:schema:index name="idx_item_types_tenant_group" fields="tenant_id,group_id" table="item_types"
	_ int

	// One type per slug in a group; items refer to a type by its slug.
	//ptah:schema:index name="idx_item_types_group_slug" fields="group_id,slug" unique="true" table="item_types"
	_ int

	// One type per name in a group; see ItemTypeNameKey.
	//ptah:schema:index name="idx_item_types_group_name_key" fields="group_id,name_key" unique="true" table="item_types"
	_ int
}

// ItemTypeNameKey folds an item type name for the uniqueness check, by the
// rule seller names follow (SellerNameKey): Unicode NFC, full case folding,
// and every run of white space reduced to one space with none at either end.
// "Vehicle", "VEHICLE" and " vehicle " give the same key.
func ItemTypeNameKey(name string) string {
	return SellerNameKey(name)
}

// ItemTypeSlugFromName makes the slug of a new type from its name, the way a
// tag slug is made from its label (NormalizeTagSlug), cut to
// MaxItemTypeSlugRunes. The result is empty when the name has no letter or
// digit; the registry then falls back to "type".
func ItemTypeSlugFromName(name string) CommodityType {
	slug := NormalizeTagSlug(name)
	if utf8.RuneCountInString(slug) > MaxItemTypeSlugRunes {
		slug = strings.TrimRight(string([]rune(slug)[:MaxItemTypeSlugRunes]), "-")
	}
	return CommodityType(slug)
}

// NewBuiltinItemType returns the seed row of a built-in type, without the
// group-scoped identity the registry fills in.
func NewBuiltinItemType(b BuiltinItemType) ItemType {
	t := ItemType{Slug: b.Slug, Icon: b.Icon}
	t.Normalize()
	return t
}

// Builtin reports whether the type is one of the built-in ones.
func (t *ItemType) Builtin() bool {
	return t.Slug.IsBuiltin()
}

// DisplayName is the name the type goes by in the API's own texts (the AI
// scan prompt, uniqueness): Name, or the English default name of a built-in
// type that has none.
func (t *ItemType) DisplayName() string {
	if t.Name != "" {
		return t.Name
	}
	if b, ok := BuiltinItemTypeFor(t.Slug); ok {
		return b.Name
	}
	return string(t.Slug)
}

// Normalize trims the fields and recomputes NameKey. The registries call it
// before every write.
func (t *ItemType) Normalize() {
	t.Name = strings.Join(strings.Fields(t.Name), " ")
	t.Icon = strings.TrimSpace(t.Icon)
	t.NameKey = ItemTypeNameKey(t.DisplayName())
}

func (*ItemType) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext validates the whole row, the group-scoped identity
// included.
func (t *ItemType) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, t,
		validation.Field(&t.TenantGroupAwareEntityID),
		validation.Field(&t.Slug, validation.Required),
	); err != nil {
		return err
	}
	return t.ValidateFields(ctx)
}

// ValidateFields validates the fields a member sets, before the registry
// fills in the slug, tenant, group and author. A built-in type may have an
// empty name; any other type needs one.
func (t *ItemType) ValidateFields(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, t,
		validation.Field(&t.Name,
			validation.When(!t.Builtin(), validation.Required),
			validation.RuneLength(0, MaxItemTypeNameRunes)),
		validation.Field(&t.Icon,
			validation.RuneLength(0, MaxItemTypeIconRunes),
			validation.Match(itemTypeIconPattern)),
	)
}
