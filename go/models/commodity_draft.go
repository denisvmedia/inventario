package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"maps"
	"strings"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*CommodityDraft)(nil)
	_ validation.ValidatableWithContext = (*CommodityDraft)(nil)
	_ TenantGroupAwareIDable            = (*CommodityDraft)(nil)
)

// CommodityDraftStatus is the lifecycle state of a draft.
type CommodityDraftStatus string

const (
	// CommodityDraftOpen is a draft that can be resumed, saved, discarded or
	// promoted.
	CommodityDraftOpen CommodityDraftStatus = "open"
	// CommodityDraftPromoted is a draft that became an item. Its row stays,
	// with the values cleared, so a repeated promotion with the same
	// promotion key returns the same item.
	CommodityDraftPromoted CommodityDraftStatus = "promoted"
	// CommodityDraftDiscarding is a draft whose discard has begun: its files
	// are being sent to Trash or released, and the row is deleted last. It
	// cannot be saved, promoted or given files, and no list shows it. A
	// repeated discard finishes the work.
	CommodityDraftDiscarding CommodityDraftStatus = "discarding"
)

// Limits on a draft's client-supplied values.
const (
	// CommodityDraftClientKeyMaxLength caps the idempotency key.
	CommodityDraftClientKeyMaxLength = 128
	// CommodityDraftPromotionKeyMaxLength caps the promotion key.
	CommodityDraftPromotionKeyMaxLength = 128
	// CommodityDraftValuesMaxBytes caps the encoded size of the form values.
	// The item form produces a few kilobytes at most.
	CommodityDraftValuesMaxBytes = 64 * 1024
	// CommodityDraftValuesMaxKeys caps the number of top-level fields.
	CommodityDraftValuesMaxKeys = 100
	// CommodityDraftTitleMaxLength caps the title derived from the values.
	CommodityDraftTitleMaxLength = 255
)

// CommodityDraftValues is the unfinished form state of a draft, stored as
// JSONB. The server does not validate it as an item: it may lack a name, a
// type or anything else a finished item requires. Promotion validates the
// complete item instead.
type CommodityDraftValues map[string]any

// Value implements driver.Valuer. An empty map is stored as an empty JSON
// object, never as NULL.
func (v CommodityDraftValues) Value() (driver.Value, error) {
	if v == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(map[string]any(v))
}

// Scan implements sql.Scanner. NULL and empty input give an empty map.
func (v *CommodityDraftValues) Scan(value any) error {
	var raw []byte
	switch val := value.(type) {
	case nil:
		*v = CommodityDraftValues{}
		return nil
	case []byte:
		raw = val
	case string:
		raw = []byte(val)
	default:
		return fmt.Errorf("cannot scan %T into CommodityDraftValues", value)
	}
	if len(raw) == 0 {
		*v = CommodityDraftValues{}
		return nil
	}
	out := CommodityDraftValues{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return err
	}
	*v = out
	return nil
}

// Title returns the draft's display title: the trimmed "name" value, cut to
// CommodityDraftTitleMaxLength runes. Empty when the form has no name yet.
func (v CommodityDraftValues) Title() string {
	name, _ := v["name"].(string)
	name = strings.TrimSpace(name)
	if runes := []rune(name); len(runes) > CommodityDraftTitleMaxLength {
		name = string(runes[:CommodityDraftTitleMaxLength])
	}
	return name
}

// MapDraftCustomFieldReference returns an entry of a draft's custom_fields
// with the id it holds replaced by mapID's result. The entry is the id
// itself, as an item stores a select option or a linked item, or an object
// whose "value" is the id, as the item form keeps it. An entry without an id
// is returned as it is; nil means mapID knows no such id or the entry is
// neither shape.
func MapDraftCustomFieldReference(entry any, mapID func(string) string) any {
	switch e := entry.(type) {
	case string:
		if e == "" {
			return e
		}
		if id := mapID(e); id != "" {
			return id
		}
	case map[string]any:
		ref, _ := e["value"].(string)
		if ref == "" {
			return e
		}
		if id := mapID(ref); id != "" {
			out := maps.Clone(e)
			out["value"] = id
			return out
		}
	}
	return nil
}

// CommodityDraftValuesRule checks the size and the number of fields of the
// values, and the type of each field the item form knows. The request
// validation and the model share it.
var CommodityDraftValuesRule = validation.By(func(value any) error {
	v, ok := value.(CommodityDraftValues)
	if !ok {
		return nil
	}
	if len(v) > CommodityDraftValuesMaxKeys {
		return validation.NewError("validation_draft_values_keys", fmt.Sprintf("a draft can hold at most %d fields", CommodityDraftValuesMaxKeys))
	}
	encoded, err := json.Marshal(map[string]any(v))
	if err != nil {
		return validation.NewError("validation_draft_values_encoding", "the draft values cannot be encoded")
	}
	if len(encoded) > CommodityDraftValuesMaxBytes {
		return validation.NewError("validation_draft_values_size", fmt.Sprintf("the draft values must not exceed %d bytes", CommodityDraftValuesMaxBytes))
	}
	return checkDraftValueTypes(v)
})

