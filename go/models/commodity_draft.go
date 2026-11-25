package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
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
	// with the values cleared, so a repeated promotion returns the same item.
	CommodityDraftPromoted CommodityDraftStatus = "promoted"
)

// Limits on a draft's client-supplied values.
const (
	// CommodityDraftClientKeyMaxLength caps the idempotency key.
	CommodityDraftClientKeyMaxLength = 128
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

// commodityDraftValuesRule checks the size and the number of fields of the
// values.
var commodityDraftValuesRule = validation.By(func(value any) error {
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
	return nil
})

// clientKeyRule accepts printable ASCII without spaces, so a key can be a
// UUID or a composite such as "intake:<session>:<candidate>".
var clientKeyRule = validation.By(func(value any) error {
	key, _ := value.(string)
	for _, r := range key {
		if r <= ' ' || r > '~' {
			return validation.NewError("validation_draft_client_key", "the client key must be printable ASCII without spaces")
		}
	}
	return nil
})

// CommodityDraft is an unfinished item saved on the server (#2812). It holds
// the form values as entered, which need not form a valid item, and the files
// chosen for it, attached through file_links with entity type "draft".
// Promotion turns it into exactly one item and moves those links to the item.
//
// A draft belongs to its group: every member can see it, members who can edit
// content can save, discard and promote it. Drafts are not items, so no item
// list, count, search, report, reminder or export sees them.
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

	// Status is open until the draft is promoted.
	//ptah:schema:field name="status" type="TEXT" not_null="true" default="open"
	Status CommodityDraftStatus `json:"status" db:"status" userinput:"false"`

	// PromotedCommodityID is the item the draft became. The item can be
	// deleted later, so there is no foreign key.
	//ptah:schema:field name="promoted_commodity_id" type="TEXT"
	PromotedCommodityID *string `json:"promoted_commodity_id,omitempty" db:"promoted_commodity_id" userinput:"false"`

	//ptah:schema:field name="promoted_at" type="TIMESTAMP"
	PromotedAt *time.Time `json:"promoted_at,omitempty" db:"promoted_at" userinput:"false"`

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

func (*CommodityDraft) Validate() error {
	return ErrMustUseValidateWithContext
}

// ValidateWithContext checks a stored draft.
func (d *CommodityDraft) ValidateWithContext(ctx context.Context) error {
	if err := validation.ValidateStructWithContext(ctx, d,
		validation.Field(&d.TenantGroupAwareEntityID),
		validation.Field(&d.Status, validation.Required, validation.In(CommodityDraftOpen, CommodityDraftPromoted)),
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
		validation.Field(&d.ClientKey, rules.NotEmpty, validation.Length(1, CommodityDraftClientKeyMaxLength), clientKeyRule),
		validation.Field(&d.Values, commodityDraftValuesRule),
	)
}

// ValidateCommodityDraftValues checks values the way ValidateFields does.
func ValidateCommodityDraftValues(values CommodityDraftValues) error {
	return validation.Validate(values, commodityDraftValuesRule)
}
