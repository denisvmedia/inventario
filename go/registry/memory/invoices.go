package memory

import (
	"context"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)

type baseInvoiceRegistry = Registry[models.Invoice, *models.Invoice]
type InvoiceRegistry struct {
	*baseInvoiceRegistry

	commodityRegistry registry.CommodityRegistry
}

func NewInvoiceRegistry(commodityRegistry registry.CommodityRegistry) *InvoiceRegistry {
	return &InvoiceRegistry{
		baseInvoiceRegistry: NewRegistry[models.Invoice, *models.Invoice](),
		commodityRegistry:   commodityRegistry,
	}
}

func (r *InvoiceRegistry) Create(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	_, err := r.commodityRegistry.Get(ctx, invoice.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newInvoice, err := r.baseInvoiceRegistry.Create(ctx, invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create invoice")
	}

	err = r.commodityRegistry.AddInvoice(ctx, invoice.CommodityID, newInvoice.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed adding invoice")
	}

	return newInvoice, nil
}

func (r *InvoiceRegistry) Delete(ctx context.Context, id string) error {
	invoice, err := r.baseInvoiceRegistry.Get(ctx, id)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoice")
	}

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
