package memory

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)

type baseCommodityRegistry = Registry[models.Commodity, *models.Commodity]
type CommodityRegistry struct {
	*baseCommodityRegistry

	userID       string
	imagesLock   sync.RWMutex
	images       models.CommodityImages
	manualsLock  sync.RWMutex
	manuals      models.CommodityManuals
	invoicesLock sync.RWMutex
	invoices     models.CommodityInvoices
	areaRegistry *AreaRegistry // required dependency for relationship tracking
}

func NewCommodityRegistry(areaRegistry *AreaRegistry) *CommodityRegistry {
	return &CommodityRegistry{
		baseCommodityRegistry: NewRegistry[models.Commodity, *models.Commodity](),
		images:                make(models.CommodityImages),
		manuals:               make(models.CommodityManuals),
		invoices:              make(models.CommodityInvoices),
		areaRegistry:          areaRegistry,
	}
}

func (r *CommodityRegistry) WithCurrentUser(ctx context.Context) (registry.CommodityRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	// Create a new registry with the same data but different userID
	tmp := &CommodityRegistry{
		baseCommodityRegistry: r.baseCommodityRegistry,
		userID:                user.ID,
		images:                r.images,
		manuals:               r.manuals,
		invoices:              r.invoices,
		areaRegistry:          r.areaRegistry,
	}

	// Set the userID on the base registry
	tmp.baseCommodityRegistry.userID = user.ID

	return tmp, nil
}

func (r *CommodityRegistry) Create(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Use CreateWithUser to ensure user context is applied
	newCommodity, err := r.baseCommodityRegistry.CreateWithUser(ctx, commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create commodity")
	}

	// Add this commodity to its parent area's commodity list
	_ = r.areaRegistry.AddCommodity(ctx, newCommodity.AreaID, newCommodity.GetID())

	return newCommodity, nil
}

func (r *CommodityRegistry) Delete(ctx context.Context, id string) error {
	// Remove this commodity from its parent area's commodity list
	commodity, err := r.baseCommodityRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodity")
	}

	_ = r.areaRegistry.DeleteCommodity(ctx, commodity.AreaID, id)

	err = r.baseCommodityRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity")
	}

	err = r.areaRegistry.DeleteCommodity(ctx, commodity.AreaID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity from area")
	}

	return nil
}

func (r *CommodityRegistry) AddImage(_ context.Context, commodityID, imageID string) error {
	r.imagesLock.Lock()
	r.images[commodityID] = append(r.images[commodityID], imageID)
	r.imagesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) GetImages(_ context.Context, commodityID string) ([]string, error) {
	r.imagesLock.RLock()
	images := make([]string, len(r.images[commodityID]))
	copy(images, r.images[commodityID])
	r.imagesLock.RUnlock()

	return images, nil
}

func (r *CommodityRegistry) DeleteImage(_ context.Context, commodityID, imageID string) error {
	r.imagesLock.Lock()
	for i, foundImageID := range r.images[commodityID] {
		if foundImageID == imageID {
			r.images[commodityID] = append(r.images[commodityID][:i], r.images[commodityID][i+1:]...)
			break
		}
	}
	r.imagesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) AddManual(_ context.Context, commodityID, manualID string) error {
	r.manualsLock.Lock()
	r.manuals[commodityID] = append(r.manuals[commodityID], manualID)
	r.manualsLock.Unlock()

	return nil
}

func (r *CommodityRegistry) GetManuals(_ context.Context, commodityID string) ([]string, error) {
	r.manualsLock.RLock()
	manuals := make([]string, len(r.manuals[commodityID]))
	copy(manuals, r.manuals[commodityID])
	r.manualsLock.RUnlock()

	return manuals, nil
}

func (r *CommodityRegistry) DeleteManual(_ context.Context, commodityID, manualID string) error {
	r.manualsLock.Lock()
	for i, foundManualID := range r.manuals[commodityID] {
		if foundManualID == manualID {
			r.manuals[commodityID] = append(r.manuals[commodityID][:i], r.manuals[commodityID][i+1:]...)
			break
		}
	}
	r.manualsLock.Unlock()

	return nil
}

