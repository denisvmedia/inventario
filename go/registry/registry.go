package registry

import (
	"context"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
)

type PIDable[T any] interface {
	*T
	IDable
}

type IDable interface {
	GetID() string
	SetID(id string)
}

type Registry[T any] interface {
	// Create creates a new T in the registry.
	Create(context.Context, T) (*T, error)

	// Get returns a T from the registry.
	Get(ctx context.Context, id string) (*T, error)

	// List returns a list of Ts from the registry.
	List(context.Context) ([]*T, error)

	// Update updates a T in the registry.
	Update(context.Context, T) (*T, error)

	// Delete deletes a T from the registry.
	Delete(ctx context.Context, id string) error

	// Count returns the number of Ts in the registry.
	Count(context.Context) (int, error)
}

type UserAwareRegistry[T any, P Registry[T]] interface {
	// WithCurrentUser returns a new registry with user context set
	WithCurrentUser(ctx context.Context) (P, error)
}

type AreaRegistry interface {
	Registry[models.Area]
	UserAwareRegistry[models.Area, AreaRegistry]

	GetCommodities(ctx context.Context, areaID string) ([]string, error)
}

type CommodityRegistry interface {
	Registry[models.Commodity]
	UserAwareRegistry[models.Commodity, CommodityRegistry]

	GetImages(ctx context.Context, commodityID string) ([]string, error)
	GetManuals(ctx context.Context, commodityID string) ([]string, error)
	GetInvoices(ctx context.Context, commodityID string) ([]string, error)

	// Enhanced search methods
	// SearchByTags(ctx context.Context, tags []string, operator TagOperator) ([]*models.Commodity, error)
	// FullTextSearch(ctx context.Context, query string, options ...SearchOption) ([]*models.Commodity, error)
	// FindSimilar(ctx context.Context, commodityID string, threshold float64) ([]*models.Commodity, error)
	// AggregateByArea(ctx context.Context, groupBy []string) ([]AggregationResult, error)
	// CountByStatus(ctx context.Context) (map[string]int, error)
	// CountByType(ctx context.Context) (map[string]int, error)
	// FindByPriceRange(ctx context.Context, minPrice, maxPrice float64, currency string) ([]*models.Commodity, error)
	// FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Commodity, error)
	// FindBySerialNumbers(ctx context.Context, serialNumbers []string) ([]*models.Commodity, error)
}

type LocationRegistry interface {
	Registry[models.Location]
	UserAwareRegistry[models.Location, LocationRegistry]

	GetAreas(ctx context.Context, locationID string) ([]string, error)
}

type ImageRegistry interface {
	Registry[models.Image]
	UserAwareRegistry[models.Image, ImageRegistry]
}

type InvoiceRegistry interface {
	Registry[models.Invoice]
	UserAwareRegistry[models.Invoice, InvoiceRegistry]
}

type ManualRegistry interface {
	Registry[models.Manual]
	UserAwareRegistry[models.Manual, ManualRegistry]
}

type SettingsRegistry interface {
	Get(ctx context.Context) (models.SettingsObject, error)
	Save(context.Context, models.SettingsObject) error
	Patch(ctx context.Context, configfield string, value any) error

	// WithCurrentUser returns a new registry with user context set
	WithCurrentUser(ctx context.Context) (SettingsRegistry, error)
}

type ExportRegistry interface {
	Registry[models.Export]
	UserAwareRegistry[models.Export, ExportRegistry]

	// ListWithDeleted returns all exports including soft deleted ones
	ListWithDeleted(ctx context.Context) ([]*models.Export, error)

	// ListDeleted returns only soft deleted exports
	ListDeleted(ctx context.Context) ([]*models.Export, error)

	// HardDelete permanently deletes an export from the database
	HardDelete(ctx context.Context, id string) error
}

type FileRegistry interface {
	Registry[models.FileEntity]
	UserAwareRegistry[models.FileEntity, FileRegistry]

	// ListByType returns files filtered by type
	ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error)

	// ListByLinkedEntity returns files linked to a specific entity
	ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error)

	// ListByLinkedEntityAndMeta returns files linked to a specific entity with specific metadata
	ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error)

	// Search returns files matching the search criteria
	Search(ctx context.Context, query string, fileType *models.FileType, tags []string) ([]*models.FileEntity, error)

	//// FullTextSearch performs enhanced text search on files
	// FullTextSearch(ctx context.Context, query string, fileType *models.FileType, options ...SearchOption) ([]*models.FileEntity, error)

	// ListPaginated returns paginated list of files
	ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType) ([]*models.FileEntity, int, error)
}

