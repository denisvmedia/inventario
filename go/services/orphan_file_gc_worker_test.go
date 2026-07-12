package services_test

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"runtime"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	_ "github.com/denisvmedia/inventario/internal/fileblob" // register the file:// driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

// The orphan-file GC (#2237) DELETES USER DATA. The bar is not "does it delete
// orphans" — it is "can it ever delete something that is NOT an orphan". The
// NEGATIVE tests below (everything that must SURVIVE a sweep) are therefore the
// point of the feature and come first; the positive tests merely confirm the
// worker is not inert.

// ---------------------------------------------------------------------------
// Fixture
// ---------------------------------------------------------------------------

const (
	gcTenantA = "tenant-a"
	gcTenantB = "tenant-b"
)

// gcFixture is a memory-backed installation with one active tenant, one active
// group and one user, plus a file:// bucket on a temp dir.
type gcFixture struct {
	c              *qt.C
	fs             *registry.FactorySet
	ctx            context.Context
	tempDir        string
	uploadLocation string

	tenantID string
	groupID  string
	userID   string

	// probeCalls counts existence probes per link type so a test can assert
	// that, e.g., an export-linked file never triggers a probe at all.
	probeCalls map[string]int

	// probeErr, when set, is returned by every probe INSTEAD of the real
	// answer — used to prove a transient failure never reads as "gone".
	probeErr error
}

func newGCFixture(c *qt.C) *gcFixture {
	c.Helper()

	tempDir := c.TempDir()
	uploadLocation := "file://" + tempDir + "?create_dir=1"
	if runtime.GOOS == "windows" {
		uploadLocation = "file:///" + tempDir + "?create_dir=1"
	}

	factorySet := memory.NewFactorySet()
	ctx := context.Background()

	f := &gcFixture{
		c:              c,
		fs:             factorySet,
		ctx:            ctx,
		tempDir:        tempDir,
		uploadLocation: uploadLocation,
		probeCalls:     make(map[string]int),
	}

	f.tenantID = f.newTenant(gcTenantA, models.TenantStatusActive)
	f.groupID = f.newGroup(f.tenantID, "group-a", models.LocationGroupStatusActive)
	f.userID = f.newUser(f.tenantID, "owner@example.com")

	return f
}

func (f *gcFixture) newTenant(slug string, status models.TenantStatus) string {
	f.c.Helper()
	t, err := f.fs.TenantRegistry.Create(f.ctx, models.Tenant{
		Name:   "Tenant " + slug,
		Slug:   slug,
		Status: status,
	})
	f.c.Assert(err, qt.IsNil)
	return t.ID
}

func (f *gcFixture) newGroup(tenantID, slug string, status models.LocationGroupStatus) string {
	f.c.Helper()
	g, err := f.fs.LocationGroupRegistry.Create(f.ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Slug:                slug + "-000000000000000000",
		Name:                "Group " + slug,
		Status:              status,
		CreatedBy:           "seed",
		GroupCurrency:       "USD",
	})
	f.c.Assert(err, qt.IsNil)
	return g.ID
}

func (f *gcFixture) newUser(tenantID, email string) string {
	f.c.Helper()
	u, err := f.fs.UserRegistry.Create(f.ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               email,
		Name:                "Owner",
		IsActive:            true,
	})
	f.c.Assert(err, qt.IsNil)
	return u.ID
}

// seedFileOpts describes one file row to plant.
type seedFileOpts struct {
	tenantID   string
	groupID    string
	userID     string
	linkType   string
	linkID     string
	linkMeta   string
	createdAgo time.Duration
	updatedAgo time.Duration
	blobKey    string // when set, a blob is written at this key and OriginalPath points at it
	mimeType   string
}

// seedFile creates a file row through the SERVICE registry so the test controls
// tenant/group/creator/timestamps exactly (the memory service registry does not
// override them the way a user registry would).
func (f *gcFixture) seedFile(o seedFileOpts) *models.FileEntity {
	f.c.Helper()

	if o.tenantID == "" {
		o.tenantID = f.tenantID
	}
	if o.groupID == "" {
		o.groupID = f.groupID
	}
	if o.userID == "" {
		o.userID = f.userID
	}
	if o.createdAgo == 0 {
		o.createdAgo = 30 * 24 * time.Hour
	}
	if o.updatedAgo == 0 {
		o.updatedAgo = o.createdAgo
	}
	if o.mimeType == "" {
		o.mimeType = "image/jpeg"
	}

	now := time.Now()
	file := models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        o.tenantID,
			GroupID:         o.groupID,
			CreatedByUserID: o.userID,
		},
		Title:            "seeded",
		Type:             models.FileTypeImage,
		LinkedEntityType: o.linkType,
		LinkedEntityID:   o.linkID,
		LinkedEntityMeta: o.linkMeta,
		CreatedAt:        now.Add(-o.createdAgo),
		UpdatedAt:        now.Add(-o.updatedAgo),
		File: &models.File{
			Path:         "seeded",
			OriginalPath: o.blobKey,
			Ext:          ".jpg",
			MIMEType:     o.mimeType,
		},
	}

	created, err := f.fs.FileRegistryFactory.CreateServiceRegistry().Create(f.ctx, file)
	f.c.Assert(err, qt.IsNil)

	if o.blobKey != "" {
		f.writeBlob(o.blobKey, []byte("payload"))
	}
	return created
}

// userCtx builds the (user, group) request context the entity registries need.
// Note the WORKER never gets this context — it is handed a bare background
// context and has to resolve the owner itself from the candidate row.
func (f *gcFixture) userCtx(userID, groupID string) context.Context {
	f.c.Helper()
	u, err := f.fs.UserRegistry.Get(f.ctx, userID)
	f.c.Assert(err, qt.IsNil)
	g, err := f.fs.LocationGroupRegistry.Get(f.ctx, groupID)
	f.c.Assert(err, qt.IsNil)
	return appctx.WithGroup(appctx.WithUser(f.ctx, u), g)
}

// seedLiveCommodity creates a real location → area → commodity chain in the
// given group and returns the three ids.
func (f *gcFixture) seedLiveCommodity(groupID, userID string) (locationID, areaID, commodityID string) {
	f.c.Helper()
	ctx := f.userCtx(userID, groupID)

	loc, err := f.fs.LocationRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Location{
		Name: "Live Location",
	})
	f.c.Assert(err, qt.IsNil)

	area, err := f.fs.AreaRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Area{
		Name:       "Live Area",
		LocationID: loc.ID,
	})
	f.c.Assert(err, qt.IsNil)

	com, err := f.fs.CommodityRegistryFactory.MustCreateUserRegistry(ctx).Create(ctx, models.Commodity{
		Name:   "Live Commodity",
		AreaID: new(area.ID),
	})
	f.c.Assert(err, qt.IsNil)

	return loc.ID, area.ID, com.ID
}

func (f *gcFixture) writeBlob(key string, data []byte) {
	f.c.Helper()
	b, err := blob.OpenBucket(f.ctx, f.uploadLocation)
	f.c.Assert(err, qt.IsNil)
	defer b.Close()
	f.c.Assert(b.WriteAll(f.ctx, key, data, nil), qt.IsNil)
}

func (f *gcFixture) blobExists(key string) bool {
	f.c.Helper()
	b, err := blob.OpenBucket(f.ctx, f.uploadLocation)
	f.c.Assert(err, qt.IsNil)
	defer b.Close()
	exists, err := b.Exists(f.ctx, key)
	f.c.Assert(err, qt.IsNil)
	return exists
}

// backdateBlobs rewinds the mtime of every file currently in the bucket's
// backing directory. The blob age gate reads the bucket's ModTime (which no
// application code can set), so this is the only way to simulate an aged blob.
//
// Walks through an os.Root so the traversal is confined to the temp dir and
// cannot follow a symlink out of it.
func (f *gcFixture) backdateBlobs(age time.Duration) {
	f.c.Helper()
	when := time.Now().Add(-age)

	root, err := os.OpenRoot(f.tempDir)
	f.c.Assert(err, qt.IsNil)
	defer root.Close()

	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		return root.Chtimes(path, when, when)
	})
	f.c.Assert(err, qt.IsNil)
}

func (f *gcFixture) fileExists(id string) bool {
	f.c.Helper()
	_, err := f.fs.FileRegistryFactory.CreateServiceRegistry().Get(f.ctx, id)
	return err == nil
}

