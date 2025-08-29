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

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="exports" comment="Enable RLS for multi-tenant export isolation"
//migrator:schema:rls:policy name="export_isolation" table="exports" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures exports can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="export_background_worker_access" table="exports" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all exports for processing"

//migrator:schema:table name="exports"
type Export struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="type" type="TEXT" not_null="true"
	Type ExportType `json:"type" db:"type"`
	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status ExportStatus `json:"status" db:"status" userinput:"false"`
	//migrator:schema:field name="include_file_data" type="BOOLEAN" not_null="true" default="false"
	IncludeFileData bool `json:"include_file_data" db:"include_file_data"`
	//migrator:schema:field name="selected_items" type="JSONB"
	SelectedItems ValuerSlice[ExportSelectedItem] `json:"selected_items" db:"selected_items"`
	//migrator:schema:field name="file_id" type="TEXT" foreign="files(id)" foreign_key_name="fk_export_file" on_delete="SET NULL"
	FileID *string `json:"file_id" db:"file_id" userinput:"false"`
	//migrator:schema:field name="file_path" type="TEXT"
	FilePath string `json:"file_path" db:"file_path" userinput:"false"` // Deprecated: will be removed after migration
	//migrator:schema:field name="created_date" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedDate PTimestamp `json:"created_date" db:"created_date" userinput:"false"`
	//migrator:schema:field name="completed_date" type="TIMESTAMP"
	CompletedDate PTimestamp `json:"completed_date" db:"completed_date" userinput:"false"`
	//migrator:schema:field name="deleted_at" type="TIMESTAMP"
	DeletedAt PTimestamp `json:"deleted_at" db:"deleted_at" userinput:"false"`
	//migrator:schema:field name="error_message" type="TEXT"
	ErrorMessage string `json:"error_message" db:"error_message" userinput:"false"`
	//migrator:schema:field name="description" type="TEXT"
	Description string `json:"description" db:"description"`
	//migrator:schema:field name="imported" type="BOOLEAN" not_null="true" default="false"
	Imported bool `json:"imported" db:"imported" userinput:"false"`
	// Export statistics
	//migrator:schema:field name="file_size" type="BIGINT" default="0"
	FileSize int64 `json:"file_size" db:"file_size" userinput:"false"`
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

// ExportIndexes defines performance indexes for the exports table
type ExportIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_exports_tenant_id" fields="tenant_id" table="exports"
	_ int

	// Composite index for tenant + status queries
	//migrator:schema:index name="idx_exports_tenant_status" fields="tenant_id,status" table="exports"
	_ int

	// Composite index for tenant + type queries
	//migrator:schema:index name="idx_exports_tenant_type" fields="tenant_id,type" table="exports"
	_ int
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
