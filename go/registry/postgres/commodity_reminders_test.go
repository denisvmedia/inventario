package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

func newPGReminder(commodityID, title, dueOn string) models.CommodityReminder {
	return models.CommodityReminder{
		CommodityID:      commodityID,
		Title:            title,
		DueOn:            models.Date(dueOn),
		RemindDaysBefore: models.ValuerSlice[int]{30, 7},
		RepeatUnit:       models.ReminderRepeatMonth,
		RepeatInterval:   1,
	}
}

func TestCommodityReminderRegistry_Postgres_CRUDAndIsolation(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)

	regA, err := f.factorySet.CommodityReminderRegistryFactory.CreateUserRegistry(f.ctxA)
	c.Assert(err, qt.IsNil)
	regB, err := f.factorySet.CommodityReminderRegistryFactory.CreateUserRegistry(f.ctxB)
	c.Assert(err, qt.IsNil)

	later, err := regA.Create(f.ctxA, newPGReminder(f.commAID, "Renewal", "2027-01-31"))
	c.Assert(err, qt.IsNil)
	sooner, err := regA.Create(f.ctxA, newPGReminder(f.commAID, "Inspection", "2026-12-15"))
	c.Assert(err, qt.IsNil)
	_, err = regB.Create(f.ctxB, newPGReminder(f.commBID, "Other group", "2026-12-01"))
	c.Assert(err, qt.IsNil)

	got, err := regA.Get(f.ctxA, later.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Title, qt.Equals, "Renewal")
	c.Assert([]int(got.RemindDaysBefore), qt.DeepEquals, []int{30, 7})
	c.Assert(got.RepeatUnit, qt.Equals, models.ReminderRepeatMonth)
	c.Assert(got.GroupID, qt.Equals, f.groupAID)
	c.Assert(got.CreatedByUserID, qt.Equals, f.user.ID)

	list, err := regA.ListByCommodity(f.ctxA, f.commAID)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 2)
	c.Assert(list[0].ID, qt.Equals, sooner.ID)
	c.Assert(list[1].ID, qt.Equals, later.ID)

	// Group B cannot read group A's reminders.
	_, err = regB.Get(f.ctxB, later.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	all, err := regB.List(f.ctxB)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)

	// The service registry, used by the worker, sees every group.
	service := f.factorySet.CommodityReminderRegistryFactory.CreateServiceRegistry()
	everything, err := service.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(len(everything) >= 3, qt.IsTrue)

	got.Title = "Annual renewal"
	got.RemindDaysBefore = models.ValuerSlice[int]{1}
	updated, err := regA.Update(f.ctxA, *got)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Title, qt.Equals, "Annual renewal")
	reread, err := regA.Get(f.ctxA, later.ID)
	c.Assert(err, qt.IsNil)
	c.Assert([]int(reread.RemindDaysBefore), qt.DeepEquals, []int{1})

	c.Assert(regA.Delete(f.ctxA, later.ID), qt.IsNil)
	_, err = regA.Get(f.ctxA, later.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestCommodityReminderSendRegistry_Postgres(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)

	reg, err := f.factorySet.CommodityReminderRegistryFactory.CreateUserRegistry(f.ctxA)
	c.Assert(err, qt.IsNil)
	reminder, err := reg.Create(f.ctxA, newPGReminder(f.commAID, "Inspection", "2026-12-15"))
	c.Assert(err, qt.IsNil)

	sends := f.factorySet.CommodityReminderSendRegistry
	send := models.CommodityReminderSend{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: f.user.TenantID, GroupID: f.groupAID, CreatedByUserID: f.user.ID},
		ReminderID:               reminder.ID,
		OccurrenceOn:             "2026-12-15",
		LeadDays:                 7,
	}
	ctx := context.Background()

	has, err := sends.HasSentAtOrBelow(ctx, reminder.ID, "2026-12-15", 30)
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsFalse)

	won, err := sends.CreateOnce(ctx, send)
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)
	won, err = sends.CreateOnce(ctx, send)
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsFalse, qt.Commentf("the second claim of the same tuple must lose"))

	// A send at 7 days covers the larger 30-day lead, not the smaller 1-day one.
	has, err = sends.HasSentAtOrBelow(ctx, reminder.ID, "2026-12-15", 30)
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsTrue)
	has, err = sends.HasSentAtOrBelow(ctx, reminder.ID, "2026-12-15", 1)
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsFalse)
	has, err = sends.HasSentAtOrBelow(ctx, reminder.ID, "2027-01-15", 30)
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsFalse, qt.Commentf("another occurrence starts clean"))

	c.Assert(sends.Release(ctx, reminder.ID, "2026-12-15", 7), qt.IsNil)
	won, err = sends.CreateOnce(ctx, send)
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue, qt.Commentf("a released claim can be taken again"))

	n, err := sends.DeleteByReminder(ctx, reminder.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)

	// Deleting the item removes its reminders and their send records.
	_, err = sends.CreateOnce(ctx, send)
	c.Assert(err, qt.IsNil)
	c.Assert(f.groupASet.CommodityRegistry.Delete(f.ctxA, f.commAID), qt.IsNil)
	_, err = reg.Get(f.ctxA, reminder.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	n, err = sends.DeleteByReminder(ctx, reminder.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)
}