// countingProbe wraps a real existence probe with a call counter and an
// optional forced error.
func (f *gcFixture) countingProbe(linkType string, inner services.EntityExistenceProbe) services.EntityExistenceProbe {
	return func(ctx context.Context, id string) error {
		f.probeCalls[linkType]++
		if f.probeErr != nil {
			return f.probeErr
		}
		return inner(ctx, id)
	}
}

func (f *gcFixture) deps() services.OrphanFileGCDeps {
	comReg := f.fs.CommodityRegistryFactory.CreateServiceRegistry()
	areaReg := f.fs.AreaRegistryFactory.CreateServiceRegistry()
	locReg := f.fs.LocationRegistryFactory.CreateServiceRegistry()

	return services.OrphanFileGCDeps{
		Files: f.fs.FileRegistryFactory.CreateServiceRegistry(),
		Probes: services.OrphanFileGCProbes{
			Commodity: f.countingProbe("commodity", func(ctx context.Context, id string) error {
				_, err := comReg.Get(ctx, id)
				return err
			}),
			Area: f.countingProbe("area", func(ctx context.Context, id string) error {
				_, err := areaReg.Get(ctx, id)
				return err
			}),
			Location: f.countingProbe("location", func(ctx context.Context, id string) error {
				_, err := locReg.Get(ctx, id)
				return err
			}),
		},
		Exports:        f.fs.ExportRegistryFactory.CreateServiceRegistry(),
		Restores:       f.fs.RestoreOperationRegistryFactory.CreateServiceRegistry(),
		Tenants:        f.fs.TenantRegistry,
		Groups:         f.fs.LocationGroupRegistry,
		Users:          f.fs.UserRegistry,
		Deleter:        services.NewFileService(f.fs, f.uploadLocation),
		UploadLocation: f.uploadLocation,
	}
}

// sweep runs one tick in DELETE mode (the dangerous mode — every negative test
// must survive it).
func (f *gcFixture) sweep(opts ...services.OrphanFileGCOption) {
	f.c.Helper()
	opts = append([]services.OrphanFileGCOption{
		services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
	}, opts...)
	services.NewOrphanFileGCWorker(f.deps(), opts...).RunOnce(f.ctx)
}

// stubPause is a PauseChecker that reports a fixed pause state.
type stubPause struct {
	paused map[models.WorkerType]bool
	calls  int
}

func (s *stubPause) IsPaused(wt models.WorkerType) bool {
	s.calls++
	return s.paused[wt]
}

// countingFileRegistry records every read so a test can prove the paused worker
// touched nothing at all.
type countingFileRegistry struct {
	registry.FileRegistry
	calls int
}

func (r *countingFileRegistry) ListOrphanCandidates(ctx context.Context, olderThan time.Time, after registry.OrphanCandidateCursor, limit int) ([]*models.FileEntity, error) {
	r.calls++
	return r.FileRegistry.ListOrphanCandidates(ctx, olderThan, after, limit)
}

func (r *countingFileRegistry) ExistingIDs(ctx context.Context, ids []string) ([]string, error) {
	r.calls++
	return r.FileRegistry.ExistingIDs(ctx, ids)
}

// failingGetFileRegistry makes the thumbnail sweep's row probe fail
// TRANSIENTLY — a DB timeout, a connection reset. It must never read as "the
// row is gone".
type failingGetFileRegistry struct {
	registry.FileRegistry
	err error
}

func (r *failingGetFileRegistry) Get(ctx context.Context, id string) (*models.FileEntity, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.FileRegistry.Get(ctx, id)
}

// failingDeleter stands in for FileService when the delete itself raises (on
// postgres: an FK the teardown could not break). Used to prove the forensic log
// never claims a destruction that did not happen.
type failingDeleter struct{ err error }

func (d *failingDeleter) DeleteFileWithPhysical(context.Context, string) error { return d.err }

// captureLogs swaps in a JSON slog handler for the duration of fn and returns
// everything the worker logged. The forensic record is the ONLY artifact from
// which a destroyed file can be reconstructed, so its content is a contract.
func captureLogs(c *qt.C, fn func()) string {
	c.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(prev)
	fn()
	return buf.String()
}

// seedThumbnailJob plants a thumbnail-generation job for fileID OWNED BY
// userID. The owner matters: RequestThumbnailGeneration deliberately stamps the
// job with the REQUESTING user (so one member cannot spend another's rate
// limit), and merely viewing a not-yet-generated thumbnail enqueues one — so in
// any shared group the job's owner is routinely NOT the file's creator.
func (f *gcFixture) seedThumbnailJob(fileID, userID, tenantID string) {
	f.c.Helper()
	_, err := f.fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry().Create(f.ctx, models.ThumbnailGenerationJob{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: tenantID, UserID: userID},
		FileID:                  fileID,
		Status:                  models.ThumbnailStatusPending,
		MaxAttempts:             3,
	})
	f.c.Assert(err, qt.IsNil)
}

// thumbnailJobCount counts the jobs still referencing fileID, RLS-free.
func (f *gcFixture) thumbnailJobCount(fileID string) int {
	f.c.Helper()
	jobs, err := f.fs.ThumbnailGenerationJobRegistryFactory.CreateServiceRegistry().ListByFileID(f.ctx, fileID)
	f.c.Assert(err, qt.IsNil)
	return len(jobs)
}

// ---------------------------------------------------------------------------
// NEGATIVE — things that MUST survive a delete-mode sweep
// ---------------------------------------------------------------------------

// A STANDALONE file (linked_entity_type = "") is LEGITIMATE, not an orphan.
// Since #2235 standalone files are first-class and are exported in backups.
// "No link" must NEVER mean "orphan": an orphan is a file whose link points at
// a NONEXISTENT entity. A predicate shaped `linked_entity_id IS NULL OR NOT
// EXISTS(...)` would silently eat every standalone file in the installation —
// this test is the guard against exactly that.
func TestOrphanFileGC_StandaloneFileIsNeverSwept(t *testing.T) {
	tests := []struct {
		name     string
		linkType string
		linkID   string
	}{
		{"standalone: no link at all", "", ""},
		{"standalone: empty type with a stray id (malformed, still not an orphan)", "", "some-dangling-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			file := f.seedFile(seedFileOpts{linkType: tt.linkType, linkID: tt.linkID})

			f.sweep()

			c.Assert(f.fileExists(file.ID), qt.IsTrue, qt.Commentf("#2235 standalone file was destroyed by the GC"))
		})
	}
}

// linked_entity_type='export' belongs to the backup subsystem's own lifecycle.
// The GC must not sweep it AND must never probe the exports table for
// existence: ExportRegistry.Get filters `deleted_at IS NULL`, so a Get-based
// probe would read a soft-deleted (recoverable) export as nonexistent and
// destroy the user's backup. Because 'export' is not in the allowlist, that
// trap is structurally unreachable — asserted here by the zero probe count.
func TestOrphanFileGC_ExportLinkedFileIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	file := f.seedFile(seedFileOpts{
		linkType: "export",
		linkID:   "an-export-id-that-does-not-exist",
		linkMeta: "inb-2.0",
	})

	f.sweep()

	c.Assert(f.fileExists(file.ID), qt.IsTrue, qt.Commentf("export-linked file was destroyed by the GC"))
	c.Assert(f.probeCalls["commodity"]+f.probeCalls["area"]+f.probeCalls["location"], qt.Equals, 0,
		qt.Commentf("an export-linked file must not trigger ANY existence probe"))
}

// The registries do NOT enforce models.FileEntity.ValidateWithContext, so the
// DB is a superset of the validator's enumeration and a link type this release
// has never heard of can appear. It must be KEPT — the allowlist fails closed.
func TestOrphanFileGC_UnknownLinkTypeIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	file := f.seedFile(seedFileOpts{linkType: "widget", linkID: "no-such-widget"})

	f.sweep()

	c.Assert(f.fileExists(file.ID), qt.IsTrue, qt.Commentf("unknown link type must fail closed, not be swept"))
}

