package memory

import (
	"context"
	"errors"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// InvoiceRegistryFactory creates InvoiceRegistry instances with proper context
type InvoiceRegistryFactory struct {
	baseInvoiceRegistry *Registry[models.Invoice, *models.Invoice]
	commodityRegistry   *CommodityRegistryFactory // required dependency for relationship tracking
}

// InvoiceRegistry is a context-aware registry that can only be created through the factory
type InvoiceRegistry struct {
	*Registry[models.Invoice, *models.Invoice]

	userID            string
	commodityRegistry *CommodityRegistry // required dependency for relationship tracking
}

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)
var _ registry.InvoiceRegistryFactory = (*InvoiceRegistryFactory)(nil)

func NewInvoiceRegistryFactory(commodityRegistry *CommodityRegistryFactory) *InvoiceRegistryFactory {
	return &InvoiceRegistryFactory{
		baseInvoiceRegistry: NewRegistry[models.Invoice, *models.Invoice](),
		commodityRegistry:   commodityRegistry,
	}
}

// Factory methods implementing registry.InvoiceRegistryFactory

func (f *InvoiceRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.InvoiceRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *InvoiceRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.InvoiceRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}

	// Create a new registry with user context already set
	userRegistry := &Registry[models.Invoice, *models.Invoice]{
		items:  f.baseInvoiceRegistry.items, // Share the data map
		lock:   f.baseInvoiceRegistry.lock,  // Share the mutex pointer
		userID: user.ID,                     // Set user-specific userID
	}

	// Create user-aware commodity registry
	commodityRegistryInterface, err := f.commodityRegistry.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create user commodity registry")
	}

	// Cast to concrete type for relationship management
	commodityRegistry, ok := commodityRegistryInterface.(*CommodityRegistry)
	if !ok {
		return nil, errors.New("failed to cast commodity registry to concrete type")
	}

	return &InvoiceRegistry{
		Registry:          userRegistry,
		userID:            user.ID,
		commodityRegistry: commodityRegistry,
	}, nil
}

func (f *InvoiceRegistryFactory) CreateServiceRegistry() registry.InvoiceRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.Invoice, *models.Invoice]{
		items:  f.baseInvoiceRegistry.items, // Share the data map
		lock:   f.baseInvoiceRegistry.lock,  // Share the mutex pointer
		userID: "",                          // Clear userID to bypass user filtering
	}

	// Create service-aware commodity registry
	commodityRegistryInterface := f.commodityRegistry.CreateServiceRegistry()

	// Cast to concrete type for relationship management
	commodityRegistry, ok := commodityRegistryInterface.(*CommodityRegistry)
	if !ok {
		panic("commodityRegistryInterface is not of type *CommodityRegistry")
	}

	return &InvoiceRegistry{
		Registry:          serviceRegistry,
		userID:            "", // Clear userID to bypass user filtering
		commodityRegistry: commodityRegistry,
	}
}

func (r *InvoiceRegistry) Create(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Use CreateWithUser to ensure user context is applied
	newInvoice, err := r.Registry.CreateWithUser(ctx, invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create invoice")
	}

	// Add this invoice to its parent commodity's invoice list
	_ = r.commodityRegistry.AddInvoice(ctx, newInvoice.CommodityID, newInvoice.GetID())

	return newInvoice, nil
}

func (r *InvoiceRegistry) Update(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Get the existing invoice to check if CommodityID changed
	var oldCommodityID string
	if existingInvoice, err := r.Registry.Get(ctx, invoice.GetID()); err == nil {
		oldCommodityID = existingInvoice.CommodityID
	}

	// Call the base registry's UpdateWithUser method to ensure user context is preserved
	updatedInvoice, err := r.Registry.UpdateWithUser(ctx, invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update invoice")
	}

	// Handle commodity registry tracking - commodity changed
	if oldCommodityID != "" && oldCommodityID != updatedInvoice.CommodityID {
		// Remove from old commodity
		_ = r.commodityRegistry.DeleteInvoice(ctx, oldCommodityID, updatedInvoice.GetID())
		// Add to new commodity
		_ = r.commodityRegistry.AddInvoice(ctx, updatedInvoice.CommodityID, updatedInvoice.GetID())
	} else if oldCommodityID == "" {
		// This is a fallback case - add to commodity if not already tracked
		_ = r.commodityRegistry.AddInvoice(ctx, updatedInvoice.CommodityID, updatedInvoice.GetID())
	}

	return updatedInvoice, nil
}

func (r *InvoiceRegistry) Delete(ctx context.Context, id string) error {
	// Remove this invoice from its parent commodity's invoice list
	invoice, err := r.Registry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoice")
	}

	_ = r.commodityRegistry.DeleteInvoice(ctx, invoice.CommodityID, id)

	err = r.Registry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice")
	}

	err = r.commodityRegistry.DeleteInvoice(ctx, invoice.CommodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice from commodity")
	}

	return nil
}
