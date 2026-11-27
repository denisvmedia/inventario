package registrytest

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/shopspring/decimal"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// FailedJobGroup is one group of the FailedJobEnv tenant: its id, the
// context that carries the member and the group, and the member's
// registries in it.
type FailedJobGroup struct {
	ID  string
	Ctx context.Context
	Set *registry.Set
}

// FailedJobEnv is what RunFailedJobContract needs from a backend: the
// failed-jobs registry, the tenant, two groups of it with the same member,
// the intake service registry the scan worker uses, and the personal data
// export registry with a user of the tenant stored in the users table. Both
// groups start empty.
type FailedJobEnv struct {
	Jobs     registry.FailedJobRegistry
	TenantID string
	// UserID is the member of both groups who starts every job.
	UserID        string
	A, B          FailedJobGroup
	IntakeService registry.IntakeRegistry
	// PersonalExports stores the personal data exports of PersonalUser,
	// a user of the tenant. They start empty.
	PersonalExports registry.PersonalDataExportRegistry
	PersonalUser    *models.User
}

// RunFailedJobContract checks a backend's FailedJobRegistry (#1307): each
// job type's failed jobs are listed from its own table, a group scope never
// reaches another group's jobs nor a personal data export, a retry puts a
// job back in its queue for the types that can be retried, and a discard
// keeps the row. newEnv is called once per subtest.
func RunFailedJobContract(t *testing.T, newEnv func(t *testing.T) FailedJobEnv) {
	tests := []struct {
		name string
		run  func(t *testing.T, env FailedJobEnv)
	}{
		{"lists each type's failed jobs in its scope", failedJobContractList},
		{"a group scope never reaches another group", failedJobContractGroupIsolation},
		{"retry puts a job back in its queue", failedJobContractRetry},
		{"a restore and a currency migration cannot be retried", failedJobContractNotRetryable},
		{"discard keeps the row and its error", failedJobContractDiscard},
		{"newest failure first and the limit", failedJobContractOrder},
		{"an unknown type and a group without a tenant are refused", failedJobContractRefusals},
		{"a discarded scan that is resumed or rescanned shows again when it fails", failedJobContractIntakeRequeue},
		{"only the newest thumbnail job of a file counts", failedJobContractThumbnailNewest},
		{"a personal data export is listed outside a group only", failedJobContractPersonalList},
		{"a retry builds a personal data export again", failedJobContractPersonalRetry},
		{"a discarded personal data export keeps its row", failedJobContractPersonalDiscard},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.run(t, newEnv(t))
		})
	}
}

// failedJobs is one failed job of each type in a group, by type.
type failedJobs map[models.WorkerType]string

