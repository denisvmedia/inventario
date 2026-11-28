package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func personalExportRequest(userID string, at time.Time) models.PersonalDataExport {
	return models.PersonalDataExport{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: "tenant-1", UserID: userID},
		CreatedAt:               at,
		ExpiresAt:               at.Add(models.PersonalDataExportRetention),
	}
}

func TestPersonalDataExportRegistryMemory_CreateUnlessRequestedSince(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewPersonalDataExportRegistry()
	now := time.Now().UTC()

	first, blocking, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-1", now), now.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(blocking, qt.IsNil)
	c.Assert(first.ID, qt.Not(qt.Equals), "")
	c.Assert(first.Status, qt.Equals, models.PersonalDataExportStatusPending)

	// A second request of the same user within the interval is refused and
	// names the request that refuses it.
	later := now.Add(time.Hour)
	created, blocking, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-1", later), later.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNil)
	c.Assert(blocking, qt.IsNotNil)
	c.Assert(blocking.ID, qt.Equals, first.ID)

	// Another user is not affected.
	other, blocking, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-2", later), later.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(blocking, qt.IsNil)
	c.Assert(other, qt.IsNotNil)

	// After the interval the user may request again.
	nextDay := now.Add(25 * time.Hour)
	again, blocking, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-1", nextDay), nextDay.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(blocking, qt.IsNil)
	c.Assert(again, qt.IsNotNil)

	list, err := r.ListByUserID(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 2)
	c.Assert(list[0].ID, qt.Equals, again.ID)
	c.Assert(list[1].ID, qt.Equals, first.ID)
}

func TestPersonalDataExportRegistryMemory_OwnerScopedReads(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewPersonalDataExportRegistry()
	now := time.Now().UTC()

	mine, _, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-1", now), now.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)

	got, err := r.GetByUser(ctx, "user-1", mine.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, mine.ID)

	_, err = r.GetByUser(ctx, "user-2", mine.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	list, err := r.ListByUserID(ctx, "user-2")
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 0)
}

func TestPersonalDataExportRegistryMemory_ClaimUpdateExpire(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewPersonalDataExportRegistry()
	now := time.Now().UTC()

	export, _, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-1", now), now.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)

	pending, err := r.ListPending(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(pending, qt.HasLen, 1)

	claimed, err := r.ClaimPending(ctx, export.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	claimed, err = r.ClaimPending(ctx, export.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsFalse)

	export.Status = models.PersonalDataExportStatusCompleted
	export.BlobKey = "t/tenant-1/personal-exports/x.tar.gz"
	export.SizeBytes = 42
	export.CompletedAt = new(now)
	export.ExpiresAt = now.Add(time.Hour)
	// The owner cannot change through Update.
	export.UserID = "user-2"
	updated, err := r.Update(ctx, *export)
	c.Assert(err, qt.IsNil)
	c.Assert(updated.UserID, qt.Equals, "user-1")
	c.Assert(updated.SizeBytes, qt.Equals, int64(42))
	c.Assert(updated.IsDownloadable(now), qt.IsTrue)

	expired, err := r.ListExpired(ctx, now, 10)
	c.Assert(err, qt.IsNil)
	c.Assert(expired, qt.HasLen, 0)
	expired, err = r.ListExpired(ctx, now.Add(2*time.Hour), 10)
	c.Assert(err, qt.IsNil)
	c.Assert(expired, qt.HasLen, 1)

	c.Assert(r.Delete(ctx, export.ID), qt.IsNil)
	c.Assert(r.Delete(ctx, export.ID), qt.IsNil)
	_, err = r.Get(ctx, export.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// A build that started before the cutoff and is still in progress goes back
// to the queue until the export has started its last allowed build; a dead
// build after that fails it. A build that started after the cutoff stays.
func TestPersonalDataExportRegistryMemory_RecoverStalled(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewPersonalDataExportRegistry()
	now := time.Now().UTC()
	sweep := registry.PersonalDataExportStalledSweep{
		StartedBefore: now.Add(-time.Hour),
		Now:           now,
		MaxBuilds:     2,
		ErrorMessage:  "the worker stopped",
	}

	stalled, _, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-1", now), now.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)
	running, _, err := r.CreateUnlessRequestedSince(ctx, personalExportRequest("user-2", now), now.Add(-24*time.Hour))
	c.Assert(err, qt.IsNil)
	claimed, err := r.ClaimPending(ctx, stalled.ID, now.Add(-2*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	claimed, err = r.ClaimPending(ctx, running.ID, now.Add(-time.Minute))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)

	requeued, failed, err := r.RecoverStalled(ctx, sweep)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 1)
	c.Assert(failed, qt.Equals, 0)

	got, err := r.Get(ctx, stalled.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusPending)
	c.Assert(got.StartedAt, qt.IsNil)
	c.Assert(got.AttemptCount, qt.Equals, 1)
	got, err = r.Get(ctx, running.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusInProgress)

	// The second build is the last one; when it dies too, the export fails.
	claimed, err = r.ClaimPending(ctx, stalled.ID, now.Add(-2*time.Hour))
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	requeued, failed, err = r.RecoverStalled(ctx, sweep)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 1)

	got, err = r.Get(ctx, stalled.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, "the worker stopped")
	c.Assert(got.CompletedAt, qt.IsNotNil)
	c.Assert(got.CompletedAt.Equal(now), qt.IsTrue)
	c.Assert(got.AttemptCount, qt.Equals, 2)

	// A failed export is not swept again.
	requeued, failed, err = r.RecoverStalled(ctx, sweep)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 0)
}
