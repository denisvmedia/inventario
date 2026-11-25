package models

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"maps"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jellydator/validation"
	"github.com/shopspring/decimal"
)

// Limits on custom field values. Text counts runes.
const (
	MaxCustomFieldTextRunes     = 200
	MaxCustomFieldLongTextRunes = 10000
	MaxCustomFieldURLRunes      = 2048
	MaxCustomFieldItemIDRunes   = 64
	// A number has at most 15 digits before the decimal point and 10
	// after it; an amount of money at most 13 before and 2 after, as the
	// item prices.
	maxCustomFieldNumberIntDigits  = 15
	maxCustomFieldNumberFracDigits = 10
	maxCustomFieldMoneyIntDigits   = 13
	maxCustomFieldMoneyFracDigits  = 2
)

// CustomFieldValues are an item's custom field values as the API carries
// them, keyed by the ID of the field's definition. CustomFieldData.JSON
// gives the form of each type.
//
// On a write, a nil map leaves every stored value as it is, a key left out
// keeps that field's value, and a JSON null clears it. A read returns every
// value the item has and nothing for a field without one, so "no value" is
// never confused with a zero, false or empty one.
type CustomFieldValues map[string]json.RawMessage

// Clone returns a copy of v that shares no bytes with it.
func (v CustomFieldValues) Clone() CustomFieldValues {
	if v == nil {
		return nil
	}
	out := make(CustomFieldValues, len(v))
	for k, raw := range v {
		out[k] = bytes.Clone(raw)
	}
	return out
}

// IsNullJSON reports whether raw is a JSON null (or empty), which clears a
// value on a write.
func IsNullJSON(raw json.RawMessage) bool {
	trimmed := bytes.TrimSpace(raw)
	return len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null"))
}

// CustomFieldMoney is the API form of a money value.
type CustomFieldMoney struct {
	Amount   json.Number `json:"amount"`
	Currency Currency    `json:"currency"`
}

// CustomFieldData is the typed value of one custom field. Exactly the
// column of the field's type is set: Text for text and URL fields, LongText,
// Number, Date, Bool, Select (an option ID), Money with Currency, or ItemID
// for a link to another item. Each type has its own column so the
// registries filter and sort by the typed value, and an index on
// (field_id, column) serves the range filters.
type CustomFieldData struct {
	//ptah:schema:field name="value_text" type="TEXT"
	Text *string `json:"-" db:"value_text"`
	//ptah:schema:field name="value_long_text" type="TEXT"
	LongText *string `json:"-" db:"value_long_text"`
	//ptah:schema:field name="value_number" type="NUMERIC"
	Number *decimal.Decimal `json:"-" db:"value_number"`
	//ptah:schema:field name="value_date" type="DATE"
	Date *Date `json:"-" db:"value_date"`
	//ptah:schema:field name="value_bool" type="BOOLEAN"
	Bool *bool `json:"-" db:"value_bool"`
	//ptah:schema:field name="value_select" type="TEXT"
	Select *string `json:"-" db:"value_select"`
	//ptah:schema:field name="value_money" type="DECIMAL(15,2)"
	Money *decimal.Decimal `json:"-" db:"value_money"`
	//ptah:schema:field name="value_currency" type="TEXT"
	Currency *Currency `json:"-" db:"value_currency"`
	// ItemID links to another item of the group. Purging that item deletes
	// the value; moving it to Trash keeps it.
	//ptah:schema:field name="value_item_id" type="TEXT" foreign="commodities(id)" foreign_key_name="fk_custom_field_value_item" on_delete="CASCADE"
	ItemID *string `json:"-" db:"value_item_id"`
}

// JSON returns the API form of v for a field of type t: a string for text,
// long text, URL, date (YYYY-MM-DD), select (the option ID) and item link
// (the item ID); a number for a number; true or false for yes/no; and
// {"amount": number, "currency": "EUR"} for money. It returns nil when v
// holds no value of type t.
func (v CustomFieldData) JSON(t CustomFieldType) json.RawMessage {
	str := func(p *string) json.RawMessage {
		if p == nil {
			return nil
		}
		b, _ := json.Marshal(*p)
		return b
	}
	switch t {
	case CustomFieldTypeText, CustomFieldTypeURL:
		return str(v.Text)
	case CustomFieldTypeLongText:
		return str(v.LongText)
	case CustomFieldTypeSelect:
		return str(v.Select)
	case CustomFieldTypeItemLink:
		return str(v.ItemID)
	case CustomFieldTypeDate:
		if v.Date == nil || *v.Date == "" {
			return nil
		}
		return str(new(string(*v.Date)))
	case CustomFieldTypeNumber:
		if v.Number == nil {
			return nil
		}
		return json.RawMessage(v.Number.String())
	case CustomFieldTypeBoolean:
		if v.Bool == nil {
			return nil
		}
		b, _ := json.Marshal(*v.Bool)
		return b
	case CustomFieldTypeMoney:
		if v.Money == nil || v.Currency == nil {
			return nil
		}
		b, _ := json.Marshal(CustomFieldMoney{Amount: json.Number(v.Money.StringFixed(maxCustomFieldMoneyFracDigits)), Currency: *v.Currency})
		return b
	}
	return nil
}

