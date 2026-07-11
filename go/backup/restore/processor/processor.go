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
	"log/slog"
	"os"
	"path"
	"sort"
	"strconv"
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
	"github.com/denisvmedia/inventario/registry"
)

// inbMemberLimits bounds the inner tar walk so a hostile archive can't exhaust
// the worker. The payload-size cap is enforced by inb.ReadContainer; these are
// the per-member and total-member caps applied while inflating.
const (
	maxInbMembers         = 1_000_000
	maxInbTotalUncompress = 8 << 30  // 8 GiB of inflated bytes
	maxJSONDocBytes       = 32 << 20 // 32 MiB per JSON document member
	maxInbManifestBytes   = 4 << 20  // 4 MiB for the manifest pre-scan
)

// maxSupportedInbMajor is the highest `.inb` format MAJOR version this build can
// read. MINOR bumps are additive-only (a new optional member dispatched by name),
// so any 2.x archive restores here; a 3.x archive would carry shape changes this
// reader cannot know about, so it is rejected outright rather than half-applied.
const maxSupportedInbMajor = 2

var (
	// ErrJSONDocTooLarge is returned when a JSON document member (a location
	// document, the area-less commodities document of #1986, or the
	// non-commodity files document of #2235) declares a size above
	// maxJSONDocBytes. Without a per-member cap one document could claim up to
	// the 8 GiB total and OOM the worker via io.ReadAll. The failing member name
	// is attached, so the message stays diagnosable across all three.
	ErrJSONDocTooLarge = errx.NewSentinel("backup JSON document exceeds the maximum allowed size")

	// ErrMissingFileMembers is returned when one or more declared commodity file
	// references were never delivered as file members in the archive. Succeeding
	// silently would drop file data with no signal to the operator.
	ErrMissingFileMembers = errx.NewSentinel("backup archive is missing declared file members")

	// ErrMalformedEntity flags a structurally-corrupt entity field (an
	// unparseable numeric/timestamp value) decoded from the archive. Unlike a
	// per-item validation or mapping error — which applyLocationDoc tolerates
	// and counts — a malformed field means the archive itself is corrupt, so it
	// aborts the whole restore rather than silently coercing the bad value.
	ErrMalformedEntity = errx.NewSentinel("backup archive contains a malformed entity field")

	// ErrUnsupportedFormatVersion is returned when the archive's manifest
	// declares a format MAJOR version above maxSupportedInbMajor. Checked BEFORE
	// prepareRestore so a full_replace never wipes the target for an archive
	// this build cannot read.
	ErrUnsupportedFormatVersion = errx.NewSentinel("backup format version is not supported by this server")
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

	// 5. Gate on the declared format version BEFORE prepareRestore: full_replace
	//    wipes the existing data in prepareRestore, so an archive this build
	//    cannot read must be rejected while the target is still intact. The
	//    payload is already spooled to a temp file, so this costs one extra gzip
	//    open + the first member (manifest.json is always written first).
	if err := checkInbFormatVersion(tmp); err != nil {
		return stats, err
	}
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

// checkInbFormatVersion inspects the archive's manifest and rejects a format
// MAJOR version this build cannot read (ErrUnsupportedFormatVersion). Only the
// FIRST inner-tar member is read: the exporter always writes manifest.json first
// (pinned by TestINBExport_ManifestIsFirstMember), so this stays a cheap pre-scan
// rather than a second full inflate.
//
// Deliberately permissive in the other direction — an absent, empty, or
// unparseable version, and any MAJOR at or below maxSupportedInbMajor, is
// accepted. MINOR bumps are additive-only (a new optional member dispatched by
// name), which is what lets a 2.0 archive keep restoring on a 2.1 reader.
func checkInbFormatVersion(payload io.Reader) error {
	gzr, err := gzip.NewReader(payload)
	if err != nil {
		return errxtrace.Wrap("failed to open gzip reader", err)
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	hdr, err := tr.Next()
	if err == io.EOF {
		// An empty payload has nothing to gate; the walk reports it.
		return nil
	}
	if err != nil {
		return errxtrace.Wrap("failed to read inner tar", err)
	}
	if hdr.Name != types.INBManifestMember || hdr.Size < 0 || hdr.Size > maxInbManifestBytes {
		// No manifest up front (or an implausibly large one) — leave it to the
		// walk, which drains unknown members and caps every one it parses.
		return nil
	}

	data, err := io.ReadAll(io.LimitReader(tr, hdr.Size))
	if err != nil {
		return errxtrace.Wrap("failed to read manifest member", err)
	}
	var manifest types.INBManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return errxtrace.Wrap("failed to decode manifest document", err)
	}

	major, ok := inbFormatMajor(manifest.Version)
	if ok && major > maxSupportedInbMajor {
		return errx.Classify(ErrUnsupportedFormatVersion,
			errx.Attrs("version", manifest.Version, "max_supported_major", maxSupportedInbMajor))
	}
	return nil
}

// inbFormatMajor extracts the MAJOR component of a manifest format version
// ("2.1" → 2). ok=false for an absent or unparseable value, which the caller
// treats as "no constraint" rather than a failure.
func inbFormatMajor(version string) (int, bool) {
	if version == "" {
		return 0, false
	}
	majorPart, _, _ := strings.Cut(version, ".")
	major, err := strconv.Atoi(majorPart)
	if err != nil {
		return 0, false
	}
	return major, true
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

	// Every declared commodity file reference must have been delivered as a file
	// member (handleFileMember deletes its key on success). Any left over means
	// the archive promised file bytes it never carried — fail rather than report
	// a successful restore that silently lost data.
	if len(walker.fileRefs) > 0 {
		missing := make([]string, 0, len(walker.fileRefs))
		for name := range walker.fileRefs {
			missing = append(missing, name)
		}
		sort.Strings(missing)
		return errx.Classify(ErrMissingFileMembers, errx.Attrs("count", len(missing), "members", missing))
	}

	// Patch cover-photo cross-references last: only now is idMapping.Files fully
	// populated, so a cover that points at one of the just-restored files can be
	// resolved to its new DB id.
	if err := walker.applyPendingCovers(); err != nil {
		return err
	}

	return nil
}

// applyPendingCovers resolves each queued commodity → cover-photo reference and
// patches the commodity's CoverFileID to the new file's DB id. A cover whose
// file UUID never landed (e.g. an orphaned/dropped attachment) is skipped
// silently — the cover-resolver's first-photo fallback then applies. The patch
// goes through the normal commodity Update (CoverFileID is an ordinary column);
// the registry preserves the acquisition pair across that write.
func (w *inbWalker) applyPendingCovers() error {
	if len(w.pendingCovers) == 0 {
		return nil
	}

	comReg, err := w.proc.factorySet.CommodityRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user commodity registry for cover patch", err)
	}

	for _, pc := range w.pendingCovers {
		commodityDBID, ok := w.idMapping.Commodities[pc.commodityUUID]
		if !ok || commodityDBID == "" {
			continue
		}
		coverFileDBID, ok := w.idMapping.Files[pc.coverFileUUID]
		if !ok || coverFileDBID == "" {
			// The cover photo's file was not restored (orphan / missing member);
			// leave the cover unset and let the first-photo fallback take over.
			continue
		}

		commodity, getErr := comReg.Get(w.ctx, commodityDBID)
		if getErr != nil {
			return errxtrace.Wrap("failed to load commodity for cover patch", getErr, errx.Attrs("commodity_id", pc.commodityUUID))
		}
		coverID := coverFileDBID
		commodity.CoverFileID = &coverID
		if _, updErr := comReg.Update(w.ctx, *commodity); updErr != nil {
			return errxtrace.Wrap("failed to patch commodity cover", updErr, errx.Attrs("commodity_id", pc.commodityUUID))
		}
	}
	return nil
}

