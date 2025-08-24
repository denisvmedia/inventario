package store

import (
	"context"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
)

func RollbackOrCommit(tx *sqlx.Tx, err error) error {
	if err != nil {
		return tx.Rollback()
	}
	return tx.Commit()
}

func setRole(ctx context.Context, tx *sqlx.Tx, role string) error {
	_, err := tx.ExecContext(ctx, "SET LOCAL ROLE = $1", role)
	return errkit.Wrap(err, "failed to set role", "role", role)
}

func setAppRole(ctx context.Context, tx *sqlx.Tx) error {
	return setRole(ctx, tx, "inventario_app")
}

func setUserContext(ctx context.Context, tx *sqlx.Tx, userID string) error {
	_, err := tx.ExecContext(ctx, "SET LOCAL app.current_user_id = $1", userID)
	return errkit.Wrap(err, "failed to set user context", "user_id", userID)
}
