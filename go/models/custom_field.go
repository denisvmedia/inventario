package models

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"maps"
	"regexp"
	"strings"
	"time"

	"github.com/jellydator/validation"
)

// CustomFieldType is the kind of value a custom field holds. A field's type
// never changes after it is created: the values of each type live in their
// own column (see CustomFieldData), so a new type would orphan them.
type CustomFieldType string

// The custom field types. Adding one? Update IsValid, CustomFieldData and
// the registries' filters and sorts.
const (
	CustomFieldTypeText     CustomFieldType = "text"
	CustomFieldTypeLongText CustomFieldType = "long_text"
	CustomFieldTypeNumber   CustomFieldType = "number"
	CustomFieldTypeDate     CustomFieldType = "date"
	CustomFieldTypeBoolean  CustomFieldType = "boolean"
	CustomFieldTypeSelect   CustomFieldType = "select"
	CustomFieldTypeURL      CustomFieldType = "url"
	CustomFieldTypeMoney    CustomFieldType = "money"
	CustomFieldTypeItemLink CustomFieldType = "item_link"
)

// CustomFieldTypes lists every custom field type, in the order the frontend
// offers them.
var CustomFieldTypes = []CustomFieldType{
	CustomFieldTypeText,
	CustomFieldTypeLongText,
	CustomFieldTypeNumber,
	CustomFieldTypeDate,
	CustomFieldTypeBoolean,
	CustomFieldTypeSelect,
	CustomFieldTypeURL,
	CustomFieldTypeMoney,
	CustomFieldTypeItemLink,
}

var _ validation.Validatable = CustomFieldType("")

// IsValid reports whether t is one of the custom field types.
func (t CustomFieldType) IsValid() bool {
	switch t {
	case CustomFieldTypeText, CustomFieldTypeLongText, CustomFieldTypeNumber,
		CustomFieldTypeDate, CustomFieldTypeBoolean, CustomFieldTypeSelect,
		CustomFieldTypeURL, CustomFieldTypeMoney, CustomFieldTypeItemLink:
		return true
	}
	return false
}

func (t CustomFieldType) Validate() error {
	if !t.IsValid() {
		return validation.NewError("invalid_custom_field_type", "unknown custom field type")
	}
	return nil
}

// Limits on a custom field definition. Names and labels count runes, so a
// Cyrillic or CJK name gets the same room as a Latin one. The frontend
// checks the same limits.
const (
	MaxCustomFieldNameRunes        = 60
	MaxCustomFieldOptions          = 100
	MaxCustomFieldOptionLabelRunes = 100
	MaxCustomFieldOptionIDRunes    = 32
	// MaxCustomFieldsPerGroup bounds how many fields a group defines; the
	// item form shows every field a value is set for.
	MaxCustomFieldsPerGroup = 200
)

// customFieldOptionIDPattern is the shape of a select option id: lowercase
// ASCII letters and digits. The server makes them; a client sends one back to
// keep an option across a rename.
var customFieldOptionIDPattern = regexp.MustCompile(`^[a-z0-9]+$`)

// CustomFieldOption is one choice of a select field. Values store the ID, so
// renaming the label keeps every value, filter and saved view that names it.
type CustomFieldOption struct {
	ID    string `json:"id" example:"a1b2c3d4"`
	Label string `json:"label" example:"Petrol"`
}

// NewCustomFieldOptionID returns a fresh option id: eight random lowercase
// hex digits.
func NewCustomFieldOptionID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

var (
	_ validation.Validatable            = (*CustomFieldDefinition)(nil)
	_ validation.ValidatableWithContext = (*CustomFieldDefinition)(nil)
	_ TenantGroupAwareIDable            = (*CustomFieldDefinition)(nil)
)

