package assets_test

import (
	"io/fs"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/assets"
)

func TestGetPlaceholderFile_Success(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "small placeholder exists",
			filename: "generating_small.gif",
		},
		{
			name:     "medium placeholder exists",
			filename: "generating_medium.gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			data, err := assets.GetPlaceholderFile(tt.filename)

			c.Assert(err, qt.IsNil)
			c.Assert(data, qt.IsNotNil)
			c.Assert(len(data), qt.Not(qt.Equals), 0)

			// Check that it's a GIF file (starts with GIF header)
			c.Assert(len(data) >= 6, qt.IsTrue)
			c.Assert(string(data[:6]), qt.Matches, "GIF8[79]a")
		})
	}
}

func TestGetPlaceholderFile_Error(t *testing.T) {
	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "non-existent file",
			filename: "non_existent.gif",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			data, err := assets.GetPlaceholderFile(tt.filename)

			c.Assert(err, qt.IsNotNil)
			c.Assert(data, qt.IsNil)
		})
	}
}

func TestGetPlaceholders(t *testing.T) {
	c := qt.New(t)

	placeholderFS := assets.GetPlaceholders()
	c.Assert(placeholderFS, qt.IsNotNil)

	// Test that we can read files from the filesystem
	entries, err := fs.ReadDir(placeholderFS, "placeholders")
	c.Assert(err, qt.IsNil)
	c.Assert(len(entries), qt.Not(qt.Equals), 0)
}

func TestGetPlaceholders_ExpectedFilesExist(t *testing.T) {
	expectedFiles := []struct {
		name     string
		filename string
	}{
		{
			name:     "small placeholder exists",
			filename: "generating_small.gif",
		},
		{
			name:     "medium placeholder exists",
			filename: "generating_medium.gif",
		},
	}

	for _, tt := range expectedFiles {
		t.Run(tt.name, func(t *testing.T) {
			c := qt.New(t)

			placeholderFS := assets.GetPlaceholders()
			c.Assert(placeholderFS, qt.IsNotNil)

			// Check that the specific file exists
			_, err := fs.Stat(placeholderFS, "placeholders/"+tt.filename)
			c.Assert(err, qt.IsNil, qt.Commentf("%s not found", tt.filename))
		})
	}
}
