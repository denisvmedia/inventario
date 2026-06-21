package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"
)

// StorageQuotaThreshold names a quota-usage tier at which the storage
// quota reminder worker emits an email. The numeric value is the
// percentage of `used_bytes / quota_bytes` at which the email fires.
// Values are part of the public surface (worker logs, Prometheus
// label, idempotency row) — add new variants conservatively.
type StorageQuotaThreshold int

const (
	// StorageQuota90Percent fires when a group's storage usage reaches
	// 90% of its quota — the threshold called out in #1585's
	// acceptance criteria. A passive heads-up so the user can free up
	// space before uploads start failing once plans-aware enforcement
	// (#1389) lands.
	StorageQuota90Percent StorageQuotaThreshold = 90
)

// StorageQuotaThresholds is the canonical, ordered (smallest →
// largest) list of thresholds the worker scans for. The smallest
// threshold appears first so the worker can short-circuit a group
// whose usage is below every tier.
var StorageQuotaThresholds = []StorageQuotaThreshold{
	StorageQuota90Percent,
}

// IsValid reports whether t is one of the canonical thresholds. Used
// as a guard before persisting a reminder row and when decoding the
// threshold value back from the database — anything else is a bug
// upstream.
func (t StorageQuotaThreshold) IsValid() bool {
	return t == StorageQuota90Percent
}

// Ratio returns the fractional usage at which this threshold fires
// (e.g. 0.9 for the 90% tier). Convenience for callers that compare
// against `used_bytes / quota_bytes`.
func (t StorageQuotaThreshold) Ratio() float64 {
	return float64(t) / 100.0
}

var (
	_ validation.Validatable            = (*StorageQuotaReminder)(nil)
	_ validation.ValidatableWithContext = (*StorageQuotaReminder)(nil)
	_ TenantGroupAwareIDable            = (*StorageQuotaReminder)(nil)
)

// Enable RLS for multi-tenant isolation on storage_quota_reminders.
// Same shape as warranty_reminders — group-scoped + a separate
// background-worker bypass policy so the periodic scan can read and
// write across all groups.
//
//migrator:schema:rls:enable table="storage_quota_reminders" comment="Enable RLS for multi-tenant storage quota reminder isolation"
//migrator:schema:rls:policy name="storage_quota_reminder_isolation" table="storage_quota_reminders" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures storage quota reminders are accessible only by their tenant and group"
//migrator:schema:rls:policy name="storage_quota_reminder_background_worker_access" table="storage_quota_reminders" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to record reminder emissions across all groups"

// StorageQuotaReminder is the idempotency row written by the storage
// quota reminder worker right after it successfully enqueues an
// email. The (group_id, threshold_percent) tuple is unique —
// re-running the worker for the same group at the same threshold
// must not produce a second email.
//
// Reset semantics: when the worker observes a group whose usage has
// fallen back below the threshold it deletes the matching row, so
// the next time the threshold is re-crossed a fresh email fires.
//
// Rows are wiped indirectly when the parent group is hard-deleted
// (ON DELETE CASCADE) so old reminders never block re-creating a
// group with the same id.
//
//migrator:schema:table name="storage_quota_reminders"
type StorageQuotaReminder struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID

	// ThresholdPercent is the StorageQuotaThreshold this row accounts
	// for (90 in v1). Stored as INTEGER rather than text so future
	// 80 / 95 tiers can compare numerically.
	//migrator:schema:field name="threshold_percent" type="INTEGER" not_null="true"
	ThresholdPercent int `json:"threshold_percent" db:"threshold_percent"`

	// SentAt is the wall-clock time the email was enqueued (not
	// necessarily delivered — the email queue is async). Surfacing
	// this directly is enough for support/audit purposes; the email
	// queue's own job log records final delivery.
	//migrator:schema:field name="sent_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	SentAt time.Time `json:"sent_at" db:"sent_at"`
}

// StorageQuotaReminderIndexes defines indexes for
// storage_quota_reminders. The unique (group_id, threshold_percent)
// composite is the idempotency key the worker reads + writes.
type StorageQuotaReminderIndexes struct {
	// Unique idempotency key — at most one reminder row per (group,
	// threshold). The worker checks this before enqueueing email and
	// deletes the matching row when usage drops back below the
	// threshold so future re-crossings fire again.
	//migrator:schema:index name="idx_storage_quota_reminders_group_threshold" fields="group_id,threshold_percent" unique="true" table="storage_quota_reminders"
	_ int

	// Tenant-scoped index for housekeeping queries.
	//migrator:schema:index name="idx_storage_quota_reminders_tenant_id" fields="tenant_id" table="storage_quota_reminders"
	_ int
}

func (*StorageQuotaReminder) Validate() error {
	return ErrMustUseValidateWithContext
}

func (s *StorageQuotaReminder) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, s,
		validation.Field(&s.TenantGroupAwareEntityID),
		validation.Field(&s.ThresholdPercent, validation.Required, validation.By(func(any) error {
			if !StorageQuotaThreshold(s.ThresholdPercent).IsValid() {
				return validation.NewError("invalid_storage_quota_threshold", "invalid storage quota reminder threshold")
			}
			return nil
		})),
	)
}
