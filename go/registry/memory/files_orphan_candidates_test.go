package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// noCursor starts a candidate scan at the oldest row.
var noCursor registry.OrphanCandidateCursor

// seedOrphanFile plants a file row through the service registry so the test
// controls tenant/group/creator/timestamps exactly.
func seedOrphanFile(c *qt.C, fs *registry.FactorySet, tenantID, groupID, linkType, linkID string, age time.Duration) string {
	c.Helper()
	return seedOrphanFileWithPath(c, fs, tenantID, groupID, linkType, linkID, age, "s.jpg")
}

// seedOrphanFileWithPath is seedOrphanFile with control over the blob key, so a
// test can plant two DISTINCT rows that share one original_path (the collision
// that makes a key-scoped blob delete destroy a live file's bytes).
func seedOrphanFileWithPath(c *qt.C, fs *registry.FactorySet, tenantID, groupID, linkType, linkID string, age time.Duration, originalPath string) string {
	c.Helper()
	now := time.Now()
	f, err := fs.FileRegistryFactory.CreateServiceRegistry().Create(context.Background(), models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID: tenantID, GroupID: groupID, CreatedByUserID: "user-1",
		},
		Title:            "seeded",
		Type:             models.FileTypeImage,
		LinkedEntityType: linkType,
		LinkedEntityID:   linkID,
		CreatedAt:        now.Add(-age),
		UpdatedAt:        now.Add(-age),
		File:             &models.File{Path: "s", OriginalPath: originalPath, Ext: ".jpg", MIMEType: "image/jpeg"},
	})
	c.Assert(err, qt.IsNil)
	return f.ID
}

// The memory backend must apply the SAME positive allowlist as the postgres
// anti-join (#2237). It is an allowlist, not a negation, precisely so that a
// STANDALONE file (linked_entity_type = "", first-class since #2235), an
// export-linked file, and any unknown/future link type can never enter the
// candidate set a destructive worker consumes.
func TestMemoryFileRegistry_ListOrphanCandidates_Allowlist(t *testing.T) {
	tests := []struct {
		name     string
		linkType string
		linkID   string
		want     bool
	}{
		{"commodity orphan is a candidate", "commodity", "gone", true},
		{"area orphan is a candidate", "area", "gone", true},
		{"location orphan is a candidate", "location", "gone", true},
		{"standalone file is NEVER a candidate (#2235)", "", "", false},
		{"standalone with a stray id is NEVER a candidate", "", "gone", false},
		{"export-linked file is NEVER a candidate", "export", "gone", false},
		{"unknown link type fails closed", "widget", "gone", false},
		{"malformed link (empty id) is data, not garbage", "commodity", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			fs := memory.NewFactorySet()

			id := seedOrphanFile(c, fs, "tenant-1", "group-1", tt.linkType, tt.linkID, 30*24*time.Hour)

			got, err := fs.FileRegistryFactory.CreateServiceRegistry().
				ListOrphanCandidates(context.Background(), time.Now().Add(-72*time.Hour), noCursor, 100)
			c.Assert(err, qt.IsNil)

			found := false
			for _, f := range got {
				if f.ID == id {
					found = true
				}
			}
			c.Assert(found, qt.Equals, tt.want)
		})
	}
}

// BOTH timestamps must clear the age gate. updated_at is the load-bearing one:
// PUT /files/{id} stamps it with the app wall clock, so a file attached
// concurrently with an entity delete is immune for the whole window.
func TestMemoryFileRegistry_ListOrphanCandidates_AgeGate(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()

	oldID := seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", 30*24*time.Hour)
	youngID := seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", time.Hour)

	got, err := fs.FileRegistryFactory.CreateServiceRegistry().
		ListOrphanCandidates(context.Background(), time.Now().Add(-72*time.Hour), noCursor, 100)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.HasLen, 1)
	c.Assert(got[0].ID, qt.Equals, oldID)
	c.Assert(got[0].ID, qt.Not(qt.Equals), youngID)
}

