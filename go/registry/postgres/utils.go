package postgres

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// generateID generates a new UUID string
func generateID() string {
	return uuid.New().String()
}

// SetUserContext sets the user context for RLS policies
func SetUserContext(ctx context.Context, db sqlx.ExtContext, userID string) error {
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	_, err := db.ExecContext(ctx, "SELECT set_user_context($1)", userID)
	if err != nil {
		return errkit.Wrap(err, "failed to set user context")
	}

	return nil
}

// WithUserContext executes a function with user context set for RLS
func WithUserContext(ctx context.Context, db sqlx.ExtContext, userID string, fn func(context.Context) error) error {
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context
	err := SetUserContext(ctx, db, userID)
	if err != nil {
		return err
	}

	// Execute the function
	return fn(ctx)
}

func InsertEntity(ctx context.Context, db sqlx.ExtContext, table string, entity any) error {
	var fields []string
	var placeholders []string
	params := make(map[string]any)

	err := typekit.ExtractDBFields(entity, &fields, &placeholders, params)
	if err != nil {
		return errkit.Wrap(err, "failed to extract fields")
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err = sqlx.NamedExecContext(ctx, db, query, params)
	if err != nil {
		return errkit.Wrap(err, "failed to insert entity")
	}

	return nil
}

// InsertEntityWithUser inserts an entity with user context set
func InsertEntityWithUser(ctx context.Context, db sqlx.ExtContext, table string, entity any) error {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user_id on entity if it's UserAware
	if userAware, ok := entity.(models.UserAware); ok {
		userAware.SetUserID(userID)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, db, userID)
	if err != nil {
		return err
	}

	// Insert the entity
	return InsertEntity(ctx, db, table, entity)
}

func UpdateEntityByField(ctx context.Context, db sqlx.ExtContext, table, field, value string, entity any) error {
	var fields []string
	var placeholders []string
	params := make(map[string]any)

	err := typekit.ExtractDBFields(entity, &fields, &placeholders, params)
	if err != nil {
		return errkit.Wrap(err, "failed to extract fields")
	}

	// Convert fields to update format
	var updateFields []string
	for _, fieldName := range fields {
		updateFields = append(updateFields, fmt.Sprintf("%s = :%s", fieldName, fieldName))
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = :entity_field_value",
		table,
		strings.Join(updateFields, ", "),
		field,
	)
	params["entity_field_value"] = value

	_, err = sqlx.NamedExecContext(ctx, db, query, params)
	if err != nil {
		return errkit.Wrap(err, "failed to update entity")
	}

	return nil
}

// UpdateEntityByFieldWithUser updates an entity with user context set
func UpdateEntityByFieldWithUser(ctx context.Context, db sqlx.ExtContext, table, field, value string, entity any) error {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, db, userID)
	if err != nil {
		return err
	}

	// Update the entity
	return UpdateEntityByField(ctx, db, table, field, value, entity)
}

func ScanEntityByField[T any, P *T](ctx context.Context, db sqlx.ExtContext, table, field, value string, entity P) error {
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", table, field)

	rows, err := db.QueryxContext(ctx, query, value)
	if err != nil {
		return err
	}
	defer rows.Close()

	if !rows.Next() {
		return ErrNotFound
	}

	err = rows.StructScan(entity)
	if err != nil {
		return err
	}

	return nil
}

// ScanEntityByFieldWithUser scans an entity with user context set
func ScanEntityByFieldWithUser[T any, P *T](ctx context.Context, db sqlx.ExtContext, table, field, value string, entity P) error {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, db, userID)
	if err != nil {
		return err
	}

	// Scan the entity
	return ScanEntityByField(ctx, db, table, field, value, entity)
}

func ScanEntities[T any](ctx context.Context, db sqlx.ExtContext, table string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		query := fmt.Sprintf("SELECT * FROM %s", table)

		rows, err := db.QueryxContext(ctx, query)
		if err != nil {
			return // yield ничего не делает при ошибке
		}
		defer rows.Close()

		for rows.Next() {
			var entity T
			err := rows.StructScan(&entity)
			if !yield(entity, err) {
				return
			}
		}
	}
}

// ScanEntitiesWithUser scans entities with user context set
func ScanEntitiesWithUser[T any](ctx context.Context, db sqlx.ExtContext, table string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		// Extract user ID from context
		userID := registry.UserIDFromContext(ctx)
		if userID == "" {
			var zero T
			yield(zero, errkit.WithStack(registry.ErrUserContextRequired))
			return
		}

		// Set user context for RLS
		err := SetUserContext(ctx, db, userID)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}

		// Use the regular ScanEntities function
		for entity, err := range ScanEntities[T](ctx, db, table) {
			if !yield(entity, err) {
				return
			}
		}
	}
}

func ScanEntitiesByField[T any](ctx context.Context, db sqlx.ExtContext, table, field, value string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", table, field)

		rows, err := db.QueryxContext(ctx, query, value)
		if err != nil {
			return // yield ничего не делает при ошибке
		}
		defer rows.Close()

		for rows.Next() {
			var entity T
			err := rows.StructScan(&entity)
			if !yield(entity, err) {
				return
			}
		}
	}
}

func DeleteEntityByField(ctx context.Context, db sqlx.ExtContext, table, field, value string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", table, field)

	_, err := db.ExecContext(ctx, query, value)
	return err
}

// DeleteEntityByFieldWithUser deletes an entity with user context set
func DeleteEntityByFieldWithUser(ctx context.Context, db sqlx.ExtContext, table, field, value string) error {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, db, userID)
	if err != nil {
		return err
	}

	// Delete the entity
	return DeleteEntityByField(ctx, db, table, field, value)
}

func CountEntities(ctx context.Context, db sqlx.ExtContext, table string) (int, error) {
	var count int

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	err := db.QueryRowxContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountEntitiesWithUser counts entities with user context set
func CountEntitiesWithUser(ctx context.Context, db sqlx.ExtContext, table string) (int, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return 0, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user context for RLS
	err := SetUserContext(ctx, db, userID)
	if err != nil {
		return 0, err
	}

	// Count the entities
	return CountEntities(ctx, db, table)
}

func RollbackOrCommit(tx *sqlx.Tx, err error) error {
	if err != nil {
		return tx.Rollback()
	}
	return tx.Commit()
}
