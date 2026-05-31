package models

import "time"

// WorkerType is the typed natural key identifying a background worker
// whose run loop can be soft-paused (#1308). The string values are the
// stable, operator-facing identifiers used in the CLI (`inventario
// workers pause <type>`), the admin REST surface, and the
// worker_control.worker_type column — keep them stable across releases
// and add new variants conservatively.
//
// `email` is intentionally NOT in this set: email delivery is a Redis
// subscriber rather than a polling worker, and its pause/resume story is
// handled separately.
type WorkerType string

const (
	// WorkerTypeExport pauses the export generation worker.
	WorkerTypeExport WorkerType = "export"
	// WorkerTypeImport pauses the import processing worker.
	WorkerTypeImport WorkerType = "import"
	// WorkerTypeRestore pauses the restore processing worker.
	WorkerTypeRestore WorkerType = "restore"
	// WorkerTypeThumbnail pauses the thumbnail generation worker.
	WorkerTypeThumbnail WorkerType = "thumbnail"
	// WorkerTypeRefreshTokenCleanup pauses the refresh-token cleanup worker.
	WorkerTypeRefreshTokenCleanup WorkerType = "refresh-token-cleanup"
	// WorkerTypeEmailVerificationCleanup pauses the email-verification cleanup worker.
	WorkerTypeEmailVerificationCleanup WorkerType = "email-verification-cleanup"
	// WorkerTypeMagicLinkTokenCleanup pauses the magic-link token cleanup worker.
	WorkerTypeMagicLinkTokenCleanup WorkerType = "magic-link-token-cleanup"
	// WorkerTypeLoginEventRetention pauses the login-event retention worker.
	WorkerTypeLoginEventRetention WorkerType = "login-event-retention"
	// WorkerTypeGroupPurge pauses the group purge worker.
	WorkerTypeGroupPurge WorkerType = "group-purge"
	// WorkerTypeWarrantyReminder pauses the warranty reminder worker.
	WorkerTypeWarrantyReminder WorkerType = "warranty-reminder"
	// WorkerTypeStorageQuotaReminder pauses the storage-quota reminder worker.
	WorkerTypeStorageQuotaReminder WorkerType = "storage-quota-reminder"
	// WorkerTypeLoanReminder pauses the loan reminder worker.
	WorkerTypeLoanReminder WorkerType = "loan-reminder"
	// WorkerTypeMaintenanceReminder pauses the maintenance reminder worker.
	WorkerTypeMaintenanceReminder WorkerType = "maintenance-reminder"
	// WorkerTypeCurrencyMigration pauses the currency migration worker.
	WorkerTypeCurrencyMigration WorkerType = "currency-migration"
)

// allWorkerTypes is the canonical ordered set of pausable worker types.
// Ordering is intentional (lifecycle workers first, then periodic
// maintenance jobs) and backs AllWorkerTypes() so the CLI/admin listing
// renders deterministically. Keep in sync with the const block above.
var allWorkerTypes = []WorkerType{
	WorkerTypeExport,
	WorkerTypeImport,
	WorkerTypeRestore,
	WorkerTypeThumbnail,
	WorkerTypeRefreshTokenCleanup,
	WorkerTypeEmailVerificationCleanup,
	WorkerTypeMagicLinkTokenCleanup,
	WorkerTypeLoginEventRetention,
	WorkerTypeGroupPurge,
	WorkerTypeWarrantyReminder,
	WorkerTypeStorageQuotaReminder,
	WorkerTypeLoanReminder,
	WorkerTypeMaintenanceReminder,
	WorkerTypeCurrencyMigration,
}

// AllWorkerTypes returns a copy of the canonical ordered worker-type set.
// Callers (CLI listing, admin endpoint enumeration) get a fresh slice so
// they can sort/filter without mutating the package-level source of truth.
func AllWorkerTypes() []WorkerType {
	out := make([]WorkerType, len(allWorkerTypes))
	copy(out, allWorkerTypes)
	return out
}

