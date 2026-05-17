package services_test

import (
	"context"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// recordingStorageQuotaEmailService captures
// SendStorageQuotaWarningEmail invocations so tests can assert (a) the
// number of sends and (b) the (group, threshold, percent) tuple each
// invocation matched. Every other Send* method is a no-op — the
// storage quota service only ever calls
// SendStorageQuotaWarningEmail.
type recordingStorageQuotaEmailService struct {
	mu    sync.Mutex
	calls []recordedStorageQuotaEmail
}

type recordedStorageQuotaEmail struct {
	to               string
	name             string
	groupName        string
	thresholdPercent int
	usagePercent     int
	usedHuman        string
	quotaHuman       string
	breakdownLines   []string
	filesURL         string
	settingsURL      string
}

func (r *recordingStorageQuotaEmailService) SendVerificationEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendPasswordResetEmail(_ context.Context, _ string, _ string, _ string) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendPasswordChangedEmail(_ context.Context, _ string, _ string, _ time.Time) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendWelcomeEmail(_ context.Context, _ string, _ string) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendWarrantyReminderEmail(_ context.Context, _, _, _, _, _ string, _ int) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendGroupInviteEmail(_ context.Context, _, _, _, _, _ string, _ time.Time) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendLoanReminderEmail(_ context.Context, _, _, _, _, _, _, _, _ string, _ int) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendMaintenanceReminderEmail(_ context.Context, _, _, _, _, _, _ string, _ int) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendFeedbackEmail(_ context.Context, _, _, _, _, _, _, _ string, _ []string) error {
	return nil
}

func (r *recordingStorageQuotaEmailService) SendStorageQuotaWarningEmail(_ context.Context, to, name, groupName string, thresholdPercent, usagePercent int, usedHuman, quotaHuman string, breakdownLines []string, filesURL, settingsURL string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, recordedStorageQuotaEmail{
		to:               to,
		name:             name,
		groupName:        groupName,
		thresholdPercent: thresholdPercent,
		usagePercent:     usagePercent,
		usedHuman:        usedHuman,
		quotaHuman:       quotaHuman,
		breakdownLines:   append([]string(nil), breakdownLines...),
		filesURL:         filesURL,
		settingsURL:      settingsURL,
	})
	return nil
}

func (r *recordingStorageQuotaEmailService) snapshot() []recordedStorageQuotaEmail {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]recordedStorageQuotaEmail, len(r.calls))
	copy(out, r.calls)
	return out
}

// storageQuotaFixture spins up a fresh memory FactorySet with one
// user + tenant + group, returning the bits the tests need to set
// up file bytes and instantiate the service. testQuotaBytes pins
// the quota tiny so the test doesn't need to stage hundreds of MiB
// of bytes to cross a 90% threshold.
type storageQuotaFixture struct {
	factorySet     *registry.FactorySet
	tenantID       string
	userID         string
	groupID        string
	groupName      string
	groupSlug      string
	testQuotaBytes int64
	serviceRegSet  *registry.Set
}

func newStorageQuotaFixture(c *qt.C) *storageQuotaFixture {
	c.Helper()
	factorySet := memory.NewFactorySet()
	const tenantID = "sq-tenant"
	const testQuotaBytes int64 = 100

	_, err := factorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		EntityID: models.EntityID{ID: tenantID},
		Name:     "SQ Tenant",
		Slug:     "sq-tenant",
	})
	c.Assert(err, qt.IsNil)

	user, err := factorySet.UserRegistry.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               "admin@example.com",
		Name:                "SQ Admin",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	group, err := factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "SQ Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           user.ID,
	})
	c.Assert(err, qt.IsNil)

	return &storageQuotaFixture{
		factorySet:     factorySet,
		tenantID:       tenantID,
		userID:         user.ID,
		groupID:        group.ID,
		groupName:      group.Name,
		groupSlug:      group.Slug,
		testQuotaBytes: testQuotaBytes,
		serviceRegSet:  factorySet.CreateServiceRegistrySet(),
	}
}

func (f *storageQuotaFixture) addFileBytes(c *qt.C, sizeBytes int64) string {
	c.Helper()
	file, err := f.serviceRegSet.FileRegistry.Create(context.Background(), models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        f.tenantID,
			GroupID:         f.groupID,
			CreatedByUserID: f.userID,
		},
		Title:    "doc",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		File: &models.File{
			Path:         "doc",
			OriginalPath: "doc.txt",
			Ext:          ".txt",
			MIMEType:     "text/plain",
			SizeBytes:    sizeBytes,
		},
	})
	c.Assert(err, qt.IsNil)
	return file.ID
}

func (f *storageQuotaFixture) deleteFile(c *qt.C, id string) {
	c.Helper()
	c.Assert(f.serviceRegSet.FileRegistry.Delete(context.Background(), id), qt.IsNil)
}