// seedFailedJobs stores one failed job of each type in the group, plus a job
// of each table that did not fail, which no list may show.
func seedFailedJobs(c *qt.C, env FailedJobEnv, g FailedJobGroup, failedAt time.Time) failedJobs {
	c.Helper()
	ctx, set := g.Ctx, g.Set
	jobs := failedJobs{}

	completed, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:          models.ExportTypeFullDatabase,
		Status:        models.ExportStatusCompleted,
		Description:   "Nightly backup",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-2 * time.Hour)),
		CompletedDate: models.NewPTimestamp(failedAt.Add(-time.Hour)),
	})
	c.Assert(err, qt.IsNil)

	export, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:          models.ExportTypeFullDatabase,
		Status:        models.ExportStatusFailed,
		Description:   "Weekly backup",
		ErrorMessage:  "disk full",
		AttemptCount:  2,
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Hour)),
		CompletedDate: models.NewPTimestamp(failedAt),
	})
	c.Assert(err, qt.IsNil)
	jobs[models.WorkerTypeExport] = export.ID

	imported, err := set.ExportRegistry.Create(ctx, models.Export{
		Type:          models.ExportTypeImported,
		Imported:      true,
		Status:        models.ExportStatusFailed,
		Description:   "Uploaded backup",
		ErrorMessage:  "not a backup file",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Hour)),
		CompletedDate: models.NewPTimestamp(failedAt),
	})
	c.Assert(err, qt.IsNil)
	jobs[models.WorkerTypeImport] = imported.ID

	restore, err := set.RestoreOperationRegistry.Create(ctx, models.RestoreOperation{
		ExportID:      completed.ID,
		Description:   "Restore nightly backup",
		Status:        models.RestoreStatusFailed,
		Options:       models.RestoreOptions{Strategy: "merge_add"},
		ErrorMessage:  "conflicting ids",
		CreatedDate:   models.NewPTimestamp(failedAt.Add(-time.Hour)),
		CompletedDate: models.NewPTimestamp(failedAt),
	})
	c.Assert(err, qt.IsNil)
	jobs[models.WorkerTypeRestore] = restore.ID

	photo, err := set.FileRegistry.Create(ctx, models.FileEntity{
		Title:    "Kitchen photo",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		File:     &models.File{Path: "kitchen", OriginalPath: "files/kitchen.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)
	hallway, err := set.FileRegistry.Create(ctx, models.FileEntity{
		Title:    "Hallway photo",
		Type:     models.FileTypeImage,
		Category: models.FileCategoryImages,
		File:     &models.File{Path: "hallway", OriginalPath: "files/hallway.jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)
	_, err = set.ThumbnailGenerationJobRegistry.Create(ctx, models.ThumbnailGenerationJob{
		FileID:      hallway.ID,
		Status:      models.ThumbnailStatusPending,
		MaxAttempts: 3,
		CreatedAt:   failedAt,
		UpdatedAt:   failedAt,
	})
	c.Assert(err, qt.IsNil)
	thumb, err := set.ThumbnailGenerationJobRegistry.Create(ctx, models.ThumbnailGenerationJob{
		FileID:                photo.ID,
		Status:                models.ThumbnailStatusFailed,
		AttemptCount:          3,
		MaxAttempts:           3,
		ErrorMessage:          "unsupported image",
		ProcessingStartedAt:   new(failedAt.Add(-time.Minute)),
		ProcessingCompletedAt: new(failedAt),
		CreatedAt:             failedAt.Add(-time.Hour),
		UpdatedAt:             failedAt,
	})
	c.Assert(err, qt.IsNil)
	jobs[models.WorkerTypeThumbnail] = thumb.ID

	receipt, err := set.FileRegistry.Create(ctx, models.FileEntity{
		Title:    "Receipt",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		File:     &models.File{Path: "receipt", OriginalPath: "files/receipt.pdf", Ext: ".pdf", MIMEType: "application/pdf"},
	})
	c.Assert(err, qt.IsNil)
	session, _, err := set.IntakeRegistry.CreateSession(ctx, models.IntakeSession{ClientKey: "failed-" + g.ID}, []registry.IntakeSourceInput{
		{FileIDs: []string{receipt.ID}},
	}, 0)
	c.Assert(err, qt.IsNil)
	source, err := env.IntakeService.ClaimNextSource(ctx, time.Now().Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(source.SessionID, qt.Equals, session.ID)
	c.Assert(env.IntakeService.FailSource(ctx, source.ID, source.Attempts, "unreadable"), qt.IsNil)
	jobs[models.WorkerTypeIntake] = source.ID

	migration, err := set.CurrencyMigrationRegistry.Create(ctx, models.CurrencyMigration{
		Status:       models.CurrencyMigrationStatusFailed,
		FromCurrency: "USD",
		ToCurrency:   "EUR",
		ExchangeRate: decimal.RequireFromString("0.9"),
		ErrorMessage: "the worker stopped",
		CreatedAt:    failedAt.Add(-time.Hour),
		CompletedAt:  new(failedAt),
	})
	c.Assert(err, qt.IsNil)
	jobs[models.WorkerTypeCurrencyMigration] = migration.ID

	return jobs
}

func groupScope(env FailedJobEnv, g FailedJobGroup) registry.FailedJobScope {
	return registry.FailedJobScope{TenantID: env.TenantID, GroupID: g.ID}
}

func failedIDs(c *qt.C, env FailedJobEnv, jobType models.WorkerType, scope registry.FailedJobScope) []string {
	c.Helper()
	jobs, err := env.Jobs.ListFailed(context.Background(), jobType, scope, 0)
	c.Assert(err, qt.IsNil)
	ids := make([]string, 0, len(jobs))
	for _, j := range jobs {
		ids = append(ids, j.ID)
	}
	return ids
}

func failedJobContractList(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	failedAt := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	a := seedFailedJobs(c, env, env.A, failedAt)
	b := seedFailedJobs(c, env, env.B, failedAt)
	ctx := context.Background()

	titles := map[models.WorkerType]string{
		models.WorkerTypeExport:            "Weekly backup",
		models.WorkerTypeImport:            "Uploaded backup",
		models.WorkerTypeRestore:           "Restore nightly backup",
		models.WorkerTypeThumbnail:         "Kitchen photo",
		models.WorkerTypeIntake:            "Receipt",
		models.WorkerTypeCurrencyMigration: "USD → EUR",
	}
	errs := map[models.WorkerType]string{
		models.WorkerTypeExport:            "disk full",
		models.WorkerTypeImport:            "not a backup file",
		models.WorkerTypeRestore:           "conflicting ids",
		models.WorkerTypeThumbnail:         "unsupported image",
		models.WorkerTypeIntake:            "unreadable",
		models.WorkerTypeCurrencyMigration: "the worker stopped",
	}
	attempts := map[models.WorkerType]int{
		models.WorkerTypeExport:            2,
		models.WorkerTypeImport:            1,
		models.WorkerTypeRestore:           1,
		models.WorkerTypeThumbnail:         3,
		models.WorkerTypeIntake:            1,
		models.WorkerTypeCurrencyMigration: 1,
	}

	for _, jobType := range models.FailedJobGroupTypes() {
		c.Run(string(jobType), func(c *qt.C) {
			c.Assert(failedIDs(c, env, jobType, groupScope(env, env.A)), qt.DeepEquals, []string{a[jobType]})
			c.Assert(failedIDs(c, env, jobType, groupScope(env, env.B)), qt.DeepEquals, []string{b[jobType]})
			c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{TenantID: env.TenantID}), qt.HasLen, 2)
			c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{}), qt.HasLen, 2)
			c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{TenantID: "another-tenant"}), qt.HasLen, 0)

			n, err := env.Jobs.CountFailed(ctx, jobType, groupScope(env, env.A))
			c.Assert(err, qt.IsNil)
			c.Assert(n, qt.Equals, 1)
			n, err = env.Jobs.CountFailed(ctx, jobType, registry.FailedJobScope{})
			c.Assert(err, qt.IsNil)
			c.Assert(n, qt.Equals, 2)

			job, err := env.Jobs.GetFailed(ctx, jobType, groupScope(env, env.A), a[jobType])
			c.Assert(err, qt.IsNil)
			c.Assert(job.Type, qt.Equals, jobType)
			c.Assert(job.TenantID, qt.Equals, env.TenantID)
			c.Assert(job.GroupID, qt.Equals, env.A.ID)
			c.Assert(job.UserID, qt.Equals, env.UserID)
			c.Assert(job.Title, qt.Equals, titles[jobType])
			c.Assert(job.Error, qt.Equals, errs[jobType])
			c.Assert(job.Attempts, qt.Equals, attempts[jobType])
			c.Assert(job.FailedAt, qt.IsNotNil)
			if jobType != models.WorkerTypeIntake {
				// A scan's failure time is when the sweep or the worker
				// stored it, not a time the test chose.
				c.Assert(job.FailedAt.Equal(failedAt), qt.IsTrue, qt.Commentf("failed at %s", job.FailedAt))
			}
		})
	}
}

