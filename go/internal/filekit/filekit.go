package filekit

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/internal/textutils"
)

var NowFunc = time.Now

// UploadFileName generates a sanitized, HUMAN-READABLE file name with a
// timestamp for the given file name input, in the format
// [sanitized_file_name]-[timestamp].[file_extension].
//
// It is NOT unique and MUST NOT be used to derive a blob key. The timestamp has
// SECOND granularity and there is no randomness, so two uploads of the same
// filename within one second produce the identical string — which is precisely
// how #2241 turned a file delete into the destruction of another live file's
// bytes. Blob keys are minted from a server-side UUID via
// blobkeys.BuildFileBlobKey; this name exists so the user sees "invoice-…"
// rather than a UUID as the title of their upload.
//
// The timestamp is obtained from the NowFunc function, which can be mocked for
// testing purposes. The file name is sanitized by converting it to lowercase and
// replacing spaces with dashes. If the file name is empty, it defaults to "h"
// (hidden). Multi-part extensions (like .tar.gz) are handled via Extension.
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

// Extension returns the file extension of filePath INCLUDING the leading dot,
// handling multi-part extensions (".tar.gz") the same way UploadFileName does.
// Returns "" for a name with no extension.
//
// Exported so the upload handler can give a UUID-minted blob key the same
// extension the human-readable name carries — the key's extension is cosmetic
// (readers open the key verbatim and take the MIME type from the row), but a
// bucket full of extensionless UUIDs is miserable to operate.
func Extension(filePath string) string {
	return getMultiPartExtension(filePath)
}

func getMultiPartExtension(filePath string) string {
	ext := filepath.Ext(filePath)                 // Get the last element of the path
	filename := strings.TrimSuffix(filePath, ext) // Remove the extension from the filename
	multiPartExt := filepath.Ext(filename) + ext  // Combine the extension with the remaining filename
	return multiPartExt
}