func (f *storageQuotaFixture) newService(emailSvc services.EmailService) *services.StorageQuotaReminderService {
	return services.NewStorageQuotaReminderService(
		f.factorySet,
		emailSvc,
		func(slug string) string { return "https://example.test/g/" + slug + "/files" },
		func(slug string) string { return "https://example.test/g/" + slug + "/settings" },
	).WithQuotaBytesFor(func(_ string) int64 { return f.testQuotaBytes })
}

// TestStorageQuotaReminderService_BelowThreshold_NoEmail pins the
// trivial path: a group nowhere near the threshold produces no
// idempotency rows + no emails.
func TestStorageQuotaReminderService_BelowThreshold_NoEmail(t *testing.T) {
	c := qt.New(t)
	f := newStorageQuotaFixture(c)
	f.addFileBytes(c, 50) // 50/100 = 50% — below the 90% threshold.

	email := &recordingStorageQuotaEmailService{}
	svc := f.newService(email)

	stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)

	has, err := f.factorySet.StorageQuotaReminderRegistry.HasSent(context.Background(), f.groupID, int(models.StorageQuota90Percent))
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsFalse)
}

// TestStorageQuotaReminderService_CrossesThreshold_SendsOnce pins the
// "89 → 90 sends, 91 → 92 does NOT re-send" half of the acceptance
// criteria. Idempotency row guarantees the second tick is a no-op
// even as usage keeps rising past the threshold.
func TestStorageQuotaReminderService_CrossesThreshold_SendsOnce(t *testing.T) {
	c := qt.New(t)
	f := newStorageQuotaFixture(c)
	f.addFileBytes(c, 90) // 90/100 = 90% exactly — threshold matched.

	email := &recordingStorageQuotaEmailService{}
	svc := f.newService(email)

	// Tick 1: crosses 90% → one email, one idempotency row.
	stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 1)
	c.Assert(stats.SentByThreshold[models.StorageQuota90Percent], qt.Equals, 1)
	calls := email.snapshot()
	c.Assert(calls, qt.HasLen, 1)
	c.Assert(calls[0].thresholdPercent, qt.Equals, 90)
	c.Assert(calls[0].usagePercent, qt.Equals, 90)
	c.Assert(calls[0].to, qt.Equals, "admin@example.com")
	c.Assert(calls[0].groupName, qt.Equals, f.groupName)
	c.Assert(calls[0].filesURL, qt.Contains, f.groupSlug)
	c.Assert(calls[0].settingsURL, qt.Contains, f.groupSlug)

	// Tick 2: same usage, same threshold → idempotency row blocks the
	// second emission.
	stats, err = svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 1)

	// Tick 3: usage rises further (91 → 92) — still no resend until
	// the row is reset.
	f.addFileBytes(c, 5) // 95/100 = 95%.
	stats, err = svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

// TestStorageQuotaReminderService_DropsBelowThreshold_ResetsRow
// pins the "90 → 89 should reset" half of the acceptance criteria.
// Once usage falls back below the threshold the worker deletes the
// row so a future re-crossing fires a fresh email.
func TestStorageQuotaReminderService_DropsBelowThreshold_ResetsRow(t *testing.T) {
	c := qt.New(t)
	f := newStorageQuotaFixture(c)
	fileID := f.addFileBytes(c, 90) // start at the threshold.

	email := &recordingStorageQuotaEmailService{}
	svc := f.newService(email)

	// Tick 1: row + email.
	_, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	has, err := f.factorySet.StorageQuotaReminderRegistry.HasSent(context.Background(), f.groupID, int(models.StorageQuota90Percent))
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsTrue)

	// Drop below threshold by deleting the file.
	f.deleteFile(c, fileID)

	// Tick 2: ratio == 0 → reset counter ticks, row removed, no extra
	// email.
	stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Reset(), qt.Equals, 1)
	c.Assert(stats.ResetByThreshold[models.StorageQuota90Percent], qt.Equals, 1)
	c.Assert(stats.Sent(), qt.Equals, 0)
	has, err = f.factorySet.StorageQuotaReminderRegistry.HasSent(context.Background(), f.groupID, int(models.StorageQuota90Percent))
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsFalse)
	c.Assert(email.snapshot(), qt.HasLen, 1)
}

// TestStorageQuotaReminderService_ReCrossesAfterReset_SendsSecondEmail
// pins the full reset → re-cross loop: a group whose usage drops
// below the threshold then rises back above it must receive a
// SECOND email, distinct from the first.
func TestStorageQuotaReminderService_ReCrossesAfterReset_SendsSecondEmail(t *testing.T) {
	c := qt.New(t)
	f := newStorageQuotaFixture(c)
	fileA := f.addFileBytes(c, 90)

	email := &recordingStorageQuotaEmailService{}
	svc := f.newService(email)

	// Tick 1: first email.
	_, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(email.snapshot(), qt.HasLen, 1)

	// Drop below threshold.
	f.deleteFile(c, fileA)
	_, err = svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(email.snapshot(), qt.HasLen, 1, qt.Commentf("no email expected on the reset tick"))

	// Rise back above threshold — fresh row + fresh email.
	f.addFileBytes(c, 95)
	stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 1)
	c.Assert(email.snapshot(), qt.HasLen, 2)
}

