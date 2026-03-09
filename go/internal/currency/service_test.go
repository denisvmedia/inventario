package currency_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/currency"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

func TestConversionService_ConvertCommodityPricesWithRate_RoundsConvertedValues(t *testing.T) {
	c := qt.New(t)

	commodityRegistry := newStubCommodityRegistry(
		models.Commodity{TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "usd-item"}}, OriginalPrice: decimal.RequireFromString("10"), OriginalPriceCurrency: models.Currency("USD"), CurrentPrice: decimal.RequireFromString("5")},
		models.Commodity{TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "gbp-item"}}, OriginalPrice: decimal.RequireFromString("3"), OriginalPriceCurrency: models.Currency("GBP"), ConvertedOriginalPrice: decimal.RequireFromString("8")},
	)
	rate := decimal.RequireFromString("1.23456")

	service := currency.NewConversionService(commodityRegistry, nil)
	err := service.ConvertCommodityPricesWithRate(context.Background(), "USD", "EUR", &rate)
	c.Assert(err, qt.IsNil)

	usdItem, err := commodityRegistry.Get(context.Background(), "usd-item")
	c.Assert(err, qt.IsNil)
	c.Assert(usdItem.OriginalPrice.Equal(decimal.RequireFromString("12.35")), qt.IsTrue)
	c.Assert(usdItem.OriginalPriceCurrency, qt.Equals, models.Currency("EUR"))
	c.Assert(usdItem.CurrentPrice.Equal(decimal.RequireFromString("6.17")), qt.IsTrue)

	gbpItem, err := commodityRegistry.Get(context.Background(), "gbp-item")
	c.Assert(err, qt.IsNil)
	c.Assert(gbpItem.OriginalPrice.Equal(decimal.RequireFromString("3")), qt.IsTrue)
	c.Assert(gbpItem.ConvertedOriginalPrice.Equal(decimal.RequireFromString("9.88")), qt.IsTrue)
}

func TestConversionService_ConvertCommodityPricesWithRate_RollsBackOnUpdateFailure(t *testing.T) {
	c := qt.New(t)

	commodityRegistry := newStubCommodityRegistry(
		models.Commodity{TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "first"}}, OriginalPrice: decimal.RequireFromString("10"), OriginalPriceCurrency: models.Currency("USD"), CurrentPrice: decimal.RequireFromString("3")},
		models.Commodity{TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: "second"}}, OriginalPrice: decimal.RequireFromString("20"), OriginalPriceCurrency: models.Currency("USD"), CurrentPrice: decimal.RequireFromString("4")},
	)
	commodityRegistry.failUpdates["second"] = 1
	rate := decimal.RequireFromString("2")

	service := currency.NewConversionService(commodityRegistry, nil)
	err := service.ConvertCommodityPricesWithRate(context.Background(), "USD", "EUR", &rate)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "update commodity second")

	first, err := commodityRegistry.Get(context.Background(), "first")
	c.Assert(err, qt.IsNil)
	c.Assert(first.OriginalPrice.Equal(decimal.RequireFromString("10")), qt.IsTrue)
	c.Assert(first.OriginalPriceCurrency, qt.Equals, models.Currency("USD"))
	c.Assert(first.CurrentPrice.Equal(decimal.RequireFromString("3")), qt.IsTrue)

	second, err := commodityRegistry.Get(context.Background(), "second")
	c.Assert(err, qt.IsNil)
	c.Assert(second.OriginalPrice.Equal(decimal.RequireFromString("20")), qt.IsTrue)
	c.Assert(second.OriginalPriceCurrency, qt.Equals, models.Currency("USD"))
	c.Assert(second.CurrentPrice.Equal(decimal.RequireFromString("4")), qt.IsTrue)
}

type stubCommodityRegistry struct {
	commodities map[string]models.Commodity
	order       []string
	failUpdates map[string]int
}

func newStubCommodityRegistry(commodities ...models.Commodity) *stubCommodityRegistry {
	stub := &stubCommodityRegistry{commodities: make(map[string]models.Commodity, len(commodities)), order: make([]string, 0, len(commodities)), failUpdates: map[string]int{}}
	for _, commodity := range commodities {
		stub.commodities[commodity.ID] = commodity
		stub.order = append(stub.order, commodity.ID)
	}
	return stub
}

func (r *stubCommodityRegistry) Create(context.Context, models.Commodity) (*models.Commodity, error) {
	return nil, errors.New("unexpected Create call")
}
func (r *stubCommodityRegistry) Delete(context.Context, string) error {
	return errors.New("unexpected Delete call")
}
func (r *stubCommodityRegistry) Count(context.Context) (int, error)                  { return len(r.commodities), nil }
func (r *stubCommodityRegistry) GetImages(context.Context, string) ([]string, error) { return nil, nil }
func (r *stubCommodityRegistry) GetManuals(context.Context, string) ([]string, error) {
	return nil, nil
}
func (r *stubCommodityRegistry) GetInvoices(context.Context, string) ([]string, error) {
	return nil, nil
}
func (r *stubCommodityRegistry) ListPaginated(context.Context, int, int) ([]*models.Commodity, int, error) {
	return nil, 0, errors.New("unexpected ListPaginated call")
}

func (r *stubCommodityRegistry) Get(_ context.Context, id string) (*models.Commodity, error) {
	commodity, ok := r.commodities[id]
	if !ok {
		return nil, registry.ErrNotFound
	}
	return &commodity, nil
}

func (r *stubCommodityRegistry) List(context.Context) ([]*models.Commodity, error) {
	commodities := make([]*models.Commodity, 0, len(r.order))
	for _, id := range r.order {
		commodity := r.commodities[id]
		commodities = append(commodities, &commodity)
	}
	return commodities, nil
}

func (r *stubCommodityRegistry) Update(_ context.Context, commodity models.Commodity) (*models.Commodity, error) {
	if remainingFailures := r.failUpdates[commodity.ID]; remainingFailures > 0 {
		r.failUpdates[commodity.ID] = remainingFailures - 1
		return nil, fmt.Errorf("forced update failure for %s", commodity.ID)
	}
	r.commodities[commodity.ID] = commodity
	return &commodity, nil
}
