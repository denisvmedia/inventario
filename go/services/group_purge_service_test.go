package services_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	_ "github.com/denisvmedia/inventario/internal/fileblob" // register file:// driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// newFileUploadLocation returns a file:// DSN for a temp directory, cross-
// platform. Mirrors the helper pattern in file_service_test.go.
func newFileUploadLocation(c *qt.C) string {
	tempDir := c.TempDir()
	if runtime.GOOS == "windows" {
		return "file:///" + tempDir + "?create_dir=1"
	}
	return "file://" + tempDir + "?create_dir=1"
}

// seedPurgeFixtures inserts one pending-deletion group with a file (and
// physical blob), one used invite, one unused-expired invite, and one active
// group with its own file that must survive the sweep. Returns the two group
// IDs so tests can assert on them.
func seedPurgeFixtures(c *qt.C, ctx context.Context, fs *registry.FactorySet, uploadLocation string) (pendingID, activeID, blobPath string) {
	c.Helper()

	// Pending-deletion group.
	pending, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		Slug:               "pending-group-slug-0000000000",
		Name:               "Pending Group",
		Status:             models.LocationGroupStatusPendingDeletion,
		CreatedBy:          "user-admin",
		MainCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)

	// Active group — must be left untouched by the sweep.
	active, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		Slug:               "active-group-slug-00000000000",
		Name:               "Active Group",
		Status:             models.LocationGroupStatusActive,
		CreatedBy:          "user-admin",
		MainCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)

	// Seed a physical blob referenced by a FileEntity in the pending group.
	blobPath = "pending/file.txt"
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	c.Assert(b.WriteAll(ctx, blobPath, []byte("payload"), nil), qt.IsNil)
	c.Assert(b.Close(), qt.IsNil)

	fileReg := fs.FileRegistryFactory.CreateServiceRegistry()
	_, err = fileReg.Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        "tenant-a",
			GroupID:         pending.ID,
			CreatedByUserID: "user-admin",
		},
		Title: "pending-file",
		Type:  models.FileTypeDocument,
		File: &models.File{
			Path:         "pending/file",
			OriginalPath: blobPath,
			Ext:          ".txt",
			MIMEType:     "text/plain",
		},
	})
	c.Assert(err, qt.IsNil)

	// Survivor file in the active group.
	_, err = fileReg.Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        "tenant-a",
			GroupID:         active.ID,
			CreatedByUserID: "user-admin",
		},
		Title: "active-file",
		Type:  models.FileTypeDocument,
		File: &models.File{
			Path:         "active/file",
			OriginalPath: "active/file.txt",
			Ext:          ".txt",
			MIMEType:     "text/plain",
		},
	})
	c.Assert(err, qt.IsNil)

	// Used invite for the pending group — must be snapshotted to audit.
	usedAt := time.Now().Add(-2 * time.Hour)
	usedBy := "user-member"
	_, err = fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		GroupID:            pending.ID,
		Token:              "used-token",
		CreatedBy:          "user-admin",
		ExpiresAt:          time.Now().Add(-1 * time.Hour),
		UsedBy:             &usedBy,
		UsedAt:             &usedAt,
	})
	c.Assert(err, qt.IsNil)

	// Unused expired invite for the active group — removed by
	// CleanExpiredInvites, NOT by the per-group purge path.
	_, err = fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		GroupID:            active.ID,
		Token:              "expired-unused-token",
		CreatedBy:          "user-admin",
		ExpiresAt:          time.Now().Add(-30 * time.Minute),
	})
	c.Assert(err, qt.IsNil)

	// Unused active invite for the active group — must survive.
	_, err = fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		GroupID:            active.ID,
		Token:              "active-unused-token",
		CreatedBy:          "user-admin",
		ExpiresAt:          time.Now().Add(24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)

	return pending.ID, active.ID, blobPath
}

func newPurgeFixture(c *qt.C) (
	ctx context.Context,
	fs *registry.FactorySet,
	svc *services.GroupPurgeService,
	uploadLocation, pendingID, activeID, blobPath string,
) {
	ctx = context.Background()
	uploadLocation = newFileUploadLocation(c)
	fs = memory.NewFactorySet()
	fileSvc := services.NewFileService(fs, uploadLocation)
	svc = services.NewGroupPurgeService(fs, fileSvc)
	pendingID, activeID, blobPath = seedPurgeFixtures(c, ctx, fs, uploadLocation)
	return ctx, fs, svc, uploadLocation, pendingID, activeID, blobPath
}

