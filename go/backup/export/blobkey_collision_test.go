//go:build !legacy_xml_backup

package export

import (
	"testing"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/internal/backupsign"
	_ "go.5x5.cz/inventario/internal/fileblob"
	"go.5x5.cz/inventario/models"
)

// testSigner is a fixed-seed signer: generateExport refuses to write an .inb
// without one, and the signature bytes are irrelevant to what these tests
// assert.
func testSigner() *backupsign.Signer {
	s := make([]byte, backupsign.SeedSize)
	for i := range s {
		s[i] = 0x7f
	}
	return must.Must(backupsign.NewSigner(s))
}

// TestGenerateExport_ConcurrentSameTypeKeysAreDistinct pins #2252 end to end.
//
// The archive key used to end in a second-granularity timestamp, so two
// exports of the same type in the same tenant that finished within the same
// wall-clock second computed the SAME key — and the writer has no existence
// check, so the second archive silently overwrote the first one's bytes. Both
// rows then pointed at one blob and one of them described bytes that no longer
// existed. MaxConcurrentExports defaults to 3 and the worker fans pending
// exports out in parallel, so a user double-clicking was enough.
//
// Running them back to back within the same second is what reproduced it; the
// test asserts the outcome that matters — two rows, two keys, both readable.
func TestGenerateExport_ConcurrentSameTypeKeysAreDistinct(t *testing.T) {
	c := qt.New(t)
	uploadLocation := "file:///" + c.TempDir() + "?create_dir=1"
	service := NewExportService(newTestFactorySet(), uploadLocation, testSigner())
	ctx := newTestContext()

	newExport := func(id string) models.Export {
		return models.Export{
			TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID(
				id, "test-tenant", testGroupID, testUserID),
			Type:            models.ExportTypeCommodities,
			Status:          models.ExportStatusPending,
			IncludeFileData: false,
		}
	}

	firstKey, _, err := service.generateExport(ctx, newExport("export-alpha"))
	c.Assert(err, qt.IsNil)
	secondKey, _, err := service.generateExport(ctx, newExport("export-beta"))
	c.Assert(err, qt.IsNil)

	c.Assert(firstKey, qt.Not(qt.Equals), secondKey,
		qt.Commentf("two exports of one type must not share an archive key"))

	b, err := blob.OpenBucket(ctx, uploadLocation)
	c.Assert(err, qt.IsNil)
	defer b.Close()

	// Both archives survive: neither overwrote the other.
	for _, key := range []string{firstKey, secondKey} {
		exists, err := b.Exists(ctx, key)
		c.Assert(err, qt.IsNil)
		c.Assert(exists, qt.IsTrue, qt.Commentf("archive %q is gone", key))

		attrs, err := b.Attributes(ctx, key)
		c.Assert(err, qt.IsNil)
		c.Assert(attrs.Size > 0, qt.IsTrue, qt.Commentf("archive %q is empty", key))
	}
}

// A retry of the SAME export row must reuse its key, so the retry overwrites
// whatever partial object the failed attempt left instead of orphaning it
// under a key nothing references.
func TestGenerateExport_RetryReusesTheSameKey(t *testing.T) {
	c := qt.New(t)
	uploadLocation := "file:///" + c.TempDir() + "?create_dir=1"
	service := NewExportService(newTestFactorySet(), uploadLocation, testSigner())
	ctx := newTestContext()

	export := models.Export{
		TenantGroupAwareEntityID: models.WithTenantGroupAwareEntityID(
			"export-retry", "test-tenant", testGroupID, testUserID),
		Type:   models.ExportTypeCommodities,
		Status: models.ExportStatusPending,
	}

	firstKey, _, err := service.generateExport(ctx, export)
	c.Assert(err, qt.IsNil)
	secondKey, _, err := service.generateExport(ctx, export)
	c.Assert(err, qt.IsNil)
	c.Assert(secondKey, qt.Equals, firstKey)
}
