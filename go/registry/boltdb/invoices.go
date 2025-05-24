package boltdb

import (
	"context"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
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

func (r *InvoiceRegistry) Create(ctx context.Context, m models.Invoice) (*models.Invoice, error) {
	result, err := r.registry.Create(m, func(_tx dbx.TransactionOrBucket, _invoice *models.Invoice) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _invoice *models.Invoice) error {
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create invoice")
	}

	err = r.commodityRegistry.AddInvoice(ctx, result.CommodityID, result.ID)
	if err != nil {
		return result, errkit.Wrap(err, "failed to add invoice to commodity")
	}

	return result, nil
}

func (r *InvoiceRegistry) Get(_ context.Context, id string) (*models.Invoice, error) {
	return r.registry.Get(id)
}

func (r *InvoiceRegistry) List(_ context.Context) ([]*models.Invoice, error) {
	return r.registry.List()
}

func (r *InvoiceRegistry) Update(_ context.Context, m models.Invoice) (*models.Invoice, error) {
	return r.registry.Update(m, func(_tx dbx.TransactionOrBucket, _invoice *models.Invoice) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.Invoice) error {
		return nil
	})
}

func (r *InvoiceRegistry) Delete(ctx context.Context, id string) error {
	var commodityID string
	err := r.registry.Delete(id, func(_tx dbx.TransactionOrBucket, invoice *models.Invoice) error {
		commodityID = invoice.CommodityID
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.Invoice) error {
		return nil
	})
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice")
	}

	err = r.commodityRegistry.DeleteInvoice(ctx, commodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to remove invoice from commodity")
	}

	return nil
}

func (r *InvoiceRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
}
