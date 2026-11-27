package store

import (
	"context"
	"errors"
	"iter"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
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

func (r *RLSGroupRepository[T, P]) ScanByFieldIn(ctx context.Context, field string, values []string) iter.Seq2[T, error] {
	return func(yield func(T, error) bool) {
		tx, err := r.beginTx(ctx)
		if err != nil {
			var zero T
			yield(zero, err)
			return
		}
		defer tx.Rollback()

		txreg := NewTxRegistry[T](tx, r.table)
		for entity, err := range txreg.ScanByFieldIn(ctx, field, values) {
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

func (r *RLSGroupRepository[T, P]) ExistsByFieldIn(ctx context.Context, field string, values []string) (bool, error) {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	return NewTxRegistry[T](tx, r.table).ExistsByFieldIn(ctx, field, values)
}

func (r *RLSGroupRepository[T, P]) ClaimByStatus(ctx context.Context, id, from, to string) (bool, error) {
	tx, err := r.beginTx(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck // the commit below is what matters

	claimed, err := NewTxRegistry[T](tx, r.table).ClaimByStatus(ctx, id, from, to)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, errxtrace.Wrap("failed to commit claim", err)
	}
	return claimed, nil
}

func (r *RLSGroupRepository[T, P]) Create(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx) error) (T, error) {
	var prepareFn func(context.Context, *sqlx.Tx, P) error
	if checkerFn != nil {
		prepareFn = func(ctx context.Context, tx *sqlx.Tx, _ P) error { return checkerFn(ctx, tx) }
	}
	return r.CreateWithPrepare(ctx, entity, prepareFn)
}

// CreateWithPrepare is Create with a hook that may change the entity: it runs
// inside the insert transaction after the ids are set and before the INSERT,
// and receives a pointer to the row about to be written. Use it for a column
// whose value has to be computed under the same transaction as the insert.
func (r *RLSGroupRepository[T, P]) CreateWithPrepare(ctx context.Context, entity T, prepareFn func(context.Context, *sqlx.Tx, P) error) (T, error) {
	return r.CreateWithHooks(ctx, entity, prepareFn, nil)
}

// CreateWithHooks is CreateWithPrepare with a second hook, afterFn, that runs
// in the same transaction after the INSERT and receives the inserted row.
// Use it for rows that reference the new one, which the database refuses
// before the row exists.
func (r *RLSGroupRepository[T, P]) CreateWithHooks(
	ctx context.Context, entity T, prepareFn, afterFn func(context.Context, *sqlx.Tx, P) error,
) (_ T, err error) {
	var zero T

	// Fail fast on a misconstructed user-scoped registry: tenant / group /
	// created_by_user_id are all required fields on the row and are also
	// required for the tx-level RLS context to be meaningful. Without this
	// guard, an empty-string constructor argument would bubble up as a
	// downstream NOT NULL / RLS failure with a confusing error.
	if !r.service {
		if r.tenantID == "" {
			return zero, ErrTenantIDRequired
		}
		if r.groupID == "" {
			return zero, ErrGroupIDRequired
		}
		if r.createdByUserID == "" {
			return zero, ErrCreatedByUserIDRequired
		}
	}

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

	if prepareFn != nil {
		err = prepareFn(ctx, tx, P(&entity))
		if err != nil {
			return zero, errxtrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.Insert(ctx, entity)
	if err != nil {
		return zero, errxtrace.Wrap("failed to insert entity", err)
	}

	if afterFn != nil {
		err = afterFn(ctx, tx, P(&entity))
		if err != nil {
			return zero, errxtrace.Wrap("failed to call after-insert function", err, errx.Attrs("entity_type", r.table))
		}
	}

	return entity, nil
}

func (r *RLSGroupRepository[T, P]) Update(ctx context.Context, entity T, checkerFn func(context.Context, *sqlx.Tx, T) error) error {
	var prepareFn func(context.Context, *sqlx.Tx, T, P) error
	if checkerFn != nil {
		prepareFn = func(ctx context.Context, tx *sqlx.Tx, dbEntity T, _ P) error { return checkerFn(ctx, tx, dbEntity) }
	}
	return r.UpdateWithPrepare(ctx, entity, prepareFn)
}

// UpdateWithPrepare is Update with a hook that may change the entity: it runs
// inside the update transaction before the UPDATE, receives the stored row and
// a pointer to the row about to be written, and can carry a server-managed
// column over from the database.
func (r *RLSGroupRepository[T, P]) UpdateWithPrepare(ctx context.Context, entity T, prepareFn func(context.Context, *sqlx.Tx, T, P) error) error {
	return r.UpdateWithHooks(ctx, entity, prepareFn, nil)
}

// UpdateWithHooks is UpdateWithPrepare with a second hook, afterFn, that runs
// in the same transaction after the UPDATE and receives the stored row as it
// was before and the row written.
func (r *RLSGroupRepository[T, P]) UpdateWithHooks(
	ctx context.Context, entity T, prepareFn, afterFn func(context.Context, *sqlx.Tx, T, P) error,
) (err error) {
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

	// created_by_user_id is an immutable audit field stamped at Create time.
	// Preserve the existing DB value so a caller can't accidentally clear or
	// change it via Update (which would break the NOT NULL + FK invariant and
	// lose provenance information).
	P(&entity).SetCreatedByUserID(P(&dbEntity).GetCreatedByUserID())

	if prepareFn != nil {
		err = prepareFn(ctx, tx, dbEntity, P(&entity))
		if err != nil {
			return errxtrace.Wrap("failed to call checker function", err, errx.Attrs("entity_type", r.table))
		}
	}

	txreg := NewTxRegistry[T](tx, r.table)
	err = txreg.UpdateByField(ctx, field, entity)
	if err != nil {
		return errxtrace.Wrap("failed to update entity", err)
	}

	if afterFn != nil {
		err = afterFn(ctx, tx, dbEntity, P(&entity))
		if err != nil {
			return errxtrace.Wrap("failed to call after-update function", err, errx.Attrs("entity_type", r.table))
		}
	}

	return nil
}

func (r *RLSGroupRepository[T, P]) Delete(ctx context.Context, id string, checkerFn func(context.Context, *sqlx.Tx) error) (err error) {
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

func (r *RLSGroupRepository[T, P]) Do(ctx context.Context, operationFn func(context.Context, *sqlx.Tx) error) (err error) {
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

func (r *RLSGroupRepository[T, P]) DoWithEntityID(ctx context.Context, entityID string, operationFn func(context.Context, *sqlx.Tx, T) error) (err error) {
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