// THE catastrophic-bug guard. PUT /files/{id} copies linked_entity_type /
// linked_entity_id straight from client input with NO existence, tenant, or
// group check, so a file in group A legitimately linked to a commodity in
// group B is reachable in production. If the existence probe were ever swapped
// for a group-scoped (user) registry it would 404 on that LIVE commodity and
// delete a LIVE file. This test fails loudly if anyone does that.
func TestOrphanFileGC_CrossGroupLinkIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	otherGroupID := f.newGroup(f.tenantID, "group-b", models.LocationGroupStatusActive)
	_, _, commodityID := f.seedLiveCommodity(otherGroupID, f.userID)

	// The file lives in group A; its link points at a LIVE commodity in group B.
	file := f.seedFile(seedFileOpts{
		groupID:  f.groupID,
		linkType: "commodity",
		linkID:   commodityID,
		linkMeta: "images",
	})

	f.sweep()

	c.Assert(f.fileExists(file.ID), qt.IsTrue,
		qt.Commentf("a file linked ACROSS GROUPS to a live commodity was destroyed — the probe is not service-mode/by-id"))
}

// Same shape across tenants: the service-mode probe matches by ID only, finds
// the live entity, and the file is kept.
func TestOrphanFileGC_CrossTenantLinkIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	otherTenantID := f.newTenant(gcTenantB, models.TenantStatusActive)
	otherGroupID := f.newGroup(otherTenantID, "group-t2", models.LocationGroupStatusActive)
	otherUserID := f.newUser(otherTenantID, "other@example.com")
	_, _, commodityID := f.seedLiveCommodity(otherGroupID, otherUserID)

	file := f.seedFile(seedFileOpts{linkType: "commodity", linkID: commodityID, linkMeta: "images"})

	f.sweep()

	c.Assert(f.fileExists(file.ID), qt.IsTrue,
		qt.Commentf("a file linked ACROSS TENANTS to a live commodity was destroyed"))
}

// The happy path: a file whose linked entity is alive is never a candidate, for
// all three allowlisted link types.
func TestOrphanFileGC_LiveEntityFileIsNeverSwept(t *testing.T) {
	tests := []string{"commodity", "area", "location"}

	for _, linkType := range tests {
		t.Run(linkType, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			locationID, areaID, commodityID := f.seedLiveCommodity(f.groupID, f.userID)
			targets := map[string]string{
				"commodity": commodityID,
				"area":      areaID,
				"location":  locationID,
			}

			file := f.seedFile(seedFileOpts{
				linkType: linkType,
				linkID:   targets[linkType],
				linkMeta: "images",
			})

			f.sweep()

			c.Assert(f.fileExists(file.ID), qt.IsTrue,
				qt.Commentf("a file linked to a LIVE %s was destroyed", linkType))
		})
	}
}

// The age gate needs BOTH timestamps. updated_at is the load-bearing one: the
// concurrent-attach case is a PUT /files/{id} that lands while the entity
// delete is in flight, and that PUT stamps UpdatedAt = time.Now(). A gate that
// only looked at created_at would sweep the freshly-attached file.
func TestOrphanFileGC_TooYoungFileIsNeverSwept(t *testing.T) {
	tests := []struct {
		name       string
		createdAgo time.Duration
		updatedAgo time.Duration
	}{
		{"concurrent attach: old row, fresh updated_at", 30 * 24 * time.Hour, time.Hour},
		{"fresh row, backdated updated_at", time.Hour, 30 * 24 * time.Hour},
		{"both fresh", time.Hour, time.Hour},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			file := f.seedFile(seedFileOpts{
				linkType:   "commodity",
				linkID:     "gone-commodity",
				linkMeta:   "images",
				createdAgo: tt.createdAgo,
				updatedAgo: tt.updatedAgo,
			})

			f.sweep()

			c.Assert(f.fileExists(file.ID), qt.IsTrue, qt.Commentf("a file inside the age window was destroyed"))
		})
	}
}

// A restore is a machine that deletes every entity and then recreates the files
// over an UNBOUNDED duration — mid-restore every surviving file row legitimately
// points at a just-deleted entity. The age gate cannot cover that window (a
// restore writes the ARCHIVE's timestamps verbatim, so a row written seconds ago
// can look years old); the DB-backed, per-tenant concurrency gate is what does.
//
// The gate must be PER-TENANT: an unrelated tenant keeps sweeping normally, so
// one stuck operation cannot disable the GC for the whole installation.
func TestOrphanFileGC_InFlightOperationBlocksItsTenantOnly(t *testing.T) {
	tests := []struct {
		name  string
		setup func(f *gcFixture, tenantID, groupID string)
	}{
		{
			name: "restore pending",
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedRestore(tenantID, groupID, models.RestoreStatusPending, nil)
			},
		},
		{
			name: "restore running",
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedRestore(tenantID, groupID, models.RestoreStatusRunning, nil)
			},
		},
		{
			name: "export pending",
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedExport(tenantID, groupID, models.ExportStatusPending, false, nil)
			},
		},
		{
			name: "export in_progress",
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedExport(tenantID, groupID, models.ExportStatusInProgress, false, nil)
			},
		},
		{
			// A pending IMPORT is an export row with Imported=true, Status=pending
			// (backup/import/worker.go claims on exactly that), so the same gate
			// covers import ingestion.
			name: "import pending",
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedExport(tenantID, groupID, models.ExportStatusPending, true, nil)
			},
		},
		{
			// COOLDOWN: a restore that finished 1h ago still blocks, because the
			// rows it wrote carry the ARCHIVE's timestamps and would clear the row
			// age gate instantly.
			name: "restore completed within min-age (cooldown)",
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedRestore(tenantID, groupID, models.RestoreStatusCompleted, models.NewPTimestamp(time.Now().Add(-time.Hour)))
			},
		},
		{
			name: "export failed within min-age (cooldown)",
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedExport(tenantID, groupID, models.ExportStatusFailed, false, models.NewPTimestamp(time.Now().Add(-time.Hour)))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			// Tenant A: the blocked one.
			blockedFile := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone-1", linkMeta: "images"})
			tt.setup(f, f.tenantID, f.groupID)

			// Tenant B: no in-flight operation — must sweep normally.
			otherTenantID := f.newTenant(gcTenantB, models.TenantStatusActive)
			otherGroupID := f.newGroup(otherTenantID, "group-t2", models.LocationGroupStatusActive)
			otherUserID := f.newUser(otherTenantID, "other@example.com")
			sweepableFile := f.seedFile(seedFileOpts{
				tenantID: otherTenantID,
				groupID:  otherGroupID,
				userID:   otherUserID,
				linkType: "commodity",
				linkID:   "gone-2",
				linkMeta: "images",
			})

			f.sweep()

			c.Assert(f.fileExists(blockedFile.ID), qt.IsTrue,
				qt.Commentf("a file in a tenant with an in-flight/just-finished operation was destroyed"))
			c.Assert(f.fileExists(sweepableFile.ID), qt.IsFalse,
				qt.Commentf("the gate blocked an UNRELATED tenant — it must be per-tenant, not global"))
		})
	}
}

// A DB timeout, a connection reset, or any driver error from the existence
// probe must NEVER be read as "the entity does not exist". It aborts the tick.
func TestOrphanFileGC_ProbeErrorAbortsTheTick(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	file := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})
	f.probeErr = context.DeadlineExceeded // NOT registry.ErrNotFound

	f.sweep()

	c.Assert(f.fileExists(file.ID), qt.IsTrue,
		qt.Commentf("a transient probe failure was treated as 'the entity is gone'"))
}

// The row scan must keep its place across an ABORTED tick. A probe failure that
// recurs on the same row (a poison row, a permanently unhealthy replica) would
// otherwise replay the identical oldest-first page every tick, forever, and no
// orphan behind it would ever be enumerated — the worker would look alive while
// collecting nothing.
func TestOrphanFileGC_AbortedTickDoesNotReplayTheSamePage(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	// Two orphans, oldest first. The probe blows up, so tick 1 aborts on the
	// first one.
	first := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone-1", linkMeta: "images"})
	second := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone-2", linkMeta: "images"})

	// EXACTLY one row per tick. Without that the assertion is vacuous: a tick
	// that restarts from the top would simply page through both orphans and the
	// regression would hide.
	deps := f.deps()
	worker := services.NewOrphanFileGCWorker(deps,
		services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
		services.WithOrphanFileGCRowBudget(1, 1),
	)

	f.probeErr = context.DeadlineExceeded
	worker.RunOnce(f.ctx) // aborts on the first candidate

	// The failure clears. The next tick must resume AFTER the row it choked on,
	// not re-serve it — so its one unit of budget reaches the SECOND orphan.
	f.probeErr = nil
	worker.RunOnce(f.ctx)

	c.Assert(f.fileExists(second.ID), qt.IsFalse,
		qt.Commentf("the scan replayed the failed page instead of making progress"))
	c.Assert(f.fileExists(first.ID), qt.IsTrue,
		qt.Commentf("the row the probe choked on must be KEPT, not deleted, and revisited when the scan wraps"))
}

