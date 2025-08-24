package store

import (
	"context"
	"fmt"
	"iter"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/typekit"
)

type TxExecutor[T any] struct {
	tx    *sqlx.Tx
	table TableName
}

func NewTxRegistry[T any](tx *sqlx.Tx, table TableName) *TxExecutor[T] {
	return &TxExecutor[T]{
		tx:    tx,
		table: table,
	}
}

func (r *TxExecutor[T]) Scan(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		query := fmt.Sprintf("SELECT * FROM %s", r.table)

		rows, err := r.tx.QueryxContext(ctx, query)
		if err != nil {
			var zero T
			yield(zero, err)
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

func (r *TxExecutor[T]) ScanByField(ctx context.Context, field FieldValue) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", r.table, field.Field)

		rows, err := r.tx.QueryxContext(ctx, query, field.Value)
		if err != nil {
			var zero T
			yield(zero, err)
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

func (r *TxExecutor[T]) Insert(ctx context.Context, entity any) error {
	var fields []string
	var placeholders []string
	params := make(map[string]any)

	err := typekit.ExtractDBFields(entity, &fields, &placeholders, params)
	if err != nil {
		return errkit.Wrap(err, "failed to extract fields")
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		r.table,
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	// slog.Info("Inserting entity", "query", query, "params", must.Must(json.Marshal(params)))

	_, err = sqlx.NamedExecContext(ctx, r.tx, query, params)
	if err != nil {
		return errkit.Wrap(err, "failed to insert entity")
	}

	return nil
}

func (r *TxExecutor[T]) UpdateByField(ctx context.Context, field FieldValue, entity any) error {
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
		r.table,
		strings.Join(updateFields, ", "),
		field.Field,
	)
	params["entity_field_value"] = field.Value

	_, err = sqlx.NamedExecContext(ctx, r.tx, query, params)
	if err != nil {
		return errkit.Wrap(err, "failed to update entity")
	}

	return nil
}

func (r *TxExecutor[T]) ScanOneByField(ctx context.Context, field FieldValue, entity *T) error {
	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1 LIMIT 1", r.table, field.Field)

	rows, err := r.tx.QueryxContext(ctx, query, field.Value)
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

func (r *TxExecutor[T]) DeleteByField(ctx context.Context, field FieldValue) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", r.table, field.Field)

	_, err := r.tx.ExecContext(ctx, query, field.Value)
	if err != nil {
		return errkit.Wrap(err, "failed to delete entity")
	}

	return nil
}

func (r *TxExecutor[T]) Count(ctx context.Context) (int, error) {
	var count int

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.table)
	err := r.tx.QueryRowxContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count entities")
	}

	return count, nil
}
