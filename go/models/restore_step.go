package models

import (
	"context"
	"encoding/json"

	"github.com/jellydator/validation"
)

// RestoreStepResult represents the result status of a restore step
type RestoreStepResult string

const (
	RestoreStepResultTodo       RestoreStepResult = "todo"
	RestoreStepResultInProgress RestoreStepResult = "in_progress"
	RestoreStepResultSuccess    RestoreStepResult = "success"
	RestoreStepResultError      RestoreStepResult = "error"
	RestoreStepResultSkipped    RestoreStepResult = "skipped"
)

func (r RestoreStepResult) IsValid() bool {
	switch r {
	case RestoreStepResultTodo,
		RestoreStepResultInProgress,
		RestoreStepResultSuccess,
		RestoreStepResultError,
		RestoreStepResultSkipped:
		return true
	}
	return false
}

func (r RestoreStepResult) Validate() error {
	return ErrMustUseValidateWithContext
}

func (r RestoreStepResult) ValidateWithContext(ctx context.Context) error {
	if !r.IsValid() {
		return validation.NewError("validation_invalid_restore_step_result", "must be a valid restore step result")
	}
	return nil
}

var (
	_ validation.Validatable            = (*RestoreStepResult)(nil)
	_ validation.ValidatableWithContext = (*RestoreStepResult)(nil)
	_ validation.Validatable            = (*RestoreStep)(nil)
	_ validation.ValidatableWithContext = (*RestoreStep)(nil)
	_ IDable                            = (*RestoreStep)(nil)
	_ json.Marshaler                    = (*RestoreStep)(nil)
	_ json.Unmarshaler                  = (*RestoreStep)(nil)
)

// RestoreStep represents an individual step in a restore operation
//
// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="restore_steps" comment="Enable RLS for multi-tenant restore step isolation"
//migrator:schema:rls:policy name="restore_step_tenant_isolation" table="restore_steps" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id()" with_check="tenant_id = get_current_tenant_id()" comment="Ensures restore steps can only be accessed and modified by their tenant"
//migrator:schema:rls:policy name="restore_step_user_isolation" table="restore_steps" for="ALL" to="inventario_app" using="user_id = get_current_user_id()" with_check="user_id = get_current_user_id()" comment="Ensures restore steps can only be accessed and modified by their user"
//migrator:schema:rls:policy name="restore_step_background_worker_access" table="restore_steps" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all restore steps for processing"

//migrator:schema:table name="restore_steps"
type RestoreStep struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="restore_operation_id" type="TEXT" not_null="true" foreign="restore_operations(id)" foreign_key_name="fk_restore_step_operation"
	RestoreOperationID string `json:"restore_operation_id" db:"restore_operation_id"`
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//migrator:schema:field name="result" type="TEXT" not_null="true"
	Result RestoreStepResult `json:"result" db:"result"`
	//migrator:schema:field name="duration" type="BIGINT"
	Duration *int64 `json:"duration" db:"duration"` // Duration in milliseconds
	//migrator:schema:field name="reason" type="TEXT"
	Reason string `json:"reason" db:"reason"` // Reason for error or skip
	//migrator:schema:field name="created_date" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedDate PTimestamp `json:"created_date" db:"created_date"`
	//migrator:schema:field name="updated_date" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedDate PTimestamp `json:"updated_date" db:"updated_date"`
}

// RestoreStepIndexes defines performance indexes for the restore_steps table
type RestoreStepIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_restore_steps_tenant_id" fields="tenant_id" table="restore_steps"
	_ int

	// Composite index for tenant + restore operation queries
	//migrator:schema:index name="idx_restore_steps_tenant_operation" fields="tenant_id,restore_operation_id" table="restore_steps"
	_ int

	// Composite index for tenant + result queries
	//migrator:schema:index name="idx_restore_steps_tenant_result" fields="tenant_id,result" table="restore_steps"
	_ int
}

func (*RestoreStep) Validate() error {
	return ErrMustUseValidateWithContext
}

func (r *RestoreStep) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&r.RestoreOperationID, validation.Required),
		validation.Field(&r.Name, validation.Required, validation.Length(1, 255)),
		validation.Field(&r.Result, validation.Required),
		validation.Field(&r.Reason, validation.Length(0, 1000)),
	)

	return validation.ValidateStructWithContext(ctx, r, fields...)
}

func (r *RestoreStep) MarshalJSON() ([]byte, error) {
	type Alias RestoreStep
	tmp := *r
	return json.Marshal(Alias(tmp))
}

func (r *RestoreStep) UnmarshalJSON(data []byte) error {
	type Alias RestoreStep
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}
	return json.Unmarshal(data, &aux)
}
