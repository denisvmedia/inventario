package postgresql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
)

// Registry is a generic registry implementation for PostgreSQL
type Registry[T any, P registry.PIDable[T]] struct {
	pool       *pgxpool.Pool
	tableName  string
	entityName string
}

// NewRegistry creates a new registry for the given entity type
func NewRegistry[T any, P registry.PIDable[T]](
	pool *pgxpool.Pool,
	tableName,
	entityName string,
) *Registry[T, P] {
	return &Registry[T, P]{
		pool:       pool,
		tableName:  tableName,
		entityName: entityName,
	}
}

// Create creates a new entity in the registry
func (r *Registry[T, P]) Create(item T) (P, error) {
	ctx := context.Background()
	result := P(&item)
	result.SetID(uuid.New().String())

	// Convert the item to JSON
	data, err := json.Marshal(result)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal item")
	}

	// Insert the item into the database
	query := fmt.Sprintf("INSERT INTO %s (data, id) VALUES ($1, $2)", r.tableName)
	_, err = r.pool.Exec(ctx, query, data, result.GetID())
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert item")
	}

	return result, nil
}

// Get retrieves an entity from the registry by ID
func (r *Registry[T, P]) Get(id string) (P, error) {
	ctx := context.Background()
	var result P
	var data []byte

	// Query the database for the item
	query := fmt.Sprintf("SELECT data FROM %s WHERE id = $1", r.tableName)
	err := r.pool.QueryRow(ctx, query, id).Scan(&data)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("%s not found", r.entityName))
		}
		return nil, errkit.Wrap(err, "failed to get item")
	}

	// Unmarshal the JSON data
	var item T
	if err := json.Unmarshal(data, &item); err != nil {
		return nil, errkit.Wrap(err, "failed to unmarshal item")
	}

	result = P(&item)
	result.SetID(id)
	return result, nil
}

// List returns all entities in the registry
func (r *Registry[T, P]) List() ([]*T, error) {
	ctx := context.Background()
	var results []*T

	// Query the database for all items
	query := fmt.Sprintf("SELECT id, data FROM %s", r.tableName)
	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list items")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var id string
		var data []byte
		if err := rows.Scan(&id, &data); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}

		var item T
		if err := json.Unmarshal(data, &item); err != nil {
			return nil, errkit.Wrap(err, "failed to unmarshal item")
		}

		result := P(&item)
		result.SetID(id)
		results = append(results, &item)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return results, nil
}

// Update updates an entity in the registry
func (r *Registry[T, P]) Update(item T) (P, error) {
	ctx := context.Background()
	result := P(&item)
	id := result.GetID()

	if id == "" {
		return nil, errkit.Wrap(registry.ErrFieldRequired, "id is required")
	}

	// Check if the item exists
	exists, err := r.exists(ctx, id)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("%s not found", r.entityName))
	}

	// Convert the item to JSON
	data, err := json.Marshal(result)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to marshal item")
	}

	// Update the item in the database
	query := fmt.Sprintf("UPDATE %s SET data = $1 WHERE id = $2", r.tableName)
	_, err = r.pool.Exec(ctx, query, data, id)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update item")
	}

	return result, nil
}

// Delete removes an entity from the registry
func (r *Registry[T, P]) Delete(id string) error {
	ctx := context.Background()

	// Check if the item exists
	exists, err := r.exists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return errkit.Wrap(registry.ErrNotFound, fmt.Sprintf("%s not found", r.entityName))
	}

	// Delete the item from the database
	query := fmt.Sprintf("DELETE FROM %s WHERE id = $1", r.tableName)
	_, err = r.pool.Exec(ctx, query, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete item")
	}

	return nil
}

// Count returns the number of entities in the registry
func (r *Registry[T, P]) Count() (int, error) {
	ctx := context.Background()
	var count int

	// Query the database for the count
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.tableName)
	err := r.pool.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count items")
	}

	return count, nil
}

// exists checks if an entity with the given ID exists
func (r *Registry[T, P]) exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	query := fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE id = $1)", r.tableName)
	err := r.pool.QueryRow(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, errkit.Wrap(err, "failed to check if item exists")
	}
	return exists, nil
}
