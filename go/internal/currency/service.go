// Package currency provides currency conversion functionality.
package currency

import (
	"context"
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ConversionService handles currency conversion operations.
type ConversionService struct {
	commodityRegistry registry.CommodityRegistry
	rateProvider      RateProvider
}

// RateProvider provides exchange rates between currencies.
type RateProvider interface {
	GetExchangeRate(ctx context.Context, from, to string) (decimal.Decimal, error)
}

// NewConversionService creates a new ConversionService.
func NewConversionService(commodityRegistry registry.CommodityRegistry, rateProvider RateProvider) *ConversionService {
	return &ConversionService{
		commodityRegistry: commodityRegistry,
		rateProvider:      rateProvider,
	}
}

// ConvertCommodityPrices converts all commodity prices from one currency to another.
// This is used when the main currency is changed.
func (s *ConversionService) ConvertCommodityPrices(ctx context.Context, fromCurrency, toCurrency string) error {
	return s.ConvertCommodityPricesWithRate(ctx, fromCurrency, toCurrency, nil)
}

// ConvertCommodityPricesWithRate converts all commodity prices from one currency to another using a specific rate.
// If rate is nil, it will use the rate provider to get the exchange rate.
func (s *ConversionService) ConvertCommodityPricesWithRate(ctx context.Context, fromCurrency, toCurrency string, rate *decimal.Decimal) error {
	if fromCurrency == toCurrency {
		return nil // No conversion needed
	}

	var exchangeRate decimal.Decimal
	var err error

	// Use provided rate or get from rate provider
	if rate != nil {
		exchangeRate = *rate
	} else {
		exchangeRate, err = s.rateProvider.GetExchangeRate(ctx, fromCurrency, toCurrency)
		if err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to get exchange rate from %s to %s", fromCurrency, toCurrency))
		}
	}

	// Get all commodities
	commodities, err := s.commodityRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list commodities")
	}

	// Convert each commodity
	for _, commodity := range commodities {
		updated := false

		// Convert ConvertedOriginalPrice (from old main currency to new main currency)
		if !commodity.ConvertedOriginalPrice.IsZero() {
			commodity.ConvertedOriginalPrice = commodity.ConvertedOriginalPrice.Mul(exchangeRate)
			updated = true
		}

		// Convert CurrentPrice (from old main currency to new main currency)
		if !commodity.CurrentPrice.IsZero() {
			commodity.CurrentPrice = commodity.CurrentPrice.Mul(exchangeRate)
			updated = true
		}

		// If original price is in the old main currency, we need to handle it differently
		if string(commodity.OriginalPriceCurrency) == fromCurrency {
			// The original price currency becomes the new main currency
			commodity.OriginalPriceCurrency = models.Currency(toCurrency)

			// Convert the original price to the new currency
			if !commodity.OriginalPrice.IsZero() {
				commodity.OriginalPrice = commodity.OriginalPrice.Mul(exchangeRate)
				updated = true
			}

			// Clear converted original price since original is now in main currency
			commodity.ConvertedOriginalPrice = decimal.Zero
		}

		// Update the commodity if any changes were made
		if updated {
			_, err = s.commodityRegistry.Update(ctx, *commodity)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to update commodity %s", commodity.ID))
			}
		}
	}

	return nil
}

// StaticRateProvider provides fixed exchange rates for testing or simple scenarios.
type StaticRateProvider struct {
	rates map[string]decimal.Decimal
}

// NewStaticRateProvider creates a new StaticRateProvider with the given rates.
// The rates map should contain keys in the format "FROM_TO" (e.g., "USD_EUR").
func NewStaticRateProvider(rates map[string]decimal.Decimal) *StaticRateProvider {
	return &StaticRateProvider{rates: rates}
}

// GetExchangeRate returns the exchange rate from one currency to another.
func (p *StaticRateProvider) GetExchangeRate(ctx context.Context, from, to string) (decimal.Decimal, error) {
	if from == to {
		return decimal.NewFromInt(1), nil
	}

	key := fmt.Sprintf("%s_%s", from, to)
	rate, exists := p.rates[key]
	if !exists {
		return decimal.Zero, fmt.Errorf("exchange rate not found for %s to %s", from, to)
	}

	return rate, nil
}