// The forensic log is the ONLY artifact from which a destroyed thumbnail can be
// reconstructed, so it must never claim a destruction that did not happen. The
// row path splits "deleting" (pre-image) from "deleted" (post-success); the
// thumbnail path must do the same, including when the tenant gate fires between
// the two.
func TestOrphanFileGC_ThumbnailLogNeverClaimsAnUnattemptedDelete(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	key := blobkeys.BuildThumbnailBlobKey(f.tenantID, "a-file-id-with-no-row", "small")
	f.writeBlob(key, []byte("s"))
	f.backdateBlobs(30 * 24 * time.Hour)

	// report mode: the candidate is found, and NOTHING is deleted.
	logs := captureLogs(c, func() {
		f.sweep(services.WithOrphanFileGCMode(services.OrphanFileGCModeReport))
	})

	c.Assert(f.blobExists(key), qt.IsTrue, qt.Commentf("report mode deleted a blob"))
	c.Assert(logs, qt.Contains, `"action":"candidate"`)
	c.Assert(logs, qt.Not(qt.Contains), `"action":"deleted"`,
		qt.Commentf("the log claimed a thumbnail was deleted while in report mode"))

	// delete mode: "deleting" precedes the attempt, "deleted" follows its success.
	logs = captureLogs(c, func() { f.sweep() })

	c.Assert(f.blobExists(key), qt.IsFalse)
	c.Assert(logs, qt.Contains, `"action":"deleting"`)
	c.Assert(logs, qt.Contains, `"action":"deleted"`)
}

// racingThumbnailRegistry deletes a blob from underneath the sweep, right when
// the sweep probes the owning row — i.e. exactly the window a SECOND `run
// workers` replica occupies: it listed the same orphan thumbnail and got to the
// delete first.
type racingThumbnailRegistry struct {
	services.OrphanFileGCFileRegistry
	c              *qt.C
	ctx            context.Context
	uploadLocation string
	key            string
}

func (r *racingThumbnailRegistry) Get(ctx context.Context, id string) (*models.FileEntity, error) {
	b, err := blob.OpenBucket(r.ctx, r.uploadLocation)
	r.c.Assert(err, qt.IsNil)
	defer b.Close()
	_ = b.Delete(r.ctx, r.key) // the other replica wins the race
	return r.OrphanFileGCFileRegistry.Get(ctx, id)
}

// Two replicas racing the same orphan thumbnail must produce one delete and one
// NO-OP — not one delete and one reported FAILURE. Counting the loser's
// already-gone key as a failure would light up orphanGCFailuresTotal and write
// an error record on every tick of a perfectly healthy two-replica deployment,
// which is exactly the noise that hides a real failure.
func TestOrphanFileGC_LosingAThumbnailRaceIsNotAFailure(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	key := blobkeys.BuildThumbnailBlobKey(f.tenantID, "a-file-id-with-no-row", "small")
	f.writeBlob(key, []byte("s"))
	f.backdateBlobs(30 * 24 * time.Hour)

	deps := f.deps()
	deps.Files = &racingThumbnailRegistry{
		OrphanFileGCFileRegistry: deps.Files,
		c:                        c,
		ctx:                      f.ctx,
		uploadLocation:           f.uploadLocation,
		key:                      key,
	}

	logs := captureLogs(c, func() {
		services.NewOrphanFileGCWorker(deps,
			services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
		).RunOnce(f.ctx)
	})

	c.Assert(f.blobExists(key), qt.IsFalse)
	c.Assert(logs, qt.Not(qt.Contains), `"action":"failed"`,
		qt.Commentf("losing the race to another replica was reported as a delete failure"))
}

// A half-wired DESTRUCTIVE worker must refuse to start, not log a successful
// startup and then panic inside the sweep goroutine. A nil Exports/Restores
// registry is not a crash to debug later — it is the concurrency gate silently
// missing, i.e. the guard that keeps the GC off a tenant mid-restore.
func TestOrphanFileGC_IncompleteWiringRefusesToStart(t *testing.T) {
	tests := []struct {
		name    string
		unwire  func(*services.OrphanFileGCDeps)
		missing string
	}{
		{name: "no files registry", unwire: func(d *services.OrphanFileGCDeps) { d.Files = nil }, missing: "Files"},
		{name: "no deleter", unwire: func(d *services.OrphanFileGCDeps) { d.Deleter = nil }, missing: "Deleter"},
		{name: "no exports registry", unwire: func(d *services.OrphanFileGCDeps) { d.Exports = nil }, missing: "Exports"},
		{name: "no restores registry", unwire: func(d *services.OrphanFileGCDeps) { d.Restores = nil }, missing: "Restores"},
		{name: "no tenants registry", unwire: func(d *services.OrphanFileGCDeps) { d.Tenants = nil }, missing: "Tenants"},
		{name: "no groups registry", unwire: func(d *services.OrphanFileGCDeps) { d.Groups = nil }, missing: "Groups"},
		{name: "no users registry", unwire: func(d *services.OrphanFileGCDeps) { d.Users = nil }, missing: "Users"},
		{name: "no upload location", unwire: func(d *services.OrphanFileGCDeps) { d.UploadLocation = "" }, missing: "UploadLocation"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			// An orphan the worker WOULD collect if it ran.
			file := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})

			deps := f.deps()
			tt.unwire(&deps)

			logs := captureLogs(c, func() {
				worker := services.NewOrphanFileGCWorker(deps,
					services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
					services.WithOrphanFileGCInterval(time.Millisecond),
				)
				worker.Start(f.ctx)
				worker.Stop() // returns immediately: nothing was started
			})

			c.Assert(logs, qt.Contains, "incomplete dependencies")
			c.Assert(logs, qt.Contains, tt.missing)
			c.Assert(logs, qt.Not(qt.Contains), "Orphan file GC worker started")
			c.Assert(f.fileExists(file.ID), qt.IsTrue,
				qt.Commentf("a half-wired worker swept anyway"))
		})
	}
}

// Soft-pause (#1308) is the operator's emergency stop for the only destructive
// worker in the tree. A paused worker must do NOTHING — not even read.
//
// workerpause.Controller fails OPEN for worker types it does not know about, so
// this test is worthless without models/worker_control_test.go's assertion that
// WorkerTypeOrphanFileGC is actually in AllWorkerTypes(). The two go together.
func TestOrphanFileGC_PausedWorkerTouchesNothing(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	file := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})

	counting := &countingFileRegistry{FileRegistry: f.fs.FileRegistryFactory.CreateServiceRegistry()}
	deps := f.deps()
	deps.Files = counting

	pause := &stubPause{paused: map[models.WorkerType]bool{models.WorkerTypeOrphanFileGC: true}}

	services.NewOrphanFileGCWorker(deps,
		services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
		services.WithOrphanFileGCPauseController(pause),
	).RunOnce(f.ctx)

	c.Assert(pause.calls, qt.Equals, 1)
	c.Assert(counting.calls, qt.Equals, 0, qt.Commentf("a paused GC still read the file registry"))
	c.Assert(f.probeCalls["commodity"], qt.Equals, 0, qt.Commentf("a paused GC still probed for entity existence"))
	c.Assert(f.fileExists(file.ID), qt.IsTrue, qt.Commentf("a PAUSED GC deleted a file"))
}

// The shipping default. Report mode evaluates the identical predicate and
// reports the identical candidates — and deletes nothing, in either class.
func TestOrphanFileGC_ReportModeDeletesNothing(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	rowFile := f.seedFile(seedFileOpts{
		linkType: "commodity",
		linkID:   "gone",
		linkMeta: "images",
		blobKey:  blobkeys.BuildFileUploadKey(f.tenantID, "orphan.jpg"),
	})
	thumbKey := blobkeys.BuildThumbnailBlobKey(f.tenantID, "no-such-file-id", "small")
	f.writeBlob(thumbKey, []byte("thumb"))
	f.backdateBlobs(30 * 24 * time.Hour)

	// Explicitly NOT delete mode.
	services.NewOrphanFileGCWorker(f.deps(),
		services.WithOrphanFileGCMode(services.OrphanFileGCModeReport),
	).RunOnce(f.ctx)

	c.Assert(f.fileExists(rowFile.ID), qt.IsTrue, qt.Commentf("report mode deleted a file row"))
	c.Assert(f.blobExists(thumbKey), qt.IsTrue, qt.Commentf("report mode deleted a thumbnail blob"))
}

