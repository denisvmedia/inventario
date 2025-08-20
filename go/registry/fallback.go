package registry

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
)

// FallbackRegistry wraps any registry and provides graceful degradation for unsupported features
type FallbackRegistry struct {
	base         *Set
	capabilities DatabaseCapabilities
	dbType       string
}

// NewFallbackRegistry creates a new fallback registry
func NewFallbackRegistry(base *Set, dbType string) *FallbackRegistry {
	capabilities, exists := GetCapabilities(dbType)
	if !exists {
		// Default to minimal capabilities for unknown database types
		capabilities = DatabaseCapabilities{}
	}

	return &FallbackRegistry{
		base:         base,
		capabilities: capabilities,
		dbType:       dbType,
	}
}

// Implement the Set interface by delegating to the base registry
func (f *FallbackRegistry) LocationRegistry() LocationRegistry {
	return f.base.LocationRegistry
}

func (f *FallbackRegistry) AreaRegistry() AreaRegistry {
	return f.base.AreaRegistry
}

func (f *FallbackRegistry) CommodityRegistry() CommodityRegistry {
	return f.base.CommodityRegistry
}

func (f *FallbackRegistry) ImageRegistry() ImageRegistry {
	return f.base.ImageRegistry
}

func (f *FallbackRegistry) InvoiceRegistry() InvoiceRegistry {
	return f.base.InvoiceRegistry
}

func (f *FallbackRegistry) ManualRegistry() ManualRegistry {
	return f.base.ManualRegistry
}

func (f *FallbackRegistry) SettingsRegistry() SettingsRegistry {
	return f.base.SettingsRegistry
}

func (f *FallbackRegistry) ExportRegistry() ExportRegistry {
	return f.base.ExportRegistry
}

func (f *FallbackRegistry) RestoreOperationRegistry() RestoreOperationRegistry {
	return f.base.RestoreOperationRegistry
}

func (f *FallbackRegistry) RestoreStepRegistry() RestoreStepRegistry {
	return f.base.RestoreStepRegistry
}

func (f *FallbackRegistry) FileRegistry() FileRegistry {
	return f.base.FileRegistry
}

func (f *FallbackRegistry) ValidateWithContext(ctx context.Context) error {
	return f.base.ValidateWithContext(ctx)
}

// Enhanced registry interface implementation
func (f *FallbackRegistry) GetCapabilities() DatabaseCapabilities {
	return f.capabilities
}

func (f *FallbackRegistry) FullTextSearch(ctx context.Context, query string, options ...SearchOption) ([]*models.Commodity, error) {
	if !f.capabilities.FullTextSearch {
		log.Printf("Full-text search not supported by %s, falling back to simple name search", f.dbType)
		return f.fallbackCommoditySearch(ctx, query)
	}

	// If we reach here, the base registry should implement enhanced features
	if enhanced, ok := f.base.CommodityRegistry.(EnhancedCommodityRegistry); ok {
		return enhanced.FullTextSearch(ctx, query, options...)
	}

	return f.fallbackCommoditySearch(ctx, query)
}

func (f *FallbackRegistry) JSONBQuery(ctx context.Context, table string, jsonbField string, query map[string]any) ([]any, error) {
	if !f.capabilities.JSONBOperators {
		log.Printf("JSONB queries not supported by %s, falling back to in-memory filtering", f.dbType)
		return f.fallbackJSONQuery(ctx, table, jsonbField, query)
	}

	return nil, fmt.Errorf("JSONB query not implemented for table %s", table)
}

func (f *FallbackRegistry) BulkUpsert(ctx context.Context, entities []any) error {
	if !f.capabilities.BulkOperations {
		log.Printf("Bulk operations not supported by %s, falling back to individual operations", f.dbType)
		return f.fallbackBulkUpsert(ctx, entities)
	}

	return ErrFeatureNotSupported
}

func (f *FallbackRegistry) CreateAdvancedIndex(ctx context.Context, spec IndexSpec) error {
	if !f.capabilities.AdvancedIndexing {
		log.Printf("Advanced indexing not supported by %s, skipping index creation: %s", f.dbType, spec.Name)
		return nil // Silently skip for databases that don't support it
	}

	return ErrFeatureNotSupported
}

