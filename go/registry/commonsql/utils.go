package commonsql

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
)

type txOrPool interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
}

// generateID generates a new UUID string
func generateID() string {
	return uuid.New().String()
}

type txKey string

const txKeyVal txKey = "tx"

func ContextWithTransaction(ctx context.Context, tx sqlx.ExtContext) context.Context {
	return context.WithValue(ctx, txKeyVal, tx)
}

func TransactionFromContext(ctx context.Context) sqlx.ExtContext {
	tx, ok := ctx.Value(txKeyVal).(sqlx.ExtContext)
	if !ok {
		return nil
	}
	return tx
}

func InsertEntity(ctx context.Context, db sqlx.ExtContext, table string, entity any) error {
	var fields []string
	var placeholders []string
	params := map[string]any{}

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
	params := map[string]any{}

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

func CountEntities(ctx context.Context, db sqlx.ExtContext, table string) (int, error) {
	var count int

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", table)
	err := db.QueryRowxContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func RollbackOrCommit(tx *sqlx.Tx, err error) error {
	if err != nil {
		return tx.Rollback()
	}
	return tx.Commit()
}
