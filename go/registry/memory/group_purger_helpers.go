package memory

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// purgeByTenantGroup lists everything from a service-mode registry view,
// keeps only rows matching (tenantID, groupID), and deletes each. The
// service-mode List returns all rows (user/group filtering disabled), so
// filtering happens here using the entity's TenantGroupAware interface.
func purgeByTenantGroup[T any](
	ctx context.Context,
	tenantID, groupID string,
	list func(context.Context) ([]*T, error),
	del func(context.Context, string) error,
) error {
	items, err := list(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		ga, ok := any(item).(models.TenantGroupAware)
		if !ok {
			continue
		}
		if ga.GetTenantID() != tenantID || ga.GetGroupID() != groupID {
			continue
		}
		idable, ok := any(item).(models.IDable)
		if !ok {
			continue
		}
		if err := del(ctx, idable.GetID()); err != nil {
			return err
		}
	}
	return nil
}

// purgeMembershipsByTenantGroup removes group_membership rows for the given
// (tenant, group). GroupMembership is TenantOnly (not TenantGroupAware), so
// it needs its own filter path.
func purgeMembershipsByTenantGroup(ctx context.Context, reg registry.GroupMembershipRegistry, tenantID, groupID string) error {
	items, err := reg.ListByGroup(ctx, groupID)
	if err != nil {
		return err
	}
	for _, m := range items {
		if m.TenantID != tenantID {
			continue
		}
		if err := reg.Delete(ctx, m.ID); err != nil {
			return err
		}
	}
	return nil
}
