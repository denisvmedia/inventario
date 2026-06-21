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

var _ registry.TenantPurger = (*TenantPurger)(nil)

// TenantPurger bulk-deletes every tenant-scoped dependent row belonging to a
// single Tenant in one background-worker transaction, honoring FK order so the
// DELETEs succeed without needing ON DELETE CASCADE on every child table.
//
// It is the tenant-level analogue of GroupPurger (#2095/#2117). Where
// GroupPurger clears one group's subtree, TenantPurger clears the union of
// every group's data PLUS the tenant-only rows that GroupPurger never touches
// (users, settings, refresh_tokens, login_events, the auth-token tables,
// location_groups themselves, …).
//
// It does NOT delete the tenants row itself — the orchestration layer
// (services.admin tenant hard-delete) drops it after the dependents are gone,
// exactly as GroupPurger leaves location_groups to its service. Issue #2115:
// before this, DeleteTenant was a bare `DELETE FROM tenants` that every one of
// the ~35 NO ACTION child FKs rejected.
type TenantPurger struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// NewTenantPurger returns a TenantPurger bound to the default table names.
func NewTenantPurger(dbx *sqlx.DB) *TenantPurger {
	return NewTenantPurgerWithTableNames(dbx, store.DefaultTableNames)
}

// NewTenantPurgerWithTableNames returns a TenantPurger using a custom
// TableNames (used by tests that want to sandbox against renamed tables).
func NewTenantPurgerWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *TenantPurger {
	return &TenantPurger{dbx: dbx, tableNames: tableNames}
}