func (r *CommodityRegistry) AddInvoice(_ context.Context, commodityID, invoiceID string) error {
	r.invoicesLock.Lock()
	r.invoices[commodityID] = append(r.invoices[commodityID], invoiceID)
	r.invoicesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) GetInvoices(_ context.Context, commodityID string) ([]string, error) {
	r.invoicesLock.RLock()
	invoices := make([]string, len(r.invoices[commodityID]))
	copy(invoices, r.invoices[commodityID])
	r.invoicesLock.RUnlock()

	return invoices, nil
}

func (r *CommodityRegistry) DeleteInvoice(_ context.Context, commodityID, invoiceID string) error {
	r.invoicesLock.Lock()
	for i, foundInvoiceID := range r.invoices[commodityID] {
		if foundInvoiceID == invoiceID {
			r.invoices[commodityID] = append(r.invoices[commodityID][:i], r.invoices[commodityID][i+1:]...)
			break
		}
	}
	r.invoicesLock.Unlock()

	return nil
}

func (r *CommodityRegistry) Update(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// Get the existing commodity to check if AreaID changed
	var oldAreaID string
	if existingCommodity, err := r.baseCommodityRegistry.Get(ctx, commodity.GetID()); err == nil {
		oldAreaID = existingCommodity.AreaID
	}

	// Call the base registry's Update method
	updatedCommodity, err := r.baseCommodityRegistry.Update(ctx, commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update commodity")
	}

	// Handle area registry tracking - area changed
	if oldAreaID != "" && oldAreaID != updatedCommodity.AreaID {
		// Remove from old area
		_ = r.areaRegistry.DeleteCommodity(ctx, oldAreaID, updatedCommodity.GetID())
		// Add to new area
		_ = r.areaRegistry.AddCommodity(ctx, updatedCommodity.AreaID, updatedCommodity.GetID())
	} else if oldAreaID == "" {
		// This is a fallback case - add to area if not already tracked
		_ = r.areaRegistry.AddCommodity(ctx, updatedCommodity.AreaID, updatedCommodity.GetID())
	}

	return updatedCommodity, nil
}

// Enhanced methods with simplified in-memory implementations

// SearchByTags searches commodities by tags using in-memory filtering
func (r *CommodityRegistry) SearchByTags(ctx context.Context, tags []string, operator registry.TagOperator) ([]*models.Commodity, error) {
	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		if r.matchesTags(commodity.Tags, tags, operator) {
			filtered = append(filtered, commodity)
		}
	}

	return filtered, nil
}