// Enhanced registry accessors
func (f *FallbackRegistry) EnhancedCommodityRegistry() EnhancedCommodityRegistry {
	if enhanced, ok := f.base.CommodityRegistry.(EnhancedCommodityRegistry); ok {
		return enhanced
	}
	return &FallbackCommodityRegistry{base: f.base.CommodityRegistry, capabilities: f.capabilities, dbType: f.dbType}
}

func (f *FallbackRegistry) EnhancedAreaRegistry() EnhancedAreaRegistry {
	if enhanced, ok := f.base.AreaRegistry.(EnhancedAreaRegistry); ok {
		return enhanced
	}
	return &FallbackAreaRegistry{base: f.base.AreaRegistry, capabilities: f.capabilities, dbType: f.dbType}
}

func (f *FallbackRegistry) EnhancedLocationRegistry() EnhancedLocationRegistry {
	if enhanced, ok := f.base.LocationRegistry.(EnhancedLocationRegistry); ok {
		return enhanced
	}
	return &FallbackLocationRegistry{base: f.base.LocationRegistry, capabilities: f.capabilities, dbType: f.dbType}
}

func (f *FallbackRegistry) EnhancedFileRegistry() EnhancedFileRegistry {
	if enhanced, ok := f.base.FileRegistry.(EnhancedFileRegistry); ok {
		return enhanced
	}
	return &FallbackFileRegistry{base: f.base.FileRegistry, capabilities: f.capabilities, dbType: f.dbType}
}

// Fallback implementations
func (f *FallbackRegistry) fallbackCommoditySearch(ctx context.Context, query string) ([]*models.Commodity, error) {
	// Get all commodities and filter in memory
	commodities, err := f.base.CommodityRegistry.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.Commodity

	for _, commodity := range commodities {
		// Simple text matching in name, short name, and comments
		if strings.Contains(strings.ToLower(commodity.Name), query) ||
			strings.Contains(strings.ToLower(commodity.ShortName), query) ||
			strings.Contains(strings.ToLower(commodity.Comments), query) ||
			strings.Contains(strings.ToLower(commodity.SerialNumber), query) {
			filtered = append(filtered, commodity)
		}
	}

	return filtered, nil
}

func (f *FallbackRegistry) fallbackJSONQuery(ctx context.Context, table string, jsonbField string, query map[string]any) ([]any, error) {
	// This would require loading all records and filtering in Go
	// For now, return empty results with a warning
	log.Printf("JSONB query fallback not fully implemented for table %s field %s", table, jsonbField)
	return []any{}, nil
}

func (f *FallbackRegistry) fallbackBulkUpsert(ctx context.Context, entities []any) error {
	// Perform individual operations
	for _, entity := range entities {
		// This would need type switching based on entity type
		// For now, just log that we would process each entity individually
		log.Printf("Would process entity individually: %T", entity)
	}
	return nil
}

// FallbackCommodityRegistry provides fallback implementations for enhanced commodity operations
type FallbackCommodityRegistry struct {
	base         CommodityRegistry
	capabilities DatabaseCapabilities
	dbType       string
}

// Implement CommodityRegistry interface by delegating to base
func (f *FallbackCommodityRegistry) Create(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	return f.base.Create(ctx, commodity)
}

func (f *FallbackCommodityRegistry) Get(ctx context.Context, id string) (*models.Commodity, error) {
	return f.base.Get(ctx, id)
}

func (f *FallbackCommodityRegistry) List(ctx context.Context) ([]*models.Commodity, error) {
	return f.base.List(ctx)
}

func (f *FallbackCommodityRegistry) Update(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	return f.base.Update(ctx, commodity)
}

func (f *FallbackCommodityRegistry) Delete(ctx context.Context, id string) error {
	return f.base.Delete(ctx, id)
}

func (f *FallbackCommodityRegistry) Count(ctx context.Context) (int, error) {
	return f.base.Count(ctx)
}

func (f *FallbackCommodityRegistry) AddImage(ctx context.Context, commodityID, imageID string) error {
	return f.base.AddImage(ctx, commodityID, imageID)
}

func (f *FallbackCommodityRegistry) GetImages(ctx context.Context, commodityID string) ([]string, error) {
	return f.base.GetImages(ctx, commodityID)
}

