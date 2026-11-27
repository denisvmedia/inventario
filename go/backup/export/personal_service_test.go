package export_test

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/export"
	"go.5x5.cz/inventario/internal/blobkeys"
	_ "go.5x5.cz/inventario/internal/fileblob" // register file://
	"go.5x5.cz/inventario/internal/inb"
	"go.5x5.cz/inventario/internal/metrics"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

const personalTenant = "tenant-a"

func personalUploadLocation(c *qt.C) string {
	dir := c.TempDir()
	if runtime.GOOS == "windows" {
		return "file:///" + dir + "?create_dir=1"
	}
	return "file://" + dir + "?create_dir=1"
}

// personalFixture is a group two members share, each of whom created an
// item, a file, a loan and a history entry in it.
type personalFixture struct {
	ctx            context.Context
	fs             *registry.FactorySet
	uploadLocation string
	alice, bob     *models.User
	group          *models.LocationGroup
	aliceItem      *models.Commodity
	bobItem        *models.Commodity
	aliceFile      *models.FileEntity
	bobFile        *models.FileEntity
}

func (f *personalFixture) as(user *models.User) context.Context {
	return appctx.WithGroup(appctx.WithUser(f.ctx, user), f.group)
}

func newPersonalFixture(c *qt.C) *personalFixture {
	c.Helper()
	f := &personalFixture{ctx: context.Background(), fs: memory.NewFactorySet(), uploadLocation: personalUploadLocation(c)}

	newUser := func(email, name string) *models.User {
		u, err := f.fs.UserRegistry.Create(f.ctx, models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: personalTenant},
			Email:               email,
			Name:                name,
			IsActive:            true,
		})
		c.Assert(err, qt.IsNil)
		return u
	}
	f.alice = newUser("alice@example.com", "Alice Example")
	f.bob = newUser("bob@example.com", "Bob Example")

	group, err := f.fs.LocationGroupRegistry.Create(f.ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: personalTenant},
		Slug:                "shared-group-slug-000000000000",
		Name:                "Family home",
		Status:              models.LocationGroupStatusActive,
		CreatedBy:           f.alice.ID,
		GroupCurrency:       "USD",
	})
	c.Assert(err, qt.IsNil)
	f.group = group
	for _, m := range []struct {
		user *models.User
		role models.GroupRole
	}{{f.alice, models.GroupRoleOwner}, {f.bob, models.GroupRoleUser}} {
		_, err := f.fs.GroupMembershipRegistry.Create(f.ctx, models.GroupMembership{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: personalTenant},
			GroupID:             group.ID,
			MemberUserID:        m.user.ID,
			Role:                m.role,
			JoinedAt:            time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		})
		c.Assert(err, qt.IsNil)
	}

	aliceCtx, bobCtx := f.as(f.alice), f.as(f.bob)
	locReg, err := f.fs.LocationRegistryFactory.CreateUserRegistry(aliceCtx)
	c.Assert(err, qt.IsNil)
	loc, err := locReg.Create(aliceCtx, models.Location{Name: "Main house"})
	c.Assert(err, qt.IsNil)
	areaReg, err := f.fs.AreaRegistryFactory.CreateUserRegistry(aliceCtx)
	c.Assert(err, qt.IsNil)
	area, err := areaReg.Create(aliceCtx, models.Area{Name: "Kitchen", LocationID: loc.ID})
	c.Assert(err, qt.IsNil)

	tagReg, err := f.fs.TagRegistryFactory.CreateUserRegistry(aliceCtx)
	c.Assert(err, qt.IsNil)
	_, err = tagReg.Create(aliceCtx, models.Tag{Kind: models.TagKindCommodity, Slug: "kitchen-gear", Label: "Kitchen gear", Color: models.TagColorBlue})
	c.Assert(err, qt.IsNil)

	f.aliceItem = f.createItem(c, aliceCtx, "Alice's kettle", area.ID, "kitchen-gear", "no-entry")
	f.bobItem = f.createItem(c, bobCtx, "Bob's toaster", area.ID)
	f.aliceFile = f.createFile(c, aliceCtx, f.alice, "manual", "alice-bytes", f.aliceItem.ID)
	f.bobFile = f.createFile(c, bobCtx, f.bob, "receipt", "bob-bytes", f.bobItem.ID)

	// Alice lends Bob's toaster; Bob lends Alice's kettle.
	f.createLoan(c, aliceCtx, f.bobItem.ID, "Neighbor Carol")
	f.createLoan(c, bobCtx, f.aliceItem.ID, "Cousin Dan")
	// Alice edits the comments of an item; the entry records the other
	// fields as they stood, some typed by Bob.
	f.createEvent(c, aliceCtx, f.aliceItem.ID, "Alice's note",
		models.CommodityEventPayload{"comments": "Comment Bob typed", "brand": "Brand Bob typed"},
		models.CommodityEventPayload{"comments": "Comment Alice typed", "brand": "Brand Bob typed", "model": "K-2"})
	f.createEvent(c, bobCtx, f.aliceItem.ID, "Bob's note", nil, nil)
	return f
}

