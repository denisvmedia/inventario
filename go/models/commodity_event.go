package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// CommodityEventKind names a single state-change event recorded against a
// commodity. The set is closed and validated at app level (no DB CHECK
// constraint — same convention as TagColor / FileCategory).
type CommodityEventKind string

const (
	// CommodityEventKindCreated is emitted on initial commodity creation.
	// Before is null; After holds the initial fields.
	CommodityEventKindCreated CommodityEventKind = "created"
	// CommodityEventKindUpdated is emitted on edits that don't fall into one
	// of the more specific kinds below. Carries the diff of changed fields.
	CommodityEventKindUpdated CommodityEventKind = "updated"
	// CommodityEventKindStatusChanged is emitted when status flips (e.g. in_use → sold).
	CommodityEventKindStatusChanged CommodityEventKind = "status_changed"
	// CommodityEventKindMoved is emitted when area_id changes.
	CommodityEventKindMoved CommodityEventKind = "moved"
	// CommodityEventKindPriceChanged is emitted when current_price or
	// original_price (or its currency) changes.
	CommodityEventKindPriceChanged CommodityEventKind = "price_changed"
	// CommodityEventKindCoverChanged is emitted when the cover_file_id override
	// is set or cleared.
	CommodityEventKindCoverChanged CommodityEventKind = "cover_changed"
	// CommodityEventKindLentOut is emitted when a commodity is lent out
	// (a new commodity_loans row is created with returned_at NULL). After
	// holds the borrower-facing fields; before is null. See #1507.
	CommodityEventKindLentOut CommodityEventKind = "lent_out"
	// CommodityEventKindReturned is emitted when an open loan closes
	// (returned_at flips from null to a date). Carries the loan id and
	// the returned_at date for kind-aware FE copy.
	CommodityEventKindReturned CommodityEventKind = "returned"
	// CommodityEventKindLoanUpdated is emitted when a loan's mutable
	// fields change (borrower_contact / borrower_note / due_back_at). The
	// service skips no-op patches so this event only lands when something
	// actually changed — same gate as EmitUpdated for commodities.
	CommodityEventKindLoanUpdated CommodityEventKind = "loan_updated"
	// CommodityEventKindDeleted is emitted right before a commodity is deleted.
	// Persisted so the event row is still in the table when the commodity row
	// is removed in the same transaction; ON DELETE CASCADE then drops it. The
	// row is still observable to anyone scanning during the same request.
	CommodityEventKindDeleted CommodityEventKind = "deleted"
)

// IsValid reports whether the event kind is one of the known values.
func (k CommodityEventKind) IsValid() bool {
	switch k {
	case CommodityEventKindCreated,
		CommodityEventKindUpdated,
		CommodityEventKindStatusChanged,
		CommodityEventKindMoved,
		CommodityEventKindPriceChanged,
		CommodityEventKindCoverChanged,
		CommodityEventKindLentOut,
		CommodityEventKindReturned,
		CommodityEventKindLoanUpdated,
		CommodityEventKindDeleted:
		return true
	}
	return false
}

// Validate makes CommodityEventKind a validation.Validatable.
func (k CommodityEventKind) Validate() error {
	if !k.IsValid() {
		return validation.NewError("invalid_event_kind", "invalid commodity event kind")
	}
	return nil
}

// CommodityEventPayload is the sparse before/after snapshot stored as JSONB.
// The map holds only the fields that changed for the event so the log
// doesn't bloat with full row snapshots.
type CommodityEventPayload map[string]any

// Value implements driver.Valuer so the payload can be written to a JSONB
// column. nil and empty maps both round-trip as SQL NULL — the timeline UI
// treats both the same.
func (p CommodityEventPayload) Value() (driver.Value, error) {
	if len(p) == 0 {
		return nil, nil
	}
	return json.Marshal(p)
}

// Scan implements sql.Scanner so SELECT queries can hydrate the JSONB
// column back into the map. NULL / empty bytes resolve to a nil map.
func (p *CommodityEventPayload) Scan(value any) error {
	if value == nil {
		*p = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		if len(v) == 0 {
			*p = nil
			return nil
		}
		return json.Unmarshal(v, p)
	case string:
		if v == "" {
			*p = nil
			return nil
		}
		return json.Unmarshal([]byte(v), p)
	default:
		return fmt.Errorf("cannot scan %T into CommodityEventPayload", value)
	}
}