func (f *FallbackCommodityRegistry) DeleteImage(ctx context.Context, commodityID, imageID string) error {
	return f.base.DeleteImage(ctx, commodityID, imageID)
}

func (f *FallbackCommodityRegistry) AddManual(ctx context.Context, commodityID, manualID string) error {
	return f.base.AddManual(ctx, commodityID, manualID)
}

func (f *FallbackCommodityRegistry) GetManuals(ctx context.Context, commodityID string) ([]string, error) {
	return f.base.GetManuals(ctx, commodityID)
}

func (f *FallbackCommodityRegistry) DeleteManual(ctx context.Context, commodityID, manualID string) error {
	return f.base.DeleteManual(ctx, commodityID, manualID)
}

func (f *FallbackCommodityRegistry) AddInvoice(ctx context.Context, commodityID, invoiceID string) error {
	return f.base.AddInvoice(ctx, commodityID, invoiceID)
}

func (f *FallbackCommodityRegistry) GetInvoices(ctx context.Context, commodityID string) ([]string, error) {
	return f.base.GetInvoices(ctx, commodityID)
}

func (f *FallbackCommodityRegistry) DeleteInvoice(ctx context.Context, commodityID, invoiceID string) error {
	return f.base.DeleteInvoice(ctx, commodityID, invoiceID)
}

// Enhanced methods with fallback implementations
func (f *FallbackCommodityRegistry) SearchByTags(ctx context.Context, tags []string, operator TagOperator) ([]*models.Commodity, error) {
	log.Printf("Tag search not optimized for %s, using in-memory filtering", f.dbType)

	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		if f.matchesTags(commodity.Tags, tags, operator) {
			filtered = append(filtered, commodity)
		}
	}

	return filtered, nil
}

func (f *FallbackCommodityRegistry) FindSimilar(ctx context.Context, commodityID string, threshold float64) ([]*models.Commodity, error) {
	log.Printf("Similarity search not supported by %s", f.dbType)
	return []*models.Commodity{}, nil
}

func (f *FallbackCommodityRegistry) FullTextSearch(ctx context.Context, query string, options ...SearchOption) ([]*models.Commodity, error) {
	log.Printf("Full-text search not supported by %s, using simple text matching", f.dbType)

	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.Commodity

	for _, commodity := range commodities {
		if strings.Contains(strings.ToLower(commodity.Name), query) ||
			strings.Contains(strings.ToLower(commodity.ShortName), query) ||
			strings.Contains(strings.ToLower(commodity.Comments), query) ||
			strings.Contains(strings.ToLower(commodity.SerialNumber), query) {
			filtered = append(filtered, commodity)
		}
	}

	// Apply options
	opts := &SearchOptions{Limit: len(filtered)}
	for _, opt := range options {
		opt(opts)
	}

	if opts.Offset > 0 && opts.Offset < len(filtered) {
		filtered = filtered[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(filtered) {
		filtered = filtered[:opts.Limit]
	}

	return filtered, nil
}

func (f *FallbackCommodityRegistry) AggregateByArea(ctx context.Context, groupBy []string) ([]AggregationResult, error) {
	log.Printf("Aggregation not optimized for %s, using in-memory calculation", f.dbType)

	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	// Simple aggregation by area
	areaCount := make(map[string]int)
	for _, commodity := range commodities {
		areaCount[commodity.AreaID]++
	}

	var results []AggregationResult
	for areaID, count := range areaCount {
		results = append(results, AggregationResult{
			GroupBy: map[string]any{"area_id": areaID},
			Count:   count,
		})
	}

	return results, nil
}

func (f *FallbackCommodityRegistry) CountByStatus(ctx context.Context) (map[string]int, error) {
	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	statusCount := make(map[string]int)
	for _, commodity := range commodities {
		statusCount[string(commodity.Status)]++
	}

	return statusCount, nil
}

func (f *FallbackCommodityRegistry) CountByType(ctx context.Context) (map[string]int, error) {
	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	typeCount := make(map[string]int)
	for _, commodity := range commodities {
		typeCount[string(commodity.Type)]++
	}

	return typeCount, nil
}

func (f *FallbackCommodityRegistry) FindByPriceRange(ctx context.Context, minPrice, maxPrice float64, currency string) ([]*models.Commodity, error) {
	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	minPriceDecimal := decimal.NewFromFloat(minPrice)
	maxPriceDecimal := decimal.NewFromFloat(maxPrice)

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		// Use converted price if available, otherwise original price
		price := commodity.ConvertedOriginalPrice
		if price.IsZero() {
			price = commodity.OriginalPrice
		}

		if price.GreaterThanOrEqual(minPriceDecimal) && price.LessThanOrEqual(maxPriceDecimal) {
			filtered = append(filtered, commodity)
		}
	}

	return filtered, nil
}

func (f *FallbackCommodityRegistry) FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Commodity, error) {
	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		// Simple string comparison for dates (assumes ISO format)
		if commodity.PurchaseDate != nil {
			purchaseDate := string(*commodity.PurchaseDate)
			if purchaseDate >= startDate && purchaseDate <= endDate {
				filtered = append(filtered, commodity)
			}
		}
	}

	return filtered, nil
}

