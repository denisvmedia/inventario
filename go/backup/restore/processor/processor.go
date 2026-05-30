//go:build !legacy_xml_backup

package processor

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/security"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/inb"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
)

// inbMemberLimits bounds the inner tar walk so a hostile archive can't exhaust
// the worker. The payload-size cap is enforced by inb.ReadContainer; these are
// the per-member and total-member caps applied while inflating.
const (
	maxInbMembers         = 1_000_000
	maxInbTotalUncompress = 8 << 30 // 8 GiB of inflated bytes
)

// decodeAndRestore is the default `.inb` decode entry point (#534). It verifies
// the archive signature against the server's own key BEFORE inflating, then
// walks the inner tar applying each entity through the shared model-level
// strategy handlers. A bad/missing signature, a non-`.inb` upload (e.g. legacy
// XML), or any framing violation fails the restore hard — there is no bypass.
func (l *RestoreOperationProcessor) decodeAndRestore(ctx context.Context, reader io.Reader, options types.RestoreOptions) (*types.RestoreStats, error) {
	stats := &types.RestoreStats{}

	if l.signer == nil {
		return stats, errx.NewSentinel("backup signer is required to restore an .inb archive")
	}

	// 1. Read the container framing: signature first, then the bounded payload
	//    stream. A legacy XML upload fails here with an inb sentinel.
	sig, payload, err := inb.ReadContainer(reader, inb.DefaultLimits())
	if err != nil {
		return stats, errxtrace.Wrap("not a valid signed .inb backup archive", err)
	}

	// 2. Spool the payload to a temp file while streaming a SHA-256 digest, so
	//    we can verify the signature WITHOUT buffering the payload in memory.
	tmp, err := os.CreateTemp("", "inventario-inb-restore-*.payload")
	if err != nil {
		return stats, errxtrace.Wrap("failed to create temp payload file", err)
	}
	tmpName := tmp.Name()
	defer func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}()

	digest := backupsign.NewDigest()
	if _, err := io.Copy(io.MultiWriter(tmp, digest), payload); err != nil {
		return stats, errxtrace.Wrap("failed to spool backup payload", err)
	}

	// 3. Verify the signature BEFORE inflating. Hard fail on mismatch.
	if err := l.signer.VerifyDigest(digest.Sum(nil), sig); err != nil {
		return stats, errxtrace.Wrap("backup signature verification failed; refusing to restore", err)
	}

	// 4. Rewind and inflate. The signature is now trusted, so it is safe to
	//    decompress the payload.
	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return stats, errxtrace.Wrap("failed to rewind temp payload", err)
	}

	prep, err := l.prepareRestore(ctx, options)
	if err != nil {
		return stats, err
	}

	if err := l.applyInbPayload(prep.ctx, tmp, stats, prep.existing, prep.idMapping, options); err != nil {
		return stats, err
	}
	return stats, nil
}

