package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-extras/errx"
	"github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
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
		return stacktrace.Wrap("failed to set role", err, errx.Attrs("role", role))
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
		return stacktrace.Wrap("failed to set user context", err, errx.Attrs("user_id", userID))
	}
	return nil
}

func setTenantContext(ctx context.Context, tx *sqlx.Tx, tenantID string) error {
	// Escape single quotes in tenantID for safety
	escapedTenantID := strings.ReplaceAll(tenantID, "'", "''")
	query := fmt.Sprintf("SET LOCAL app.current_tenant_id = '%s'", escapedTenantID)
	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return stacktrace.Wrap("failed to set tenant context", err, errx.Attrs("tenant_id", tenantID))
	}
	return nil
}

func beginTxWithTenantAndUser(ctx context.Context, dbx *sqlx.DB, userID, tenantID string) (*sqlx.Tx, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	tx, err := dbx.Beginx()
	if err != nil {
		return nil, stacktrace.Wrap("failed to begin transaction", err)
	}

	err = setAppRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, stacktrace.Wrap("failed to set app role", err)
	}

	err = setTenantContext(ctx, tx, tenantID)
	if err != nil {
		tx.Rollback()
		return nil, stacktrace.Wrap("failed to set tenant context", err)
	}

	err = setUserContext(ctx, tx, userID)
	if err != nil {
		tx.Rollback()
		return nil, stacktrace.Wrap("failed to set user context", err)
	}

	return tx, nil
}

func beginServiceTx(ctx context.Context, dbx *sqlx.DB) (*sqlx.Tx, error) {
	tx, err := dbx.Beginx()
	if err != nil {
		return nil, stacktrace.Wrap("failed to begin transaction", err)
	}

	err = setBackgroundWorkerRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, stacktrace.Wrap("failed to set background worker role", err)
	}

	return tx, nil
}

// generateID generates a new UUID string
func generateID() string {
	return uuid.New().String()
}