// TestGroupPurgeService_PurgeOnce_HappyPath verifies the full purge sweep:
// pending group and its dependents are hard-deleted, the used invite is
// snapshotted into the audit table, the physical blob is removed, and the
// active group is untouched.
func TestGroupPurgeService_PurgeOnce_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, uploadLocation, pendingID, activeID, blobPath := newPurgeFixture(c)

	purged, failed, err := svc.PurgeOnce(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 1)
	c.Assert(failed, qt.Equals, 0)

	// Pending group row is gone.
	_, err = fs.LocationGroupRegistry.Get(ctx, pendingID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Active group survives.
	survivor, err := fs.LocationGroupRegistry.Get(ctx, activeID)
	c.Assert(err, qt.IsNil)
	c.Assert(survivor.Status, qt.Equals, models.LocationGroupStatusActive)

	// Pending group's file is gone (service-mode view).
	fileReg := fs.FileRegistryFactory.CreateServiceRegistry()
	files, err := fileReg.List(ctx)
	c.Assert(err, qt.IsNil)
	for _, f := range files {
		c.Assert(f.GroupID, qt.Not(qt.Equals), pendingID)
	}

	// Physical blob removed.
	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	exists, err := b.Exists(ctx, blobPath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)

	// Used invite was archived.
	audits, err := fs.GroupInviteAuditRegistry.ListByOriginalGroup(ctx, pendingID)
	c.Assert(err, qt.IsNil)
	c.Assert(audits, qt.HasLen, 1)
	c.Assert(audits[0].TenantID, qt.Equals, "tenant-a")
	c.Assert(audits[0].OriginalGroupID, qt.Equals, pendingID)
	c.Assert(audits[0].OriginalGroupSlug, qt.Equals, "pending-group-slug-0000000000")
	c.Assert(audits[0].Token, qt.Equals, "used-token")
	c.Assert(audits[0].UsedBy, qt.Equals, "user-member")

	// All invites belonging to the pending group are gone.
	allInvites, err := fs.GroupInviteRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	for _, inv := range allInvites {
		c.Assert(inv.GroupID, qt.Not(qt.Equals), pendingID)
	}
}

// TestGroupPurgeService_PurgeOnce_SkipsActive verifies that a factory set
// with only active groups yields a no-op sweep.
func TestGroupPurgeService_PurgeOnce_SkipsActive(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	fileSvc := services.NewFileService(fs, newFileUploadLocation(c))
	svc := services.NewGroupPurgeService(fs, fileSvc)

	created, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		Slug:               "active-only-slug-000000000000",
		Name:               "Active Only",
		Status:             models.LocationGroupStatusActive,
		CreatedBy:          "user-admin",
		MainCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)

	purged, failed, err := svc.PurgeOnce(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 0)

	// Group row is still there.
	_, err = fs.LocationGroupRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
}