// inbPendingFile records the metadata a file member needs once its bytes arrive.
type inbPendingFile struct {
	ref  types.INBFileRef
	link inbFileLink
}

// inbFileLink is a file's ARCHIVE-side link: the linked entity's immutable UUID
// plus its bucket, resolved to a destination DB id only when the bytes arrive
// (the ID mapping is filled as the entity documents are applied).
//
// linkedType is "commodity" (nested under a commodity — the pre-#2235 shape),
// "location"/"area" (from the files member), or "" for a standalone file.
// fileType/category are carried by the files member and empty for a commodity
// ref, which re-derives them from the bucket + MIME as before.
type inbFileLink struct {
	linkedType string
	entityUUID string
	meta       string
	fileType   string
	category   string
}

// inbPendingCover records a commodity → cover-photo cross-reference that can
// only be resolved after the commodity's files are created. The cover is
// archived as the file's immutable UUID; the new file DB id is not known until
// the file member is streamed and its row created later in the tar walk, so the
// patch is deferred to the end of the payload walk. #534 / #1451.
type inbPendingCover struct {
	commodityUUID string
	coverFileUUID string
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

	// pendingCovers accumulates cover-photo cross-references to patch once every
	// file member has been processed (so idMapping.Files is fully populated).
	// Only created commodities (write strategies, non-dry-run) register one.
	pendingCovers []inbPendingCover
}