// Off mode does not even scan.
func TestOrphanFileGC_OffModeDoesNotScan(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	file := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})

	counting := &countingFileRegistry{FileRegistry: f.fs.FileRegistryFactory.CreateServiceRegistry()}
	deps := f.deps()
	deps.Files = counting

	services.NewOrphanFileGCWorker(deps, services.WithOrphanFileGCMode(services.OrphanFileGCModeOff)).RunOnce(f.ctx)

	c.Assert(counting.calls, qt.Equals, 0)
	c.Assert(f.fileExists(file.ID), qt.IsTrue)
}

// The blob sweep enumerates EXACTLY ONE prefix — t/<tenant>/thumbnails/ — and
// every other blob class is NEVER-SWEEP, structurally (never listed, never
// read, never deleted). Each of these keys is aged well past the gate and owned
// by NO file row, i.e. each is exactly what a naive "blobs with no owning file
// row" sweep would destroy:
//
//   - restores/  — #2121. POST /uploads/restores writes the blob and creates NO
//     ROW OF ANY KIND. Rowless until POST /exports/import, rowless forever if
//     the user never submits, rowless forever if the import FAILS. Deleting it
//     destroys a user's uploaded backup. THE most dangerous case.
//   - files/     — upload is blob-first/row-second, so every in-flight upload is
//     legitimately a rowless blob; a restore and the blobbackfill CLI are
//     blob-first too.
//   - exports/   — a FAILED export leaves its .inb referenced by no file row AND
//     no export row.
//   - seed-*     — sits directly under the tenant root.
//   - legacy flat keys — still live wherever `inventario backfill blobs` never ran.
func TestOrphanFileGC_NeverSweepsAnythingButThumbnails(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	keys := map[string]string{
		"#2121 pending-import source blob": blobkeys.BuildRestoreUploadKey(f.tenantID, "backup.inb"),
		"fresh upload (blob-before-row)":   blobkeys.BuildFileUploadKey(f.tenantID, "invoice-123.pdf"),
		"failed-export artifact":           blobkeys.BuildBackupBlobKey(f.tenantID, "full", "20260101"),
		"seed fixture":                     blobkeys.BuildSeedKey(f.tenantID, "seed-abc.jpg"),
		"legacy flat key (pre-#1793)":      "thumbnails/legacy-file-id_small.jpg",
		"legacy flat upload key":           "some-old-upload-1234.jpg",
	}
	for _, key := range keys {
		f.writeBlob(key, []byte("precious"))
	}
	f.backdateBlobs(30 * 24 * time.Hour)

	f.sweep()

	for name, key := range keys {
		c.Assert(f.blobExists(key), qt.IsTrue, qt.Commentf("NEVER-SWEEP blob was destroyed: %s (%s)", name, key))
	}
}

// A live file's thumbnail must survive: RequestThumbnailGeneration returns the
// EXISTING job and enqueues nothing unless its status is 'failed', so a deleted
// thumbnail on a live file does NOT self-heal — it would be a visible,
// user-facing regression.
func TestOrphanFileGC_LiveFileThumbnailIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	_, _, commodityID := f.seedLiveCommodity(f.groupID, f.userID)
	live := f.seedFile(seedFileOpts{linkType: "commodity", linkID: commodityID, linkMeta: "images"})

	small := blobkeys.BuildThumbnailBlobKey(f.tenantID, live.ID, "small")
	medium := blobkeys.BuildThumbnailBlobKey(f.tenantID, live.ID, "medium")
	f.writeBlob(small, []byte("s"))
	f.writeBlob(medium, []byte("m"))
	f.backdateBlobs(30 * 24 * time.Hour)

	f.sweep()

	c.Assert(f.blobExists(small), qt.IsTrue, qt.Commentf("a LIVE file's thumbnail was destroyed"))
	c.Assert(f.blobExists(medium), qt.IsTrue)
}

// Guards the thumbnail worker's detached Get→write window (bounded at 2 minutes
// by the detached-job timeout): a thumbnail written moments ago is inside the
// age gate and must be kept even if its owning row is already gone.
func TestOrphanFileGC_FreshThumbnailIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	key := blobkeys.BuildThumbnailBlobKey(f.tenantID, "no-such-file-id", "small")
	f.writeBlob(key, []byte("thumb")) // ModTime = now

	f.sweep()

	c.Assert(f.blobExists(key), qt.IsTrue, qt.Commentf("a freshly-written thumbnail was destroyed"))
}

// The ROUND-TRIP GUARD: a key is only deletable if it parses AND
// blobkeys.BuildThumbnailBlobKey rebuilds it byte-for-byte. Anything else is
// kept — the worker never hands a raw, externally-derived string to Delete.
func TestOrphanFileGC_UnparseableThumbnailKeyIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	prefix := blobkeys.TenantPrefix(f.tenantID) + blobkeys.ThumbnailsSegment + "/"
	keys := []string{
		prefix + "garbage",                    // no size, no extension
		prefix + "nested/dir/file_small.jpg",  // nesting is not a legal thumbnail key
		prefix + "some-file-id_large.jpg",     // 'large' is not a thumbnail size we write
		prefix + "some-file-id_small.png",     // thumbnails are always JPEG
		prefix + "_small.jpg",                 // empty file id
		prefix + "some-file-id_.jpg",          // empty size
		prefix + "some-file-id_small.jpg.bak", // suffix confusion
	}
	for _, key := range keys {
		f.writeBlob(key, []byte("x"))
	}
	f.backdateBlobs(30 * 24 * time.Hour)

	f.sweep()

	for _, key := range keys {
		c.Assert(f.blobExists(key), qt.IsTrue, qt.Commentf("a non-round-tripping key was deleted: %s", key))
	}
}

// A pending_deletion group belongs to GroupPurgeWorker; a non-active tenant may
// be mid-administrative-operation (#2115 hard delete). Both are under-collection,
// which is free.
func TestOrphanFileGC_InactiveGroupOrTenantIsNeverSwept(t *testing.T) {
	tests := []struct {
		name         string
		tenantStatus models.TenantStatus
		groupStatus  models.LocationGroupStatus
	}{
		{"group pending_deletion", models.TenantStatusActive, models.LocationGroupStatusPendingDeletion},
		{"tenant suspended", models.TenantStatusSuspended, models.LocationGroupStatusActive},
		{"tenant inactive", models.TenantStatusInactive, models.LocationGroupStatusActive},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			tenantID := f.newTenant("tenant-x", tt.tenantStatus)
			groupID := f.newGroup(tenantID, "group-x", tt.groupStatus)
			userID := f.newUser(tenantID, "x@example.com")

			file := f.seedFile(seedFileOpts{
				tenantID: tenantID, groupID: groupID, userID: userID,
				linkType: "commodity", linkID: "gone", linkMeta: "images",
			})

			f.sweep()

			c.Assert(f.fileExists(file.ID), qt.IsTrue,
				qt.Commentf("a file in an inactive group/tenant was destroyed"))
		})
	}
}

// Without a resolvable owner the delete cannot run under the file's own RLS
// scope. The worker KEEPS the file — it must never fall back to a raw
// service-mode row delete (which would also FK-fail on the thumbnail job chain).
func TestOrphanFileGC_UnresolvableOwnerIsNeverSwept(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(o *seedFileOpts)
		comment string
	}{
		{
			name:    "creator no longer exists",
			mutate:  func(o *seedFileOpts) { o.userID = "user-that-was-deleted" },
			comment: "a file whose creator is gone was deleted anyway",
		},
		{
			name:    "group no longer exists",
			mutate:  func(o *seedFileOpts) { o.groupID = "group-that-was-purged" },
			comment: "a file whose group is gone was deleted anyway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			opts := seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"}
			tt.mutate(&opts)
			file := f.seedFile(opts)

			f.sweep()

			c.Assert(f.fileExists(file.ID), qt.IsTrue, qt.Commentf("%s", tt.comment))
		})
	}
}

// ---------------------------------------------------------------------------
// POSITIVE — the residues the worker exists to reclaim
// ---------------------------------------------------------------------------

