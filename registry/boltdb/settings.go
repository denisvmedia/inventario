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
		return nil
	}, NoopHook[models.Setting, *models.Setting])
}

func (r *SettingsRegistry) Get(id string) (*models.Setting, error) {
	return r.registry.Get(id)
}

func (r *SettingsRegistry) List() ([]*models.Setting, error) {
	return r.registry.List()
}

func (r *SettingsRegistry) Update(m models.Setting) (*models.Setting, error) {
	return r.registry.Update(m, NoopHook[models.Setting, *models.Setting], NoopHook[models.Setting, *models.Setting])
}

func (r *SettingsRegistry) Delete(id string) error {
	return r.registry.Delete(id)
}

func (r *SettingsRegistry) Count() (int, error) {
	return r.registry.Count()
}

// TLS methods removed as requested

func (r *SettingsRegistry) GetUIConfig() (*models.UIConfig, error) {
	setting, err := r.Get(settingIDUIConfig)
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
		ID:    settingIDUIConfig,
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