// draftValueKind is the JSON type a known field of the item form holds.
type draftValueKind int

const (
	draftText draftValueKind = iota
	draftNumber
	draftList
	draftFlag
	draftObject
)

// draftValueKinds lists the fields of the item form. A draft can hold other
// fields too, within the size limits; these must have the type the form
// reads, so a draft written through the API opens in the form. A number
// field may be a JSON number or the string the form keeps while it is typed.
var draftValueKinds = map[string]draftValueKind{
	"name":                     draftText,
	"short_name":               draftText,
	"brand":                    draftText,
	"model":                    draftText,
	"type":                     draftText,
	"area_id":                  draftText,
	"seller_id":                draftText,
	"status":                   draftText,
	"original_price_currency":  draftText,
	"serial_number":            draftText,
	"purchase_date":            draftText,
	"comments":                 draftText,
	"warranty_expires_at":      draftText,
	"warranty_notes":           draftText,
	"count":                    draftNumber,
	"original_price":           draftNumber,
	"converted_original_price": draftNumber,
	"current_price":            draftNumber,
	"extra_serial_numbers":     draftList,
	"part_numbers":             draftList,
	"tags":                     draftList,
	"urls":                     draftList,
	"draft":                    draftFlag,
	// The custom field values by field id (#2803); each entry is the
	// form's {value, currency} or a value as an item stores it.
	"custom_fields": draftObject,
}

