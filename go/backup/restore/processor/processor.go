//go:build !legacy_xml_backup

package processor

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/restore/security"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/internal/backupsign"
	"go.5x5.cz/inventario/internal/blobkeys"
	"go.5x5.cz/inventario/internal/inb"
	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// inbMemberLimits bounds the inner tar walk so a hostile archive can't exhaust
// the worker. The payload-size cap is enforced by inb.ReadContainer; these are
// the per-member and total-member caps applied while inflating.
const (
	maxInbMembers         = 1_000_000
	maxInbTotalUncompress = 8 << 30  // 8 GiB of inflated bytes
	maxJSONDocBytes       = 32 << 20 // 32 MiB per JSON document member; the export checks the drafts document against the same limit (backup/export defaultMaxJSONDocBytes)
	// The manifest carries the group's sellers (format 3.3) and item types
	// (3.4), so it gets the same room as any other JSON document.
	maxInbManifestBytes = maxJSONDocBytes
)

// maxSupportedInbMajor is the highest `.inb` format MAJOR version this build can
// read. MINOR bumps are additive-only (a new optional member dispatched by name).
// A 2.x archive references every file once; a 3.x archive may reference a
// shared file from several documents, which this reader turns into several
// links on one file. A 4.x archive would carry shape changes this reader cannot
// know about, so it is rejected outright rather than half-applied.
const maxSupportedInbMajor = 3

