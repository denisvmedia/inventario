package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.SystemAdminGrantRegistry = (*SystemAdminGrantRegistry)(nil)

// SystemAdminGrantRegistry is the postgres-backed system-admin grant
// store (#1784). The table is NOT RLS-enabled — system-admin is a
// platform privilege orthogonal to tenants. All operations use
// NonRLSRepository.
//
// Mutating operations take pg_advisory_xact_lock('system_admin_mutations')
// — the SAME lock key the legacy users.is_system_admin path used. This
// keeps a rolling deploy race-free: even when half the replicas are on
// the old code path (reading users.is_system_admin under that lock) and
// half are on the new path (reading system_admin_grants under the same
// lock), the last-admin invariant still holds across the cutover.
type SystemAdminGrantRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewSystemAdminGrantRegistry creates a new SystemAdminGrantRegistry.
func NewSystemAdminGrantRegistry(dbx *sqlx.DB) *SystemAdminGrantRegistry {
	return NewSystemAdminGrantRegistryWithTableNames(dbx, store.DefaultTableNames)
}

// NewSystemAdminGrantRegistryWithTableNames is the test-friendly
// constructor that lets a caller override the table-name mapping.
func NewSystemAdminGrantRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *SystemAdminGrantRegistry {
	return &SystemAdminGrantRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *SystemAdminGrantRegistry) newSQLRegistry() *store.NonRLSRepository[models.SystemAdminGrant, *models.SystemAdminGrant] {
	return store.NewSQLRegistry[models.SystemAdminGrant](r.dbx, r.tableNames.SystemAdminGrants())
}

// Exists returns true when the user has a grant row. Hot path —
// RequireSystemAdmin runs this on every /api/v1/admin/* request.
// Backed by the unique index on user_id so the lookup is constant-time
// regardless of the total grant count.
func (r *SystemAdminGrantRegistry) Exists(ctx context.Context, userID string) (bool, error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var ok bool
	query := fmt.Sprintf(
		`SELECT EXISTS(SELECT 1 FROM %s WHERE user_id = $1)`,
		r.tableNames.SystemAdminGrants(),
	)
	if err := r.dbx.QueryRowContext(ctx, query, userID).Scan(&ok); err != nil {
		return false, errxtrace.Wrap("failed to check system_admin_grants existence", err)
	}
	return ok, nil
}

// Grant inserts a grant row. Idempotent: on duplicate (user already a
// system admin) returns (true, nil) — the unique index on user_id is
// the source of truth for the dedup, and ON CONFLICT DO NOTHING reads
// any inserts-not-performed as a successful idempotent call.
//
// The lock is held for the duration of the transaction so the unique
// constraint is the safety net of last resort, not the primary
// concurrency control. Two concurrent grants for the same user are
// serialised; the loser sees the existing row.
func (r *SystemAdminGrantRegistry) Grant(ctx context.Context, userID string, grantedBy *string) (hadGrant bool, err error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, lockErr := tx.ExecContext(ctx,
			`SELECT pg_advisory_xact_lock(hashtext('system_admin_mutations'))`,
		); lockErr != nil {
			return errxtrace.Wrap("failed to acquire system-admin advisory lock", lockErr)
		}

		// Check existing grant under the lock so the idempotent branch
		// returns a stable hadGrant=true rather than racing with another
		// grant call.
		var existingID string
		probe := fmt.Sprintf(
			`SELECT id FROM %s WHERE user_id = $1`,
			r.tableNames.SystemAdminGrants(),
		)
		switch scanErr := tx.QueryRowContext(ctx, probe, userID).Scan(&existingID); {
		case scanErr == nil:
			hadGrant = true
			return nil
		case errors.Is(scanErr, sql.ErrNoRows):
			// Fall through to the insert.
		default:
			return errxtrace.Wrap("failed to probe existing system_admin_grants row", scanErr)
		}

		insertQuery := fmt.Sprintf(
			`INSERT INTO %s (id, uuid, user_id, granted_by, granted_at)
			 VALUES ($1, $2, $3, $4, now())`,
			r.tableNames.SystemAdminGrants(),
		)
		if _, execErr := tx.ExecContext(ctx, insertQuery,
			uuid.New().String(), uuid.New().String(), userID, grantedBy,
		); execErr != nil {
			return errxtrace.Wrap("failed to insert system_admin_grants row", execErr)
		}
		return nil
	})
	if err != nil {
		return hadGrant, err
	}
	return hadGrant, nil
}

