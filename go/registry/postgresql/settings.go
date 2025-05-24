package postgresql

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

const settingsID = "settings"

type SettingsRegistry struct {
	pool *pgxpool.Pool
}

func NewSettingsRegistry(pool *pgxpool.Pool) *SettingsRegistry {
	return &SettingsRegistry{
		pool: pool,
	}
}

func (r *SettingsRegistry) Get() (models.SettingsObject, error) {
	ctx := context.Background()
	var settings models.SettingsObject
	var data []byte

	// Query the database for the settings
	err := r.pool.QueryRow(ctx, `
		SELECT data
		FROM settings
		WHERE id = $1
	`, settingsID).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			// If no settings exist, return an empty settings object
			return models.SettingsObject{}, nil
		}
		return models.SettingsObject{}, errkit.Wrap(err, "failed to get settings")
	}

	// Unmarshal the JSON data
	if err := json.Unmarshal(data, &settings); err != nil {
		return models.SettingsObject{}, errkit.Wrap(err, "failed to unmarshal settings")
	}

	return settings, nil
}

func (r *SettingsRegistry) Save(settings models.SettingsObject) error {
	ctx := context.Background()

	// Check if the main currency is being changed
	currentSettings, err := r.Get()
	if err != nil {
		return err
	}

	// If the main currency is already set and is being changed, return an error
	if currentSettings.MainCurrency != nil && settings.MainCurrency != nil &&
		*currentSettings.MainCurrency != "" && *settings.MainCurrency != *currentSettings.MainCurrency {
		return errkit.Wrap(registry.ErrMainCurrencyAlreadySet, "main currency already set")
	}

	// Marshal the settings to JSON
	data, err := json.Marshal(settings)
	if err != nil {
		return errkit.Wrap(err, "failed to marshal settings")
	}

	// Upsert the settings in the database
	_, err = r.pool.Exec(ctx, `
		INSERT INTO settings (id, data)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE
		SET data = $2
	`, settingsID, data)
	if err != nil {
		return errkit.Wrap(err, "failed to save settings")
	}

	return nil
}

func (r *SettingsRegistry) Patch(configfield string, value any) error {
	ctx := context.Background()

	// Get the current settings
	settings, err := r.Get()
	if err != nil {
		return err
	}

	// If the main currency is already set and is being changed, return an error
	if configfield == "system.main_currency" && settings.MainCurrency != nil && *settings.MainCurrency != "" {
		newValue, ok := value.(string)
		if !ok {
			return errkit.Wrap(registry.ErrInvalidConfig, "invalid main currency value")
		}
		if newValue != *settings.MainCurrency {
			return errkit.Wrap(registry.ErrMainCurrencyAlreadySet, "main currency already set")
		}
	}

	// Update the field
	err = typekit.SetFieldByConfigfieldTag(&settings, configfield, value)
	if err != nil {
		return errkit.Wrap(err, "failed to patch settings")
	}

	// Marshal the settings to JSON
	data, err := json.Marshal(settings)
	if err != nil {
		return errkit.Wrap(err, "failed to marshal settings")
	}

	// Update the settings in the database
	_, err = r.pool.Exec(ctx, `
		UPDATE settings
		SET data = $1
		WHERE id = $2
	`, data, settingsID)
	if err != nil {
		return errkit.Wrap(err, "failed to update settings")
	}

	return nil
}
