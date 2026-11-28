package postgres_test

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
)

func TestCommodityRegistry_BackfillRegisteredDates(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)

	recordEvent := func(commodityID string, kind models.CommodityEventKind, at time.Time) {
		c.Helper()
		_, err := f.set.CommodityEventRegistry.Create(f.ctx, models.CommodityEvent{CommodityID: commodityID, Kind: kind, OccurredAt: at})
		c.Assert(err, qt.IsNil)
	}

	// Items an older binary stored without the day they were added.
	withHistory := f.create(c, models.Commodity{Name: "with-history"})
	noHistory := f.create(c, models.Commodity{Name: "no-history"})
	_, err := f.db.ExecContext(c.Context(),
		`UPDATE commodities SET registered_date = NULL, last_modified_date = NULL WHERE id = ANY($1)`,
		[]string{withHistory.ID, noHistory.ID})
	c.Assert(err, qt.IsNil)

	// 00:30 at +02:00 on 5 March is still 4 March in UTC, the day the
	// History tab shows for the instant.
	addedAt := time.Date(2026, 3, 5, 0, 30, 0, 0, time.FixedZone("UTC+2", 2*60*60))
	recordEvent(withHistory.ID, models.CommodityEventKindCreated, addedAt.Add(48*time.Hour))
	recordEvent(withHistory.ID, models.CommodityEventKindCreated, addedAt)
	// Only a "created" event records when the item was added.
	recordEvent(noHistory.ID, models.CommodityEventKindUpdated, addedAt)

	// An item that has its date keeps it.
	current := f.create(c, models.Commodity{Name: "current", RegisteredDate: models.ToPDate("2020-01-02")})
	recordEvent(current.ID, models.CommodityEventKindCreated, addedAt)

	dated, pending, err := f.service.BackfillRegisteredDates(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(dated, qt.Equals, 1)
	c.Assert(pending, qt.IsFalse)

	got, err := f.set.CommodityRegistry.Get(f.ctx, withHistory.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2026-03-04"))
	c.Assert(got.LastModifiedDate, qt.IsNil)
	// An edit form does not send the date back, so the revision stays.
	c.Assert(got.Revision, qt.Equals, withHistory.Revision)

	got, err = f.set.CommodityRegistry.Get(f.ctx, noHistory.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.RegisteredDate, qt.IsNil)

	got, err = f.set.CommodityRegistry.Get(f.ctx, current.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2020-01-02"))

	dated, pending, err = f.service.BackfillRegisteredDates(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(dated, qt.Equals, 0)
	c.Assert(pending, qt.IsFalse)
}

// TestCommodityRegistry_BackfillRegisteredDates_LockedRow checks that a pass
// skips a row another transaction holds locked, as a currency migration
// does, without waiting for it, and reports the row as pending so the
// caller runs another pass; the next pass after the lock is gone dates it.
func TestCommodityRegistry_BackfillRegisteredDates_LockedRow(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)

	item := f.create(c, models.Commodity{Name: "locked"})
	_, err := f.db.ExecContext(c.Context(), `UPDATE commodities SET registered_date = NULL WHERE id = $1`, item.ID)
	c.Assert(err, qt.IsNil)
	_, err = f.set.CommodityEventRegistry.Create(f.ctx, models.CommodityEvent{
		CommodityID: item.ID, Kind: models.CommodityEventKindCreated, OccurredAt: time.Date(2025, 6, 7, 12, 0, 0, 0, time.UTC),
	})
	c.Assert(err, qt.IsNil)

	tx, err := f.db.BeginTxx(c.Context(), nil)
	c.Assert(err, qt.IsNil)
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(c.Context(), `SELECT id FROM commodities WHERE id = $1 FOR UPDATE`, item.ID)
	c.Assert(err, qt.IsNil)

	dated, pending, err := f.service.BackfillRegisteredDates(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(dated, qt.Equals, 0)
	c.Assert(pending, qt.IsTrue)

	c.Assert(tx.Rollback(), qt.IsNil)
	dated, pending, err = f.service.BackfillRegisteredDates(c.Context())
	c.Assert(err, qt.IsNil)
	c.Assert(dated, qt.Equals, 1)
	c.Assert(pending, qt.IsFalse)
	got, err := f.set.CommodityRegistry.Get(f.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2025-06-07"))
}

func TestCommodityRegistry_UpdateKeepsBackfilledRegisteredDate(t *testing.T) {
	f := newUnknownPricePGFixture(t)
	c := qt.New(t)

	item := f.create(c, models.Commodity{Name: "item"})
	_, err := f.db.ExecContext(c.Context(), `UPDATE commodities SET registered_date = NULL WHERE id = $1`, item.ID)
	c.Assert(err, qt.IsNil)
	_, err = f.set.CommodityEventRegistry.Create(f.ctx, models.CommodityEvent{
		CommodityID: item.ID, Kind: models.CommodityEventKindCreated, OccurredAt: time.Date(2025, 6, 7, 12, 0, 0, 0, time.UTC),
	})
	c.Assert(err, qt.IsNil)

	// An edit form opened before the backfill, saved after it.
	stale, err := f.set.CommodityRegistry.Get(f.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stale.RegisteredDate, qt.IsNil)
	_, _, err = f.service.BackfillRegisteredDates(c.Context())
	c.Assert(err, qt.IsNil)

	stale.Comments = "edited"
	_, err = f.set.CommodityRegistry.Update(f.ctx, *stale)
	c.Assert(err, qt.IsNil)
	got, err := f.set.CommodityRegistry.Get(f.ctx, item.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.RegisteredDate, qt.DeepEquals, models.ToPDate("2025-06-07"))
	c.Assert(got.Comments, qt.Equals, "edited")
}
