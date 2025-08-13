package postgres

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)

type InvoiceRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewInvoiceRegistry(dbx *sqlx.DB) *InvoiceRegistry {
	return NewInvoiceRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewInvoiceRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *InvoiceRegistry {
	return &InvoiceRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *InvoiceRegistry) Create(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	var commodity models.Commodity
	err = ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", invoice.CommodityID, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// Generate a new ID if one is not already provided
	if invoice.GetID() == "" {
		invoice.SetID(generateID())
	}

	err = InsertEntity(ctx, tx, r.tableNames.Invoices(), invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &invoice, nil
}

func (r *InvoiceRegistry) Get(ctx context.Context, id string) (*models.Invoice, error) {
	return r.get(ctx, r.dbx, id)
}

func (r *InvoiceRegistry) List(ctx context.Context) ([]*models.Invoice, error) {
	var invoices []*models.Invoice

	// Query the database for all invoices (atomic operation)
	for invoice, err := range ScanEntities[models.Invoice](ctx, r.dbx, r.tableNames.Invoices()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list invoices")
		}
		invoices = append(invoices, &invoice)
	}

	return invoices, nil
}

func (r *InvoiceRegistry) Update(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the invoice exists
	_, err = r.get(ctx, tx, invoice.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get invoice")
	}

	// Check if the commodity exists
	_, err = r.getCommodity(ctx, tx, invoice.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// TODO: what if commodity has changed, allow or not? (currently allowed)

	err = UpdateEntityByField(ctx, tx, r.tableNames.Invoices(), "id", invoice.ID, invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update invoice")
	}

	return &invoice, nil
}

func (r *InvoiceRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the invoice exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return err
	}

	// Finally, delete the invoice
	err = DeleteEntityByField(ctx, tx, r.tableNames.Invoices(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice")
	}

	return nil
}

func (r *InvoiceRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := CountEntities(ctx, r.dbx, r.tableNames.Invoices())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count invoices")
	}

	return cnt, nil
}

func (r *InvoiceRegistry) get(ctx context.Context, tx sqlx.ExtContext, id string) (*models.Invoice, error) {
	var invoice models.Invoice
	err := ScanEntityByField(ctx, tx, r.tableNames.Invoices(), "id", id, &invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get invoice")
	}

	return &invoice, nil
}

func (r *InvoiceRegistry) getCommodity(ctx context.Context, tx sqlx.ExtContext, commodityID string) (*models.Commodity, error) {
	var commodity models.Commodity
	err := ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodityID, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	return &commodity, nil
}
