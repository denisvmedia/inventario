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

var _ registry.ManualRegistry = (*ManualRegistry)(nil)

type ManualRegistry struct {
	pool              *pgxpool.Pool
	commodityRegistry registry.CommodityRegistry
}

func NewManualRegistry(pool *pgxpool.Pool, commodityRegistry registry.CommodityRegistry) *ManualRegistry {
	return &ManualRegistry{
		pool:              pool,
		commodityRegistry: commodityRegistry,
	}
}

func (r *ManualRegistry) Create(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Validate the manual
	err := validation.Validate(&manual)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the commodity exists
	_, err = r.commodityRegistry.Get(ctx, manual.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	// Generate a new ID
	if manual.ID == "" {
		manual.SetID(generateID())
	}

	// Insert the manual into the database
	_, err = r.pool.Exec(ctx, `
		INSERT INTO manuals (id, commodity_id, path, original_path, ext, mime_type)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, manual.ID, manual.CommodityID, manual.Path, manual.OriginalPath, manual.Ext, manual.MIMEType)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create manual")
	}

	// Add the manual to the commodity
	err = r.commodityRegistry.AddManual(ctx, manual.CommodityID, manual.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to add manual to commodity")
	}

	return &manual, nil
}

func (r *ManualRegistry) Get(ctx context.Context, id string) (*models.Manual, error) {
	var manual models.Manual
	manual.File = &models.File{}

	// Query the database for the manual
	err := r.pool.QueryRow(ctx, `
		SELECT id, commodity_id, path, original_path, ext, mime_type
		FROM manuals
		WHERE id = $1
	`, id).Scan(&manual.ID, &manual.CommodityID, &manual.Path, &manual.OriginalPath, &manual.Ext, &manual.MIMEType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errkit.Wrap(registry.ErrNotFound, "manual not found")
		}
		return nil, errkit.Wrap(err, "failed to get manual")
	}

	return &manual, nil
}

func (r *ManualRegistry) List(ctx context.Context) ([]*models.Manual, error) {
	var manuals []*models.Manual

	// Query the database for all manuals
	rows, err := r.pool.Query(ctx, `
		SELECT id, commodity_id, path, original_path, ext, mime_type
		FROM manuals
	`)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list manuals")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var manual models.Manual
		manual.File = &models.File{}
		if err := rows.Scan(&manual.ID, &manual.CommodityID, &manual.Path, &manual.OriginalPath, &manual.Ext, &manual.MIMEType); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		manuals = append(manuals, &manual)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return manuals, nil
}

func (r *ManualRegistry) Update(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Validate the manual
	err := validation.Validate(&manual)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the manual exists
	existingManual, err := r.Get(ctx, manual.ID)
	if err != nil {
		return nil, err
	}

	// Check if the commodity exists
	_, err = r.commodityRegistry.Get(ctx, manual.CommodityID)
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
	if existingManual.CommodityID != manual.CommodityID {
		// Remove the manual from the old commodity
		err = r.commodityRegistry.DeleteManual(ctx, existingManual.CommodityID, manual.ID)
		if err != nil {
			return nil, err
		}

		// Add the manual to the new commodity
		err = r.commodityRegistry.AddManual(ctx, manual.CommodityID, manual.ID)
		if err != nil {
			return nil, err
		}
	}

	// Update the manual in the database
	_, err = tx.Exec(ctx, `
		UPDATE manuals
		SET commodity_id = $1, path = $2, original_path = $3, ext = $4, mime_type = $5
		WHERE id = $6
	`, manual.CommodityID, manual.Path, manual.OriginalPath, manual.Ext, manual.MIMEType, manual.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update manual")
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to commit transaction")
	}

	return &manual, nil
}

func (r *ManualRegistry) Delete(ctx context.Context, id string) error {
	// Check if the manual exists
	manual, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Delete the manual from the database
	_, err = tx.Exec(ctx, `
		DELETE FROM manuals
		WHERE id = $1
	`, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete manual")
	}

	// Remove the manual from the commodity
	err = r.commodityRegistry.DeleteManual(ctx, manual.CommodityID, id)
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

func (r *ManualRegistry) Count(ctx context.Context) (int, error) {
	var count int

	// Query the database for the count
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM manuals
	`).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count manuals")
	}

	return count, nil
}
