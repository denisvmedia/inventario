package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable            = (*ItemTypeCustomField)(nil)
	_ validation.ValidatableWithContext = (*ItemTypeCustomField)(nil)
	_ TenantGroupAwareIDable            = (*ItemTypeCustomField)(nil)
)

// ItemTypeCustomField puts a custom field on an item type's template
// (#2803): the item form shows the field for an item of that type and
// prefills DefaultValue while the item has no value of its own. A template
// suggests; it never writes a value, so switching an item's type keeps every
// value it has, and a default never replaces one.
//
// Deleting the type or the field deletes the entry (ON DELETE CASCADE).
//
//ptah:schema:rls:enable table="item_type_custom_fields" comment="Enable RLS for multi-tenant item type template isolation"
//ptah:schema:rls:policy name="item_type_custom_field_isolation" table="item_type_custom_fields" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures item type templates can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="item_type_custom_field_background_worker_access" table="item_type_custom_fields" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all item type templates for processing"
//ptah:schema:table name="item_type_custom_fields"
type ItemTypeCustomField struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	//ptah:schema:field name="item_type_id" type="TEXT" not_null="true" foreign="item_types(id)" foreign_key_name="fk_item_type_custom_field_type" on_delete="CASCADE"
	ItemTypeID string `json:"item_type_id" db:"item_type_id"`

	//ptah:schema:field name="field_id" type="TEXT" not_null="true" foreign="custom_field_definitions(id)" foreign_key_name="fk_item_type_custom_field_field" on_delete="CASCADE"
	FieldID string `json:"field_id" db:"field_id"`

	// DefaultValue is the value the form prefills, in the API form of the
	// field's type (CustomFieldData.JSON), or null for none. A link to an
	// item has no default.
	//ptah:schema:field name="default_value" type="JSONB"
	DefaultValue JSONValue `json:"default_value" db:"default_value" swaggertype:"object"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"-" db:"created_at" userinput:"false"`
}

// ItemTypeCustomFieldIndexes defines the indexes of the
// item_type_custom_fields table.
type ItemTypeCustomFieldIndexes struct {
	//ptah:schema:index name="idx_item_type_custom_fields_uuid" fields="uuid" unique="true" table="item_type_custom_fields"
	_ int

	//ptah:schema:index name="idx_item_type_custom_fields_tenant_group" fields="tenant_id,group_id" table="item_type_custom_fields"
	_ int

	// A field is on a type's template once.
	//ptah:schema:index name="idx_item_type_custom_fields_type_field" fields="item_type_id,field_id" unique="true" table="item_type_custom_fields"
	_ int

	// The entries of one field, read and replaced with the field.
	//ptah:schema:index name="idx_item_type_custom_fields_field" fields="field_id" table="item_type_custom_fields"
	_ int
}

func (*ItemTypeCustomField) Validate() error {
	return ErrMustUseValidateWithContext
}

func (e *ItemTypeCustomField) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, e,
		validation.Field(&e.ItemTypeID, validation.Required),
		validation.Field(&e.FieldID, validation.Required),
	)
}

// ValidateDefault checks the entry's default value against field d: a
// default must be a valid value of the field, and a link to an item has
// none. A valid default is returned in its canonical form.
func (e *ItemTypeCustomField) ValidateDefault(d *CustomFieldDefinition) error {
	if e.DefaultValue.IsNull() {
		e.DefaultValue = nil
		return nil
	}
	if d.Type == CustomFieldTypeItemLink {
		return validation.NewError("custom_field_default_not_allowed", "a link to an item has no default value")
	}
	data, err := d.ParseValue([]byte(e.DefaultValue))
	if err != nil {
		return err
	}
	e.DefaultValue = JSONValue(data.JSON(d.Type))
	return nil
}
