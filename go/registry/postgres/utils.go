package postgres

import (
	"context"
	"errors"
	"fmt"
	"iter"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
	"github.com/denisvmedia/inventario/registry"
)

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

func ScanEntityByFieldForUserID[T any, P *T](ctx context.Context, dbx *sqlx.DB, userID string, table TableName, field, value string, entity P) (err error) {
	if userID == "" {
		return errkit.WithStack(ErrUserIDRequired)
	}

	tx, err := dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback()

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", table, field)

	rows, err := tx.QueryxContext(ctx, query, value)
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

func ScanEntities[T any](ctx context.Context, tx *sqlx.Tx, table TableName) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		query := fmt.Sprintf("SELECT * FROM %s", table)

		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return
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

func ScanEntitiesForUserID[T any](ctx context.Context, dbx *sqlx.DB, userID string, table TableName) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		if userID == "" {
			yield(nil, errkit.WithStack(ErrUserIDRequired))
			return
		}

		tx, err := dbx.Beginx()
		if err != nil {
			yield(nil, errkit.Wrap(err, "failed to begin transaction"))
			return
		}
		defer tx.Rollback()

		err = errors.Join(setAppRole(ctx, tx), setUserContext(ctx, tx, userID))
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}

		for entity, err := range ScanEntities[T](ctx, tx, table) {
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

func ScanEntiiesByFieldForUserID[T any](ctx context.Context, dbx *sqlx.DB, userID string, table TableName, field, value string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		if userID == "" {
			yield(nil, errkit.WithStack(ErrUserIDRequired))
			return
		}

		tx, err := dbx.Beginx()
		if err != nil {
			yield(nil, errkit.Wrap(err, "failed to begin transaction"))
			return
		}
		defer tx.Rollback()

		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", table, field)

		rows, err := tx.QueryxContext(ctx, query, value)
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

func setRole(ctx context.Context, tx *sqlx.Tx, role string) error {
	_, err := tx.ExecContext(ctx, "SET LOCAL ROLE = $1", role)
	return errkit.Wrap(err, "failed to set role", "role", role)
}

func setAppRole(ctx context.Context, tx *sqlx.Tx) error {
	return setRole(ctx, tx, "inventario_app")
}

func setUserContext(ctx context.Context, tx *sqlx.Tx, userID string) error {
	_, err := tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", userID)
	return errkit.Wrap(err, "failed to set user context", "user_id", userID)
}

// generateID generates a new UUID string
func generateID() string {
	return uuid.New().String()
}
