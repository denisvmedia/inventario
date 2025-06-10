package models

import (
	"context"
	"encoding/json"

	"github.com/jellydator/validation"
)

// ImportStatus represents the status of an import operation
type ImportStatus string

const (
	ImportStatusPending   ImportStatus = "pending"
	ImportStatusRunning   ImportStatus = "running"
	ImportStatusCompleted ImportStatus = "completed"
	ImportStatusFailed    ImportStatus = "failed"
)

// ImportType represents the type of import operation
type ImportType string

const (
	ImportTypeXMLBackup ImportType = "xml_backup"
)

var (
	_ validation.Validatable = (*Import)(nil)
	_ IDable                 = (*Import)(nil)
)

// Import represents an import operation record
type Import struct {
	EntityID
	Type            ImportType  `json:"type" db:"type"`
	Status          ImportStatus `json:"status" db:"status"`
	SourceFilePath  string      `json:"source_file_path" db:"source_file_path"`
	CreatedDate     PTimestamp  `json:"created_date" db:"created_date"`
	StartedDate     PTimestamp  `json:"started_date" db:"started_date"`
	CompletedDate   PTimestamp  `json:"completed_date" db:"completed_date"`
	ErrorMessage    string      `json:"error_message" db:"error_message"`
	Description     string      `json:"description" db:"description"`
	
	// Import statistics
	LocationCount  int   `json:"location_count" db:"location_count"`
	AreaCount      int   `json:"area_count" db:"area_count"`
	CommodityCount int   `json:"commodity_count" db:"commodity_count"`
	ImageCount     int   `json:"image_count" db:"image_count"`
	InvoiceCount   int   `json:"invoice_count" db:"invoice_count"`
	ManualCount    int   `json:"manual_count" db:"manual_count"`
	BinaryDataSize int64 `json:"binary_data_size" db:"binary_data_size"`
	ErrorCount     int   `json:"error_count" db:"error_count"`
	Errors         ValuerSlice[string] `json:"errors" db:"errors"`
}

func (*Import) Validate() error {
	return ErrMustUseValidateWithContext
}

func (i *Import) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&i.Description, validation.Required, validation.Length(0, 500)),
		validation.Field(&i.Type, validation.Required),
		validation.Field(&i.SourceFilePath, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, i, fields...)
}

func (i *Import) MarshalJSON() ([]byte, error) {
	type Alias Import
	tmp := *i
	return json.Marshal(Alias(tmp))
}

// ImportProgress represents the current progress of an import operation
type ImportProgress struct {
	ImportID        string  `json:"import_id"`
	Phase           string  `json:"phase"`
	CurrentItem     string  `json:"current_item"`
	ProcessedItems  int     `json:"processed_items"`
	TotalItems      int     `json:"total_items"`
	PercentComplete float64 `json:"percent_complete"`
	LocationCount   int     `json:"location_count"`
	AreaCount       int     `json:"area_count"`
	CommodityCount  int     `json:"commodity_count"`
	ImageCount      int     `json:"image_count"`
	InvoiceCount    int     `json:"invoice_count"`
	ManualCount     int     `json:"manual_count"`
	BinaryDataSize  int64   `json:"binary_data_size"`
	ErrorCount      int     `json:"error_count"`
	Errors          []string `json:"errors"`
}

// ImportRequest represents a request to start an import operation
type ImportRequest struct {
	Type        ImportType `json:"type" validate:"required"`
	Description string     `json:"description" validate:"required,max=500"`
	FilePath    string     `json:"file_path" validate:"required"`
}

func (r *ImportRequest) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&r.Description, validation.Required, validation.Length(1, 500)),
		validation.Field(&r.Type, validation.Required),
		validation.Field(&r.FilePath, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, r, fields...)
}
