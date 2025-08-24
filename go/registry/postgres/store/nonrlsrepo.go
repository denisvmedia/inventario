package store

import (
	"context"
	"errors"
	"iter"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

// NonRLSRepository provides basic SQL operations without user context requirements
// This is useful for entities that don't need user isolation (like users themselves)
type NonRLSRepository[T any] struct {
	dbx   *sqlx.DB
	table TableName
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

func NewSQLRegistry[T any](dbx *sqlx.DB, table TableName) *NonRLSRepository[T] {
	return &NonRLSRepository[T]{
		dbx:   dbx,
		table: table,
	}
}

// Scan returns an iterator over all entities in the table
func (r *NonRLSRepository[T]) Scan(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.dbx.Beginx()
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer tx.Rollback() // Read-only transaction, so rollback is safe

		txreg := NewTxRegistry[T](tx, r.table)
		for entity, err := range txreg.Scan(ctx) {
			if !yield(entity, err) {
				return
			}
		}
	}
}

// ScanByField returns an iterator over entities matching a field value
func (r *NonRLSRepository[T]) ScanByField(ctx context.Context, field FieldValue) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.dbx.Beginx()
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer tx.Rollback() // Read-only transaction, so rollback is safe

		txreg := NewTxRegistry[T](tx, r.table)
		for entity, err := range txreg.ScanByField(ctx, field) {
			if !yield(entity, err) {
				return
			}
		}
	}
}

// ScanOneByField scans a single entity by field value
func (r *NonRLSRepository[T]) ScanOneByField(ctx context.Context, field FieldValue, entity *T) error {
	tx, err := r.dbx.Beginx()
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

// Count returns the total number of entities in the table
func (r *NonRLSRepository[T]) Count(ctx context.Context) (int, error) {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() // Read-only transaction, so rollback is safe

	txreg := NewTxRegistry[T](tx, r.table)
	count, err := txreg.Count(ctx)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Create creates a new entity with transaction support
func (r *NonRLSRepository[T]) Create(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.dbx.Beginx()
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

// Update updates an entity with transaction support
func (r *NonRLSRepository[T]) Update(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.dbx.Beginx()
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

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
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

// Delete deletes an entity by ID with transaction support
func (r *NonRLSRepository[T]) Delete(ctx context.Context, id string, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", id)

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return errkit.Wrap(err, "failed to scan entity")
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return errkit.Wrap(err, "failed to call checker function", "entity_type", r.table)
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.DeleteByField(ctx, field)
	if err != nil {
		return errkit.Wrap(err, "failed to delete entity")
	}

	return nil
}

// Do executes a function within a transaction
func (r *NonRLSRepository[T]) Do(ctx context.Context, operationFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.dbx.Beginx()
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

func (r *NonRLSRepository[T]) entityToIDAble(entity T) models.IDable {
	var tmp any = entity
	idable, ok := tmp.(models.IDable)
	if !ok {
		panic("entity is not IDable")
	}
	return idable
}