// CustomFieldDefinition is a field a group adds to its items (#2803): a
// name, a type, and the options of a select field. The type is fixed when
// the field is created. Items hold their values in CustomFieldValue rows,
// keyed by the definition's ID, so renaming a field keeps its values, the
// filters that name it and the list columns that show it. Deleting a
// definition deletes its values (ON DELETE CASCADE).
//
// Names are unique in the group regardless of case, Unicode normalization
// and runs of spaces: NameKey holds the folded name and carries the unique
// index.
//
//ptah:schema:rls:enable table="custom_field_definitions" comment="Enable RLS for multi-tenant custom field isolation"
//ptah:schema:rls:policy name="custom_field_definition_isolation" table="custom_field_definitions" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures custom field definitions can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="custom_field_definition_background_worker_access" table="custom_field_definitions" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all custom field definitions for processing"
//ptah:schema:table name="custom_field_definitions"
type CustomFieldDefinition struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	//ptah:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name" example:"License plate"`

	// NameKey is CustomFieldNameKey(Name). The registries set it on every
	// write; it is not part of the API.
	//ptah:schema:field name="name_key" type="TEXT" not_null="true"
	NameKey string `json:"-" db:"name_key" userinput:"false"`

	// Type is the kind of value the field holds. The registries keep the
	// stored type on every update.
	//ptah:schema:field name="type" type="TEXT" not_null="true"
	Type CustomFieldType `json:"type" db:"type" swaggertype:"string" example:"text"`

	// Options are the choices of a select field, in the order the form
	// offers them and a sort by the field follows. Empty for other types.
	//ptah:schema:field name="options" type="JSONB"
	Options ValuerSlice[CustomFieldOption] `json:"options" db:"options"`

	// SetEntry is the entry of the item set that created the field
	// (#1607), or empty for a field a member added. The registries keep the
	// stored value on every update; it is not part of the API.
	//ptah:schema:field name="set_entry" type="TEXT" not_null="true" default=""
	SetEntry ItemSetEntry `json:"-" db:"set_entry" userinput:"false"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`

	// Templates are the item types whose template shows the field, each
	// with the default the form prefills (ItemTypeCustomField rows, not a
	// column). The registries read them with the field. On Create and
	// Update they replace the stored entries in the same transaction, except
	// that a nil slice on Update leaves them as they are.
	Templates []CustomFieldTemplate `json:"templates" db:"-"`
}

// CustomFieldTemplate is one item type whose template shows a field, see
// ItemTypeCustomField.
type CustomFieldTemplate struct {
	ItemTypeID   string    `json:"item_type_id" example:"8f1c..."`
	DefaultValue JSONValue `json:"default_value"`
}

// CustomFieldDefinitionIndexes defines the indexes of the
// custom_field_definitions table.
type CustomFieldDefinitionIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//ptah:schema:index name="idx_custom_field_definitions_uuid" fields="uuid" unique="true" table="custom_field_definitions"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//ptah:schema:index name="idx_custom_field_definitions_tenant_group" fields="tenant_id,group_id" table="custom_field_definitions"
	_ int

	// One field per name in a group; see CustomFieldNameKey.
	//ptah:schema:index name="idx_custom_field_definitions_group_name_key" fields="group_id,name_key" unique="true" table="custom_field_definitions"
	_ int
}

// CustomFieldNameKey folds a field name for the uniqueness check, by the
// rule seller names follow (SellerNameKey).
func CustomFieldNameKey(name string) string {
	return SellerNameKey(name)
}

// Normalize trims the name and the option labels, drops the options of a
// field that is not a select, gives every option without an ID a fresh one,
// and recomputes NameKey. The registries call it before every write.
func (d *CustomFieldDefinition) Normalize() {
	d.Name = strings.Join(strings.Fields(d.Name), " ")
	d.NameKey = CustomFieldNameKey(d.Name)
	if d.Type != CustomFieldTypeSelect {
		d.Options = nil
		return
	}
	options := make(ValuerSlice[CustomFieldOption], 0, len(d.Options))
	used := make(map[string]bool, len(d.Options))
	for _, o := range d.Options {
		used[o.ID] = true
	}
	for _, o := range d.Options {
		o.Label = strings.Join(strings.Fields(o.Label), " ")
		o.ID = strings.TrimSpace(o.ID)
		for o.ID == "" {
			if id := NewCustomFieldOptionID(); !used[id] {
				o.ID = id
				used[id] = true
			}
		}
		options = append(options, o)
	}
	d.Options = options
}

// Option returns the option with the given ID.
func (d *CustomFieldDefinition) Option(id string) (CustomFieldOption, bool) {
	for _, o := range d.Options {
		if o.ID == id {
			return o, true
		}
	}
	return CustomFieldOption{}, false
}

// OptionIDs returns the IDs of the options, in their order.
func (d *CustomFieldDefinition) OptionIDs() []string {
	ids := make([]string, 0, len(d.Options))
	for _, o := range d.Options {
		ids = append(ids, o.ID)
	}
	return ids
}