// applyInbPayload inflates the verified payload and walks the inner tar in
// order: per-location JSON members recreate entities; file members stream their
// bytes into a re-minted tenant blob key and create the file row.
func (l *RestoreOperationProcessor) applyInbPayload(
	ctx context.Context,
	payload io.Reader,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	gzr, err := gzip.NewReader(payload)
	if err != nil {
		return errxtrace.Wrap("failed to open gzip reader", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	walker := &inbWalker{
		proc:      l,
		ctx:       ctx,
		stats:     stats,
		existing:  existing,
		idMapping: idMapping,
		options:   options,
		fileRefs:  map[string]inbPendingFile{},
	}

	// Open the destination bucket once for the whole walk rather than per file
	// member. Skipped for dry-run / no upload location, where file bytes are
	// drained for the byte count but never written.
	if !options.DryRun && l.uploadLocation != "" {
		bucket, err := blob.OpenBucket(ctx, l.uploadLocation)
		if err != nil {
			return errxtrace.Wrap("failed to open blob bucket for restore", err)
		}
		defer bucket.Close()
		walker.bucket = bucket
	}

	var memberCount int
	var totalBytes int64
	for {
		hdr, nextErr := tr.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return errxtrace.Wrap("failed to read inner tar", nextErr)
		}
		memberCount++
		if memberCount > maxInbMembers {
			return errx.NewSentinel("backup archive exceeds the maximum member count")
		}
		if hdr.Typeflag != tar.TypeReg {
			return errx.Classify(errx.NewSentinel("backup archive contains a non-regular member"), errx.Attrs("name", hdr.Name, "typeflag", hdr.Typeflag))
		}

		safeName := blobkeys.SanitizeArchivePath(hdr.Name)
		if safeName != hdr.Name {
			return errx.Classify(errx.NewSentinel("backup archive member name is unsafe"), errx.Attrs("name", hdr.Name))
		}

		totalBytes += hdr.Size
		if totalBytes > maxInbTotalUncompress {
			return errx.NewSentinel("backup archive exceeds the maximum uncompressed size")
		}

		if err := walker.handleMember(hdr, tr); err != nil {
			return err
		}
	}

	return nil
}

// inbPendingFile records the metadata a file member needs once its bytes arrive.
type inbPendingFile struct {
	ref           types.INBFileRef
	bucket        string // images / invoices / manuals
	commodityUUID string
}

// inbWalker carries the per-restore state while walking inner-tar members.
type inbWalker struct {
	proc      *RestoreOperationProcessor
	ctx       context.Context
	stats     *types.RestoreStats
	existing  *types.ExistingEntities
	idMapping *types.IDMapping
	options   types.RestoreOptions

	// bucket is the destination blob bucket, opened once for the whole walk
	// (nil in dry-run or when no upload location is configured — file bytes are
	// then drained, not written).
	bucket *blob.Bucket

	// fileRefs maps an archive file path → its metadata, registered when a
	// location document is processed. A file member must match one of these.
	fileRefs map[string]inbPendingFile
}

// handleMember dispatches a tar member: the manifest is read for nothing
// security-relevant (stats come from the entity walk), location JSONs recreate
// entities and register their file refs, and files/ members stream their bytes.
func (w *inbWalker) handleMember(hdr *tar.Header, r io.Reader) error {
	switch {
	case hdr.Name == types.INBManifestMember:
		// Manifest is informational; drain it so the tar advances.
		_, err := io.Copy(io.Discard, r)
		return err
	case strings.HasPrefix(hdr.Name, "files/"):
		return w.handleFileMember(hdr, r)
	case strings.HasSuffix(hdr.Name, ".json"):
		return w.handleLocationMember(hdr, r)
	default:
		// Unknown member — drain and ignore for forward-compat.
		_, err := io.Copy(io.Discard, r)
		return err
	}
}

// handleLocationMember decodes a per-location JSON document, recreates the
// location → areas → commodities via the shared strategy handlers, and
// registers each commodity file reference by its archive path.
func (w *inbWalker) handleLocationMember(hdr *tar.Header, r io.Reader) error {
	data, err := io.ReadAll(io.LimitReader(r, hdr.Size))
	if err != nil {
		return errxtrace.Wrap("failed to read location member", err, errx.Attrs("member", hdr.Name))
	}
	var doc types.INBLocationDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return errxtrace.Wrap("failed to decode location document", err, errx.Attrs("member", hdr.Name))
	}
	return w.applyLocationDoc(&doc)
}