// matchesTags checks if commodity tags match the search criteria
func (r *CommodityRegistry) matchesTags(commodityTags []string, searchTags []string, operator registry.TagOperator) bool {
	if len(searchTags) == 0 {
		return true
	}

	switch operator {
	case registry.TagOperatorAND:
		for _, searchTag := range searchTags {
			found := false
			for _, commodityTag := range commodityTags {
				if strings.EqualFold(commodityTag, searchTag) {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
		return true
	case registry.TagOperatorOR:
		for _, searchTag := range searchTags {
			for _, commodityTag := range commodityTags {
				if strings.EqualFold(commodityTag, searchTag) {
					return true
				}
			}
		}
		return false
	default:
		return false
	}
}

// FindSimilar finds similar commodities using simple name comparison (simplified)
func (r *CommodityRegistry) FindSimilar(ctx context.Context, commodityID string, threshold float64) ([]*models.Commodity, error) {
	// Get the reference commodity
	refCommodity, err := r.Get(ctx, commodityID)
	if err != nil {
		return nil, err
	}

	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var similar []*models.Commodity
	refName := strings.ToLower(refCommodity.Name)

	for _, commodity := range commodities {
		if commodity.ID == commodityID {
			continue
		}

		// Simple similarity check based on common words
		commodityName := strings.ToLower(commodity.Name)
		if r.calculateSimpleSimilarity(refName, commodityName) > threshold {
			similar = append(similar, commodity)
		}
	}

	return similar, nil
}

// calculateSimpleSimilarity calculates a simple similarity score between two strings
func (r *CommodityRegistry) calculateSimpleSimilarity(s1, s2 string) float64 {
	words1 := strings.Fields(s1)
	words2 := strings.Fields(s2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	commonWords := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 {
				commonWords++
				break
			}
		}
	}

	// Simple similarity score: common words / max words
	maxWords := len(words1)
	if len(words2) > maxWords {
		maxWords = len(words2)
	}

	return float64(commonWords) / float64(maxWords)
}

// FullTextSearch performs simple text search on commodities (simplified)
func (r *CommodityRegistry) FullTextSearch(ctx context.Context, query string, options ...registry.SearchOption) ([]*models.Commodity, error) {
	commodities, err := r.List(ctx)
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
	opts := &registry.SearchOptions{Limit: len(filtered)}
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

// AggregateByArea aggregates commodities by area (simplified)
func (r *CommodityRegistry) AggregateByArea(ctx context.Context, groupBy []string) ([]registry.AggregationResult, error) {
	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	areaMap := make(map[string][]float64)
	for _, commodity := range commodities {
		if commodity.Draft {
			continue
		}
		price, _ := commodity.OriginalPrice.Float64()
		if !commodity.ConvertedOriginalPrice.IsZero() {
			price, _ = commodity.ConvertedOriginalPrice.Float64()
		}
		areaMap[commodity.AreaID] = append(areaMap[commodity.AreaID], price)
	}

	var results []registry.AggregationResult
	for areaID, prices := range areaMap {
		count := len(prices)
		var sum, avg float64
		for _, price := range prices {
			sum += price
		}
		if count > 0 {
			avg = sum / float64(count)
		}

		result := registry.AggregationResult{
			GroupBy: map[string]any{"area_id": areaID},
			Count:   count,
			Avg:     map[string]float64{"price": avg},
			Sum:     map[string]float64{"price": sum},
		}
		results = append(results, result)
	}

	return results, nil
}

// CountByStatus counts commodities by status (simplified)
func (r *CommodityRegistry) CountByStatus(ctx context.Context) (map[string]int, error) {
	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for _, commodity := range commodities {
		result[string(commodity.Status)]++
	}

	return result, nil
}

// CountByType counts commodities by type (simplified)
func (r *CommodityRegistry) CountByType(ctx context.Context) (map[string]int, error) {
	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]int)
	for _, commodity := range commodities {
		result[string(commodity.Type)]++
	}

	return result, nil
}

// FindByPriceRange finds commodities within a price range (simplified)
func (r *CommodityRegistry) FindByPriceRange(ctx context.Context, minPrice, maxPrice float64, currency string) ([]*models.Commodity, error) {
	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		price, _ := commodity.OriginalPrice.Float64()
		if !commodity.ConvertedOriginalPrice.IsZero() {
			price, _ = commodity.ConvertedOriginalPrice.Float64()
		}

		// Check currency if specified
		if currency != "" && string(commodity.OriginalPriceCurrency) != currency {
			continue
		}

		if price >= minPrice && price <= maxPrice {
			filtered = append(filtered, commodity)
		}
	}

	return filtered, nil
}

// FindByDateRange finds commodities within a date range (simplified)
func (r *CommodityRegistry) FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.Commodity, error) {
	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, errkit.Wrap(err, "invalid start date format")
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, errkit.Wrap(err, "invalid end date format")
	}

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		if commodity.PurchaseDate != nil {
			purchaseDate, err := time.Parse("2006-01-02", string(*commodity.PurchaseDate))
			if err != nil {
				continue
			}
			if (purchaseDate.Equal(start) || purchaseDate.After(start)) &&
				(purchaseDate.Equal(end) || purchaseDate.Before(end)) {
				filtered = append(filtered, commodity)
			}
		}
	}

	return filtered, nil
}

// FindBySerialNumbers finds commodities by serial numbers (simplified)
func (r *CommodityRegistry) FindBySerialNumbers(ctx context.Context, serialNumbers []string) ([]*models.Commodity, error) {
	commodities, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.Commodity
	for _, commodity := range commodities {
		// Check main serial number
		for _, searchSerial := range serialNumbers {
			if commodity.SerialNumber == searchSerial {
				filtered = append(filtered, commodity)
				break
			}
		}

		// Check extra serial numbers
		for _, extraSerial := range commodity.ExtraSerialNumbers {
			for _, searchSerial := range serialNumbers {
				if extraSerial == searchSerial {
					filtered = append(filtered, commodity)
					break
				}
			}
		}
	}

	return filtered, nil
}
