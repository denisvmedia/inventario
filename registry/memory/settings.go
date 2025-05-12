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
	nameIdx  map[string]string // map[name]id
	lock     sync.RWMutex
}

func NewSettingsRegistry() *SettingsRegistry {
	return &SettingsRegistry{
		registry: NewRegistry[models.Setting, *models.Setting](),
		nameIdx:  make(map[string]string),
	}
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
		if err == registry.ErrNotFound {
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
	return r.DeleteByName(settingIDUIConfig)
}

func (r *SettingsRegistry) GetSystemConfig() (*models.SystemConfig, error) {
	setting, err := r.GetByName(settingIDSystemConfig)
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
		Name:  settingIDSystemConfig,
		Value: value,
	}

	// Try to get the setting by name first
	existingSetting, err := r.GetByName(settingIDSystemConfig)
	if err != nil {
		// If not found, create it
		if err == registry.ErrNotFound {
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

func (r *SettingsRegistry) RemoveSystemConfig() error {
	return r.DeleteByName(settingIDSystemConfig)
}

// Registry interface implementation
func (r *SettingsRegistry) Create(m models.Setting) (*models.Setting, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Check if a setting with this name already exists
	if m.Name != "" {
		if id, ok := r.nameIdx[m.Name]; ok {
			// Setting with this name already exists, update it
			m.ID = id
			return r.registry.Update(m)
		}
	}

	// Create a new setting
	setting, err := r.registry.Create(m)
	if err != nil {
		return nil, err
	}

	// Update the name index
	if setting.Name != "" {
		r.nameIdx[setting.Name] = setting.ID
	}

	return setting, nil
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

	// Get the existing setting to check if the name has changed
	oldSetting, err := r.registry.Get(m.ID)
	if err != nil {
		return nil, err
	}

	// Update the setting
	setting, err := r.registry.Update(m)
	if err != nil {
		return nil, err
	}

	// Update the name index
	if oldSetting.Name != "" && oldSetting.Name != setting.Name {
		// Remove the old name from the index
		delete(r.nameIdx, oldSetting.Name)
	}
	if setting.Name != "" {
		r.nameIdx[setting.Name] = setting.ID
	}

	return setting, nil
}

func (r *SettingsRegistry) Delete(id string) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Get the setting to find its name
	setting, err := r.registry.Get(id)
	if err != nil {
		return err
	}

	// Remove the setting from the name index
	if setting.Name != "" {
		delete(r.nameIdx, setting.Name)
	}

	return r.registry.Delete(id)
}

func (r *SettingsRegistry) Count() (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.registry.Count()
}

func (r *SettingsRegistry) GetByName(name string) (*models.Setting, error) {
	r.lock.RLock()
	id, ok := r.nameIdx[name]
	r.lock.RUnlock()

	if !ok {
		return nil, registry.ErrNotFound
	}

	return r.Get(id)
}

func (r *SettingsRegistry) DeleteByName(name string) error {
	r.lock.Lock()
	id, ok := r.nameIdx[name]
	if ok {
		delete(r.nameIdx, name)
	}
	r.lock.Unlock()

	if !ok {
		return registry.ErrNotFound
	}

	return r.Delete(id)
}