func failedJobContractGroupIsolation(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	now := time.Now().UTC()
	seedFailedJobs(c, env, env.A, now.Add(-time.Hour))
	b := seedFailedJobs(c, env, env.B, now.Add(-time.Hour))
	ctx := context.Background()
	scopeA := groupScope(env, env.A)

	for _, jobType := range models.FailedJobGroupTypes() {
		c.Run(string(jobType), func(c *qt.C) {
			_, err := env.Jobs.GetFailed(ctx, jobType, scopeA, b[jobType])
			c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
			if models.FailedJobRetryable(jobType) {
				c.Assert(env.Jobs.Retry(ctx, jobType, scopeA, b[jobType], now), qt.ErrorIs, registry.ErrNotFound)
			}
			c.Assert(env.Jobs.Discard(ctx, jobType, scopeA, b[jobType], now), qt.ErrorIs, registry.ErrNotFound)
			c.Assert(env.Jobs.Discard(ctx, jobType, registry.FailedJobScope{TenantID: "another-tenant"}, b[jobType], now),
				qt.ErrorIs, registry.ErrNotFound)

			// The other group's job is still failed and listed.
			c.Assert(failedIDs(c, env, jobType, groupScope(env, env.B)), qt.DeepEquals, []string{b[jobType]})
		})
	}
}

