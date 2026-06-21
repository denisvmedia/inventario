package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/registry"
)

var _ registry.GroupPurger = (*GroupPurger)(nil)

// GroupPurger is the in-memory counterpart to postgres.GroupPurger. It
// deletes every group-scoped row whose (tenant_id, group_id) matches the
// request via each registry's service-mode view, in FK-safe order.
//
// It does NOT touch location_groups, group_invites, or group_invites_audit —
// the orchestration layer (services.GroupPurgeService) owns those.
type GroupPurger struct {
	locations            registry.LocationRegistryFactory
	areas                registry.AreaRegistryFactory
	commodities          registry.CommodityRegistryFactory
	commodityEvents      registry.CommodityEventRegistryFactory
	commodityLoans       registry.CommodityLoanRegistryFactory
	commodityServices    registry.CommodityServiceRegistryFactory
	supplyLinks          registry.SupplyLinkRegistryFactory
	tags                 registry.TagRegistryFactory
	exports              registry.ExportRegistryFactory
	restoreOperations    registry.RestoreOperationRegistryFactory
	restoreSteps         registry.RestoreStepRegistryFactory
	files                registry.FileRegistryFactory
	thumbnailJobs        registry.ThumbnailGenerationJobRegistryFactory
	concurrencySlots     registry.UserConcurrencySlotRegistryFactory
	maintenanceSchedules registry.MaintenanceScheduleRegistryFactory
	maintenanceReminders registry.MaintenanceReminderRegistry
	currencyMigrations   registry.CurrencyMigrationRegistryFactory
	notificationPrefs    registry.GroupNotificationPrefRegistry
	memberships          registry.GroupMembershipRegistry
}

// NewGroupPurger wires a GroupPurger to the registry factories that own the
// shared in-memory data maps. All parameters are required. The legacy
// commodity-scoped image/invoice/manual factories were dropped under #1421
// along with their SQL tables.
func NewGroupPurger(
	locations registry.LocationRegistryFactory,
	areas registry.AreaRegistryFactory,
	commodities registry.CommodityRegistryFactory,
	commodityEvents registry.CommodityEventRegistryFactory,
	commodityLoans registry.CommodityLoanRegistryFactory,
	commodityServices registry.CommodityServiceRegistryFactory,
	supplyLinks registry.SupplyLinkRegistryFactory,
	tags registry.TagRegistryFactory,
	exports registry.ExportRegistryFactory,
	restoreOperations registry.RestoreOperationRegistryFactory,
	restoreSteps registry.RestoreStepRegistryFactory,
	files registry.FileRegistryFactory,
	thumbnailJobs registry.ThumbnailGenerationJobRegistryFactory,
	concurrencySlots registry.UserConcurrencySlotRegistryFactory,
	maintenanceSchedules registry.MaintenanceScheduleRegistryFactory,
	maintenanceReminders registry.MaintenanceReminderRegistry,
	currencyMigrations registry.CurrencyMigrationRegistryFactory,
	notificationPrefs registry.GroupNotificationPrefRegistry,
	memberships registry.GroupMembershipRegistry,
) *GroupPurger {
	return &GroupPurger{
		locations:            locations,
		areas:                areas,
		commodities:          commodities,
		commodityEvents:      commodityEvents,
		commodityLoans:       commodityLoans,
		commodityServices:    commodityServices,
		supplyLinks:          supplyLinks,
		tags:                 tags,
		exports:              exports,
		restoreOperations:    restoreOperations,
		restoreSteps:         restoreSteps,
		files:                files,
		thumbnailJobs:        thumbnailJobs,
		concurrencySlots:     concurrencySlots,
		maintenanceSchedules: maintenanceSchedules,
		maintenanceReminders: maintenanceReminders,
		currencyMigrations:   currencyMigrations,
		notificationPrefs:    notificationPrefs,
		memberships:          memberships,
	}
}

