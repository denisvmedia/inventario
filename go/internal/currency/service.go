// Package currency provides scoped commodity price conversion helpers.
package currency

import (
	"context"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var (
	// ErrExchangeRateRequired is returned when no exchange rate is available for a currency pair.
	ErrExchangeRateRequired = errx.NewSentinel("exchange rate required for currency conversion")

	// ErrInvalidExchangeRate is returned when the provided exchange rate is not greater than zero.
	ErrInvalidExchangeRate = errx.NewSentinel("exchange rate must be greater than zero")
)

const convertedMoneyScale = 2

var defaultRates = map[string]decimal.Decimal{
	"USD_EUR": decimal.RequireFromString("0.85"),
	"EUR_USD": decimal.RequireFromString("1.18"),
	"EUR_GBP": decimal.RequireFromString("0.86"),
	"GBP_EUR": decimal.RequireFromString("1.16"),
}

// ConversionService converts commodity prices using a scoped commodity registry.
type ConversionService struct {
	commodityRegistry registry.CommodityRegistry
	rateProvider      RateProvider
}

// RateProvider resolves an exchange rate for a currency pair.
type RateProvider interface {
	GetExchangeRate(ctx context.Context, from, to string) (decimal.Decimal, error)
}

// StaticRateProvider resolves exchange rates from a fixed in-memory table.
type StaticRateProvider struct {
	rates map[string]decimal.Decimal
}

// NewConversionService creates a ConversionService.
func NewConversionService(commodityRegistry registry.CommodityRegistry, rateProvider RateProvider) *ConversionService {
	return &ConversionService{commodityRegistry: commodityRegistry, rateProvider: rateProvider}
}

// NewStaticRateProvider creates a StaticRateProvider.
func NewStaticRateProvider(rates map[string]decimal.Decimal) *StaticRateProvider {
	return &StaticRateProvider{rates: rates}
}

// NewDefaultRateProvider creates a StaticRateProvider with the built-in fallback rates.
func NewDefaultRateProvider() *StaticRateProvider {
	return NewStaticRateProvider(defaultRates)
}

// ConvertCommodityPrices converts all commodity prices using the configured rate provider.
func (s *ConversionService) ConvertCommodityPrices(ctx context.Context, fromCurrency, toCurrency string) error {
	return s.ConvertCommodityPricesWithRate(ctx, fromCurrency, toCurrency, nil)
}

// ConvertCommodityPricesWithRate converts all commodity prices using the provided rate when present.
func (s *ConversionService) ConvertCommodityPricesWithRate(ctx context.Context, fromCurrency, toCurrency string, rate *decimal.Decimal) error {
	if fromCurrency == toCurrency {
		return nil
	}

	exchangeRate, err := s.exchangeRate(ctx, fromCurrency, toCurrency, rate)
	if err != nil {
		return err
	}

	commodities, err := s.commodityRegistry.List(ctx)
	if err != nil {
		return fmt.Errorf("list commodities: %w", err)
	}

	originals := make([]models.Commodity, 0, len(commodities))

	for _, commodity := range commodities {
		original := *commodity

		if !applyExchangeRate(commodity, fromCurrency, toCurrency, exchangeRate) {
			continue
		}

		if _, err := s.commodityRegistry.Update(ctx, *commodity); err != nil {
			rollbackErr := s.rollbackCommodityUpdates(ctx, originals)
			if rollbackErr != nil {
				return fmt.Errorf("update commodity %s: %w (rollback failed: %v)", commodity.ID, err, rollbackErr)
			}

			return fmt.Errorf("update commodity %s: %w", commodity.ID, err)
		}

		originals = append(originals, original)
	}

	return nil
}

// GetExchangeRate returns the configured exchange rate for a currency pair.
func (p *StaticRateProvider) GetExchangeRate(_ context.Context, from, to string) (decimal.Decimal, error) {
	if from == to {
		return decimal.NewFromInt(1), nil
	}

	rate, ok := p.rates[fmt.Sprintf("%s_%s", from, to)]
	if !ok {
		return decimal.Zero, fmt.Errorf("%w: %s to %s", ErrExchangeRateRequired, from, to)
	}

	return rate, nil
}

func (s *ConversionService) exchangeRate(ctx context.Context, fromCurrency, toCurrency string, rate *decimal.Decimal) (decimal.Decimal, error) {
	if rate != nil {
		if !rate.GreaterThan(decimal.Zero) {
			return decimal.Zero, ErrInvalidExchangeRate
		}

		return *rate, nil
	}

	if s.rateProvider == nil {
		return decimal.Zero, fmt.Errorf("%w: %s to %s", ErrExchangeRateRequired, fromCurrency, toCurrency)
	}

	return s.rateProvider.GetExchangeRate(ctx, fromCurrency, toCurrency)
}

func applyExchangeRate(commodity *models.Commodity, fromCurrency, toCurrency string, exchangeRate decimal.Decimal) bool {
	updated := false

	if !commodity.ConvertedOriginalPrice.IsZero() {
		commodity.ConvertedOriginalPrice = quantizeConvertedMoney(commodity.ConvertedOriginalPrice.Mul(exchangeRate))
		updated = true
	}

	if !commodity.CurrentPrice.IsZero() {
		commodity.CurrentPrice = quantizeConvertedMoney(commodity.CurrentPrice.Mul(exchangeRate))
		updated = true
	}

	if string(commodity.OriginalPriceCurrency) == fromCurrency {
		commodity.OriginalPriceCurrency = models.Currency(toCurrency)
		commodity.OriginalPrice = quantizeConvertedMoney(commodity.OriginalPrice.Mul(exchangeRate))
		commodity.ConvertedOriginalPrice = decimal.Zero
		updated = true
	}

	return updated
}

func (s *ConversionService) rollbackCommodityUpdates(ctx context.Context, originals []models.Commodity) error {
	var rollbackErr error

	for i := len(originals) - 1; i >= 0; i-- {
		if _, err := s.commodityRegistry.Update(ctx, originals[i]); err != nil {
			rollbackErr = errors.Join(rollbackErr, fmt.Errorf("rollback commodity %s: %w", originals[i].ID, err))
		}
	}

	return rollbackErr
}

func quantizeConvertedMoney(amount decimal.Decimal) decimal.Decimal {
	return amount.Round(convertedMoneyScale)
}