var (
	// ErrJSONDocTooLarge is returned when a JSON document member (a location
	// document, the area-less commodities document of #1986, the drafts
	// document, or the non-commodity files document of #2235) declares a size
	// above maxJSONDocBytes. Without a per-member cap one document could claim
	// up to the 8 GiB total and OOM the worker via io.ReadAll. The failing
	// member name is attached, so the message stays diagnosable across all of
	// them.
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
	if hdr.Name != types.INBManifestMember {
		// No manifest up front. Pre-manifest archives are still readable, and the
		// walk drains unknown members, so this is not itself a violation — there
		// is simply no declared version to gate on.
		return nil
	}
	if hdr.Size < 0 || hdr.Size > maxInbManifestBytes {
		// An implausibly-sized manifest is a framing violation, and it must NOT
		// fall through to "no constraint": that would let a crafted archive skip
		// the version gate entirely and reach prepareRestore, where full_replace
		// wipes the target before a format this build cannot read is even parsed.
		// Fail while the target is still intact.
		return errx.Classify(ErrJSONDocTooLarge,
			errx.Attrs("member", hdr.Name, "size", hdr.Size, "max", maxInbManifestBytes))
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
		fileRefs:  map[string]*inbPendingFile{},
		delivered: map[string]string{},
		parked:    map[string]bool{},

		draftIDs: map[string]string{},

		priceReading: types.PricesOmitZero,
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

// inbPendingFile records the metadata a file member needs once its bytes
// arrive. A file attached to several entities is referenced once per entity,
// every reference naming the same archive path; links collects one entry per
// reference registered before the bytes arrived.
type inbPendingFile struct {
	ref   types.INBFileRef
	links []inbFileLink
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
	fileRefs map[string]*inbPendingFile

	// delivered maps the archive path of every file member already processed →
	// the file's UUID. A reference registered after its bytes arrived (the
	// second entity of a shared file, in a later document) becomes a link on
	// the restored file instead of a pending member.
	delivered map[string]string

	// parked holds the UUIDs of the files this restore created in Trash
	// because every link they had pointed at an item in Trash. A later
	// reference to a live entity brings such a file back.
	parked map[string]bool

	// pendingCovers accumulates cover-photo cross-references to patch once every
	// file member has been processed (so idMapping.Files is fully populated).
	// Only created commodities (write strategies, non-dry-run) register one.
	pendingCovers []inbPendingCover

	// priceReading says how the archive's prices are read. It starts as the
	// reading for archives without a manifest, and the manifest, which the
	// exporter always writes first, sets it from its format version.
	priceReading types.PriceReading

	// sellerIDs maps an archived seller's UUID to the destination seller's
	// DB id, filled from the manifest. An item's seller is set only through
	// this map, so an archive can never point an item at a seller it did not
	// carry.
	sellerIDs map[string]string

	// draftIDs maps an archived draft's UUID to the restored draft's DB id.
	// A link to an archived draft that is not in it (a draft a merge kept,
	// one that failed to restore, one the archive does not carry) is left
	// out without an error.
	draftIDs map[string]string
}

// handleMember dispatches a tar member: the manifest is read for nothing
// security-relevant (stats come from the entity walk), location JSONs recreate
// entities and register their file refs, and files/ members stream their bytes.
func (w *inbWalker) handleMember(hdr *tar.Header, r io.Reader) error {
	switch {
	case hdr.Name == types.INBManifestMember:
		// The format version was already gated in checkInbFormatVersion,
		// before any data was touched. Here it only selects how prices read.
		return w.handleManifestMember(hdr, r)
	case hdr.Name == types.INBFilesMember:
		// Non-commodity files document (issue #2235). It lives UNDER the `files/`
		// prefix, so it must be matched before the file-bytes case below.
		return w.handleFilesMember(hdr, r)
	case hdr.Name == types.INBDraftsMember:
		// Drafts document (format 3.5), also under `files/`.
		return w.handleDraftsMember(hdr, r)
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

// handleManifestMember reads the manifest's format version, which decides how
// an absent price in the commodity documents that follow is read.
func (w *inbWalker) handleManifestMember(hdr *tar.Header, r io.Reader) error {
	if hdr.Size < 0 || hdr.Size > maxInbManifestBytes {
		return errx.Classify(ErrJSONDocTooLarge,
			errx.Attrs("member", hdr.Name, "size", hdr.Size, "max", maxInbManifestBytes))
	}
	data, err := io.ReadAll(io.LimitReader(r, hdr.Size))
	if err != nil {
		return errxtrace.Wrap("failed to read manifest member", err)
	}
	var manifest types.INBManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return errxtrace.Wrap("failed to decode manifest document", err)
	}
	w.priceReading = types.PriceReadingFor(manifest.Version)
	if err := w.restoreItemTypes(manifest.ItemTypes); err != nil {
		return err
	}
	return w.restoreSellers(manifest.Sellers)
}

// restoreItemTypes maps the archived item types onto the destination group's
// before the first item names one (#2802).
func (w *inbWalker) restoreItemTypes(archived []types.INBItemType) error {
	resolver, err := w.proc.itemTypeResolver(w.ctx, w.options)
	if err != nil {
		return err
	}
	return resolver.importArchived(w.ctx, archived, w.stats)
}

// restoreSellers finds or creates the destination seller of each archived
// seller and records the mapping. A seller is matched by name, the way the
// group keeps seller names unique, so a restore into a group that already
// has the seller reuses it and leaves its fields as they are. A dry run
// writes nothing and maps nothing, and an archived seller that fails
// validation is counted as an error and its items restore without a seller.
func (w *inbWalker) restoreSellers(archived []types.INBSeller) error {
	w.sellerIDs = map[string]string{}
	if len(archived) == 0 || w.options.DryRun || w.proc.factorySet.SellerRegistryFactory == nil {
		return nil
	}
	reg, err := w.proc.factorySet.SellerRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create seller registry", err)
	}
	for _, a := range archived {
		if a.ID == "" {
			continue
		}
		if existing, err := reg.GetByName(w.ctx, a.Name); err == nil {
			w.sellerIDs[a.ID] = existing.ID
			continue
		} else if !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to look up seller", err, errx.Attrs("seller_id", a.ID))
		}
		seller := models.Seller{Name: a.Name, Website: a.Website, Contact: a.Contact, Note: a.Note}
		seller.Normalize()
		if err := seller.ValidateFields(w.ctx); err != nil {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors, fmt.Sprintf("seller %s: %v", a.ID, err))
			continue
		}
		created, err := reg.Create(w.ctx, seller)
		if err != nil {
			return errxtrace.Wrap("failed to create seller", err, errx.Attrs("seller_id", a.ID))
		}
		w.sellerIDs[a.ID] = created.ID
	}
	return nil
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
	commodity, err := c.ConvertToCommodity(w.priceReading)
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
	// The seller resolves through the manifest's sellers only; an unknown
	// one leaves the item without a seller.
	commodity.SellerID = nil
	if sellerID, ok := w.sellerIDs[c.SellerID]; ok && c.SellerID != "" {
		commodity.SellerID = &sellerID
	}

	resolver, err := l.itemTypeResolver(w.ctx, w.options)
	if err != nil {
		return err
	}
	if commodity.Type, err = resolver.resolve(w.ctx, commodity.Type, w.stats); err != nil {
		return errxtrace.Wrap("failed to resolve item type", err, errx.Attrs("commodity_id", c.ID))
	}

	// Mirror the XML restore: when the commodity's original currency matches the
	// group currency, a converted price does not apply (the validator rejects a
	// non-zero one in that case), so it restores as unknown.
	if groupCurrency, gcErr := validationctx.GroupCurrencyFromContext(w.ctx); gcErr == nil && string(commodity.OriginalPriceCurrency) == groupCurrency {
		commodity.ConvertedOriginalPrice = nil
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
	// The asset number travels the same way: Create keeps the archived number
	// when the target group has no item with it, otherwise it assigns the next
	// free one, so labels printed before the backup still match and a merge
	// never yields two items with one asset ID.
	if c.AssetNumber > 0 {
		createCtx = registry.WithRestoreAssetNumber(createCtx, c.AssetNumber)
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
	if err := w.registerFileRefs(c.ID, models.FileLinkMetaImages, c.Images); err != nil {
		return err
	}
	return w.registerFileRefs(c.ID, models.FileLinkMetaDocuments, c.Documents)
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
func (w *inbWalker) registerFileRefs(commodityUUID, bucket string, refs []types.INBFileRef) error {
	for _, ref := range refs {
		link := inbFileLink{linkedType: models.FileLinkCommodity, entityUUID: commodityUUID, meta: bucket}
		if err := w.registerRef(ref, link); err != nil {
			return err
		}
	}
	return nil
}

// registerRef records one archived reference to a file. The first reference to
// an archive path creates the pending entry; later ones add their link to it,
// or, when the bytes were already delivered, to the restored file.
func (w *inbWalker) registerRef(ref types.INBFileRef, link inbFileLink) error {
	key := blobkeys.SanitizeArchivePath(ref.Path)
	if pending, ok := w.fileRefs[key]; ok {
		pending.links = append(pending.links, link)
		return nil
	}
	if fileUUID, ok := w.delivered[key]; ok {
		return w.linkDeliveredFile(fileUUID, ref, link)
	}
	w.fileRefs[key] = &inbPendingFile{ref: ref, links: []inbFileLink{link}}
	return nil
}

// linkDeliveredFile attaches an already-restored file to the entity of a
// reference that arrived after the file's bytes. The attach is idempotent, so a
// retried restore never duplicates a link.
func (w *inbWalker) linkDeliveredFile(fileUUID string, ref types.INBFileRef, link inbFileLink) error {
	if fileUUID == "" || link.linkedType == "" || w.isUnrestoredDraft(link) {
		// fileUUID is empty for bytes that arrived before any reference
		// could use them; they were dropped with a counted error.
		return nil
	}
	dbID, ok := w.resolveLinkedDBID(link)
	live := ok
	if !ok {
		if dbID = w.trashedItemID(link); dbID == "" {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors,
				fmt.Sprintf("file %s references unmapped %s %s", fileUUID, link.linkedType, link.entityUUID))
			return nil
		}
	}
	if live {
		w.incBucketStat(link, ref.Tags)
	}
	if w.options.DryRun {
		return nil
	}
	fileDBID := w.idMapping.Files[fileUUID]
	if fileDBID == "" {
		// The file itself did not land (every earlier link was unmapped), so
		// there is nothing to attach.
		return nil
	}
	if err := w.ensureLinks(fileDBID, []types.INBFileLink{{Type: link.linkedType, DBID: dbID, Meta: link.meta}}); err != nil {
		return err
	}
	if live && w.parked[fileUUID] {
		// A live entity uses the file now, so it leaves Trash.
		delete(w.parked, fileUUID)
		return w.proc.unparkFile(w.ctx, fileDBID)
	}
	return nil
}

// ensureLinks attaches the file to every given entity it is not attached to
// yet. Used when the file row already existed (merge strategies) and for
// references that arrive after the bytes.
func (w *inbWalker) ensureLinks(fileDBID string, links []types.INBFileLink) error {
	if len(links) == 0 || w.options.DryRun {
		return nil
	}
	fileReg, err := w.proc.factorySet.FileRegistryFactory.CreateUserRegistry(w.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user file registry", err)
	}
	for _, l := range links {
		if l.Type == "" {
			continue
		}
		if _, _, err := fileReg.AddLink(w.ctx, models.FileLink{FileID: fileDBID, EntityType: l.Type, EntityID: l.DBID, Meta: l.Meta}); err != nil {
			return errxtrace.Wrap("failed to restore file link", err, errx.Attrs("file_id", fileDBID))
		}
	}
	return nil
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
	return w.registerEntityFileRefs(doc.Files)
}

// registerEntityFileRefs indexes the non-commodity file references by their
// archive path (issue #2235). Each ref carries its own link, so the later file
// member recreates the row with the original linked_entity_type / meta instead of
// assuming a commodity attachment.
func (w *inbWalker) registerEntityFileRefs(refs []types.INBEntityFileRef) error {
	for _, ref := range refs {
		link := inbFileLink{
			linkedType: ref.LinkedEntityType,
			entityUUID: ref.LinkedEntityID,
			meta:       ref.LinkedEntityMeta,
			fileType:   ref.Type,
			category:   ref.Category,
		}
		if err := w.registerRef(ref.INBFileRef, link); err != nil {
			return err
		}
	}
	return nil
}

// trashedItemID returns the DB id of the item in Trash the link points at, or
// "" when it points elsewhere. A merge leaves that item alone and keeps the
// link to it.
func (w *inbWalker) trashedItemID(link inbFileLink) string {
	if link.linkedType != models.FileLinkCommodity {
		return ""
	}
	return w.existing.TrashedCommodities[link.entityUUID]
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
	case models.FileLinkDraft:
		dbID, found = w.draftIDs[link.entityUUID]
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
		// would create an orphan blob with no owning row. It still counts as
		// delivered: the document that referenced it first may have failed
		// (an item that did not validate, or any item in a dry run), and a
		// later reference to the same file must not fail the whole restore
		// as a missing member.
		_, _ = io.Copy(io.Discard, r)
		w.delivered[hdr.Name] = ""
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
	w.delivered[hdr.Name] = pending.ref.ID

	// Resolve every archived link (commodity / location / area) to a
	// destination DB id. A link whose entity never landed is dropped with a
	// counted error — a dangling link must never be persisted. A file whose
	// links all failed to resolve is dropped; a standalone file has no link to
	// resolve and always lands.
	links, standalone, holderID := w.resolveLinks(pending)
	if !standalone && len(links) == 0 {
		_, _ = io.Copy(io.Discard, r)
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
	fileEntity, err := pending.ref.ConvertToFileEntity(links, blobKey)
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
	if w.existing.TrashedFiles[pending.ref.ID] {
		// The file is in Trash: a merge leaves it there.
		return w.skipFileMember(r, hdr.Size, blobKey)
	}
	action := decideFileStrategyAction(w.options.Strategy, w.existing.Files[pending.ref.ID])

	if action == fileActionSkip {
		if err := w.skipFileMember(r, hdr.Size, blobKey); err != nil {
			return err
		}
		// The file is already here; a merge still brings over the links the
		// archive has and the database lacks.
		if existingFile := w.existing.Files[pending.ref.ID]; existingFile != nil {
			if err := w.ensureLinks(existingFile.ID, links); err != nil {
				return err
			}
		}
		return nil
	}

	written, checksum, err := w.streamFileBytes(blobKey, r, hdr.Size)
	if err != nil {
		return err
	}
	w.stats.BinaryDataSize += written

	// Record the restored byte count so per-group storage-usage accounting
	// (#1388) isn't under-counted — `.inb` is the first format to round-trip
	// commodity file bytes, so this is the authoritative size on restore. The
	// checksum is computed from the same stream.
	if fileEntity.File != nil {
		fileEntity.File.SizeBytes = written
		fileEntity.File.SHA256 = checksum
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
	if err := w.finishFileRow(action, pending.ref.ID, fileEntity.ID, links, holderID); err != nil {
		return err
	}

	w.deleteSupersededBlob(staleBlobKey, blobKey)

	for _, link := range pending.links {
		w.incBucketStat(link, pending.ref.Tags)
	}
	return nil
}

// finishFileRow completes a file row the strategy just wrote. An updated row
// gets the links it lacks (an update leaves links alone); a created row whose
// only links point at items in Trash goes to Trash with holderID.
func (w *inbWalker) finishFileRow(action fileAction, fileUUID, fileID string, links []types.INBFileLink, holderID string) error {
	switch action {
	case fileActionUpdate:
		return w.ensureLinks(fileID, links)
	case fileActionCreate:
		fileDBID := w.idMapping.Files[fileUUID]
		if holderID == "" || fileDBID == "" {
			return nil
		}
		if err := w.proc.parkFileInTrash(w.ctx, fileDBID, holderID); err != nil {
			return err
		}
		w.parked[fileUUID] = true
	case fileActionSkip:
	}
	return nil
}

// resolveLinks maps a pending file's archived links to destination links.
// standalone is true for a file archived without any link. Unmapped links are
// counted as errors and left out. A link to an item in Trash is kept; when
// every link is one, holderID is the first such item, which the new file goes
// to Trash with.
func (w *inbWalker) resolveLinks(pending *inbPendingFile) (links []types.INBFileLink, standalone bool, holderID string) {
	live := false
	for _, link := range pending.links {
		if link.linkedType == "" || w.isUnrestoredDraft(link) {
			// A file only drafts the restore did not recreate used is
			// restored as a standalone file rather than lost.
			standalone = true
			continue
		}
		dbID, ok := w.resolveLinkedDBID(link)
		if ok {
			live = true
		} else if dbID = w.trashedItemID(link); dbID != "" {
			if holderID == "" {
				holderID = dbID
			}
		} else {
			w.stats.ErrorCount++
			w.stats.Errors = append(w.stats.Errors,
				fmt.Sprintf("file %s references unmapped %s %s", pending.ref.ID, link.linkedType, link.entityUUID))
			continue
		}
		links = append(links, types.INBFileLink{
			Type:     link.linkedType,
			DBID:     dbID,
			Meta:     link.meta,
			FileType: link.fileType,
			Category: link.category,
		})
	}
	if len(links) > 0 {
		standalone = false
	}
	if live {
		holderID = ""
	}
	return links, standalone, holderID
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
//
// SHARED-KEY GUARD (#2250). staleBlobKey is whatever key the EXISTING row
// carried, and for a pre-#2241 row that is `<sanitized-name>-<unix SECONDS><ext>`
// — a key another row can legitimately share (two members of different groups
// who uploaded the same filename in one second). The new key is UUID-minted, so
// `stale != new` is the NORMAL case for such a row and this delete always fires.
// Without the guard, restoring one file destroys the other, still-live file's
// bytes: `files` has no soft-delete and there is no trash.
//
// The guard lives in FileService for the delete paths that go through it; this
// path does not, so it must ask the same question itself. Fail closed on any
// doubt — an unswept blob is recoverable, a destroyed one is not.
//
// NO row is exempt from the check, and that is load-bearing: this runs AFTER
// applyStrategyForFileModel has already re-pointed our row to newBlobKey, so
// anything still holding staleBlobKey is by definition somebody else. (Asking
// before the re-point would be useless — our own row would answer every time.
// Excluding our row by id would be worse than useless: it reads as if it
// protects something while matching nothing.)
func (w *inbWalker) deleteSupersededBlob(staleBlobKey, newBlobKey string) {
	if staleBlobKey == "" || staleBlobKey == newBlobKey || w.bucket == nil {
		return
	}

	shared, err := w.proc.fileService().BlobSharedByOtherRows(w.ctx, staleBlobKey, "")
	if err != nil {
		slog.WarnContext(w.ctx, "keeping superseded file blob: could not establish sole ownership (#2250)",
			"stale_blob_key", staleBlobKey, "new_blob_key", newBlobKey, "error", err.Error())
		return
	}
	if shared {
		slog.WarnContext(w.ctx, "keeping superseded file blob: another file row still references it (#2250)",
			"stale_blob_key", staleBlobKey, "new_blob_key", newBlobKey)
		return
	}

	if err := w.bucket.Delete(w.ctx, staleBlobKey); err != nil {
		slog.WarnContext(w.ctx, "failed to delete superseded file blob after merge-update restore (best-effort)",
			"stale_blob_key", staleBlobKey, "new_blob_key", newBlobKey, "error", err.Error())
	}
}

// streamFileBytes copies a file member's bytes into the destination blob key,
// without buffering, and returns the byte count and the SHA-256 of the bytes.
// When no bucket is configured (DryRun or no upload location) it drains the
// bytes so the byte count still flows into stats.
func (w *inbWalker) streamFileBytes(blobKey string, r io.Reader, size int64) (int64, string, error) {
	hasher := sha256.New()
	src := io.TeeReader(io.LimitReader(r, size), hasher)
	if w.bucket == nil {
		n, err := io.Copy(io.Discard, src)
		return n, hex.EncodeToString(hasher.Sum(nil)), err
	}

	writer, err := w.bucket.NewWriter(w.ctx, blobKey, nil)
	if err != nil {
		return 0, "", errxtrace.Wrap("failed to create blob writer", err, errx.Attrs("blob_key", blobKey))
	}
	n, copyErr := io.Copy(writer, src)
	closeErr := writer.Close()
	if copyErr != nil {
		return 0, "", errxtrace.Wrap("failed to stream file bytes", copyErr, errx.Attrs("blob_key", blobKey))
	}
	if closeErr != nil {
		return 0, "", errxtrace.Wrap("failed to close blob writer", closeErr, errx.Attrs("blob_key", blobKey))
	}
	return n, hex.EncodeToString(hasher.Sum(nil)), nil
}

// incBucketStat increments the per-bucket file counter. ImageCount/InvoiceCount/
// ManualCount are COMMODITY-scoped counters, so a location/area file (whose
// buckets are images/files) or a standalone file must NOT inflate them — those
// only feed the unified FileCount (issue #2235).
//
// Invoices and manuals no longer have buckets of their own; both live in
// `documents` and the kind is a tag (#1989). The two counters stay because the
// exports screen shows them, so they are derived from the tags the same way the
// export side derives them.
func (w *inbWalker) incBucketStat(link inbFileLink, tags []string) {
	if link.linkedType != models.FileLinkCommodity {
		return
	}
	switch link.meta {
	case "images":
		w.stats.ImageCount++
	case "documents":
		if slices.Contains(tags, models.FileTagInvoice) {
			w.stats.InvoiceCount++
		}
		if slices.Contains(tags, models.FileTagManual) {
			w.stats.ManualCount++
		}
	}
}