// RevokeAtomic deletes the grant row for the target user inside a
// transaction that also takes the global advisory lock. allowZero=false
// makes the count check (under the same lock) refuse to drop to zero
// grants — two concurrent revokes serialise; the loser sees count==1
// and gets ErrLastSystemAdmin.
//
//revive:disable-next-line:flag-parameter
func (r *SystemAdminGrantRegistry) RevokeAtomic(ctx context.Context, userID string, allowZero bool) (hadGrant bool, err error) {
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	reg := r.newSQLRegistry()
	err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if _, lockErr := tx.ExecContext(ctx,
			`SELECT pg_advisory_xact_lock(hashtext('system_admin_mutations'))`,
		); lockErr != nil {
			return errxtrace.Wrap("failed to acquire system-admin advisory lock", lockErr)
		}

		// FOR UPDATE pins the row so any concurrent revoke targeting
		// the same user blocks on us — defense-in-depth in case a
		// future code path bypasses this method.
		var grantID string
		probe := fmt.Sprintf(
			`SELECT id FROM %s WHERE user_id = $1 FOR UPDATE`,
			r.tableNames.SystemAdminGrants(),
		)
		switch scanErr := tx.QueryRowContext(ctx, probe, userID).Scan(&grantID); {
		case scanErr == nil:
			hadGrant = true
		case errors.Is(scanErr, sql.ErrNoRows):
			// Idempotent: no grant, nothing to do.
			return nil
		default:
			return errxtrace.Wrap("failed to probe system_admin_grants for revoke", scanErr)
		}

		if !allowZero {
			var count int
			countQuery := fmt.Sprintf(
				`SELECT COUNT(*) FROM %s`,
				r.tableNames.SystemAdminGrants(),
			)
			if countErr := tx.QueryRowContext(ctx, countQuery).Scan(&count); countErr != nil {
				return errxtrace.Wrap("failed to count system_admin_grants under lock", countErr)
			}
			if count <= 1 {
				return errxtrace.Classify(registry.ErrLastSystemAdmin, errx.Attrs("user_id", userID))
			}
		}

		deleteQuery := fmt.Sprintf(
			`DELETE FROM %s WHERE id = $1`,
			r.tableNames.SystemAdminGrants(),
		)
		if _, execErr := tx.ExecContext(ctx, deleteQuery, grantID); execErr != nil {
			return errxtrace.Wrap("failed to delete system_admin_grants row", execErr)
		}
		return nil
	})
	if err != nil {
		return hadGrant, err
	}
	return hadGrant, nil
}

// List returns every grant row, ordered by granted_at ASC. Backs the
// CLI list-system-admins command; the CLI joins each row to its user
// row separately so the registry stays focused on a single table.
func (r *SystemAdminGrantRegistry) List(ctx context.Context) ([]*models.SystemAdminGrant, error) {
	reg := r.newSQLRegistry()

	var grants []*models.SystemAdminGrant
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s ORDER BY granted_at ASC`,
			r.tableNames.SystemAdminGrants(),
		)
		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return errxtrace.Wrap("failed to list system_admin_grants", err)
		}
		defer rows.Close()
		for rows.Next() {
			var g models.SystemAdminGrant
			if err := rows.StructScan(&g); err != nil {
				return errxtrace.Wrap("failed to scan system_admin_grants row", err)
			}
			grants = append(grants, &g)
		}
		if err := rows.Err(); err != nil {
			return errxtrace.Wrap("failed during system_admin_grants iteration", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return grants, nil
}
