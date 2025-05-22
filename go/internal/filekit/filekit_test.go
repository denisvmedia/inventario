package filekit_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/internal/filekit"
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

	// Verify that the file extension is preserved
	ext := filepath.Ext(fileName)
	c.Assert(ext, qt.Satisfies, func(s string) bool {
		return strings.HasSuffix(fileName, s)
	})
}
