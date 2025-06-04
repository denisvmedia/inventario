package models

import (
	"context"
	"encoding/json"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*ExportStatus)(nil)
	_ validation.Validatable            = (*ExportType)(nil)
	_ validation.Validatable            = (*Export)(nil)
	_ validation.ValidatableWithContext = (*Export)(nil)
	_ IDable                            = (*Export)(nil)
	_ json.Marshaler                    = (*Export)(nil)
	_ json.Unmarshaler                  = (*Export)(nil)
)

type ExportStatus string

// Export statuses. Adding a new status? Don't forget to update IsValid() method.
const (
	ExportStatusPending    ExportStatus = "pending"
	ExportStatusInProgress ExportStatus = "in_progress"
	ExportStatusCompleted  ExportStatus = "completed"
	ExportStatusFailed     ExportStatus = "failed"
)

func (e ExportStatus) IsValid() bool {
	switch e {
	case ExportStatusPending,
		ExportStatusInProgress,
		ExportStatusCompleted,
		ExportStatusFailed:
		return true
	}
	return false
}

func (e ExportStatus) Validate() error {
	if !e.IsValid() {
		return validation.NewError("invalid_export_status", "invalid export status")
	}
	return nil
}

type ExportType string

// Export types. Adding a new type? Don't forget to update IsValid() method.
const (
	ExportTypeFullDatabase   ExportType = "full_database"
	ExportTypeSelectedItems  ExportType = "selected_items"
	ExportTypeLocations      ExportType = "locations"
	ExportTypeAreas          ExportType = "areas"
	ExportTypeCommodities    ExportType = "commodities"
)

func (e ExportType) IsValid() bool {
	switch e {
	case ExportTypeFullDatabase,
		ExportTypeSelectedItems,
		ExportTypeLocations,
		ExportTypeAreas,
		ExportTypeCommodities:
		return true
	}
	return false
}

func (e ExportType) Validate() error {
	if !e.IsValid() {
		return validation.NewError("invalid_export_type", "invalid export type")
	}
	return nil
}

type Export struct {
	EntityID
	Type              ExportType          `json:"type" db:"type"`
	Status            ExportStatus        `json:"status" db:"status"`
	IncludeFileData   bool               `json:"include_file_data" db:"include_file_data"`
	SelectedItemIDs   ValuerSlice[string] `json:"selected_item_ids" db:"selected_item_ids"`
	FilePath          string             `json:"file_path" db:"file_path"`
	CreatedDate       PDate              `json:"created_date" db:"created_date"`
	CompletedDate     *PDate             `json:"completed_date" db:"completed_date"`
	ErrorMessage      string             `json:"error_message" db:"error_message"`
	Description       string             `json:"description" db:"description"`
}

func (*Export) Validate() error {
	return ErrMustUseValidateWithContext
}

func (e *Export) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&e.Type, rules.NotEmpty),
		validation.Field(&e.Status, rules.NotEmpty),
		validation.Field(&e.CreatedDate, rules.NotEmpty),
		validation.Field(&e.Description, validation.Length(0, 500)),
		validation.Field(&e.ErrorMessage, validation.Length(0, 1000)),
		validation.Field(&e.FilePath, validation.Length(0, 500)),
	)

	// Validate selected item IDs only for selected_items type
	if e.Type == ExportTypeSelectedItems {
		fields = append(fields,
			validation.Field(&e.SelectedItemIDs, validation.Required, validation.Length(1, 1000)),
		)
	}

	return validation.ValidateStructWithContext(ctx, e, fields...)
}

func (e *Export) MarshalJSON() ([]byte, error) {
	type Alias Export
	tmp := *e
	return json.Marshal(Alias(tmp))
}

func (e *Export) UnmarshalJSON(data []byte) error {
	type Alias Export
	tmp := &Alias{}
	err := json.Unmarshal(data, tmp)
	if err != nil {
		return err
	}

	*e = Export(*tmp)
	return nil
}