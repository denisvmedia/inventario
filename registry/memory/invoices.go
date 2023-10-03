package memory

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type baseInvoiceRegistry = Registry[models.Invoice]
type InvoiceRegistry struct {
	*baseInvoiceRegistry

	commodityRegistry registry.CommodityRegistry
}

func NewInvoiceRegistry(commodityRegistry registry.CommodityRegistry) *InvoiceRegistry {
	return &InvoiceRegistry{
		baseInvoiceRegistry: NewRegistry[models.Invoice](),
		commodityRegistry:   commodityRegistry,
	}
}

func (r *InvoiceRegistry) Create(invoice models.Invoice) (*models.Invoice, error) {
	err := validation.Validate(&invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(invoice.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newInvoice, err := r.baseInvoiceRegistry.Create(invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create invoice")
	}

	r.commodityRegistry.AddInvoice(invoice.CommodityID, newInvoice.ID)

	return newInvoice, err
}

func (r *InvoiceRegistry) Delete(id string) error {
	invoice, err := r.baseInvoiceRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseInvoiceRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.commodityRegistry.DeleteInvoice(invoice.CommodityID, id)

	return nil
}
