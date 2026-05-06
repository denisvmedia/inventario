package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// WarrantyReminderThreshold names a moment-before-expiry at which the
// warranty reminder worker emits an email. The numeric value is the
// number of days before the warranty's expiry date at which the email
// fires. Values are part of the public surface (worker logs, Prometheus
// label, idempotency row) — add new variants conservatively.
type WarrantyReminderThreshold int

const (
	// WarrantyReminder60Days fires when the warranty has exactly 60 days
	// remaining (matches the WarrantyExpiringWindowDays edge so the email
	// goes out the same day the FE first surfaces the item under
	// "Expiring soon").
	WarrantyReminder60Days WarrantyReminderThreshold = 60
	// WarrantyReminder30Days fires at 30 days remaining.
	WarrantyReminder30Days WarrantyReminderThreshold = 30
	// WarrantyReminder7Days fires at 7 days remaining — last call.
	WarrantyReminder7Days WarrantyReminderThreshold = 7
)

// WarrantyReminderThresholds is the canonical, ordered (largest →
// smallest) list of thresholds the worker scans for. Order is preserved
// so the backfill query can OR the windows in a deterministic shape.
var WarrantyReminderThresholds = []WarrantyReminderThreshold{
	WarrantyReminder60Days,
	WarrantyReminder30Days,
	WarrantyReminder7Days,
}

// IsValid reports whether t is one of the canonical thresholds. Used as
// a guard before persisting a reminder row and when decoding the
// threshold value back from the database — anything else is a bug
// upstream.
func (t WarrantyReminderThreshold) IsValid() bool {
	switch t {
	case WarrantyReminder60Days, WarrantyReminder30Days, WarrantyReminder7Days:
		return true
	}
	return false
}

var (
	_ validation.Validatable            = (*WarrantyReminder)(nil)
	_ validation.ValidatableWithContext = (*WarrantyReminder)(nil)
	_ TenantGroupAwareIDable            = (*WarrantyReminder)(nil)
)

// Enable RLS for multi-tenant isolation on warranty_reminders. Same
// shape as commodities — group-scoped + a separate background-worker
// bypass policy so the periodic scan can read across all groups.
//
//migrator:schema:rls:enable table="warranty_reminders" comment="Enable RLS for multi-tenant warranty reminder isolation"
//migrator:schema:rls:policy name="warranty_reminder_isolation" table="warranty_reminders" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures warranty reminders are accessible only by their tenant and group"
//migrator:schema:rls:policy name="warranty_reminder_background_worker_access" table="warranty_reminders" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to record reminder emissions across all groups"

// WarrantyReminder is the idempotency row written by the warranty
// reminder worker right after it successfully enqueues an email. The
// (commodity_id, threshold_days) tuple is unique — re-running the worker
// for the same commodity in the same window must not produce a second
// email.
//
// Rows are never deleted by user action; they are wiped indirectly when
// the parent commodity is deleted (ON DELETE CASCADE) or when the
// commodity's group is purged (handled by GroupPurger like every other
// group-scoped table).
//
//migrator:schema:table name="warranty_reminders"
type WarrantyReminder struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// CommodityID is the warranty's owning commodity. Cascade-deletes
	// with the commodity row so old reminders never block re-creating an
	// item with the same name.
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_warranty_reminder_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	// ThresholdDays is the WarrantyReminderThreshold this row accounts
	// for (60, 30, 7). Stored as INTEGER rather than text so a future
	// "X days" cadence change can compare numerically.
	//migrator:schema:field name="threshold_days" type="INTEGER" not_null="true"
	ThresholdDays int `json:"threshold_days" db:"threshold_days"`

	// SentAt is the wall-clock time the email was enqueued (not
	// necessarily delivered — the email queue is async). Surfacing this
	// directly is enough for support/audit purposes; the email queue's
	// own job log records final delivery.
	//migrator:schema:field name="sent_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	SentAt time.Time `json:"sent_at" db:"sent_at"`
}

// WarrantyReminderIndexes defines indexes for warranty_reminders. The
// unique (commodity_id, threshold_days) composite is the idempotency
// key the worker reads + writes.
type WarrantyReminderIndexes struct {
	// Unique idempotency key — at most one reminder row per (commodity,
	// threshold). The worker checks this before enqueueing email.
	//migrator:schema:index name="idx_warranty_reminders_commodity_threshold" fields="commodity_id,threshold_days" unique="true" table="warranty_reminders"
	_ int

	// Tenant-scoped index for housekeeping queries.
	//migrator:schema:index name="idx_warranty_reminders_tenant_id" fields="tenant_id" table="warranty_reminders"
	_ int

	// Group-scoped index — used when the group purge worker fans
	// reminders out for hard-delete in FK order.
	//migrator:schema:index name="idx_warranty_reminders_group_id" fields="group_id" table="warranty_reminders"
	_ int
}

func (*WarrantyReminder) Validate() error {
	return ErrMustUseValidateWithContext
}

func (w *WarrantyReminder) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, w,
		validation.Field(&w.TenantGroupAwareEntityID),
		validation.Field(&w.CommodityID, rules.NotEmpty),
		validation.Field(&w.ThresholdDays, validation.Required, validation.By(func(any) error {
			if !WarrantyReminderThreshold(w.ThresholdDays).IsValid() {
				return validation.NewError("invalid_threshold", "invalid warranty reminder threshold")
			}
			return nil
		})),
	)
}
