package schema

import (
	"time"
)

// Location represents a physical location where areas are located
//
//migrator:schema:table name="locations"
type Location struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name"`

	//migrator:schema:field name="address" type="TEXT" not_null="true"
	Address string `json:"address"`
}

// Area represents a specific area within a location
//
//migrator:schema:table name="areas"
type Area struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name"`

	//migrator:schema:field name="location_id" type="TEXT" not_null="true" foreign="locations(id)" foreign_key_name="fk_area_location"
	LocationID string `json:"location_id"`
}

// Commodity represents an inventory item
//
//migrator:schema:table name="commodities"
type Commodity struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name"`

	//migrator:schema:field name="short_name" type="TEXT"
	ShortName string `json:"short_name"`

	//migrator:schema:field name="type" type="TEXT" not_null="true"
	Type string `json:"type"`

	//migrator:schema:field name="area_id" type="TEXT" not_null="true" foreign="areas(id)" foreign_key_name="fk_commodity_area"
	AreaID string `json:"area_id"`

	//migrator:schema:field name="count" type="INTEGER" not_null="true" default="1"
	Count int `json:"count"`

	//migrator:schema:field name="original_price" type="DECIMAL(15,2)"
	OriginalPrice *float64 `json:"original_price"`

	//migrator:schema:field name="original_price_currency" type="TEXT"
	OriginalPriceCurrency *string `json:"original_price_currency"`

	//migrator:schema:field name="converted_original_price" type="DECIMAL(15,2)"
	ConvertedOriginalPrice *float64 `json:"converted_original_price"`

	//migrator:schema:field name="current_price" type="DECIMAL(15,2)"
	CurrentPrice *float64 `json:"current_price"`

	//migrator:schema:field name="serial_number" type="TEXT"
	SerialNumber *string `json:"serial_number"`

	//migrator:schema:field name="extra_serial_numbers" type="JSONB"
	ExtraSerialNumbers []string `json:"extra_serial_numbers"`

	//migrator:schema:field name="part_numbers" type="JSONB"
	PartNumbers []string `json:"part_numbers"`

	//migrator:schema:field name="tags" type="JSONB"
	Tags []string `json:"tags"`

	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status string `json:"status"`

	//migrator:schema:field name="purchase_date" type="TEXT"
	PurchaseDate *string `json:"purchase_date"`

	//migrator:schema:field name="registered_date" type="TEXT"
	RegisteredDate *string `json:"registered_date"`

	//migrator:schema:field name="last_modified_date" type="TEXT"
	LastModifiedDate *string `json:"last_modified_date"`

	//migrator:schema:field name="urls" type="JSONB"
	URLs []string `json:"urls"`

	//migrator:schema:field name="comments" type="TEXT"
	Comments *string `json:"comments"`

	//migrator:schema:field name="draft" type="BOOLEAN" not_null="true" default="false"
	Draft bool `json:"draft"`

	// PostgreSQL-specific full-text search vector (added by migration)
	//migrator:schema:field name="search_vector" type="TSVECTOR"
	SearchVector *string `json:"-"`
}

// CommodityIndexes defines indexes for the commodities table
//
//migrator:schema:index name="commodities_search_vector_idx" fields="search_vector" type="GIN"
//migrator:schema:index name="commodities_tags_gin_idx" fields="tags" type="GIN"
//migrator:schema:index name="commodities_extra_serial_numbers_gin_idx" fields="extra_serial_numbers" type="GIN"
//migrator:schema:index name="commodities_part_numbers_gin_idx" fields="part_numbers" type="GIN"
//migrator:schema:index name="commodities_urls_gin_idx" fields="urls" type="GIN"
//migrator:schema:index name="commodities_active_idx" fields="status,area_id" condition="draft = false"
//migrator:schema:index name="commodities_draft_idx" fields="last_modified_date" condition="draft = true"
//migrator:schema:index name="commodities_name_trgm_idx" fields="name" type="GIN" platform.postgres.ops="gin_trgm_ops"
//migrator:schema:index name="commodities_short_name_trgm_idx" fields="short_name" type="GIN" platform.postgres.ops="gin_trgm_ops"
type CommodityIndexes struct{}

// Image represents an image file associated with a commodity
//
//migrator:schema:table name="images"
type Image struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_image_commodity"
	CommodityID string `json:"commodity_id"`

	//migrator:schema:field name="path" type="TEXT" not_null="true"
	Path string `json:"path"`

	//migrator:schema:field name="original_path" type="TEXT" not_null="true"
	OriginalPath string `json:"original_path"`

	//migrator:schema:field name="ext" type="TEXT" not_null="true"
	Ext string `json:"ext"`

	//migrator:schema:field name="mime_type" type="TEXT" not_null="true"
	MimeType string `json:"mime_type"`
}

// Invoice represents an invoice file associated with a commodity
//
//migrator:schema:table name="invoices"
type Invoice struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_invoice_commodity"
	CommodityID string `json:"commodity_id"`

	//migrator:schema:field name="path" type="TEXT" not_null="true"
	Path string `json:"path"`

	//migrator:schema:field name="original_path" type="TEXT" not_null="true"
	OriginalPath string `json:"original_path"`

	//migrator:schema:field name="ext" type="TEXT" not_null="true"
	Ext string `json:"ext"`

	//migrator:schema:field name="mime_type" type="TEXT" not_null="true"
	MimeType string `json:"mime_type"`
}

