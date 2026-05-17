package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*MaintenanceSchedule)(nil)
	_ validation.ValidatableWithContext = (*MaintenanceSchedule)(nil)
	_ TenantGroupAwareIDable            = (*MaintenanceSchedule)(nil)
)

// MaintenanceSchedule is a per-commodity recurring care reminder
// (#1368). The user creates a row like "Replace water filter every 180
// days" and the reminder worker fires emails at 14 / 7 / 1 days before
// the row's next_due_at, plus a final "now overdue" reminder once it
// flips into the past.
//
// The mental model intentionally mirrors warranty tracking
// (one-shot, "when does coverage end") — see #1367 — but on a recurring
// cadence. v1 stores a single fixed interval in days; cron-like
// recurrence is deliberately out of scope (issue body §Options 1).
//
// Marking a schedule done advances next_due_at by IntervalDays from the
// supplied done date (server clock by default). The same idempotency
// pattern as warranty reminders is reused via the maintenance_reminders
// table: at most one row per (schedule_id, threshold_days) tuple.
//
// Enable RLS for multi-tenant isolation.
//
//migrator:schema:rls:enable table="maintenance_schedules" comment="Enable RLS for multi-tenant maintenance schedule isolation"
//migrator:schema:rls:policy name="maintenance_schedule_isolation" table="maintenance_schedules" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures maintenance schedules can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="maintenance_schedule_background_worker_access" table="maintenance_schedules" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all maintenance schedules for processing"
//migrator:schema:table name="maintenance_schedules"
type MaintenanceSchedule struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// CommodityID is the schedule's owning commodity. ON DELETE CASCADE
	// is added manually to the generated migration: hard-deleting a
	// commodity drops its maintenance history (no orphan rows). Mirrors
	// commodity_loans / commodity_services.
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_maintenance_schedule_commodity" on_delete="CASCADE"
	CommodityID string `json:"commodity_id" db:"commodity_id"`

	// Title is required and free-form ("Replace water filter",
	// "Descale espresso machine"). Capped at 200 chars to match the
	// soft cap used by other text fields and leave room for indexes.
	//migrator:schema:field name="title" type="TEXT" not_null="true"
	Title string `json:"title" db:"title"`

	// IntervalDays is the fixed cadence in days. v1 keeps this as a
	// plain integer — cron-like recurrence is deliberately out of
	// scope (#1368 options §1). Validated to be strictly positive: a
	// non-positive interval would either spam reminders (0) or make
	// next_due_at recede (negative).
	//migrator:schema:field name="interval_days" type="INTEGER" not_null="true"
	IntervalDays int `json:"interval_days" db:"interval_days"`

	// NextDueAt is the date the next instance is due. Stored as TEXT
	// in YYYY-MM-DD format to match the codebase's other date fields
	// (lent_at, sent_at, warranty_expires_at). Recomputed on every
	// MarkDone call as `done_date + interval_days`.
	//migrator:schema:field name="next_due_at" type="TEXT" not_null="true"
	NextDueAt Date `json:"next_due_at" db:"next_due_at"`

	// LastDoneAt is the most recent date the user marked the schedule
	// as done. Nullable for freshly-created rows the user has not yet
	// performed once — the FE renders "—" for those. The done date may
	// be in the past (the user logging a maintenance they performed
	// earlier and forgot to tick off) and may differ from the previous
	// next_due_at by an arbitrary delta (life happens).
	//migrator:schema:field name="last_done_at" type="TEXT"
	LastDoneAt PDate `json:"last_done_at" db:"last_done_at"`

	// Notes is a free-form aide-mémoire ("use NSF-53 filter, comes in
	// 2-packs"). Capped at 1000 chars — same convention as the loan /
	// service note fields.
	//migrator:schema:field name="notes" type="TEXT"
	Notes string `json:"notes" db:"notes"`

	// Enabled gates the reminder worker. Disabled rows are still
	// surfaced on the FE (with an "off" pill) so the user can pause a
	// schedule without losing the configuration, but the worker skips
	// them at scan time — no reminder rows are written and no email
	// fires.
	//migrator:schema:field name="enabled" type="BOOLEAN" not_null="true" default="true"
	Enabled bool `json:"enabled" db:"enabled"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// MaintenanceScheduleIndexes defines the postgres indexes for maintenance_schedules.
type MaintenanceScheduleIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore).
	//migrator:schema:index name="idx_maintenance_schedules_uuid" fields="uuid" unique="true" table="maintenance_schedules"
	_ int

	// Index for tenant-based queries.
	//migrator:schema:index name="idx_maintenance_schedules_tenant_id" fields="tenant_id" table="maintenance_schedules"
	_ int

	// Composite index for tenant+group RLS-filtered queries.
	//migrator:schema:index name="idx_maintenance_schedules_tenant_group" fields="tenant_id,group_id" table="maintenance_schedules"
	_ int

	// Composite index for per-commodity reads (the per-item Maintenance
	// section orders by next_due_at).
	//migrator:schema:index name="idx_maintenance_schedules_commodity" fields="commodity_id,next_due_at" table="maintenance_schedules"
	_ int

	// Composite index for the group-wide upcoming list — the FE sorts
	// by next_due_at ASC across the whole group.
	//migrator:schema:index name="idx_maintenance_schedules_group_due" fields="group_id,next_due_at" table="maintenance_schedules"
	_ int

	// Partial index for the reminder worker's scan — only enabled rows
	// are eligible to fire a reminder. The unenabled rows still match
	// the index above but the worker filters them out; this index
	// keeps the scan cheap.
	//migrator:schema:index name="idx_maintenance_schedules_enabled_due" fields="next_due_at" condition="enabled = true" table="maintenance_schedules"
	_ int
}

// IsDueWithin reports whether the schedule's NextDueAt falls within
// [today, today + days] inclusive at the supplied `now`. Used by the
// reminder worker to decide threshold matches.
func (m *MaintenanceSchedule) IsDueWithin(now time.Time, days int) bool {
	if m.NextDueAt == "" || days < 0 {
		return false
	}
	due := m.NextDueAt.ToTime()
	if due.IsZero() {
		return false
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	if due.Before(today) {
		return false
	}
	daysUntil := int(due.Sub(today).Hours() / 24)
	return daysUntil <= days
}

// IsOverdue reports whether the schedule's NextDueAt is strictly before
// today at the supplied `now`. Overdue rows fire the
// MaintenanceReminderOverdue threshold once and stop until the user
// marks them done.
func (m *MaintenanceSchedule) IsOverdue(now time.Time) bool {
	if m.NextDueAt == "" {
		return false
	}
	due := m.NextDueAt.ToTime()
	if due.IsZero() {
		return false
	}
	n := now.UTC()
	today := time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, time.UTC)
	return due.Before(today)
}

// AdvanceFromDone returns the new NextDueAt computed from a done date
// plus the row's IntervalDays. Pure function on the row — the caller
// is responsible for writing the result back via the registry.
func (m *MaintenanceSchedule) AdvanceFromDone(doneDate time.Time) Date {
	d := doneDate.UTC()
	day := time.Date(d.Year(), d.Month(), d.Day(), 0, 0, 0, 0, time.UTC)
	next := day.AddDate(0, 0, m.IntervalDays)
	return Date(next.Format("2006-01-02"))
}

func (*MaintenanceSchedule) Validate() error {
	return ErrMustUseValidateWithContext
}

func (m *MaintenanceSchedule) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, m,
		validation.Field(&m.CommodityID, rules.NotEmpty),
		validation.Field(&m.Title, rules.NotEmpty, validation.Length(1, 200)),
		validation.Field(&m.IntervalDays, validation.Required, validation.Min(1), validation.Max(36500)),
		validation.Field(&m.NextDueAt, validation.Required),
		validation.Field(&m.LastDoneAt),
		validation.Field(&m.Notes, validation.Length(0, 1000)),
	)
}
