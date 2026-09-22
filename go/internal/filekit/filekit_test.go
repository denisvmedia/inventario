package filekit_test

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/internal/filekit"
)

func TestUploadFileName(t *testing.T) {
	c := qt.New(t)

	// Set a fixed time for testing
	fixedTime := time.Date(2023, 6, 25, 12, 0, 0, 0, time.UTC)
	filekit.NowFunc = func() time.Time {
		return fixedTime
	}
	defer func() {
		// Reset NowFunc after the test
		filekit.NowFunc = time.Now
	}()

	// Test case 1: Simple file name
	fileName1 := "example.txt"
	expected1 := "example-1687694400.txt"
	c.Assert(filekit.UploadFileName(fileName1), qt.Equals, expected1)

	// Test case 2: File name with spaces
	fileName2 := "my document.pdf"
	expected2 := "my-document-1687694400.pdf"
	c.Assert(filekit.UploadFileName(fileName2), qt.Equals, expected2)

	// Test case 3: File name with uppercase letters
	fileName3 := "ImportantFile.TXT"
	expected3 := "importantfile-1687694400.TXT"
	c.Assert(filekit.UploadFileName(fileName3), qt.Equals, expected3)

	// Test case 4: File name with multiple dots
	fileName4 := "archive.tar.gz"
	expected4 := "archive-1687694400.tar.gz"
	c.Assert(filekit.UploadFileName(fileName4), qt.Equals, expected4)

	// Test case 5: File name with leading dot
	fileName5 := ".hidden-file"
	expected5 := "h-1687694400.hidden-file"
	c.Assert(filekit.UploadFileName(fileName5), qt.Equals, expected5)
}

func TestUploadFileNameWithCurrentTime(t *testing.T) {
	c := qt.New(t)

	// Call UploadFileName without setting NowFunc (use current time)
	fileName := "example.txt"
	result := filekit.UploadFileName(fileName)

	// Verify that the result is as expected
	c.Assert(result, qt.Contains, "example-")

	// Verify that the result includes the current timestamp
	now := time.Now().Unix()
	c.Assert(result, qt.Contains, fmt.Sprintf("%v", now))

	// Verify that the generated name ends with the original extension
	ext := filepath.Ext(fileName)
	c.Assert(filepath.Ext(result), qt.Equals, ext)
}

// The download filename is what the user gets on disk. `files.path` is nominally
// the name WITHOUT its extension, but the API accepts one that carries it, and
// concatenating blindly produced `receipt.pdf.pdf`.
//
// This is the server-side half, and it is the half that decides: the
// Content-Disposition header takes priority over the browser's `download`
// attribute (RFC 6266), so fixing only the frontend changed nothing a user could
// see (#2250).
func TestDownloadName(t *testing.T) {
	tests := []struct {
		name string
		path string
		ext  string
		want string
	}{
		{"appends a missing extension", "receipt", ".pdf", "receipt.pdf"},
		{"does not double an extension already present", "receipt.pdf", ".pdf", "receipt.pdf"},
		{"matches case-insensitively", "Receipt.PDF", ".pdf", "Receipt.PDF"},
		{"a different extension is appended, not swallowed", "archive.tar", ".gz", "archive.tar.gz"},
		{"multi-part extension already present", "archive.tar.gz", ".tar.gz", "archive.tar.gz"},
		{"no extension to add", "receipt", "", "receipt"},
		{"a name that merely contains the ext is not confused", "pdf-notes", ".pdf", "pdf-notes.pdf"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)
			c.Assert(filekit.DownloadName(tt.path, tt.ext), qt.Equals, tt.want)
		})
	}
}

// #2131: a dot in the NAME is not an extension. `report.v2.pdf` used to yield
// extension ".v2.pdf", which moved half the filename out of the title the
// user sees and produced a nonsense extension. Only the compound extensions
// that really are two parts stay joined.
func TestUploadFileName_DotsInTheName(t *testing.T) {
	c := qt.New(t)
	filekit.NowFunc = func() time.Time { return time.Unix(1700000000, 0) }
	t.Cleanup(func() { filekit.NowFunc = time.Now })

	tests := []struct {
		in   string
		want string
	}{
		{"report.v2.pdf", "report_v2-1700000000.pdf"},
		{"invoice.2024.pdf", "invoice_2024-1700000000.pdf"},
		{"archive.tar.gz", "archive-1700000000.tar.gz"},
		{"ARCHIVE.TAR.GZ", "archive-1700000000.TAR.GZ"},
		{"plain.pdf", "plain-1700000000.pdf"},
	}
	for _, tc := range tests {
		c.Run(tc.in, func(c *qt.C) {
			c.Assert(filekit.UploadFileName(tc.in), qt.Equals, tc.want)
		})
	}
}

// #251: `--upload-location file://D:\Work\inventario/uploads?create_dir=1`,
// which is what the Makefile builds on Windows, does not parse: url.Parse
// reads `D` as the host and `:\Work\inventario` as its port.
func TestNormalizeFileURL(t *testing.T) {
	c := qt.New(t)

	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			name: "windows path with backslashes, as the Makefile writes it",
			in:   `file://D:\Work\inventario/uploads?create_dir=1`,
			want: "file:///D:/Work/inventario/uploads?create_dir=1",
		},
		{
			// Parses, but with D: as the host, so the drive is dropped.
			name: "windows path already using forward slashes",
			in:   "file://D:/Work/inventario/uploads?create_dir=1",
			want: "file:///D:/Work/inventario/uploads?create_dir=1",
		},
		{
			name: "windows path already in the right shape",
			in:   "file:///D:/Work/inventario/uploads?create_dir=1",
			want: "file:///D:/Work/inventario/uploads?create_dir=1",
		},
		{
			name: "lower-case drive letter",
			in:   `file://c:\uploads`,
			want: "file:///c:/uploads",
		},
		{
			name: "posix absolute path is untouched",
			in:   "file:///srv/uploads?create_dir=1",
			want: "file:///srv/uploads?create_dir=1",
		},
		{
			name: "posix relative path is untouched",
			in:   "file://./uploads?create_dir=1",
			want: "file://./uploads?create_dir=1",
		},
		{
			// A POSIX directory may legitimately contain a backslash.
			name: "backslash in a posix path is not a separator",
			in:   `file:///srv/odd\name/uploads`,
			want: `file:///srv/odd\name/uploads`,
		},
		{
			name: "another scheme is untouched",
			in:   "s3://my-bucket?region=eu-central-1",
			want: "s3://my-bucket?region=eu-central-1",
		},
		{
			name: "empty",
			in:   "",
			want: "",
		},
	} {
		c.Run(tc.name, func(c *qt.C) {
			got := filekit.NormalizeFileURL(tc.in)
			c.Assert(got, qt.Equals, tc.want)

			// Whatever comes out has to parse, and has to keep the drive in
			// the path rather than in the authority.
			u, err := url.Parse(got)
			if tc.in == "" {
				return
			}
			c.Assert(err, qt.IsNil, qt.Commentf("%q does not parse", got))
			if strings.Contains(tc.in, ":\\") || strings.Contains(tc.in, "D:/") || strings.Contains(tc.in, "c:") {
				c.Check(u.Host, qt.Equals, "", qt.Commentf("drive letter leaked into the authority of %q", got))
				c.Check(strings.HasPrefix(u.Path, "/D:/") || strings.HasPrefix(u.Path, "/c:/"), qt.IsTrue,
					qt.Commentf("path is %q", u.Path))
			}
		})
	}
}