// SortText returns the text a sort by a text or URL field compares, folded
// to lower case.
func (v CustomFieldData) SortText() string {
	if v.Text == nil {
		return ""
	}
	return strings.ToLower(*v.Text)
}

// ParseValue reads the API form of a value of field d (see
// CustomFieldData.JSON) and checks it against the field's type, limits and
// options. raw must not be null; a null clears the value and never reaches
// here. A text value is trimmed, and an empty one is a value, distinct from
// none. Whether a linked item exists is the registries' check.
func (d *CustomFieldDefinition) ParseValue(raw json.RawMessage) (CustomFieldData, error) {
	var out CustomFieldData
	switch d.Type {
	case CustomFieldTypeText, CustomFieldTypeLongText:
		return parseCustomFieldText(d.Type, raw)
	case CustomFieldTypeURL:
		s, err := parseCustomFieldURL(raw)
		if err != nil {
			return out, err
		}
		out.Text = &s
	case CustomFieldTypeNumber:
		n, err := parseCustomFieldDecimal(raw, maxCustomFieldNumberIntDigits, maxCustomFieldNumberFracDigits)
		if err != nil {
			return out, err
		}
		out.Number = &n
	case CustomFieldTypeDate:
		s, err := parseCustomFieldString(raw)
		if err != nil {
			return out, err
		}
		if _, err := time.Parse(time.DateOnly, s); err != nil {
			return out, validation.NewError("custom_field_invalid_date", "must be a date as YYYY-MM-DD")
		}
		out.Date = new(Date(s))
	case CustomFieldTypeBoolean:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return out, validation.NewError("custom_field_invalid_boolean", "must be true or false")
		}
		out.Bool = &b
	case CustomFieldTypeSelect:
		s, err := parseCustomFieldString(raw)
		if err != nil {
			return out, err
		}
		if _, ok := d.Option(s); !ok {
			return out, validation.NewError("custom_field_unknown_option", "is not one of the field's options")
		}
		out.Select = &s
	case CustomFieldTypeMoney:
		return parseCustomFieldMoney(raw)
	case CustomFieldTypeItemLink:
		return parseCustomFieldItemLink(raw)
	default:
		return out, validation.NewError("invalid_custom_field_type", "unknown custom field type")
	}
	return out, nil
}

// parseCustomFieldText reads a text value, trimmed, or a long text value,
// kept as written.
func parseCustomFieldText(t CustomFieldType, raw json.RawMessage) (CustomFieldData, error) {
	var out CustomFieldData
	s, err := parseCustomFieldString(raw)
	if err != nil {
		return out, err
	}
	limit := MaxCustomFieldLongTextRunes
	if t == CustomFieldTypeText {
		s = strings.TrimSpace(s)
		limit = MaxCustomFieldTextRunes
	}
	if utf8.RuneCountInString(s) > limit {
		return out, validation.NewError("custom_field_too_long", fmt.Sprintf("must be at most %d characters", limit))
	}
	if t == CustomFieldTypeText {
		out.Text = &s
	} else {
		out.LongText = &s
	}
	return out, nil
}

func parseCustomFieldItemLink(raw json.RawMessage) (CustomFieldData, error) {
	var out CustomFieldData
	s, err := parseCustomFieldString(raw)
	if err != nil {
		return out, err
	}
	s = strings.TrimSpace(s)
	if s == "" || utf8.RuneCountInString(s) > MaxCustomFieldItemIDRunes {
		return out, validation.NewError("custom_field_invalid_item", "must be the ID of an item")
	}
	out.ItemID = &s
	return out, nil
}

func parseCustomFieldString(raw json.RawMessage) (string, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", validation.NewError("custom_field_invalid_text", "must be a string")
	}
	return s, nil
}