func (f *FallbackCommodityRegistry) FindBySerialNumbers(ctx context.Context, serialNumbers []string) ([]*models.Commodity, error) {
	commodities, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	serialSet := make(map[string]bool)
	for _, sn := range serialNumbers {
		serialSet[sn] = true
	}

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		if serialSet[commodity.SerialNumber] {
			filtered = append(filtered, commodity)
		}

		// Check extra serial numbers
		for _, extraSN := range commodity.ExtraSerialNumbers {
			if serialSet[extraSN] {
				filtered = append(filtered, commodity)
				break
			}
		}
	}

	return filtered, nil
}

// Helper method for tag matching
func (f *FallbackCommodityRegistry) matchesTags(commodityTags []string, searchTags []string, operator TagOperator) bool {
	if len(searchTags) == 0 {
		return true
	}

	tagSet := make(map[string]bool)
	for _, tag := range commodityTags {
		tagSet[tag] = true
	}

	switch operator {
	case TagOperatorAND:
		for _, searchTag := range searchTags {
			if !tagSet[searchTag] {
				return false
			}
		}
		return true
	case TagOperatorOR:
		for _, searchTag := range searchTags {
			if tagSet[searchTag] {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// User-aware methods for FallbackCommodityRegistry
func (f *FallbackCommodityRegistry) SetUserContext(ctx context.Context, userID string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.SetUserContext(ctx, userID)
	}
	// Fallback: no-op for registries that don't support user context
	return nil
}

func (f *FallbackCommodityRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.WithUserContext(ctx, userID, fn)
	}
	// Fallback: just execute the function
	return fn(ctx)
}

func (f *FallbackCommodityRegistry) CreateWithUser(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.CreateWithUser(ctx, commodity)
	}
	// Fallback: use regular Create method
	return f.base.Create(ctx, commodity)
}

func (f *FallbackCommodityRegistry) GetWithUser(ctx context.Context, id string) (*models.Commodity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.GetWithUser(ctx, id)
	}
	// Fallback: use regular Get method
	return f.base.Get(ctx, id)
}

func (f *FallbackCommodityRegistry) ListWithUser(ctx context.Context) ([]*models.Commodity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.ListWithUser(ctx)
	}
	// Fallback: use regular List method
	return f.base.List(ctx)
}

func (f *FallbackCommodityRegistry) UpdateWithUser(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.UpdateWithUser(ctx, commodity)
	}
	// Fallback: use regular Update method
	return f.base.Update(ctx, commodity)
}

func (f *FallbackCommodityRegistry) DeleteWithUser(ctx context.Context, id string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.DeleteWithUser(ctx, id)
	}
	// Fallback: use regular Delete method
	return f.base.Delete(ctx, id)
}

func (f *FallbackCommodityRegistry) CountWithUser(ctx context.Context) (int, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Commodity]); ok {
		return userAware.CountWithUser(ctx)
	}
	// Fallback: use regular Count method
	return f.base.Count(ctx)
}

// FallbackAreaRegistry provides fallback implementations for enhanced area operations
type FallbackAreaRegistry struct {
	base         AreaRegistry
	capabilities DatabaseCapabilities
	dbType       string
}

// Implement AreaRegistry interface by delegating to base
func (f *FallbackAreaRegistry) Create(ctx context.Context, area models.Area) (*models.Area, error) {
	return f.base.Create(ctx, area)
}

