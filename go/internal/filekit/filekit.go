package filekit

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/internal/textutils"
)

var NowFunc = time.Now

// UploadFileName generates a sanitized and unique file name with a timestamp for the given file name input.
// The generated file name is in the format: [sanitized_file_name]-[timestamp].[file_extension]
// The timestamp is obtained from the NowFunc function, which can be mocked for testing purposes.
// The file name is sanitized by converting it to lowercase and replacing spaces with dashes.
// If the file name is empty, it defaults to "h" (hidden).
// The function properly handles multi-part extensions (like .tar.gz) using getMultiPartExtension.
// The function returns the generated file name.
func UploadFileName(fileName string) string {
	fileExt := getMultiPartExtension(fileName)
	originalFileName := strings.TrimSuffix(
		filepath.Base(fileName),
		fileExt,
	)
	if originalFileName == "" {
		originalFileName = "h"
	}
	now := NowFunc()

	var buf strings.Builder

	buf.WriteString(textutils.CleanFilename(originalFileName))
	buf.WriteRune('-')
	fmt.Fprintf(&buf, "%v", now.Unix())
	buf.WriteString(fileExt)

	return buf.String()
}

func getMultiPartExtension(filePath string) string {
	ext := filepath.Ext(filePath)                 // Get the last element of the path
	filename := strings.TrimSuffix(filePath, ext) // Remove the extension from the filename
	multiPartExt := filepath.Ext(filename) + ext  // Combine the extension with the remaining filename
	return multiPartExt
}