// The crash window: a process dies between an entity-row delete and its
// DeleteLinkedFiles, leaving file rows pointing at an id that will never exist
// again (entity ids are server-minted and never reused, so "entity X does not
// exist" is monotone and irreversible). Nothing else in the tree sweeps these
// for a live group.
//
// The commodity case matters most: DeleteCommodityRecursive has NO already-gone
// self-heal (unlike area/location), so a crashed commodity delete's files are
// permanent orphans today.
func TestOrphanFileGC_SweepsCrashWindowOrphanRows(t *testing.T) {
	tests := []string{"commodity", "area", "location"}

	for _, linkType := range tests {
		t.Run(linkType, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			blobKey := blobkeys.BuildFileUploadKey(f.tenantID, "orphan-"+linkType+".jpg")
			file := f.seedFile(seedFileOpts{
				linkType: linkType,
				linkID:   "id-of-an-entity-that-no-longer-exists",
				linkMeta: "images",
				blobKey:  blobKey,
			})
			// Its thumbnails go with it — DeleteFileWithPhysical removes them.
			small := blobkeys.BuildThumbnailBlobKey(f.tenantID, file.ID, "small")
			f.writeBlob(small, []byte("s"))

			f.sweep()

			c.Assert(f.fileExists(file.ID), qt.IsFalse, qt.Commentf("the orphan %s file row survived", linkType))
			c.Assert(f.blobExists(blobKey), qt.IsFalse, qt.Commentf("the orphan file's blob survived"))
			c.Assert(f.blobExists(small), qt.IsFalse, qt.Commentf("the orphan file's thumbnail survived"))
		})
	}
}

// The thumbnail mid-generation race: a file deleted while the RLS-bypassing
// thumbnail worker sits between its Get and its blob writes leaves thumbnails
// nothing will ever collect.
func TestOrphanFileGC_SweepsOrphanThumbnails(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	small := blobkeys.BuildThumbnailBlobKey(f.tenantID, "a-file-id-with-no-row", "small")
	medium := blobkeys.BuildThumbnailBlobKey(f.tenantID, "a-file-id-with-no-row", "medium")
	f.writeBlob(small, []byte("s"))
	f.writeBlob(medium, []byte("m"))
	f.backdateBlobs(30 * 24 * time.Hour)

	f.sweep()

	c.Assert(f.blobExists(small), qt.IsFalse, qt.Commentf("the orphan thumbnail survived"))
	c.Assert(f.blobExists(medium), qt.IsFalse)
}

// Two `run workers` replicas sweep concurrently (no cleanup worker in the tree
// takes a claim or a lock). Safety does not depend on a lock — every delete is
// re-verified against a monotone predicate and both the row delete and the blob
// delete are not-found-tolerant. A second identical tick must be a clean no-op.
func TestOrphanFileGC_IsIdempotent(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	blobKey := blobkeys.BuildFileUploadKey(f.tenantID, "orphan.jpg")
	file := f.seedFile(seedFileOpts{
		linkType: "commodity", linkID: "gone", linkMeta: "images", blobKey: blobKey,
	})
	thumb := blobkeys.BuildThumbnailBlobKey(f.tenantID, "no-row", "small")
	f.writeBlob(thumb, []byte("t"))
	f.backdateBlobs(30 * 24 * time.Hour)

	f.sweep()
	f.sweep() // the "other replica"

	c.Assert(f.fileExists(file.ID), qt.IsFalse)
	c.Assert(f.blobExists(blobKey), qt.IsFalse)
	c.Assert(f.blobExists(thumb), qt.IsFalse)
}

// ---------------------------------------------------------------------------
// Configuration surface
// ---------------------------------------------------------------------------

func TestParseOrphanFileGCMode(t *testing.T) {
	tests := []struct {
		in   string
		want services.OrphanFileGCMode
		ok   bool
	}{
		{"off", services.OrphanFileGCModeOff, true},
		{"report", services.OrphanFileGCModeReport, true},
		{"delete", services.OrphanFileGCModeDelete, true},
		{"", "", false},
		{"DELETE", "", false},
		{"purge", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			c := qt.New(t)
			got, ok := services.ParseOrphanFileGCMode(tt.in)
			c.Assert(ok, qt.Equals, tt.ok)
			c.Assert(got, qt.Equals, tt.want)
		})
	}
}

// The min-age floor is a safety property, not a preference: a programmatic
// caller cannot shrink it below MinOrphanFileGCMinAge (bootstrap fails fast on
// the same value, this is the defence-in-depth).
func TestOrphanFileGC_MinAgeFloorIsEnforced(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	// A genuine orphan, but only 2h old — well inside the 24h floor.
	file := f.seedFile(seedFileOpts{
		linkType: "commodity", linkID: "gone", linkMeta: "images",
		createdAgo: 2 * time.Hour, updatedAgo: 2 * time.Hour,
	})

	// Try to shrink the window to an hour. The option must refuse.
	f.sweep(services.WithOrphanFileGCMinAge(time.Hour))

	c.Assert(f.fileExists(file.ID), qt.IsTrue,
		qt.Commentf("the min-age floor was bypassed by WithOrphanFileGCMinAge"))
}

// ---------------------------------------------------------------------------
// Fixture helpers that need models the tests above reference
// ---------------------------------------------------------------------------

func (f *gcFixture) seedExport(tenantID, groupID string, status models.ExportStatus, imported bool, completed models.PTimestamp) {
	f.c.Helper()
	_, err := f.fs.ExportRegistryFactory.CreateServiceRegistry().Create(f.ctx, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: tenantID, GroupID: groupID, CreatedByUserID: f.userID,
		},
		Type:          models.ExportTypeFullDatabase,
		Status:        status,
		Imported:      imported,
		Description:   "seeded",
		CreatedDate:   models.NewPTimestamp(time.Now().Add(-2 * time.Hour)),
		CompletedDate: completed,
	})
	f.c.Assert(err, qt.IsNil)
}

func (f *gcFixture) seedRestore(tenantID, groupID string, status models.RestoreStatus, completed models.PTimestamp) {
	f.c.Helper()
	// A restore_operation needs an export to point at.
	exp, err := f.fs.ExportRegistryFactory.CreateServiceRegistry().Create(f.ctx, models.Export{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: tenantID, GroupID: groupID, CreatedByUserID: f.userID,
		},
		Type:        models.ExportTypeFullDatabase,
		Status:      models.ExportStatusCompleted,
		Description: "source",
		// Old enough that the export cooldown does not itself block the tenant —
		// the restore under test is what must do the blocking.
		CreatedDate:   models.NewPTimestamp(time.Now().Add(-30 * 24 * time.Hour)),
		CompletedDate: models.NewPTimestamp(time.Now().Add(-30 * 24 * time.Hour)),
	})
	f.c.Assert(err, qt.IsNil)

	_, err = f.fs.RestoreOperationRegistryFactory.CreateServiceRegistry().Create(f.ctx, models.RestoreOperation{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: tenantID, GroupID: groupID, CreatedByUserID: f.userID,
		},
		ExportID:      exp.ID,
		Description:   "seeded",
		Status:        status,
		Options:       models.RestoreOptions{Strategy: "merge_add"},
		CreatedDate:   models.NewPTimestamp(time.Now().Add(-2 * time.Hour)),
		CompletedDate: completed,
	})
	f.c.Assert(err, qt.IsNil)
}

// ---------------------------------------------------------------------------
// NEGATIVE — a blob key is NOT row-unique
// ---------------------------------------------------------------------------