func parseCustomFieldURL(raw json.RawMessage) (string, error) {
	s, err := parseCustomFieldString(raw)
	if err != nil {
		return "", err
	}
	s = strings.TrimSpace(s)
	invalid := validation.NewError("custom_field_invalid_url", "must be an http or https address")
	if utf8.RuneCountInString(s) > MaxCustomFieldURLRunes {
		return "", invalid
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "", invalid
	}
	return s, nil
}

// parseCustomFieldDecimal reads a JSON number, or a string holding one, and
// checks its digits against the limits.
func parseCustomFieldDecimal(raw json.RawMessage, intDigits, fracDigits int32) (decimal.Decimal, error) {
	invalid := validation.NewError("custom_field_invalid_number", "must be a number")
	text := strings.TrimSpace(string(raw))
	if strings.HasPrefix(text, `"`) {
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return decimal.Zero, invalid
		}
		text = strings.TrimSpace(s)
	}
	if text == "" {
		return decimal.Zero, invalid
	}
	n, err := decimal.NewFromString(text)
	if err != nil {
		return decimal.Zero, invalid
	}
	if -n.Exponent() > fracDigits && !n.Equal(n.Truncate(fracDigits)) {
		return decimal.Zero, validation.NewError("custom_field_too_precise", fmt.Sprintf("must have at most %d decimal places", fracDigits))
	}
	if n.Abs().GreaterThanOrEqual(decimal.New(1, intDigits)) {
		return decimal.Zero, validation.NewError("custom_field_too_large", fmt.Sprintf("must have at most %d digits before the decimal point", intDigits))
	}
	return n.Truncate(fracDigits), nil
}

func parseCustomFieldMoney(raw json.RawMessage) (CustomFieldData, error) {
	var out CustomFieldData
	invalid := validation.NewError("custom_field_invalid_money", `must be {"amount": number, "currency": "ISO code"}`)
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	var m struct {
		Amount   json.RawMessage `json:"amount"`
		Currency *string         `json:"currency"`
	}
	if err := dec.Decode(&m); err != nil || len(m.Amount) == 0 || m.Currency == nil {
		return out, invalid
	}
	amount, err := parseCustomFieldDecimal(m.Amount, maxCustomFieldMoneyIntDigits, maxCustomFieldMoneyFracDigits)
	if err != nil {
		return out, err
	}
	currency := Currency(strings.ToUpper(strings.TrimSpace(*m.Currency)))
	if !currency.IsValid() {
		return out, validation.NewError("custom_field_invalid_currency", "must have a known ISO 4217 currency code")
	}
	out.Money = &amount
	out.Currency = &currency
	return out, nil
}

var (
	_ validation.Validatable            = (*CustomFieldValue)(nil)
	_ validation.ValidatableWithContext = (*CustomFieldValue)(nil)
)

