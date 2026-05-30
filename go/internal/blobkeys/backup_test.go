package blobkeys_test

import (
	"strings"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/blobkeys"
)

func TestBuildBackupBlobKey(t *testing.T) {
	tests := []struct {
		name       string
		tenant     string
		exportType string
		timestamp  string
		expected   string
	}{
		{"full database", "tenant-a", "full_database", "20060102_150405",
			"t/tenant-a/exports/backup_full_database_20060102_150405.inb"},
		{"lowercases type", "tenant-a", "FullDatabase", "20060102_150405",
			"t/tenant-a/exports/backup_fulldatabase_20060102_150405.inb"},
		{"locations", "tenant-b", "locations", "20240101_000000",
			"t/tenant-b/exports/backup_locations_20240101_000000.inb"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got := blobkeys.BuildBackupBlobKey(tc.tenant, tc.exportType, tc.timestamp)
			c.Assert(got, qt.Equals, tc.expected)
			c.Assert(strings.HasPrefix(got, "t/"+tc.tenant+"/"), qt.IsTrue,
				qt.Commentf("backup key must carry tenant prefix"))
			c.Assert(strings.HasSuffix(got, ".inb"), qt.IsTrue,
				qt.Commentf("backup key must have .inb extension"))
		})
	}
}

func TestSanitizeArchivePath_Safe(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"empty", "", ""},
		{"manifest", "manifest.json", "manifest.json"},
		{"location json", "location-home-uuid.json", "location-home-uuid.json"},
		{"files member preserves slashes", "files/home/abc/images/photo.jpg", "files/home/abc/images/photo.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(blobkeys.SanitizeArchivePath(tc.input), qt.Equals, tc.expected)
		})
	}
}

func TestSanitizeArchivePath_Neutralizes(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"parent traversal", "../../etc/passwd"},
		{"embedded traversal", "files/../../../etc/passwd"},
		{"absolute path", "/etc/passwd"},
		{"backslash windows", "..\\..\\windows\\system32"},
		{"nul byte", "files/photo\x00.jpg"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			got := blobkeys.SanitizeArchivePath(tc.input)
			// The sanitized form must never contain a `..` segment, a
			// leading slash, a backslash, or a NUL byte.
			c.Assert(got, qt.Not(qt.Contains), "..",
				qt.Commentf("sanitized path must not retain a traversal token: %q", got))
			c.Assert(strings.HasPrefix(got, "/"), qt.IsFalse,
				qt.Commentf("sanitized path must not be absolute: %q", got))
			c.Assert(got, qt.Not(qt.Contains), "\\",
				qt.Commentf("sanitized path must not retain a backslash: %q", got))
			c.Assert(got, qt.Not(qt.Contains), "\x00",
				qt.Commentf("sanitized path must not retain a NUL byte: %q", got))
			// The sanitized form must differ from the raw form so the
			// restore caller can detect and reject the hostile member.
			c.Assert(got, qt.Not(qt.Equals), tc.input)
		})
	}
}
