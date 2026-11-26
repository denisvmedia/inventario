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
	// deletionPreCheck marks the tables the account-deletion pre-check
	// (HasRetainedOwnedContent) reads: content that would survive the
	// deletion of the user's private groups and that nothing purges or hands
	// over. The others are removed with the account (memberships, reminders,
	// drafts, intake sessions, saved views), handed to another member (item
	// types, custom fields, intake documents), or not yet classified for
	// deletion (#2882).
	deletionPreCheck bool
}

// userOwnedTables is the one list of what ties a user to their tenant. The
// tenant move refuses a user with a row in any of them; the account-deletion
// pre-check reads the ones marked deletionPreCheck. TestUserTenantMove_
// CoversEveryUserReference checks it against the foreign keys to users(id) in
// the live schema, so a new table cannot be left out of both.
var userOwnedTables = []userOwnedTable{
	{registry.OwnedGroupMemberships, func(t store.TableNames) store.TableName { return t.GroupMemberships() }, "member_user_id", "group_id", false},
	{registry.OwnedLocationGroups, func(t store.TableNames) store.TableName { return t.LocationGroups() }, "created_by", "id", true},
	{registry.OwnedGroupInvites, func(t store.TableNames) store.TableName { return t.GroupInvites() }, "created_by", "group_id", false},
	{registry.OwnedCommodities, func(t store.TableNames) store.TableName { return t.Commodities() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedFiles, func(t store.TableNames) store.TableName { return t.Files() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedFileLinks, func(t store.TableNames) store.TableName { return t.FileLinks() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedLocations, func(t store.TableNames) store.TableName { return t.Locations() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedAreas, func(t store.TableNames) store.TableName { return t.Areas() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedTags, func(t store.TableNames) store.TableName { return t.Tags() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedSellers, func(t store.TableNames) store.TableName { return t.Sellers() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedExports, func(t store.TableNames) store.TableName { return t.Exports() }, createdByUserIDColumn, "group_id", true},
	{registry.OwnedCommodityEvents, func(t store.TableNames) store.TableName { return t.CommodityEvents() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCommodityLoans, func(t store.TableNames) store.TableName { return t.CommodityLoans() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCommodityServices, func(t store.TableNames) store.TableName { return t.CommodityServices() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCommoditySupplyLinks, func(t store.TableNames) store.TableName { return t.CommoditySupplyLinks() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedMaintenanceSchedules, func(t store.TableNames) store.TableName { return t.MaintenanceSchedules() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedMaintenanceReminders, func(t store.TableNames) store.TableName { return t.MaintenanceReminders() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCommodityReminders, func(t store.TableNames) store.TableName { return t.CommodityReminders() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCommodityReminderSends, func(t store.TableNames) store.TableName { return t.CommodityReminderSends() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedWarrantyReminders, func(t store.TableNames) store.TableName { return t.WarrantyReminders() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedStorageQuotaReminders, func(t store.TableNames) store.TableName { return t.StorageQuotaReminders() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCommodityDrafts, func(t store.TableNames) store.TableName { return t.CommodityDrafts() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedIntakeSessions, func(t store.TableNames) store.TableName { return t.IntakeSessions() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedIntakeSources, func(t store.TableNames) store.TableName { return t.IntakeSources() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedIntakeCandidates, func(t store.TableNames) store.TableName { return t.IntakeCandidates() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedIntakeDocuments, func(t store.TableNames) store.TableName { return t.IntakeDocuments() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedItemTypes, func(t store.TableNames) store.TableName { return t.ItemTypes() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCustomFieldDefinitions, func(t store.TableNames) store.TableName { return t.CustomFieldDefinitions() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedItemTypeCustomFields, func(t store.TableNames) store.TableName { return t.ItemTypeCustomFields() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedSavedViews, func(t store.TableNames) store.TableName { return t.SavedViews() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCurrencyMigrations, func(t store.TableNames) store.TableName { return t.CurrencyMigrations() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedCurrencyMigrationAudit, func(t store.TableNames) store.TableName { return t.CurrencyMigrationAudit() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedRestoreOperations, func(t store.TableNames) store.TableName { return t.RestoreOperations() }, createdByUserIDColumn, "group_id", false},
	{registry.OwnedRestoreSteps, func(t store.TableNames) store.TableName { return t.RestoreSteps() }, createdByUserIDColumn, "group_id", false},
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