func failedJobContractRetry(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	now := time.Now().UTC().Truncate(time.Second)
	a := seedFailedJobs(c, env, env.A, now.Add(-time.Hour))
	ctx := context.Background()
	scopeA := groupScope(env, env.A)

	for _, jobType := range models.FailedJobGroupTypes() {
		if !models.FailedJobRetryable(jobType) {
			continue
		}
		c.Assert(env.Jobs.Retry(ctx, jobType, scopeA, a[jobType], now), qt.IsNil, qt.Commentf("%s", jobType))
		c.Assert(failedIDs(c, env, jobType, scopeA), qt.HasLen, 0, qt.Commentf("%s", jobType))
		c.Assert(env.Jobs.Retry(ctx, jobType, scopeA, a[jobType], now), qt.ErrorIs, registry.ErrNotFound,
			qt.Commentf("%s: a job retried once is no longer failed", jobType))
	}

	g := env.A
	export, err := g.Set.ExportRegistry.Get(g.Ctx, a[models.WorkerTypeExport])
	c.Assert(err, qt.IsNil)
	c.Assert(export.Status, qt.Equals, models.ExportStatusPending)
	c.Assert(export.ErrorMessage, qt.Equals, "")
	c.Assert(export.CompletedDate, qt.IsNil)
	c.Assert(export.AttemptCount, qt.Equals, 2, qt.Commentf("the attempts of earlier runs are kept"))

	imported, err := g.Set.ExportRegistry.Get(g.Ctx, a[models.WorkerTypeImport])
	c.Assert(err, qt.IsNil)
	c.Assert(imported.Status, qt.Equals, models.ExportStatusPending)
	c.Assert(imported.Type, qt.Equals, models.ExportTypeImported)

	thumb, err := g.Set.ThumbnailGenerationJobRegistry.Get(g.Ctx, a[models.WorkerTypeThumbnail])
	c.Assert(err, qt.IsNil)
	c.Assert(thumb.Status, qt.Equals, models.ThumbnailStatusPending)
	c.Assert(thumb.AttemptCount, qt.Equals, 0, qt.Commentf("the worker's own retries start over"))
	c.Assert(thumb.ErrorMessage, qt.Equals, "")
	c.Assert(thumb.ProcessingCompletedAt, qt.IsNil)

	source, err := g.Set.IntakeRegistry.GetSource(g.Ctx, a[models.WorkerTypeIntake])
	c.Assert(err, qt.IsNil)
	c.Assert(source.Status, qt.Equals, models.IntakeSourcePending)
	c.Assert(source.ErrorCode, qt.Equals, "")
	c.Assert(source.Attempts, qt.Equals, 1, qt.Commentf("attempts never go down"))
	c.Assert(source.CountedAttempts(), qt.Equals, 0, qt.Commentf("the retry brings a fresh allowance"))
	again, err := env.IntakeService.ClaimNextSource(g.Ctx, now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(again.ID, qt.Equals, source.ID, qt.Commentf("the scan is due again at once"))
}

func failedJobContractNotRetryable(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	now := time.Now().UTC()
	a := seedFailedJobs(c, env, env.A, now.Add(-time.Hour))
	ctx := context.Background()
	scopeA := groupScope(env, env.A)

	for _, jobType := range models.FailedJobGroupTypes() {
		if models.FailedJobRetryable(jobType) {
			continue
		}
		c.Assert(env.Jobs.Retry(ctx, jobType, scopeA, a[jobType], now), qt.ErrorIs, registry.ErrFailedJobNotRetryable,
			qt.Commentf("%s", jobType))
		c.Assert(failedIDs(c, env, jobType, scopeA), qt.DeepEquals, []string{a[jobType]},
			qt.Commentf("%s: a refused retry leaves the job failed", jobType))
	}
}

func failedJobContractDiscard(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	now := time.Now().UTC().Truncate(time.Second)
	a := seedFailedJobs(c, env, env.A, now.Add(-time.Hour))
	ctx := context.Background()
	scopeA := groupScope(env, env.A)

	for _, jobType := range models.FailedJobGroupTypes() {
		c.Assert(env.Jobs.Discard(ctx, jobType, scopeA, a[jobType], now), qt.IsNil, qt.Commentf("%s", jobType))
		c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{}), qt.HasLen, 0, qt.Commentf("%s", jobType))
		n, err := env.Jobs.CountFailed(ctx, jobType, registry.FailedJobScope{})
		c.Assert(err, qt.IsNil)
		c.Assert(n, qt.Equals, 0, qt.Commentf("%s", jobType))
		c.Assert(env.Jobs.Discard(ctx, jobType, scopeA, a[jobType], now), qt.ErrorIs, registry.ErrNotFound,
			qt.Commentf("%s: a job is discarded once", jobType))
		if models.FailedJobRetryable(jobType) {
			c.Assert(env.Jobs.Retry(ctx, jobType, scopeA, a[jobType], now), qt.ErrorIs, registry.ErrNotFound,
				qt.Commentf("%s: a discarded job is not retried", jobType))
		}
	}

	// The rows stay failed, with their errors.
	g := env.A
	export, err := g.Set.ExportRegistry.Get(g.Ctx, a[models.WorkerTypeExport])
	c.Assert(err, qt.IsNil)
	c.Assert(export.Status, qt.Equals, models.ExportStatusFailed)
	c.Assert(export.ErrorMessage, qt.Equals, "disk full")
	c.Assert(export.DiscardedAt, qt.IsNotNil)

	restore, err := g.Set.RestoreOperationRegistry.Get(g.Ctx, a[models.WorkerTypeRestore])
	c.Assert(err, qt.IsNil)
	c.Assert(restore.Status, qt.Equals, models.RestoreStatusFailed)
	c.Assert(restore.ErrorMessage, qt.Equals, "conflicting ids")

	thumb, err := g.Set.ThumbnailGenerationJobRegistry.Get(g.Ctx, a[models.WorkerTypeThumbnail])
	c.Assert(err, qt.IsNil)
	c.Assert(thumb.Status, qt.Equals, models.ThumbnailStatusFailed)
	c.Assert(thumb.ErrorMessage, qt.Equals, "unsupported image")
	c.Assert(thumb.DiscardedAt, qt.IsNotNil)

	source, err := g.Set.IntakeRegistry.GetSource(g.Ctx, a[models.WorkerTypeIntake])
	c.Assert(err, qt.IsNil)
	c.Assert(source.Status, qt.Equals, models.IntakeSourceFailed)
	c.Assert(source.ErrorCode, qt.Equals, "unreadable")

	migration, err := g.Set.CurrencyMigrationRegistry.Get(g.Ctx, a[models.WorkerTypeCurrencyMigration])
	c.Assert(err, qt.IsNil)
	c.Assert(migration.Status, qt.Equals, models.CurrencyMigrationStatusFailed)
	c.Assert(migration.ErrorMessage, qt.Equals, "the worker stopped")
}