// PurgeGroupDependents walks each group-scoped registry in FK-safe order and
// deletes every row whose (TenantID, GroupID) matches. Unlike the postgres
// variant, these deletes are not in a single transaction — memory mode is
// only used for tests where partial failure is acceptable.
func (r *GroupPurger) PurgeGroupDependents(ctx context.Context, tenantID, groupID string) error {
	if tenantID == "" {
		return errxtrace.Wrap("tenantID required", registry.ErrFieldRequired)
	}
	if groupID == "" {
		return errxtrace.Wrap("groupID required", registry.ErrFieldRequired)
	}

	type step struct {
		name string
		run  func() error
	}
	steps := []step{
		{"restore_steps", func() error {
			reg := r.restoreSteps.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		{"restore_operations", func() error {
			reg := r.restoreOperations.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		{"exports", func() error {
			reg := r.exports.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		// Thumbnail chain (#2117) dropped BEFORE files: neither
		// thumbnail_generation_jobs nor user_concurrency_slots carries a
		// group_id (postgres scopes them through files via subqueries), so
		// here we resolve the group's files first, then for each file delete
		// its concurrency slots (DeleteByJobID) before its job
		// (DeleteByFileID) — slots -> jobs -> files, matching the FK order.
		{"thumbnail_chain", func() error {
			return r.purgeThumbnailChain(ctx, tenantID, groupID)
		}},
		{"files", func() error {
			reg := r.files.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		// commodity_events purged before commodities so the FK CASCADE
		// (postgres) and the explicit-delete (memory) match the parity
		// the test surface expects.
		{"commodity_events", func() error {
			reg := r.commodityEvents.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		// Maintenance reminders (#1368) dropped before their parent
		// schedules so the explicit purge mirrors the postgres path
		// (which deletes reminders before schedules to keep tenant +
		// group scoping local rather than relying on the FK cascade).
		// The reminder registry is service-mode only — fan out via
		// DeleteBySchedule for every schedule that matches the
		// (tenant, group) pair.
		{"maintenance_reminders", func() error {
			scheduleReg := r.maintenanceSchedules.CreateServiceRegistry()
			schedules, listErr := scheduleReg.List(ctx)
			if listErr != nil {
				return listErr
			}
			for _, s := range schedules {
				if s == nil || s.TenantID != tenantID || s.GroupID != groupID {
					continue
				}
				if _, derr := r.maintenanceReminders.DeleteBySchedule(ctx, s.ID); derr != nil {
					return derr
				}
			}
			return nil
		}},
		// Maintenance schedules (#1368) dropped before commodities — FK
		// is ON DELETE CASCADE but we mirror the postgres purger which
		// deletes explicitly to keep tenant + group scoping local.
		{"maintenance_schedules", func() error {
			reg := r.maintenanceSchedules.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		// Currency-migration audit rows (#2095) dropped before the migration
		// rows. The audit slice is bespoke (not a generic registry), so use
		// the Piece-A service-mode DeleteAuditRowsByGroup which mirrors the
		// postgres explicit DELETE.
		{"currency_migration_audit_rows", func() error {
			reg := r.currencyMigrations.CreateServiceRegistry()
			_, derr := reg.DeleteAuditRowsByGroup(ctx, tenantID, groupID)
			return derr
		}},
		// Currency migrations (#2095). TenantGroupAware, so the generic
		// purge-by-tenant-group path applies.
		{"currency_migrations", func() error {
			reg := r.currencyMigrations.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		// Commodity sub-resources (#2095) dropped before commodities so the
		// explicit deletes mirror the postgres purger (which doesn't rely on
		// the commodities cascade for tenant + group scoping).
		{"commodity_supply_links", func() error {
			reg := r.supplyLinks.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		{"commodity_services", func() error {
			reg := r.commodityServices.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		{"commodity_loans", func() error {
			reg := r.commodityLoans.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		{"commodities", func() error {
			reg := r.commodities.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		{"areas", func() error {
			reg := r.areas.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		{"locations", func() error {
			reg := r.locations.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		// Tags (#2095). TenantGroupAware; dropped after the inventory rows
		// that referenced them, mirroring the postgres order.
		{"tags", func() error {
			reg := r.tags.CreateServiceRegistry()
			return purgeByTenantGroup(ctx, tenantID, groupID, reg.List, reg.Delete)
		}},
		// Per-user/per-group notification overrides (#2095). The registry is
		// service-mode only (not TenantGroupAware), so use the Piece-A
		// DeleteByGroup which mirrors the postgres parameterized DELETE.
		{"group_notification_prefs", func() error {
			_, derr := r.notificationPrefs.DeleteByGroup(ctx, tenantID, groupID)
			return derr
		}},
		{"group_memberships", func() error {
			return purgeMembershipsByTenantGroup(ctx, r.memberships, tenantID, groupID)
		}},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			return errxtrace.Wrap("failed to purge "+s.name, err)
		}
	}
	return nil
}

// purgeThumbnailChain removes the thumbnail generation jobs (and their
// concurrency slots) attached to the group's files (#2117). The job and slot
// tables have no group_id, so the group is resolved through its files: for each
// file we delete every job's concurrency slots (DeleteByJobID) before deleting
// the jobs (DeleteByFileID), mirroring the postgres slots -> jobs FK order. A
// file can own more than one job (a failed job plus a retry), so every job's
// slots must be cleared — not just one — or the leftover slots dangle. All
// deletes run before the files step in PurgeGroupDependents.
//
// Idempotent: a file with no thumbnail job is skipped, and the underlying
// DeleteBy* calls are no-ops on zero matches.
func (r *GroupPurger) purgeThumbnailChain(ctx context.Context, tenantID, groupID string) error {
	fileReg := r.files.CreateServiceRegistry()
	files, err := fileReg.ListByGroup(ctx, tenantID, groupID)
	if err != nil {
		return err
	}

	jobReg := r.thumbnailJobs.CreateServiceRegistry()
	slotReg := r.concurrencySlots.CreateServiceRegistry()

	for _, f := range files {
		if f == nil {
			continue
		}
		fileID := f.GetID()

		jobs, jobErr := jobReg.ListByFileID(ctx, fileID)
		if jobErr != nil {
			return jobErr
		}
		if len(jobs) == 0 {
			// No thumbnail job for this file — nothing to purge.
			continue
		}

		// Concurrency slots (deepest child) of every job before the jobs they
		// reference are removed.
		for _, job := range jobs {
			if err := slotReg.DeleteByJobID(ctx, job.ID); err != nil {
				return err
			}
		}
		if err := jobReg.DeleteByFileID(ctx, fileID); err != nil {
			return err
		}
	}

	return nil
}
