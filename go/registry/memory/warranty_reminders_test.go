package memory_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/memory"
)

// TestWarrantyReminderRegistry_CreateOnce_Race regression-guards the
// (commodity_id, threshold_days) idempotency contract under
// concurrent CreateOnce calls. Before the fix, the implementation
// dropped the lock between the existence check and the insert, so
// two goroutines could both observe an empty store, both pass, and
// both create a duplicate row. With the lock held across check+insert
// the store is guaranteed to end with exactly one row, and exactly
// one of the goroutines sees inserted=true.
func TestWarrantyReminderRegistry_CreateOnce_Race(t *testing.T) {
	c := qt.New(t)
	reg := memory.NewWarrantyReminderRegistry()

	const concurrency = 20
	var inserted atomic.Int32
	var failed atomic.Int32

	tgaID := models.TenantGroupAwareEntityID{
		TenantID:        "tenant-1",
		GroupID:         "group-1",
		CreatedByUserID: "user-1",
	}
	reminder := models.WarrantyReminder{
		TenantGroupAwareEntityID: tgaID,
		CommodityID:              "commodity-race",
		ThresholdDays:            60,
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	for range concurrency {
		wg.Go(func() {
			<-start
			ok, err := reg.CreateOnce(context.Background(), reminder)
			if err != nil {
				failed.Add(1)
				return
			}
			if ok {
				inserted.Add(1)
			}
		})
	}
	close(start)
	wg.Wait()

	c.Assert(failed.Load(), qt.Equals, int32(0))
	c.Assert(inserted.Load(), qt.Equals, int32(1),
		qt.Commentf("CreateOnce must atomically check+insert; got %d concurrent inserts", inserted.Load()))

	count, err := reg.Count(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1,
		qt.Commentf("registry must end with exactly one row regardless of concurrency"))

	// Subsequent CreateOnce on the same tuple still no-ops.
	ok, err := reg.CreateOnce(context.Background(), reminder)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsFalse)
}

// A silent row keeps its threshold from being processed again, but
// HasSentInRange ignores it: no email went out for it (#2805).
func TestReminderRegistries_HasSentInRange_IgnoresSilentRows(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	owner := models.TenantGroupAwareEntityID{TenantID: "tenant-1", GroupID: "group-1", CreatedByUserID: "user-1"}

	warranty := memory.NewWarrantyReminderRegistry()
	_, err := warranty.CreateOnce(ctx, models.WarrantyReminder{TenantGroupAwareEntityID: owner, CommodityID: "c1", ThresholdDays: 60, Silent: true})
	c.Assert(err, qt.IsNil)
	sent, err := warranty.HasSent(ctx, "c1", 60)
	c.Assert(err, qt.IsNil)
	c.Assert(sent, qt.IsTrue)
	got, err := warranty.HasSentInRange(ctx, "c1", 30, 90)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsFalse)
	_, err = warranty.CreateOnce(ctx, models.WarrantyReminder{TenantGroupAwareEntityID: owner, CommodityID: "c1", ThresholdDays: 45})
	c.Assert(err, qt.IsNil)
	got, err = warranty.HasSentInRange(ctx, "c1", 30, 90)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsTrue)

	maintenance := memory.NewMaintenanceReminderRegistry()
	_, err = maintenance.CreateOnce(ctx, models.MaintenanceReminder{TenantGroupAwareEntityID: owner, ScheduleID: "s1", ThresholdDays: 14, Silent: true})
	c.Assert(err, qt.IsNil)
	sent, err = maintenance.HasSent(ctx, "s1", 14)
	c.Assert(err, qt.IsNil)
	c.Assert(sent, qt.IsTrue)
	got, err = maintenance.HasSentInRange(ctx, "s1", 7, 30)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsFalse)
	_, err = maintenance.CreateOnce(ctx, models.MaintenanceReminder{TenantGroupAwareEntityID: owner, ScheduleID: "s1", ThresholdDays: 7})
	c.Assert(err, qt.IsNil)
	got, err = maintenance.HasSentInRange(ctx, "s1", 7, 30)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.IsTrue)
}