func failedJobContractOrder(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	g := env.A
	var ids []string
	for i, at := range []time.Time{base, base.Add(2 * time.Hour), base.Add(time.Hour)} {
		e, err := g.Set.ExportRegistry.Create(g.Ctx, models.Export{
			Type:          models.ExportTypeFullDatabase,
			Status:        models.ExportStatusFailed,
			Description:   "backup " + string(rune('a'+i)),
			ErrorMessage:  "failed",
			CreatedDate:   models.NewPTimestamp(at.Add(-time.Minute)),
			CompletedDate: models.NewPTimestamp(at),
		})
		c.Assert(err, qt.IsNil)
		ids = append(ids, e.ID)
	}
	c.Assert(failedIDs(c, env, models.WorkerTypeExport, groupScope(env, g)), qt.DeepEquals, []string{ids[1], ids[2], ids[0]})

	limited, err := env.Jobs.ListFailed(ctx, models.WorkerTypeExport, groupScope(env, g), 1)
	c.Assert(err, qt.IsNil)
	c.Assert(limited, qt.HasLen, 1)
	c.Assert(limited[0].ID, qt.Equals, ids[1])
}

func failedJobContractRefusals(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	now := time.Now()

	_, err := env.Jobs.ListFailed(ctx, models.WorkerTypeWeeklyDigest, registry.FailedJobScope{}, 0)
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
	_, err = env.Jobs.CountFailed(ctx, "nope", registry.FailedJobScope{})
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
	c.Assert(env.Jobs.Retry(ctx, "nope", registry.FailedJobScope{}, "id", now), qt.ErrorIs, registry.ErrUnknownFailedJobType)
	c.Assert(env.Jobs.Discard(ctx, "nope", registry.FailedJobScope{}, "id", now), qt.ErrorIs, registry.ErrUnknownFailedJobType)

	_, err = env.Jobs.ListFailed(ctx, models.WorkerTypeExport, registry.FailedJobScope{GroupID: env.A.ID}, 0)
	c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
}

