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

func NewFactorySet() *registry.FactorySet {
	// Create factory instances that will create context-aware registries
	locationFactory := NewLocationRegistry()
	areaFactory := NewAreaRegistry(locationFactory)
	settingsFactory := NewSettingsRegistry()
	fileFactory := NewFileRegistry()
	commodityFactory := NewCommodityRegistry(areaFactory)
	imageFactory := NewImageRegistry(commodityFactory)
	invoiceFactory := NewInvoiceRegistry(commodityFactory)
	manualFactory := NewManualRegistry(commodityFactory)
	exportFactory := NewExportRegistry()
	restoreStepFactory := NewRestoreStepRegistry()
	restoreOperationFactory := NewRestoreOperationRegistry(restoreStepFactory)

	fs := &registry.FactorySet{}
	fs.LocationRegistryFactory = locationFactory
	fs.AreaRegistryFactory = areaFactory
	fs.SettingsRegistryFactory = settingsFactory
	fs.FileRegistryFactory = fileFactory
	fs.CommodityRegistryFactory = commodityFactory
	fs.ImageRegistryFactory = imageFactory
	fs.InvoiceRegistryFactory = invoiceFactory
	fs.ManualRegistryFactory = manualFactory
	fs.ExportRegistryFactory = exportFactory
	fs.RestoreStepRegistryFactory = restoreStepFactory
	fs.RestoreOperationRegistryFactory = restoreOperationFactory
	fs.TenantRegistry = NewTenantRegistry()
	fs.UserRegistry = NewUserRegistry()

	return fs
}

func NewRegistrySetWithUserID(userID string) *registry.Set {
	ctx := appctx.WithUser(context.Background(), &models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: userID},
		},
	})

	fs := NewFactorySet()
	s, err := fs.CreateUserRegistrySet(ctx)
	if err != nil {
		panic(err) // This maintains the same behavior as the original must.Must calls
	}

	// Create the test user and tenant
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

func NewMemoryRegistrySet() (func(registry.Config) (*registry.FactorySet, error), func() error) {
	newFn := func(_ registry.Config) (*registry.FactorySet, error) {
		factorySet := NewFactorySet()
		return factorySet, nil
	}

	doCleanup := func() error {
		// Memory registry doesn't need cleanup
		return nil
	}

	return newFn, doCleanup
}
