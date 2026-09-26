package postgres_test

import (
	"context"
	"database/sql"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry/postgres"
)

// The weekly digest leans on a unique index to decide who sends: two workers
// both see no claim, both insert, and only the index can pick one. That
// property only exists in PostgreSQL, so the in-memory implementation and the
// service tests above it cannot show it.
func TestWeeklyDigestSendRegistry_ClaimIsExclusive(t *testing.T) {
	c := qt.New(t)

	dsn := skipIfNoPostgreSQL(t)
	regSet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	ctx := context.Background()
	// The tenant and user the shared fixture created; the claim row has a
	// foreign key to both.
	users, err := regSet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.Not(qt.HasLen), 0)
	user := users[0]

	db, err := sql.Open("pgx", dsn)
	c.Assert(err, qt.IsNil)
	t.Cleanup(func() { _ = db.Close() })
	reg := postgres.NewWeeklyDigestSendRegistry(sqlx.NewDb(db, "pgx"))

	week := time.Date(2026, 9, 21, 0, 0, 0, 0, time.UTC)
	claim := func() (bool, error) {
		return reg.ClaimWeek(ctx, models.WeeklyDigestSend{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{
				TenantID: user.TenantID,
				UserID:   user.ID,
			},
			WeekStart: week,
		})
	}

	c.Run("only one of several concurrent claims wins", func(c *qt.C) {
		const racers = 8
		var (
			wg  sync.WaitGroup
			mu  sync.Mutex
			won int
		)
		for range racers {
			wg.Go(func() {
				claimed, claimErr := claim()
				mu.Lock()
				defer mu.Unlock()
				c.Check(claimErr, qt.IsNil)
				if claimed {
					won++
				}
			})
		}
		wg.Wait()
		c.Assert(won, qt.Equals, 1,
			qt.Commentf("%d of %d claims won; the unique index is not deciding", won, racers))
	})

	c.Run("releasing lets the week be claimed again", func(c *qt.C) {
		c.Assert(reg.ReleaseWeek(ctx, user.ID, week), qt.IsNil)
		claimed, claimErr := claim()
		c.Assert(claimErr, qt.IsNil)
		c.Assert(claimed, qt.IsTrue)
	})

	c.Run("a different week is a different claim", func(c *qt.C) {
		nextWeek := week.AddDate(0, 0, 7)
		claimed, claimErr := reg.ClaimWeek(ctx, models.WeeklyDigestSend{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{
				TenantID: user.TenantID,
				UserID:   user.ID,
			},
			WeekStart: nextWeek,
		})
		c.Assert(claimErr, qt.IsNil)
		c.Assert(claimed, qt.IsTrue)
	})

	c.Run("the sweep drops old claims and keeps current ones", func(c *qt.C) {
		deleted, sweepErr := reg.DeleteSentBefore(ctx, week.AddDate(0, 0, 7))
		c.Assert(sweepErr, qt.IsNil)
		c.Assert(deleted, qt.Equals, 1, qt.Commentf("only the older of the two weeks should go"))

		// The swept week can be claimed again; the kept one cannot.
		claimed, claimErr := claim()
		c.Assert(claimErr, qt.IsNil)
		c.Assert(claimed, qt.IsTrue)

		claimed, claimErr = reg.ClaimWeek(ctx, models.WeeklyDigestSend{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{
				TenantID: user.TenantID,
				UserID:   user.ID,
			},
			WeekStart: week.AddDate(0, 0, 7),
		})
		c.Assert(claimErr, qt.IsNil)
		c.Assert(claimed, qt.IsFalse)
	})
}
