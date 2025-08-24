package memory

import (
	"context"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)

type baseInvoiceRegistry = Registry[models.Invoice, *models.Invoice]
type InvoiceRegistry struct {
	*baseInvoiceRegistry

	userID            string
	commodityRegistry *CommodityRegistry // required dependency for relationship tracking
}

func NewInvoiceRegistry(commodityRegistry *CommodityRegistry) *InvoiceRegistry {
	return &InvoiceRegistry{
		baseInvoiceRegistry: NewRegistry[models.Invoice, *models.Invoice](),
		commodityRegistry:   commodityRegistry,
	}
}

func (r *InvoiceRegistry) MustWithCurrentUser(ctx context.Context) registry.InvoiceRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *InvoiceRegistry) WithCurrentUser(ctx context.Context) (registry.InvoiceRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}
	tmp.userID = user.ID
	return &tmp, nil
}

func (r *InvoiceRegistry) Create(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Use CreateWithUser to ensure user context is applied
	newInvoice, err := r.baseInvoiceRegistry.CreateWithUser(ctx, invoice)
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
	if existingInvoice, err := r.baseInvoiceRegistry.Get(ctx, invoice.GetID()); err == nil {
		oldCommodityID = existingInvoice.CommodityID
	}

	// Call the base registry's Update method
	updatedInvoice, err := r.baseInvoiceRegistry.Update(ctx, invoice)
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
	invoice, err := r.baseInvoiceRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoice")
	}

	_ = r.commodityRegistry.DeleteInvoice(ctx, invoice.CommodityID, id)

	err = r.baseInvoiceRegistry.Delete(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice")
	}

	err = r.commodityRegistry.DeleteInvoice(ctx, invoice.CommodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice from commodity")
	}

	return nil
}
