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
type RestoreOperation struct {
	EntityID
	ExportID      string         `json:"export_id" db:"export_id"`
	Description   string         `json:"description" db:"description"`
	Status        RestoreStatus  `json:"status" db:"status" userinput:"false"`
	Options       RestoreOptions `json:"options" db:"options"`
	CreatedDate   PTimestamp     `json:"created_date" db:"created_date" userinput:"false"`
	StartedDate   PTimestamp     `json:"started_date" db:"started_date" userinput:"false"`
	CompletedDate PTimestamp     `json:"completed_date" db:"completed_date" userinput:"false"`
	ErrorMessage  string         `json:"error_message" db:"error_message" userinput:"false"`

	// Statistics
	LocationCount  int   `json:"location_count" db:"location_count" userinput:"false"`
	AreaCount      int   `json:"area_count" db:"area_count" userinput:"false"`
	CommodityCount int   `json:"commodity_count" db:"commodity_count" userinput:"false"`
	ImageCount     int   `json:"image_count" db:"image_count" userinput:"false"`
	InvoiceCount   int   `json:"invoice_count" db:"invoice_count" userinput:"false"`
	ManualCount    int   `json:"manual_count" db:"manual_count" userinput:"false"`
	BinaryDataSize int64 `json:"binary_data_size" db:"binary_data_size" userinput:"false"`
	ErrorCount     int   `json:"error_count" db:"error_count" json:"error_count" userinput:"false"`

	// Related steps (not stored in DB, loaded separately)
	Steps []RestoreStep `json:"steps,omitempty" db:"-"`
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
