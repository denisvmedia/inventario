package models

import (
	"context"
	"encoding/json"

	"github.com/jellydator/validation"
)

// RestoreStatus represents the status of a restore operation
type RestoreStatus string

const (
	RestoreStatusPending   RestoreStatus = "pending"
	RestoreStatusRunning   RestoreStatus = "running"
	RestoreStatusCompleted RestoreStatus = "completed"
	RestoreStatusFailed    RestoreStatus = "failed"
)

var (
	_ validation.Validatable            = (*RestoreOperation)(nil)
	_ validation.ValidatableWithContext = (*RestoreOperation)(nil)
	_ IDable                            = (*RestoreOperation)(nil)
	_ json.Marshaler                    = (*RestoreOperation)(nil)
	_ json.Unmarshaler                  = (*RestoreOperation)(nil)
)

// RestoreOptions contains options for the restore operation
type RestoreOptions struct {
	Strategy        string `json:"strategy"`
	IncludeFileData bool   `json:"include_file_data"`
	DryRun          bool   `json:"dry_run"`
}

func (r RestoreOptions) Validate() error {
	return ErrMustUseValidateWithContext
}

func (r RestoreOptions) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&r.Strategy, validation.Required, validation.In(
			"full_replace",
			"merge_add",
			"merge_update",
		)),
	)

	return validation.ValidateStructWithContext(ctx, &r, fields...)
}

// RestoreOperation represents a restore operation performed on an export
//
// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="restore_operations" comment="Enable RLS for multi-tenant restore operation isolation"
//migrator:schema:rls:policy name="restore_operation_tenant_isolation" table="restore_operations" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id()" with_check="tenant_id = get_current_tenant_id()" comment="Ensures restore operations can only be accessed and modified by their tenant"

//migrator:schema:table name="restore_operations"
type RestoreOperation struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="export_id" type="TEXT" not_null="true" foreign="exports(id)" foreign_key_name="fk_restore_operation_export"
	ExportID string `json:"export_id" db:"export_id"`
	//migrator:schema:field name="description" type="TEXT" not_null="true"
	Description string `json:"description" db:"description"`
	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status RestoreStatus `json:"status" db:"status" userinput:"false"`
	//migrator:schema:field name="options" type="JSONB" not_null="true"
	Options RestoreOptions `json:"options" db:"options"`
	//migrator:schema:field name="created_date" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedDate PTimestamp `json:"created_date" db:"created_date" userinput:"false"`
	//migrator:schema:field name="started_date" type="TIMESTAMP"
	StartedDate PTimestamp `json:"started_date" db:"started_date" userinput:"false"`
	//migrator:schema:field name="completed_date" type="TIMESTAMP"
	CompletedDate PTimestamp `json:"completed_date" db:"completed_date" userinput:"false"`
	//migrator:schema:field name="error_message" type="TEXT"
	ErrorMessage string `json:"error_message" db:"error_message" userinput:"false"`

	// Statistics
	//migrator:schema:field name="location_count" type="INTEGER" default="0"
	LocationCount int `json:"location_count" db:"location_count" userinput:"false"`
	//migrator:schema:field name="area_count" type="INTEGER" default="0"
	AreaCount int `json:"area_count" db:"area_count" userinput:"false"`
	//migrator:schema:field name="commodity_count" type="INTEGER" default="0"
	CommodityCount int `json:"commodity_count" db:"commodity_count" userinput:"false"`
	//migrator:schema:field name="image_count" type="INTEGER" default="0"
	ImageCount int `json:"image_count" db:"image_count" userinput:"false"`
	//migrator:schema:field name="invoice_count" type="INTEGER" default="0"
	InvoiceCount int `json:"invoice_count" db:"invoice_count" userinput:"false"`
	//migrator:schema:field name="manual_count" type="INTEGER" default="0"
	ManualCount int `json:"manual_count" db:"manual_count" userinput:"false"`
	//migrator:schema:field name="binary_data_size" type="BIGINT" default="0"
	BinaryDataSize int64 `json:"binary_data_size" db:"binary_data_size" userinput:"false"`
	//migrator:schema:field name="error_count" type="INTEGER" default="0"
	ErrorCount int `json:"error_count" db:"error_count" userinput:"false"`

	// Related steps (not stored in DB, loaded separately)
	Steps []RestoreStep `json:"steps,omitempty" db:"-"`
}

// RestoreOperationIndexes defines performance indexes for the restore_operations table
type RestoreOperationIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_restore_operations_tenant_id" fields="tenant_id" table="restore_operations"
	_ int

	// Composite index for tenant + status queries
	//migrator:schema:index name="idx_restore_operations_tenant_status" fields="tenant_id,status" table="restore_operations"
	_ int

	// Composite index for tenant + export queries
	//migrator:schema:index name="idx_restore_operations_tenant_export" fields="tenant_id,export_id" table="restore_operations"
	_ int
}

func NewRestoreOperationFromUserInput(restoreOperation *RestoreOperation) RestoreOperation {
	result := *restoreOperation

	// Clean up any fields that should not be set by the client using the generic function
	SanitizeUserInput(&result)

	// Set specific values that are not zero but should be set by the system
	result.CreatedDate = PNow()
	result.Status = RestoreStatusPending

	return result
}

func (*RestoreOperation) Validate() error {
	return ErrMustUseValidateWithContext
}

func (r *RestoreOperation) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&r.ExportID, validation.Required),
		validation.Field(&r.Description, validation.Required, validation.Length(1, 500)),
		validation.Field(&r.Status, validation.Required),
		validation.Field(&r.Options),
		validation.Field(&r.ErrorMessage, validation.Length(0, 2000)),
	)

	return validation.ValidateStructWithContext(ctx, r, fields...)
}

func (r *RestoreOperation) MarshalJSON() ([]byte, error) {
	type Alias RestoreOperation
	tmp := *r
	return json.Marshal(Alias(tmp))
}

func (r *RestoreOperation) UnmarshalJSON(data []byte) error {
	type Alias RestoreOperation
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	return json.Unmarshal(data, &aux)
}