// THE catastrophic case. The row delete is RLS-narrow, but the blob delete that
// follows it is KEY-scoped — and `files.original_path` has no unique index. An
// upload key is `t/<tenant>/files/<sanitized-name>-<unix SECONDS><ext>`
// (filekit.UploadFileName): no group segment, no row segment, no randomness. Two
// uploads of the same filename inside one tenant in the same second — two
// members of different groups, or one user multi-selecting two same-named files
// — produce two DISTINCT rows that share one blob key.
//
// So an orphan row can share its key with a LIVE row (here: a #2235 standalone
// file, which no other gate in this worker even looks at). Deleting the orphan's
// blob would destroy the live file's BYTES, irreversibly — `files` has no
// soft-delete and there is no trash. Anything ambiguous is KEPT: the orphan is
// not collected at all.
func TestOrphanFileGC_SharedBlobKeyIsNeverSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	// The exact collision production mints: same tenant, same basename, same second.
	shared := blobkeys.BuildFileUploadKey(f.tenantID, "receipt-1783824560.jpg")

	otherGroupID := f.newGroup(f.tenantID, "group-b", models.LocationGroupStatusActive)
	live := f.seedFile(seedFileOpts{
		groupID: otherGroupID, // a standalone file in ANOTHER group — 100% legitimate
		blobKey: shared,
	})
	orphan := f.seedFile(seedFileOpts{
		linkType: "commodity",
		linkID:   "id-of-an-entity-that-no-longer-exists",
		linkMeta: "images",
		blobKey:  shared, // same key, different row
	})

	f.sweep()

	c.Assert(f.blobExists(shared), qt.IsTrue,
		qt.Commentf("the GC destroyed the bytes of a LIVE file that shares the orphan's blob key"))
	c.Assert(f.fileExists(live.ID), qt.IsTrue)
	c.Assert(f.fileExists(orphan.ID), qt.IsTrue,
		qt.Commentf("an orphan whose blob key is shared must be KEPT WHOLE, not half-deleted"))
}

// The guard must not turn into a blanket refusal: an orphan that solely owns its
// key is still collected, blob and all.
func TestOrphanFileGC_SoleOwnedBlobKeyIsStillSwept(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	key := blobkeys.BuildFileUploadKey(f.tenantID, "sole-1783824560.jpg")
	orphan := f.seedFile(seedFileOpts{
		linkType: "commodity", linkID: "gone", linkMeta: "images", blobKey: key,
	})

	f.sweep()

	c.Assert(f.fileExists(orphan.ID), qt.IsFalse)
	c.Assert(f.blobExists(key), qt.IsFalse)
}

// ---------------------------------------------------------------------------
// The thumbnail-generation job chain
// ---------------------------------------------------------------------------

// A thumbnail job is owned by the user who REQUESTED generation, not by the
// file's creator (RequestThumbnailGeneration stamps the requesting user so one
// member cannot spend another's rate limit), and any group member who merely
// VIEWS a not-yet-generated thumbnail enqueues one. The GC deletes through the
// file's CREATOR, so a user-scoped teardown of the job chain cannot see — let
// alone delete — a co-member's job. On postgres the surviving job row then trips
// fk_thumbnail_job_file (NO ACTION; the FK check bypasses RLS) and the orphan can
// NEVER be collected: it fails on every tick, forever.
//
// The chain teardown therefore runs service-mode. Asserted here by the job row
// actually being gone (the memory backend has no FKs, so the row is the tell).
func TestOrphanFileGC_TearsDownAThumbnailJobOwnedByAnotherGroupMember(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	viewerID := f.newUser(f.tenantID, "viewer@example.com")

	orphan := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})
	f.seedThumbnailJob(orphan.ID, viewerID, f.tenantID) // requested by a co-member, not the creator

	f.sweep()

	c.Assert(f.thumbnailJobCount(orphan.ID), qt.Equals, 0,
		qt.Commentf("a co-member's thumbnail job survived the teardown — on postgres this FK-fails the delete forever"))
	c.Assert(f.fileExists(orphan.ID), qt.IsFalse)
}

// ---------------------------------------------------------------------------
// The forensic log is the recovery artifact — it must not lie
// ---------------------------------------------------------------------------

// `files` has no soft-delete, so the log line is the ONLY record of what was
// destroyed. A record that says action=deleted for a file that still exists is
// worse than no record: it sends an operator reconciling a data-loss report
// looking for a file that was never touched. "deleted" is emitted only after the
// delete RETURNS SUCCESS.
func TestOrphanFileGC_NeverLogsDeletedWhenTheDeleteFailed(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	orphan := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})

	deps := f.deps()
	deps.Deleter = &failingDeleter{err: errors.New("fk_thumbnail_job_file violation")}

	logs := captureLogs(c, func() {
		services.NewOrphanFileGCWorker(deps,
			services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
		).RunOnce(f.ctx)
	})

	c.Assert(logs, qt.Contains, `"action":"deleting"`,
		qt.Commentf("the pre-image must be written BEFORE the attempt — it is the recovery artifact"))
	c.Assert(logs, qt.Not(qt.Contains), `"action":"deleted"`,
		qt.Commentf("the log claimed a file was deleted, but the delete failed and the row is still there"))
	c.Assert(logs, qt.Contains, `"action":"failed"`)
	c.Assert(f.fileExists(orphan.ID), qt.IsTrue)
}

// The confirmation is emitted on the success path, so an operator can tell the
// two apart in the same grep.
func TestOrphanFileGC_LogsDeletedOnlyAfterTheDeleteSucceeded(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	orphan := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})

	logs := captureLogs(c, func() { f.sweep() })

	c.Assert(logs, qt.Contains, `"action":"deleted"`)
	c.Assert(f.fileExists(orphan.ID), qt.IsFalse)
}

// ---------------------------------------------------------------------------
// Thumbnail-sweep survival guards
// ---------------------------------------------------------------------------

// The blob-side gates need their own reds: the row-side tests seed no blob, so
// they cannot tell whether the THUMBNAIL sweep honours the in-flight gate and the
// tenant-status guard. A deleted thumbnail does not self-heal
// (RequestThumbnailGeneration returns the existing job unless it is 'failed'), so
// this is a user-visible regression, not a free re-render.
func TestOrphanFileGC_ThumbnailSweepHonoursTenantGuards(t *testing.T) {
	tests := []struct {
		name         string
		tenantStatus models.TenantStatus
		setup        func(f *gcFixture, tenantID, groupID string)
	}{
		{
			name:         "restore in flight pins the tenant",
			tenantStatus: models.TenantStatusActive,
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedRestore(tenantID, groupID, models.RestoreStatusRunning, nil)
			},
		},
		{
			name:         "export in flight pins the tenant",
			tenantStatus: models.TenantStatusActive,
			setup: func(f *gcFixture, tenantID, groupID string) {
				f.seedExport(tenantID, groupID, models.ExportStatusInProgress, false, nil)
			},
		},
		{
			name:         "suspended tenant is never swept",
			tenantStatus: models.TenantStatusSuspended,
			setup:        func(*gcFixture, string, string) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			f := newGCFixture(c)

			tenantID := f.newTenant("tenant-guard", tt.tenantStatus)
			groupID := f.newGroup(tenantID, "group-guard", models.LocationGroupStatusActive)
			f.newUser(tenantID, "guard@example.com")
			tt.setup(f, tenantID, groupID)

			// An aged thumbnail with no owning row — a genuine orphan by every
			// other rule. Only the tenant guard stands between it and deletion.
			key := blobkeys.BuildThumbnailBlobKey(tenantID, "a-file-id-with-no-row", "small")
			f.writeBlob(key, []byte("t"))
			f.backdateBlobs(30 * 24 * time.Hour)

			f.sweep()

			c.Assert(f.blobExists(key), qt.IsTrue,
				qt.Commentf("the thumbnail sweep ignored the tenant guard"))
		})
	}
}

// A transient failure of the thumbnail sweep's row probe (a DB timeout, a
// connection reset) must NEVER be read as "the row is gone". Only
// registry.ErrNotFound is evidence of absence; anything else aborts the tick.
func TestOrphanFileGC_ThumbnailProbeErrorNeverReadsAsGone(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	key := blobkeys.BuildThumbnailBlobKey(f.tenantID, "a-file-id-the-probe-cannot-resolve", "small")
	f.writeBlob(key, []byte("t"))
	f.backdateBlobs(30 * 24 * time.Hour)

	deps := f.deps()
	deps.Files = &failingGetFileRegistry{
		FileRegistry: f.fs.FileRegistryFactory.CreateServiceRegistry(),
		err:          context.DeadlineExceeded, // NOT registry.ErrNotFound
	}

	services.NewOrphanFileGCWorker(deps,
		services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
	).RunOnce(f.ctx)

	c.Assert(f.blobExists(key), qt.IsTrue,
		qt.Commentf("a transient probe failure was treated as 'the file row is gone'"))
}

// ---------------------------------------------------------------------------
// Liveness — a bounded tick must still make progress
// ---------------------------------------------------------------------------