// CustomFieldValue is the value one item has for one custom field (#2803).
// An item has at most one value per field, and an item without a value for
// a field has no row: no value is never a zero, false or empty one. Deleting
// the item or the field deletes the value (ON DELETE CASCADE). The commodity
// registries write the rows inside the item's own transaction and return
// them on the item as Commodity.CustomFields; there is no other writer.
//
// The rows carry no author: they are part of the item, whoever edited it
// last.
//
//ptah:schema:rls:enable table="commodity_custom_field_values" comment="Enable RLS for multi-tenant custom field value isolation"
//ptah:schema:rls:policy name="commodity_custom_field_value_isolation" table="commodity_custom_field_values" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures custom field values can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="commodity_custom_field_value_background_worker_access" table="commodity_custom_field_values" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all custom field values for processing"
//ptah:schema:table name="commodity_custom_field_values"
type CustomFieldValue struct {
	//ptah:embedded mode="inline"
	TenantAwareEntityID

	//ptah:schema:field name="group_id" type="TEXT" not_null="true" foreign="location_groups(id)" foreign_key_name="fk_entity_group"
	GroupID string `json:"-" db:"group_id"`

	//ptah:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_custom_field_value_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	//ptah:schema:field name="field_id" type="TEXT" not_null="true" foreign="custom_field_definitions(id)" foreign_key_name="fk_custom_field_value_field" on_delete="CASCADE"
	FieldID string `json:"field_id" db:"field_id"`

	//ptah:embedded mode="inline"
	CustomFieldData

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CustomFieldValueIndexes defines the indexes of the
// commodity_custom_field_values table.
type CustomFieldValueIndexes struct {
	//ptah:schema:index name="idx_commodity_custom_field_values_uuid" fields="uuid" unique="true" table="commodity_custom_field_values"
	_ int

	//ptah:schema:index name="idx_commodity_custom_field_values_tenant_group" fields="tenant_id,group_id" table="commodity_custom_field_values"
	_ int

	// One value per item and field. Also serves reading an item's values.
	//ptah:schema:index name="idx_commodity_custom_field_values_commodity_field" fields="commodity_id,field_id" unique="true" table="commodity_custom_field_values"
	_ int

	// The typed filters and sorts of the item query, one index per value
	// column, each led by the field.
	//ptah:schema:index name="idx_commodity_custom_field_values_field_text" fields="field_id,value_text" table="commodity_custom_field_values"
	_ int
	//ptah:schema:index name="idx_commodity_custom_field_values_field_number" fields="field_id,value_number" table="commodity_custom_field_values"
	_ int
	//ptah:schema:index name="idx_commodity_custom_field_values_field_date" fields="field_id,value_date" table="commodity_custom_field_values"
	_ int
	//ptah:schema:index name="idx_commodity_custom_field_values_field_bool" fields="field_id,value_bool" table="commodity_custom_field_values"
	_ int
	//ptah:schema:index name="idx_commodity_custom_field_values_field_select" fields="field_id,value_select" table="commodity_custom_field_values"
	_ int
	//ptah:schema:index name="idx_commodity_custom_field_values_field_money" fields="field_id,value_money" table="commodity_custom_field_values"
	_ int
	//ptah:schema:index name="idx_commodity_custom_field_values_field_item" fields="field_id,value_item_id" table="commodity_custom_field_values"
	_ int

	// The free-text search matches text values as substrings.
	//ptah:schema:index name="idx_commodity_custom_field_values_text_trgm" fields="value_text" type="GIN" ops="gin_trgm_ops" table="commodity_custom_field_values"
	_ int
	//ptah:schema:index name="idx_commodity_custom_field_values_long_text_trgm" fields="value_long_text" type="GIN" ops="gin_trgm_ops" table="commodity_custom_field_values"
	_ int

	// The cascade when a linked item is purged looks the values up by item.
	//ptah:schema:index name="idx_commodity_custom_field_values_item" fields="value_item_id" table="commodity_custom_field_values"
	_ int
}

func (*CustomFieldValue) Validate() error {
	return ErrMustUseValidateWithContext
}

func (v *CustomFieldValue) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, v,
		validation.Field(&v.TenantID, validation.Required),
		validation.Field(&v.GroupID, validation.Required),
		validation.Field(&v.CommodityID, validation.Required),
		validation.Field(&v.FieldID, validation.Required),
	)
}

// JSONValue is a JSON document kept in a JSONB column as it is, or SQL NULL
// for a nil or JSON null value.
type JSONValue json.RawMessage

var (
	_ json.Marshaler   = JSONValue(nil)
	_ json.Unmarshaler = (*JSONValue)(nil)
	_ driver.Valuer    = JSONValue(nil)
)

// IsNull reports whether v holds no value: nil, empty or a JSON null.
func (v JSONValue) IsNull() bool {
	return IsNullJSON(json.RawMessage(v))
}

func (v JSONValue) MarshalJSON() ([]byte, error) {
	if v.IsNull() {
		return []byte("null"), nil
	}
	return []byte(v), nil
}

func (v *JSONValue) UnmarshalJSON(data []byte) error {
	if IsNullJSON(data) {
		*v = nil
		return nil
	}
	*v = JSONValue(bytes.Clone(data))
	return nil
}

func (v JSONValue) Value() (driver.Value, error) {
	if v.IsNull() {
		return nil, nil
	}
	return []byte(v), nil
}

func (v *JSONValue) Scan(src any) error {
	switch s := src.(type) {
	case nil:
		*v = nil
	case []byte:
		*v = JSONValue(bytes.Clone(s))
	case string:
		*v = JSONValue(s)
	default:
		return fmt.Errorf("cannot scan type %T into JSONValue", src)
	}
	if v.IsNull() {
		*v = nil
	}
	return nil
}

// SameCustomFieldValues reports whether two value maps hold the same values,
// comparing each value's JSON bytes.
func SameCustomFieldValues(a, b CustomFieldValues) bool {
	return maps.EqualFunc(a, b, func(x, y json.RawMessage) bool {
		return bytes.Equal(bytes.TrimSpace(x), bytes.TrimSpace(y))
	})
}
