package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
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

// contentTablesByCreatedByUserID lists the group-scoped content tables that
// embed TenantGroupAwareEntityID — every one carries tenant_id, group_id and a
// NOT NULL created_by_user_id FK to users(id). location_groups is handled
// separately below (it has no group_id and uses the `created_by` column).
var contentTablesByCreatedByUserID = []func(t store.TableNames) string{
	func(t store.TableNames) string { return string(t.Commodities()) },
	func(t store.TableNames) string { return string(t.Files()) },
	func(t store.TableNames) string { return string(t.Areas()) },
	func(t store.TableNames) string { return string(t.Locations()) },
	func(t store.TableNames) string { return string(t.Exports()) },
	func(t store.TableNames) string { return string(t.Tags()) },
}

// HasRetainedOwnedContent runs one read-only transaction with an EXISTS query
// per content table plus one for location_groups. It returns true on the FIRST
// match (short-circuit), so a user who owns nothing outside their private groups
// pays the full set of cheap index-backed existence checks and a user who owns
// something stops early.
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
		// Group-scoped content authored by the user in a RETAINED group.
		for _, nameFn := range contentTablesByCreatedByUserID {
			table := nameFn(r.tableNames)
			found, err := r.existsOwnedContent(ctx, tx, table, tenantID, userID, purgedGroupIDs)
			if err != nil {
				return err
			}
			if found {
				owns = true
				return nil
			}
		}

		// location_groups the user CREATED that are NOT being purged. This table
		// has no group_id of its own — its primary key id IS the group id — so
		// the exclusion is on id, and the authorship column is `created_by`.
		found, err := r.existsRetainedCreatedGroup(ctx, tx, tenantID, userID, purgedGroupIDs)
		if err != nil {
			return err
		}
		owns = found
		return nil
	})
	if err != nil {
		return false, errxtrace.Wrap("failed to check retained owned content", err)
	}
	return owns, nil
}

// existsOwnedContent runs EXISTS on one content table for rows the user authored
// in a group outside purgedGroupIDs. The NOT IN list is built with positional
// placeholders so the query stays parameterised.
func (r *UserContentOwnershipChecker) existsOwnedContent(ctx context.Context, tx *sqlx.Tx, table, tenantID, userID string, purgedGroupIDs []string) (bool, error) {
	args := []any{tenantID, userID}
	groupExclusion, args := buildGroupExclusion("group_id", purgedGroupIDs, args)

	query := fmt.Sprintf(
		"SELECT EXISTS (SELECT 1 FROM %s WHERE tenant_id = $1 AND created_by_user_id = $2%s)",
		table, groupExclusion,
	)
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

// existsRetainedCreatedGroup runs EXISTS on location_groups for groups the user
// created (created_by) whose id is outside purgedGroupIDs.
func (r *UserContentOwnershipChecker) existsRetainedCreatedGroup(ctx context.Context, tx *sqlx.Tx, tenantID, userID string, purgedGroupIDs []string) (bool, error) {
	groups := string(r.tableNames.LocationGroups())
	args := []any{tenantID, userID}
	idExclusion, args := buildGroupExclusion("id", purgedGroupIDs, args)

	query := fmt.Sprintf(
		"SELECT EXISTS (SELECT 1 FROM %s WHERE tenant_id = $1 AND created_by = $2%s)",
		groups, idExclusion,
	)
	var exists bool
	if err := tx.QueryRowxContext(ctx, query, args...).Scan(&exists); err != nil {
		return false, errxtrace.Wrap(
			"failed to check retained created group",
			err,
			errx.Attrs("table", groups, "tenant_id", tenantID, "user_id", userID),
		)
	}
	return exists, nil
}

// buildGroupExclusion appends a ` AND <column> NOT IN ($3, $4, …)` clause for the
// given purged ids onto args, numbering placeholders from len(args)+1. When there
// are no ids to exclude it returns an empty clause and args unchanged, so the
// EXISTS query matches every owned row (the user purges nothing → any owned row
// is retained).
func buildGroupExclusion(column string, purgedGroupIDs []string, args []any) (string, []any) {
	if len(purgedGroupIDs) == 0 {
		return "", args
	}
	placeholders := make([]string, 0, len(purgedGroupIDs))
	for _, id := range purgedGroupIDs {
		args = append(args, id)
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
	}
	return fmt.Sprintf(" AND %s NOT IN (%s)", column, strings.Join(placeholders, ", ")), args
}