// Manual represents a manual file associated with a commodity
//
//migrator:schema:table name="manuals"
type Manual struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_manual_commodity"
	CommodityID string `json:"commodity_id"`

	//migrator:schema:field name="path" type="TEXT" not_null="true"
	Path string `json:"path"`

	//migrator:schema:field name="original_path" type="TEXT" not_null="true"
	OriginalPath string `json:"original_path"`

	//migrator:schema:field name="ext" type="TEXT" not_null="true"
	Ext string `json:"ext"`

	//migrator:schema:field name="mime_type" type="TEXT" not_null="true"
	MimeType string `json:"mime_type"`
}

// Settings represents application settings stored as key-value pairs
//
//migrator:schema:table name="settings"
type Settings struct {
	//migrator:schema:field name="key" type="TEXT" primary="true"
	Key string `json:"key"`

	//migrator:schema:field name="value" type="JSONB" not_null="true"
	Value any `json:"value"`
}

// Export represents an export operation
//
//migrator:schema:table name="exports"
type Export struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="type" type="TEXT" not_null="true"
	Type string `json:"type"`

	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status string `json:"status"`

	//migrator:schema:field name="created_date" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedDate time.Time `json:"created_date"`

	//migrator:schema:field name="completed_date" type="TIMESTAMP"
	CompletedDate *time.Time `json:"completed_date"`

	//migrator:schema:field name="file_id" type="TEXT" foreign="files(id)" foreign_key_name="fk_export_file"
	FileID *string `json:"file_id"`

	//migrator:schema:field name="imported" type="BOOLEAN" not_null="true" default="false"
	Imported bool `json:"imported"`

	//migrator:schema:field name="deleted_at" type="TIMESTAMP"
	DeletedAt *time.Time `json:"deleted_at"`

	//migrator:schema:field name="statistics" type="JSONB"
	Statistics any `json:"statistics"`
}

// FileEntity represents a generic file entity
//
//migrator:schema:table name="files"
type FileEntity struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="type" type="TEXT" not_null="true"
	Type string `json:"type"`

	//migrator:schema:field name="title" type="TEXT"
	Title *string `json:"title"`

	//migrator:schema:field name="description" type="TEXT"
	Description *string `json:"description"`

	//migrator:schema:field name="path" type="TEXT" not_null="true"
	Path string `json:"path"`

	//migrator:schema:field name="original_path" type="TEXT" not_null="true"
	OriginalPath string `json:"original_path"`

	//migrator:schema:field name="ext" type="TEXT" not_null="true"
	Ext string `json:"ext"`

	//migrator:schema:field name="mime_type" type="TEXT" not_null="true"
	MimeType string `json:"mime_type"`

	//migrator:schema:field name="linked_entity_type" type="TEXT"
	LinkedEntityType *string `json:"linked_entity_type"`

	//migrator:schema:field name="linked_entity_id" type="TEXT"
	LinkedEntityID *string `json:"linked_entity_id"`

	//migrator:schema:field name="linked_entity_meta" type="TEXT"
	LinkedEntityMeta *string `json:"linked_entity_meta"`

	//migrator:schema:field name="tags" type="JSONB"
	Tags []string `json:"tags"`

	//migrator:schema:field name="readonly" type="BOOLEAN" not_null="true" default="false"
	ReadOnly bool `json:"readonly"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at"`

	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at"`

	// PostgreSQL-specific full-text search vector (added by migration)
	//migrator:schema:field name="search_vector" type="TSVECTOR"
	SearchVector *string `json:"-"`
}

// FileIndexes defines indexes for the files table
//
//migrator:schema:index name="files_search_vector_idx" fields="search_vector" type="GIN"
//migrator:schema:index name="files_tags_gin_idx" fields="tags" type="GIN"
//migrator:schema:index name="files_type_created_idx" fields="type,created_at"
//migrator:schema:index name="files_linked_entity_idx" fields="linked_entity_type,linked_entity_id"
//migrator:schema:index name="files_linked_entity_meta_idx" fields="linked_entity_type,linked_entity_id,linked_entity_meta"
//migrator:schema:index name="files_title_trgm_idx" fields="title" type="GIN" platform.postgres.ops="gin_trgm_ops"
//migrator:schema:index name="files_path_trgm_idx" fields="path" type="GIN" platform.postgres.ops="gin_trgm_ops"
type FileIndexes struct{}

// RestoreOperation represents a restore operation
//
//migrator:schema:table name="restore_operations"
type RestoreOperation struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="export_id" type="TEXT" not_null="true" foreign="exports(id)" foreign_key_name="fk_restore_export"
	ExportID string `json:"export_id"`

	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status string `json:"status"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at"`

	//migrator:schema:field name="completed_at" type="TIMESTAMP"
	CompletedAt *time.Time `json:"completed_at"`

	//migrator:schema:field name="error_message" type="TEXT"
	ErrorMessage *string `json:"error_message"`
}

// RestoreStep represents a step in a restore operation
//
//migrator:schema:table name="restore_steps"
type RestoreStep struct {
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id"`

	//migrator:schema:field name="restore_operation_id" type="TEXT" not_null="true" foreign="restore_operations(id)" foreign_key_name="fk_restore_step_operation"
	RestoreOperationID string `json:"restore_operation_id"`

	//migrator:schema:field name="step_number" type="INTEGER" not_null="true"
	StepNumber int `json:"step_number"`

	//migrator:schema:field name="description" type="TEXT" not_null="true"
	Description string `json:"description"`

	//migrator:schema:field name="status" type="TEXT" not_null="true"
	Status string `json:"status"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at"`

	//migrator:schema:field name="completed_at" type="TIMESTAMP"
	CompletedAt *time.Time `json:"completed_at"`

	//migrator:schema:field name="error_message" type="TEXT"
	ErrorMessage *string `json:"error_message"`
}
