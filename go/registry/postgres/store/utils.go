package store

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
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
		return errxtrace.Wrap("failed to set role", err, errx.Attrs("role", role))
	}
	return nil
}

func setAppRole(ctx context.Context, tx *sqlx.Tx) error {
	return setRole(ctx, tx, "inventario_app")
}

func setBackgroundWorkerRole(ctx context.Context, tx *sqlx.Tx) error {
	return setRole(ctx, tx, "inventario_background_worker")
}

func setAdminRole(ctx context.Context, tx *sqlx.Tx) error {
	return setRole(ctx, tx, "inventario_admin")
}

func setUserContext(ctx context.Context, tx *sqlx.Tx, userID string) error {
	// Escape single quotes in userID for safety
	escapedUserID := strings.ReplaceAll(userID, "'", "''")
	query := fmt.Sprintf("SET LOCAL app.current_user_id = '%s'", escapedUserID)
	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return errxtrace.Wrap("failed to set user context", err, errx.Attrs("user_id", userID))
	}
	return nil
}

func setTenantContext(ctx context.Context, tx *sqlx.Tx, tenantID string) error {
	// Escape single quotes in tenantID for safety
	escapedTenantID := strings.ReplaceAll(tenantID, "'", "''")
	query := fmt.Sprintf("SET LOCAL app.current_tenant_id = '%s'", escapedTenantID)
	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return errxtrace.Wrap("failed to set tenant context", err, errx.Attrs("tenant_id", tenantID))
	}
	return nil
}

func beginTxWithTenantAndUser(ctx context.Context, dbx *sqlx.DB, userID, tenantID string) (*sqlx.Tx, error) {
	if userID == "" {
		return nil, ErrUserIDRequired
	}

	tx, err := dbx.Beginx()
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin transaction", err)
	}

	err = setAppRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set app role", err)
	}

	err = setTenantContext(ctx, tx, tenantID)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set tenant context", err)
	}

	err = setUserContext(ctx, tx, userID)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set user context", err)
	}

	return tx, nil
}

func beginServiceTx(ctx context.Context, dbx *sqlx.DB) (*sqlx.Tx, error) {
	tx, err := dbx.Beginx()
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin transaction", err)
	}

	err = setBackgroundWorkerRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set background worker role", err)
	}

	return tx, nil
}

// beginAdminTx begins a transaction that has SET LOCAL ROLE to
// inventario_admin. That role carries the BYPASSRLS attribute, so RLS
// policies are skipped entirely for the duration of the transaction —
// the cross-tenant admin surfaces (#1787) need to read and write rows
// in every tenant regardless of the per-tenant isolation policies.
//
// Crucially, BYPASSRLS lives ONLY on inventario_admin, never on
// inventario_app: a plain app request is still bound by the
// tenant-isolation policies because it never assumes this role. The
// override is scoped to the transaction by SET LOCAL ROLE and is
// reset automatically on commit or rollback.
func beginAdminTx(ctx context.Context, dbx *sqlx.DB) (*sqlx.Tx, error) {
	tx, err := dbx.Beginx()
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin transaction", err)
	}

	err = setAdminRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set admin role", err)
	}

	return tx, nil
}

func setGroupContext(ctx context.Context, tx *sqlx.Tx, groupID string) error {
	// Escape single quotes in groupID for safety
	escapedGroupID := strings.ReplaceAll(groupID, "'", "''")
	query := fmt.Sprintf("SET LOCAL app.current_group_id = '%s'", escapedGroupID)
	_, err := tx.ExecContext(ctx, query)
	if err != nil {
		return errxtrace.Wrap("failed to set group context", err, errx.Attrs("group_id", groupID))
	}
	return nil
}

func beginTxWithTenantAndGroup(ctx context.Context, dbx *sqlx.DB, tenantID, groupID string) (*sqlx.Tx, error) {
	if tenantID == "" {
		return nil, ErrTenantIDRequired
	}
	if groupID == "" {
		return nil, ErrGroupIDRequired
	}

	tx, err := dbx.Beginx()
	if err != nil {
		return nil, errxtrace.Wrap("failed to begin transaction", err)
	}

	err = setAppRole(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set app role", err)
	}

	err = setTenantContext(ctx, tx, tenantID)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set tenant context", err)
	}

	err = setGroupContext(ctx, tx, groupID)
	if err != nil {
		tx.Rollback()
		return nil, errxtrace.Wrap("failed to set group context", err)
	}

	return tx, nil
}

// generateID generates a new UUID string
func generateID() string {
	return uuid.New().String()
}

// DoAsBackgroundWorker runs fn inside a transaction that has SET LOCAL ROLE
// to inventario_background_worker, committing on nil return and rolling back
// otherwise. It is the public entry point for cross-table maintenance that
// legitimately needs to bypass the tenant-scoped RLS policies on
// inventario_app (group purge, housekeeping sweeps, etc.). New callers
// should be rare; prefer per-registry methods where possible.
func DoAsBackgroundWorker(ctx context.Context, dbx *sqlx.DB, fn func(context.Context, *sqlx.Tx) error) (err error) {
	tx, err := beginServiceTx(ctx, dbx)
	if err != nil {
		return errxtrace.Wrap("failed to begin background-worker transaction", err)
	}
	defer func() {
		if rbErr := RollbackOrCommit(tx, err); rbErr != nil && err == nil {
			err = errxtrace.Wrap("failed to finish background-worker transaction", rbErr)
		}
	}()

	if err = fn(ctx, tx); err != nil {
		return errxtrace.Wrap("background-worker operation failed", err)
	}
	return nil
}

// DoAsAdmin runs fn inside a transaction that has SET LOCAL ROLE to
// inventario_admin (a BYPASSRLS role), committing on nil return and
// rolling back otherwise. It is the entry point for the cross-tenant
// system-admin surfaces (#1787): listing every tenant's groups,
// loading a group/tenant/user detail row in another tenant, editing a
// group's membership as a system administrator, etc.
//
// Why a dedicated role rather than `SET LOCAL row_security = off`:
// Postgres raises SQLSTATE 42501 ("query would be affected by
// row-level security policy") when a query that WOULD be filtered by
// an RLS policy runs with row_security=off under a role that can
// neither own the table nor bypass RLS. inventario_app and
// inventario_background_worker are both non-bypass roles, so the old
// admin code tripped 42501 on every PostgreSQL deployment. Switching
// the active role to inventario_admin — which has BYPASSRLS — makes
// the cross-tenant reads/writes succeed without weakening the
// per-tenant isolation that normal inventario_app traffic relies on.
//
// New callers should be rare and gated behind RequireSystemAdmin.
func DoAsAdmin(ctx context.Context, dbx *sqlx.DB, fn func(context.Context, *sqlx.Tx) error) (err error) {
	tx, err := beginAdminTx(ctx, dbx)
	if err != nil {
		return errxtrace.Wrap("failed to begin admin transaction", err)
	}
	defer func() {
		if rbErr := RollbackOrCommit(tx, err); rbErr != nil && err == nil {
			err = errxtrace.Wrap("failed to finish admin transaction", rbErr)
		}
	}()

	if err = fn(ctx, tx); err != nil {
		return errxtrace.Wrap("admin operation failed", err)
	}
	return nil
}