// Head-of-line blocking. Re-verification KEEPS far more rows than it deletes, and
// several keep-reasons NEVER clear (a tenant pinned by a crashed restore, a
// suspended tenant, a purged owner). Those rows are also among the OLDEST
// orphans, so they sort to the front of an oldest-first window and — because
// nothing ever removes them — a non-resumable bounded scan would re-serve the
// same page every tick and never reach anything behind it.
//
// Here: a full page of permanently-skipped rows sits in front of a collectable
// orphan. The tick must page PAST them, within the tick.
func TestOrphanFileGC_SkippedRowsDoNotStarveTheScanWithinATick(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	// A suspended tenant: its rows are candidates forever and deleted never.
	deadTenant := f.newTenant("tenant-suspended", models.TenantStatusSuspended)
	deadGroup := f.newGroup(deadTenant, "group-suspended", models.LocationGroupStatusActive)
	deadUser := f.newUser(deadTenant, "dead@example.com")
	for range 4 {
		f.seedFile(seedFileOpts{
			tenantID: deadTenant, groupID: deadGroup, userID: deadUser,
			linkType: "commodity", linkID: "gone", linkMeta: "images",
			createdAgo: 40 * 24 * time.Hour, // OLDER than the collectable one below
		})
	}

	// The collectable orphan sorts behind all of them.
	orphan := f.seedFile(seedFileOpts{
		linkType: "commodity", linkID: "gone", linkMeta: "images",
		createdAgo: 10 * 24 * time.Hour,
	})

	// Two rows per candidate query — the skipped rows fill the first page whole.
	f.sweep(services.WithOrphanFileGCRowBudget(2, 100))

	c.Assert(f.fileExists(orphan.ID), qt.IsFalse,
		qt.Commentf("permanently-skipped rows squatted on the candidate window and starved a collectable orphan"))
}

// The same starvation across TICKS: when a tick runs out of its row budget the
// next one must RESUME from the keyset cursor, not restart at the oldest row.
// Otherwise the same head rows are re-examined forever and nothing behind them
// is ever reached — and in the shipping REPORT mode, where nothing is ever
// deleted, every row is a head row.
func TestOrphanFileGC_RowScanResumesOnTheNextTick(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	deadTenant := f.newTenant("tenant-suspended", models.TenantStatusSuspended)
	deadGroup := f.newGroup(deadTenant, "group-suspended", models.LocationGroupStatusActive)
	deadUser := f.newUser(deadTenant, "dead@example.com")
	for range 2 {
		f.seedFile(seedFileOpts{
			tenantID: deadTenant, groupID: deadGroup, userID: deadUser,
			linkType: "commodity", linkID: "gone", linkMeta: "images",
			createdAgo: 40 * 24 * time.Hour,
		})
	}

	orphan := f.seedFile(seedFileOpts{
		linkType: "commodity", linkID: "gone", linkMeta: "images",
		createdAgo: 10 * 24 * time.Hour,
	})

	// One row per tick: tick 1 can only reach the first skipped row.
	worker := services.NewOrphanFileGCWorker(f.deps(),
		services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
		services.WithOrphanFileGCRowBudget(1, 1),
	)

	worker.RunOnce(f.ctx)
	c.Assert(f.fileExists(orphan.ID), qt.IsTrue, qt.Commentf("the budget was not honoured"))

	// Ticks 2 and 3 must advance past the two skipped rows and reach the orphan.
	worker.RunOnce(f.ctx)
	worker.RunOnce(f.ctx)

	c.Assert(f.fileExists(orphan.ID), qt.IsFalse,
		qt.Commentf("the scan restarted from the top every tick — it can never reach past the skipped rows"))
}

// The thumbnail listing budget must be PER TENANT. A single shared budget is
// spent by whichever tenant is enumerated first — by its LIVE thumbnails, which
// outnumber orphans by orders of magnitude — and every tenant after it is then
// skipped on this tick and (the listing being lexicographic from the top, and
// nothing ever removing those keys) on every future tick too. The blob half of
// the GC would be a permanent no-op for most of the installation, invisibly.
func TestOrphanFileGC_OneTenantsThumbnailsDoNotStarveAnother(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	// Tenant A: live thumbnails that will exhaust a small budget.
	_, _, commodityID := f.seedLiveCommodity(f.groupID, f.userID)
	live := f.seedFile(seedFileOpts{linkType: "commodity", linkID: commodityID, linkMeta: "images"})
	f.writeBlob(blobkeys.BuildThumbnailBlobKey(f.tenantID, live.ID, "small"), []byte("s"))
	f.writeBlob(blobkeys.BuildThumbnailBlobKey(f.tenantID, live.ID, "medium"), []byte("m"))

	// Tenant B: one orphan thumbnail, behind tenant A in the iteration order.
	otherTenantID := f.newTenant(gcTenantB, models.TenantStatusActive)
	orphanKey := blobkeys.BuildThumbnailBlobKey(otherTenantID, "a-file-id-with-no-row", "small")
	f.writeBlob(orphanKey, []byte("o"))

	f.backdateBlobs(30 * 24 * time.Hour)

	// A budget of 2 keys is exactly tenant A's live thumbnails.
	f.sweep(services.WithOrphanFileGCThumbnailBudget(2))

	c.Assert(f.blobExists(orphanKey), qt.IsFalse,
		qt.Commentf("tenant A's live thumbnails consumed a SHARED budget and tenant B was never swept"))
}

// Within one tenant, a listing that runs out of budget must RESUME on the next
// tick. Live thumbnails are never deleted, so a truncated window would never
// advance on its own: every orphan sorting after the cutoff key would be
// unreachable forever.
func TestOrphanFileGC_ThumbnailListingResumesOnTheNextTick(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	prefix := blobkeys.TenantPrefix(f.tenantID) + blobkeys.ThumbnailsSegment + "/"

	// Two keys that are always KEPT (they do not round-trip), sorting FIRST, and
	// an orphan thumbnail sorting last. blob.List walks keys lexicographically.
	kept := []string{prefix + "aaa-1_large.jpg", prefix + "aaa-2_large.jpg"}
	for _, k := range kept {
		f.writeBlob(k, []byte("x"))
	}
	orphanKey := blobkeys.BuildThumbnailBlobKey(f.tenantID, "zzz-file-id-with-no-row", "small")
	f.writeBlob(orphanKey, []byte("o"))
	f.backdateBlobs(30 * 24 * time.Hour)

	worker := services.NewOrphanFileGCWorker(f.deps(),
		services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
		services.WithOrphanFileGCThumbnailBudget(2),
	)

	worker.RunOnce(f.ctx) // burns the whole budget on the two kept keys
	c.Assert(f.blobExists(orphanKey), qt.IsTrue, qt.Commentf("the per-tenant budget was not honoured"))

	worker.RunOnce(f.ctx) // must resume AFTER them

	c.Assert(f.blobExists(orphanKey), qt.IsFalse,
		qt.Commentf("the listing restarted from the top of the prefix — keys past the budget are unreachable forever"))
	for _, k := range kept {
		c.Assert(f.blobExists(k), qt.IsTrue)
	}
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// The ticker path is the one production actually runs; every other test drives
// RunOnce directly. Mirrors the EmailVerificationCleanupWorker /
// OperationSlotCleanupWorker lifecycle tests.
func TestOrphanFileGC_StartRunsTicksAndStopIsClean(t *testing.T) {
	c := qt.New(t)
	f := newGCFixture(c)

	orphan := f.seedFile(seedFileOpts{linkType: "commodity", linkID: "gone", linkMeta: "images"})

	worker := services.NewOrphanFileGCWorker(f.deps(),
		services.WithOrphanFileGCMode(services.OrphanFileGCModeDelete),
		services.WithOrphanFileGCInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)
	c.Assert(eventually(c, 2*time.Second, func() bool { return !f.fileExists(orphan.ID) }), qt.IsTrue,
		qt.Commentf("the ticker never drove a sweep"))

	worker.Stop() // returns cleanly: no hang, no panic
	worker.Stop() // idempotent
}

// Start is a no-op without the two dependencies that make a delete possible.
func TestOrphanFileGC_StartWithIncompleteDepsIsANoOp(t *testing.T) {
	worker := services.NewOrphanFileGCWorker(services.OrphanFileGCDeps{},
		services.WithOrphanFileGCInterval(10*time.Millisecond),
	)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	worker.Start(ctx)
	worker.Stop()
}

// Compile-time guard: registry.FileRegistry must satisfy the narrow, read-only
// slice the GC consumes, so a signature drift is caught here rather than at
// wiring time.
var _ services.OrphanFileGCFileRegistry = (registry.FileRegistry)(nil)
