package models

import (
	"slices"
	"time"
)

// FailedJob is a background job that failed for good, as the failed-jobs
// surface shows it (#1307). It is not a table of its own: each job type's
// adapter reads it from the type's own table (exports, restore_operations,
// thumbnail_generation_jobs, intake_sources, currency_migrations,
// personal_data_exports), so the row, its error and its history stay where
// the job lives.
type FailedJob struct {
	// Type is the worker type that ran the job.
	Type WorkerType `json:"type"`
	// ID is the id of the job's row in its own table.
	ID       string `json:"id"`
	TenantID string `json:"tenant_id"`
	// GroupID is the group the job belongs to. A thumbnail job takes it
	// from its file. Empty for a personal data export, which belongs to a
	// user.
	GroupID string `json:"group_id"`
	// UserID is the member who started the job.
	UserID string `json:"user_id"`
	// Title says which job it is in words: an export's description, a
	// file's title, the currencies of a migration, the email address of
	// the user a personal data export is for.
	Title string `json:"title"`
	// Error is the last error the job recorded.
	Error string `json:"error"`
	// Attempts is how many times the job ran. A type that runs a job once
	// reports 1.
	Attempts  int       `json:"attempts"`
	CreatedAt time.Time `json:"created_at"`
	// FailedAt is when the job failed. It is nil only for a row whose
	// table has no time for it.
	FailedAt *time.Time `json:"failed_at,omitempty"`
}

// FailedJobTypePersonalDataExport is the type of a failed personal data
// export. The export worker builds personal data exports, so it is not a
// worker type of its own: pausing export pauses them too, and `workers
// pause` does not take it. A personal data export belongs to a user, not
// to a group, so only the system-admin plane and the CLI show it.
const FailedJobTypePersonalDataExport WorkerType = "personal-data-export"

// failedJobTypes are the types that keep their failed jobs in a table, in
// the order the failed-jobs surface lists them. The other worker types have
// nothing to dead-letter: the email queue retries in memory or Redis, the
// reminder sweeps write a row only on success and try again on the next
// sweep, and the cleanup sweeps have no job rows.
var failedJobTypes = []WorkerType{
	WorkerTypeExport,
	WorkerTypeImport,
	WorkerTypeRestore,
	WorkerTypeThumbnail,
	WorkerTypeIntake,
	WorkerTypeCurrencyMigration,
	FailedJobTypePersonalDataExport,
}

// FailedJobTypes returns the types that have failed jobs to show on the
// system-admin plane, in display order. The slice is a copy.
func FailedJobTypes() []WorkerType {
	return slices.Clone(failedJobTypes)
}

// FailedJobGroupTypes returns the types whose failed jobs belong to a group
// and show in group settings, in display order. The slice is a copy.
func FailedJobGroupTypes() []WorkerType {
	return slices.DeleteFunc(slices.Clone(failedJobTypes), func(t WorkerType) bool {
		return !IsFailedJobGroupType(t)
	})
}

// IsFailedJobType reports whether the type has failed jobs to show.
func IsFailedJobType(t WorkerType) bool {
	return slices.Contains(failedJobTypes, t)
}

// IsFailedJobGroupType reports whether the type has failed jobs that belong
// to a group. A personal data export belongs to a user.
func IsFailedJobGroupType(t WorkerType) bool {
	return IsFailedJobType(t) && t != FailedJobTypePersonalDataExport
}

// FailedJobRetryable reports whether a failed job of the type can be put
// back in its queue. A restore may have written part of its backup before it
// failed, and a currency migration ran on a rate confirmed for that moment,
// so a job of either type can only be discarded; the user starts a new one.
func FailedJobRetryable(t WorkerType) bool {
	switch t {
	case WorkerTypeExport, WorkerTypeImport, WorkerTypeThumbnail, WorkerTypeIntake, FailedJobTypePersonalDataExport:
		return true
	default:
		return false
	}
}