// failIntakeSource claims the group's next due source and fails it, as the
// scan worker does after the last attempt.
func failIntakeSource(c *qt.C, env FailedJobEnv, g FailedJobGroup, want string) {
	c.Helper()
	source, err := env.IntakeService.ClaimNextSource(g.Ctx, time.Now().Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(source.ID, qt.Equals, want)
	c.Assert(env.IntakeService.FailSource(g.Ctx, source.ID, source.Attempts, "unreadable"), qt.IsNil)
}

func failedJobContractIntakeRequeue(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	now := time.Now().UTC().Truncate(time.Second)
	g := env.A
	id := seedFailedJobs(c, env, g, now.Add(-time.Hour))[models.WorkerTypeIntake]
	scope := groupScope(env, g)
	source, err := g.Set.IntakeRegistry.GetSource(g.Ctx, id)
	c.Assert(err, qt.IsNil)

	c.Assert(env.Jobs.Discard(context.Background(), models.WorkerTypeIntake, scope, id, now), qt.IsNil)
	_, err = g.Set.IntakeRegistry.ResumeSession(g.Ctx, source.SessionID, now)
	c.Assert(err, qt.IsNil)
	failIntakeSource(c, env, g, id)
	c.Assert(failedIDs(c, env, models.WorkerTypeIntake, scope), qt.DeepEquals, []string{id},
		qt.Commentf("resuming the session clears the discard"))

	c.Assert(env.Jobs.Discard(context.Background(), models.WorkerTypeIntake, scope, id, now), qt.IsNil)
	_, err = g.Set.IntakeRegistry.RescanSource(g.Ctx, id, now)
	c.Assert(err, qt.IsNil)
	failIntakeSource(c, env, g, id)
	c.Assert(failedIDs(c, env, models.WorkerTypeIntake, scope), qt.DeepEquals, []string{id},
		qt.Commentf("a rescan clears the discard"))
	n, err := env.Jobs.CountFailed(context.Background(), models.WorkerTypeIntake, registry.FailedJobScope{})
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
}

func failedJobContractThumbnailNewest(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	g := env.A
	base := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	scope := groupScope(env, g)
	newFile := func(name string) string {
		f, err := g.Set.FileRegistry.Create(g.Ctx, models.FileEntity{
			Title:    name,
			Type:     models.FileTypeImage,
			Category: models.FileCategoryImages,
			File:     &models.File{Path: name, OriginalPath: "files/" + name + ".jpg", Ext: ".jpg", MIMEType: "image/jpeg"},
		})
		c.Assert(err, qt.IsNil)
		return f.ID
	}
	newJob := func(fileID string, status models.ThumbnailGenerationStatus, at time.Time) string {
		job, err := g.Set.ThumbnailGenerationJobRegistry.Create(g.Ctx, models.ThumbnailGenerationJob{
			FileID:                fileID,
			Status:                status,
			AttemptCount:          3,
			MaxAttempts:           3,
			ErrorMessage:          "unsupported image",
			ProcessingCompletedAt: new(at),
			CreatedAt:             at,
			UpdatedAt:             at,
		})
		c.Assert(err, qt.IsNil)
		return job.ID
	}

	// A file failed twice: only the newer failure counts.
	twice := newFile("twice")
	older := newJob(twice, models.ThumbnailStatusFailed, base)
	newer := newJob(twice, models.ThumbnailStatusFailed, base.Add(time.Hour))
	// A file failed and was tried again since: the failure is history.
	retried := newFile("retried")
	stale := newJob(retried, models.ThumbnailStatusFailed, base)
	newJob(retried, models.ThumbnailStatusPending, base.Add(time.Hour))

	c.Assert(failedIDs(c, env, models.WorkerTypeThumbnail, scope), qt.DeepEquals, []string{newer})
	n, err := env.Jobs.CountFailed(ctx, models.WorkerTypeThumbnail, registry.FailedJobScope{})
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	for _, id := range []string{older, stale} {
		_, err = env.Jobs.GetFailed(ctx, models.WorkerTypeThumbnail, scope, id)
		c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
		c.Assert(env.Jobs.Retry(ctx, models.WorkerTypeThumbnail, scope, id, base), qt.ErrorIs, registry.ErrNotFound)
		c.Assert(env.Jobs.Discard(ctx, models.WorkerTypeThumbnail, scope, id, base), qt.ErrorIs, registry.ErrNotFound)
	}
}

// seedPersonalExports stores, for the env's personal user, a personal data
// export whose build failed at failedAt, and one that finished, which no
// list may show. It returns the failed one.
func seedPersonalExports(c *qt.C, env FailedJobEnv, failedAt time.Time) *models.PersonalDataExport {
	c.Helper()
	ctx := context.Background()
	request := func(at time.Time) *models.PersonalDataExport {
		created, blocking, err := env.PersonalExports.CreateUnlessRequestedSince(ctx, models.PersonalDataExport{
			TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: env.TenantID, UserID: env.PersonalUser.ID},
			CreatedAt:               at,
			ExpiresAt:               at.Add(models.PersonalDataExportRetention),
		}, at)
		c.Assert(err, qt.IsNil)
		c.Assert(blocking, qt.IsNil)
		claimed, err := env.PersonalExports.ClaimPending(ctx, created.ID, at)
		c.Assert(err, qt.IsNil)
		c.Assert(claimed, qt.IsTrue)
		got, err := env.PersonalExports.Get(ctx, created.ID)
		c.Assert(err, qt.IsNil)
		return got
	}

	done := request(failedAt.Add(-2 * time.Hour))
	done.Status = models.PersonalDataExportStatusCompleted
	done.BlobKey = "done.tar.gz"
	done.CompletedAt = new(failedAt.Add(-time.Hour))
	_, err := env.PersonalExports.Update(ctx, *done)
	c.Assert(err, qt.IsNil)

	failed := request(failedAt.Add(-time.Hour))
	failed.Status = models.PersonalDataExportStatusFailed
	failed.ErrorMessage = "the account is not active"
	failed.CompletedAt = new(failedAt)
	stored, err := env.PersonalExports.Update(ctx, *failed)
	c.Assert(err, qt.IsNil)
	return stored
}

