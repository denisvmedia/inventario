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

	// Maintenance reminder idempotency rows (#1368). FK to
	// maintenance_schedules is ON DELETE CASCADE but the explicit
	// DELETE keeps tenant + group scoping in the purge transaction
	// rather than relying on the schedules cascade.
	func(t store.TableNames) string { return string(t.MaintenanceReminders()) },

	// Maintenance schedules (#1368). Dropped before commodities so the
	// FK cascade isn't relied on for tenant + group scoping.
	func(t store.TableNames) string { return string(t.MaintenanceSchedules()) },

	// Currency-migration audit rows (#2095). Their FK to currency_migrations
	// is ON DELETE CASCADE, but group_id -> location_groups is NO ACTION, so
	// the purge must clear them explicitly. Dropped BEFORE currency_migrations
	// so the explicit DELETE owns tenant + group scoping rather than relying
	// on the migration cascade.
	func(t store.TableNames) string { return string(t.CurrencyMigrationAudit()) },

	// Currency migrations (#2095). group_id -> location_groups is NO ACTION;
	// location_groups.currency_migration_id is a SET NULL back-ref, so
	// deleting these rows before the group is harmless.
	func(t store.TableNames) string { return string(t.CurrencyMigrations()) },

	// Commodity sub-resources (#2095). All three FK to commodities ON DELETE
	// CASCADE but group_id -> location_groups is NO ACTION. Dropped before
	// commodities so the explicit DELETE keeps tenant + group scoping local
	// rather than relying on the commodities cascade.
	func(t store.TableNames) string { return string(t.CommoditySupplyLinks()) },
	func(t store.TableNames) string { return string(t.CommodityServices()) },
	func(t store.TableNames) string { return string(t.CommodityLoans()) },

	// Inventory hierarchy.
	func(t store.TableNames) string { return string(t.Commodities()) },
	func(t store.TableNames) string { return string(t.Areas()) },
	func(t store.TableNames) string { return string(t.Locations()) },

	// Tags (#2095). group_id -> location_groups is NO ACTION. Dropped after
	// commodities/files because the join rows that referenced them are gone
	// by now; the tags table itself is plain (tenant_id + group_id).
	func(t store.TableNames) string { return string(t.Tags()) },

	// Per-user/per-group notification overrides (#2095). group_id ->
	// location_groups is NO ACTION, so they must be cleared explicitly
	// before the orchestration layer drops the group row.
	func(t store.TableNames) string { return string(t.GroupNotificationPrefs()) },

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
		// Thumbnail chain (#2117). thumbnail_generation_jobs has no group_id
		// (only tenant_id + user_id), so it can't ride the WHERE tenant_id =
		// $1 AND group_id = $2 template. Its FK to files is NO ACTION, and
		// user_concurrency_slots.job_id -> thumbnail_generation_jobs is also
		// NO ACTION, so both must be cleared explicitly BEFORE the files
		// DELETE below. Order: slots (deepest child) -> jobs -> files.
		if err := r.purgeThumbnailChain(ctx, tx, tenantID, groupID); err != nil {
			return err
		}

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

// purgeThumbnailChain clears the thumbnail generation jobs (and their
// concurrency slots) attached to the group's files (#2117). Neither
// thumbnail_generation_jobs nor user_concurrency_slots carries a group_id,
// so they are scoped through the files they reference. Both DELETEs run in
// the caller's background-worker transaction BEFORE the files DELETE, in
// FK-safe order (slots -> jobs), so the files table can be wiped without
// tripping the NO ACTION FKs.
//
// Idempotent: when the group has no files (or none with a thumbnail job)
// every DELETE matches zero rows and returns nil.
func (r *GroupPurger) purgeThumbnailChain(ctx context.Context, tx *sqlx.Tx, tenantID, groupID string) error {
	files := string(r.tableNames.Files())
	jobs := string(r.tableNames.ThumbnailGenerationJobs())
	slots := string(r.tableNames.UserConcurrencySlots())

	fileSubquery := fmt.Sprintf(
		"SELECT id FROM %s WHERE tenant_id = $1 AND group_id = $2", files,
	)

	slotQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE job_id IN (SELECT id FROM %s WHERE file_id IN (%s))",
		slots, jobs, fileSubquery,
	)
	if _, err := tx.ExecContext(ctx, slotQuery, tenantID, groupID); err != nil {
		return errxtrace.Wrap(
			"failed to purge group thumbnail concurrency slots",
			err,
			errx.Attrs("table", slots, "tenant_id", tenantID, "group_id", groupID),
		)
	}

	jobQuery := fmt.Sprintf(
		"DELETE FROM %s WHERE file_id IN (%s)", jobs, fileSubquery,
	)
	if _, err := tx.ExecContext(ctx, jobQuery, tenantID, groupID); err != nil {
		return errxtrace.Wrap(
			"failed to purge group thumbnail generation jobs",
			err,
			errx.Attrs("table", jobs, "tenant_id", tenantID, "group_id", groupID),
		)
	}

	return nil
}