// TestStorageQuotaReminderService_NoQuota_NoEmail guards the
// nullable-quota path the plans-aware future relies on: a group with
// zero quota (e.g. unlimited plan) must never fire a reminder.
func TestStorageQuotaReminderService_NoQuota_NoEmail(t *testing.T) {
	c := qt.New(t)
	f := newStorageQuotaFixture(c)
	f.addFileBytes(c, 1000) // far above any default quota.

	email := &recordingStorageQuotaEmailService{}
	svc := services.NewStorageQuotaReminderService(
		f.factorySet,
		email,
		nil, nil,
	).WithQuotaBytesFor(func(string) int64 { return 0 })

	stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 0)
	c.Assert(email.snapshot(), qt.HasLen, 0)
}

// TestStorageQuotaReminderService_EnqueueFailureRetries pins the
// write-after-send ordering: when every recipient's email enqueue
// fails (queue down), the idempotency row must NOT be written so
// the next sweep retries. Mirrors the warranty reminder regression
// test.
func TestStorageQuotaReminderService_EnqueueFailureRetries(t *testing.T) {
	c := qt.New(t)
	f := newStorageQuotaFixture(c)
	f.addFileBytes(c, 90)

	failing := &failingEmailService{}
	svc := f.newService(failing)

	// Tick 1: every enqueue fails → failed counter ticks, no row.
	stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 1)
	c.Assert(stats.Sent(), qt.Equals, 0)
	has, err := f.factorySet.StorageQuotaReminderRegistry.HasSent(context.Background(), f.groupID, int(models.StorageQuota90Percent))
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsFalse)

	// Tick 2: queue recovers (swap out email service) — fresh sweep,
	// row committed, exactly one email enqueued.
	recording := &recordingStorageQuotaEmailService{}
	svc = f.newService(recording)
	stats, err = svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Sent(), qt.Equals, 1)
	c.Assert(recording.snapshot(), qt.HasLen, 1)
}

// TestStorageQuotaReminderService_NoRecipients_RowStillCommitted
// covers the edge case where the group has no admin recipients
// (no memberships AND no CreatedBy user). The worker must commit the
// idempotency row anyway so it doesn't repeatedly sweep an
// unreachable group on every tick.
func TestStorageQuotaReminderService_NoRecipients_RowStillCommitted(t *testing.T) {
	c := qt.New(t)
	factorySet := memory.NewFactorySet()
	const tenantID = "sq-orphan-tenant"
	_, err := factorySet.TenantRegistry.Create(context.Background(), models.Tenant{
		EntityID: models.EntityID{ID: tenantID},
		Name:     "Orphan",
		Slug:     "sq-orphan",
	})
	c.Assert(err, qt.IsNil)
	// Group with empty CreatedBy field — the fixture intentionally
	// skips creating any user / membership rows. Validation requires
	// a non-empty CreatedBy at Create time, so we generate a UUID
	// here but never insert the matching User row — the worker's
	// recipient lookup falls through and out is empty.
	group, err := factorySet.LocationGroupRegistry.Create(context.Background(), models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Slug:                must.Must(models.GenerateGroupSlug()),
		Name:                "Orphan Group",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           "00000000-orphan-user",
	})
	c.Assert(err, qt.IsNil)

	serviceRegSet := factorySet.CreateServiceRegistrySet()
	_, err = serviceRegSet.FileRegistry.Create(context.Background(), models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         group.ID,
			CreatedByUserID: "00000000-orphan-user",
		},
		Title:    "doc",
		Type:     models.FileTypeDocument,
		Category: models.FileCategoryDocuments,
		File: &models.File{
			Path:         "doc",
			OriginalPath: "doc.txt",
			Ext:          ".txt",
			MIMEType:     "text/plain",
			SizeBytes:    95,
		},
	})
	c.Assert(err, qt.IsNil)

	email := &recordingStorageQuotaEmailService{}
	svc := services.NewStorageQuotaReminderService(factorySet, email, nil, nil).
		WithQuotaBytesFor(func(string) int64 { return 100 })

	stats, err := svc.RemindOnce(context.Background(), time.Now().UTC())
	c.Assert(err, qt.IsNil)
	c.Assert(stats.Failed, qt.Equals, 0)
	c.Assert(stats.Sent(), qt.Equals, 0, qt.Commentf("no recipients => no email enqueued"))
	c.Assert(email.snapshot(), qt.HasLen, 0)
	// But the row must still be committed so we stop re-evaluating
	// this (group, threshold) on every tick.
	has, err := factorySet.StorageQuotaReminderRegistry.HasSent(context.Background(), group.ID, int(models.StorageQuota90Percent))
	c.Assert(err, qt.IsNil)
	c.Assert(has, qt.IsTrue)
}