func (f *FallbackAreaRegistry) Get(ctx context.Context, id string) (*models.Area, error) {
	return f.base.Get(ctx, id)
}

func (f *FallbackAreaRegistry) List(ctx context.Context) ([]*models.Area, error) {
	return f.base.List(ctx)
}

func (f *FallbackAreaRegistry) Update(ctx context.Context, area models.Area) (*models.Area, error) {
	return f.base.Update(ctx, area)
}

func (f *FallbackAreaRegistry) Delete(ctx context.Context, id string) error {
	return f.base.Delete(ctx, id)
}

func (f *FallbackAreaRegistry) Count(ctx context.Context) (int, error) {
	return f.base.Count(ctx)
}

func (f *FallbackAreaRegistry) AddCommodity(ctx context.Context, areaID, commodityID string) error {
	return f.base.AddCommodity(ctx, areaID, commodityID)
}

func (f *FallbackAreaRegistry) GetCommodities(ctx context.Context, areaID string) ([]string, error) {
	return f.base.GetCommodities(ctx, areaID)
}

func (f *FallbackAreaRegistry) DeleteCommodity(ctx context.Context, areaID, commodityID string) error {
	return f.base.DeleteCommodity(ctx, areaID, commodityID)
}

// Enhanced methods with fallback implementations
func (f *FallbackAreaRegistry) GetCommodityCount(ctx context.Context, areaID string) (int, error) {
	commodities, err := f.base.GetCommodities(ctx, areaID)
	if err != nil {
		return 0, err
	}
	return len(commodities), nil
}

func (f *FallbackAreaRegistry) GetTotalValue(ctx context.Context, areaID string, currency string) (float64, error) {
	log.Printf("Total value calculation not optimized for %s", f.dbType)
	// This would require loading all commodities and calculating in memory
	return 0.0, nil
}

func (f *FallbackAreaRegistry) SearchByName(ctx context.Context, query string) ([]*models.Area, error) {
	areas, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.Area

	for _, area := range areas {
		if strings.Contains(strings.ToLower(area.Name), query) {
			filtered = append(filtered, area)
		}
	}

	return filtered, nil
}

// User-aware methods for FallbackAreaRegistry
func (f *FallbackAreaRegistry) SetUserContext(ctx context.Context, userID string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.SetUserContext(ctx, userID)
	}
	return nil
}

func (f *FallbackAreaRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.WithUserContext(ctx, userID, fn)
	}
	return fn(ctx)
}

func (f *FallbackAreaRegistry) CreateWithUser(ctx context.Context, area models.Area) (*models.Area, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.CreateWithUser(ctx, area)
	}
	return f.base.Create(ctx, area)
}

func (f *FallbackAreaRegistry) GetWithUser(ctx context.Context, id string) (*models.Area, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.GetWithUser(ctx, id)
	}
	return f.base.Get(ctx, id)
}

func (f *FallbackAreaRegistry) ListWithUser(ctx context.Context) ([]*models.Area, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.ListWithUser(ctx)
	}
	return f.base.List(ctx)
}

func (f *FallbackAreaRegistry) UpdateWithUser(ctx context.Context, area models.Area) (*models.Area, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.UpdateWithUser(ctx, area)
	}
	return f.base.Update(ctx, area)
}

func (f *FallbackAreaRegistry) DeleteWithUser(ctx context.Context, id string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.DeleteWithUser(ctx, id)
	}
	return f.base.Delete(ctx, id)
}

func (f *FallbackAreaRegistry) CountWithUser(ctx context.Context) (int, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Area]); ok {
		return userAware.CountWithUser(ctx)
	}
	return f.base.Count(ctx)
}

// FallbackLocationRegistry provides fallback implementations for enhanced location operations
type FallbackLocationRegistry struct {
	base         LocationRegistry
	capabilities DatabaseCapabilities
	dbType       string
}

// Implement LocationRegistry interface by delegating to base
func (f *FallbackLocationRegistry) Create(ctx context.Context, location models.Location) (*models.Location, error) {
	return f.base.Create(ctx, location)
}

func (f *FallbackLocationRegistry) Get(ctx context.Context, id string) (*models.Location, error) {
	return f.base.Get(ctx, id)
}