func (f *personalFixture) createItem(c *qt.C, ctx context.Context, name, areaID string, tags ...string) *models.Commodity {
	c.Helper()
	reg, err := f.fs.CommodityRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	item, err := reg.Create(ctx, models.Commodity{
		Name:      name,
		ShortName: name,
		AreaID:    new(areaID),
		Count:     1,
		Status:    models.CommodityStatusInUse,
		Type:      "electronics",
		Tags:      tags,
	})
	c.Assert(err, qt.IsNil)
	return item
}

func (f *personalFixture) createFile(c *qt.C, ctx context.Context, owner *models.User, title, content, itemID string) *models.FileEntity {
	c.Helper()
	key := blobkeys.BuildFileBlobKey(personalTenant, title+"-blob", ".txt")
	bucket, err := blob.OpenBucket(f.ctx, f.uploadLocation)
	c.Assert(err, qt.IsNil)
	c.Assert(bucket.WriteAll(f.ctx, key, []byte(content), nil), qt.IsNil)
	c.Assert(bucket.Close(), qt.IsNil)

	reg, err := f.fs.FileRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	file, err := reg.Create(ctx, models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{TenantID: personalTenant, GroupID: f.group.ID, CreatedByUserID: owner.ID},
		Title:                    title,
		Type:                     models.FileTypeDocument,
		Links:                    []models.FileLink{{EntityType: models.FileLinkCommodity, EntityID: itemID, Meta: "manuals"}},
		File: &models.File{
			Path:         title,
			OriginalPath: key,
			Ext:          ".txt",
			MIMEType:     "text/plain",
			SizeBytes:    int64(len(content)),
		},
	})
	c.Assert(err, qt.IsNil)
	return file
}

func (f *personalFixture) createLoan(c *qt.C, ctx context.Context, itemID, borrower string) {
	c.Helper()
	reg, err := f.fs.CommodityLoanRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	_, err = reg.Create(ctx, models.CommodityLoan{CommodityID: itemID, BorrowerName: borrower, LentAt: models.Date("2026-05-01")})
	c.Assert(err, qt.IsNil)
}

func (f *personalFixture) createEvent(c *qt.C, ctx context.Context, itemID, note string, before, after models.CommodityEventPayload) {
	c.Helper()
	reg, err := f.fs.CommodityEventRegistryFactory.CreateUserRegistry(ctx)
	c.Assert(err, qt.IsNil)
	_, err = reg.Create(ctx, models.CommodityEvent{
		CommodityID: itemID,
		Kind:        models.CommodityEventKindUpdated,
		OccurredAt:  time.Date(2026, 5, 2, 10, 0, 0, 0, time.UTC),
		Before:      before,
		After:       after,
		Note:        note,
	})
	c.Assert(err, qt.IsNil)
}

// readArchive returns the members of a gzip-compressed tar, in order.
func readArchive(c *qt.C, r io.Reader) (names []string, members map[string][]byte) {
	c.Helper()
	gz, err := gzip.NewReader(r)
	c.Assert(err, qt.IsNil)
	tr := tar.NewReader(gz)
	members = map[string][]byte{}
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return names, members
		}
		c.Assert(err, qt.IsNil)
		data, err := io.ReadAll(tr)
		c.Assert(err, qt.IsNil)
		names = append(names, h.Name)
		members[h.Name] = data
	}
}

