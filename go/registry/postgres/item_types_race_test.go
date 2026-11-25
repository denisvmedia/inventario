package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// An item update that keeps the item's type races a delete that moves the
// type's items elsewhere. A third transaction holds the item's row, so the
// delete queues on it first and the update second, after the update has read
// the row with the old type. The update must then see the type the delete
// left and refuse the old one, instead of writing back a type the group no
// longer has.
func TestCommodityUpdate_KeepingATypeDeletedWhileItWaits(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	ctx := fx.ctxA
	types := fx.groupASet.ItemTypeRegistry
	items := fx.groupASet.CommodityRegistry

	vehicle, err := types.Create(ctx, models.ItemType{Name: "Vehicle"})
	c.Assert(err, qt.IsNil)
	other, err := types.GetBySlug(ctx, models.CommodityTypeOther)
	c.Assert(err, qt.IsNil)
	created, err := items.Create(ctx, models.Commodity{
		Name: "Car", ShortName: "Car", Type: vehicle.Slug,
		AreaID: new(fx.areaAID), Count: 1, Status: models.CommodityStatusInUse,
	})
	c.Assert(err, qt.IsNil)
	car, err := items.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	// waitForWaiter waits until a statement starting with prefix waits for
	// a lock.
	waitForWaiter := func(prefix string) {
		c.Helper()
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			var n int
			err := fx.dbx.GetContext(context.Background(), &n,
				`SELECT count(*) FROM pg_stat_activity
				 WHERE wait_event_type = 'Lock' AND datname = current_database() AND query LIKE $1`,
				prefix+"%")
			c.Assert(err, qt.IsNil)
			if n > 0 {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		c.Fatalf("timed out waiting for a lock waiter running %q", prefix)
	}

	locked := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- store.DoAsBackgroundWorker(context.Background(), fx.dbx, func(ctx context.Context, tx *sqlx.Tx) error {
			if _, err := tx.ExecContext(ctx, `SELECT 1 FROM commodities WHERE id = $1 FOR UPDATE`, car.ID); err != nil {
				return err
			}
			close(locked)
			<-release
			return nil
		})
	}()
	<-locked

	deleteDone := make(chan error, 1)
	go func() {
		_, err := types.DeleteReassigning(ctx, vehicle.ID, other.ID)
		deleteDone <- err
	}()
	waitForWaiter("UPDATE commodities SET type =")

	updateDone := make(chan error, 1)
	go func() {
		edit := *car
		edit.Name = "Car (renamed)" // the form sends the type it loaded
		_, err := items.Update(ctx, edit)
		updateDone <- err
	}()
	waitForWaiter("SELECT deleted_at IS NOT NULL FROM commodities")

	close(release)
	c.Assert(<-holderDone, qt.IsNil)
	c.Assert(<-deleteDone, qt.IsNil)
	c.Check(<-updateDone, qt.ErrorIs, registry.ErrUnknownItemType)

	got, err := items.Get(ctx, car.ID)
	c.Assert(err, qt.IsNil)
	c.Check(got.Type, qt.Equals, models.CommodityTypeOther)
	c.Check(got.Name, qt.Equals, "Car")
}
