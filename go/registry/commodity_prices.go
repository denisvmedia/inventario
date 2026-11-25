package registry

import (
	"context"

	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
)

// NormalizeCommodityPrices applies models.Commodity.NormalizePrices with the
// group currency the context carries. The API handlers and the restore put
// it there; a write without it keeps the prices as given.
func NormalizeCommodityPrices(ctx context.Context, commodity *models.Commodity) {
	if groupCurrency, err := validationctx.GroupCurrencyFromContext(ctx); err == nil {
		commodity.NormalizePrices(models.Currency(groupCurrency))
	}
}