// The tick is bounded so a pathological install cannot blow memory; the
// remainder is picked up on the next tick, so the ordering must be stable
// (oldest first).
func TestMemoryFileRegistry_ListOrphanCandidates_LimitAndOrdering(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()

	// Oldest last on insertion, so a naive implementation that skips the sort
	// would return them in the wrong order.
	seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", 10*24*time.Hour)
	seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", 30*24*time.Hour)
	seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", 20*24*time.Hour)

	reg := fs.FileRegistryFactory.CreateServiceRegistry()
	ctx := context.Background()
	cutoff := time.Now().Add(-72 * time.Hour)

	got, err := reg.ListOrphanCandidates(ctx, cutoff, noCursor, 2)
	c.Assert(err, qt.IsNil)
	c.Assert(got, qt.HasLen, 2)
	c.Assert(got[0].CreatedAt.After(got[1].CreatedAt), qt.IsFalse)

	none, err := reg.ListOrphanCandidates(ctx, cutoff, noCursor, 0)
	c.Assert(err, qt.IsNil)
	c.Assert(none, qt.HasLen, 0)
}

// A bare NewFileRegistryFactory() has no sibling registries to probe, so it
// cannot tell whether a linked entity exists. It must FAIL CLOSED — returning
// an error rather than reporting "the entity is gone" on no evidence.
func TestMemoryFileRegistry_ListOrphanCandidates_FailsClosedWithoutSiblings(t *testing.T) {
	c := qt.New(t)

	reg := memory.NewFileRegistryFactory().CreateServiceRegistry()
	_, err := reg.ListOrphanCandidates(context.Background(), time.Now(), noCursor, 100)
	c.Assert(err, qt.IsNotNil)
}

// The scan must be RESUMABLE from a (created_at, id) keyset cursor. Most
// candidates a tick looks at are KEPT, not deleted, and several keep-reasons
// never clear (a tenant pinned by a crashed restore, a suspended tenant, a
// purged owner), so a non-resumable oldest-first window is squatted on by those
// rows forever and no other orphan is ever enumerated — and in report mode,
// where nothing is ever deleted, EVERY row is a kept row.
func TestMemoryFileRegistry_ListOrphanCandidates_KeysetCursor(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()

	seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", 30*24*time.Hour)
	seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", 20*24*time.Hour)
	seedOrphanFile(c, fs, "tenant-1", "group-1", "commodity", "gone", 10*24*time.Hour)

	reg := fs.FileRegistryFactory.CreateServiceRegistry()
	ctx := context.Background()
	cutoff := time.Now().Add(-72 * time.Hour)

	// Page through the whole set one row at a time, deleting nothing — exactly
	// what report mode does.
	var seen []string
	cursor := noCursor
	for range 3 {
		page, err := reg.ListOrphanCandidates(ctx, cutoff, cursor, 1)
		c.Assert(err, qt.IsNil)
		c.Assert(page, qt.HasLen, 1)
		seen = append(seen, page[0].ID)
		cursor = registry.OrphanCandidateCursor{CreatedAt: page[0].CreatedAt, ID: page[0].ID}
	}

	// Three DISTINCT rows, oldest first — not the same row three times.
	c.Assert(seen[0], qt.Not(qt.Equals), seen[1])
	c.Assert(seen[1], qt.Not(qt.Equals), seen[2])
	c.Assert(seen[0], qt.Not(qt.Equals), seen[2])

	// The scan is now exhausted, which is how the worker knows to rewind.
	empty, err := reg.ListOrphanCandidates(ctx, cutoff, cursor, 10)
	c.Assert(err, qt.IsNil)
	c.Assert(empty, qt.HasLen, 0)
}