func buildExport(c *qt.C, f *personalFixture, svc *export.PersonalDataExportService, user *models.User) (*models.PersonalDataExport, []string, map[string][]byte) {
	c.Helper()
	requested, err := svc.Request(f.ctx, user)
	c.Assert(err, qt.IsNil)
	c.Assert(requested.Status, qt.Equals, models.PersonalDataExportStatusPending)
	c.Assert(svc.Process(f.ctx, requested.ID), qt.IsNil)

	e, reader, err := svc.Open(f.ctx, user.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	defer reader.Close()
	names, members := readArchive(c, reader)
	return e, names, members
}

func TestPersonalDataExport_HoldsOnlyTheSubjectsData(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation)

	e, names, members := buildExport(c, f, svc, f.alice)

	c.Assert(e.Status, qt.Equals, models.PersonalDataExportStatusCompleted)
	c.Assert(e.GroupCount, qt.Equals, 1)
	c.Assert(e.ItemCount, qt.Equals, 1)
	c.Assert(e.FileCount, qt.Equals, 1)
	c.Assert(e.SizeBytes > 0, qt.IsTrue)
	c.Assert(names[0], qt.Equals, export.PersonalManifestName)

	var manifest export.PersonalManifest
	c.Assert(json.Unmarshal(members[export.PersonalManifestName], &manifest), qt.IsNil)
	c.Assert(manifest.Kind, qt.Equals, export.PersonalDataExportKind)
	c.Assert(manifest.SubjectUserID, qt.Equals, f.alice.ID)
	c.Assert(manifest.ExportID, qt.Equals, e.ID)
	c.Assert(manifest.FormatVersion, qt.Equals, export.PersonalFormatVersion)
	c.Assert(manifest.Restorable, qt.IsFalse)
	c.Assert(manifest.Notice, qt.Contains, "cannot be restored")
	c.Assert(manifest.CreatedAt, qt.Not(qt.Equals), "")
	c.Assert(manifest.Groups, qt.HasLen, 1)

	var profile export.PersonalProfile
	c.Assert(json.Unmarshal(members[export.PersonalProfileName], &profile), qt.IsNil)
	c.Assert(profile.User.ID, qt.Equals, f.alice.ID)
	c.Assert(profile.User.Email, qt.Equals, "alice@example.com")
	c.Assert(profile.Memberships, qt.HasLen, 1)
	c.Assert(profile.Memberships[0].GroupName, qt.Equals, "Family home")
	c.Assert(profile.Memberships[0].Role, qt.Equals, string(models.GroupRoleOwner))
	c.Assert(profile.Memberships[0].JoinedAt, qt.Not(qt.Equals), "")

	groupDocName := "groups/" + f.group.UUID + "/group.json"
	var doc export.PersonalGroupDocument
	c.Assert(json.Unmarshal(members[groupDocName], &doc), qt.IsNil)
	c.Assert(doc.Group.Name, qt.Equals, "Family home")

	c.Assert(doc.Items, qt.HasLen, 1)
	c.Assert(doc.Items[0].ID, qt.Equals, f.aliceItem.UUID)
	c.Assert(doc.Items[0].Name, qt.Equals, "Alice's kettle")
	c.Assert(doc.Items[0].Location, qt.Equals, "Main house")
	c.Assert(doc.Items[0].Area, qt.Equals, "Kitchen")
	// Type and tags by the names members see; a tag with no entry by its slug.
	c.Assert(doc.Items[0].Type, qt.DeepEquals, &export.PersonalName{Name: "Electronics", Slug: "electronics"})
	c.Assert(doc.Items[0].Tags, qt.DeepEquals, []export.PersonalName{
		{Name: "Kitchen gear", Slug: "kitchen-gear"},
		{Name: "no-entry", Slug: "no-entry"},
	})

	c.Assert(doc.Files, qt.HasLen, 1)
	c.Assert(doc.Files[0].ID, qt.Equals, f.aliceFile.UUID)
	c.Assert(doc.Files[0].Links, qt.HasLen, 1)
	c.Assert(doc.Files[0].Links[0].EntityID, qt.Equals, f.aliceItem.UUID)
	c.Assert(string(members[doc.Files[0].Path]), qt.Equals, "alice-bytes")

	// The loan Alice recorded is on Bob's item: the item is named, Bob is not.
	c.Assert(doc.Loans, qt.HasLen, 1)
	c.Assert(doc.Loans[0].BorrowerName, qt.Equals, "Neighbor Carol")
	c.Assert(doc.Loans[0].ItemID, qt.Equals, f.bobItem.UUID)

	// Of Alice's entry, only what she changed, with the values she wrote.
	c.Assert(doc.History, qt.HasLen, 1)
	c.Assert(doc.History[0].Note, qt.Equals, "Alice's note")
	c.Assert(doc.History[0].Changes, qt.DeepEquals, map[string]any{"comments": "Comment Alice typed", "model": "K-2"})

	// Nothing of Bob's: not his rows, his bytes, his name or his email.
	for name, data := range members {
		for _, leak := range []string{
			"Bob Example", "bob@example.com", "bob-bytes", "Cousin Dan", "Bob's note", f.bobFile.UUID,
			"Comment Bob typed", "Brand Bob typed",
		} {
			c.Assert(string(data), qt.Not(qt.Contains), leak, qt.Commentf("%s holds %q", name, leak))
		}
	}
}

