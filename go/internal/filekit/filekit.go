package filekit

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"go.5x5.cz/inventario/internal/textutils"
)

var NowFunc = time.Now

// displayNameMaxRunes is the longest title the file API accepts, in
// characters.
const displayNameMaxRunes = 255

// DisplayName is the name an uploaded file is shown under, its Title and its
// Path: the name the client sent without its last extension, as the upload
// dialogs pre-fill the title ("Invoice 121.pdf" gives "Invoice 121"). A dot
// that starts the name does not start an extension, so ".env" keeps its name.
// Bytes that are not UTF-8 and control characters are dropped, since a text
// column cannot store some of them, surrounding spaces are trimmed, and the
// result is cut to 255 characters, the longest title the file API accepts.
// It returns "" when nothing is left.
func DisplayName(fileName string) string {
	name := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(fileName, ""))
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		name = name[:i]
	}
	name = strings.TrimSpace(name)
	if runes := []rune(name); len(runes) > displayNameMaxRunes {
		name = strings.TrimSpace(string(runes[:displayNameMaxRunes]))
	}
	return name
}

// UploadFileName generates a sanitized file name with a timestamp for the
// given file name input, in the format
// [sanitized_file_name]-[timestamp].[file_extension]. It names the blob of a
// backup uploaded for a restore (blobkeys.BuildRestoreUploadKey) and gives the
// blob key of an uploaded file its extension. The title of an uploaded file
// comes from DisplayName, without the timestamp.
//
// It is NOT unique and MUST NOT be used alone to derive a blob key. The
// timestamp has SECOND granularity and there is no randomness, so two uploads
// of the same filename within one second produce the identical string — which
// is precisely how #2241 turned a file delete into the destruction of another
// live file's bytes. Blob keys are minted from a server-side UUID via
// blobkeys.BuildFileBlobKey.
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

// DownloadName is the filename the user gets on disk: the human-readable Path
// with its extension, without doubling one that is already there.
//
// `files.path` is NOMINALLY the name without its extension, but nothing enforces
// that and the API accepts one that carries it ("receipt.pdf" round-trips
// through the update validator). Concatenating blindly produced
// `Content-Disposition: filename="receipt.pdf.pdf"` — and since the header takes
// priority over the browser's `download` attribute (RFC 6266), no amount of
// fixing the frontend could change what was actually saved (#2250).
//
// Case-insensitive: a user who typed "Receipt.PDF" gets one extension, not two.
func DownloadName(path, ext string) string {
	if ext == "" {
		return path
	}
	if strings.HasSuffix(strings.ToLower(path), strings.ToLower(ext)) {
		return path
	}
	return path + ext
}

// compoundExtensionStems are the first halves of the two-part extensions worth
// keeping together. Anything else that looks like one — `report.v2.pdf`,
// `invoice.2024.pdf` — is a dot in the NAME, and swallowing it moved half the
// filename into the extension and out of the title the user sees (#2131).
var compoundExtensionStems = map[string]bool{
	".tar": true,
}

func getMultiPartExtension(filePath string) string {
	ext := filepath.Ext(filePath)                 // Get the last element of the path
	filename := strings.TrimSuffix(filePath, ext) // Remove the extension from the filename
	stem := filepath.Ext(filename)
	if compoundExtensionStems[strings.ToLower(stem)] {
		return stem + ext
	}
	return ext
}

// driveLetterPrefix reports whether s starts with a Windows drive letter, as
// in `D:` — the shape that tells a Windows path from a POSIX one.
func driveLetterPrefix(s string) bool {
	if len(s) < 2 || s[1] != ':' {
		return false
	}
	c := s[0]
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// NormalizeFileURL rewrites a file:// URL that names a Windows path so it
// parses, and parses as the path that was meant.
//
// `file://D:\Work\uploads` does not parse at all: url.Parse reads `D` as the
// host and `:\Work\uploads` as its port, and reports `invalid port`. Repairing
// only the separators is not enough either — `file://D:/Work/uploads` parses,
// but with `D:` as the *host* and `/Work/uploads` as the path, so the drive is
// silently dropped. Both want the authority left empty and the drive inside
// the path: `file:///D:/Work/uploads`.
//
// Only a path that begins with a drive letter is touched. A POSIX directory
// name may legitimately contain a backslash, and this must not rewrite it.
func NormalizeFileURL(raw string) string {
	rest, ok := strings.CutPrefix(raw, "file://")
	if !ok {
		return raw
	}

	path, query, hasQuery := strings.Cut(rest, "?")
	if !driveLetterPrefix(strings.TrimPrefix(path, "/")) {
		return raw
	}

	normalized := "file:///" + strings.ReplaceAll(strings.TrimPrefix(path, "/"), `\`, "/")
	if hasQuery {
		normalized += "?" + query
	}
	return normalized
}