// IsValid reports whether w is one of the known pausable worker types.
// The empty string is not valid — callers must pass a concrete type.
func (w WorkerType) IsValid() bool {
	switch w {
	case WorkerTypeExport,
		WorkerTypeImport,
		WorkerTypeRestore,
		WorkerTypeThumbnail,
		WorkerTypeRefreshTokenCleanup,
		WorkerTypeEmailVerificationCleanup,
		WorkerTypeMagicLinkTokenCleanup,
		WorkerTypeLoginEventRetention,
		WorkerTypeGroupPurge,
		WorkerTypeWarrantyReminder,
		WorkerTypeStorageQuotaReminder,
		WorkerTypeLoanReminder,
		WorkerTypeMaintenanceReminder,
		WorkerTypeCurrencyMigration:
		return true
	}
	return false
}

// ParseWorkerType validates s and returns the typed worker type. The
// second return reports whether s named a known worker — callers (CLI
// arg parsing, admin request validation) branch on it to reject unknown
// types with a clear error rather than silently creating a control row
// for a worker that doesn't exist.
func ParseWorkerType(s string) (WorkerType, bool) {
	w := WorkerType(s)
	if !w.IsValid() {
		return "", false
	}
	return w, true
}

// WorkerControl is the global control row for a single background worker
// type (#1308). A present row with paused=true soft-pauses that worker;
// an absent row means the worker runs normally. Soft-pause means the
// worker's run loop keeps ticking but skips its unit of work while
// paused, so resuming is immediate and needs no process restart.
//
// The table is NOT tenant-scoped and has NO RLS policy: worker pause
// state is a platform-operator control orthogonal to tenants (same
// posture as system_admin_grants / audit_logs). It is stored directly on
// FactorySet rather than behind a per-request Factory.
//
//migrator:schema:table name="worker_control"
type WorkerControl struct {
	//migrator:embedded mode="inline"
	EntityID

	// WorkerType is the natural key — one control row per worker type.
	// Backed by a unique index so Pause can use ON CONFLICT (worker_type).
	//migrator:schema:field name="worker_type" type="TEXT" not_null="true"
	WorkerType WorkerType `json:"worker_type" db:"worker_type"`

	// Paused is the soft-pause flag the worker run loop checks each tick.
	//migrator:schema:field name="paused" type="BOOLEAN" not_null="true" default="false"
	Paused bool `json:"paused" db:"paused"`

	// PausedBy records who paused the worker: the back-office operator id
	// for an API pause, or the literal "cli" for a CLI pause (the CLI has
	// no authenticated operator session). NULL only when no actor was
	// recorded. Not an FK — this control plane is intentionally decoupled
	// from the users table (and a CLI/back-office operator may not be a
	// tenant user row).
	//migrator:schema:field name="paused_by" type="TEXT"
	PausedBy *string `json:"paused_by,omitempty" db:"paused_by"`

	// PausedAt is when the worker was first paused. Preserved across
	// re-pauses (updating by/reason does not reset the original pause
	// time), and cleared to NULL on resume.
	//migrator:schema:field name="paused_at" type="TIMESTAMP"
	PausedAt *time.Time `json:"paused_at,omitempty" db:"paused_at"`

	// Reason is the optional operator-supplied note for the pause. NULL
	// when none was given; cleared on resume.
	//migrator:schema:field name="reason" type="TEXT"
	Reason *string `json:"reason,omitempty" db:"reason"`

	// UpdatedAt is the wall-clock time of the last state change (pause,
	// re-pause, or resume).
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// WorkerControlIndexes defines the PostgreSQL indexes for the
// worker_control table.
type WorkerControlIndexes struct {
	// Unique index for the immutable UUID (the dedup key every entity
	// carries for import/restore — mirrors the convention used elsewhere).
	//migrator:schema:index name="idx_worker_control_uuid" fields="uuid" unique="true" table="worker_control"
	_ int

	// Unique index on worker_type: at most one control row per worker.
	// Backs the Pause INSERT ... ON CONFLICT (worker_type) upsert and the
	// hot-path lookup the worker run loop runs each tick.
	//migrator:schema:index name="worker_control_worker_type_idx" fields="worker_type" unique="true" table="worker_control"
	_ int
}
