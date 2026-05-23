package blobkeys_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/blobkeys"
)

func TestTenantPrefix(t *testing.T) {
	c := qt.New(t)
	c.Assert(blobkeys.TenantPrefix("tenant-a"), qt.Equals, "t/tenant-a/")
	c.Assert(blobkeys.TenantPrefix(""), qt.Equals, "")
}

func TestBuildFileBlobKey(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		fileID   string
		ext      string
		expected string
	}{
		{"with extension", "tenant-a", "file-1", ".pdf", "t/tenant-a/files/file-1.pdf"},
		{"jpeg extension", "tenant-a", "file-2", ".jpg", "t/tenant-a/files/file-2.jpg"},
		{"empty extension", "tenant-a", "file-3", "", "t/tenant-a/files/file-3"},
		{"uuid file id", "tenant-a", "f47ac10b-58cc-4372-a567-0e02b2c3d479", ".png",
			"t/tenant-a/files/f47ac10b-58cc-4372-a567-0e02b2c3d479.png"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got := blobkeys.BuildFileBlobKey(tc.tenant, tc.fileID, tc.ext)
			c.Assert(got, qt.Equals, tc.expected)
			c.Assert(strings.HasPrefix(got, "t/"+tc.tenant+"/"), qt.IsTrue,
				qt.Commentf("key must carry tenant prefix"))
		})
	}
}

func TestBuildFileUploadKey(t *testing.T) {
	c := qt.New(t)
	got := blobkeys.BuildFileUploadKey("tenant-a", "my-photo-1748000000.jpg")
	c.Assert(got, qt.Equals, "t/tenant-a/files/my-photo-1748000000.jpg")
}

func TestBuildThumbnailBlobKey(t *testing.T) {
	tests := []struct {
		name     string
		tenant   string
		fileID   string
		size     string
		expected string
	}{
		{"small thumb", "tenant-a", "file-1", "small", "t/tenant-a/thumbnails/file-1_small.jpg"},
		{"medium thumb", "tenant-a", "file-2", "medium", "t/tenant-a/thumbnails/file-2_medium.jpg"},
		{"uuid file id", "tenant-a", "f47ac10b-58cc-4372-a567-0e02b2c3d479", "small",
			"t/tenant-a/thumbnails/f47ac10b-58cc-4372-a567-0e02b2c3d479_small.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got := blobkeys.BuildThumbnailBlobKey(tc.tenant, tc.fileID, tc.size)
			c.Assert(got, qt.Equals, tc.expected)
			c.Assert(strings.HasSuffix(got, ".jpg"), qt.IsTrue,
				qt.Commentf("all thumbnails are JPEG"))
		})
	}
}

func TestBuildExportBlobKey(t *testing.T) {
	c := qt.New(t)
	got := blobkeys.BuildExportBlobKey("tenant-a", "full_database", "20260523_120000")
	c.Assert(got, qt.Equals, "t/tenant-a/exports/export_full_database_20260523_120000.xml")
}

func TestBuildExportBlobKey_LowercasesType(t *testing.T) {
	c := qt.New(t)
	got := blobkeys.BuildExportBlobKey("tenant-a", "COMMODITIES", "20260523_120000")
	c.Assert(got, qt.Equals, "t/tenant-a/exports/export_commodities_20260523_120000.xml")
}

func TestBuildRestoreUploadKey(t *testing.T) {
	c := qt.New(t)
	got := blobkeys.BuildRestoreUploadKey("tenant-a", "backup-1748000000.xml")
	c.Assert(got, qt.Equals, "t/tenant-a/restores/backup-1748000000.xml")
}

func TestBuildSeedKey(t *testing.T) {
	c := qt.New(t)
	got := blobkeys.BuildSeedKey("tenant-a", "seed-12345.jpg")
	c.Assert(got, qt.Equals, "t/tenant-a/seed-12345.jpg")
}

func TestHasTenantPrefix(t *testing.T) {
	tests := []struct {
		key      string
		expected bool
	}{
		{"t/tenant-a/files/x.pdf", true},
		{"t/tenant-a/thumbnails/x_small.jpg", true},
		{"files/x.pdf", false},
		{"thumbnails/x_small.jpg", false},
		{"exports/export_full_database_20260523_120000.xml", false},
		{"", false},
		{"seed-12345.jpg", false},
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(blobkeys.HasTenantPrefix(tc.key), qt.Equals, tc.expected)
		})
	}
}

func TestRewriteForTenant_Idempotent(t *testing.T) {
	c := qt.New(t)
	already := "t/tenant-a/files/file-1.pdf"
	c.Assert(blobkeys.RewriteForTenant(already, "tenant-b"), qt.Equals, already)
}

func TestRewriteForTenant_LegacyKeys(t *testing.T) {
	tests := []struct {
		name     string
		legacy   string
		tenant   string
		expected string
	}{
		{"legacy export", "exports/export_full_database_20260523.xml", "tenant-a",
			"t/tenant-a/exports/export_full_database_20260523.xml"},
		{"legacy thumbnail", "thumbnails/file-1_small.jpg", "tenant-a",
			"t/tenant-a/thumbnails/file-1_small.jpg"},
		{"legacy files prefix", "files/x.pdf", "tenant-a",
			"t/tenant-a/files/x.pdf"},
		{"legacy restore", "restores/backup.xml", "tenant-a",
			"t/tenant-a/restores/backup.xml"},
		{"legacy upload (filekit name)", "my-receipt-1748000000.pdf", "tenant-a",
			"t/tenant-a/files/my-receipt-1748000000.pdf"},
		{"legacy seed fixture", "seed-12345.jpg", "tenant-a",
			"t/tenant-a/files/seed-12345.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(blobkeys.RewriteForTenant(tc.legacy, tc.tenant), qt.Equals, tc.expected)
		})
	}
}

func TestRewriteForTenant_EmptyInputs(t *testing.T) {
	c := qt.New(t)
	c.Assert(blobkeys.RewriteForTenant("", "tenant-a"), qt.Equals, "")
	c.Assert(blobkeys.RewriteForTenant("legacy.pdf", ""), qt.Equals, "legacy.pdf")
}

func TestKeysAlwaysCarryTenantNamespace(t *testing.T) {
	// Structural invariant: no helper may emit a key that escapes the
	// tenant's namespace, however absurd the inputs. This is the core
	// defence-in-depth property issue #1793 asks for.
	c := qt.New(t)
	tenant := "tenant-a"
	prefix := blobkeys.TenantPrefix(tenant)

	c.Assert(strings.HasPrefix(blobkeys.BuildFileBlobKey(tenant, "../../escape", ".pdf"), prefix), qt.IsTrue)
	c.Assert(strings.HasPrefix(blobkeys.BuildFileUploadKey(tenant, "../../escape.pdf"), prefix), qt.IsTrue)
	c.Assert(strings.HasPrefix(blobkeys.BuildThumbnailBlobKey(tenant, "../../escape", "small"), prefix), qt.IsTrue)
	c.Assert(strings.HasPrefix(blobkeys.BuildExportBlobKey(tenant, "any", "ts"), prefix), qt.IsTrue)
	c.Assert(strings.HasPrefix(blobkeys.BuildRestoreUploadKey(tenant, "../escape.xml"), prefix), qt.IsTrue)
	c.Assert(strings.HasPrefix(blobkeys.BuildSeedKey(tenant, "anything"), prefix), qt.IsTrue)
}
