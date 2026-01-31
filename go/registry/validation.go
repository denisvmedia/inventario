package registry

import (
	"context"

	"github.com/go-extras/errx/stacktrace"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
)

// ValidateCommodity validates a given commodity using the application's settings.
// It retrieves the main currency from the provided SettingsRegistry and ensures it is set.
// The function then creates a validation context with the main currency and performs
// standard validation on the commodity using the validation library.
// Returns an error if the settings cannot be retrieved, the main currency is not set,
// or if the commodity fails validation.
func ValidateCommodity(commodity *models.Commodity, settingsRegistry SettingsRegistry) error {
	// Get main currency from settings
	ctx := context.Background()
	settings, err := settingsRegistry.Get(ctx)
	if err != nil {
		return stacktrace.Wrap("failed to get settings", err)
	}

	if settings.MainCurrency == nil || *settings.MainCurrency == "" {
		return ErrMainCurrencyNotSet
	}

	mainCurrency := *settings.MainCurrency

	// First, validate the commodity using standard validation
	ctx = validationctx.WithMainCurrency(ctx, mainCurrency)
	err = validation.ValidateWithContext(ctx, commodity)

	return err
}