func TestPersonalDataExport_OneRequestPerDay(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	now := time.Date(2026, 11, 26, 12, 0, 0, 0, time.UTC)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportClock(func() time.Time { return now }))

	first, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)

	now = now.Add(23 * time.Hour)
	_, err = svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.ErrorIs, export.ErrPersonalDataExportRateLimited)
	limited, ok := errors.AsType[*export.PersonalDataExportRateLimitedError](err)
	c.Assert(ok, qt.IsTrue)
	c.Assert(limited.RetryAt.Equal(first.CreatedAt.Add(24*time.Hour)), qt.IsTrue)

	exports, err := svc.List(f.ctx, f.alice.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(svc.NextRequestAt(exports).Equal(limited.RetryAt), qt.IsTrue)

	// Another user is not limited by Alice's request.
	_, err = svc.Request(f.ctx, f.bob)
	c.Assert(err, qt.IsNil)

	now = now.Add(time.Hour)
	_, err = svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	exports, err = svc.List(f.ctx, f.alice.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(exports, qt.HasLen, 2)
}

func TestPersonalDataExport_OnlyTheOwnerCanReachIt(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation)

	e, _, _ := buildExport(c, f, svc, f.alice)

	_, err := svc.Get(f.ctx, f.bob.ID, e.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	_, _, err = svc.Open(f.ctx, f.bob.ID, e.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
	bobs, err := svc.List(f.ctx, f.bob.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(bobs, qt.HasLen, 0)
}

func TestPersonalDataExport_NotReadyUntilBuilt(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation)

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	_, _, err = svc.Open(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.ErrorIs, export.ErrPersonalDataExportNotReady)
}

type recordingNotifier struct {
	users   []string
	exports []*models.PersonalDataExport
}

func (n *recordingNotifier) PersonalDataExportReady(_ context.Context, user *models.User, e *models.PersonalDataExport) error {
	n.users = append(n.users, user.ID)
	n.exports = append(n.exports, e)
	return nil
}

func TestPersonalDataExport_NotifiesWhenReady(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	notifier := &recordingNotifier{}
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportNotifier(notifier))

	e, _, _ := buildExport(c, f, svc, f.alice)

	c.Assert(notifier.users, qt.DeepEquals, []string{f.alice.ID})
	c.Assert(notifier.exports[0].ID, qt.Equals, e.ID)
	c.Assert(notifier.exports[0].ExpiresAt.Equal(e.CompletedAt.Add(models.PersonalDataExportRetention)), qt.IsTrue)
}

func TestPersonalDataExport_ExpiresAfterSevenDays(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	now := time.Now().UTC()
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportClock(func() time.Time { return now }))

	e, _, _ := buildExport(c, f, svc, f.alice)
	key := blobkeys.BuildPersonalDataExportBlobKey(personalTenant, e.ID)
	bucket, err := blob.OpenBucket(f.ctx, f.uploadLocation)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()

	now = now.Add(models.PersonalDataExportRetention - time.Minute)
	n, err := svc.ExpireDue(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	now = now.Add(2 * time.Minute)
	// Past its expiry and before the worker runs, the export is gone for
	// the user already.
	listed, err := svc.List(f.ctx, f.alice.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(listed, qt.HasLen, 0)

	n, err = svc.ExpireDue(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	exists, err := bucket.Exists(f.ctx, key)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
	_, err = f.fs.PersonalDataExportRegistry.Get(f.ctx, e.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestPersonalDataExport_SweepDeletesArchivesWithoutARow(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	now := time.Now().UTC()
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportClock(func() time.Time { return now }))

	kept, _, _ := buildExport(c, f, svc, f.alice)
	bucket, err := blob.OpenBucket(f.ctx, f.uploadLocation)
	c.Assert(err, qt.IsNil)
	defer bucket.Close()
	orphan := blobkeys.BuildPersonalDataExportBlobKey(personalTenant, "00000000-0000-0000-0000-000000000000")
	c.Assert(bucket.WriteAll(f.ctx, orphan, []byte("left behind"), nil), qt.IsNil)

	// Within the grace period an archive may belong to a build in progress.
	n, err := svc.SweepOrphanedArchives(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	now = now.Add(2 * time.Hour)
	n, err = svc.SweepOrphanedArchives(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 1)
	exists, err := bucket.Exists(f.ctx, orphan)
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsFalse)
	exists, err = bucket.Exists(f.ctx, blobkeys.BuildPersonalDataExportBlobKey(personalTenant, kept.ID))
	c.Assert(err, qt.IsNil)
	c.Assert(exists, qt.IsTrue)
}

func TestPersonalDataExport_IsRefusedAsABackup(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation)

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	c.Assert(svc.Process(f.ctx, requested.ID), qt.IsNil)
	_, reader, err := svc.Open(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	defer reader.Close()
	archive, err := io.ReadAll(reader)
	c.Assert(err, qt.IsNil)

	c.Assert(inb.IsPersonalDataExport(archive[:min(len(archive), inb.SniffHeadSize)]), qt.IsTrue)
	_, _, err = inb.ReadContainer(strings.NewReader(string(archive)), inb.DefaultLimits())
	c.Assert(err, qt.ErrorIs, inb.ErrPersonalDataExport)
}

// A file deleted after the build planned the archive is not in it, and the
// archive says so: the build starts over and lists the file as missing.
func TestPersonalDataExport_FileDeletedDuringTheBuildIsListedAsMissing(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation)
	var once sync.Once
	export.SetPersonalDataExportAfterPlanHookForTesting(t, func() {
		once.Do(func() {
			bucket, err := blob.OpenBucket(f.ctx, f.uploadLocation)
			c.Assert(err, qt.IsNil)
			c.Assert(bucket.Delete(f.ctx, f.aliceFile.OriginalPath), qt.IsNil)
			c.Assert(bucket.Close(), qt.IsNil)
		})
	})

	e, names, members := buildExport(c, f, svc, f.alice)
	c.Assert(e.Status, qt.Equals, models.PersonalDataExportStatusCompleted)
	c.Assert(e.FileCount, qt.Equals, 0)

	var manifest export.PersonalManifest
	c.Assert(json.Unmarshal(members[export.PersonalManifestName], &manifest), qt.IsNil)
	c.Assert(manifest.Statistics.FileCount, qt.Equals, 0)
	c.Assert(manifest.Statistics.MissingFileCount, qt.Equals, 1)

	var doc export.PersonalGroupDocument
	c.Assert(json.Unmarshal(members[manifest.Groups[0].File], &doc), qt.IsNil)
	c.Assert(doc.Files, qt.HasLen, 1)
	c.Assert(doc.Files[0].Missing, qt.IsTrue)
	c.Assert(doc.Files[0].Path, qt.Equals, "")
	for _, name := range names {
		c.Assert(name, qt.Not(qt.Contains), "/files/")
	}
}

// A build stopped by the worker shutting down goes back to the queue instead
// of failing, and the next build finishes it.
func TestPersonalDataExport_InterruptedBuildIsQueuedAgain(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation)

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	claimed, err := svc.Claim(f.ctx, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)

	stopping, cancel := context.WithCancel(f.ctx)
	cancel()
	c.Assert(svc.Process(stopping, requested.ID), qt.IsNotNil)
	got, err := svc.Get(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusPending)

	claimed, err = svc.Claim(f.ctx, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	c.Assert(svc.Process(f.ctx, requested.ID), qt.IsNil)
	got, err = svc.Get(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusCompleted)
}

// A build whose process died stays in progress with no result; after an hour
// the worker queues it again, so the user still gets the copy.
func TestPersonalDataExport_StalledBuildIsQueuedAgain(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportClock(func() time.Time { return now }))

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	claimed, err := svc.Claim(f.ctx, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)

	now = now.Add(30 * time.Minute)
	requeued, failed, err := svc.RecoverStalled(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 0)
	c.Assert(failed, qt.Equals, 0)

	now = now.Add(time.Hour)
	requeued, failed, err = svc.RecoverStalled(f.ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(requeued, qt.Equals, 1)
	c.Assert(failed, qt.Equals, 0)
	got, err := svc.Get(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusPending)

	claimed, err = svc.Claim(f.ctx, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	c.Assert(svc.Process(f.ctx, requested.ID), qt.IsNil)
}

// A build whose worker dies on each of its three builds fails for good, so
// it is listed among the failed jobs and counted once, instead of being
// queued again every hour.
func TestPersonalDataExport_StalledBuildFailsAfterItsLastBuild(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportClock(func() time.Time { return now }))
	counter := metrics.JobsDeadLetteredFor(string(models.FailedJobTypePersonalDataExport))
	before := testutil.ToFloat64(counter)

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	// deadBuild claims the export and lets its build die.
	deadBuild := func() (requeued, failed int) {
		claimed, err := svc.Claim(f.ctx, requested.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(claimed, qt.IsTrue)
		now = now.Add(2 * time.Hour)
		requeued, failed, err = svc.RecoverStalled(f.ctx)
		c.Assert(err, qt.IsNil)
		return requeued, failed
	}
	for build := 1; build <= 2; build++ {
		requeued, failed := deadBuild()
		c.Assert([]int{requeued, failed}, qt.DeepEquals, []int{1, 0}, qt.Commentf("build %d is queued again", build))
	}
	requeued, failed := deadBuild()
	c.Assert([]int{requeued, failed}, qt.DeepEquals, []int{0, 1}, qt.Commentf("the third build fails the export"))

	got, err := svc.Get(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusFailed)
	c.Assert(got.ErrorMessage, qt.Equals, export.StalledPersonalDataExportError)
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, 1.0)

	jobs, err := f.fs.FailedJobRegistry.ListFailed(f.ctx, models.FailedJobTypePersonalDataExport, registry.FailedJobScope{}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(jobs, qt.HasLen, 1)
	c.Assert(jobs[0].ID, qt.Equals, requested.ID)
	c.Assert(jobs[0].Attempts, qt.Equals, 3)
}

// A build that fails is a failed job: it is listed among the failed jobs of
// the installation with the user's email and the time it failed, and counts
// once as dead-lettered.
func TestPersonalDataExport_InactiveAccountFails(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportClock(func() time.Time { return now }))
	counter := metrics.JobsDeadLetteredFor(string(models.FailedJobTypePersonalDataExport))
	before := testutil.ToFloat64(counter)

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	f.alice.IsActive = false
	_, err = f.fs.UserRegistry.Update(f.ctx, *f.alice)
	c.Assert(err, qt.IsNil)

	claimed, err := svc.Claim(f.ctx, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	now = now.Add(time.Minute)
	c.Assert(svc.Process(f.ctx, requested.ID), qt.IsNotNil)
	got, err := svc.Get(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusFailed)
	c.Assert(testutil.ToFloat64(counter)-before, qt.Equals, 1.0)

	jobs, err := f.fs.FailedJobRegistry.ListFailed(f.ctx, models.FailedJobTypePersonalDataExport, registry.FailedJobScope{}, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(jobs, qt.HasLen, 1)
	job := jobs[0]
	c.Assert(job.ID, qt.Equals, requested.ID)
	c.Assert(job.TenantID, qt.Equals, f.alice.TenantID)
	c.Assert(job.UserID, qt.Equals, f.alice.ID)
	c.Assert(job.GroupID, qt.Equals, "")
	c.Assert(job.Title, qt.Equals, "alice@example.com")
	c.Assert(job.Error, qt.Equals, "the account is not active")
	c.Assert(job.Attempts, qt.Equals, 1)
	c.Assert(job.FailedAt, qt.IsNotNil)
	c.Assert(job.FailedAt.Equal(now), qt.IsTrue, qt.Commentf("failed at %s", job.FailedAt))
}

// A retry of a failed export builds it again from the start. The row is kept
// while it waits for the worker, and the 7 days start again when the build
// finishes. It is no new request, so the daily limit still counts from the
// first one.
func TestPersonalDataExport_RetryBuildsItAgain(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	now := time.Date(2026, 11, 26, 10, 0, 0, 0, time.UTC)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation, export.WithPersonalDataExportClock(func() time.Time { return now }))

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	f.alice.IsActive = false
	_, err = f.fs.UserRegistry.Update(f.ctx, *f.alice)
	c.Assert(err, qt.IsNil)
	claimed, err := svc.Claim(f.ctx, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	c.Assert(svc.Process(f.ctx, requested.ID), qt.IsNotNil)

	f.alice.IsActive = true
	_, err = f.fs.UserRegistry.Update(f.ctx, *f.alice)
	c.Assert(err, qt.IsNil)
	// Six days later, a day before the failed row would expire.
	now = now.Add(6 * 24 * time.Hour)
	c.Assert(f.fs.FailedJobRegistry.Retry(f.ctx, models.FailedJobTypePersonalDataExport, registry.FailedJobScope{}, requested.ID, now), qt.IsNil)

	got, err := svc.Get(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusPending)
	c.Assert(got.ErrorMessage, qt.Equals, "")
	c.Assert(got.StartedAt, qt.IsNil)
	c.Assert(got.CompletedAt, qt.IsNil)
	c.Assert(got.BlobKey, qt.Equals, "")
	c.Assert(got.ExpiresAt.After(now.Add(6*24*time.Hour)), qt.IsTrue,
		qt.Commentf("the row waits for the build: expires at %s", got.ExpiresAt))
	c.Assert(got.CreatedAt.Equal(requested.CreatedAt), qt.IsTrue, qt.Commentf("the request keeps its time"))

	// The build after the retry is the export's second.
	now = now.Add(2 * 24 * time.Hour)
	claimed, err = svc.Claim(f.ctx, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(claimed, qt.IsTrue)
	c.Assert(svc.Process(f.ctx, requested.ID), qt.IsNil)
	got, err = svc.Get(f.ctx, f.alice.ID, requested.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(got.Status, qt.Equals, models.PersonalDataExportStatusCompleted)
	c.Assert(got.AttemptCount, qt.Equals, 2)
	c.Assert(got.ExpiresAt.Equal(now.Add(models.PersonalDataExportRetention)), qt.IsTrue,
		qt.Commentf("the 7 days start when the build finishes: expires at %s", got.ExpiresAt))
}

func TestExportWorker_BuildsPersonalDataExports(t *testing.T) {
	c := qt.New(t)
	f := newPersonalFixture(c)
	svc := export.NewPersonalDataExportService(f.fs, f.uploadLocation)
	worker := export.NewExportWorker(export.NewExportService(f.fs, f.uploadLocation, nil), f.fs, 2,
		export.WithPollInterval(10*time.Millisecond), export.WithPersonalDataExports(svc))

	requested, err := svc.Request(f.ctx, f.alice)
	c.Assert(err, qt.IsNil)
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()
	worker.Start(ctx)
	defer worker.Stop()

	deadline := time.Now().Add(10 * time.Second)
	for {
		got, err := svc.Get(f.ctx, f.alice.ID, requested.ID)
		c.Assert(err, qt.IsNil)
		if got.Status == models.PersonalDataExportStatusCompleted {
			c.Assert(got.IsDownloadable(time.Now()), qt.IsTrue)
			return
		}
		c.Assert(got.Status, qt.Not(qt.Equals), models.PersonalDataExportStatusFailed)
		if time.Now().After(deadline) {
			c.Fatalf("export still %s", got.Status)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