// A blob key is NOT row-unique — there is no unique index on original_path, and
// an upload key carries only a sanitized filename plus a unix SECOND. Two rows
// in one tenant can legitimately share one key, and deleting one row's blob by
// key destroys the other row's bytes. CountByOriginalPath is what lets the GC
// see that, so it must count rows the caller cannot otherwise see: every tenant,
// every group.
func TestMemoryFileRegistry_ListIDsByOriginalPath(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()

	// A pre-#2241 key: `<name>-<unix SECONDS><ext>`, which two same-named
	// uploads in one second collide on. Rows like these are already sitting in
	// deployed databases and can never be un-collided by a code change, so the
	// delete paths must keep asking who else points at the blob.
	const shared = "t/tenant-1/files/receipt-1783824560.jpg"
	a := seedOrphanFileWithPath(c, fs, "tenant-1", "group-1", "commodity", "gone", 30*24*time.Hour, shared)
	b := seedOrphanFileWithPath(c, fs, "tenant-1", "group-2", "", "", 30*24*time.Hour, shared)
	sole := seedOrphanFileWithPath(c, fs, "tenant-1", "group-1", "", "", 30*24*time.Hour, "t/tenant-1/files/sole-1783824560.jpg")

	reg := fs.FileRegistryFactory.CreateServiceRegistry()
	ctx := context.Background()

	// Service mode: the row in the OTHER group must be visible, or the delete
	// that consults this would happily destroy its bytes.
	ids, err := reg.ListIDsByOriginalPath(ctx, shared)
	c.Assert(err, qt.IsNil)
	c.Assert(ids, qt.HasLen, 2)
	c.Assert(ids, qt.Contains, a)
	c.Assert(ids, qt.Contains, b, qt.Commentf("a sharer in another group was invisible"))

	ids, err = reg.ListIDsByOriginalPath(ctx, "t/tenant-1/files/sole-1783824560.jpg")
	c.Assert(err, qt.IsNil)
	c.Assert(ids, qt.DeepEquals, []string{sole})

	ids, err = reg.ListIDsByOriginalPath(ctx, "t/tenant-1/files/never-uploaded.jpg")
	c.Assert(err, qt.IsNil)
	c.Assert(ids, qt.HasLen, 0)

	// "" is the sentinel for a row with no blob, not a key. It must never come
	// back as "these rows reference it".
	ids, err = reg.ListIDsByOriginalPath(ctx, "")
	c.Assert(err, qt.IsNil)
	c.Assert(ids, qt.HasLen, 0)
}

func TestMemoryFileRegistry_ExistingIDs(t *testing.T) {
	c := qt.New(t)
	fs := memory.NewFactorySet()

	a1 := seedOrphanFile(c, fs, "tenant-1", "group-1", "", "", time.Hour)
	a2 := seedOrphanFile(c, fs, "tenant-1", "group-2", "", "", time.Hour)
	b1 := seedOrphanFile(c, fs, "tenant-2", "group-3", "", "", time.Hour)

	reg := fs.FileRegistryFactory.CreateServiceRegistry()

	// Service mode answers by ID ALONE: an id is "live" wherever it lives, so a
	// thumbnail whose owning row sits in another group is never mistaken for an
	// orphan.
	ids, err := reg.ExistingIDs(context.Background(), []string{a1, a2, b1, "no-such-file"})
	c.Assert(err, qt.IsNil)

	set := make(map[string]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	c.Assert(set[a1], qt.IsTrue)
	c.Assert(set[a2], qt.IsTrue, qt.Commentf("a live row in another group of the tenant was reported missing"))
	c.Assert(set[b1], qt.IsTrue, qt.Commentf("a live row in another tenant was reported missing"))
	c.Assert(set["no-such-file"], qt.IsFalse)
	c.Assert(ids, qt.HasLen, 3)

	// Empty input never queries.
	ids, err = reg.ExistingIDs(context.Background(), nil)
	c.Assert(err, qt.IsNil)
	c.Assert(ids, qt.HasLen, 0)
}
