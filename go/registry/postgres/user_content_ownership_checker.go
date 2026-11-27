package postgres

import (
	"context"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

var _ registry.UserContentOwnershipChecker = (*UserContentOwnershipChecker)(nil)

// UserContentOwnershipChecker is the postgres implementation of the read-only
// account-deletion pre-check (#2147). It runs a handful of EXISTS queries in one
// read-only background-worker transaction to decide whether the user still owns
// content that would survive deleting only their private groups.
//
// It mutates nothing — it is deliberately separate from UserPurger so the
// account-deletion orchestration can abort BEFORE any purge runs.
type UserContentOwnershipChecker struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewUserContentOwnershipChecker returns a checker bound to the default table
// names.
func NewUserContentOwnershipChecker(dbx *sqlx.DB) *UserContentOwnershipChecker {
	return NewUserContentOwnershipCheckerWithTableNames(dbx, store.DefaultTableNames)
}

// NewUserContentOwnershipCheckerWithTableNames returns a checker using a custom
// TableNames (used by tests that sandbox against renamed tables).
func NewUserContentOwnershipCheckerWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *UserContentOwnershipChecker {
	return &UserContentOwnershipChecker{dbx: dbx, tableNames: tableNames}
}

// HasRetainedOwnedContent runs one read-only transaction with an EXISTS query
// per userOwnedTables entry whose rows block a deletion. It returns true on the
// FIRST match (short-circuit), so a user who owns nothing outside their private
// groups pays the full set of cheap index-backed existence checks and a user
// who owns something stops early.
func (r *UserContentOwnershipChecker) HasRetainedOwnedContent(ctx context.Context, tenantID, userID string, purgedGroupIDs []string) (bool, error) {
	if tenantID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if userID == "" {
		return false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var owns bool
	// DoAsBackgroundWorker is the established cross-table seam that bypasses the
	// per-tenant RLS policies (the user's own groups are visible under RLS, but
	// reading across the full owned set in one place is simplest under the
	// worker role). This transaction issues only EXISTS reads — no writes — so
	// the commit is a harmless no-op.
	err := store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Rows the user authored in a RETAINED group, from the shared list of
		// tables that tie a user to their tenant. location_groups is on it with
		// its own id as the group column: a group's primary key IS the group id.
		for _, t := range userOwnedTables {
			if t.onDeletion != deletionBlocks {
				continue
			}
			found, err := r.existsOwnedContent(ctx, tx, t, tenantID, userID, purgedGroupIDs)
			if err != nil {
				return err
			}
			if found {
				owns = true
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to check retained owned content", err)
	}
	return owns, nil
}

// existsOwnedContent runs EXISTS on one table for rows the user authored in a
// group outside purgedGroupIDs.
func (r *UserContentOwnershipChecker) existsOwnedContent(ctx context.Context, tx *sqlx.Tx, t userOwnedTable, tenantID, userID string, purgedGroupIDs []string) (bool, error) {
	table := string(t.table(r.tableNames))
	query, args := ownedRowExistsQuery(table, t.userColumn, t.groupColumn, tenantID, userID, purgedGroupIDs)
	var exists bool
	if err := tx.QueryRowxContext(ctx, query, args...).Scan(&exists); err != nil {
		return false, errxtrace.Wrap(
			"failed to check owned content",
			err,
			errx.Attrs("table", table, "tenant_id", tenantID, "user_id", userID),
		)
	}
	return exists, nil
}

// ownedRowExistsQuery asks whether table holds a row of the tenant whose
// authorColumn is the user and whose groupColumn is not one of purgedGroupIDs.
// With no ids to exclude it matches every owned row: the user purges nothing,
// so any owned row is retained.
func ownedRowExistsQuery(table, authorColumn, groupColumn, tenantID, userID string, purgedGroupIDs []string) (string, []any) {
	owned := newSelect().Select("1").From(table)
	owned.Where(owned.Equal("tenant_id", tenantID), owned.Equal(authorColumn, userID))
	if len(purgedGroupIDs) > 0 {
		owned.Where(owned.NotIn(groupColumn, anyStrings(purgedGroupIDs)...))
	}
	sb := newSelect()
	return sb.Select(sb.Exists(owned)).Build()
}
