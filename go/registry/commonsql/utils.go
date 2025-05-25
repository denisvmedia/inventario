package commonsql

import (
	"context"
	"fmt"
	"iter"
	"reflect"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jmoiron/sqlx"
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
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	var fields []string
	var placeholders []string
	params := map[string]any{}

	for i := 0; i < t.NumField(); i++ {
		dbTag := t.Field(i).Tag.Get("db")
		if dbTag == "" {
			continue
		}
		fields = append(fields, dbTag)
		placeholders = append(placeholders, ":"+dbTag)
		params[dbTag] = v.Field(i).Interface()
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		table,
		strings.Join(fields, ", "),
		strings.Join(placeholders, ", "),
	)

	_, err := sqlx.NamedExecContext(ctx, db, query, params)
	return err
}

func UpdateEntityByField(ctx context.Context, db sqlx.ExtContext, table, field, value string, entity any) error {
	t := reflect.TypeOf(entity)
	v := reflect.ValueOf(entity)

	var fields []string
	params := map[string]any{}

	for i := 0; i < t.NumField(); i++ {
		dbTag := t.Field(i).Tag.Get("db")
		if dbTag == "" {
			continue
		}
		fields = append(fields, fmt.Sprintf("%s = :%s", dbTag, dbTag))
		params[dbTag] = v.Field(i).Interface()
	}

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = :entity_field_value",
		table,
		strings.Join(fields, ", "),
		field,
	)
	params["entity_field_value"] = value

	_, err := sqlx.NamedExecContext(ctx, db, query, params)
	return err
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
	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1 LIMIT 1", table, field)

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
