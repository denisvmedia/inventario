package memory

import (
	"encoding/json"
	"sync"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const (
	settingIDUIConfig     = "ui_config"
	settingIDSystemConfig = "system_config"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

type SettingsRegistry struct {
	registry *Registry[models.Setting, *models.Setting]
	lock     sync.RWMutex
}

func NewSettingsRegistry() *SettingsRegistry {
	return &SettingsRegistry{
		registry: NewRegistry[models.Setting, *models.Setting](),
	}
}

// TLS methods removed as requested

func (r *SettingsRegistry) GetUIConfig() (*models.UIConfig, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	setting, err := r.registry.Get(settingIDUIConfig)
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
	r.lock.Lock()
	defer r.lock.Unlock()

	value, err := json.Marshal(config)
	if err != nil {
		return errkit.Wrap(err, "failed to marshal UI config")
	}

	setting := models.Setting{
		ID:    settingIDUIConfig,
		Value: value,
	}

	// Try to get the setting first
	_, err = r.registry.Get(settingIDUIConfig)
	if err != nil {
		// If not found, create it
		if err == registry.ErrNotFound {
			_, err = r.registry.Create(setting)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// If found, update it
		_, err = r.registry.Update(setting)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *SettingsRegistry) RemoveUIConfig() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.registry.Delete(settingIDUIConfig)
}

func (r *SettingsRegistry) GetSystemConfig() (*models.SystemConfig, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	setting, err := r.registry.Get(settingIDSystemConfig)
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
	r.lock.Lock()
	defer r.lock.Unlock()

	value, err := json.Marshal(config)
	if err != nil {
		return errkit.Wrap(err, "failed to marshal system config")
	}

	setting := models.Setting{
		ID:    settingIDSystemConfig,
		Value: value,
	}

	// Try to get the setting first
	_, err = r.registry.Get(settingIDSystemConfig)
	if err != nil {
		// If not found, create it
		if err == registry.ErrNotFound {
			_, err = r.registry.Create(setting)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	} else {
		// If found, update it
		_, err = r.registry.Update(setting)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *SettingsRegistry) RemoveSystemConfig() error {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.registry.Delete(settingIDSystemConfig)
}

// Currency methods removed as requested

// Registry interface implementation
func (r *SettingsRegistry) Create(m models.Setting) (*models.Setting, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.registry.Create(m)
}

func (r *SettingsRegistry) Get(id string) (*models.Setting, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.registry.Get(id)
}

func (r *SettingsRegistry) List() ([]*models.Setting, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.registry.List()
}

func (r *SettingsRegistry) Update(m models.Setting) (*models.Setting, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.registry.Update(m)
}

func (r *SettingsRegistry) Delete(id string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	return r.registry.Delete(id)
}

func (r *SettingsRegistry) Count() (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.registry.Count()
}
