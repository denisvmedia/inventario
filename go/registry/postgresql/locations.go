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

var _ registry.LocationRegistry = (*LocationRegistry)(nil)

type LocationRegistry struct {
	pool *pgxpool.Pool
}

func NewLocationRegistry(pool *pgxpool.Pool) *LocationRegistry {
	return &LocationRegistry{
		pool: pool,
	}
}

func (r *LocationRegistry) Create(ctx context.Context, location models.Location) (*models.Location, error) {
	if location.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	// Generate a new ID
	if location.ID == "" {
		location.SetID(generateID())
	}

	// Insert the location into the database
	_, err := r.pool.Exec(ctx, `
		INSERT INTO locations (id, name, address)
		VALUES ($1, $2, $3)
	`, location.ID, location.Name, location.Address)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create location")
	}

	return &location, nil
}

func (r *LocationRegistry) Get(ctx context.Context, id string) (*models.Location, error) {
	var location models.Location

	// Query the database for the location
	err := r.pool.QueryRow(ctx, `
		SELECT id, name, address
		FROM locations
		WHERE id = $1
	`, id).Scan(&location.ID, &location.Name, &location.Address)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errkit.Wrap(registry.ErrNotFound, "location not found")
		}
		return nil, errkit.Wrap(err, "failed to get location")
	}

	return &location, nil
}

func (r *LocationRegistry) List(ctx context.Context) ([]*models.Location, error) {
	var locations []*models.Location

	// Query the database for all locations
	rows, err := r.pool.Query(ctx, `
		SELECT id, name, address
		FROM locations
	`)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list locations")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var location models.Location
		if err := rows.Scan(&location.ID, &location.Name, &location.Address); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		locations = append(locations, &location)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return locations, nil
}

func (r *LocationRegistry) Update(ctx context.Context, location models.Location) (*models.Location, error) {
	// Validate the location
	err := validation.Validate(&location)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the location exists
	_, err = r.Get(ctx, location.ID)
	if err != nil {
		return nil, err
	}

	// Update the location in the database
	_, err = r.pool.Exec(ctx, `
		UPDATE locations
		SET name = $1, address = $2
		WHERE id = $3
	`, location.Name, location.Address, location.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update location")
	}

	return &location, nil
}

func (r *LocationRegistry) Delete(ctx context.Context, id string) error {
	// Check if the location exists
	_, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	// Check if the location has areas
	areas, err := r.GetAreas(ctx, id)
	if err != nil {
		return err
	}
	if len(areas) > 0 {
		return errkit.Wrap(registry.ErrCannotDelete, "location has areas")
	}

	// Delete the location from the database
	_, err = r.pool.Exec(ctx, `
		DELETE FROM locations
		WHERE id = $1
	`, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete location")
	}

	return nil
}

func (r *LocationRegistry) Count(ctx context.Context) (int, error) {
	var count int

	// Query the database for the count
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM locations
	`).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count locations")
	}

	return count, nil
}

func (r *LocationRegistry) AddArea(ctx context.Context, locationID, areaID string) error {
	// Check if the location exists
	_, err := r.Get(ctx, locationID)
	if err != nil {
		return err
	}

	// Check if the area exists and has the correct location ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM areas
		WHERE id = $1 AND location_id = $2
	`, areaID, locationID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check area")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "area not found or does not belong to this location")
	}

	return nil
}

func (r *LocationRegistry) GetAreas(ctx context.Context, locationID string) ([]string, error) {
	var areas []string

	// Check if the location exists
	_, err := r.Get(ctx, locationID)
	if err != nil {
		return nil, err
	}

	// Query the database for all areas in the location
	rows, err := r.pool.Query(ctx, `
		SELECT id
		FROM areas
		WHERE location_id = $1
	`, locationID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list areas")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var areaID string
		if err := rows.Scan(&areaID); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		areas = append(areas, areaID)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return areas, nil
}

func (r *LocationRegistry) DeleteArea(ctx context.Context, locationID, areaID string) error {
	// Check if the location exists
	_, err := r.Get(ctx, locationID)
	if err != nil {
		return err
	}

	// Check if the area exists and has the correct location ID
	var count int
	err = r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM areas
		WHERE id = $1 AND location_id = $2
	`, areaID, locationID).Scan(&count)
	if err != nil {
		return errkit.Wrap(err, "failed to check area")
	}
	if count == 0 {
		return errkit.Wrap(registry.ErrNotFound, "area not found or does not belong to this location")
	}

	// Delete the area from the database
	_, err = r.pool.Exec(ctx, `
		DELETE FROM areas
		WHERE id = $1 AND location_id = $2
	`, areaID, locationID)
	if err != nil {
		return errkit.Wrap(err, "failed to delete area")
	}

	return nil
}