func failedJobContractPersonalList(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	failedAt := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	export := seedPersonalExports(c, env, failedAt)
	jobType := models.FailedJobTypePersonalDataExport

	c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{}), qt.DeepEquals, []string{export.ID})
	c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{TenantID: env.TenantID}), qt.DeepEquals, []string{export.ID})
	c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{TenantID: "another-tenant"}), qt.HasLen, 0)
	n, err := env.Jobs.CountFailed(ctx, jobType, registry.FailedJobScope{})
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)

	job, err := env.Jobs.GetFailed(ctx, jobType, registry.FailedJobScope{}, export.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(job.Type, qt.Equals, jobType)
	c.Assert(job.TenantID, qt.Equals, env.TenantID)
	c.Assert(job.GroupID, qt.Equals, "")
	c.Assert(job.UserID, qt.Equals, env.PersonalUser.ID)
	c.Assert(job.Title, qt.Equals, env.PersonalUser.Email)
	c.Assert(job.Error, qt.Equals, "the account is not active")
	c.Assert(job.Attempts, qt.Equals, 1)
	c.Assert(job.FailedAt, qt.IsNotNil)
	c.Assert(job.FailedAt.Equal(failedAt), qt.IsTrue, qt.Commentf("failed at %s", job.FailedAt))

	// A group scope does not know the type: the export belongs to a user.
	scopeA := groupScope(env, env.A)
	_, err = env.Jobs.ListFailed(ctx, jobType, scopeA, 0)
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
	_, err = env.Jobs.CountFailed(ctx, jobType, scopeA)
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
	_, err = env.Jobs.GetFailed(ctx, jobType, scopeA, export.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrUnknownFailedJobType)
	c.Assert(env.Jobs.Retry(ctx, jobType, scopeA, export.ID, failedAt), qt.ErrorIs, registry.ErrUnknownFailedJobType)
	c.Assert(env.Jobs.Discard(ctx, jobType, scopeA, export.ID, failedAt), qt.ErrorIs, registry.ErrUnknownFailedJobType)
	c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{}), qt.DeepEquals, []string{export.ID})
}

