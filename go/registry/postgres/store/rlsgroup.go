package store

import (
	"context"
	"errors"
	"iter"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
)

// RLSGroupRepository provides SQL operations with tenant + group RLS enforcement.
// This is the group-aware counterpart to RLSRepository (which enforces tenant + user).
// It sets app.current_tenant_id and app.current_group_id on each transaction.
//
// On Create, it sets group_id and created_by_user_id on the entity automatically
// (analogous to how RLSRepository sets user_id and tenant_id).
type RLSGroupRepository[T any, P ptrTenantGroupAware[T]] struct {
	dbx             *sqlx.DB
	tenantID        string
	groupID         string
	createdByUserID string
	table           TableName
	service         bool
}

func NewGroupAwareSQLRegistry[T any, P ptrTenantGroupAware[T]](
	dbx *sqlx.DB, tenantID, groupID, createdByUserID string, table TableName,
) *RLSGroupRepository[T, P] {
	return &RLSGroupRepository[T, P]{
		dbx:             dbx,
		tenantID:        tenantID,
		groupID:         groupID,
		createdByUserID: createdByUserID,
		table:           table,
		service:         false,
	}
}

func NewGroupServiceSQLRegistry[T any, P ptrTenantGroupAware[T]](dbx *sqlx.DB, table TableName) *RLSGroupRepository[T, P] {
	return &RLSGroupRepository[T, P]{
		dbx:     dbx,
		table:   table,
		service: true,
	}
}

func (r *RLSGroupRepository[T, P]) Scan(ctx context.Context) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.beginTx(ctx)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer tx.Rollback()

		txreg := NewTxRegistry[T](tx, r.table)
		for entity, err := range txreg.Scan(ctx) {
			if !yield(entity, err) {
				return
			}
		}
	}
}

func (r *RLSGroupRepository[T, P]) ScanByField(ctx context.Context, field FieldValue) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.beginTx(ctx)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer tx.Rollback()

		txreg := NewTxRegistry[T](tx, r.table)
		for entity, err := range txreg.ScanByField(ctx, field) {
			if !yield(entity, err) {
				return
			}
		}
	}
}

func (r *RLSGroupRepository[T, P]) ScanOneByField(ctx context.Context, field FieldValue, entity *T) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to begin transaction", err)
	}
	defer tx.Rollback()

	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, entity)
	if err != nil {
		return errxtrace.Wrap("failed to scan entity", err)
	}

	return nil
}

func (r *RLSGroupRepository[T, P]) Count(ctx context.Context) (int, error) {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	txreg := NewTxRegistry[T](tx, r.table)
	count, err := txreg.Count(ctx)
	if err != nil {
		return 0, err
	}

	return count, nil
}

func (r *RLSGroupRepository[T, P]) Create(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) (T, error) {
	var zero T
	tx, err := r.beginTx(ctx)
	if err != nil {
		return zero, errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	P(&entity).SetID(generateID())
	if uuidable, ok := any(P(&entity)).(models.UUIDable); ok {
		uuidable.SetUUID(generateID())
	}

	if !r.service {
		P(&entity).SetTenantID(r.tenantID)
		P(&entity).SetGroupID(r.groupID)
		P(&entity).SetCreatedByUserID(r.createdByUserID)
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return zero, errxtrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.Insert(ctx, entity)
	if err != nil {
		return zero, errxtrace.Wrap("failed to insert entity", err)
	}

	return entity, nil
}

func (r *RLSGroupRepository[T, P]) Update(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx, T) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	if !r.service {
		P(&entity).SetTenantID(r.tenantID)
		P(&entity).SetGroupID(r.groupID)
	}

	field := Pair("id", P(&entity).GetID())

	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return errxtrace.Wrap("failed to scan entity", err)
	}

	if uuidable, ok := any(P(&entity)).(models.UUIDable); ok {
		if dbUuidable, ok := any(P(&dbEntity)).(models.UUIDable); ok {
			uuidable.SetUUID(dbUuidable.GetUUID())
		}
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx, dbEntity)
		if err != nil {
			return errxtrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.UpdateByField(ctx, field, entity)
	if err != nil {
		return errxtrace.Wrap("failed to update entity", err)
	}

	return nil
}

func (r *RLSGroupRepository[T, P]) Delete(ctx context.Context, id string, checkerFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", id)

	var entity T
	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.ScanOneByField(ctx, field, &entity)
	if err != nil {
		return errxtrace.Wrap("entity not found", err)
	}

	if checkerFn != nil {
		err = checkerFn(ctx, tx)
		if err != nil {
			return errxtrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	err = txreg.DeleteByField(ctx, field)
	if err != nil {
		return errxtrace.Wrap("failed to delete entity", err)
	}

	return nil
}

func (r *RLSGroupRepository[T, P]) Do(ctx context.Context, operationFn func(context.Context, *sqlx.Tx) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	if operationFn != nil {
		err = operationFn(ctx, tx)
		if err != nil {
			return errxtrace.Wrap("failed to call operationFn (RLSGroupRepository.Do)", err, errx.Attrs("entity_type", r.table))
		}
	}

	return nil
}

func (r *RLSGroupRepository[T, P]) DoWithEntityID(ctx context.Context, entityID string, operationFn func(context.Context, *sqlx.Tx, T) error) error {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to begin transaction", err)
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	field := Pair("id", entityID)

	var dbEntity T
	err = NewTxRegistry[T](tx, r.table).ScanOneByField(ctx, field, &dbEntity)
	if err != nil {
		return errxtrace.Wrap("failed to scan entity", err)
	}

	if operationFn != nil {
		err = operationFn(ctx, tx, dbEntity)
		if err != nil {
			return errxtrace.Wrap("failed to call operationFn (RLSGroupRepository.DoWithEntityID)", err, errx.Attrs("entity_type", r.table))
		}
	}

	return nil
}

func (r *RLSGroupRepository[T, P]) beginTx(ctx context.Context) (*sqlx.Tx, error) {
	if r.service {
		return beginServiceTx(ctx, r.dbx)
	}
	return beginTxWithTenantAndGroup(ctx, r.dbx, r.tenantID, r.groupID)
}
