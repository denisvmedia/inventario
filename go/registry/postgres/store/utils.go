package store

import (
	"context"
	"fmt"
	"strings"

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
