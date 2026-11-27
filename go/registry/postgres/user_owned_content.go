package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// userOwnedTable is a tenant-scoped table whose rows name a user as their
// member or author. A row there ties the user to the tenant it lives in.
type userOwnedTable struct {
	kind  registry.OwnedContentKind
	table func(store.TableNames) store.TableName
	// userColumn names the user: created_by_user_id on content,
	// created_by on groups and invites, member_user_id on memberships.
	userColumn string
	// groupColumn names the group the row belongs to. The account-deletion
	// pre-check uses it to leave out the groups it purges.
	groupColumn string
	// onDeletion is what deleting the user's account does with the rows
	// outside the private groups it purges.
	onDeletion accountDeletion
}

// accountDeletion is what deleting an account does with a row that names the
// user, in a group that stays.
type accountDeletion int

const (
	// deletionBlocks marks the user's content: items, files, places, exports
	// and the groups they created. The account-deletion pre-check
	// (HasRetainedOwnedContent) refuses the deletion, before anything is
	// changed, while such a row exists in a group that stays.
	deletionBlocks accountDeletion = iota + 1
	// deletionRemoves marks rows that are personal to the user: memberships,
	// the invites they created, reminders that email only them, drafts,
	// intake sessions and saved views. UserPurger deletes them.
	deletionRemoves
	// deletionHandsOver marks rows that belong to the group: tags, sellers,
	// item history, loans, service records, supply links, maintenance,
	// reminder send records, item types, custom fields, delivered documents,
	// restores and currency changes. UserPurger gives them to another member
	// of the group and records each hand-over in the audit log.
	deletionHandsOver
)

// String names the rule for test output.
func (d accountDeletion) String() string {
	switch d {
	case deletionBlocks:
		return "blocks"
	case deletionRemoves:
		return "removed"
	case deletionHandsOver:
		return "handed over"
	default:
		return "unclassified"
	}
}

