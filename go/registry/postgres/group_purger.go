package postgres

import (
	"context"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.GroupPurger = (*GroupPurger)(nil)

// GroupPurger bulk-deletes all data rows belonging to a single LocationGroup
// in one background-worker transaction, honoring FK order so the DELETEs
// succeed without needing ON DELETE CASCADE on every child table.
//
// It does NOT touch location_groups, group_invites or group_invites_audit —
// the orchestration layer (services.GroupPurgeService) snapshots used invites,
// deletes invites, then drops the group row itself.
type GroupPurger struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewGroupPurger returns a GroupPurger bound to the default table names.
func NewGroupPurger(dbx *sqlx.DB) *GroupPurger {
	return NewGroupPurgerWithTableNames(dbx, store.DefaultTableNames)
}

// NewGroupPurgerWithTableNames returns a GroupPurger using a custom TableNames
// (used by tests that want to sandbox against renamed tables).
func NewGroupPurgerWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *GroupPurger {
	return &GroupPurger{dbx: dbx, tableNames: tableNames}
}

// purgeOrder is the FK-safe DELETE sequence. Each entry resolves to the
// fully-qualified name of a dependent table; PurgeGroupDependents then
// issues `DELETE FROM <table> WHERE tenant_id = $1 AND group_id = $2`
// against each one inside a single background-worker transaction, so the
// worker role can't accidentally wipe the wrong tenant.
var purgeOrder = []func(t store.TableNames) string{
	// Restore pipeline (deepest children first).
	func(t store.TableNames) string { return string(t.RestoreSteps()) },
	func(t store.TableNames) string { return string(t.RestoreOperations()) },
	func(t store.TableNames) string { return string(t.Exports()) },

	// Generic group-scoped files (linked by polymorphic entity_type/id, no FK chain).
	// (Legacy commodity-scoped images/invoices/manuals tables were dropped under #1421.)
	func(t store.TableNames) string { return string(t.Files()) },

	// Audit timelines belonging to commodities (#1450). FK to commodities is
	// ON DELETE CASCADE so this entry is technically redundant, but keep it
	// explicit so the order of operations matches the rest of the table:
	// the purge transaction shouldn't rely on cascade for tenant + group
	// scoping. Listed before commodities so the dependent rows are gone
	// before the parent DELETE runs.
	func(t store.TableNames) string { return string(t.CommodityEvents()) },

	// Warranty reminder idempotency rows (#1367). Same FK-cascade
	// rationale as commodity_events — the explicit DELETE keeps tenant +
	// group scoping in the purge transaction rather than relying on the
	// commodities cascade.
	func(t store.TableNames) string { return string(t.WarrantyReminders()) },

	// Storage quota reminder idempotency rows (#1585). Group-scoped
	// idempotency rows must drop before the parent location_groups row;
	// the orchestration layer hard-deletes the group itself, so the FK
	// on group_id is left without ON DELETE CASCADE.
	func(t store.TableNames) string { return string(t.StorageQuotaReminders()) },

	// Inventory hierarchy.
	func(t store.TableNames) string { return string(t.Commodities()) },
	func(t store.TableNames) string { return string(t.Areas()) },
	func(t store.TableNames) string { return string(t.Locations()) },

	// Memberships last — they don't block child deletes but are cheapest to
	// drop after everything else is already gone.
	func(t store.TableNames) string { return string(t.GroupMemberships()) },
}

// PurgeGroupDependents issues a DELETE ... WHERE tenant_id = $1 AND
// group_id = $2 against each dependent table in FK-safe order, inside a
// single background-worker transaction. All deletes succeed or none do —
// that's the whole reason this isn't per-registry: partial progress would
// leave orphaned rows that RLS hides from inventario_app entirely.
//
// Idempotent: a second call after a clean purge is a no-op (every DELETE
// affects zero rows).
func (r *GroupPurger) PurgeGroupDependents(ctx context.Context, tenantID, groupID string) error {
	if tenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	if groupID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	return store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		for _, nameFn := range purgeOrder {
			table := nameFn(r.tableNames)
			query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1 AND group_id = $2", table)
			if _, err := tx.ExecContext(ctx, query, tenantID, groupID); err != nil {
				return errxtrace.Wrap(
					"failed to purge group dependents",
					err,
					errx.Attrs("table", table, "tenant_id", tenantID, "group_id", groupID),
				)
			}
		}
		return nil
	})
}
