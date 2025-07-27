package registry

import (
	"context"

	"github.com/denisvmedia/inventario/models"
)

// SearchOption configures search behavior
type SearchOption func(*SearchOptions)

// SearchOptions contains search configuration
type SearchOptions struct {
	Limit  int
	Offset int
	SortBy string
	Order  string // "ASC" or "DESC"
}

// WithLimit sets the search result limit
func WithLimit(limit int) SearchOption {
	return func(opts *SearchOptions) {
		opts.Limit = limit
	}
}

// WithOffset sets the search result offset
func WithOffset(offset int) SearchOption {
	return func(opts *SearchOptions) {
		opts.Offset = offset
	}
}

// WithSort sets the search result sorting
func WithSort(sortBy, order string) SearchOption {
	return func(opts *SearchOptions) {
		opts.SortBy = sortBy
		opts.Order = order
	}
}

// TagOperator defines how tags should be matched
type TagOperator string

const (
	TagOperatorAND TagOperator = "AND" // All tags must match
	TagOperatorOR  TagOperator = "OR"  // Any tag must match
)

// IndexSpec defines an advanced database index
type IndexSpec struct {
	Name      string
	Table     string
	Column    string
	Type      string // "gin_jsonb", "gist_tsvector", "btree_partial", etc.
	Condition string // For partial indexes
}

// AggregationResult represents aggregated data
type AggregationResult struct {
	GroupBy map[string]any
	Count   int
	Sum     map[string]float64
	Avg     map[string]float64
}

// EnhancedRegistry extends the base registry with advanced features
type EnhancedRegistry interface {
	// Embed all the base registry interfaces
	LocationRegistry() LocationRegistry
	AreaRegistry() AreaRegistry
	CommodityRegistry() CommodityRegistry
	ImageRegistry() ImageRegistry
	InvoiceRegistry() InvoiceRegistry
	ManualRegistry() ManualRegistry
	SettingsRegistry() SettingsRegistry
	ExportRegistry() ExportRegistry
	RestoreOperationRegistry() RestoreOperationRegistry
	RestoreStepRegistry() RestoreStepRegistry
	FileRegistry() FileRegistry
	ValidateWithContext(ctx context.Context) error

	// GetCapabilities returns the database capabilities
	GetCapabilities() DatabaseCapabilities

	// PostgreSQL-specific features (gracefully degrade for other databases)
	FullTextSearch(ctx context.Context, query string, options ...SearchOption) ([]*models.Commodity, error)
	JSONBQuery(ctx context.Context, table string, jsonbField string, query map[string]any) ([]any, error)
	BulkUpsert(ctx context.Context, entities []any) error
	CreateAdvancedIndex(ctx context.Context, spec IndexSpec) error

	// Enhanced registry accessors
	EnhancedCommodityRegistry() EnhancedCommodityRegistry
	EnhancedAreaRegistry() EnhancedAreaRegistry
	EnhancedLocationRegistry() EnhancedLocationRegistry
	EnhancedFileRegistry() EnhancedFileRegistry
}

// EnhancedCommodityRegistry extends CommodityRegistry with advanced features
type EnhancedCommodityRegistry interface {
	CommodityRegistry // Embed base interface

	// Advanced search capabilities
	SearchByTags(ctx context.Context, tags []string, operator TagOperator) ([]*models.Commodity, error)
	FindSimilar(ctx context.Context, commodityID string, threshold float64) ([]*models.Commodity, error)
	FullTextSearch(ctx context.Context, query string, options ...SearchOption) ([]*models.Commodity, error)

	// Aggregation and analytics
	AggregateByArea(ctx context.Context, groupBy []string) ([]AggregationResult, error)
	CountByStatus(ctx context.Context) (map[string]int, error)
	CountByType(ctx context.Context) (map[string]int, error)

	// Advanced filtering
	FindByPriceRange(ctx context.Context, minPrice, maxPrice float64, currency string) ([]*models.Commodity, error)
	FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Commodity, error)
	FindBySerialNumbers(ctx context.Context, serialNumbers []string) ([]*models.Commodity, error)
}

// EnhancedAreaRegistry extends AreaRegistry with advanced features
type EnhancedAreaRegistry interface {
	AreaRegistry // Embed base interface

	// Advanced queries
	GetCommodityCount(ctx context.Context, areaID string) (int, error)
	GetTotalValue(ctx context.Context, areaID string, currency string) (float64, error)
	SearchByName(ctx context.Context, query string) ([]*models.Area, error)
}

// EnhancedLocationRegistry extends LocationRegistry with advanced features
type EnhancedLocationRegistry interface {
	LocationRegistry // Embed base interface

	// Advanced queries
	GetAreaCount(ctx context.Context, locationID string) (int, error)
	GetTotalCommodityCount(ctx context.Context, locationID string) (int, error)
	SearchByName(ctx context.Context, query string) ([]*models.Location, error)
}

// EnhancedFileRegistry extends FileRegistry with advanced features
type EnhancedFileRegistry interface {
	FileRegistry // Embed base interface

	// Advanced search with full-text capabilities
	FullTextSearch(ctx context.Context, query string, fileType *models.FileType, options ...SearchOption) ([]*models.FileEntity, error)

	// Advanced filtering
	FindByMimeType(ctx context.Context, mimeTypes []string) ([]*models.FileEntity, error)
	FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.FileEntity, error)
	FindLargeFiles(ctx context.Context, minSizeBytes int64) ([]*models.FileEntity, error)
}
