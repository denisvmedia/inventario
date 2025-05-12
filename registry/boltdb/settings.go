package boltdb

import (
	"encoding/json"
	"errors"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameSetting = "setting"

	bucketNameSettings = "settings"

	idxSettingsByName = "settings_by_name"

	settingIDUIConfig     = "ui_config"
	settingIDSystemConfig = "system_config"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

type SettingsRegistry struct {
	db       *bolt.DB
	base     *dbx.BaseRepository[models.Setting, *models.Setting]
	registry *Registry[models.Setting, *models.Setting]
}

func NewSettingsRegistry(db *bolt.DB) *SettingsRegistry {
	base := dbx.NewBaseRepository[models.Setting, *models.Setting](bucketNameSettings)
	registry := NewRegistry[models.Setting, *models.Setting](
		db,
		base,
		entityNameSetting,
		"",
	)

	return &SettingsRegistry{
		db:       db,
		base:     base,
		registry: registry,
	}
}

func (r *SettingsRegistry) Create(m models.Setting) (*models.Setting, error) {
	return r.registry.Create(m, func(tx dbx.TransactionOrBucket, setting *models.Setting) error {
		if setting.ID == "" {
			return errkit.WithStack(registry.ErrFieldRequired,
				"field_name", "ID",
			)
		}

		if setting.Name != "" {
			// Check if a setting with this name already exists
			_, err := r.base.GetIndexValue(tx, idxSettingsByName, setting.Name)
			if err == nil {
				// Setting with this name already exists, update it instead
				return errkit.Wrap(registry.ErrAlreadyExists, "setting name is already used")
			}
			if !errors.Is(err, registry.ErrNotFound) {
				// Any other error is a problem
				return err
			}
		}

		return nil
	}, func(tx dbx.TransactionOrBucket, setting *models.Setting) error {
		if setting.Name != "" {
			// Save the name index
			err := r.base.SaveIndexValue(tx, idxSettingsByName, setting.Name, setting.ID)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *SettingsRegistry) Get(id string) (*models.Setting, error) {
	return r.registry.Get(id)
}

func (r *SettingsRegistry) GetByName(name string) (*models.Setting, error) {
	return r.registry.GetBy(idxSettingsByName, name)
}

func (r *SettingsRegistry) List() ([]*models.Setting, error) {
	return r.registry.List()
}

func (r *SettingsRegistry) Update(m models.Setting) (*models.Setting, error) {
	var old *models.Setting
	return r.registry.Update(m, func(tx dbx.TransactionOrBucket, setting *models.Setting) error {
		old = setting
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Setting) error {
		if old.Name == result.Name {
			return nil
		}

		if result.Name != "" {
			// Check if a setting with this name already exists
			u := &models.Setting{}
			err := r.base.GetByIndexValue(tx, idxSettingsByName, result.Name, u)
			switch {
			case err == nil:
				return errkit.Wrap(registry.ErrAlreadyExists, "setting name is already used")
			case errors.Is(err, registry.ErrNotFound):
				// skip, it's expected
			case err != nil:
				return errkit.Wrap(err, "failed to check if setting name is already used")
			}

			// Remove the old name from the index
			if old.Name != "" {
				err = r.base.DeleteIndexValue(tx, idxSettingsByName, old.Name)
				if err != nil {
					return err
				}
			}

			// Save the new name in the index
			err = r.base.SaveIndexValue(tx, idxSettingsByName, result.Name, result.GetID())
			if err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *SettingsRegistry) Delete(id string) error {
	return r.registry.Delete(id, func(tx dbx.TransactionOrBucket, setting *models.Setting) error {
		return nil
	}, func(tx dbx.TransactionOrBucket, setting *models.Setting) error {
		if setting.Name != "" {
			return r.base.DeleteIndexValue(tx, idxSettingsByName, setting.Name)
		}
		return nil
	})
}

func (r *SettingsRegistry) Count() (int, error) {
	return r.registry.Count()
}

func (r *SettingsRegistry) DeleteByName(name string) error {
	setting, err := r.GetByName(name)
	if err != nil {
		return err
	}
	return r.Delete(setting.ID)
}

// TLS methods removed as requested

func (r *SettingsRegistry) GetUIConfig() (*models.UIConfig, error) {
	setting, err := r.GetByName(settingIDUIConfig)
	if err != nil {
		return nil, err
	}

	var config models.UIConfig
	if err := json.Unmarshal(setting.Value, &config); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal UI config")
	}

	return &config, nil
}

func (r *SettingsRegistry) SetUIConfig(config *models.UIConfig) error {
	value, err := json.Marshal(config)
	if err != nil {
		return errkit.Wrap(err, "failed to marshal UI config")
	}

	setting := models.Setting{
		Name:  settingIDUIConfig,
		Value: value,
	}

	// Try to get the setting by name first
	existingSetting, err := r.GetByName(settingIDUIConfig)
	if err != nil {
		// If not found, create it
		if errors.Is(err, registry.ErrNotFound) {
			_, err = r.Create(setting)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// If found, update it
		setting.ID = existingSetting.ID
		_, err = r.Update(setting)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *SettingsRegistry) RemoveUIConfig() error {
	return r.Delete(settingIDUIConfig)
}

func (r *SettingsRegistry) GetSystemConfig() (*models.SystemConfig, error) {
	setting, err := r.Get(settingIDSystemConfig)
	if err != nil {
		return nil, err
	}

	var config models.SystemConfig
	if err := json.Unmarshal(setting.Value, &config); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal system config")
	}

	return &config, nil
}

func (r *SettingsRegistry) SetSystemConfig(config *models.SystemConfig) error {
	value, err := json.Marshal(config)
	if err != nil {
		return errkit.Wrap(err, "failed to marshal system config")
	}

	setting := models.Setting{
		ID:    settingIDSystemConfig,
		Value: value,
	}

	_, err = r.Update(setting)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			_, err = r.Create(setting)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *SettingsRegistry) RemoveSystemConfig() error {
	return r.Delete(settingIDSystemConfig)
}

// Currency methods removed as requested
