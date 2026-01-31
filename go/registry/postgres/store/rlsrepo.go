package store

import (
	"context"
	"errors"
	"iter"

	"github.com/go-extras/errx"
	"github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"
)

type RLSRepository[T any, P ptrTenantUserAware[T]] struct {
	dbx      *sqlx.DB
	userID   string
	tenantID string
	table    TableName
	service  bool
}

func NewUserAwareSQLRegistry[T any, P ptrTenantUserAware[T]](dbx *sqlx.DB, userID, tenantID string, table TableName) *RLSRepository[T, P] {
	// slog.Info("Creating new user aware SQL registry", "table", table, "userID", userID)
	return &RLSRepository[T, P]{
		dbx:      dbx,
		userID:   userID,
		tenantID: tenantID,
		table:    table,
		service:  false,
	}
}

func NewServiceSQLRegistry[T any, P ptrTenantUserAware[T]](dbx *sqlx.DB, table TableName) *RLSRepository[T, P] {
	// slog.Info("Creating new service SQL registry", "table", table)
	return &RLSRepository[T, P]{
		dbx:     dbx,
		table:   table,
		service: true,
	}
}

func (r *RLSRepository[T, P]) ScanByField(ctx context.Context, field FieldValue) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.beginTx(ctx)
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

func (r *RLSRepository[T, P]) ScanOneByField(ctx context.Context, field FieldValue, entity *T) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer tx.Rollback() // Read-only transaction, so rollback is safe

	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, entity)
	if err != nil {
		return stacktrace.Wrap("failed to scan entity", err)
	}

	return nil
}

func (r *RLSRepository[T, P]) Scan(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.beginTx(ctx)
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

func (r *RLSRepository[T, P]) Count(ctx context.Context) (int, error) {
	tx, err := r.beginTx(ctx)
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

func (r *RLSRepository[T, P]) Create(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) (T, error) {
	var zero T
	tx, err := r.beginTx(ctx)
	if err != nil {
		return zero, stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Always generate a new server-side ID for security (ignore any user-provided ID)
	P(&entity).SetID(generateID())

	// For service registries, preserve the tenant and user IDs from the entity
	// For user registries, use the registry's context
	if !r.service {
		P(&entity).SetUserID(r.userID)
		P(&entity).SetTenantID(r.tenantID)
	}

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

func (r *RLSRepository[T, P]) Update(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx, T) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Only set user/tenant IDs for user registries, not service registries
	if !r.service {
		P(&entity).SetUserID(r.userID)
		P(&entity).SetTenantID(r.tenantID)
	}

	field := Pair("id", P(&entity).GetID())

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return stacktrace.Wrap("failed to scan entity", err)
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx, dbEntity)
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

func (r *RLSRepository[T, P]) Delete(ctx context.Context, id string, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", id)

	var entity T
	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.ScanOneByField(ctx, field, &entity)
	if err != nil {
		return stacktrace.Wrap("entity not found", err)
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return stacktrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	err = txreg.DeleteByField(ctx, field)
	if err != nil {
		return stacktrace.Wrap("failed to delete entity", err)
	}

	return nil
}

func (r *RLSRepository[T, P]) DoWithEntity(ctx context.Context, entity T, operationFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
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

	if operationFn != nil {
		err = operationFn(ctx, tx)
		if err != nil {
			return stacktrace.Wrap("failed to call operationFn (RLSRepository.DoWithEntity)", err, errx.Attrs("entity_type", r.table))
		}
	}

	return nil
}

func (r *RLSRepository[T, P]) DoWithEntityID(ctx context.Context, entityID string, operationFn func(context.Context, *sqlx.Tx, T) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", entityID)

	// check if entity exists
	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return stacktrace.Wrap("failed to scan entity", err)
	}

	if operationFn != nil {
		err = operationFn(ctx, tx, dbEntity)
		if err != nil {
			return stacktrace.Wrap("failed to call operationFn (RLSRepository.DoWithEntityID)", err, errx.Attrs("entity_type", r.table))
		}
	}

	return nil
}

func (r *RLSRepository[T, P]) Do(ctx context.Context, operationFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return stacktrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	if operationFn != nil {
		err = operationFn(ctx, tx)
		if err != nil {
			return stacktrace.Wrap("failed to call operationFn (RLSRepository.Do)", err, errx.Attrs("entity_type", r.table))
		}
	}

	return nil
}

func (r *RLSRepository[T, P]) beginTx(ctx context.Context) (*sqlx.Tx, error) {
	if r.service {
		return beginServiceTx(ctx, r.dbx)
	}
	return beginTxWithTenantAndUser(ctx, r.dbx, r.userID, r.tenantID)
}
