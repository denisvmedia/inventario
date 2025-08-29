package memory

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

const Name = "memory"

func Register() (cleanup func() error) {
	newFn, cleanup := NewMemoryRegistrySet()
	registry.Register(Name, newFn)
	return cleanup
}

func NewRegistrySet() *registry.Set {
	s := &registry.Set{}
	s.LocationRegistry = NewLocationRegistry()
	s.AreaRegistry = NewAreaRegistry(s.LocationRegistry.(*LocationRegistry))
	s.SettingsRegistry = NewSettingsRegistry()
	s.FileRegistry = NewFileRegistry()
	s.CommodityRegistry = NewCommodityRegistry(s.AreaRegistry.(*AreaRegistry))
	s.ImageRegistry = NewImageRegistry(s.CommodityRegistry.(*CommodityRegistry))
	s.InvoiceRegistry = NewInvoiceRegistry(s.CommodityRegistry.(*CommodityRegistry))
	s.ManualRegistry = NewManualRegistry(s.CommodityRegistry.(*CommodityRegistry))
	s.ExportRegistry = NewExportRegistry()
	s.RestoreStepRegistry = NewRestoreStepRegistry()
	s.RestoreOperationRegistry = NewRestoreOperationRegistry(s.RestoreStepRegistry)
	s.TenantRegistry = NewTenantRegistry()
	s.UserRegistry = NewUserRegistry()

	return s
}

func NewRegistrySetWithUserID(userID string) *registry.Set {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
		},
	})

	s := &registry.Set{}
	s.LocationRegistry = must.Must(NewLocationRegistry().WithCurrentUser(ctx))
	s.AreaRegistry = must.Must(NewAreaRegistry(s.LocationRegistry.(*LocationRegistry)).WithCurrentUser(ctx))
	s.SettingsRegistry = must.Must(NewSettingsRegistry().WithCurrentUser(ctx))
	s.FileRegistry = must.Must(NewFileRegistry().WithCurrentUser(ctx))
	s.CommodityRegistry = must.Must(NewCommodityRegistry(s.AreaRegistry.(*AreaRegistry)).WithCurrentUser(ctx))
	s.ImageRegistry = must.Must(NewImageRegistry(s.CommodityRegistry.(*CommodityRegistry)).WithCurrentUser(ctx))
	s.InvoiceRegistry = must.Must(NewInvoiceRegistry(s.CommodityRegistry.(*CommodityRegistry)).WithCurrentUser(ctx))
	s.ManualRegistry = must.Must(NewManualRegistry(s.CommodityRegistry.(*CommodityRegistry)).WithCurrentUser(ctx))
	s.ExportRegistry = must.Must(NewExportRegistry().WithCurrentUser(ctx))
	s.RestoreStepRegistry = must.Must(NewRestoreStepRegistry().WithCurrentUser(ctx))
	s.RestoreOperationRegistry = must.Must(NewRestoreOperationRegistry(s.RestoreStepRegistry).WithCurrentUser(ctx))
	s.TenantRegistry = NewTenantRegistry()
	s.UserRegistry = NewUserRegistry()
	must.Must(s.UserRegistry.(*UserRegistry).Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
		},
		Email:    "test@example.com",
		Name:     "Test User",
		Role:     models.UserRoleUser,
		IsActive: true,
	}))
	must.Must(s.TenantRegistry.Create(ctx, models.Tenant{
		EntityID: models.EntityID{ID: "test-tenant"},
		Name:     "Test Tenant",
	}))

	return s
}

func NewMemoryRegistrySet() (func(registry.Config) (*registry.Set, error), func() error) {
	newFn := func(_ registry.Config) (*registry.Set, error) {
		registrySet := NewRegistrySet()
		return registrySet, nil
	}

	doCleanup := func() error {
		// Memory registry doesn't need cleanup
		return nil
	}

	return newFn, doCleanup
}
