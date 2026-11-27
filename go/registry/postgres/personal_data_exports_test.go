package postgres_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres"
)

type personalExportPGEnv struct {
	exports registry.PersonalDataExportRegistry
	user    *models.User
	other   *models.User
}

func newPersonalExportPGEnv(t *testing.T) *personalExportPGEnv {
	t.Helper()
	c := qt.New(t)
	set, _ := setupTestRegistrySet(t)
	user := getTestUser(c, set)

	other, err := set.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		Email:               "personal-export-other@example.com",
		Name:                "Other",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	dsn := skipIfNoPostgreSQL(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	fs := postgres.NewFactorySet(sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx"))
	c.Assert(fs.PersonalDataExportRegistry, qt.IsNotNil)
	return &personalExportPGEnv{exports: fs.PersonalDataExportRegistry, user: user, other: other}
}

func (e *personalExportPGEnv) request(c *qt.C, user *models.User, at time.Time) (created, blocking *models.PersonalDataExport) {
	c.Helper()
	created, blocking, err := e.exports.CreateUnlessRequestedSince(context.Background(), models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: user.TenantID, UserID: user.ID},
		CreatedAt:               at,
		ExpiresAt:               at.Add(models.PersonalDataExportRetention),
	}, at.Add(-models.PersonalDataExportRequestInterval))
	c.Assert(err, qt.IsNil)
	return created, blocking
}

func TestPersonalDataExportRegistryPostgres_DailyLimit(t *testing.T) {
	c := qt.New(t)
	env := newPersonalExportPGEnv(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	first, blocking := env.request(c, env.user, now)
	c.Assert(blocking, qt.IsNil)
	c.Assert(first.Status, qt.Equals, models.PersonalDataExportStatusPending)

	created, blocking := env.request(c, env.user, now.Add(time.Hour))
	c.Assert(created, qt.IsNil)
	c.Assert(blocking, qt.IsNotNil)
	c.Assert(blocking.ID, qt.Equals, first.ID)
	c.Assert(blocking.CreatedAt.Equal(now), qt.IsTrue, qt.Commentf("stored %v, requested %v", blocking.CreatedAt, now))

	other, blocking := env.request(c, env.other, now.Add(time.Hour))
	c.Assert(blocking, qt.IsNil)
	c.Assert(other, qt.IsNotNil)

	again, blocking := env.request(c, env.user, now.Add(25*time.Hour))
	c.Assert(blocking, qt.IsNil)
	c.Assert(again, qt.IsNotNil)
}

// Concurrent requests of one user insert one row: the check and the insert
// run under a per-user lock.
func TestPersonalDataExportRegistryPostgres_ConcurrentRequestsInsertOne(t *testing.T) {
	c := qt.New(t)
	env := newPersonalExportPGEnv(t)
	now := time.Now().UTC().Truncate(time.Microsecond)

	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		created int
	)
	for range 8 {
		wg.Go(func() {
			row, _, err := env.exports.CreateUnlessRequestedSince(context.Background(), models.PersonalDataExport{
				TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: env.user.TenantID, UserID: env.user.ID},
				CreatedAt:               now,
				ExpiresAt:               now.Add(models.PersonalDataExportRetention),
			}, now.Add(-models.PersonalDataExportRequestInterval))
			c.Check(err, qt.IsNil)
			if row != nil {
				mu.Lock()
				created++
				mu.Unlock()
			}
		})
	}
	wg.Wait()
	c.Assert(created, qt.Equals, 1)

	list, err := env.exports.ListByUserID(context.Background(), env.user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 1)
}

