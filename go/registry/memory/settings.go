package memory

import (
	"context"
	"sync"

	"github.com/denisvmedia/inventario/internal/errkit"
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

func (r *SettingsRegistry) Get(ctx context.Context) (models.SettingsObject, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	return r.settings, nil
}

func (r *SettingsRegistry) Save(ctx context.Context, settings models.SettingsObject) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.settings = settings
	return nil
}

func (r *SettingsRegistry) Patch(ctx context.Context, configfield string, value any) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	err := typekit.SetFieldByConfigfieldTag(&r.settings, configfield, value)
	if err != nil {
		return errkit.Wrap(err, "failed to patch settings")
	}

	return nil
}
