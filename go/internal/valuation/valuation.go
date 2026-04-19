// Package valuation provides functionality for calculating the total value of commodities.
package valuation

import (
	"context"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// CommodityValue represents the calculated value of a commodity.
type CommodityValue struct {
	CommodityID string          `json:"commodity_id"`
	Name        string          `json:"name"`
	AreaID      string          `json:"area_id"`
	LocationID  string          `json:"location_id"` // Derived from Area
	Value       decimal.Decimal `json:"value"`
}

// TotalValue represents the total value of commodities, possibly grouped by location or area.
type TotalValue struct {
	ID    string          `json:"id"`   // ID of the entity (location, area, or "global" for global total)
	Name  string          `json:"name"` // Name of the entity (location, area, or "Global" for global total)
	Value decimal.Decimal `json:"value"`
}

// Valuator provides methods for calculating commodity values.
type Valuator struct {
	CommodityRegistry registry.CommodityRegistry
	AreaRegistry      registry.AreaRegistry
	LocationRegistry  registry.LocationRegistry
	ctx               context.Context
}

// NewValuator creates a new Valuator instance. Pass a context carrying the
// group whose valuation you want (via appctx.WithGroup); GetMainCurrency
// falls back to USD when no group is on the context or the group's currency
// is empty, which is a safe default for the valuation math but NOT a
// replacement for producing the right totals — callers that care should
// stamp the group themselves.
func NewValuator(ctx context.Context, registrySet *registry.Set) *Valuator {
	return &Valuator{
		CommodityRegistry: registrySet.CommodityRegistry,
		AreaRegistry:      registrySet.AreaRegistry,
		LocationRegistry:  registrySet.LocationRegistry,
		ctx:               ctx,
	}
}

// GetMainCurrency returns the main currency of the group in context, falling
// back to USD when no group is attached or the group's currency is empty.
func (v *Valuator) GetMainCurrency() (string, error) {
	group := appctx.GroupFromContext(v.ctx)
	if group == nil || group.MainCurrency == "" {
		return "USD", nil
	}
	return string(group.MainCurrency), nil
}

// CalculateGlobalTotalValue calculates the total value of all commodities.
// It follows these rules:
// 1. Ignores draft commodities and commodities with status other than "in use"
// 2. If mainCurrency is empty, considers all prices in USD
// 3. Uses current price if available (which is in the main currency)
// 4. If no current price, uses original price if it's in the main currency
// 5. If no current price and original price is not in main currency, uses converted original price
// 6. If none of the above conditions are met, the commodity is not counted
func (v *Valuator) CalculateGlobalTotalValue() (decimal.Decimal, error) {
	ctx := v.ctx

	// Get main currency
	mainCurrency, err := v.GetMainCurrency()
	if err != nil {
		return decimal.Zero, err
	}

	// Get all commodities
	commodities, err := v.CommodityRegistry.List(ctx)
	if err != nil {
		return decimal.Zero, err
	}

	// Get all areas
	areas, err := v.AreaRegistry.List(ctx)
	if err != nil {
		return decimal.Zero, err
	}

	// Create a map of area IDs to location IDs for quick lookup
	areaToLocation := make(map[string]string)
	for _, area := range areas {
		areaToLocation[area.ID] = area.LocationID
	}

	// Calculate the total value
	total := decimal.NewFromInt(0)

	for _, commodity := range commodities {
		// Skip draft commodities
		if commodity.Draft {
			continue
		}

		// Skip commodities that are not in use
		if commodity.Status != models.CommodityStatusInUse {
			continue
		}

		// Calculate the value of the commodity
		value := getCommodityValue(commodity, mainCurrency)
		if value.IsZero() {
			// Skip commodities with no valid price
			continue
		}

		// Note: The price already represents the total value for all items in the lot

		// Add to the total
		total = total.Add(value)
	}

	return total, nil
}

// CalculateTotalValueByLocation calculates the total value of commodities grouped by location.
func (v *Valuator) CalculateTotalValueByLocation() (map[string]decimal.Decimal, error) {
	ctx := v.ctx

	// Get main currency
	mainCurrency, err := v.GetMainCurrency()
	if err != nil {
		return nil, err
	}

	// Get all commodities
	commodities, err := v.CommodityRegistry.List(ctx)
	if err != nil {
		return nil, err
	}

	// Get all areas
	areas, err := v.AreaRegistry.List(ctx)
	if err != nil {
		return nil, err
	}

	// Create a map of area IDs to location IDs for quick lookup
	areaToLocation := make(map[string]string)
	for _, area := range areas {
		areaToLocation[area.ID] = area.LocationID
	}

	// Calculate the total value by location
	locationTotals := make(map[string]decimal.Decimal)

	for _, commodity := range commodities {
		// Skip draft commodities
		if commodity.Draft {
			continue
		}

		// Skip commodities that are not in use
		if commodity.Status != models.CommodityStatusInUse {
			continue
		}

		// Calculate the value of the commodity
		value := getCommodityValue(commodity, mainCurrency)
		if value.IsZero() {
			// Skip commodities with no valid price
			continue
		}

		// Note: The price already represents the total value for all items in the lot

		// Get the location ID for this commodity
		locationID, ok := areaToLocation[commodity.AreaID]
		if !ok {
			// Skip commodities with no valid location
			continue
		}

		// Add to the location total
		if _, ok := locationTotals[locationID]; !ok {
			locationTotals[locationID] = decimal.NewFromInt(0)
		}
		locationTotals[locationID] = locationTotals[locationID].Add(value)
	}

	return locationTotals, nil
}

// CalculateTotalValueByArea calculates the total value of commodities grouped by area.
func (v *Valuator) CalculateTotalValueByArea() (map[string]decimal.Decimal, error) {
	ctx := v.ctx

	// Get main currency
	mainCurrency, err := v.GetMainCurrency()
	if err != nil {
		return nil, err
	}

	// Get all commodities
	commodities, err := v.CommodityRegistry.List(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate the total value by area
	areaTotals := make(map[string]decimal.Decimal)

	for _, commodity := range commodities {
		// Skip draft commodities
		if commodity.Draft {
			continue
		}

		// Skip commodities that are not in use
		if commodity.Status != models.CommodityStatusInUse {
			continue
		}

		// Calculate the value of the commodity
		value := getCommodityValue(commodity, mainCurrency)
		if value.IsZero() {
			// Skip commodities with no valid price
			continue
		}

		// Note: The price already represents the total value for all items in the lot

		// Add to the area total
		if _, ok := areaTotals[commodity.AreaID]; !ok {
			areaTotals[commodity.AreaID] = decimal.NewFromInt(0)
		}
		areaTotals[commodity.AreaID] = areaTotals[commodity.AreaID].Add(value)
	}

	return areaTotals, nil
}

// getCommodityValue returns the value of a commodity based on the specified rules.
// Returns zero decimal if the commodity has no valid price.
func getCommodityValue(commodity *models.Commodity, mainCurrency string) decimal.Decimal {
	// If we have current price, use it (the currency is our main currency)
	if !commodity.CurrentPrice.IsZero() {
		return commodity.CurrentPrice
	}

	// If no current price, check if the original price is in our main currency
	if !commodity.OriginalPrice.IsZero() && string(commodity.OriginalPriceCurrency) == mainCurrency {
		return commodity.OriginalPrice
	}

	// If still no price detected, check if we have converted original price
	if !commodity.ConvertedOriginalPrice.IsZero() {
		return commodity.ConvertedOriginalPrice
	}

	// If only original price set and the currency is not our main currency,
	// the commodity state is invalid and we should not count it
	return decimal.NewFromInt(0)
}
