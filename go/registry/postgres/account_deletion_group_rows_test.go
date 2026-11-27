package postgres_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// A member who only recorded things on another member's item in a shared
// group can delete their account (#2882). Item history, loans, service
// records, supply links, maintenance, reminder send records and restores carry
// a NO ACTION created_by_user_id foreign key to users(id); they pass to the
// group's owner, and the invites the member created or accepted go. Without
// that, the final users delete fails after the member's private group,
// memberships and sessions are already gone.
func TestAccountDeletion_Postgres_MemberWhoRecordedOnASharedItem(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	pool, err := getOrCreatePool(skipIfNoPostgreSQL(t))
	c.Assert(err, qt.IsNil)
	db := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	fs := f.factorySet
	owner := f.user

	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        owner.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)
	member, err := fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		Email:               "recording-member@test-org.com",
		Name:                "Recording Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	// The member's own private group, which the deletion purges.
	privateSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	private, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		Name:                "Member's own",
		Slug:                privateSlug,
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           member.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: owner.TenantID},
		GroupID:             private.ID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)

	memberCtx := loanCtxFor(member, f.groupAID)
	authored := models.TenantGroupAwareEntityID{TenantID: owner.TenantID, GroupID: f.groupAID, CreatedByUserID: member.ID}
	handedOver := map[string]string{}

	event, err := fs.CommodityEventRegistryFactory.CreateServiceRegistry().Create(memberCtx, models.CommodityEvent{
		TenantGroupAwareEntityID: authored,
		CommodityID:              f.commAID,
		Kind:                     models.CommodityEventKindUpdated,
		OccurredAt:               time.Now(),
	})
	c.Assert(err, qt.IsNil)
	handedOver["commodity_events"] = event.ID

	loan, err := fs.CommodityLoanRegistryFactory.CreateServiceRegistry().Create(memberCtx, models.CommodityLoan{
		TenantGroupAwareEntityID: authored,
		CommodityID:              f.commAID,
		BorrowerName:             "Neighbor",
		LentAt:                   models.Date("2026-05-01"),
	})
	c.Assert(err, qt.IsNil)
	handedOver["commodity_loans"] = loan.ID

	service, err := fs.CommodityServiceRegistryFactory.CreateServiceRegistry().Create(memberCtx, models.CommodityService{
		TenantGroupAwareEntityID: authored,
		CommodityID:              f.commAID,
		ProviderName:             "Repair shop",
		Reason:                   "Blade",
		SentAt:                   "2026-05-01",
		CostCurrency:             "USD",
	})
	c.Assert(err, qt.IsNil)
	handedOver["commodity_services"] = service.ID

	link, err := fs.SupplyLinkRegistryFactory.CreateServiceRegistry().Create(memberCtx, models.SupplyLink{
		TenantGroupAwareEntityID: authored,
		CommodityID:              f.commAID,
		Label:                    "Spare blades",
		URL:                      "https://example.com/blades",
	})
	c.Assert(err, qt.IsNil)
	handedOver["commodity_supply_links"] = link.ID

	schedule, err := fs.MaintenanceScheduleRegistryFactory.CreateServiceRegistry().Create(memberCtx, models.MaintenanceSchedule{
		TenantGroupAwareEntityID: authored,
		CommodityID:              f.commAID,
		Title:                    "Sharpen the blade",
		IntervalDays:             90,
		NextDueAt:                "2027-01-10",
		Enabled:                  true,
	})
	c.Assert(err, qt.IsNil)
	handedOver["maintenance_schedules"] = schedule.ID

	won, err := fs.MaintenanceReminderRegistry.CreateOnce(ctx, models.MaintenanceReminder{TenantGroupAwareEntityID: authored, ScheduleID: schedule.ID, ThresholdDays: 14})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)
	won, err = fs.WarrantyReminderRegistry.CreateOnce(ctx, models.WarrantyReminder{TenantGroupAwareEntityID: authored, CommodityID: f.commAID, ThresholdDays: 30})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)
	won, err = fs.StorageQuotaReminderRegistry.CreateOnce(ctx, models.StorageQuotaReminder{TenantGroupAwareEntityID: authored, ThresholdPercent: int(models.StorageQuota90Percent)})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)

	export, err := f.groupASet.ExportRegistry.Create(f.ctxA, models.Export{
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "owner's backup",
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)
	restores, err := fs.RestoreOperationRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	restore, err := restores.Create(memberCtx, models.RestoreOperation{
		ExportID:    export.ID,
		Description: "member's restore",
		Status:      models.RestoreStatusCompleted,
		Options:     models.RestoreOptions{Strategy: "merge_add"},
		CreatedDate: models.PNow(),
	})
	c.Assert(err, qt.IsNil)
	handedOver["restore_operations"] = restore.ID
	steps, err := fs.RestoreStepRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	step, err := steps.Create(memberCtx, models.RestoreStep{
		RestoreOperationID: restore.ID,
		Name:               "items",
		Result:             models.RestoreStepResultSuccess,
	})
	c.Assert(err, qt.IsNil)
	handedOver["restore_steps"] = step.ID

	// The invite the member joined with, and one the member sent.
	usedAt := time.Now()
	accepted := mustCreateInvite(c, ctx, fs, owner.TenantID, f.groupAID, owner.ID, time.Now().Add(time.Hour), &member.ID, &usedAt)
	sent := mustCreateInvite(c, ctx, fs, owner.TenantID, f.groupAID, member.ID, time.Now().Add(time.Hour), nil, nil)

	fileSvc := services.NewFileService(fs, newPostgresFileUploadLocation(c))
	deletion := services.NewAccountDeletionService(fs, services.NewGroupPurgeService(fs, fileSvc), nil)
	c.Assert(deletion.DeleteAccount(ctx, owner.TenantID, member.ID), qt.IsNil)

	_, err = fs.UserRegistry.Get(ctx, member.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, err = fs.LocationGroupRegistry.Get(ctx, private.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	for table, id := range handedOver {
		c.Check(creatorOf(c, db, table, "id = $1", id), qt.Equals, owner.ID, qt.Commentf("%s passes to the owner", table))
	}
	c.Check(creatorOf(c, db, "maintenance_reminders", "schedule_id = $1", schedule.ID), qt.Equals, owner.ID)
	c.Check(creatorOf(c, db, "warranty_reminders", "commodity_id = $1", f.commAID), qt.Equals, owner.ID)
	c.Check(creatorOf(c, db, "storage_quota_reminders", "group_id = $1", f.groupAID), qt.Equals, owner.ID)

	var invites int
	c.Assert(db.QueryRowContext(ctx, "SELECT COUNT(*) FROM group_invites WHERE id IN ($1, $2)", accepted, sent).Scan(&invites), qt.IsNil)
	c.Check(invites, qt.Equals, 0)

	_, err = f.groupASet.CommodityRegistry.Get(f.ctxA, f.commAID)
	c.Assert(err, qt.IsNil)
}

// creatorOf reads created_by_user_id of the one row of table that where
// selects.
func creatorOf(c *qt.C, db *sqlx.DB, table, where string, args ...any) string {
	c.Helper()
	var creator string
	query := fmt.Sprintf("SELECT created_by_user_id FROM %s WHERE %s", table, where)
	c.Assert(db.QueryRowContext(context.Background(), query, args...).Scan(&creator), qt.IsNil, qt.Commentf("%s", table))
	return creator
}