// handleMember dispatches a tar member: the manifest is read for nothing
// security-relevant (stats come from the entity walk), location JSONs recreate
// entities and register their file refs, and files/ members stream their bytes.
func (w *inbWalker) handleMember(hdr *tar.Header, r io.Reader) error {
	switch {
	case hdr.Name == types.INBManifestMember:
		// Manifest is informational here — its format version was already gated
		// in checkInbFormatVersion, before any data was touched. Drain it so the
		// tar advances.
		_, err := io.Copy(io.Discard, r)
		return err
	case hdr.Name == types.INBFilesMember:
		// Non-commodity files document (issue #2235). It lives UNDER the `files/`
		// prefix, so it must be matched before the file-bytes case below.
		return w.handleFilesMember(hdr, r)
	case strings.HasPrefix(hdr.Name, "files/"):
		return w.handleFileMember(hdr, r)
	case hdr.Name == types.INBUnassignedMember:
		// Area-less commodities document (issue #1986) — must be matched
		// before the generic ".json" case since it also ends in ".json".
		return w.handleUnassignedMember(hdr, r)
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
	// Per-member cap: a location JSON is parsed whole via io.ReadAll, so without
	// this a single member could declare up to the 8 GiB total and OOM the
	// worker. 32 MiB is far above any realistic location document.
	if hdr.Size < 0 || hdr.Size > maxJSONDocBytes {
		return errx.Classify(ErrJSONDocTooLarge, errx.Attrs("member", hdr.Name, "size", hdr.Size, "max", maxJSONDocBytes))
	}
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
			if errors.Is(err, ErrMalformedEntity) {
				return err
			}
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("failed to process area: %v", err))
		}
	}
	for i := range doc.Commodities {
		if err := w.applyCommodity(&doc.Commodities[i]); err != nil {
			// A malformed field (unparseable price/timestamp) is archive
			// corruption — abort the whole restore. Other per-item errors stay
			// tolerated-and-counted as before.
			if errors.Is(err, ErrMalformedEntity) {
				return err
			}
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("failed to process commodity: %v", err))
		}
	}
	return nil
}

// handleUnassignedMember decodes the area-less commodities document (issue
// #1986) and recreates each commodity with a nil area. The same per-member size
// cap as location documents applies (a flat commodity list, no file bytes).
func (w *inbWalker) handleUnassignedMember(hdr *tar.Header, r io.Reader) error {
	if hdr.Size < 0 || hdr.Size > maxJSONDocBytes {
		return errx.Classify(ErrJSONDocTooLarge, errx.Attrs("member", hdr.Name, "size", hdr.Size, "max", maxJSONDocBytes))
	}
	data, err := io.ReadAll(io.LimitReader(r, hdr.Size))
	if err != nil {
		return errxtrace.Wrap("failed to read unassigned member", err, errx.Attrs("member", hdr.Name))
	}
	var doc types.INBUnassignedDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return errxtrace.Wrap("failed to decode unassigned document", err, errx.Attrs("member", hdr.Name))
	}
	return w.applyUnassignedDoc(&doc)
}

