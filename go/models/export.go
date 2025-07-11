package models

import (
	"context"
	"encoding/json"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable            = (*ExportStatus)(nil)
	_ validation.Validatable            = (*ExportType)(nil)
	_ validation.Validatable            = (*ExportSelectedItem)(nil)
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
	return ErrMustUseValidateWithContext
}

func (e ExportStatus) ValidateWithContext(context.Context) error {
	if !e.IsValid() {
		return validation.NewError("invalid_export_status", "invalid export status")
	}
	return nil
}

type ExportType string

// Export types. Adding a new type? Don't forget to update IsValid() method.
const (
	ExportTypeFullDatabase  ExportType = "full_database"
	ExportTypeSelectedItems ExportType = "selected_items"
	ExportTypeLocations     ExportType = "locations"
	ExportTypeAreas         ExportType = "areas"
	ExportTypeCommodities   ExportType = "commodities"
	ExportTypeImported      ExportType = "imported"
)

func (e ExportType) IsValid() bool {
	switch e {
	case ExportTypeFullDatabase,
		ExportTypeSelectedItems,
		ExportTypeLocations,
		ExportTypeAreas,
		ExportTypeCommodities,
		ExportTypeImported:
		return true
	}
	return false
}

func (e ExportType) Validate() error {
	return ErrMustUseValidateWithContext
}

func (e ExportType) ValidateWithContext(context.Context) error {
	if !e.IsValid() {
		return validation.NewError("invalid_export_type", "invalid export type")
	}
	return nil
}

type ExportSelectedItemType string

// Export selected item types. Adding a new type? Don't forget to update IsValid() method.
const (
	ExportSelectedItemTypeLocation  ExportSelectedItemType = "location"
	ExportSelectedItemTypeArea      ExportSelectedItemType = "area"
	ExportSelectedItemTypeCommodity ExportSelectedItemType = "commodity"
)

func (e ExportSelectedItemType) IsValid() bool {
	switch e {
	case ExportSelectedItemTypeLocation,
		ExportSelectedItemTypeArea,
		ExportSelectedItemTypeCommodity:
		return true
	}
	return false
}

func (e ExportSelectedItemType) Validate() error {
	return ErrMustUseValidateWithContext
}

func (e ExportSelectedItemType) ValidateWithContext(context.Context) error {
	if !e.IsValid() {
		return validation.NewError("invalid_export_selected_item_type", "invalid export selected item type")
	}
	return nil
}

type ExportSelectedItem struct {
	ID         string                 `json:"id"`
	Type       ExportSelectedItemType `json:"type"`
	Name       string                 `json:"name"`
	IncludeAll bool                   `json:"include_all,omitempty"`
	// Relationship fields for preserving hierarchy snapshot
	LocationID string `json:"location_id,omitempty"` // For areas: which location they belong to
	AreaID     string `json:"area_id,omitempty"`     // For commodities: which area they belong to
}

func (e ExportSelectedItem) Validate() error {
	return ErrMustUseValidateWithContext
}

func (e ExportSelectedItem) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, &e,
		validation.Field(&e.ID, validation.Required, validation.Length(1, 100)),
		validation.Field(&e.Type, validation.Required),
	)
}

type Export struct {
	EntityID
	Type            ExportType                      `json:"type" db:"type"`
	Status          ExportStatus                    `json:"status" db:"status" userinput:"false"`
	IncludeFileData bool                            `json:"include_file_data" db:"include_file_data"`
	SelectedItems   ValuerSlice[ExportSelectedItem] `json:"selected_items" db:"selected_items"`
	FileID          string                          `json:"file_id" db:"file_id" userinput:"false"`
	FilePath        string                          `json:"file_path" db:"file_path" userinput:"false"` // Deprecated: will be removed after migration
	CreatedDate     PTimestamp                      `json:"created_date" db:"created_date" userinput:"false"`
	CompletedDate   PTimestamp                      `json:"completed_date" db:"completed_date" userinput:"false"`
	DeletedAt       PTimestamp                      `json:"deleted_at" db:"deleted_at" userinput:"false"`
	ErrorMessage    string                          `json:"error_message" db:"error_message" userinput:"false"`
	Description     string                          `json:"description" db:"description"`
	Imported        bool                            `json:"imported" db:"imported" userinput:"false"`
	// Export statistics
	FileSize       int64 `json:"file_size" db:"file_size" userinput:"false"`
	LocationCount  int   `json:"location_count" db:"location_count" userinput:"false"`
	AreaCount      int   `json:"area_count" db:"area_count" userinput:"false"`
	CommodityCount int   `json:"commodity_count" db:"commodity_count" userinput:"false"`
	ImageCount     int   `json:"image_count" db:"image_count" userinput:"false"`
	InvoiceCount   int   `json:"invoice_count" db:"invoice_count" userinput:"false"`
	ManualCount    int   `json:"manual_count" db:"manual_count" userinput:"false"`
	BinaryDataSize int64 `json:"binary_data_size" db:"binary_data_size" userinput:"false"`
}

func NewImportedExport(description, sourceFilePath string) Export {
	return Export{
		Description: description,
		Type:        ExportTypeImported,
		Status:      ExportStatusPending,
		CreatedDate: PNow(),
		FilePath:    sourceFilePath, // Temporary: will be replaced by FileID during import processing
		Imported:    true,
	}
}

func NewExportFromUserInput(export *Export) Export {
	result := *export

	// Clean up any fields that should not be set by the client using the generic function
	SanitizeUserInput(&result)

	// Set specific values that are not zero but should be set by the system
	result.CreatedDate = PNow()
	result.Status = ExportStatusPending
	result.Imported = false

	return result
}

func (*Export) Validate() error {
	return ErrMustUseValidateWithContext
}

func (e *Export) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&e.Description, validation.Required, validation.Length(0, 500)),
		validation.Field(&e.Type, validation.Required),
	)

	// Validate selected items only for selected_items type
	if e.Type == ExportTypeSelectedItems {
		fields = append(fields,
			validation.Field(&e.SelectedItems, validation.Required, validation.Length(1, 1000)),
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

// IsDeleted returns true if the export has been soft deleted
func (e *Export) IsDeleted() bool {
	return e.DeletedAt != nil
}

// CanPerformOperations returns true if operations can be performed on this export
func (e *Export) CanPerformOperations() bool {
	return !e.IsDeleted()
}