// userOwnedTables is the one list of what ties a user to their tenant. The
// tenant move refuses a user with a row in any of them. Account deletion
// treats each table as its onDeletion says: the pre-check reads the
// deletionBlocks tables, and UserPurger removes the deletionRemoves rows and
// hands over the deletionHandsOver rows. TestUserTenantMove_
// CoversEveryUserReference and TestAccountDeletion_CoversEveryUserReference
// check it against the foreign keys to users(id) in the live schema, so a new
// table cannot be left out.
//
// The deletionRemoves entries are deleted in list order, so a child table
// whose foreign key to its parent is not ON DELETE CASCADE must come first.
var userOwnedTables = []userOwnedTable{
	{registry.OwnedGroupMemberships, func(t store.TableNames) store.TableName { return t.GroupMemberships() }, "member_user_id", "group_id", deletionRemoves},
	{registry.OwnedLocationGroups, func(t store.TableNames) store.TableName { return t.LocationGroups() }, "created_by", "id", deletionBlocks},
	{registry.OwnedGroupInvites, func(t store.TableNames) store.TableName { return t.GroupInvites() }, "created_by", "group_id", deletionRemoves},
	{registry.OwnedCommodities, func(t store.TableNames) store.TableName { return t.Commodities() }, createdByUserIDColumn, "group_id", deletionBlocks},
	{registry.OwnedFiles, func(t store.TableNames) store.TableName { return t.Files() }, createdByUserIDColumn, "group_id", deletionBlocks},
	{registry.OwnedFileLinks, func(t store.TableNames) store.TableName { return t.FileLinks() }, createdByUserIDColumn, "group_id", deletionBlocks},
	{registry.OwnedLocations, func(t store.TableNames) store.TableName { return t.Locations() }, createdByUserIDColumn, "group_id", deletionBlocks},
	{registry.OwnedAreas, func(t store.TableNames) store.TableName { return t.Areas() }, createdByUserIDColumn, "group_id", deletionBlocks},
	{registry.OwnedTags, func(t store.TableNames) store.TableName { return t.Tags() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedSellers, func(t store.TableNames) store.TableName { return t.Sellers() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedExports, func(t store.TableNames) store.TableName { return t.Exports() }, createdByUserIDColumn, "group_id", deletionBlocks},
	{registry.OwnedCommodityEvents, func(t store.TableNames) store.TableName { return t.CommodityEvents() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedCommodityLoans, func(t store.TableNames) store.TableName { return t.CommodityLoans() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedCommodityServices, func(t store.TableNames) store.TableName { return t.CommodityServices() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedCommoditySupplyLinks, func(t store.TableNames) store.TableName { return t.CommoditySupplyLinks() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedMaintenanceSchedules, func(t store.TableNames) store.TableName { return t.MaintenanceSchedules() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedMaintenanceReminders, func(t store.TableNames) store.TableName { return t.MaintenanceReminders() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedCommodityReminders, func(t store.TableNames) store.TableName { return t.CommodityReminders() }, createdByUserIDColumn, "group_id", deletionRemoves},
	{registry.OwnedCommodityReminderSends, func(t store.TableNames) store.TableName { return t.CommodityReminderSends() }, createdByUserIDColumn, "group_id", deletionRemoves},
	{registry.OwnedWarrantyReminders, func(t store.TableNames) store.TableName { return t.WarrantyReminders() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedStorageQuotaReminders, func(t store.TableNames) store.TableName { return t.StorageQuotaReminders() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedCommodityDrafts, func(t store.TableNames) store.TableName { return t.CommodityDrafts() }, createdByUserIDColumn, "group_id", deletionRemoves},
	{registry.OwnedIntakeSessions, func(t store.TableNames) store.TableName { return t.IntakeSessions() }, createdByUserIDColumn, "group_id", deletionRemoves},
	{registry.OwnedIntakeSources, func(t store.TableNames) store.TableName { return t.IntakeSources() }, createdByUserIDColumn, "group_id", deletionRemoves},
	{registry.OwnedIntakeCandidates, func(t store.TableNames) store.TableName { return t.IntakeCandidates() }, createdByUserIDColumn, "group_id", deletionRemoves},
	{registry.OwnedIntakeDocuments, func(t store.TableNames) store.TableName { return t.IntakeDocuments() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedItemTypes, func(t store.TableNames) store.TableName { return t.ItemTypes() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedCustomFieldDefinitions, func(t store.TableNames) store.TableName { return t.CustomFieldDefinitions() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedItemTypeCustomFields, func(t store.TableNames) store.TableName { return t.ItemTypeCustomFields() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedSavedViews, func(t store.TableNames) store.TableName { return t.SavedViews() }, createdByUserIDColumn, "group_id", deletionRemoves},
	{registry.OwnedCurrencyMigrations, func(t store.TableNames) store.TableName { return t.CurrencyMigrations() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedCurrencyMigrationAudit, func(t store.TableNames) store.TableName { return t.CurrencyMigrationAudit() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedRestoreOperations, func(t store.TableNames) store.TableName { return t.RestoreOperations() }, createdByUserIDColumn, "group_id", deletionHandsOver},
	{registry.OwnedRestoreSteps, func(t store.TableNames) store.TableName { return t.RestoreSteps() }, createdByUserIDColumn, "group_id", deletionHandsOver},
}

const createdByUserIDColumn = "created_by_user_id"

// countUserOwnedContent counts the user's rows in every userOwnedTables entry,
// in any tenant, with one UNION ALL query.
func countUserOwnedContent(ctx context.Context, tx *sqlx.Tx, tableNames store.TableNames, userID string) (registry.UserOwnedContent, error) {
	parts := make([]string, 0, len(userOwnedTables))
	for i, t := range userOwnedTables {
		// The index, not the kind, labels each row: the kind is data, and the
		// query text is built from identifiers only.
		parts = append(parts, fmt.Sprintf("SELECT %d, COUNT(*) FROM %s WHERE %s = $1", i, t.table(tableNames), t.userColumn))
	}
	rows, err := tx.QueryContext(ctx, strings.Join(parts, " UNION ALL "), userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to count the user's content", err, errx.Attrs("user_id", userID))
	}
	defer rows.Close()

	counts := make(map[registry.OwnedContentKind]int, len(userOwnedTables))
	for rows.Next() {
		var index, count int
		if err := rows.Scan(&index, &count); err != nil {
			return nil, errxtrace.Wrap("failed to read the user's content count", err)
		}
		counts[userOwnedTables[index].kind] = count
	}
	if err := rows.Err(); err != nil {
		return nil, errxtrace.Wrap("failed to read the user's content counts", err)
	}
	return registry.NewUserOwnedContent(counts), nil
}
