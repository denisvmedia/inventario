package registry

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type InvoiceRegistry interface {
	Registry[models.Invoice]
}

type baseMemoryInvoiceRegistry = MemoryRegistry[models.Invoice]
type MemoryInvoiceRegistry struct {
	*baseMemoryInvoiceRegistry

	commodityRegistry CommodityRegistry
}

func NewMemoryInvoiceRegistry(commodityRegistry CommodityRegistry) *MemoryInvoiceRegistry {
	return &MemoryInvoiceRegistry{
		baseMemoryInvoiceRegistry: NewMemoryRegistry[models.Invoice](),
		commodityRegistry:         commodityRegistry,
	}
}

func (r *MemoryInvoiceRegistry) Create(invoice models.Invoice) (*models.Invoice, error) {
	err := validation.Validate(invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	_, err = r.commodityRegistry.Get(invoice.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	newInvoice, err := r.baseMemoryInvoiceRegistry.Create(invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create invoice")
	}

	r.commodityRegistry.AddInvoice(invoice.CommodityID, newInvoice.ID)

	return newInvoice, err
}

func (r *MemoryInvoiceRegistry) Delete(id string) error {
	invoice, err := r.baseMemoryInvoiceRegistry.Get(id)
	if err != nil {
		return err
	}

	err = r.baseMemoryInvoiceRegistry.Delete(id)
	if err != nil {
		return err
	}

	r.commodityRegistry.DeleteInvoice(invoice.CommodityID, id)

	return nil
}