// applyLocationDoc recreates one location's full subtree.
func (w *inbWalker) applyLocationDoc(doc *types.INBLocationDoc) error {
	l := w.proc

	location := doc.Location.ConvertToLocation()
	if err := location.ValidateWithContext(w.ctx); err != nil {
		return errxtrace.Wrap("invalid location", err, errx.Attrs("location_id", doc.Location.ID))
	}
	step := locationStep(doc.Location.Name)
	l.createRestoreStep(w.ctx, step, models.RestoreStepResultInProgress, "")
	existingLoc := w.existing.Locations[doc.Location.ID]
	if err := l.applyStrategyForLocationModel(w.ctx, location, existingLoc, doc.Location.ID, doc.Location.Name, w.stats, w.existing, w.idMapping, w.options); err != nil {
		l.updateRestoreStep(w.ctx, step, models.RestoreStepResultError, err.Error())
		return err
	}
	l.updateRestoreStep(w.ctx, step, models.RestoreStepResultSuccess, "Completed")

	for i := range doc.Areas {
		if err := w.applyArea(&doc.Areas[i]); err != nil {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("failed to process area: %v", err))
		}
	}
	for i := range doc.Commodities {
		if err := w.applyCommodity(&doc.Commodities[i]); err != nil {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("failed to process commodity: %v", err))
		}
	}
	return nil
}

// applyArea recreates one area, resolving its parent location UUID → DB ID.
func (w *inbWalker) applyArea(a *types.INBArea) error {
	l := w.proc
	actualLocationID, ok := w.idMapping.Locations[a.LocationID]
	if !ok || actualLocationID == "" {
		return fmt.Errorf("area %s references unmapped location %s", a.ID, a.LocationID)
	}
	area := a.ConvertToArea()
	area.LocationID = actualLocationID
	if err := area.ValidateWithContext(w.ctx); err != nil {
		return errxtrace.Wrap("invalid area", err, errx.Attrs("area_id", a.ID))
	}
	existingArea := w.existing.Areas[a.ID]
	return l.applyStrategyForAreaModel(w.ctx, area, existingArea, a.ID, w.stats, w.existing, w.idMapping, w.options)
}

// applyCommodity recreates one commodity (resolving its area UUID → DB ID) and
// registers its file references for the later file members.
func (w *inbWalker) applyCommodity(c *types.INBCommodity) error {
	l := w.proc
	actualAreaID, ok := w.idMapping.Areas[c.AreaID]
	if !ok || actualAreaID == "" {
		return fmt.Errorf("commodity %s references unmapped area %s", c.ID, c.AreaID)
	}
	commodity := c.ConvertToCommodity()
	commodity.AreaID = actualAreaID

	// Mirror the XML restore: when the commodity's original currency matches the
	// group currency, the converted original price must be zero (the validator
	// rejects a non-zero converted price in that case).
	if groupCurrency, gcErr := validationctx.GroupCurrencyFromContext(w.ctx); gcErr == nil && string(commodity.OriginalPriceCurrency) == groupCurrency {
		commodity.ConvertedOriginalPrice = decimal.Zero
	}

	if err := commodity.ValidateWithContext(w.ctx); err != nil {
		return errxtrace.Wrap("invalid commodity", err, errx.Attrs("commodity_id", c.ID))
	}

	currentUser := appctx.UserFromContext(w.ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}
	if err := l.validateCommodityOwnershipInDB(w.ctx, c.ID, currentUser, w.existing, w.stats); err != nil {
		return err
	}

	existingCommodity := w.existing.Commodities[c.ID]
	if err := l.applyStrategyForCommodityModel(w.ctx, commodity, existingCommodity, c.ID, w.stats, w.existing, w.idMapping, w.options); err != nil {
		return err
	}

	// Register file refs so the later file members know which commodity (and
	// bucket) they belong to.
	w.registerFileRefs(c.ID, "images", c.Images)
	w.registerFileRefs(c.ID, "invoices", c.Invoices)
	w.registerFileRefs(c.ID, "manuals", c.Manuals)
	return nil
}

// registerFileRefs indexes a commodity's file references by their archive path.
func (w *inbWalker) registerFileRefs(commodityUUID, bucket string, refs []types.INBFileRef) {
	for _, ref := range refs {
		key := blobkeys.SanitizeArchivePath(ref.Path)
		w.fileRefs[key] = inbPendingFile{ref: ref, bucket: bucket, commodityUUID: commodityUUID}
	}
}