// tenantPurgeOrder is the FK-safe DELETE sequence for tenant-scoped tables that
// carry a tenant_id column. Each entry resolves to the fully-qualified table
// name; PurgeTenantDependents issues `DELETE FROM <table> WHERE tenant_id = $1`
// against each one inside a single background-worker transaction.
//
// Ordering rules (every cross-table FK in the schema is NO ACTION unless noted,
// so the child must be deleted before its parent):
//
//   - restore_steps -> restore_operations -> exports -> files (the export /
//     restore pipeline; each FK is NO ACTION).
//   - user_concurrency_slots -> thumbnail_generation_jobs -> files (the
//     thumbnail chain; both FK to files/jobs are NO ACTION). Unlike GroupPurger
//     these tables carry a tenant_id, so they ride the same WHERE template and
//     no subquery scoping is needed.
//   - commodity sub-resources (events/loans/services/supply_links, the two
//     maintenance tables, warranty_reminders, currency_migration_audit_rows)
//     before their parents. Their FK to commodities/schedules/migrations is
//     ON DELETE CASCADE, but the explicit DELETE keeps tenant scoping local to
//     this transaction rather than relying on a cascade.
//   - commodities -> areas -> locations (inventory hierarchy, NO ACTION).
//   - currency_migration_audit_rows -> currency_migrations (NO ACTION on the
//     audit row's group_id; CASCADE on migration_id).
//   - everything group-scoped before location_groups, which is appended at the
//     very end of PurgeTenantDependents (group_id -> location_groups is NO
//     ACTION everywhere). location_groups.currency_migration_id ->
//     currency_migrations is SET NULL, so currency_migrations may precede it.
//   - users is NOT in this slice; it is deleted LAST (see PurgeTenantDependents)
//     because virtually every tenant-scoped table FKs user_id/created_by ->
//     users NO ACTION, and location_groups.created_by -> users NO ACTION too.
//
// The settings and commodity_scan_audits tables have no FK to the rows below
// and no children, so their position within the data block is unconstrained.
var tenantPurgeOrder = []func(t store.TableNames) string{
	// Restore pipeline (deepest children first): steps -> operations ->
	// exports. exports.file_id -> files is NO ACTION, so exports drop before
	// files (which is later in this slice).
	func(t store.TableNames) string { return string(t.RestoreSteps()) },
	func(t store.TableNames) string { return string(t.RestoreOperations()) },
	func(t store.TableNames) string { return string(t.Exports()) },

	// Thumbnail chain (#2117). Both tables carry a tenant_id (unlike the
	// group-scoped purge, which had to subquery through files), so the plain
	// WHERE tenant_id = $1 template works. slots -> jobs -> files.
	func(t store.TableNames) string { return string(t.UserConcurrencySlots()) },
	func(t store.TableNames) string { return string(t.ThumbnailGenerationJobs()) },

	// Generic polymorphic files (linked by entity_type/id, no FK chain). Must
	// drop after exports + the thumbnail chain (both FK to files NO ACTION) and
	// after commodities.cover_file_id (SET NULL — harmless either way).
	func(t store.TableNames) string { return string(t.Files()) },

	// Commodity audit timelines (#1450). FK to commodities is CASCADE; explicit
	// DELETE keeps tenant scoping local.
	func(t store.TableNames) string { return string(t.CommodityEvents()) },

	// Warranty reminder idempotency rows (#1367). commodity_id -> commodities
	// CASCADE; dropped before commodities anyway to keep scoping local.
	func(t store.TableNames) string { return string(t.WarrantyReminders()) },

	// Storage quota reminder idempotency rows (#1585). No commodity FK;
	// group_id -> location_groups NO ACTION and created_by_user_id -> users
	// NO ACTION, so they must drop before location_groups and users (both
	// appended at the end of PurgeTenantDependents).
	func(t store.TableNames) string { return string(t.StorageQuotaReminders()) },

	// Maintenance reminder idempotency rows (#1368). schedule_id ->
	// maintenance_schedules CASCADE; dropped before the schedules.
	func(t store.TableNames) string { return string(t.MaintenanceReminders()) },

	// Maintenance schedules (#1368). commodity_id -> commodities CASCADE;
	// dropped before commodities.
	func(t store.TableNames) string { return string(t.MaintenanceSchedules()) },

	// Currency-migration audit rows (#2095). migration_id ->
	// currency_migrations CASCADE, commodity_id -> commodities SET NULL.
	// Dropped before both currency_migrations and commodities.
	func(t store.TableNames) string { return string(t.CurrencyMigrationAudit()) },

	// Commodity sub-resources (#2095). All FK commodities ON DELETE CASCADE;
	// explicit DELETE before commodities keeps tenant scoping local.
	func(t store.TableNames) string { return string(t.CommoditySupplyLinks()) },
	func(t store.TableNames) string { return string(t.CommodityServices()) },
	func(t store.TableNames) string { return string(t.CommodityLoans()) },

	// Currency migrations (#2095). group_id -> location_groups NO ACTION, so
	// they must drop before location_groups (appended at the end). The
	// location_groups.currency_migration_id back-ref is SET NULL.
	func(t store.TableNames) string { return string(t.CurrencyMigrations()) },

	// AI photo-scan audit rows (#1720). tenant_id + user_id only (no group_id);
	// no children, so unconstrained — drop before users (appended at the end).
	func(t store.TableNames) string { return string(t.CommodityScanAudits()) },

	// Inventory hierarchy: commodities -> areas -> locations (NO ACTION).
	func(t store.TableNames) string { return string(t.Commodities()) },
	func(t store.TableNames) string { return string(t.Areas()) },
	func(t store.TableNames) string { return string(t.Locations()) },

	// Tags (#2095). tenant_id + group_id, no incoming FK left now that the
	// inventory rows that referenced them are gone.
	func(t store.TableNames) string { return string(t.Tags()) },

	// Per-user/per-group notification overrides (#2095). group_id ->
	// location_groups NO ACTION; cleared before location_groups.
	func(t store.TableNames) string { return string(t.GroupNotificationPrefs()) },

	// Installation settings rows. One per (tenant, key); no children, no
	// incoming FK — unconstrained.
	func(t store.TableNames) string { return string(t.Settings()) },

	// Audit logs (nullable tenant_id, no FK to tenants). Tenant-scoped data
	// that should not outlive the tenant. No children.
	func(t store.TableNames) string { return string(t.AuditLogs()) },

	// Auth/session tables (all FK user_id -> users NO ACTION, so they must
	// drop before users; none FK each other). login_events, refresh_tokens,
	// the token tables, MFA + OAuth identities, operation slots.
	func(t store.TableNames) string { return string(t.LoginEvents()) },
	func(t store.TableNames) string { return string(t.RefreshTokens()) },
	func(t store.TableNames) string { return string(t.EmailVerifications()) },
	func(t store.TableNames) string { return string(t.PasswordResets()) },
	func(t store.TableNames) string { return string(t.MagicLinkTokens()) },
	func(t store.TableNames) string { return string(t.UserMFASecrets()) },
	func(t store.TableNames) string { return string(t.UserOAuthIdentities()) },
	func(t store.TableNames) string { return string(t.OperationSlots()) },

	// Membership + invite rows. group_id -> location_groups NO ACTION and
	// created_by/used_by/member_user_id -> users NO ACTION, so they drop before
	// both location_groups and users.
	func(t store.TableNames) string { return string(t.GroupMemberships()) },
	func(t store.TableNames) string { return string(t.GroupInvites()) },
	func(t store.TableNames) string { return string(t.GroupInvitesAudit()) },
}

