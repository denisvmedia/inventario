package memory

import (
	"context"
	"sync"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)

type SettingsRegistry struct {
	settings map[string]models.SettingsObject // userID -> settings
	lock     sync.RWMutex
	userID   string
}

func NewSettingsRegistry() *SettingsRegistry {
	return &SettingsRegistry{
		settings: make(map[string]models.SettingsObject),
	}
}

func (r *SettingsRegistry) MustWithCurrentUser(ctx context.Context) registry.SettingsRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *SettingsRegistry) WithCurrentUser(ctx context.Context) (registry.SettingsRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}

	// Create a new registry with the same data but different userID
	tmp := &SettingsRegistry{
		settings: r.settings,
		userID:   user.ID,
	}

	return tmp, nil
}

func (r *SettingsRegistry) WithServiceAccount() registry.SettingsRegistry {
	// For memory registries, service account access is the same as regular access
	// since memory registries don't enforce RLS restrictions
	return r
}

func (r *SettingsRegistry) Get(_ context.Context) (models.SettingsObject, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	if settings, exists := r.settings[r.userID]; exists {
		return settings, nil
	}

	// Return empty settings for new users
	return models.SettingsObject{}, nil
}

func (r *SettingsRegistry) Save(_ context.Context, settings models.SettingsObject) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.settings[r.userID] = settings
	return nil
}

func (r *SettingsRegistry) Patch(_ctx context.Context, configfield string, value any) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	// Get current settings for the user
	currentSettings := r.settings[r.userID]

	err := typekit.SetFieldByConfigfieldTag(&currentSettings, configfield, value)
	if err != nil {
		return errkit.Wrap(err, "failed to patch settings")
	}

	// Save updated settings
	r.settings[r.userID] = currentSettings
	return nil
}
