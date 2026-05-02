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
	locations         registry.LocationRegistryFactory
	areas             registry.AreaRegistryFactory
	commodities       registry.CommodityRegistryFactory
	exports           registry.ExportRegistryFactory
	restoreOperations registry.RestoreOperationRegistryFactory
	restoreSteps      registry.RestoreStepRegistryFactory
	files             registry.FileRegistryFactory
	memberships       registry.GroupMembershipRegistry
}

// NewGroupPurger wires a GroupPurger to the registry factories that own the
// shared in-memory data maps. All parameters are required. The legacy
// commodity-scoped image/invoice/manual factories were dropped under #1421
// along with their SQL tables.
func NewGroupPurger(
	locations registry.LocationRegistryFactory,
	areas registry.AreaRegistryFactory,
	commodities registry.CommodityRegistryFactory,
	exports registry.ExportRegistryFactory,
	restoreOperations registry.RestoreOperationRegistryFactory,
	restoreSteps registry.RestoreStepRegistryFactory,
	files registry.FileRegistryFactory,
	memberships registry.GroupMembershipRegistry,
) *GroupPurger {
	return &GroupPurger{
		locations:         locations,
		areas:             areas,
		commodities:       commodities,
		exports:           exports,
		restoreOperations: restoreOperations,
		restoreSteps:      restoreSteps,
		files:             files,
		memberships:       memberships,
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
		{"files", func() error {
			reg := r.files.CreateServiceRegistry()
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