type RestoreOperationRegistry interface {
	Registry[models.RestoreOperation]
	UserAwareRegistry[models.RestoreOperation, RestoreOperationRegistry]

	// ListByExport returns all restore operations for an export
	ListByExport(ctx context.Context, exportID string) ([]*models.RestoreOperation, error)
}

type RestoreStepRegistry interface {
	Registry[models.RestoreStep]
	UserAwareRegistry[models.RestoreStep, RestoreStepRegistry]

	// ListByRestoreOperation returns all restore steps for a restore operation
	ListByRestoreOperation(ctx context.Context, restoreOperationID string) ([]*models.RestoreStep, error)

	// DeleteByRestoreOperation deletes all restore steps for a restore operation
	DeleteByRestoreOperation(ctx context.Context, restoreOperationID string) error
}

type TenantRegistry interface {
	Registry[models.Tenant]

	// GetBySlug returns a tenant by its slug
	GetBySlug(ctx context.Context, slug string) (*models.Tenant, error)

	// GetByDomain returns a tenant by its domain
	GetByDomain(ctx context.Context, domain string) (*models.Tenant, error)
}

type UserRegistry interface {
	Registry[models.User]

	// GetByEmail returns a user by email within a tenant
	GetByEmail(ctx context.Context, tenantID, email string) (*models.User, error)

	// ListByTenant returns all users for a tenant
	ListByTenant(ctx context.Context, tenantID string) ([]*models.User, error)

	// ListByRole returns all users with a specific role within a tenant
	ListByRole(ctx context.Context, tenantID string, role models.UserRole) ([]*models.User, error)
}

type Set struct {
	LocationRegistry         LocationRegistry
	AreaRegistry             AreaRegistry
	CommodityRegistry        CommodityRegistry
	ImageRegistry            ImageRegistry
	InvoiceRegistry          InvoiceRegistry
	ManualRegistry           ManualRegistry
	SettingsRegistry         SettingsRegistry
	ExportRegistry           ExportRegistry
	RestoreOperationRegistry RestoreOperationRegistry
	RestoreStepRegistry      RestoreStepRegistry
	FileRegistry             FileRegistry
	TenantRegistry           TenantRegistry
	UserRegistry             UserRegistry
}

// Search-related types and functions

// TagOperator defines how tags should be matched
type TagOperator string

const (
	TagOperatorAND TagOperator = "AND"
	TagOperatorOR  TagOperator = "OR"
)

// SearchOptions contains options for search operations
type SearchOptions struct {
	Limit  int
	Offset int
}

// SearchOption is a function that modifies SearchOptions
type SearchOption func(*SearchOptions)

// WithLimit sets the limit for search results
func WithLimit(limit int) SearchOption {
	return func(opts *SearchOptions) {
		opts.Limit = limit
	}
}

// WithOffset sets the offset for search results
func WithOffset(offset int) SearchOption {
	return func(opts *SearchOptions) {
		opts.Offset = offset
	}
}

// AggregationResult represents the result of an aggregation query
type AggregationResult struct {
	GroupBy map[string]any     `json:"group_by"`
	Count   int                `json:"count"`
	Avg     map[string]float64 `json:"avg,omitempty"`
	Sum     map[string]float64 `json:"sum,omitempty"`
	Min     map[string]float64 `json:"min,omitempty"`
	Max     map[string]float64 `json:"max,omitempty"`
}

// UserIDFromContext extracts the user ID from the context
func UserIDFromContext(ctx context.Context) string {
	return appctx.UserIDFromContext(ctx)
}

// WithUserContext adds a user ID to the context
func WithUserContext(ctx context.Context, userID string) context.Context {
	return appctx.WithUserID(ctx, userID)
}

func (s *Set) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&s.LocationRegistry, validation.Required),
		validation.Field(&s.AreaRegistry, validation.Required),
		validation.Field(&s.CommodityRegistry, validation.Required),
		validation.Field(&s.ImageRegistry, validation.Required),
		validation.Field(&s.ManualRegistry, validation.Required),
		validation.Field(&s.InvoiceRegistry, validation.Required),
		validation.Field(&s.SettingsRegistry, validation.Required),
		validation.Field(&s.ExportRegistry, validation.Required),
		validation.Field(&s.FileRegistry, validation.Required),
		validation.Field(&s.TenantRegistry, validation.Required),
		validation.Field(&s.UserRegistry, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, s, fields...)
}
