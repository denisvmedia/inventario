//go:build !legacy_xml_backup

package export

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/export/types"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/inb"
	"github.com/denisvmedia/inventario/models"
)

// gzipLevel is the compression level for the inner payload.tar.gz. Level 3
// trades a little ratio for much faster compression, which matters because the
// archive is built synchronously in a worker and streamed straight to blob
// storage (#534).
const gzipLevel = 3

// exportFileMeta returns the FileEntity stamping for a signed `.inb` export
// artifact: `.inb` / application/x-inventario-backup / "inb-2.0" (#534).
func exportFileMeta() exportFileMetaFields {
	return exportFileMetaFields{
		Ext:              ".inb",
		MIMEType:         "application/x-inventario-backup",
		LinkedEntityMeta: "inb-2.0",
		Tags:             models.StringSlice{"export", "inb"},
	}
}

// generateExport produces a signed `.inb` archive for the export and returns
// the blob key it was written to plus the collected statistics (#534).
//
// Memory safety: the inner payload.tar.gz is spooled to a LOCAL TEMP FILE while
// a streaming SHA-256 digest is computed via io.MultiWriter. The signature is
// the Ed25519 signature over that digest (never over a buffered payload). The
// finished container is then streamed from the temp file into blob storage. At
// no point does the whole archive (or any single file) live on the heap.
func (s *ExportService) generateExport(ctx context.Context, export models.Export) (string, *types.ExportStats, error) {
	if s.signer == nil {
		return "", nil, errx.NewSentinel("backup signer is required to generate an .inb export")
	}

	tenantID := resolveExportTenant(ctx, export)
	if tenantID == "" {
		return "", nil, errx.NewSentinel("tenant context is required to generate an export")
	}

	bucket, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer bucket.Close()

	// 1. Spool the inner payload to a temp file while digesting it.
	tmp, err := os.CreateTemp("", "inventario-inb-*.payload")
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to create temp payload file", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	stats, digest, err := s.writePayload(ctx, export, tenantID, bucket, tmp)
	if err != nil {
		return "", nil, err
	}

	// 2. Sign the streaming digest (never the buffered payload).
	sig := s.signer.SignDigest(digest)

	// 3. Rewind the temp file and stream it into the container in blob storage.
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return "", nil, errxtrace.Wrap("failed to rewind temp payload", err)
	}
	info, err := tmp.Stat()
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to stat temp payload", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	blobKey := blobkeys.BuildBackupBlobKey(tenantID, string(export.Type), timestamp)

	if err := s.writeContainerToBlob(ctx, bucket, blobKey, sig, tmp, info.Size()); err != nil {
		return "", nil, err
	}

	return blobKey, stats, nil
}

// resolveExportTenant resolves the owning tenant for an export — the export
// row's own TenantID first (set for worker-driven exports), falling back to the
// user in context (so tests driving generateExport directly still work).
func resolveExportTenant(ctx context.Context, export models.Export) string {
	if export.TenantID != "" {
		return export.TenantID
	}
	if user := appctx.UserFromContext(ctx); user != nil {
		return user.TenantID
	}
	return ""
}

// writePayload builds the inner gzip(tar) payload into w while streaming the
// bytes through a SHA-256 digest. It writes one per-location JSON member plus
// the referenced commodity file members, then the manifest, and returns the
// collected stats and the finalized digest.
func (s *ExportService) writePayload(
	ctx context.Context,
	export models.Export,
	tenantID string,
	bucket *blob.Bucket,
	w io.Writer,
) (*types.ExportStats, []byte, error) {
	digest := backupsign.NewDigest()
	gzw, err := gzip.NewWriterLevel(io.MultiWriter(w, digest), gzipLevel)
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to create gzip writer", err)
	}
	tw := tar.NewWriter(gzw)

	stats := &types.ExportStats{}
	builder := &inbBuilder{
		svc:    s,
		ctx:    ctx,
		export: export,
		tenant: tenantID,
		bucket: bucket,
		tw:     tw,
		stats:  stats,
	}

	manifestLocs, err := builder.run()
	if err != nil {
		return nil, nil, err
	}

	if err := builder.writeManifest(manifestLocs); err != nil {
		return nil, nil, err
	}

	if err := tw.Close(); err != nil {
		return nil, nil, errxtrace.Wrap("failed to close inner tar", err)
	}
	if err := gzw.Close(); err != nil {
		return nil, nil, errxtrace.Wrap("failed to close gzip writer", err)
	}

	return stats, digest.Sum(nil), nil
}

