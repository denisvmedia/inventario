package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"
)

// ThumbnailGenerationStatus represents the status of thumbnail generation
type ThumbnailGenerationStatus string

const (
	ThumbnailStatusPending    ThumbnailGenerationStatus = "pending"
	ThumbnailStatusProcessing ThumbnailGenerationStatus = "processing"
	ThumbnailStatusCompleted  ThumbnailGenerationStatus = "completed"
	ThumbnailStatusFailed     ThumbnailGenerationStatus = "failed"
)

// String returns the string representation of ThumbnailGenerationStatus
func (s ThumbnailGenerationStatus) String() string {
	return string(s)
}

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="thumbnail_generation_jobs" comment="Enable RLS for multi-tenant thumbnail generation job isolation"
//migrator:schema:rls:policy name="thumbnail_generation_job_isolation" table="thumbnail_generation_jobs" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures thumbnail generation jobs can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="thumbnail_generation_job_background_worker_access" table="thumbnail_generation_jobs" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all thumbnail generation jobs for processing"

//migrator:schema:table name="thumbnail_generation_jobs"
type ThumbnailGenerationJob struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// FileID is the ID of the file for which thumbnails need to be generated
	//migrator:schema:field name="file_id" type="TEXT" not_null="true" foreign="files(id)" foreign_key_name="fk_thumbnail_job_file"
	FileID string `json:"file_id" db:"file_id"`

	// Status represents the current status of the thumbnail generation
	//migrator:schema:field name="status" type="TEXT" not_null="true" default="pending"
	Status ThumbnailGenerationStatus `json:"status" db:"status"`

	// AttemptCount tracks how many times generation has been attempted
	//migrator:schema:field name="attempt_count" type="INTEGER" not_null="true" default="0"
	AttemptCount int `json:"attempt_count" db:"attempt_count"`

	// MaxAttempts defines the maximum number of attempts before marking as failed
	//migrator:schema:field name="max_attempts" type="INTEGER" not_null="true" default="3"
	MaxAttempts int `json:"max_attempts" db:"max_attempts"`

	// ErrorMessage stores the last error message if generation failed
	//migrator:schema:field name="error_message" type="TEXT"
	ErrorMessage string `json:"error_message" db:"error_message"`

	// ProcessingStartedAt tracks when processing began
	//migrator:schema:field name="processing_started_at" type="TIMESTAMP"
	ProcessingStartedAt *time.Time `json:"processing_started_at" db:"processing_started_at"`

	// ProcessingCompletedAt tracks when processing completed
	//migrator:schema:field name="processing_completed_at" type="TIMESTAMP"
	ProcessingCompletedAt *time.Time `json:"processing_completed_at" db:"processing_completed_at"`

	// CreatedAt is when the job was created
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// UpdatedAt is when the job was last updated
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PostgreSQL-specific indexes for thumbnail generation jobs
type ThumbnailGenerationJobIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_thumbnail_jobs_tenant_id" fields="tenant_id" table="thumbnail_generation_jobs"
	_ int

	// Composite index for status queries (for job processing)
	//migrator:schema:index name="idx_thumbnail_jobs_status_created" fields="status,created_at ASC" table="thumbnail_generation_jobs"
	_ int

	// Index for file-based queries
	//migrator:schema:index name="idx_thumbnail_jobs_file_id" fields="file_id" table="thumbnail_generation_jobs"
	_ int

	// Composite index for user and status queries
	//migrator:schema:index name="idx_thumbnail_jobs_user_status" fields="user_id,status" table="thumbnail_generation_jobs"
	_ int

	// Index for cleanup queries (completed/failed jobs)
	//migrator:schema:index name="idx_thumbnail_jobs_cleanup" fields="status,processing_completed_at" table="thumbnail_generation_jobs"
	_ int
}

func (*ThumbnailGenerationJob) Validate() error {
	return ErrMustUseValidateWithContext
}

func (t *ThumbnailGenerationJob) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&t.FileID, validation.Required),
		validation.Field(&t.Status, validation.Required, validation.In(
			ThumbnailStatusPending,
			ThumbnailStatusProcessing,
			ThumbnailStatusCompleted,
			ThumbnailStatusFailed,
		)),
		validation.Field(&t.AttemptCount, validation.Min(0)),
		validation.Field(&t.MaxAttempts, validation.Min(1)),
	)

	return validation.ValidateStructWithContext(ctx, t, fields...)
}

// SlotStatus represents the status of a concurrency slot
type SlotStatus string

const (
	SlotStatusActive     SlotStatus = "active"
	SlotStatusProcessing SlotStatus = "processing"
)

// String returns the string representation of SlotStatus
func (s SlotStatus) String() string {
	return string(s)
}

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="user_concurrency_slots" comment="Enable RLS for multi-tenant user concurrency slot isolation"
//migrator:schema:rls:policy name="user_concurrency_slot_isolation" table="user_concurrency_slots" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures user concurrency slots can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="user_concurrency_slot_background_worker_access" table="user_concurrency_slots" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all user concurrency slots for coordination"

//migrator:schema:table name="user_concurrency_slots"
type UserConcurrencySlot struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// JobID is the ID of the thumbnail generation job currently using this slot
	//migrator:schema:field name="job_id" type="TEXT" not_null="true" foreign="thumbnail_generation_jobs(id)" foreign_key_name="fk_concurrency_slot_job"
	JobID string `json:"job_id" db:"job_id"`

	// Status represents the current status of the slot
	//migrator:schema:field name="status" type="TEXT" not_null="true" default="active"
	Status SlotStatus `json:"status" db:"status"`

	// CreatedAt is when the slot was created
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// UpdatedAt is when the slot was last updated
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// PostgreSQL-specific indexes for user concurrency slots
type UserConcurrencySlotIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_user_concurrency_slots_tenant_id" fields="tenant_id" table="user_concurrency_slots"
	_ int

	// Index for user-based queries
	//migrator:schema:index name="idx_user_concurrency_slots_user_id" fields="user_id" table="user_concurrency_slots"
	_ int

	// Unique index for job-based queries (one slot per job)
	//migrator:schema:index name="idx_user_concurrency_slots_job_id" fields="job_id" unique="true" table="user_concurrency_slots"
	_ int

	// Index for status-based queries
	//migrator:schema:index name="idx_user_concurrency_slots_status" fields="status" table="user_concurrency_slots"
	_ int

	// Composite index for user and status queries
	//migrator:schema:index name="idx_user_concurrency_slots_user_status" fields="user_id,status" table="user_concurrency_slots"
	_ int
}

func (*UserConcurrencySlot) Validate() error {
	return ErrMustUseValidateWithContext
}

func (u *UserConcurrencySlot) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&u.JobID, validation.Required),
		validation.Field(&u.Status, validation.Required, validation.In(
			SlotStatusActive,
			SlotStatusProcessing,
		)),
	)

	return validation.ValidateStructWithContext(ctx, u, fields...)
}

// GetID returns the ID of the user concurrency slot
func (u *UserConcurrencySlot) GetID() string {
	return u.ID
}
