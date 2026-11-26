package inb

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"io"

	"github.com/go-extras/errx"
)

// PersonalDataExportKind is the manifest `kind` of a personal data export
// (#2177): a user's copy of their own data. It is a gzip-compressed tar like
// the payload of a backup, with manifest.json as its first member, but it is
// not a backup and is never restored.
const PersonalDataExportKind = "personal_data_export"

// PersonalDataExportRestoreMessage is what a user who tries to restore a
// personal data export is told.
const PersonalDataExportRestoreMessage = "This file is a personal data export, a copy of one person's data. " +
	"It cannot be restored. To restore a group, upload a group backup (.inb) instead."

// ErrPersonalDataExport is what reading a personal data export as a backup
// fails with.
var ErrPersonalDataExport = errx.NewSentinel("a personal data export cannot be restored")

// SniffHeadSize is how many leading bytes IsPersonalDataExport needs: the
// compressed tar header and manifest of a personal data export fit well
// within it.
const SniffHeadSize = 64 << 10

// maxSniffedManifestBytes caps the manifest bytes IsPersonalDataExport reads.
const maxSniffedManifestBytes = 1 << 20

// IsPersonalDataExport reports whether head, the first bytes of a file, is
// the start of a personal data export: gzip, whose tar's first member is a
// manifest.json with the personal data export kind. A head cut short inside
// the manifest reads as not one.
func IsPersonalDataExport(head []byte) bool {
	if len(head) < 2 || head[0] != 0x1f || head[1] != 0x8b {
		return false
	}
	gz, err := gzip.NewReader(bytes.NewReader(head))
	if err != nil {
		return false
	}
	tr := tar.NewReader(gz)
	h, err := tr.Next()
	if err != nil || h.Name != "manifest.json" || h.Typeflag != tar.TypeReg {
		return false
	}
	var manifest struct {
		Kind string `json:"kind"`
	}
	if err := json.NewDecoder(io.LimitReader(tr, maxSniffedManifestBytes)).Decode(&manifest); err != nil {
		return false
	}
	return manifest.Kind == PersonalDataExportKind
}