// TestGroupPurgeService_CleanExpiredInvites verifies the expiry sweep
// (spec #1309 Option 2i): unused expired invites are removed, used invites
// and unused non-expired invites are preserved.
func TestGroupPurgeService_CleanExpiredInvites(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs := memory.NewFactorySet()
	fileSvc := services.NewFileService(fs, newFileUploadLocation(c))
	svc := services.NewGroupPurgeService(fs, fileSvc)

	// Parent group (active).
	group, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		Slug:               "expiry-group-slug-0000000000",
		Name:               "Expiry Group",
		Status:             models.LocationGroupStatusActive,
		CreatedBy:          "user-admin",
		MainCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)

	usedAt := time.Now().Add(-90 * time.Minute)
	usedBy := "user-member"
	// Used expired invite — NOT touched by CleanExpiredInvites.
	usedExpired, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		GroupID:            group.ID,
		Token:              "used-expired",
		CreatedBy:          "user-admin",
		ExpiresAt:          time.Now().Add(-1 * time.Hour),
		UsedBy:             &usedBy,
		UsedAt:             &usedAt,
	})
	c.Assert(err, qt.IsNil)

	// Unused expired invite — must be removed.
	unusedExpired, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		GroupID:            group.ID,
		Token:              "unused-expired",
		CreatedBy:          "user-admin",
		ExpiresAt:          time.Now().Add(-5 * time.Minute),
	})
	c.Assert(err, qt.IsNil)

	// Unused active invite — must survive.
	unusedActive, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: "tenant-a"},
		GroupID:            group.ID,
		Token:              "unused-active",
		CreatedBy:          "user-admin",
		ExpiresAt:          time.Now().Add(24 * time.Hour),
	})
	c.Assert(err, qt.IsNil)

	deleted, err := svc.CleanExpiredInvites(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(deleted, qt.Equals, 1)

	// Used expired invite stays.
	_, err = fs.GroupInviteRegistry.Get(ctx, usedExpired.ID)
	c.Assert(err, qt.IsNil)

	// Unused expired invite is gone.
	_, err = fs.GroupInviteRegistry.Get(ctx, unusedExpired.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Unused active invite stays.
	_, err = fs.GroupInviteRegistry.Get(ctx, unusedActive.ID)
	c.Assert(err, qt.IsNil)
}

// TestGroupPurgeService_PurgeOnce_Reentrancy verifies that a second sweep
// over an already-purged factory set is a no-op: the previously-purged group
// row is gone, so nothing is reprocessed and no duplicate audit rows are
// written.
func TestGroupPurgeService_PurgeOnce_Reentrancy(t *testing.T) {
	c := qt.New(t)
	ctx, fs, svc, _, pendingID, _, _ := newPurgeFixture(c)

	// First sweep does the real work.
	purged, failed, err := svc.PurgeOnce(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 1)
	c.Assert(failed, qt.Equals, 0)

	// Second sweep has nothing to do — the pending row is gone.
	purged, failed, err = svc.PurgeOnce(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 0)

	// Exactly one audit row for the purged group, not two.
	audits, err := fs.GroupInviteAuditRegistry.ListByOriginalGroup(ctx, pendingID)
	c.Assert(err, qt.IsNil)
	c.Assert(audits, qt.HasLen, 1)
}

// TestGroupPurgeService_PurgeOnce_PartialFailure_NoDuplicateAudit drives the
// re-entrancy contract on a hard case: the first sweep writes the invite
// audit snapshot and then crashes during blob deletion, leaving the group
// pending_deletion. A second sweep must snapshot the same used invite again
// and succeed, but the unique (tenant_id, original_invite_id) index on
// group_invites_audit must collapse the retry into a no-op so we end up with
// exactly one audit row.
func TestGroupPurgeService_PurgeOnce_PartialFailure_NoDuplicateAudit(t *testing.T) {
	c := qt.New(t)
	ctx, fs, _, uploadLocation, pendingID, _, blobPath := newPurgeFixture(c)

	// First attempt: bad upload location so blob.OpenBucket fails inside
	// DeletePhysicalFilesForGroup, which is invoked AFTER snapshotUsedInvites.
	brokenFileSvc := services.NewFileService(fs, "unknownscheme://invalid")
	brokenSvc := services.NewGroupPurgeService(fs, brokenFileSvc)
	purged, failed, err := brokenSvc.PurgeOnce(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 1)

	// Audit row was written by the failed first attempt.
	audits, err := fs.GroupInviteAuditRegistry.ListByOriginalGroup(ctx, pendingID)
	c.Assert(err, qt.IsNil)
	c.Assert(audits, qt.HasLen, 1)
	firstAuditID := audits[0].ID

	// Group is still pending_deletion — nothing downstream of the failed
	// blob delete ran.
	pending, err := fs.LocationGroupRegistry.Get(ctx, pendingID)
	c.Assert(err, qt.IsNil)
	c.Assert(pending.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)

	// Second attempt: healthy file service finishes the purge.
	workingSvc := services.NewGroupPurgeService(fs, services.NewFileService(fs, uploadLocation))
	purged, failed, err = workingSvc.PurgeOnce(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 1)
	c.Assert(failed, qt.Equals, 0)

	// Exactly one audit row survives — the retry's snapshotUsedInvites
	// re-ran but the idempotent Create dropped the duplicate. The original
	// audit ID from attempt #1 is still the one present.
	audits, err = fs.GroupInviteAuditRegistry.ListByOriginalGroup(ctx, pendingID)
	c.Assert(err, qt.IsNil)
	c.Assert(audits, qt.HasLen, 1)
	c.Assert(audits[0].ID, qt.Equals, firstAuditID)

	// Group and blob are gone.
	_, err = fs.LocationGroupRegistry.Get(ctx, pendingID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()
	exists, err := b.Exists(ctx, blobPath)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
}