// PurgeTenantDependents issues a DELETE ... WHERE tenant_id = $1 against each
// dependent table in FK-safe order, inside a single background-worker
// transaction. All deletes succeed or none do — partial progress would leave
// orphaned rows that RLS hides from inventario_app entirely, and would still
// block the final tenants DELETE the orchestration layer runs afterwards.
//
// location_groups and users are deleted last (in that order): location_groups
// because every group-scoped row and currency_migrations FK it NO ACTION (and
// it FKs users NO ACTION via created_by), and users because virtually every
// tenant-scoped table FKs user_id/created_by -> users NO ACTION. The tenants
// row itself is left for the caller to drop.
//
// Idempotent: a second call after a clean purge is a no-op (every DELETE
// affects zero rows).
func (r *TenantPurger) PurgeTenantDependents(ctx context.Context, tenantID string) error {
	if tenantID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	return store.DoAsBackgroundWorker(ctx, r.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
		// Data + auth tables first, in FK-safe order.
		for _, nameFn := range tenantPurgeOrder {
			if err := r.purgeTable(ctx, tx, nameFn(r.tableNames), tenantID); err != nil {
				return err
			}
		}

		// location_groups next: now that every group-scoped row and
		// currency_migrations is gone, the only remaining inbound FKs are
		// users.default_group_id (SET NULL) — harmless — so the parent rows
		// can be removed. location_groups.created_by -> users is NO ACTION, so
		// this must precede the users DELETE below.
		if err := r.purgeTable(ctx, tx, string(r.tableNames.LocationGroups()), tenantID); err != nil {
			return err
		}

		// users last: almost every tenant-scoped table FKs user_id/created_by
		// -> users NO ACTION, so the rows referencing them must all be gone by
		// now. users.default_group_id was cleared (SET NULL) by the
		// location_groups DELETE above.
		if err := r.purgeTable(ctx, tx, string(r.tableNames.Users()), tenantID); err != nil {
			return err
		}

		return nil
	})
}

// purgeTable runs a single `DELETE FROM <table> WHERE tenant_id = $1` and wraps
// any error with the offending table + tenant for diagnosis.
func (r *TenantPurger) purgeTable(ctx context.Context, tx *sqlx.Tx, table, tenantID string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE tenant_id = $1", table)
	if _, err := tx.ExecContext(ctx, query, tenantID); err != nil {
		return errxtrace.Wrap(
			"failed to purge tenant dependents",
			err,
			errx.Attrs("table", table, "tenant_id", tenantID),
		)
	}
	return nil
}
