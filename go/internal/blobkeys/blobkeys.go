// Package blobkeys is the single source of truth for the layout of blob
// storage keys inside the upload bucket.
//
// Every blob key written by the application MUST be produced by one of
// the BuildXxx helpers below. The keys are tenant-prefixed so that a row
// in tenant A physically cannot reference a blob owned by tenant B —
// defence-in-depth on top of the row-level RLS that already prevents
// cross-tenant SELECT/UPDATE. See issue #1793 for the threat model and
// PR #1810 / #1823 for the predecessor work this hardens.
//
// Layout:
//
//	t/<tenant>/files/<blob-id><ext>             — original uploaded file
//	t/<tenant>/thumbnails/<file-id>_<size>.jpg  — derived thumbnail (JPEG)
//	t/<tenant>/exports/export_<type>_<ts>.xml   — exported XML bundle
//	t/<tenant>/restores/<blob-id>-<filename>    — uploaded archive for restore
//	t/<tenant>/seed-<uuid><ext>                 — demo / seed fixtures
//
// Every <blob-id> is a server-minted UUID. That is a correctness
// requirement, not a style choice: the key is the DELETE key, nothing
// enforces one row per key, and `files` has no soft-delete — so a key
// that two rows can share is a key whose deletion destroys a live file's
// bytes. See #2241, where an upload key of `<name>-<unix SECONDS><ext>`
// did exactly that.
//
// The tenant is always supplied by server-side context (the authenticated
// user's TenantID, or — for cross-tenant background workers — the tenant
// the operation is performing work on). A caller MUST NOT plumb a tenant
// value derived from request input through these helpers.
//
// Backwards compatibility: HasTenantPrefix detects legacy flat keys
// (those that pre-date this package) so the migration backfill can decide
// whether to rewrite a row. Readers are intentionally NOT tenant-aware —
// they take whatever string the database hands them and ask the bucket
// for it. That keeps the legacy → prefixed transition forward-only and
// idempotent: a row that was already rewritten reads itself unchanged;
// a row that was not stays readable at its legacy key until backfill
// touches it.
package blobkeys

import (
	"fmt"
	"strings"
)

// Prefix is the per-tenant namespace root inside the bucket. Stable —
// changing it would invalidate every existing key in deployed buckets.
const Prefix = "t/"

// FilesSegment is the per-tenant subfolder for original uploaded files.
const FilesSegment = "files"

// ThumbnailsSegment is the per-tenant subfolder for derived thumbnails.
const ThumbnailsSegment = "thumbnails"

// ExportsSegment is the per-tenant subfolder for generated export XML
// bundles.
const ExportsSegment = "exports"

// RestoresSegment is the per-tenant subfolder for uploaded restore XML
// files.
const RestoresSegment = "restores"

// TenantPrefix returns the bucket-relative root for a single tenant —
// `t/<tenant>/`. Callers that need to enumerate or sweep blobs for a
// tenant use this with the bucket's List operation.
func TenantPrefix(tenantID string) string {
	if tenantID == "" {
		return ""
	}
	return Prefix + tenantID + "/"
}

// sanitizeSegment defence-in-depth-normalises a single per-call
// basename or identifier so the resulting blob key cannot escape the
// tenant namespace under any backend's traversal rules. Replaces
// `/`, `\`, and the `..` traversal token with `_`.
//
// Callers normally feed already-sanitized strings (`filekit.UploadFileName`,
// `textutils.CleanFilename`, server-minted UUIDs); this is the structural
// safety net so a future caller that bypasses upstream sanitisation
// cannot punch a hole in the tenant namespace through these helpers.
func sanitizeSegment(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}

// BuildFileBlobKey produces the canonical blob key for an original file
// upload: `t/<tenant>/files/<blob-id><ext>`. The extension argument is
// expected to start with a dot (".pdf", ".jpg"); an empty string is
// allowed for files whose MIME type carries no canonical extension.
//
// blobID MUST be a server-minted UUID and MUST NOT be derived from the
// user-supplied filename. Two properties depend on it, and #2241 is what
// happens when either is lost:
//
//   - UNIQUENESS. The key is the delete key. Nothing enforces one row per
//     key (there is no unique index on files.original_path), so a key two
//     rows can share is a key whose deletion destroys a live file's bytes.
//     The old upload key was `<sanitized-name>-<unix SECONDS><ext>`, which
//     two same-named uploads in one second collide on exactly.
//   - UNGUESSABILITY. The key must not be derivable from a Path the user
//     chooses, or from a filename they control.
//
// Callers that already have the FileEntity row ID (the restore importer)
// pass that; the upload handler mints a fresh UUID because the row does
// not exist yet. Both are unique and unguessable, which is all this key
// needs — it is not required to equal the row id, and does not.
func BuildFileBlobKey(tenantID, blobID, ext string) string {
	return fmt.Sprintf("%s%s/%s/%s%s",
		Prefix, tenantID, FilesSegment,
		sanitizeSegment(blobID), sanitizeSegment(ext),
	)
}

