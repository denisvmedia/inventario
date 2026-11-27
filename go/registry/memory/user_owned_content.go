package memory

import (
	"context"

	"go.5x5.cz/inventario/registry"
)

// ownedSource is one store whose rows name a user as their member or author,
// the in-memory counterpart of postgres.userOwnedTables. The account-deletion
// pre-check reads the sources whose rows block a deletion; the tenant move
// counts all of them.
type ownedSource struct {
	kind       registry.OwnedContentKind
	onDeletion accountDeletion
	rows       func(context.Context) ([]createdByEntry, error)
}

// accountDeletion is what deleting an account does with a row that names the
// user in a group that stays, as postgres.accountDeletion.
type accountDeletion int

const (
	// deletionBlocks marks the user's content; the pre-check refuses the
	// deletion while a row exists in a group that stays.
	deletionBlocks accountDeletion = iota + 1
	// deletionRemoves marks personal rows, which UserPurger deletes.
	deletionRemoves
	// deletionHandsOver marks rows of the group, which UserPurger gives to
	// another member (see UserPurger.SetGroupRowStores).
	deletionHandsOver
)

// ownedSources lists the stores the memory backend can enumerate. Several
// tables the postgres list covers have no enumerable in-memory store and are
// not counted here: file links (kept on the file rows), the reminder send
// records (warranty, storage quota, maintenance, item), intake sources and
// candidates (kept inside their session), item type field links and currency
// change audit rows. The postgres backend is the one production runs.
func ownedSources(fs *registry.FactorySet) []ownedSource {
	return []ownedSource{
		{registry.OwnedGroupMemberships, deletionRemoves, membershipRows(fs)},
		{registry.OwnedLocationGroups, deletionBlocks, createdGroupRows(fs)},
		{registry.OwnedGroupInvites, deletionRemoves, inviteRows(fs)},
		{registry.OwnedCommodities, deletionBlocks, func(ctx context.Context) ([]createdByEntry, error) {
			reg := fs.CommodityRegistryFactory.CreateServiceRegistry()
			return authoredRows(ctx, serviceRows(reg, reg.List))
		}},
		{registry.OwnedFiles, deletionBlocks, func(ctx context.Context) ([]createdByEntry, error) {
			reg := fs.FileRegistryFactory.CreateServiceRegistry()
			return authoredRows(ctx, serviceRows(reg, reg.List))
		}},
		{registry.OwnedLocations, deletionBlocks, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.LocationRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedAreas, deletionBlocks, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.AreaRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedTags, deletionBlocks, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.TagRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedSellers, deletionBlocks, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.SellerRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedExports, deletionBlocks, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.ExportRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedCommodityEvents, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.CommodityEventRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedCommodityLoans, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			reg := fs.CommodityLoanRegistryFactory.CreateServiceRegistry()
			return authoredRows(ctx, serviceRows(reg, reg.List))
		}},
		{registry.OwnedCommodityServices, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			reg := fs.CommodityServiceRegistryFactory.CreateServiceRegistry()
			return authoredRows(ctx, serviceRows(reg, reg.List))
		}},
		{registry.OwnedCommoditySupplyLinks, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.SupplyLinkRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedMaintenanceSchedules, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			reg := fs.MaintenanceScheduleRegistryFactory.CreateServiceRegistry()
			return authoredRows(ctx, serviceRows(reg, reg.List))
		}},
		{registry.OwnedCommodityReminders, deletionRemoves, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.CommodityReminderRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedCommodityDrafts, deletionRemoves, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.CommodityDraftRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedIntakeSessions, deletionRemoves, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.IntakeRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedIntakeDocuments, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.IntakeDocumentRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedItemTypes, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.ItemTypeRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedCustomFieldDefinitions, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.CustomFieldRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedSavedViews, deletionRemoves, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.SavedViewRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedCurrencyMigrations, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.CurrencyMigrationRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedRestoreOperations, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.RestoreOperationRegistryFactory.CreateServiceRegistry().List)
		}},
		{registry.OwnedRestoreSteps, deletionHandsOver, func(ctx context.Context) ([]createdByEntry, error) {
			return authoredRows(ctx, fs.RestoreStepRegistryFactory.CreateServiceRegistry().List)
		}},
	}
}

// authoredRows lists a group-scoped store and returns the tenant, group and
// author of every row.
func authoredRows[T any](ctx context.Context, list func(context.Context) ([]*T, error)) ([]createdByEntry, error) {
	items, err := list(ctx)
	if err != nil {
		return nil, err
	}
	entries := make([]createdByEntry, 0, len(items))
	for _, item := range items {
		if item != nil {
			entries = append(entries, createdByGroupAware(item))
		}
	}
	return entries, nil
}

func membershipRows(fs *registry.FactorySet) func(context.Context) ([]createdByEntry, error) {
	return func(ctx context.Context) ([]createdByEntry, error) {
		memberships, err := fs.GroupMembershipRegistry.List(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]createdByEntry, 0, len(memberships))
		for _, m := range memberships {
			if m != nil {
				entries = append(entries, createdByEntry{tenantID: m.TenantID, groupID: m.GroupID, createdBy: m.MemberUserID})
			}
		}
		return entries, nil
	}
}

// createdGroupRows lists the groups with their creator. A group's own id is
// its group, which is what the deletion pre-check's purged-group exclusion
// compares against.
func createdGroupRows(fs *registry.FactorySet) func(context.Context) ([]createdByEntry, error) {
	return func(ctx context.Context) ([]createdByEntry, error) {
		groups, err := fs.LocationGroupRegistry.List(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]createdByEntry, 0, len(groups))
		for _, g := range groups {
			if g != nil {
				entries = append(entries, createdByEntry{tenantID: g.GetTenantID(), groupID: g.ID, createdBy: g.CreatedBy})
			}
		}
		return entries, nil
	}
}

func inviteRows(fs *registry.FactorySet) func(context.Context) ([]createdByEntry, error) {
	return func(ctx context.Context) ([]createdByEntry, error) {
		invites, err := fs.GroupInviteRegistry.List(ctx)
		if err != nil {
			return nil, err
		}
		entries := make([]createdByEntry, 0, len(invites))
		for _, inv := range invites {
			if inv != nil {
				entries = append(entries, createdByEntry{tenantID: inv.TenantID, groupID: inv.GroupID, createdBy: inv.CreatedBy})
			}
		}
		return entries, nil
	}
}

// countOwnedContent counts the user's rows in every source the filter keeps,
// in any tenant.
func countOwnedContent(ctx context.Context, sources []ownedSource, userID string, keep func(ownedSource) bool) (map[registry.OwnedContentKind]int, error) {
	counts := make(map[registry.OwnedContentKind]int, len(sources))
	for _, src := range sources {
		if !keep(src) {
			continue
		}
		rows, err := src.rows(ctx)
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			if row.createdBy == userID {
				counts[src.kind]++
			}
		}
	}
	return counts, nil
}
