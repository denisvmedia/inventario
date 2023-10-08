package boltdb

import (
	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameInvoice = "invoice"

	bucketNameInvoices         = "invoices"
	bucketNameInvoicesChildren = "invoices-children"
)

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)

type InvoiceRegistry struct {
	db                *bolt.DB
	base              *dbx.BaseRepository[models.Invoice, *models.Invoice]
	registry          *Registry[models.Invoice, *models.Invoice]
	commodityRegistry registry.CommodityRegistry
}

func NewInvoiceRegistry(db *bolt.DB, commodityRegistry registry.CommodityRegistry) *InvoiceRegistry {
	base := dbx.NewBaseRepository[models.Invoice, *models.Invoice](bucketNameInvoices)

	return &InvoiceRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.Invoice, *models.Invoice](
			db,
			base,
			entityNameInvoice,
			bucketNameInvoicesChildren,
		),
		commodityRegistry: commodityRegistry,
	}
}

func (r *InvoiceRegistry) Create(m models.Invoice) (*models.Invoice, error) {
	result, err := r.registry.Create(m, func(tx dbx.TransactionOrBucket, invoice *models.Invoice) error {
		return nil
	}, func(tx dbx.TransactionOrBucket, invoice *models.Invoice) error {
		return nil
	})
	if err != nil {
		return nil, err
	}

	err = r.commodityRegistry.AddInvoice(result.CommodityID, result.ID)
	if err != nil {
		return result, err
	}

	return result, nil
}

func (r *InvoiceRegistry) Get(id string) (*models.Invoice, error) {
	return r.registry.Get(id)
}

func (r *InvoiceRegistry) List() ([]*models.Invoice, error) {
	return r.registry.List()
}

func (r *InvoiceRegistry) Update(m models.Invoice) (*models.Invoice, error) {
	return r.registry.Update(m, func(tx dbx.TransactionOrBucket, invoice *models.Invoice) error {
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Invoice) error {
		return nil
	})
}

func (r *InvoiceRegistry) Delete(id string) error {
	var commodityID string
	err := r.registry.Delete(id, func(tx dbx.TransactionOrBucket, invoice *models.Invoice) error {
		commodityID = invoice.CommodityID
		return nil
	}, func(tx dbx.TransactionOrBucket, result *models.Invoice) error {
		return nil
	})
	if err != nil {
		return err
	}

	err = r.commodityRegistry.DeleteInvoice(commodityID, id)
	if err != nil {
		return err
	}

	return nil
}

func (r *InvoiceRegistry) Count() (int, error) {
	return r.registry.Count()
}