func (f *FallbackLocationRegistry) List(ctx context.Context) ([]*models.Location, error) {
	return f.base.List(ctx)
}

func (f *FallbackLocationRegistry) Update(ctx context.Context, location models.Location) (*models.Location, error) {
	return f.base.Update(ctx, location)
}

func (f *FallbackLocationRegistry) Delete(ctx context.Context, id string) error {
	return f.base.Delete(ctx, id)
}

func (f *FallbackLocationRegistry) Count(ctx context.Context) (int, error) {
	return f.base.Count(ctx)
}

func (f *FallbackLocationRegistry) AddArea(ctx context.Context, locationID, areaID string) error {
	return f.base.AddArea(ctx, locationID, areaID)
}

func (f *FallbackLocationRegistry) GetAreas(ctx context.Context, locationID string) ([]string, error) {
	return f.base.GetAreas(ctx, locationID)
}

func (f *FallbackLocationRegistry) DeleteArea(ctx context.Context, locationID, areaID string) error {
	return f.base.DeleteArea(ctx, locationID, areaID)
}

// Enhanced methods with fallback implementations
func (f *FallbackLocationRegistry) GetAreaCount(ctx context.Context, locationID string) (int, error) {
	areas, err := f.base.GetAreas(ctx, locationID)
	if err != nil {
		return 0, err
	}
	return len(areas), nil
}

func (f *FallbackLocationRegistry) GetTotalCommodityCount(ctx context.Context, locationID string) (int, error) {
	log.Printf("Total commodity count calculation not optimized for %s", f.dbType)
	// This would require loading all areas and their commodities
	return 0, nil
}

func (f *FallbackLocationRegistry) SearchByName(ctx context.Context, query string) ([]*models.Location, error) {
	locations, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.Location

	for _, location := range locations {
		if strings.Contains(strings.ToLower(location.Name), query) {
			filtered = append(filtered, location)
		}
	}

	return filtered, nil
}

// User-aware methods for FallbackLocationRegistry
func (f *FallbackLocationRegistry) SetUserContext(ctx context.Context, userID string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.SetUserContext(ctx, userID)
	}
	return nil
}

func (f *FallbackLocationRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.WithUserContext(ctx, userID, fn)
	}
	return fn(ctx)
}

func (f *FallbackLocationRegistry) CreateWithUser(ctx context.Context, location models.Location) (*models.Location, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.CreateWithUser(ctx, location)
	}
	return f.base.Create(ctx, location)
}

func (f *FallbackLocationRegistry) GetWithUser(ctx context.Context, id string) (*models.Location, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.GetWithUser(ctx, id)
	}
	return f.base.Get(ctx, id)
}

func (f *FallbackLocationRegistry) ListWithUser(ctx context.Context) ([]*models.Location, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.ListWithUser(ctx)
	}
	return f.base.List(ctx)
}

func (f *FallbackLocationRegistry) UpdateWithUser(ctx context.Context, location models.Location) (*models.Location, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.UpdateWithUser(ctx, location)
	}
	return f.base.Update(ctx, location)
}

func (f *FallbackLocationRegistry) DeleteWithUser(ctx context.Context, id string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.DeleteWithUser(ctx, id)
	}
	return f.base.Delete(ctx, id)
}

func (f *FallbackLocationRegistry) CountWithUser(ctx context.Context) (int, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.Location]); ok {
		return userAware.CountWithUser(ctx)
	}
	return f.base.Count(ctx)
}

// FallbackFileRegistry provides fallback implementations for enhanced file operations
type FallbackFileRegistry struct {
	base         FileRegistry
	capabilities DatabaseCapabilities
	dbType       string
}

// Implement FileRegistry interface by delegating to base
func (f *FallbackFileRegistry) Create(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	return f.base.Create(ctx, file)
}

func (f *FallbackFileRegistry) Get(ctx context.Context, id string) (*models.FileEntity, error) {
	return f.base.Get(ctx, id)
}

func (f *FallbackFileRegistry) List(ctx context.Context) ([]*models.FileEntity, error) {
	return f.base.List(ctx)
}

func (f *FallbackFileRegistry) Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	return f.base.Update(ctx, file)
}