func TestPersonalDataExportRegistryPostgres_OwnerScopedReads(t *testing.T) {
	c := qt.New(t)
	env := newPersonalExportPGEnv(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	mine, _ := env.request(c, env.user, now)

	got, err := env.exports.GetByUser(ctx, env.user.ID, mine.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, mine.ID)
	c.Assert(got.TenantID, qt.Equals, env.user.TenantID)

	_, err = env.exports.GetByUser(ctx, env.other.ID, mine.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	list, err := env.exports.ListByUserID(ctx, env.other.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 0)
}

func TestPersonalDataExportRegistryPostgres_ClaimUpdateExpire(t *testing.T) {
	c := qt.New(t)
	env := newPersonalExportPGEnv(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)

	export, _ := env.request(c, env.user, now)

	pending, err := env.exports.ListPending(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(pending, qt.HasLen, 1)

	claimed, err := env.exports.ClaimPending(ctx, export.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	claimed, err = env.exports.ClaimPending(ctx, export.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsFalse)

	export.Status = models.PersonalDataExportStatusCompleted
	export.BlobKey = "t/x/personal-exports/" + export.ID + ".tar.gz"
	export.SizeBytes = 1234
	export.GroupCount = 2
	export.ItemCount = 3
	export.FileCount = 4
	export.CompletedAt = new(now)
	export.ExpiresAt = now.Add(time.Hour)
	export.UserID = env.other.ID
	updated, err := env.exports.Update(ctx, *export)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.UserID, qt.Equals, env.user.ID)
	c.Assert(updated.SizeBytes, qt.Equals, int64(1234))
	c.Assert(updated.FileCount, qt.Equals, 4)
	c.Assert(updated.IsDownloadable(now), qt.IsTrue)

	expired, err := env.exports.ListExpired(ctx, now, 10)
	c.Assert(err, qt.IsNil)
	c.Assert(expired, qt.HasLen, 0)
	expired, err = env.exports.ListExpired(ctx, now.Add(2*time.Hour), 10)
	c.Assert(err, qt.IsNil)
	c.Assert(expired, qt.HasLen, 1)

	c.Assert(env.exports.Delete(ctx, export.ID), qt.IsNil)
	c.Assert(env.exports.Delete(ctx, export.ID), qt.IsNil)
	_, err = env.exports.Get(ctx, export.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// A build that started before the cutoff and is still in progress goes back
// to the queue until the export has started its last allowed build; a dead
// build after that fails it. A build that started after the cutoff stays.
func TestPersonalDataExportRegistryPostgres_RecoverStalled(t *testing.T) {
	c := qt.New(t)
	env := newPersonalExportPGEnv(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond)
	sweep := registry.PersonalDataExportStalledSweep{
		StartedBefore: now.Add(-time.Hour),
		Now:           now,
		MaxBuilds:     2,
		ErrorMessage:  "the worker stopped",
	}

	stalled, _ := env.request(c, env.user, now)
	running, _ := env.request(c, env.other, now)
	claimed, err := env.exports.ClaimPending(ctx, stalled.ID, now.Add(-2*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	claimed, err = env.exports.ClaimPending(ctx, running.ID, now.Add(-time.Minute))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)

	requeued, failed, err := env.exports.RecoverStalled(ctx, sweep)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 1)
	c.Assert(failed, qt.Equals, 0)

	got, err := env.exports.Get(ctx, stalled.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusPending)
	c.Assert(got.StartedAt, qt.IsNil)
	c.Assert(got.AttemptCount, qt.Equals, 1)
	got, err = env.exports.Get(ctx, running.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusInProgress)
	c.Assert(got.StartedAt, qt.IsNotNil)

	// The second build is the last one; when it dies too, the export fails.
	claimed, err = env.exports.ClaimPending(ctx, stalled.ID, now.Add(-2*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	requeued, failed, err = env.exports.RecoverStalled(ctx, sweep)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 1)

	got, err = env.exports.Get(ctx, stalled.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, "the worker stopped")
	c.Assert(got.CompletedAt, qt.IsNotNil)
	c.Assert(got.CompletedAt.Equal(now), qt.IsTrue, qt.Commentf("completed at %s", got.CompletedAt))
	c.Assert(got.AttemptCount, qt.Equals, 2)

	// A failed export is not swept again.
	requeued, failed, err = env.exports.RecoverStalled(ctx, sweep)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 0)
}
