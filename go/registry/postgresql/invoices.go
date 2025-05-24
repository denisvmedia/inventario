package postgresql

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.InvoiceRegistry = (*InvoiceRegistry)(nil)

type InvoiceRegistry struct {
	pool              *pgxpool.Pool
	commodityRegistry registry.CommodityRegistry
}

func NewInvoiceRegistry(pool *pgxpool.Pool, commodityRegistry registry.CommodityRegistry) *InvoiceRegistry {
	return &InvoiceRegistry{
		pool:              pool,
		commodityRegistry: commodityRegistry,
	}
}

func (r *InvoiceRegistry) Create(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Validate the invoice
	err := validation.Validate(&invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the commodity exists
	_, err = r.commodityRegistry.Get(ctx, invoice.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	// Generate a new ID
	if invoice.ID == "" {
		invoice.SetID(generateID())
	}

	// Insert the invoice into the database
	_, err = r.pool.Exec(ctx, `
		INSERT INTO invoices (id, commodity_id, path, original_path, ext, mime_type)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, invoice.ID, invoice.CommodityID, invoice.Path, invoice.OriginalPath, invoice.Ext, invoice.MIMEType)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create invoice")
	}

	// Add the invoice to the commodity
	err = r.commodityRegistry.AddInvoice(ctx, invoice.CommodityID, invoice.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to add invoice to commodity")
	}

	return &invoice, nil
}

func (r *InvoiceRegistry) Get(ctx context.Context, id string) (*models.Invoice, error) {
	var invoice models.Invoice
	invoice.File = &models.File{}

	// Query the database for the invoice
	err := r.pool.QueryRow(ctx, `
		SELECT id, commodity_id, path, original_path, ext, mime_type
		FROM invoices
		WHERE id = $1
	`, id).Scan(&invoice.ID, &invoice.CommodityID, &invoice.Path, &invoice.OriginalPath, &invoice.Ext, &invoice.MIMEType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errkit.Wrap(registry.ErrNotFound, "invoice not found")
		}
		return nil, errkit.Wrap(err, "failed to get invoice")
	}

	return &invoice, nil
}

func (r *InvoiceRegistry) List(ctx context.Context) ([]*models.Invoice, error) {
	var invoices []*models.Invoice

	// Query the database for all invoices
	rows, err := r.pool.Query(ctx, `
		SELECT id, commodity_id, path, original_path, ext, mime_type
		FROM invoices
	`)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list invoices")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var invoice models.Invoice
		invoice.File = &models.File{}
		if err := rows.Scan(&invoice.ID, &invoice.CommodityID, &invoice.Path, &invoice.OriginalPath, &invoice.Ext, &invoice.MIMEType); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		invoices = append(invoices, &invoice)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return invoices, nil
}

func (r *InvoiceRegistry) Update(ctx context.Context, invoice models.Invoice) (*models.Invoice, error) {
	// Validate the invoice
	err := validation.Validate(&invoice)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the invoice exists
	existingInvoice, err := r.Get(ctx, invoice.ID)
	if err != nil {
		return nil, err
	}

	// Check if the commodity exists
	_, err = r.commodityRegistry.Get(ctx, invoice.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// If the commodity ID has changed, update the commodity references
	if existingInvoice.CommodityID != invoice.CommodityID {
		// Remove the invoice from the old commodity
		err = r.commodityRegistry.DeleteInvoice(ctx, existingInvoice.CommodityID, invoice.ID)
		if err != nil {
			return nil, err
		}

		// Add the invoice to the new commodity
		err = r.commodityRegistry.AddInvoice(ctx, invoice.CommodityID, invoice.ID)
		if err != nil {
			return nil, err
		}
	}

	// Update the invoice in the database
	_, err = tx.Exec(ctx, `
		UPDATE invoices
		SET commodity_id = $1, path = $2, original_path = $3, ext = $4, mime_type = $5
		WHERE id = $6
	`, invoice.CommodityID, invoice.Path, invoice.OriginalPath, invoice.Ext, invoice.MIMEType, invoice.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update invoice")
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to commit transaction")
	}

	return &invoice, nil
}

func (r *InvoiceRegistry) Delete(ctx context.Context, id string) error {
	// Check if the invoice exists
	invoice, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Delete the invoice from the database
	_, err = tx.Exec(ctx, `
		DELETE FROM invoices
		WHERE id = $1
	`, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete invoice")
	}

	// Remove the invoice from the commodity
	err = r.commodityRegistry.DeleteInvoice(ctx, invoice.CommodityID, id)
	if err != nil {
		return err
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to commit transaction")
	}

	return nil
}

func (r *InvoiceRegistry) Count(ctx context.Context) (int, error) {
	var count int

	// Query the database for the count
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM invoices
	`).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count invoices")
	}

	return count, nil
}