// checkDraftValueTypes returns the known fields whose value has the wrong
// type, keyed by field.
func checkDraftValueTypes(v CommodityDraftValues) error {
	errs := validation.Errors{}
	for key, value := range v {
		kind, known := draftValueKinds[key]
		if !known {
			continue
		}
		if err := checkDraftValueKind(kind, value); err != nil {
			errs[key] = err
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

func checkDraftValueKind(kind draftValueKind, value any) error {
	switch kind {
	case draftText:
		if _, ok := value.(string); !ok {
			return validation.NewError("validation_draft_value_text", "must be a string")
		}
	case draftNumber:
		switch value.(type) {
		case string, float64, json.Number:
		default:
			return validation.NewError("validation_draft_value_number", "must be a number or a string")
		}
	case draftList:
		list, ok := value.([]any)
		if !ok {
			return validation.NewError("validation_draft_value_list", "must be a list of strings")
		}
		for _, item := range list {
			if _, ok := item.(string); !ok {
				return validation.NewError("validation_draft_value_list", "must be a list of strings")
			}
		}
	case draftFlag:
		if _, ok := value.(bool); !ok {
			return validation.NewError("validation_draft_value_flag", "must be true or false")
		}
	case draftObject:
		if _, ok := value.(map[string]any); !ok {
			return validation.NewError("validation_draft_value_object", "must be an object")
		}
	}
	return nil
}

// clientKeyRule accepts printable ASCII without spaces, so a key can be a
// UUID or a composite such as "intake:<session>:<candidate>".
var clientKeyRule = validation.By(func(value any) error {
	key, _ := value.(string)
	if !isDraftKey(key) {
		return validation.NewError("validation_draft_client_key", "the client key must be printable ASCII without spaces")
	}
	return nil
})

// PromotionKeyRule checks a promotion key: 1 to
// CommodityDraftPromotionKeyMaxLength printable ASCII characters without
// spaces, such as a UUID.
var PromotionKeyRule = validation.By(func(value any) error {
	key, _ := value.(string)
	if key == "" {
		return validation.NewError("validation_required", "cannot be blank")
	}
	if len(key) > CommodityDraftPromotionKeyMaxLength || !isDraftKey(key) {
		return validation.NewError("validation_draft_promotion_key",
			fmt.Sprintf("the promotion key must be at most %d printable ASCII characters without spaces", CommodityDraftPromotionKeyMaxLength))
	}
	return nil
})

func isDraftKey(key string) bool {
	for _, r := range key {
		if r <= ' ' || r > '~' {
			return false
		}
	}
	return true
}

// CommodityDraft is an unfinished item saved on the server (#2812). It holds
// the form values as entered, which need not form a valid item, and the files
// chosen for it, attached through file_links with entity type "draft".
// Promotion turns it into exactly one item and moves those links to the item.
//
// A draft belongs to its group: every member can see it, members who can edit
// content can save, discard and promote it. Drafts are not items, so no item
// list, count, search, report or reminder sees them. A whole-group backup
// carries the open ones in a document of their own.
//
// ClientKey is the client's identity for the draft. Creating a draft with a key
// the group already has returns the existing draft, so a repeated create after
// a lost response does not make a second draft. Revision increases with every
// save; a save names the revision it was based on and is refused when the
// draft has moved on, so a stale tab cannot overwrite newer input.
//
//ptah:schema:rls:enable table="commodity_drafts" comment="Enable RLS for multi-tenant commodity draft isolation"
//ptah:schema:rls:policy name="commodity_draft_isolation" table="commodity_drafts" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodity drafts can only be accessed and modified by their tenant and group with required contexts"
//ptah:schema:rls:policy name="commodity_draft_background_worker_access" table="commodity_drafts" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to read and purge commodity drafts across all groups"
//ptah:schema:table name="commodity_drafts"
type CommodityDraft struct {
	//ptah:embedded mode="inline"
	TenantGroupAwareEntityID

	// ClientKey identifies the draft for the client that created it; unique
	// in the group.
	//ptah:schema:field name="client_key" type="TEXT" not_null="true"
	ClientKey string `json:"client_key" db:"client_key"`

	// Values is the form state as entered.
	//ptah:schema:field name="form_values" type="JSONB" not_null="true"
	Values CommodityDraftValues `json:"values" db:"form_values"`

	// Revision starts at 1 and increases with every save and the promotion.
	//ptah:schema:field name="revision" type="INTEGER" not_null="true" default="1"
	Revision int `json:"revision" db:"revision" userinput:"false"`

	// Status is open until the draft is promoted or its discard begins.
	//ptah:schema:field name="status" type="TEXT" not_null="true" default="open"
	Status CommodityDraftStatus `json:"status" db:"status" userinput:"false"`

	// PromotedCommodityID is the item the draft became. The item can be
	// deleted later, so there is no foreign key.
	//ptah:schema:field name="promoted_commodity_id" type="TEXT"
	PromotedCommodityID *string `json:"promoted_commodity_id,omitempty" db:"promoted_commodity_id" userinput:"false"`

	//ptah:schema:field name="promoted_at" type="TIMESTAMP"
	PromotedAt *time.Time `json:"promoted_at,omitempty" db:"promoted_at" userinput:"false"`

	// PromotionKey is the key the promotion was made with. Only a repeat with
	// the same key gets the item back; any other caller learns that the
	// draft is already an item, so a second device does not take the first
	// one's item for its own.
	//ptah:schema:field name="promotion_key" type="TEXT"
	PromotionKey *string `json:"-" db:"promotion_key" userinput:"false"`

	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// CommodityDraftIndexes defines the postgres indexes for commodity_drafts.
type CommodityDraftIndexes struct {
	// Unique index for the immutable UUID.
	//ptah:schema:index name="idx_commodity_drafts_uuid" fields="uuid" unique="true" table="commodity_drafts"
	_ int

	// One draft per client key in a group: the idempotency of create.
	//ptah:schema:index name="idx_commodity_drafts_group_client_key" fields="group_id,client_key" unique="true" table="commodity_drafts"
	_ int

	// Composite index for tenant+group RLS-filtered queries and the purges.
	//ptah:schema:index name="idx_commodity_drafts_tenant_group" fields="tenant_id,group_id" table="commodity_drafts"
	_ int

	// The drafts list reads the open drafts of a group, newest first.
	//ptah:schema:index name="idx_commodity_drafts_group_status_updated" fields="group_id,status,updated_at" table="commodity_drafts"
	_ int
}

// IsOpen reports whether the draft can still be saved, discarded or promoted.
func (d *CommodityDraft) IsOpen() bool {
	return d.Status == CommodityDraftOpen
}

// IsDiscarding reports whether the draft's discard has begun.
func (d *CommodityDraft) IsDiscarding() bool {
	return d.Status == CommodityDraftDiscarding
}

func (*CommodityDraft) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext checks a stored draft.
func (d *CommodityDraft) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.TenantGroupAwareEntityID),
		validation.Field(&d.Status, validation.Required, validation.In(CommodityDraftOpen, CommodityDraftPromoted, CommodityDraftDiscarding)),
		validation.Field(&d.Revision, validation.Required, validation.Min(1)),
	); err != nil {
		return err
	}
	return d.ValidateFields(ctx)
}

// ValidateFields checks the client-supplied fields only. The service calls it
// before a write, when the registry has not filled in the tenant, group and
// author yet.
func (d *CommodityDraft) ValidateFields(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.ClientKey, rules.NotEmpty, validation.RuneLength(1, CommodityDraftClientKeyMaxLength), clientKeyRule),
		validation.Field(&d.Values, CommodityDraftValuesRule),
	)
}

// ValidateCommodityDraftValues checks values the way ValidateFields does,
// reporting the errors under "values" as ValidateFields does.
func ValidateCommodityDraftValues(values CommodityDraftValues) error {
	if err := validation.Validate(values, CommodityDraftValuesRule); err != nil {
		return validation.Errors{"values": err}
	}
	return nil
}