// writeContainerToBlob streams the signed outer container into blob storage.
func (s *ExportService) writeContainerToBlob(
	ctx context.Context,
	bucket *blob.Bucket,
	blobKey string,
	sig []byte,
	payload io.Reader,
	payloadSize int64,
) (err error) {
	writer, err := bucket.NewWriter(ctx, blobKey, nil)
	if err != nil {
		return errxtrace.Wrap("failed to create blob writer", err)
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil && err == nil {
			err = errxtrace.Wrap("failed to close blob writer", closeErr)
		}
	}()

	if werr := inb.WriteContainer(writer, sig, payload, payloadSize); werr != nil {
		return errxtrace.Wrap("failed to write backup container", werr)
	}
	return nil
}

// writeJSONMember marshals v and writes it as a single regular tar member named
// name. The JSON is built in memory (a single location document or the manifest
// — both bounded by row count, not file bytes), then streamed; the heavy file
// bytes go through writeFileMember which never buffers.
func (b *inbBuilder) writeJSONMember(name string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errxtrace.Wrap("failed to marshal JSON member", err, errx.Attrs("member", name))
	}
	hdr := &tar.Header{
		Name:     name,
		Mode:     0o600,
		Size:     int64(len(data)),
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	if err := b.tw.WriteHeader(hdr); err != nil {
		return errxtrace.Wrap("failed to write tar header", err, errx.Attrs("member", name))
	}
	if _, err := b.tw.Write(data); err != nil {
		return errxtrace.Wrap("failed to write tar member", err, errx.Attrs("member", name))
	}
	return nil
}

// writeFileMember streams a single commodity file's bytes from blob storage into
// the tar at archivePath, without buffering. Returns the byte count written.
func (b *inbBuilder) writeFileMember(archivePath, blobKey string, size int64) (int64, error) {
	reader, err := b.bucket.NewReader(b.ctx, blobKey, nil)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open blob reader for file member", err, errx.Attrs("blob_key", blobKey))
	}
	defer reader.Close()

	hdr := &tar.Header{
		Name:     archivePath,
		Mode:     0o600,
		Size:     size,
		Typeflag: tar.TypeReg,
		ModTime:  time.Now(),
	}
	if err := b.tw.WriteHeader(hdr); err != nil {
		return 0, errxtrace.Wrap("failed to write file member header", err, errx.Attrs("member", archivePath))
	}
	n, err := io.Copy(b.tw, reader)
	if err != nil {
		return 0, errxtrace.Wrap("failed to stream file member", err, errx.Attrs("member", archivePath))
	}
	return n, nil
}

// writeManifest emits the manifest.json member from the accumulated stats and
// the per-location index.
func (b *inbBuilder) writeManifest(locs []INBManifestLoc) error {
	manifest := INBManifest{
		ExportDate:  time.Now().UTC().Format(time.RFC3339),
		ExportType:  string(b.export.Type),
		Version:     INBFormatVersion,
		Format:      "json",
		Compression: "tar.gz",
		Signature: INBSignatureInfo{
			Algorithm:   backupsign.Algorithm,
			PublicKey:   b.svc.signer.PublicKeyBase64(),
			Fingerprint: b.svc.signer.Fingerprint(),
		},
		Locations: locs,
		Statistics: INBManifestStats{
			LocationCount:  b.stats.LocationCount,
			AreaCount:      b.stats.AreaCount,
			CommodityCount: b.stats.CommodityCount,
			ImageCount:     b.stats.ImageCount,
			InvoiceCount:   b.stats.InvoiceCount,
			ManualCount:    b.stats.ManualCount,
			FileCount:      b.stats.FileCount,
			TotalFileSize:  b.stats.BinaryDataSize,
		},
	}
	return b.writeJSONMember(INBManifestName, manifest)
}
