package postgres_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

// HasSentInRange backs the "a larger lead time is never sent after a
// smaller one" rule of the warranty and maintenance workers (#2805).
func TestReminderRows_Postgres_HasSentInRange(t *testing.T) {
	c := qt.New(t)
	f := newLoanPGFixture(t)
	ctx := context.Background()
	owner := models.TenantGroupAwareEntityID{TenantID: f.user.TenantID, GroupID: f.groupAID, CreatedByUserID: f.user.ID}

	warranty := f.factorySet.WarrantyReminderRegistry
	won, err := warranty.CreateOnce(ctx, models.WarrantyReminder{TenantGroupAwareEntityID: owner, CommodityID: f.commAID, ThresholdDays: 60})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)
	for _, tc := range []struct {
		from, below int
		want        bool
	}{
		{45, 90, true},
		{60, 61, true},
		{45, 60, false},
		{61, 365, false},
	} {
		got, err := warranty.HasSentInRange(ctx, f.commAID, tc.from, tc.below)
		c.Assert(err, qt.IsNil)
		c.Assert(got, qt.Equals, tc.want, qt.Commentf("warranty [%d, %d)", tc.from, tc.below))
	}
	got, err := warranty.HasSentInRange(ctx, f.commBID, 1, 365)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsFalse, qt.Commentf("another commodity's rows do not count"))

	schedule, err := f.groupASet.MaintenanceScheduleRegistry.Create(f.ctxA, models.MaintenanceSchedule{
		CommodityID:  f.commAID,
		Title:        "Clean the filter",
		IntervalDays: 90,
		NextDueAt:    "2027-01-10",
		Enabled:      true,
	})
	c.Assert(err, qt.IsNil)
	maintenance := f.factorySet.MaintenanceReminderRegistry
	won, err = maintenance.CreateOnce(ctx, models.MaintenanceReminder{TenantGroupAwareEntityID: owner, ScheduleID: schedule.ID, ThresholdDays: 14})
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)
	got, err = maintenance.HasSentInRange(ctx, schedule.ID, 10, 60)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsTrue)
	got, err = maintenance.HasSentInRange(ctx, schedule.ID, 1, 14)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsFalse)
}
