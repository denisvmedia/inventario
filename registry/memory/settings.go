package memory

import (
	"sync"

	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/models"
)

type SettingsRegistry struct {
	settings models.SettingsObject
	lock     sync.RWMutex
}

func NewSettingsRegistry() *SettingsRegistry {
	return &SettingsRegistry{
		settings: models.SettingsObject{},
	}
}

func (r *SettingsRegistry) Get() (models.SettingsObject, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.settings, nil
}

func (r *SettingsRegistry) Save(settings models.SettingsObject) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.settings = settings
	return nil
}

func (r *SettingsRegistry) Patch(configfield string, value any) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	err := typekit.SetFieldByConfigfieldTag(&r.settings, configfield, value)
	if err != nil{
		return err
	}

	return nil
}