func (*CustomFieldDefinition) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext validates the whole row, the group-scoped identity
// included.
func (d *CustomFieldDefinition) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.TenantGroupAwareEntityID),
	); err != nil {
		return err
	}
	return d.ValidateFields(ctx)
}

// ValidateFields validates the fields a member sets, before the registry
// fills in the tenant, group and author. A select field needs at least one
// option; labels are required and unique in the field, ignoring case and
// runs of spaces, and so are option IDs.
func (d *CustomFieldDefinition) ValidateFields(ctx context.Context) error {
	errs := validation.Errors{}
	if err := validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.Name, validation.Required, validation.RuneLength(1, MaxCustomFieldNameRunes), validation.By(refuseNUL)),
		validation.Field(&d.Type, validation.Required),
	); err != nil {
		fieldErrs, ok := errors.AsType[validation.Errors](err)
		if !ok {
			return err
		}
		maps.Copy(errs, fieldErrs)
	}
	// Checked outside ValidateStruct, which would skip an empty slice and
	// so never report the missing options of a select field.
	if err := d.validateOptions(); err != nil {
		errs["options"] = err
	} else if err := d.validateTemplates(); err != nil {
		errs["templates"] = err
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// refuseNUL refuses a string with a NUL character, which a PostgreSQL text
// column cannot store.
func refuseNUL(v any) error {
	if s, _ := v.(string); strings.ContainsRune(s, 0) {
		return validation.NewError("custom_field_invalid_character", "must not contain a NUL character")
	}
	return nil
}

func (d *CustomFieldDefinition) validateOptions() error {
	if d.Type != CustomFieldTypeSelect {
		if len(d.Options) > 0 {
			return validation.NewError("options_not_allowed", "only a select field has options")
		}
		return nil
	}
	if len(d.Options) == 0 {
		return validation.NewError("options_required", "a select field needs at least one option")
	}
	if len(d.Options) > MaxCustomFieldOptions {
		return validation.NewError("too_many_options", "a select field has at most 100 options")
	}
	labels := make(map[string]bool, len(d.Options))
	ids := make(map[string]bool, len(d.Options))
	for _, o := range d.Options {
		if err := validation.Validate(o.Label, validation.Required, validation.RuneLength(1, MaxCustomFieldOptionLabelRunes)); err != nil {
			return validation.NewError("invalid_option_label", "every option needs a label of at most 100 characters")
		}
		if err := refuseNUL(o.Label); err != nil {
			return err
		}
		if err := validation.Validate(o.ID, validation.Required,
			validation.RuneLength(1, MaxCustomFieldOptionIDRunes), validation.Match(customFieldOptionIDPattern)); err != nil {
			return validation.NewError("invalid_option_id", "an option id is up to 32 lowercase letters and digits")
		}
		key := CustomFieldNameKey(o.Label)
		if labels[key] {
			return validation.NewError("duplicate_option_label", "two options have the same label")
		}
		labels[key] = true
		if ids[o.ID] {
			return validation.NewError("duplicate_option_id", "two options have the same id")
		}
		ids[o.ID] = true
	}
	return nil
}

// ErrCustomFieldTypeChanged is the field error of an update that names a
// different type than the stored one.
var ErrCustomFieldTypeChanged = validation.NewError("custom_field_type_immutable", "the type of a custom field cannot change")

// validateTemplates checks the template entries: each names an item type,
// none twice, and its default is a valid value of the field, which it
// rewrites into its canonical form. Whether the item type exists in the
// group is the registries' check.
func (d *CustomFieldDefinition) validateTemplates() error {
	seen := make(map[string]bool, len(d.Templates))
	for i := range d.Templates {
		t := &d.Templates[i]
		t.ItemTypeID = strings.TrimSpace(t.ItemTypeID)
		if t.ItemTypeID == "" {
			return validation.NewError("template_item_type_required", "every template entry names an item type")
		}
		if seen[t.ItemTypeID] {
			return validation.NewError("duplicate_template_item_type", "an item type is listed twice")
		}
		seen[t.ItemTypeID] = true
		entry := ItemTypeCustomField{DefaultValue: t.DefaultValue}
		if err := entry.ValidateDefault(d); err != nil {
			return err
		}
		t.DefaultValue = entry.DefaultValue
	}
	return nil
}
