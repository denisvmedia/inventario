package postgres

import (
	"context"
	"errors"
	"fmt"
	"iter"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

type UserAwareSQLRegistry[T any] struct {
	dbx    *sqlx.DB
	userID string
	table  TableName
}

type FieldValue struct {
	Field string
	Value any
}

func Pair(field string, value any) FieldValue {
	return FieldValue{
		Field: field,
		Value: value,
	}
}

func NewUserAwareSQLRegistry[T any](dbx *sqlx.DB, userID string, table TableName) *UserAwareSQLRegistry[T] {
	return &UserAwareSQLRegistry[T]{
		dbx:    dbx,
		userID: userID,
		table:  table,
	}
}

func (r *UserAwareSQLRegistry[T]) ScanByField(ctx context.Context, field FieldValue) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.beginTx(ctx)
		if err != nil {
			yield(nil, err)
			return
		}
		defer tx.Rollback() // Read-only transaction, so rollback is safe

		query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", r.table, field.Field)

		rows, err := r.dbx.QueryxContext(ctx, query, field.Value)
		if err != nil {
			yield(nil, err)
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

func (r *UserAwareSQLRegistry[T]) ScanOneByField(ctx context.Context, field FieldValue, entity *T) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, entity)
	if err != nil {
		return errkit.Wrap(err, "failed to scan entity")
	}

	return nil
}

func (r *UserAwareSQLRegistry[T]) Scan(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.beginTx(ctx)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer tx.Rollback() // Read-only transaction, so rollback is safe

		query := fmt.Sprintf("SELECT * FROM %s", r.table)

		rows, err := tx.QueryxContext(ctx, query)
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

func (r *UserAwareSQLRegistry[T]) Count(ctx context.Context) (int, error) {
	var count int

	tx, err := r.beginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // Read-only transaction, so rollback is safe

	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", r.table)
	err = r.dbx.QueryRowxContext(ctx, query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *UserAwareSQLRegistry[T]) Create(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return errkit.Wrap(err, "failed to call checker function", "entity_type", r.table)
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.Insert(ctx, entity)
	if err != nil {
		return errkit.Wrap(err, "failed to insert entity")
	}

	return nil
}

func (r *UserAwareSQLRegistry[T]) Update(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx, T) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	idable := r.entityToIDAble(entity)
	field := Pair("id", idable.GetID())

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, Pair("id", idable.GetID()), &dbEntity)
	if err != nil {
		return errkit.Wrap(err, "failed to scan entity")
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx, dbEntity)
		if err != nil {
			return errkit.Wrap(err, "failed to call checker function", "entity_type", r.table)
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.UpdateByField(ctx, field, entity)
	if err != nil {
		return errkit.Wrap(err, "failed to update entity")
	}

	return nil
}

func (r *UserAwareSQLRegistry[T]) Delete(ctx context.Context, id string, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", id)

	var entity T
	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.ScanOneByField(ctx, field, &entity)
	if err != nil {
		return errkit.Wrap(err, "entity not found")
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return errkit.Wrap(err, "failed to call checker function", "entity_type", r.table)
		}
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", r.table, field.Field)

	_, err = tx.ExecContext(ctx, query, field.Value)
	if err != nil {
		return errkit.Wrap(err, "failed to delete entity")
	}

	return nil
}

func (r *UserAwareSQLRegistry[T]) DoWithEntity(ctx context.Context, entity T, operationFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	idable := r.entityToIDAble(entity)
	field := Pair("id", idable.GetID())

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return errkit.Wrap(err, "failed to scan entity")
	}

	if operationFn != nil {
		err = operationFn(ctx, tx)
		if err != nil {
			return errkit.Wrap(err, "failed to call operationFn", "entity_type", r.table)
		}
	}

	return nil
}

func (r *UserAwareSQLRegistry[T]) DoWithEntityID(ctx context.Context, entityID string, operationFn func(context.Context, *sqlx.Tx, T) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", entityID)

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return errkit.Wrap(err, "failed to scan entity")
	}

	if operationFn != nil {
		err = operationFn(ctx, tx, dbEntity)
		if err != nil {
			return errkit.Wrap(err, "failed to call operationFn", "entity_type", r.table)
		}
	}

	return nil
}

func (r *UserAwareSQLRegistry[T]) Do(ctx context.Context, operationFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	if operationFn != nil {
		err = operationFn(ctx, tx)
		if err != nil {
			return errkit.Wrap(err, "failed to call operationFn", "entity_type", r.table)
		}
	}

	return nil
}

func (r *UserAwareSQLRegistry[T]) beginTx(ctx context.Context) (*sqlx.Tx, error) {
	if r.userID == "" {
		return nil, ErrUserIDRequired
	}

	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}

	err = setAppRole(ctx, tx)
	if err != nil {
		return tx, errkit.Wrap(err, "failed to set app role")
	}

	err = setUserContext(ctx, tx, r.userID)
	if err != nil {
		return tx, errkit.Wrap(err, "failed to set user context")
	}

	return tx, nil
}

func (r *UserAwareSQLRegistry[T]) entityToIDAble(entity T) models.IDable {
	var tmp any = entity
	idable, ok := tmp.(models.IDable)
	if !ok {
		panic("entity is not IDable")
	}
	return idable
}