var (
	_ validation.Validatable            = (*CommodityEvent)(nil)
	_ validation.ValidatableWithContext = (*CommodityEvent)(nil)
	_ TenantGroupAwareIDable            = (*CommodityEvent)(nil)
)

// CommodityEvent is an append-only audit row recording a state change
// against a single commodity (#1450). The actor is the
// created_by_user_id field on TenantGroupAwareEntityID — same column the
// rest of the data tables use to track who performed the write.
//
// Enable RLS for multi-tenant isolation
//
//migrator:schema:rls:enable table="commodity_events" comment="Enable RLS for multi-tenant commodity event isolation"
//migrator:schema:rls:policy name="commodity_event_isolation" table="commodity_events" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures commodity events can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="commodity_event_background_worker_access" table="commodity_events" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all commodity events for processing"
//migrator:schema:table name="commodity_events"
type CommodityEvent struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// CommodityID is the parent commodity. ON DELETE CASCADE: when the
	// commodity is hard-deleted, the entire timeline goes with it (the
	// detail page won't exist anymore either). The "deleted" kind is
	// persisted in the same transaction so the row is observable for the
	// rest of the request before the cascade fires.
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_commodity_event_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	// Kind is one of CommodityEventKind*. Validated at app level — the FE
	// renders kind-aware copy and unknown kinds fall through to a generic
	// "updated" line.
	//migrator:schema:field name="kind" type="TEXT" not_null="true"
	Kind CommodityEventKind `json:"kind" db:"kind"`

	// OccurredAt is when the event was recorded; defaults to now() on
	// insert. Distinct from EntityID's created_at (we don't have one here)
	// because the issue uses occurred_at as the canonical timestamp.
	//migrator:schema:field name="occurred_at" type="TIMESTAMPTZ" not_null="true" default_expr="now()"
	OccurredAt time.Time `json:"occurred_at" db:"occurred_at"`

	// Before is the sparse JSONB snapshot of changed fields BEFORE the
	// event. Null on `created`. Stored as map[string]any so different
	// kinds can use different field sets without a schema bump.
	//migrator:schema:field name="before" type="JSONB"
	Before CommodityEventPayload `json:"before,omitempty" db:"before"`

	// After is the sparse JSONB snapshot of changed fields AFTER the
	// event. Null on `deleted`.
	//migrator:schema:field name="after" type="JSONB"
	After CommodityEventPayload `json:"after,omitempty" db:"after"`

	// Note is a free-form, optional operator-supplied reason. Reserved for
	// a future "leave a comment" UX; today every event lands with an
	// empty note.
	//migrator:schema:field name="note" type="TEXT"
	Note string `json:"note,omitempty" db:"note"`
}

// CommodityEventIndexes defines indexes for commodity_events.
type CommodityEventIndexes struct {
	// Unique index on uuid for restore/import dedup parity with siblings.
	//migrator:schema:index name="idx_commodity_events_uuid" fields="uuid" unique="true" table="commodity_events"
	_ int

	// Per-commodity timeline lookup, descending by occurrence — backs the
	// detail page's `GET /commodities/{id}/events?per_page=N` (and the
	// composite group_id+commodity_id+occurred_at order avoids a sort step).
	//migrator:schema:index name="commodity_events_lookup" fields="group_id,commodity_id,occurred_at" table="commodity_events"
	_ int

	// Tenant-scope index for cross-group analytics — same shape as siblings.
	//migrator:schema:index name="idx_commodity_events_tenant_id" fields="tenant_id" table="commodity_events"
	_ int

	// Composite tenant + group index for RLS-filtered queries.
	//migrator:schema:index name="idx_commodity_events_tenant_group" fields="tenant_id,group_id" table="commodity_events"
	_ int

	// Kind filter — narrow per-commodity queries by event kind without a
	// table scan when the timeline grows large.
	//migrator:schema:index name="commodity_events_kind_idx" fields="commodity_id,kind" table="commodity_events"
	_ int
}

func (*CommodityEvent) Validate() error {
	return ErrMustUseValidateWithContext
}

func (e *CommodityEvent) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, e,
		validation.Field(&e.CommodityID, rules.NotEmpty),
		validation.Field(&e.Kind, validation.Required),
	)
}
