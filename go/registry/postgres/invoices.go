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

// WithUserContext executes a function with user context set
func (r *InvoiceRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	return WithUserContext(ctx, r.dbx, userID, fn)
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

// User-aware methods that automatically use user context from the request context

// CreateWithUser creates an invoice with user context
func (r *InvoiceRegistry) CreateWithUser(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user_id on the invoice
	invoice.SetUserID(userID)

	// Generate a new ID if one is not already provided
	if invoice.GetID() == "" {
		invoice.SetID(generateID())
	}

	// Set user context for RLS and insert the invoice
	err := InsertEntityWithUser(ctx, r.dbx, r.tableNames.Invoices(), invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &invoice, nil
}

// GetWithUser gets an invoice with user context
func (r *InvoiceRegistry) GetWithUser(ctx context.Context, id string) (*models.Invoice, error) {
	var invoice models.Invoice
	err := ScanEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Invoices(), "id", id, &invoice)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "Invoice",
				"entity_id", id,
			)
		}
		return nil, errkit.Wrap(err, "failed to get entity")
	}

	return &invoice, nil
}

// ListWithUser lists invoices with user context
func (r *InvoiceRegistry) ListWithUser(ctx context.Context) ([]*models.Invoice, error) {
	var invoices []*models.Invoice

	// Query the database for all invoices with user context
	for invoice, err := range ScanEntitiesWithUser[models.Invoice](ctx, r.dbx, r.tableNames.Invoices()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list invoices")
		}
		invoices = append(invoices, &invoice)
	}

	return invoices, nil
}

// UpdateWithUser updates an invoice with user context
func (r *InvoiceRegistry) UpdateWithUser(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user_id on the invoice
	invoice.SetUserID(userID)

	// Update the invoice with user context
	err := UpdateEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Invoices(), "id", invoice.ID, invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update entity")
	}

	return &invoice, nil
}

// DeleteWithUser deletes an invoice with user context
func (r *InvoiceRegistry) DeleteWithUser(ctx context.Context, id string) error {
	return DeleteEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Invoices(), "id", id)
}

// CountWithUser counts invoices with user context
func (r *InvoiceRegistry) CountWithUser(ctx context.Context) (int, error) {
	return CountEntitiesWithUser(ctx, r.dbx, r.tableNames.Invoices())
}