// BuildThumbnailBlobKey produces the canonical blob key for a derived
// thumbnail: `t/<tenant>/thumbnails/<file-id>_<size>.jpg`. All
// thumbnails are JPEG regardless of the source format — the file
// service re-encodes during generation.
func BuildThumbnailBlobKey(tenantID, fileID, size string) string {
	return fmt.Sprintf("%s%s/%s/%s_%s.jpg",
		Prefix, tenantID, ThumbnailsSegment,
		sanitizeSegment(fileID), sanitizeSegment(size),
	)
}

// BuildExportBlobKey produces the canonical blob key for a generated
// export bundle: `t/<tenant>/exports/export_<type>_<timestamp>.xml`.
// `exportType` is lowercased so the resulting key is stable across the
// casing used by callers.
func BuildExportBlobKey(tenantID, exportType, timestamp string) string {
	return fmt.Sprintf("%s%s/%s/export_%s_%s.xml",
		Prefix, tenantID, ExportsSegment,
		sanitizeSegment(strings.ToLower(exportType)),
		sanitizeSegment(timestamp),
	)
}

// BuildBackupBlobKey produces the canonical blob key for a generated
// signed `.inb` backup archive:
// `t/<tenant>/exports/backup_<type>_<timestamp>.inb` (issue #534).
//
// It mirrors BuildExportBlobKey's layout (same `exports/` subfolder,
// same lowercase-type + timestamp shape) but uses the `backup_` prefix
// and `.inb` extension so the new signed archives sit alongside — and
// are distinguishable from — the legacy XML bundles.
func BuildBackupBlobKey(tenantID, exportType, timestamp string) string {
	return fmt.Sprintf("%s%s/%s/backup_%s_%s.inb",
		Prefix, tenantID, ExportsSegment,
		sanitizeSegment(strings.ToLower(exportType)),
		sanitizeSegment(timestamp),
	)
}

// SanitizeArchivePath neutralises a tar member name read out of an `.inb`
// inner archive so it cannot escape its intended namespace on any backend
// (issue #534). The `.inb` inner tar carries metadata members
// (`manifest.json`, `location-<slug>-<uuid>.json`) and file members
// (`files/<loc>/<commodity>/<bucket>/<name>`); a hostile archive could
// instead carry `../../etc/passwd`, an absolute `/etc/passwd`, a
// backslash-segmented Windows path, or an embedded NUL.
//
// This function is the structural safety net the restore path validates
// every member name through BEFORE it is used for any blob key or
// lookup. It:
//
//   - strips a leading `/` (absolute path),
//   - replaces backslashes with `/` (Windows separators),
//   - drops embedded NUL bytes,
//   - replaces every `..` path segment with `_` so traversal is
//     impossible, while
//   - preserving the forward slashes that legitimately segment
//     `files/<...>` member names.
//
// Note that the restore path never uses the returned value as a blob key
// directly — file bytes are always re-keyed under the importing tenant's
// namespace via BuildFileBlobKey. SanitizeArchivePath is defence-in-depth
// for the lookup/identity side (matching a file member to its expected
// metadata) and a loud signal: a member whose sanitised form differs from
// its raw form is rejected by the caller rather than silently rewritten.
func SanitizeArchivePath(p string) string {
	if p == "" {
		return p
	}
	// Drop NUL bytes outright.
	p = strings.ReplaceAll(p, "\x00", "")
	// Normalise Windows separators to forward slashes.
	p = strings.ReplaceAll(p, "\\", "/")
	// Strip any leading slashes (absolute path → relative).
	p = strings.TrimLeft(p, "/")
	// Neutralise `..` traversal in every segment while keeping the
	// segmenting slashes intact.
	segments := strings.Split(p, "/")
	for i, seg := range segments {
		if seg == ".." {
			segments[i] = "_"
		}
	}
	return strings.Join(segments, "/")
}