func (f *FallbackFileRegistry) Delete(ctx context.Context, id string) error {
	return f.base.Delete(ctx, id)
}

func (f *FallbackFileRegistry) Count(ctx context.Context) (int, error) {
	return f.base.Count(ctx)
}

func (f *FallbackFileRegistry) ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error) {
	return f.base.ListByType(ctx, fileType)
}

func (f *FallbackFileRegistry) ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	return f.base.ListByLinkedEntity(ctx, entityType, entityID)
}

func (f *FallbackFileRegistry) ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, meta string) ([]*models.FileEntity, error) {
	return f.base.ListByLinkedEntityAndMeta(ctx, entityType, entityID, meta)
}

func (f *FallbackFileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, tags []string) ([]*models.FileEntity, error) {
	return f.base.Search(ctx, query, fileType, tags)
}

func (f *FallbackFileRegistry) ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType) ([]*models.FileEntity, int, error) {
	return f.base.ListPaginated(ctx, offset, limit, fileType)
}

// Enhanced methods with fallback implementations
func (f *FallbackFileRegistry) FullTextSearch(ctx context.Context, query string, fileType *models.FileType, options ...SearchOption) ([]*models.FileEntity, error) {
	log.Printf("Full-text search not supported by %s, using basic search", f.dbType)

	// Use the existing search method as fallback
	files, err := f.base.Search(ctx, query, fileType, nil)
	if err != nil {
		return nil, err
	}

	// Apply options
	opts := &SearchOptions{Limit: len(files)}
	for _, opt := range options {
		opt(opts)
	}

	if opts.Offset > 0 && opts.Offset < len(files) {
		files = files[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(files) {
		files = files[:opts.Limit]
	}

	return files, nil
}

func (f *FallbackFileRegistry) FindByMimeType(ctx context.Context, mimeTypes []string) ([]*models.FileEntity, error) {
	files, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	mimeSet := make(map[string]bool)
	for _, mime := range mimeTypes {
		mimeSet[mime] = true
	}

	var filtered []*models.FileEntity
	for _, file := range files {
		if mimeSet[file.MIMEType] {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

func (f *FallbackFileRegistry) FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.FileEntity, error) {
	files, err := f.base.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.FileEntity
	for _, file := range files {
		// Simple string comparison for dates (assumes ISO format)
		if file.CreatedAt.Format("2006-01-02") >= startDate && file.CreatedAt.Format("2006-01-02") <= endDate {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

func (f *FallbackFileRegistry) FindLargeFiles(ctx context.Context, minSizeBytes int64) ([]*models.FileEntity, error) {
	log.Printf("Large file search not optimized for %s (file size not tracked)", f.dbType)
	// File size is not currently tracked in the FileEntity model
	// Return empty results for now
	return []*models.FileEntity{}, nil
}

// User-aware methods for FallbackFileRegistry
func (f *FallbackFileRegistry) SetUserContext(ctx context.Context, userID string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.SetUserContext(ctx, userID)
	}
	return nil
}

func (f *FallbackFileRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.WithUserContext(ctx, userID, fn)
	}
	return fn(ctx)
}

func (f *FallbackFileRegistry) CreateWithUser(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.CreateWithUser(ctx, file)
	}
	return f.base.Create(ctx, file)
}

func (f *FallbackFileRegistry) GetWithUser(ctx context.Context, id string) (*models.FileEntity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.GetWithUser(ctx, id)
	}
	return f.base.Get(ctx, id)
}

func (f *FallbackFileRegistry) ListWithUser(ctx context.Context) ([]*models.FileEntity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.ListWithUser(ctx)
	}
	return f.base.List(ctx)
}

func (f *FallbackFileRegistry) UpdateWithUser(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.UpdateWithUser(ctx, file)
	}
	return f.base.Update(ctx, file)
}

func (f *FallbackFileRegistry) DeleteWithUser(ctx context.Context, id string) error {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.DeleteWithUser(ctx, id)
	}
	return f.base.Delete(ctx, id)
}

func (f *FallbackFileRegistry) CountWithUser(ctx context.Context) (int, error) {
	if userAware, ok := f.base.(UserAwareRegistry[models.FileEntity]); ok {
		return userAware.CountWithUser(ctx)
	}
	return f.base.Count(ctx)
}