// applyUnassignedDoc recreates the area-less commodities (issue #1986). Per-item
// errors are tolerated-and-counted exactly like the location-member commodity
// loop; a malformed field aborts the whole restore.
func (w *inbWalker) applyUnassignedDoc(doc *types.INBUnassignedDoc) error {
	for i := range doc.Commodities {
		if err := w.applyUnassignedCommodity(&doc.Commodities[i]); err != nil {
			if errors.Is(err, ErrMalformedEntity) {
				return err
			}
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

// applyCommodity recreates one area-bound commodity (resolving its area UUID →
// DB ID) and registers its file references for the later file members.
func (w *inbWalker) applyCommodity(c *types.INBCommodity) error {
	actualAreaID, ok := w.idMapping.Areas[c.AreaID]
	if !ok || actualAreaID == "" {
		return fmt.Errorf("commodity %s references unmapped area %s", c.ID, c.AreaID)
	}
	return w.applyCommodityModel(c, &actualAreaID)
}

// applyUnassignedCommodity recreates one area-less ("unassigned") commodity
// (issue #1986) and registers its file references. No area is resolved or
// created — the row is persisted with a nil AreaID, and no synthetic
// location/area is fabricated in the restored data.
func (w *inbWalker) applyUnassignedCommodity(c *types.INBCommodity) error {
	return w.applyCommodityModel(c, nil)
}

// applyCommodityModel is the shared restore body for a commodity. actualAreaID is
// the resolved destination area DB ID for an area-bound commodity, or nil for an
// area-less one (issue #1986). It builds, validates, and persists the row,
// queues the cover-photo patch, and registers the commodity's file references.
func (w *inbWalker) applyCommodityModel(c *types.INBCommodity, actualAreaID *string) error {
	l := w.proc
	commodity, err := c.ConvertToCommodity()
	if err != nil {
		// A malformed numeric field corrupts the restored value — abort the
		// restore (ErrMalformedEntity is propagated hard by applyLocationDoc)
		// rather than silently coercing it to zero.
		return errx.Classify(
			errxtrace.Wrap("failed to convert commodity", err, errx.Attrs("commodity_id", c.ID)),
			ErrMalformedEntity,
		)
	}
	// Area is optional (issue #1986): a nil actualAreaID leaves the row
	// area-less; otherwise pin it to the resolved destination area DB ID.
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

	// Acquisition provenance (#202) is reconstructed on CREATE only, and only
	// through the trusted WithRestoreAcquisition context seam — never a public
	// registry bypass. Create reads the pair from ctx and writes it onto the
	// fresh row; a merge_update of an existing row goes through Update, which
	// ignores the signal and keeps the row's own write-once acquisition.
	createCtx := w.ctx
	if price, currency, aqErr := c.RestoredAcquisition(); aqErr != nil {
		// A malformed acquisition price is archive corruption — abort the whole
		// restore, consistent with ConvertToCommodity's strict numeric parsing.
		return errx.Classify(
			errxtrace.Wrap("failed to decode restored acquisition", aqErr, errx.Attrs("commodity_id", c.ID)),
			ErrMalformedEntity,
		)
	} else if price != nil && currency != nil {
		createCtx = registry.WithRestoreAcquisition(createCtx, *price, *currency)
	}

	existingCommodity := w.existing.Commodities[c.ID]
	if err := l.applyStrategyForCommodityModel(createCtx, commodity, existingCommodity, c.ID, w.stats, w.existing, w.idMapping, w.options); err != nil {
		return err
	}

	// Cover-photo cross-reference (#1451) is patched only after the commodity's
	// files are restored (their new DB ids aren't known yet), and only for a
	// freshly CREATED row — a dry-run writes nothing and a merge_update keeps the
	// existing row's own cover.
	if w.commodityWasCreated(existingCommodity) {
		w.queueCoverPatch(c)
	}

	// Register file refs so the later file members know which commodity (and
	// bucket) they belong to.
	w.registerFileRefs(c.ID, "images", c.Images)
	w.registerFileRefs(c.ID, "invoices", c.Invoices)
	w.registerFileRefs(c.ID, "manuals", c.Manuals)
	return nil
}

// commodityWasCreated reports whether applyStrategyForCommodityModel just
// created a fresh row (vs. skipped/updated an existing one) for the current
// options. A dry-run never persists, so it is always false. full_replace always
// creates; merge_add / merge_update create only when no row pre-existed.
func (w *inbWalker) commodityWasCreated(existingCommodity *models.Commodity) bool {
	if w.options.DryRun {
		return false
	}
	if w.options.Strategy == types.RestoreStrategyFullReplace {
		return true
	}
	return existingCommodity == nil
}

// queueCoverPatch records the commodity's cover-photo cross-reference (#1451)
// for the post-files patch. The cover is stored in the archive as the cover
// file's immutable UUID; its destination DB id isn't known until that file is
// restored, so the patch is deferred. A no-op when no cover was set.
func (w *inbWalker) queueCoverPatch(c *types.INBCommodity) {
	if c.CoverFileID != "" {
		w.pendingCovers = append(w.pendingCovers, inbPendingCover{
			commodityUUID: c.ID,
			coverFileUUID: c.CoverFileID,
		})
	}
}

// registerFileRefs indexes a commodity's file references by their archive path.
func (w *inbWalker) registerFileRefs(commodityUUID, bucket string, refs []types.INBFileRef) {
	for _, ref := range refs {
		key := blobkeys.SanitizeArchivePath(ref.Path)
		w.fileRefs[key] = inbPendingFile{
			ref:  ref,
			link: inbFileLink{linkedType: "commodity", entityUUID: commodityUUID, meta: bucket},
		}
	}
}

// handleFilesMember decodes the non-commodity files document (issue #2235) and
// registers its references, so the file members that follow are matched exactly
// like commodity attachments (same fileRefs map, so the missing-member check and
// the JSON-before-bytes contract keep working unchanged). The same per-member size
// cap as location documents applies — the document is parsed whole via io.ReadAll.
func (w *inbWalker) handleFilesMember(hdr *tar.Header, r io.Reader) error {
	if hdr.Size < 0 || hdr.Size > maxJSONDocBytes {
		return errx.Classify(ErrJSONDocTooLarge, errx.Attrs("member", hdr.Name, "size", hdr.Size, "max", maxJSONDocBytes))
	}
	data, err := io.ReadAll(io.LimitReader(r, hdr.Size))
	if err != nil {
		return errxtrace.Wrap("failed to read files member", err, errx.Attrs("member", hdr.Name))
	}
	var doc types.INBFilesDoc
	if err := json.Unmarshal(data, &doc); err != nil {
		return errxtrace.Wrap("failed to decode files document", err, errx.Attrs("member", hdr.Name))
	}
	w.registerEntityFileRefs(doc.Files)
	return nil
}

// registerEntityFileRefs indexes the non-commodity file references by their
// archive path (issue #2235). Each ref carries its own link, so the later file
// member recreates the row with the original linked_entity_type / meta instead of
// assuming a commodity attachment.
func (w *inbWalker) registerEntityFileRefs(refs []types.INBEntityFileRef) {
	for _, ref := range refs {
		key := blobkeys.SanitizeArchivePath(ref.Path)
		w.fileRefs[key] = inbPendingFile{
			ref: ref.INBFileRef,
			link: inbFileLink{
				linkedType: ref.LinkedEntityType,
				entityUUID: ref.LinkedEntityID,
				meta:       ref.LinkedEntityMeta,
				fileType:   ref.Type,
				category:   ref.Category,
			},
		}
	}
}

// resolveLinkedDBID maps a file's archived link to the destination entity's DB id.
// A standalone file (empty type) resolves to an empty id, which is correct — the
// row is persisted with no link at all. ok=false when the archive references an
// entity that never landed (or an entity type this build does not know), in which
// case the caller drops the file rather than persisting a dangling reference.
func (w *inbWalker) resolveLinkedDBID(link inbFileLink) (string, bool) {
	var dbID string
	var found bool
	switch link.linkedType {
	case "":
		return "", true
	case "commodity":
		dbID, found = w.idMapping.Commodities[link.entityUUID]
	case "location":
		dbID, found = w.idMapping.Locations[link.entityUUID]
	case "area":
		dbID, found = w.idMapping.Areas[link.entityUUID]
	default:
		return "", false
	}
	if !found || dbID == "" {
		return "", false
	}
	return dbID, true
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
	// The declared reference WAS delivered as a member, so it must not later be
	// reported as missing — drop it from the pending set regardless of whether
	// it ultimately links/streams below. The missing-member check only flags
	// refs whose member never appeared in the archive (e.g. dry-run, where the
	// commodity isn't persisted, still delivers and consumes the member here).
	delete(w.fileRefs, hdr.Name)

	// Resolve the archived link (commodity / location / area / standalone) to a
	// destination DB id. A link whose entity never landed is dropped with a
	// counted error — a dangling linked_entity_id must never be persisted.
	linkedDBID, resolved := w.resolveLinkedDBID(pending.link)
	if !resolved {
		_, _ = io.Copy(io.Discard, r)
		w.stats.ErrorCount++
		w.stats.Errors = append(w.stats.Errors,
			fmt.Sprintf("file %s references unmapped %s %s", pending.ref.ID, pending.link.linkedType, pending.link.entityUUID))
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

	// Build the file entity BEFORE streaming the bytes: a malformed timestamp
	// corrupts the restored metadata, so fail the restore (rather than coercing
	// to time.Now()) without first writing a doomed blob.
	fileEntity, err := pending.ref.ConvertToFileEntity(types.INBFileLink{
		Type:     pending.link.linkedType,
		DBID:     linkedDBID,
		Meta:     pending.link.meta,
		FileType: pending.link.fileType,
		Category: pending.link.category,
	}, blobKey)
	if err != nil {
		// A malformed timestamp is archive corruption — drain the member and
		// abort the restore (handleFileMember errors already propagate hard).
		_, _ = io.Copy(io.Discard, r)
		return errx.Classify(
			errxtrace.Wrap("failed to convert file entity", err, errx.Attrs("file_id", pending.ref.ID)),
			ErrMalformedEntity,
		)
	}

	// Decide the create/update/skip action BEFORE streaming so a MergeAdd-skip
	// never writes a blob that has no owning row to clean it up (issue #2125).
	// The decision depends only on the strategy and whether the file already
	// exists — both known here — so the bytes can be drained (skip) instead of
	// committed to blobKey. This mirrors the legacy XML path's
	// shouldWriteFileBlob guard, which already gates the blob write.
	action := decideFileStrategyAction(w.options.Strategy, w.existing.Files[pending.ref.ID])

	if action == fileActionSkip {
		return w.skipFileMember(r, hdr.Size, blobKey)
	}

	written, err := w.streamFileBytes(blobKey, r, hdr.Size)
	if err != nil {
		return err
	}
	w.stats.BinaryDataSize += written

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

	// On update, capture the existing row's blob key so a superseded blob can be
	// removed after the row is re-pointed (resolved before the row update so the
	// old OriginalPath is still readable).
	staleBlobKey := w.staleBlobKeyForUpdate(action, pending.ref.ID)

	if err := w.proc.applyStrategyForFileModel(w.ctx, fileEntity, pending.ref.ID, w.stats, w.existing, w.idMapping, w.options); err != nil {
		return err
	}

	w.deleteSupersededBlob(staleBlobKey, blobKey)

	w.incBucketStat(pending.link)
	return nil
}

// skipFileMember drains a file member's bytes (so the byte count still flows
// into stats and the tar stream advances) WITHOUT writing them to the bucket,
// for a MergeAdd-skip. This is what prevents the orphan blob a pre-decision
// stream would leave behind (issue #2125).
func (w *inbWalker) skipFileMember(r io.Reader, size int64, blobKey string) error {
	drained, err := io.Copy(io.Discard, io.LimitReader(r, size))
	if err != nil {
		return errxtrace.Wrap("failed to drain skipped file member", err, errx.Attrs("blob_key", blobKey))
	}
	w.stats.BinaryDataSize += drained
	w.stats.SkippedCount++
	return nil
}

// staleBlobKeyForUpdate returns the existing row's blob key on a MergeUpdate so
// it can be cleaned up after the row is re-pointed, or "" when there is nothing
// to clean (not an update, or no existing blob). blobKey is deterministic from
// the (tenant, file-UUID, ext) tuple, so it usually equals the existing key and
// the bytes overwrite in place; but when the existing row stored a different key
// (an upload-time basename key, a different ext) the old blob would dangle
// without this (issue #2125).
func (w *inbWalker) staleBlobKeyForUpdate(action fileAction, refID string) string {
	if action != fileActionUpdate {
		return ""
	}
	if existingFile := w.existing.Files[refID]; existingFile != nil && existingFile.File != nil {
		return existingFile.File.OriginalPath
	}
	return ""
}

// deleteSupersededBlob best-effort removes the old blob after a MergeUpdate
// re-pointed the row to newBlobKey. A no-op when the key is unchanged, empty, or
// no bucket is configured. Swallow+log: a failed cleanup must never fail an
// otherwise-successful restore (issue #2125).
func (w *inbWalker) deleteSupersededBlob(staleBlobKey, newBlobKey string) {
	if staleBlobKey == "" || staleBlobKey == newBlobKey || w.bucket == nil {
		return
	}
	if err := w.bucket.Delete(w.ctx, staleBlobKey); err != nil {
		slog.WarnContext(w.ctx, "failed to delete superseded file blob after merge-update restore (best-effort)",
			"stale_blob_key", staleBlobKey, "new_blob_key", newBlobKey, "error", err.Error())
	}
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

// incBucketStat increments the per-bucket file counter. ImageCount/InvoiceCount/
// ManualCount are legacy COMMODITY-scoped counters, so a location/area file (whose
// buckets are images/files) or a standalone file must NOT inflate them — those
// only feed the unified FileCount (issue #2235).
func (w *inbWalker) incBucketStat(link inbFileLink) {
	if link.linkedType != "commodity" {
		return
	}
	switch link.meta {
	case "images":
		w.stats.ImageCount++
	case "invoices":
		w.stats.InvoiceCount++
	case "manuals":
		w.stats.ManualCount++
	}
}
