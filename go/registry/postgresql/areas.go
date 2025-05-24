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

var _ registry.AreaRegistry = (*AreaRegistry)(nil)

type AreaRegistry struct {
	pool             *pgxpool.Pool
	locationRegistry registry.LocationRegistry
}

func NewAreaRegistry(pool *pgxpool.Pool, locationRegistry registry.LocationRegistry) *AreaRegistry {
	return &AreaRegistry{
		pool:             pool,
		locationRegistry: locationRegistry,
	}
}

func (r *AreaRegistry) Create(area models.Area) (*models.Area, error) {
	ctx := context.Background()

	// Validate the area
	err := validation.Validate(&area)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the location exists
	_, err = r.locationRegistry.Get(area.LocationID)
	if err != nil {
		return nil, errkit.Wrap(err, "location not found")
	}

	// Generate a new ID
	if area.ID == "" {
		area.SetID(generateID())
	}

	// Insert the area into the database
	_, err = r.pool.Exec(ctx, `
		INSERT INTO areas (id, name, location_id)
		VALUES ($1, $2, $3)
	`, area.ID, area.Name, area.LocationID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create area")
	}

	// Add the area to the location
	err = r.locationRegistry.AddArea(area.LocationID, area.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to add area to location")
	}

	return &area, nil
}

func (r *AreaRegistry) Get(id string) (*models.Area, error) {
	ctx := context.Background()
	var area models.Area

	// Query the database for the area
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, location_id
		FROM areas
		WHERE id = $1
	`, id).Scan(&area.ID, &area.Name, &area.LocationID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errkit.Wrap(registry.ErrNotFound, "area not found")
		}
		return nil, errkit.Wrap(err, "failed to get area")
	}

	return &area, nil
}

func (r *AreaRegistry) List() ([]*models.Area, error) {
	ctx := context.Background()
	var areas []*models.Area

	// Query the database for all areas
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, location_id
		FROM areas
	`)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list areas")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var area models.Area
		if err := rows.Scan(&area.ID, &area.Name, &area.LocationID); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		areas = append(areas, &area)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return areas, nil
}

func (r *AreaRegistry) Update(area models.Area) (*models.Area, error) {
	ctx := context.Background()

	// Validate the area
	err := validation.Validate(&area)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the area exists
	existingArea, err := r.Get(area.ID)
	if err != nil {
		return nil, err
	}

	// Check if the location exists
	_, err = r.locationRegistry.Get(area.LocationID)
	if err != nil {
		return nil, errkit.Wrap(err, "location not found")
	}

	// If the location ID has changed, update the location references
	if existingArea.LocationID != area.LocationID {
		// Get the commodities in the area
		commodities, err := r.GetCommodities(area.ID)
		if err != nil {
			return nil, err
		}

		// Begin a transaction
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to begin transaction")
		}
		defer tx.Rollback(ctx)

		// Remove the area from the old location
		err = r.locationRegistry.DeleteArea(existingArea.LocationID, area.ID)
		if err != nil {
			return nil, err
		}

		// Update the area in the database
		_, err = tx.Exec(ctx, `
			UPDATE areas
			SET name = $1, location_id = $2
			WHERE id = $3
		`, area.Name, area.LocationID, area.ID)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to update area")
		}

		// Add the area to the new location
		err = r.locationRegistry.AddArea(area.LocationID, area.ID)
		if err != nil {
			return nil, err
		}

		// Commit the transaction
		err = tx.Commit(ctx)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to commit transaction")
		}
	} else {
		// Update the area in the database
		_, err = r.pool.Exec(ctx, `
			UPDATE areas
			SET name = $1
			WHERE id = $2
		`, area.Name, area.ID)
		if err != nil {
			return nil, errkit.Wrap(err, "failed to update area")
		}
	}

	return &area, nil
}

func (r *AreaRegistry) Delete(id string) error {
	ctx := context.Background()

	// Check if the area exists
	area, err := r.Get(id)
	if err != nil {
		return err
	}

	// Check if the area has commodities
	commodities, err := r.GetCommodities(id)
	if err != nil {
		return err
	}
	if len(commodities) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "area has commodities")
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Delete the area from the database
	_, err = tx.Exec(ctx, `
		DELETE FROM areas
		WHERE id = $1
	`, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area")
	}

	// Remove the area from the location
	err = r.locationRegistry.DeleteArea(area.LocationID, id)
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

func (r *AreaRegistry) Count() (int, error) {
	ctx := context.Background()
	var count int

	// Query the database for the count
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM areas
	`).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count areas")
	}

	return count, nil
}

func (r *AreaRegistry) AddCommodity(areaID, commodityID string) error {
	ctx := context.Background()

	// Check if the area exists
	_, err := r.Get(areaID)
	if err != nil {
		return err
	}

	// Check if the commodity exists and has the correct area ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM commodities
		WHERE id = $1 AND area_id = $2
	`, commodityID, areaID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check commodity")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "commodity not found or does not belong to this area")
	}

	return nil
}

func (r *AreaRegistry) GetCommodities(areaID string) ([]string, error) {
	ctx := context.Background()
	var commodities []string

	// Check if the area exists
	_, err := r.Get(areaID)
	if err != nil {
		return nil, err
	}

	// Query the database for all commodities in the area
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM commodities
		WHERE area_id = $1
	`, areaID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list commodities")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var commodityID string
		if err := rows.Scan(&commodityID); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		commodities = append(commodities, commodityID)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return commodities, nil
}

func (r *AreaRegistry) DeleteCommodity(areaID, commodityID string) error {
	ctx := context.Background()

	// Check if the area exists
	_, err := r.Get(areaID)
	if err != nil {
		return err
	}

	// Check if the commodity exists and has the correct area ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM commodities
		WHERE id = $1 AND area_id = $2
	`, commodityID, areaID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check commodity")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "commodity not found or does not belong to this area")
	}

	// Delete the commodity from the database
	_, err = r.pool.Exec(ctx, `
		DELETE FROM commodities
		WHERE id = $1 AND area_id = $2
	`, commodityID, areaID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete commodity")
	}

	return nil
}
