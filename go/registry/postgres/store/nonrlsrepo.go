package store

import (
	"context"
	"errors"
	"iter"

	"github.com/go-extras/errx"
	"github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"
)

// NonRLSRepository provides basic SQL operations without user context requirements
// This is useful for entities that don't need user isolation (like users themselves)
type NonRLSRepository[T any, P ptrIDable[T]] struct {
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

func NewSQLRegistry[T any, P ptrIDable[T]](dbx *sqlx.DB, table TableName) *NonRLSRepository[T, P] {
	return &NonRLSRepository[T, P]{
		dbx:   dbx,
		table: table,
	}
}

// Scan returns an iterator over all entities in the table
func (r *NonRLSRepository[T, P]) Scan(ctx context.Context) iter.Seq2[T, error] {
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
func (r *NonRLSRepository[T, P]) ScanByField(ctx context.Context, field FieldValue) iter.Seq2[T, error] {
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
func (r *NonRLSRepository[T, P]) ScanOneByField(ctx context.Context, field FieldValue, entity *T) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, entity)
	if err != nil {
		return stacktrace.Wrap("failed to scan entity", err)
	}

	return nil
}

// Count returns the total number of entities in the table
func (r *NonRLSRepository[T, P]) Count(ctx context.Context) (int, error) {
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
func (r *NonRLSRepository[T, P]) Create(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) (T, error) {
	var zero T
	tx, err := r.dbx.Beginx()
	if err != nil {
		return zero, stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Always generate a new server-side ID for security (ignore any user-provided ID)
	P(&entity).SetID(generateID())

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return zero, stacktrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.Insert(ctx, entity)
	if err != nil {
		return zero, stacktrace.Wrap("failed to insert entity", err)
	}

	return entity, nil
}

// Update updates an entity with transaction support
func (r *NonRLSRepository[T, P]) Update(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", P(&entity).GetID())

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return stacktrace.Wrap("failed to scan entity", err)
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return stacktrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.UpdateByField(ctx, field, entity)
	if err != nil {
		return stacktrace.Wrap("failed to update entity", err)
	}

	return nil
}

// Delete deletes an entity by ID with transaction support
func (r *NonRLSRepository[T, P]) Delete(ctx context.Context, id string, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", id)

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return stacktrace.Wrap("failed to scan entity", err)
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return stacktrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.DeleteByField(ctx, field)
	if err != nil {
		return stacktrace.Wrap("failed to delete entity", err)
	}

	return nil
}

// Do executes a function within a transaction
func (r *NonRLSRepository[T, P]) Do(ctx context.Context, operationFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.dbx.Beginx()
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	if operationFn != nil {
		err = operationFn(ctx, tx)
		if err != nil {
			return stacktrace.Wrap("failed to call operationFn (NonRLSRepository.Do)", err, errx.Attrs("entity_type", r.table))
		}
	}

	return nil
}