// handleFileMember streams a file member's bytes into a re-minted tenant blob
// key and creates the file row. The member name is validated and must match a
// file reference registered by an earlier location document (which is why the
// exporter writes the location JSON before its file bytes).
func (w *inbWalker) handleFileMember(hdr *tar.Header, r io.Reader) error {
	pending, ok := w.fileRefs[hdr.Name]
	if !ok {
		// A file member with no matching reference is rejected — restoring it
		// would create an orphan blob with no owning row.
		_, _ = io.Copy(io.Discard, r)
		w.stats.ErrorCount++
		w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("file member %s has no matching reference", hdr.Name))
		return nil
	}

	linkedDBID, resolved := w.idMapping.Commodities[pending.commodityUUID]
	if !resolved || linkedDBID == "" {
		_, _ = io.Copy(io.Discard, r)
		w.stats.ErrorCount++
		w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("file %s references unmapped commodity %s", pending.ref.ID, pending.commodityUUID))
		return nil
	}

	user := appctx.UserFromContext(w.ctx)
	if user == nil || user.TenantID == "" {
		return errors.New("tenant context is required to restore file data")
	}

	// Re-mint the destination blob key under the importing tenant. NEVER reuse
	// the archive path or the source OriginalPath as a key.
	ext := pending.ref.Extension
	if ext == "" {
		ext = path.Ext(hdr.Name)
	}
	blobKey := blobkeys.BuildFileBlobKey(user.TenantID, pending.ref.ID, ext)

	written, err := w.streamFileBytes(blobKey, r, hdr.Size)
	if err != nil {
		return err
	}
	w.stats.BinaryDataSize += written

	fileEntity := pending.ref.ConvertToFileEntity(linkedDBID, pending.bucket, blobKey)
	// Record the restored byte count so per-group storage-usage accounting
	// (#1388) isn't under-counted — `.inb` is the first format to round-trip
	// commodity file bytes, so this is the authoritative size on restore.
	if fileEntity.File != nil {
		fileEntity.File.SizeBytes = written
	}
	fileEntity.TenantID = user.TenantID
	fileEntity.CreatedByUserID = user.ID
	if group := appctx.GroupFromContext(w.ctx); group != nil {
		fileEntity.GroupID = group.ID
	}

	if err := w.proc.applyStrategyForFileModel(w.ctx, fileEntity, pending.ref.ID, w.stats, w.existing, w.idMapping, w.options); err != nil {
		return err
	}
	w.incBucketStat(pending.bucket)
	return nil
}

// streamFileBytes copies a file member's bytes into the destination blob key,
// without buffering. When no bucket is configured (DryRun or no upload location)
// it drains the bytes so the byte count still flows into stats.
func (w *inbWalker) streamFileBytes(blobKey string, r io.Reader, size int64) (int64, error) {
	src := io.LimitReader(r, size)
	if w.bucket == nil {
		return io.Copy(io.Discard, src)
	}

	writer, err := w.bucket.NewWriter(w.ctx, blobKey, nil)
	if err != nil {
		return 0, errxtrace.Wrap("failed to create blob writer", err, errx.Attrs("blob_key", blobKey))
	}
	n, copyErr := io.Copy(writer, src)
	closeErr := writer.Close()
	if copyErr != nil {
		return 0, errxtrace.Wrap("failed to stream file bytes", copyErr, errx.Attrs("blob_key", blobKey))
	}
	if closeErr != nil {
		return 0, errxtrace.Wrap("failed to close blob writer", closeErr, errx.Attrs("blob_key", blobKey))
	}
	return n, nil
}

// incBucketStat increments the per-bucket file counter.
func (w *inbWalker) incBucketStat(bucket string) {
	switch bucket {
	case "images":
		w.stats.ImageCount++
	case "invoices":
		w.stats.InvoiceCount++
	case "manuals":
		w.stats.ManualCount++
	}
}
