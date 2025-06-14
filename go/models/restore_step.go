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
type RestoreStep struct {
	EntityID
	RestoreOperationID string            `json:"restore_operation_id" db:"restore_operation_id"`
	Name               string            `json:"name" db:"name"`
	Result             RestoreStepResult `json:"result" db:"result"`
	Duration           *int64            `json:"duration" db:"duration"` // Duration in milliseconds
	Reason             string            `json:"reason" db:"reason"`     // Reason for error or skip
	CreatedDate        PTimestamp        `json:"created_date" db:"created_date"`
	UpdatedDate        PTimestamp        `json:"updated_date" db:"updated_date"`
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
