package memory

import (
	"context"
	"sync"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SettingsRegistryFactory creates SettingsRegistry instances with proper context
type SettingsRegistryFactory struct {
	settings map[string]models.SettingsObject // userID -> settings
	lock     *sync.RWMutex
}

// SettingsRegistry is a context-aware registry that can only be created through the factory
type SettingsRegistry struct {
	settings map[string]models.SettingsObject // userID -> settings
	lock     *sync.RWMutex
	userID   string
}

var _ registry.SettingsRegistry = (*SettingsRegistry)(nil)
var _ registry.SettingsRegistryFactory = (*SettingsRegistryFactory)(nil)

func NewSettingsRegistryFactory() *SettingsRegistryFactory {
	return &SettingsRegistryFactory{
		settings: make(map[string]models.SettingsObject),
		lock:     &sync.RWMutex{},
	}
}

// Factory methods implementing registry.SettingsRegistryFactory

func (f *SettingsRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SettingsRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SettingsRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SettingsRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	// Create a new registry with the same data but different userID
	return &SettingsRegistry{
		settings: f.settings,
		lock:     f.lock,
		userID:   user.ID,
	}, nil
}

func (f *SettingsRegistryFactory) CreateServiceRegistry() registry.SettingsRegistry {
	// Create a new registry with the same data but no user filtering
	return &SettingsRegistry{
		settings: f.settings,
		lock:     f.lock,
		userID:   "", // Clear userID to bypass user filtering
	}
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
		return errxtrace.Wrap("failed to patch settings", err)
	}

	// Save updated settings
	r.settings[r.userID] = currentSettings
	return nil
}
