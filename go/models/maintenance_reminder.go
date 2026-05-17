package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// MaintenanceReminderThreshold names a moment-before-due (or overdue)
// at which the maintenance reminder worker emits an email. Positive
// values are days remaining until next_due_at; the dedicated
// MaintenanceReminderOverdue sentinel fires once after the due date
// has passed. Values are part of the public surface (worker logs,
// Prometheus label, idempotency row) — add new variants
// conservatively.
type MaintenanceReminderThreshold int

const (
	// MaintenanceReminder14Days fires when next_due_at is exactly
	// 14 days out.
	MaintenanceReminder14Days MaintenanceReminderThreshold = 14
	// MaintenanceReminder7Days fires at 7 days remaining.
	MaintenanceReminder7Days MaintenanceReminderThreshold = 7
	// MaintenanceReminder1Day fires at 1 day remaining — last call.
	MaintenanceReminder1Day MaintenanceReminderThreshold = 1
	// MaintenanceReminderOverdue fires once after the due date has
	// passed. Stored in the idempotency table as the integer 0; the
	// worker stops emitting once the row exists and only re-fires
	// after a MarkDone advances next_due_at into the future (which
	// clears the per-schedule reminder rows — see
	// MaintenanceReminderRegistry.DeleteBySchedule).
	MaintenanceReminderOverdue MaintenanceReminderThreshold = 0
)

// MaintenanceReminderThresholds is the canonical, ordered (largest →
// overdue) list of thresholds the worker scans for. Order is preserved
// so the backfill query can OR the windows in a deterministic shape.
var MaintenanceReminderThresholds = []MaintenanceReminderThreshold{
	MaintenanceReminder14Days,
	MaintenanceReminder7Days,
	MaintenanceReminder1Day,
	MaintenanceReminderOverdue,
}

// IsValid reports whether t is one of the canonical thresholds. Used
// as a guard before persisting a reminder row and when decoding the
// threshold value back from the database.
func (t MaintenanceReminderThreshold) IsValid() bool {
	switch t {
	case MaintenanceReminder14Days, MaintenanceReminder7Days,
		MaintenanceReminder1Day, MaintenanceReminderOverdue:
		return true
	}
	return false
}

// Label returns a short human-readable label for logs and email copy.
func (t MaintenanceReminderThreshold) Label() string {
	switch t {
	case MaintenanceReminder14Days:
		return "14-day"
	case MaintenanceReminder7Days:
		return "7-day"
	case MaintenanceReminder1Day:
		return "1-day"
	case MaintenanceReminderOverdue:
		return "overdue"
	}
	return ""
}

var (
	_ validation.Validatable            = (*MaintenanceReminder)(nil)
	_ validation.ValidatableWithContext = (*MaintenanceReminder)(nil)
	_ TenantGroupAwareIDable            = (*MaintenanceReminder)(nil)
)

// Enable RLS for multi-tenant isolation on maintenance_reminders.
// Same shape as warranty_reminders — group-scoped + a separate
// background-worker bypass policy so the periodic scan can read
// across all groups.
//
//migrator:schema:rls:enable table="maintenance_reminders" comment="Enable RLS for multi-tenant maintenance reminder isolation"
//migrator:schema:rls:policy name="maintenance_reminder_isolation" table="maintenance_reminders" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures maintenance reminders are accessible only by their tenant and group"
//migrator:schema:rls:policy name="maintenance_reminder_background_worker_access" table="maintenance_reminders" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to record reminder emissions across all groups"

// MaintenanceReminder is the idempotency row written by the
// maintenance reminder worker right after it successfully enqueues an
// email. The (schedule_id, threshold_days) tuple is unique —
// re-running the worker for the same schedule/threshold within the
// same cycle must not produce a second email.
//
// Reset semantics: when the user marks a schedule done the service
// calls DeleteBySchedule so the next cycle starts with a clean slate.
// On commodity hard-delete the rows cascade away with the schedule
// (and the schedule cascades with the commodity).
//
//migrator:schema:table name="maintenance_reminders"
type MaintenanceReminder struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// ScheduleID is the owning maintenance_schedules row. Cascade-
	// deletes with the schedule so old reminders never block re-
	// creating a similar schedule for the same commodity.
	//migrator:schema:field name="schedule_id" type="TEXT" not_null="true" foreign="maintenance_schedules(id)" foreign_key_name="fk_maintenance_reminder_schedule" on_delete="CASCADE"
	ScheduleID string `json:"schedule_id" db:"schedule_id"`

	// ThresholdDays is the MaintenanceReminderThreshold this row
	// accounts for (14 / 7 / 1 / 0). Stored as INTEGER rather than
	// text so a future cadence change can compare numerically. The
	// overdue sentinel is stored as 0.
	//migrator:schema:field name="threshold_days" type="INTEGER" not_null="true"
	ThresholdDays int `json:"threshold_days" db:"threshold_days"`

	// SentAt is the wall-clock time the email was enqueued (not
	// necessarily delivered — the email queue is async). Surfacing
	// this directly is enough for support/audit purposes; the email
	// queue's own job log records final delivery.
	//migrator:schema:field name="sent_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	SentAt time.Time `json:"sent_at" db:"sent_at"`
}

// MaintenanceReminderIndexes defines indexes for maintenance_reminders.
// The unique (schedule_id, threshold_days) composite is the
// idempotency key the worker reads + writes.
type MaintenanceReminderIndexes struct {
	// Unique idempotency key — at most one reminder row per
	// (schedule, threshold). The worker checks this before enqueueing
	// the email.
	//migrator:schema:index name="idx_maintenance_reminders_schedule_threshold" fields="schedule_id,threshold_days" unique="true" table="maintenance_reminders"
	_ int

	// Tenant-scoped index for housekeeping queries.
	//migrator:schema:index name="idx_maintenance_reminders_tenant_id" fields="tenant_id" table="maintenance_reminders"
	_ int

	// Group-scoped index — used when the group purge worker fans
	// reminders out for hard-delete in FK order.
	//migrator:schema:index name="idx_maintenance_reminders_group_id" fields="group_id" table="maintenance_reminders"
	_ int
}

func (*MaintenanceReminder) Validate() error {
	return ErrMustUseValidateWithContext
}

func (m *MaintenanceReminder) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, m,
		validation.Field(&m.TenantGroupAwareEntityID),
		validation.Field(&m.ScheduleID, rules.NotEmpty),
		validation.Field(&m.ThresholdDays, validation.By(func(any) error {
			if !MaintenanceReminderThreshold(m.ThresholdDays).IsValid() {
				return validation.NewError("invalid_threshold", "invalid maintenance reminder threshold")
			}
			return nil
		})),
	)
}