// BuildRestoreUploadKey produces the canonical blob key for a backup
// archive uploaded as part of a restore operation:
// `t/<tenant>/restores/<blob-id>-<filename>`. `filename` is expected to
// already be sanitized (the upload pipeline runs every name through
// filekit.UploadFileName before reaching us) and is kept only so an
// operator staring at the bucket can tell what a key is.
//
// blobID is a server-minted UUID and carries the uniqueness (#2241). A
// restore blob has NO owning row of any kind (#2121) — it stays rowless
// until the user submits the import, and forever if they never do — so a
// name-only key means two uploads of `backup.inb` in the same second
// silently OVERWRITE each other, and a pending import then restores bytes
// its user never uploaded.
func BuildRestoreUploadKey(tenantID, blobID, filename string) string {
	return fmt.Sprintf("%s%s/%s/%s-%s",
		Prefix, tenantID, RestoresSegment,
		sanitizeSegment(blobID), sanitizeSegment(filename),
	)
}

// BuildSeedKey produces the canonical blob key for a demo / seed
// fixture: `t/<tenant>/seed-<uuid><ext>`. The seed package owns the
// uuid+ext combination — this helper just sticks the tenant prefix in
// front so the seed corpus is namespaced the same as everything else.
//
// `seedBasename` is the seed-internal name (e.g. `seed-<uuid>.jpg`)
// that the noopUploader / bucketUploader generates.
func BuildSeedKey(tenantID, seedBasename string) string {
	return fmt.Sprintf("%s%s/%s", Prefix, tenantID, sanitizeSegment(seedBasename))
}

// HasTenantPrefix reports whether the given blob key already lives
// under the per-tenant namespace (i.e. it was written after this
// package shipped). Used by the backfill to skip rows that are already
// prefixed.
func HasTenantPrefix(blobKey string) bool {
	return strings.HasPrefix(blobKey, Prefix)
}

// RewriteForTenant produces the tenant-prefixed equivalent of a legacy
// flat key, used by the restore importer (XML emitted by an old export
// carries flat keys, which the importer rewrites under the importing
// tenant's namespace) and by the migration backfill (existing rows
// carry flat keys until the bucket move completes).
//
// If `legacyKey` is already prefixed (`t/...`), it is returned as-is —
// the rewrite is idempotent. Otherwise the rewrite is structural:
//
//	exports/export_<type>_<ts>.xml      → t/<tenant>/exports/...
//	thumbnails/<file-id>_<size>.jpg     → t/<tenant>/thumbnails/...
//	(anything else)                     → t/<tenant>/files/<legacyKey>
//
// The fallback bucket (`files/`) catches:
//   - uploads written by filekit.UploadFileName (`<sanitized>-<ts>.<ext>`)
//   - seed fixtures (`seed-<uuid>.<ext>`)
//   - any other legacy shape we don't know about
//
// This keeps the importer + backfill able to handle arbitrary legacy
// inputs without burning the migration over an unrecognized prefix.
func RewriteForTenant(legacyKey, tenantID string) string {
	if HasTenantPrefix(legacyKey) {
		return legacyKey
	}
	if tenantID == "" || legacyKey == "" {
		return legacyKey
	}
	switch {
	case strings.HasPrefix(legacyKey, ExportsSegment+"/"):
		return sanitizeRewritten(Prefix + tenantID + "/" + legacyKey)
	case strings.HasPrefix(legacyKey, ThumbnailsSegment+"/"):
		return sanitizeRewritten(Prefix + tenantID + "/" + legacyKey)
	case strings.HasPrefix(legacyKey, FilesSegment+"/"):
		return sanitizeRewritten(Prefix + tenantID + "/" + legacyKey)
	case strings.HasPrefix(legacyKey, RestoresSegment+"/"):
		return sanitizeRewritten(Prefix + tenantID + "/" + legacyKey)
	default:
		return sanitizeRewritten(Prefix + tenantID + "/" + FilesSegment + "/" + legacyKey)
	}
}

// sanitizeRewritten is the rewrite-side counterpart of sanitizeSegment:
// it strips traversal tokens (`..`, backslash) from a key the rewriter
// just produced while preserving the forward slashes that segment the
// canonical layout (`t/<tenant>/exports/...`). Restore imports XML keys
// that are attacker-controllable, so a malicious `originalPath` like
// `t/other/../../escape` must not become a key that resolves outside
// `t/<importer>/` on a filesystem-backed bucket.
func sanitizeRewritten(s string) string {
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	return s
}
