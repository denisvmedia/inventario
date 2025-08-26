package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
)

func RollbackOrCommit(tx *sqlx.Tx, err error) error {
	if err != nil {
		return tx.Rollback()
	}
	return tx.Commit()
}

func setRole(ctx context.Context, tx *sqlx.Tx, role string) error {
	// PostgreSQL doesn't allow parameterized role names, so we use string formatting
	// Role names are controlled by the application, so this is safe
	query := fmt.Sprintf("SET LOCAL ROLE = %s", role)
	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return errkit.Wrap(err, "failed to set role", "role", role)
	}
	return nil
}

func setAppRole(ctx context.Context, tx *sqlx.Tx) error {
	return setRole(ctx, tx, "inventario_app")
}

func setBackgroundWorkerRole(ctx context.Context, tx *sqlx.Tx) error {
	return setRole(ctx, tx, "inventario_background_worker")
}

func setUserContext(ctx context.Context, tx *sqlx.Tx, userID string) error {
	// Escape single quotes in userID for safety
	escapedUserID := strings.ReplaceAll(userID, "'", "''")
	query := fmt.Sprintf("SET LOCAL app.current_user_id = '%s'", escapedUserID)
	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return errkit.Wrap(err, "failed to set user context", "user_id", userID)
	}
	return nil
}

func beginUserTx(ctx context.Context, dbx *sqlx.DB, userID string) (*sqlx.Tx, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	tx, err := dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}

	err = setAppRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, errkit.Wrap(err, "failed to set app role")
	}

	err = setUserContext(ctx, tx, userID)
	if err != nil {
		tx.Rollback()
		return nil, errkit.Wrap(err, "failed to set user context")
	}

	return tx, nil
}

func beginServiceTx(ctx context.Context, dbx *sqlx.DB) (*sqlx.Tx, error) {
	tx, err := dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}

	err = setBackgroundWorkerRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, errkit.Wrap(err, "failed to set background worker role")
	}

	return tx, nil
}

func entityToIDAble[T any](entity T) models.IDable {
	idable, ok := (any(entity)).(models.IDable)
	if ok {
		return idable
	}
	idable, ok = (any(&entity)).(models.IDable)
	if ok {
		return idable
	}
	panic("entity is not IDable")
}