func failedJobContractPersonalRetry(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	export := seedPersonalExports(c, env, now.Add(-time.Hour))
	jobType := models.FailedJobTypePersonalDataExport

	c.Assert(env.Jobs.Retry(ctx, jobType, registry.FailedJobScope{TenantID: env.TenantID}, export.ID, now), qt.IsNil)
	c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{}), qt.HasLen, 0)
	c.Assert(env.Jobs.Retry(ctx, jobType, registry.FailedJobScope{}, export.ID, now), qt.ErrorIs, registry.ErrNotFound,
		qt.Commentf("an export retried once is no longer failed"))

	got, err := env.PersonalExports.Get(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusPending)
	c.Assert(got.ErrorMessage, qt.Equals, "")
	c.Assert(got.StartedAt, qt.IsNil)
	c.Assert(got.CompletedAt, qt.IsNil)
	c.Assert(got.BlobKey, qt.Equals, "")
	c.Assert(got.SizeBytes, qt.Equals, int64(0))
	c.Assert(got.AttemptCount, qt.Equals, 1, qt.Commentf("the builds of earlier runs are kept"))
	c.Assert(got.ExpiresAt.Equal(now.Add(models.PersonalDataExportRetention)), qt.IsTrue,
		qt.Commentf("the row is kept while the build waits: expires at %s", got.ExpiresAt))
	c.Assert(got.CreatedAt.Equal(export.CreatedAt), qt.IsTrue, qt.Commentf("the request keeps its time"))

	claimed, err := env.PersonalExports.ClaimPending(ctx, export.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue, qt.Commentf("the worker builds it again"))
}

func failedJobContractPersonalDiscard(t *testing.T, env FailedJobEnv) {
	c := qt.New(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Second)
	export := seedPersonalExports(c, env, now.Add(-time.Hour))
	jobType := models.FailedJobTypePersonalDataExport

	c.Assert(env.Jobs.Discard(ctx, jobType, registry.FailedJobScope{TenantID: "another-tenant"}, export.ID, now), qt.ErrorIs, registry.ErrNotFound)
	c.Assert(env.Jobs.Discard(ctx, jobType, registry.FailedJobScope{}, export.ID, now), qt.IsNil)
	c.Assert(failedIDs(c, env, jobType, registry.FailedJobScope{}), qt.HasLen, 0)
	c.Assert(env.Jobs.Discard(ctx, jobType, registry.FailedJobScope{}, export.ID, now), qt.ErrorIs, registry.ErrNotFound,
		qt.Commentf("an export is discarded once"))
	c.Assert(env.Jobs.Retry(ctx, jobType, registry.FailedJobScope{}, export.ID, now), qt.ErrorIs, registry.ErrNotFound,
		qt.Commentf("a discarded export is not retried"))

	got, err := env.PersonalExports.Get(ctx, export.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, "the account is not active")
	c.Assert(got.DiscardedAt, qt.IsNotNil)
}
