package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
)

// A plain member who set an item reminder in a group they share can still
// delete their account (#2805). The reminder's created_by_user_id is a NO
// ACTION foreign key to users(id), so unless the purge removes the reminder
// and its send records, the final users delete fails after the memberships
// and settings are already gone.
func TestAccountDeletion_Postgres_MemberWithItemReminders(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()

	// The fixture's group has no membership rows; give it its owner so the
	// group is shared and survives the member's account deletion.
	_, err := f.factorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        f.user.ID,
		Role:                models.GroupRoleOwner,
	})
	c.Assert(err, qt.IsNil)

	member, err := f.factorySet.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		Email:               "reminder-member@test-org.com",
		Name:                "Reminder Member",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = f.factorySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: f.user.TenantID},
		GroupID:             f.groupAID,
		MemberUserID:        member.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	// The member's reminder on the owner's item, with a send record.
	memberCtx := loanCtxFor(member, f.groupAID)
	memberReminders, err := f.factorySet.CommodityReminderRegistryFactory.CreateUserRegistry(memberCtx)
	c.Assert(err, qt.IsNil)
	memberReminder, err := memberReminders.Create(memberCtx, newPGReminder(f.commAID, "Return window", "2026-12-15"))
	c.Assert(err, qt.IsNil)
	c.Assert(memberReminder.CreatedByUserID, qt.Equals, member.ID)
	won, err := f.factorySet.CommodityReminderSendRegistry.CreateOnce(ctx, models.CommodityReminderSend{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: f.user.TenantID, GroupID: f.groupAID, CreatedByUserID: member.ID},
		ReminderID:               memberReminder.ID,
		OccurrenceOn:             "2026-12-15",
		LeadDays:                 7,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)

	// The owner's own reminder on the same item stays.
	ownerReminders, err := f.factorySet.CommodityReminderRegistryFactory.CreateUserRegistry(f.ctxA)
	c.Assert(err, qt.IsNil)
	ownerReminder, err := ownerReminders.Create(f.ctxA, newPGReminder(f.commAID, "Insurance renewal", "2027-01-31"))
	c.Assert(err, qt.IsNil)

	fileSvc := services.NewFileService(f.factorySet, newPostgresFileUploadLocation(c))
	deletion := services.NewAccountDeletionService(f.factorySet, services.NewGroupPurgeService(f.factorySet, fileSvc), nil)
	c.Assert(deletion.DeleteAccount(ctx, f.user.TenantID, member.ID), qt.IsNil)

	_, err = f.factorySet.UserRegistry.Get(ctx, member.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	service := f.factorySet.CommodityReminderRegistryFactory.CreateServiceRegistry()
	_, err = service.Get(ctx, memberReminder.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	n, err := f.factorySet.CommodityReminderSendRegistry.DeleteByReminder(ctx, memberReminder.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	kept, err := ownerReminders.Get(f.ctxA, ownerReminder.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(kept.CreatedByUserID, qt.Equals, f.user.ID)
	_, err = f.groupASet.CommodityRegistry.Get(appctx.WithUser(f.ctxA, f.user), f.commAID)
	c.Assert(err, qt.IsNil)
}
