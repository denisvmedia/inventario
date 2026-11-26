package models

import "time"

// PersonalDataExportStatus is where a personal data export is in its life.
type PersonalDataExportStatus string

const (
	// PersonalDataExportStatusPending waits for the export worker.
	PersonalDataExportStatusPending PersonalDataExportStatus = "pending"
	// PersonalDataExportStatusInProgress is being built by a worker.
	PersonalDataExportStatusInProgress PersonalDataExportStatus = "in_progress"
	// PersonalDataExportStatusCompleted has an archive ready to download.
	PersonalDataExportStatusCompleted PersonalDataExportStatus = "completed"
	// PersonalDataExportStatusFailed ended without an archive.
	PersonalDataExportStatusFailed PersonalDataExportStatus = "failed"
)

// IsValid reports whether the status is one of the known values.
func (s PersonalDataExportStatus) IsValid() bool {
	switch s {
	case PersonalDataExportStatusPending,
		PersonalDataExportStatusInProgress,
		PersonalDataExportStatusCompleted,
		PersonalDataExportStatusFailed:
		return true
	}
	return false
}

const (
	// PersonalDataExportRetention is how long a personal data export is kept.
	// A finished archive is deleted this long after it was finished, and a
	// request that never finished this long after it was made.
	PersonalDataExportRetention = 7 * 24 * time.Hour
	// PersonalDataExportRequestInterval is how long a user waits between two
	// requests. Every request counts, also one that failed: building the
	// archive reads every group the user belongs to and streams their files.
	PersonalDataExportRequestInterval = 24 * time.Hour
)

// Row-level security mirrors api_tokens: the application role sees only the
// caller's own rows. The export worker builds and expires the archives of
// every user, so the registry runs under the background-worker role, which the
// second policy admits, and every statement that reads one user's rows names
// the user itself.
//
//ptah:schema:rls:enable table="personal_data_exports" comment="Enable RLS for multi-tenant personal data export isolation"
//ptah:schema:rls:policy name="personal_data_export_isolation" table="personal_data_exports" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures personal data exports can only be accessed by the owning user within their tenant"
//ptah:schema:rls:policy name="personal_data_export_background_worker_access" table="personal_data_exports" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows the export worker to build and expire the personal data exports of every user"

// PersonalDataExport is a user's request for a copy of their own data (GDPR
// Art. 20): their profile, settings and memberships, and the rows they created
// in every group they belong to. It belongs to the user, not to a group, and
// is never restorable.
//
//ptah:schema:table name="personal_data_exports"
type PersonalDataExport struct {
	//ptah:embedded mode="inline"
	TenantUserAwareEntityID
	//ptah:schema:field name="status" type="VARCHAR(16)" not_null="true" default="pending"
	Status PersonalDataExportStatus `json:"status" db:"status"`
	// BlobKey is where the archive is stored, empty until it is.
	//ptah:schema:field name="blob_key" type="TEXT" not_null="true" default=""
	BlobKey string `json:"-" db:"blob_key"`
	// SizeBytes is the archive's size.
	//ptah:schema:field name="size_bytes" type="BIGINT" not_null="true" default="0"
	SizeBytes int64 `json:"size_bytes" db:"size_bytes"`
	// GroupCount, ItemCount and FileCount describe what the archive holds.
	//ptah:schema:field name="group_count" type="INTEGER" not_null="true" default="0"
	GroupCount int `json:"group_count" db:"group_count"`
	//ptah:schema:field name="item_count" type="INTEGER" not_null="true" default="0"
	ItemCount int `json:"item_count" db:"item_count"`
	//ptah:schema:field name="file_count" type="INTEGER" not_null="true" default="0"
	FileCount int `json:"file_count" db:"file_count"`
	// ErrorMessage is the reason a build failed. It is for the server log and
	// the operator; the API does not return it.
	//ptah:schema:field name="error_message" type="TEXT" not_null="true" default=""
	ErrorMessage string `json:"-" db:"error_message"`
	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	// StartedAt is when a worker claimed the export and began the build. A
	// build that started long ago and recorded no result died with its
	// process, and the worker queues it again.
	//ptah:schema:field name="started_at" type="TIMESTAMP"
	StartedAt *time.Time `json:"-" db:"started_at"`
	//ptah:schema:field name="completed_at" type="TIMESTAMP"
	CompletedAt *time.Time `json:"completed_at,omitempty" db:"completed_at"`
	// ExpiresAt is when the export worker deletes the row and its archive.
	//ptah:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
}

// PersonalDataExportIndexes defines PostgreSQL indexes for the
// personal_data_exports table.
type PersonalDataExportIndexes struct {
	// Unique index for the immutable UUID.
	//ptah:schema:index name="idx_personal_data_exports_uuid" fields="uuid" unique="true" table="personal_data_exports"
	_ int

	// The list and the daily limit read a user's rows newest first.
	//ptah:schema:index name="idx_personal_data_exports_user_created" fields="user_id,created_at" table="personal_data_exports"
	_ int

	// The worker looks for pending rows on every tick.
	//ptah:schema:index name="idx_personal_data_exports_status" fields="status" table="personal_data_exports"
	_ int

	// The worker deletes the rows past their expiry.
	//ptah:schema:index name="idx_personal_data_exports_expires_at" fields="expires_at" table="personal_data_exports"
	_ int
}

// IsExpired reports whether the export is past its expiry at now.
func (e *PersonalDataExport) IsExpired(now time.Time) bool {
	return !now.Before(e.ExpiresAt)
}

// IsDownloadable reports whether the archive can be downloaded at now.
func (e *PersonalDataExport) IsDownloadable(now time.Time) bool {
	return e.Status == PersonalDataExportStatusCompleted && e.BlobKey != "" && !e.IsExpired(now)
}
