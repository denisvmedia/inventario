//go:build !legacy_xml_backup

package export

import (
	"archive/tar"
	"context"
	"fmt"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"github.com/denisvmedia/inventario/backup/export/types"
	"github.com/denisvmedia/inventario/internal/blobkeys"
	"github.com/denisvmedia/inventario/internal/textutils"
	"github.com/denisvmedia/inventario/models"
)

// inbBuilder accumulates the per-location JSON documents + their commodity file
// members into the inner tar while collecting export statistics.
//
// The whole in-scope tree (locations, areas, commodities, commodity-attached
// files) is preloaded ONCE up front (preload) and indexed by parent ID, so the
// plan/write passes never re-`List()` a registry inside a loop (no N+1). Only
// metadata is buffered; file bytes are streamed straight from blob storage.
type inbBuilder struct {
	svc    *ExportService
	ctx    context.Context
	export models.Export
	tenant string
	bucket *blob.Bucket
	tw     *tar.Writer
	stats  *types.ExportStats

	// Indexes built once by preload — every per-location/area/commodity/file
	// lookup in the plan/write passes reads from these (O(1)), never from a
	// registry List().
	locations           []*models.Location
	areasByLocationID   map[string][]*models.Area
	commoditiesByAreaID map[string][]*models.Commodity
	// unassignedCommodities are the area-less commodities (issue #1986),
	// collected during preload. They have no location/area home in the
	// location-centric layout, so they are emitted in a dedicated top-level
	// member instead (see planUnassigned / INBUnassignedDoc).
	unassignedCommodities []*models.Commodity
	// filesByCommodityID maps a commodity DB ID → bucket (images/invoices/
	// manuals) → its in-scope, size-resolved candidate files (orphans already
	// dropped).
	filesByCommodityID map[string]map[string][]*candidateFile
	// fileUUIDByDBID maps a file's volatile DB id → its immutable UUID, for
	// every commodity-attached file (orphans included — this is metadata only,
	// used to resolve a commodity's cover_file_id DB key to the stable UUID
	// written into the archive). #534 / #1451.
	fileUUIDByDBID map[string]string

	// locUUIDByDBID / areaUUIDByDBID resolve a file's volatile
	// linked_entity_id (a DB key) to the linked entity's IMMUTABLE UUID, which
	// is the only identity that round-trips across databases (issue #2235). A
	// file whose linked id resolves to nothing (dangling link, out of RLS
	// scope) is dropped rather than emitted with a reference restore cannot
	// map.
	locUUIDByDBID  map[string]string
	areaUUIDByDBID map[string]string
	// entityFiles / standaloneFiles are the NON-commodity files (issue #2235),
	// size-resolved and in list order (orphans already dropped). They are
	// emitted in the dedicated files member, never nested under a commodity.
	entityFiles     []*entityCandidateFile
	standaloneFiles []*entityCandidateFile
}

// candidateFile is a commodity-attached file whose size has already been
// resolved (recorded SizeBytes or a one-time bucket probe). Orphans (missing or
// unsized blobs) are never turned into a candidateFile, so they are excluded
// from both the doc and the statistics.
type candidateFile struct {
	file *models.FileEntity
	size int64
}

// entityCandidateFile is a size-resolved NON-commodity file (issue #2235): one
// linked to a location or an area, or a standalone (unlinked) file. linkedType is
// "location", "area", or "" for standalone; entityUUID is the linked entity's
// IMMUTABLE UUID (empty for standalone), already resolved from the row's volatile
// linked_entity_id.
type entityCandidateFile struct {
	candidateFile
	linkedType string
	entityUUID string
}

// pendingFile records a commodity file whose bytes must be streamed into the tar
// AFTER its location's JSON member is written. The exporter writes the location
// JSON first so the restore side can register the file reference before the
// bytes arrive (the restore matches a file member to a known reference).
type pendingFile struct {
	archivePath string
	blobKey     string
	size        int64
	bucket      string // images / invoices / manuals (for stats)
}

// plannedLocation is the in-memory plan for one location produced by Pass 1: its
// fully-built JSON document (metadata only — never file bytes), the in-archive
// member name, and the ordered list of file bytes to stream right after the JSON
// member in Pass 2.
type plannedLocation struct {
	member  string
	doc     INBLocationDoc
	pending []pendingFile
}

// unassignedFileSlug is the synthetic archive-path slug used for the file
// members of area-less commodities (issue #1986), standing in for the parent
// location's cleaned name that these commodities lack.
const unassignedFileSlug = "unassigned"

// plannedUnassigned is the in-memory plan for the area-less commodities member:
// its built JSON document (metadata only) plus the ordered file bytes to stream
// after it. present is false when no unassigned commodity is in scope, so the
// member is omitted entirely (archive byte-stability — see INBUnassignedName).
type plannedUnassigned struct {
	present bool
	doc     INBUnassignedDoc
	pending []pendingFile
}

// plannedFiles is the in-memory plan for the non-commodity files member (issue
// #2235): the JSON document (metadata only) plus the ordered file bytes to
// stream after it. present is false when no location-/area-linked or standalone
// file is in scope, so the member is omitted entirely (archive byte-stability —
// see INBFilesName).
type plannedFiles struct {
	present bool
	doc     INBFilesDoc
	pending []pendingFile
}

// inbScope captures the selection filter for an export. For whole-class export
// types every in-scope entity is included; for ExportTypeSelectedItems the
// allow-sets restrict which locations/areas/commodities round-trip.
//
// The `.inb` format is location-centric: every commodity is emitted under its
// location's document. An "areas" or "commodities" export therefore still walks
// locations, but only emits the areas/commodities the user asked for (and the
// locations that contain them).
type inbScope struct {
	// wholeClass marks one of the four whole-class export types (as opposed to
	// selected_items). Standalone files have no parent entity that could imply
	// them into a selection, so they ride along with a whole-class export only —
	// mirroring the legacy XML exporter's rule (issue #2235).
	wholeClass bool

	// allLocations/allAreas/allCommodities mean "no per-entity filter for
	// this class" — include every row of that class that survives the other
	// filters.
	allLocations   bool
	allAreas       bool
	allCommodities bool

	// locationIDs/areaIDs/commodityIDs are DB-ID allow-sets used when the
	// matching all* flag is false (selected_items, and the whole-class
	// exports that restrict a single class).
	locationIDs  map[string]bool
	areaIDs      map[string]bool
	commodityIDs map[string]bool
}

// resolveScope builds the inbScope for the export type. full_database includes
// everything; locations/areas/commodities include the whole class plus the
// containing parents; selected_items restricts to the explicitly chosen rows.
func (b *inbBuilder) resolveScope() (*inbScope, error) {
	switch b.export.Type {
	case models.ExportTypeFullDatabase,
		models.ExportTypeLocations,
		models.ExportTypeAreas,
		models.ExportTypeCommodities:
		// All four whole-class exports emit the full location → area →
		// commodity tree. The legacy XML exporter also streamed the entire
		// group-wide file set for each of these, so matching that keeps the
		// formats behaviourally equivalent. The per-type distinction only
		// mattered for the flat XML sectioning, which the location-centric
		// JSON format collapses.
		return &inbScope{wholeClass: true, allLocations: true, allAreas: true, allCommodities: true}, nil
	case models.ExportTypeSelectedItems:
		return b.resolveSelectedScope()
	default:
		return nil, errxtrace.ClassifyNew("unsupported export type", nil)
	}
}

// resolveSelectedScope builds the allow-sets for a selected_items export from
// the export's SelectedItems list.
func (b *inbBuilder) resolveSelectedScope() (*inbScope, error) {
	scope := &inbScope{
		locationIDs:  map[string]bool{},
		areaIDs:      map[string]bool{},
		commodityIDs: map[string]bool{},
	}
	for _, item := range b.export.SelectedItems {
		switch item.Type {
		case models.ExportSelectedItemTypeLocation:
			scope.locationIDs[item.ID] = true
		case models.ExportSelectedItemTypeArea:
			scope.areaIDs[item.ID] = true
		case models.ExportSelectedItemTypeCommodity:
			scope.commodityIDs[item.ID] = true
		}
	}
	return scope, nil
}

// preload lists locations, areas, commodities, and the commodity-attached files
// ONCE (RLS-scoped user registries — never CreateServiceRegistry) and builds the
// parent-keyed indexes the plan/write passes read from. Each candidate file's
// size is resolved here (recorded SizeBytes else a single bucket probe); orphans
// are dropped so they never reach the doc or the statistics.
func (b *inbBuilder) preload() error {
	locations, err := b.loadLocations()
	if err != nil {
		return err
	}
	b.locations = locations

	// A file's linked_entity_id is a DB key; the archive may only carry the
	// immutable UUID (issue #2235), so index both directions up front.
	b.locUUIDByDBID = make(map[string]string, len(locations))
	for _, loc := range locations {
		b.locUUIDByDBID[loc.ID] = loc.UUID
	}

	areas, err := b.loadAreas()
	if err != nil {
		return err
	}
	b.areasByLocationID = make(map[string][]*models.Area, len(areas))
	b.areaUUIDByDBID = make(map[string]string, len(areas))
	for _, area := range areas {
		b.areasByLocationID[area.LocationID] = append(b.areasByLocationID[area.LocationID], area)
		b.areaUUIDByDBID[area.ID] = area.UUID
	}

	commodities, err := b.loadCommodities()
	if err != nil {
		return err
	}
	b.commoditiesByAreaID = make(map[string][]*models.Commodity, len(commodities))
	for _, com := range commodities {
		// Area is optional (issue #1986): only index commodities that have an
		// area under it; collect the area-less ones for the unassigned member.
		if com.AreaID != nil && *com.AreaID != "" {
			b.commoditiesByAreaID[*com.AreaID] = append(b.commoditiesByAreaID[*com.AreaID], com)
		} else {
			b.unassignedCommodities = append(b.unassignedCommodities, com)
		}
	}

	if err := b.indexFiles(); err != nil {
		return err
	}
	return nil
}

// loadLocations lists every location once via the RLS-scoped user registry.
func (b *inbBuilder) loadLocations() ([]*models.Location, error) {
	locReg, err := b.svc.factorySet.LocationRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create location registry", err)
	}
	locations, err := locReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list locations", err)
	}
	return locations, nil
}

// loadAreas lists every area once via the RLS-scoped user registry.
func (b *inbBuilder) loadAreas() ([]*models.Area, error) {
	areaReg, err := b.svc.factorySet.AreaRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create area registry", err)
	}
	areas, err := areaReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list areas", err)
	}
	return areas, nil
}

// loadCommodities lists every commodity once via the RLS-scoped user registry.
func (b *inbBuilder) loadCommodities() ([]*models.Commodity, error) {
	comReg, err := b.svc.factorySet.CommodityRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity registry", err)
	}
	commodities, err := comReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities", err)
	}
	return commodities, nil
}

// indexFiles lists the group's files ONCE and routes every row to its home:
// commodity attachments are grouped by commodity DB ID and bucket, while
// location-/area-linked and standalone files are collected for the dedicated
// files member (issue #2235). Export artifacts (linked_entity_type "export") are
// skipped — a backup must never embed previous backups.
//
// Orphan rows (missing/unsized blob) are dropped in EVERY branch, so they never
// reach a document or the statistics. That drop is mandatory: the restore side
// hard-fails when a declared file reference has no member in the archive.
func (b *inbBuilder) indexFiles() error {
	fileReg, err := b.svc.factorySet.FileRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}
	files, err := fileReg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list files", err)
	}

	b.filesByCommodityID = map[string]map[string][]*candidateFile{}
	b.fileUUIDByDBID = make(map[string]string, len(files))
	for _, file := range files {
		if file == nil || file.File == nil {
			continue
		}
		// A blob probe that cannot determine a file's state aborts the export:
		// an incomplete-but-successful backup is worse than a failed one.
		var indexErr error
		switch file.LinkedEntityType {
		case "commodity":
			indexErr = b.indexCommodityFile(file)
		case "location", "area":
			indexErr = b.indexEntityFile(file)
		case "":
			indexErr = b.indexStandaloneFile(file)
		default:
			// "export" (and any future type this build does not know) is not
			// user inventory — skip it.
			continue
		}
		if indexErr != nil {
			return indexErr
		}
	}
	return nil
}

// indexCommodityFile files one commodity attachment under its commodity + bucket.
// A bucket outside images/invoices/manuals cannot pass model validation, so such
// a row is skipped rather than emitted.
func (b *inbBuilder) indexCommodityFile(file *models.FileEntity) error {
	bucket := file.LinkedEntityMeta
	if !isCommodityFileBucket(bucket) {
		return nil
	}

	// Record the DB-id → UUID mapping for EVERY commodity-attached file,
	// before the orphan-size filter below: the cover-file cross-reference
	// only needs the stable UUID, and resolving it for a file whose blob
	// later proves missing is harmless (the restore drops the cover if its
	// target file never lands).
	b.fileUUIDByDBID[file.ID] = file.UUID

	size, present, err := b.resolveFileSize(file)
	if err != nil {
		return err
	}
	if !present {
		// A confirmed-missing/unsized blob (orphan row, manual delete) is dropped
		// so it is excluded from both the doc and the statistics.
		return nil
	}

	byBucket := b.filesByCommodityID[file.LinkedEntityID]
	if byBucket == nil {
		byBucket = map[string][]*candidateFile{}
		b.filesByCommodityID[file.LinkedEntityID] = byBucket
	}
	byBucket[bucket] = append(byBucket[bucket], &candidateFile{file: file, size: size})
	return nil
}

// indexEntityFile collects one location-/area-linked file (issue #2235),
// resolving the row's volatile linked_entity_id to the linked entity's immutable
// UUID. A file whose linked entity does not resolve (dangling link, out of RLS
// scope) or whose bucket is outside images/files is skipped rather than emitted
// with a reference the restore side could not map.
func (b *inbBuilder) indexEntityFile(file *models.FileEntity) error {
	if !isEntityFileBucket(file.LinkedEntityMeta) {
		return nil
	}

	var entityUUID string
	switch file.LinkedEntityType {
	case "location":
		entityUUID = b.locUUIDByDBID[file.LinkedEntityID]
	case "area":
		entityUUID = b.areaUUIDByDBID[file.LinkedEntityID]
	}
	if entityUUID == "" {
		return nil
	}

	size, present, err := b.resolveFileSize(file)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	b.entityFiles = append(b.entityFiles, &entityCandidateFile{
		candidateFile: candidateFile{file: file, size: size},
		linkedType:    file.LinkedEntityType,
		entityUUID:    entityUUID,
	})
	return nil
}

// indexStandaloneFile collects one unlinked file (issue #2235). linkedType and
// entityUUID stay empty — model validation requires all three link fields empty
// for a standalone row.
func (b *inbBuilder) indexStandaloneFile(file *models.FileEntity) error {
	size, present, err := b.resolveFileSize(file)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	b.standaloneFiles = append(b.standaloneFiles, &entityCandidateFile{
		candidateFile: candidateFile{file: file, size: size},
	})
	return nil
}

// resolveFileSize returns the byte size to declare for a file member: the
// recorded SizeBytes when >0, else a single bucket attributes probe.
//
// present=false means the blob is CONFIRMED absent (an orphan row: a manual
// delete, a seed fixture that never wrote bytes) — the caller drops the file from
// both the document and the statistics. Dropping it here is deliberate: a
// declared member whose bytes never land would abort the WHOLE export in
// flushPendingFile (the streaming exporter cannot recover once the document
// referencing the member has been emitted), and would hard-fail every restore of
// the resulting archive with ErrMissingFileMembers.
//
// A non-nil error means the probe could NOT determine the blob's state (a
// transient bucket failure: network, throttling, a 5xx). That must NOT be
// conflated with "absent": treating it as an orphan would silently drop a live
// file and hand the user a backup that looks successful but is incomplete —
// exactly the failure class this format change exists to eliminate. So it fails
// the export instead.
func (b *inbBuilder) resolveFileSize(file *models.FileEntity) (size int64, present bool, err error) {
	if hint := fileSizeHint(file); hint > 0 {
		// Trust the recorded size for the tar header, but still confirm the blob
		// actually exists before committing the file to the plan. Exists reports
		// (false, nil) only when the blob is definitively missing; any error means
		// the state is unknown, so it propagates.
		exists, err := b.bucket.Exists(b.ctx, file.OriginalPath)
		if err != nil {
			return 0, false, errxtrace.Wrap("failed to probe file blob", err,
				errx.Attrs("file_id", file.ID, "blob_key", file.OriginalPath))
		}
		if !exists {
			return 0, false, nil
		}
		return hint, true, nil
	}
	// No usable size hint — probe the bucket for the exact size, which the tar
	// header needs. Only a NotFound is an orphan; every other failure is unknown
	// state and fails the export.
	attrs, err := b.bucket.Attributes(b.ctx, file.OriginalPath)
	if err != nil {
		if gcerrors.Code(err) == gcerrors.NotFound {
			return 0, false, nil
		}
		return 0, false, errxtrace.Wrap("failed to read file blob attributes", err,
			errx.Attrs("file_id", file.ID, "blob_key", file.OriginalPath))
	}
	// A ZERO size is a real file, not an orphan: the upload path stores whatever
	// the multipart part carried and never rejects an empty one, so a 0-byte row
	// is legitimate — and tar members may be zero-length. Dropping it here would
	// silently omit the file from the backup and lose the row on a full_replace
	// restore. Only a negative size is impossible; that is a corrupt attribute
	// and would produce an invalid tar header, so it fails the export.
	if attrs.Size < 0 {
		return 0, false, errxtrace.Wrap("blob reported a negative size",
			errx.NewSentinel("corrupt blob attributes"),
			errx.Attrs("file_id", file.ID, "blob_key", file.OriginalPath, "size", attrs.Size))
	}
	return attrs.Size, true, nil
}

// inScopeLocations returns the locations to emit. For selected_items it also
// includes the locations that own a selected area or commodity, so a selected
// commodity's location document exists to hang it under. Reads only from the
// preloaded indexes — no registry List().
func (b *inbBuilder) inScopeLocations(scope *inbScope) []*models.Location {
	if scope.allLocations {
		return b.locations
	}

	// selected_items: a location is in scope if it was explicitly selected
	// OR it owns a selected area/commodity.
	implied := b.impliedLocationIDs(scope)
	var filtered []*models.Location
	for _, loc := range b.locations {
		if scope.locationIDs[loc.ID] || implied[loc.ID] {
			filtered = append(filtered, loc)
		}
	}
	return filtered
}

// impliedLocationIDs computes the set of location DB IDs that own a selected
// area or commodity, so their documents are emitted even when the location
// itself was not directly selected. Reads only from the preloaded indexes.
func (b *inbBuilder) impliedLocationIDs(scope *inbScope) map[string]bool {
	implied := map[string]bool{}
	for locID, areas := range b.areasByLocationID {
		for _, area := range areas {
			if scope.areaIDs[area.ID] {
				implied[locID] = true
			}
			for _, com := range b.commoditiesByAreaID[area.ID] {
				if scope.commodityIDs[com.ID] {
					implied[locID] = true
				}
			}
		}
	}
	return implied
}

// areasForLocation returns the in-scope areas of a location from the preloaded
// index. An area is in scope if the class is unfiltered, it was selected, or it
// hosts a selected commodity.
func (b *inbBuilder) areasForLocation(loc *models.Location, scope *inbScope) []*models.Area {
	var result []*models.Area
	for _, area := range b.areasByLocationID[loc.ID] {
		if scope.allAreas || scope.areaIDs[area.ID] || b.areaHasSelectedCommodity(area, scope) {
			result = append(result, area)
		}
	}
	return result
}

// areaHasSelectedCommodity reports whether a selected_items export picked a
// commodity that lives in this area (so the area must be emitted to host it).
// Pure index lookup — no registry call.
func (b *inbBuilder) areaHasSelectedCommodity(area *models.Area, scope *inbScope) bool {
	if len(scope.commodityIDs) == 0 {
		return false
	}
	for _, com := range b.commoditiesByAreaID[area.ID] {
		if scope.commodityIDs[com.ID] {
			return true
		}
	}
	return false
}

// commoditiesForArea returns the in-scope commodities of an area from the
// preloaded index.
func (b *inbBuilder) commoditiesForArea(area *models.Area, scope *inbScope) []*models.Commodity {
	var out []*models.Commodity
	for _, com := range b.commoditiesByAreaID[area.ID] {
		if scope.allCommodities || scope.commodityIDs[com.ID] {
			out = append(out, com)
		}
	}
	return out
}

// unassignedInScope returns the area-less commodities to emit (issue #1986). For
// whole-class exports every unassigned commodity is included; for selected_items
// only those whose UUID was explicitly selected round-trip. (A location/area
// selection never implies an unassigned commodity — it has no parent to imply
// it.)
func (b *inbBuilder) unassignedInScope(scope *inbScope) []*models.Commodity {
	if scope.allCommodities {
		return b.unassignedCommodities
	}
	var out []*models.Commodity
	for _, com := range b.unassignedCommodities {
		if scope.commodityIDs[com.ID] {
			out = append(out, com)
		}
	}
	return out
}

// planUnassigned builds the in-memory plan for the area-less commodities member
// (issue #1986): the flat JSON document plus the ordered pending-file list, with
// every commodity's file/bucket statistics accumulated. Returns present=false
// (and writes nothing) when no unassigned commodity is in scope, so an archive
// with none stays byte-stable.
func (b *inbBuilder) planUnassigned(scope *inbScope) plannedUnassigned {
	commodities := b.unassignedInScope(scope)
	if len(commodities) == 0 {
		return plannedUnassigned{present: false}
	}

	var doc INBUnassignedDoc
	var pending []pendingFile
	for _, com := range commodities {
		// areaUUID "" marks the commodity as area-less; the synthetic slug
		// gives its file members a stable archive home.
		inbCom, comPending := b.planCommodity(unassignedFileSlug, "", com)
		doc.Commodities = append(doc.Commodities, inbCom)
		pending = append(pending, comPending...)
		b.stats.CommodityCount++
	}
	return plannedUnassigned{present: true, doc: doc, pending: pending}
}

// planFiles builds the in-memory plan for the non-commodity files member (issue
// #2235): every location-/area-linked and standalone file in scope, each carrying
// its own polymorphic link so restore recreates the row with the original
// linked_entity_type / linked_entity_id / linked_entity_meta.
//
// Scope (legacy XML parity): a whole-class export carries every entity file plus
// the standalone files; a selected_items export carries only the files of the
// locations/areas the user EXPLICITLY selected, and NO standalone files (they have
// no parent that could imply them into the selection). See entityFileInScope.
//
// Statistics: these files bump FileCount + BinaryDataSize only. ImageCount /
// InvoiceCount / ManualCount are legacy COMMODITY-scoped counters (see
// export/types.ExportStats), so a location image must not inflate imageCount.
func (b *inbBuilder) planFiles(scope *inbScope) plannedFiles {
	var doc INBFilesDoc
	var pending []pendingFile

	appendCandidate := func(cand *entityCandidateFile) {
		ref, pf := b.buildEntityFileRef(cand)
		doc.Files = append(doc.Files, ref)
		pending = append(pending, pf)
		b.stats.FileCount++
		b.stats.BinaryDataSize += cand.size
	}

	for _, cand := range b.entityFiles {
		if !b.entityFileInScope(cand, scope) {
			continue
		}
		appendCandidate(cand)
	}
	if scope.wholeClass {
		for _, cand := range b.standaloneFiles {
			appendCandidate(cand)
		}
	}

	if len(doc.Files) == 0 {
		return plannedFiles{present: false}
	}
	return plannedFiles{present: true, doc: doc, pending: pending}
}

// entityFileInScope reports whether a location-/area-linked file rides along with
// this export: always for a whole-class export, and for selected_items only when
// the user EXPLICITLY selected that location/area.
//
// The explicit-only rule is the legacy XML exporter's rule (selectedFileScope is
// built from the selected item IDs — see service_legacy_xml.go), and the one the
// user-facing docs promise ("files attached to a location or area you selected come
// with it"). Scoping on "was the parent EMITTED" instead would silently bundle the
// files of a parent that is only in the archive because it hosts a selected
// commodity — e.g. selecting one item would leak the location's lease and floor
// plans into a shared archive.
//
// An explicitly selected location/area is always emitted (inScopeLocations /
// areasForLocation), so its files can never reference a missing parent document.
func (*inbBuilder) entityFileInScope(cand *entityCandidateFile, scope *inbScope) bool {
	if scope.wholeClass {
		return true
	}
	// The row's linked_entity_id is a DB key, and so are the selection sets.
	switch cand.linkedType {
	case "location":
		return scope.locationIDs[cand.file.LinkedEntityID]
	case "area":
		return scope.areaIDs[cand.file.LinkedEntityID]
	}
	return false
}

// buildEntityFileRef builds a non-commodity file reference + its pending stream
// entry (issue #2235). The archive member keeps a <file-uuid> segment for the
// same anti-collision reason as the commodity layout (two files of one entity can
// share a basename), and the whole path goes through SanitizeArchivePath — the
// restore side rejects any member whose sanitized form differs from its raw name.
func (b *inbBuilder) buildEntityFileRef(cand *entityCandidateFile) (INBEntityFileRef, pendingFile) {
	file := cand.file
	name := blobkeys.SanitizeArchivePath(fileMemberName(file))

	var archivePath string
	if cand.linkedType == "" {
		archivePath = fmt.Sprintf("%s%s/%s", INBStandaloneFilesPrefix, file.UUID, name)
	} else {
		archivePath = fmt.Sprintf("%s%s/%s/%s/%s/%s",
			INBEntityFilesPrefix, cand.linkedType, cand.entityUUID, file.LinkedEntityMeta, file.UUID, name)
	}
	archivePath = blobkeys.SanitizeArchivePath(archivePath)

	ref := INBEntityFileRef{
		INBFileRef: INBFileRef{
			ID:           file.UUID,
			Path:         archivePath,
			Name:         file.Path,
			OriginalPath: file.OriginalPath,
			Extension:    file.Ext,
			MimeType:     file.MIMEType,
			Title:        file.Title,
			Description:  file.Description,
			Tags:         []string(file.Tags),
			CreatedAt:    file.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:    file.UpdatedAt.UTC().Format(time.RFC3339),
		},
		LinkedEntityType: cand.linkedType,
		LinkedEntityID:   cand.entityUUID,
		LinkedEntityMeta: file.LinkedEntityMeta,
		Type:             string(file.Type),
		Category:         string(file.Category),
	}
	// A standalone file must carry NO link at all — model validation requires
	// linked_entity_type/id/meta to be empty together.
	if cand.linkedType == "" {
		ref.LinkedEntityMeta = ""
	}

	pf := pendingFile{archivePath: archivePath, blobKey: file.OriginalPath, size: cand.size}
	return ref, pf
}

// planLocation builds the in-memory plan for one location: its JSON document
// (metadata only) plus the ordered pending-file list (sizes already resolved),
// and accumulates ALL statistics for the location's subtree. No DB/List and no
// file bytes touch the heap here.
func (b *inbBuilder) planLocation(loc *models.Location, scope *inbScope) plannedLocation {
	doc := INBLocationDoc{
		Location: INBLocation{ID: loc.UUID, Name: loc.Name, Address: loc.Address},
	}
	b.stats.LocationCount++

	var pending []pendingFile
	for _, area := range b.areasForLocation(loc, scope) {
		doc.Areas = append(doc.Areas, INBArea{
			ID:         area.UUID,
			Name:       area.Name,
			LocationID: loc.UUID,
		})
		b.stats.AreaCount++

		for _, com := range b.commoditiesForArea(area, scope) {
			inbCom, comPending := b.planCommodity(textutils.CleanFilename(loc.Name), area.UUID, com)
			doc.Commodities = append(doc.Commodities, inbCom)
			pending = append(pending, comPending...)
			b.stats.CommodityCount++
		}
	}

	member := fmt.Sprintf("location-%s-%s.json", textutils.CleanFilename(loc.Name), loc.UUID)
	member = blobkeys.SanitizeArchivePath(member)
	return plannedLocation{member: member, doc: doc, pending: pending}
}

// planCommodity converts a commodity row to its INBCommodity shape and collects
// its image/invoice/manual file references from the preloaded index, recording
// each candidate as a pendingFile (size already resolved). All file/bucket stats
// are accumulated here so Pass 1 holds the full statistics before manifest.json
// is written.
//
// locSlug is the archive-path slug for the commodity's file members (the parent
// location's cleaned name, or a synthetic "unassigned" slug for area-less
// commodities). areaUUID is the parent area's immutable UUID, or "" for an
// area-less commodity (issue #1986).
func (b *inbBuilder) planCommodity(locSlug, areaUUID string, com *models.Commodity) (INBCommodity, []pendingFile) {
	inbCom := INBCommodity{
		ID:                     com.UUID,
		Name:                   com.Name,
		ShortName:              com.ShortName,
		Type:                   string(com.Type),
		AreaID:                 areaUUID,
		Count:                  com.Count,
		OriginalPrice:          decimalString(com.OriginalPrice),
		OriginalPriceCurrency:  string(com.OriginalPriceCurrency),
		ConvertedOriginalPrice: decimalString(com.ConvertedOriginalPrice),
		CurrentPrice:           decimalString(com.CurrentPrice),
		SerialNumber:           com.SerialNumber,
		ExtraSerialNumbers:     []string(com.ExtraSerialNumbers),
		PartNumbers:            []string(com.PartNumbers),
		Tags:                   []string(com.Tags),
		Status:                 string(com.Status),
		Comments:               com.Comments,
		Draft:                  com.Draft,
		URLs:                   urlStrings(com.URLs),
	}
	if com.PurchaseDate != nil {
		inbCom.PurchaseDate = string(*com.PurchaseDate)
	}
	if com.RegisteredDate != nil {
		inbCom.RegisteredDate = string(*com.RegisteredDate)
	}
	if com.LastModifiedDate != nil {
		inbCom.LastModifiedDate = string(*com.LastModifiedDate)
	}

	// Extended commodity fields (#534 round-trip parity).
	if com.WarrantyExpiresAt != nil {
		inbCom.WarrantyExpiresAt = string(*com.WarrantyExpiresAt)
	}
	inbCom.WarrantyNotes = com.WarrantyNotes
	if com.StatusDate != nil {
		inbCom.StatusDate = string(*com.StatusDate)
	}
	inbCom.StatusNote = com.StatusNote
	inbCom.SalePrice = ptrDecimalString(com.SalePrice)
	inbCom.AcquisitionPrice = ptrDecimalString(com.AcquisitionPrice)
	if com.AcquisitionCurrency != nil {
		inbCom.AcquisitionCurrency = string(*com.AcquisitionCurrency)
	}
	// Resolve the cover photo's volatile DB id to its immutable UUID so the
	// reference round-trips stably. A cover pointing at a file outside the
	// preloaded commodity-file index (already deleted, or never an attachment)
	// resolves to "" and is simply omitted — the restore then leaves the cover
	// unset and the resolver's first-photo fallback takes over.
	if com.CoverFileID != nil {
		if coverUUID := b.fileUUIDByDBID[*com.CoverFileID]; coverUUID != "" {
			inbCom.CoverFileID = coverUUID
		}
	}

	pending := b.planCommodityFiles(locSlug, com, &inbCom)
	return inbCom, pending
}

// planCommodityFiles records the commodity's image/invoice/manual file
// references on inbCom (from the preloaded, size-resolved index) and returns the
// corresponding ordered pendingFile list. Bucket/file/totalFileSize statistics
// are accumulated here. locSlug is the archive-path slug (parent location's
// cleaned name, or a synthetic "unassigned" for area-less commodities).
func (b *inbBuilder) planCommodityFiles(locSlug string, com *models.Commodity, inbCom *INBCommodity) []pendingFile {
	byBucket := b.filesByCommodityID[com.ID]
	if byBucket == nil {
		return nil
	}

	var pending []pendingFile
	// Iterate the buckets in a fixed order so the archive layout is
	// deterministic regardless of map iteration order.
	for _, bucket := range commodityFileBuckets {
		for _, cand := range byBucket[bucket] {
			ref, pf := b.buildFileRef(locSlug, com, bucket, cand)
			appendFileRef(inbCom, bucket, ref)
			pending = append(pending, pf)

			b.stats.BinaryDataSize += cand.size
			b.stats.FileCount++
			incBucketStat(b.stats, bucket)
		}
	}
	return pending
}

// buildFileRef builds a file reference + its pending stream entry from a
// size-resolved candidate file. No probing happens here — the size was resolved
// once in preload.
func (b *inbBuilder) buildFileRef(locSlug string, com *models.Commodity, bucket string, cand *candidateFile) (INBFileRef, pendingFile) {
	file := cand.file
	name := blobkeys.SanitizeArchivePath(fileMemberName(file))
	// Disambiguate by the file's immutable UUID: two files attached to the same
	// commodity bucket can share a user-facing basename (e.g. two "invoice.pdf").
	// Without the UUID segment they would collide on one archive member name and
	// restore would silently drop one file's bytes (and mis-assign the survivor's
	// metadata) while still reporting Completed. The UUID guarantees uniqueness;
	// the original filename is preserved in the ref's Name/OriginalPath.
	archivePath := blobkeys.SanitizeArchivePath(
		fmt.Sprintf("%s%s/%s/%s/%s/%s", INBFilesPrefix, locSlug, com.UUID, bucket, file.UUID, name),
	)

	ref := INBFileRef{
		ID:           file.UUID,
		Path:         archivePath,
		Name:         file.Path,
		OriginalPath: file.OriginalPath,
		Extension:    file.Ext,
		MimeType:     file.MIMEType,
		Title:        file.Title,
		Description:  file.Description,
		Tags:         []string(file.Tags),
		CreatedAt:    file.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    file.UpdatedAt.UTC().Format(time.RFC3339),
	}
	pf := pendingFile{archivePath: archivePath, blobKey: file.OriginalPath, size: cand.size, bucket: bucket}
	return ref, pf
}

// run preloads the in-scope tree once, then makes two cheap in-memory passes:
//
//	Pass 1 (plan): walk the indexes, build each location's JSON doc + ordered
//	pending-file list, and accumulate ALL statistics. The manifest location
//	index is produced here too.
//
//	Pass 2 (write): the caller writes manifest.json FIRST (stats are now known),
//	then calls writePlannedLocations to emit, per location, its JSON member
//	followed by its file bytes (streamed via io.Copy — no file bytes on the
//	heap). The JSON precedes its file bytes so restore registers each ref before
//	the member arrives.
//
// inbPlan is the full output of Pass 1: every location's plan, the manifest
// location index, and the area-less ("unassigned") commodities plan (issue
// #1986). Bundled into one struct so run returns a single value plus error
// (revive function-result-limit).
type inbPlan struct {
	locations    []plannedLocation
	manifestLocs []INBManifestLoc
	unassigned   plannedUnassigned
	files        plannedFiles
}

// run returns the Pass-1 plan; it does NOT write anything itself, so writePayload
// can interleave the manifest first.
func (b *inbBuilder) run() (inbPlan, error) {
	scope, err := b.resolveScope()
	if err != nil {
		return inbPlan{}, err
	}
	if err := b.preload(); err != nil {
		return inbPlan{}, err
	}

	locations := b.inScopeLocations(scope)
	planned := make([]plannedLocation, 0, len(locations))
	manifestLocs := make([]INBManifestLoc, 0, len(locations))
	for _, loc := range locations {
		pl := b.planLocation(loc, scope)
		planned = append(planned, pl)
		manifestLocs = append(manifestLocs, INBManifestLoc{
			ID:   loc.UUID,
			Name: loc.Name,
			File: pl.member,
		})
	}

	unassigned := b.planUnassigned(scope)

	return inbPlan{
		locations:    planned,
		manifestLocs: manifestLocs,
		unassigned:   unassigned,
		files:        b.planFiles(scope),
	}, nil
}

// writePlannedLocations performs Pass 2: for each planned location it writes the
// JSON member FIRST, then streams that location's file bytes. Called by
// writePayload AFTER manifest.json has been emitted.
func (b *inbBuilder) writePlannedLocations(planned []plannedLocation) error {
	for i := range planned {
		pl := &planned[i]
		// Location JSON first…
		if err := b.writeJSONMember(pl.member, pl.doc); err != nil {
			return err
		}
		// …then the referenced file bytes.
		for _, pf := range pl.pending {
			if err := b.flushPendingFile(pf); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeUnassigned writes the area-less commodities member (issue #1986): its
// JSON document FIRST, then the referenced file bytes — mirroring the
// location-member ordering so restore registers each file ref before its bytes
// arrive. A no-op when no unassigned commodity was in scope (present=false), so
// the member is omitted and the archive stays byte-stable.
func (b *inbBuilder) writeUnassigned(pu plannedUnassigned) error {
	if !pu.present {
		return nil
	}
	if err := b.writeJSONMember(INBUnassignedName, pu.doc); err != nil {
		return err
	}
	for _, pf := range pu.pending {
		if err := b.flushPendingFile(pf); err != nil {
			return err
		}
	}
	return nil
}

// writeFiles writes the non-commodity files member (issue #2235): its JSON
// document FIRST, then the referenced file bytes — the same JSON-before-bytes
// ordering as the location members, so restore registers each file reference
// before its bytes arrive. A no-op when no such file was in scope (present=false),
// so the member is omitted and the archive stays byte-stable.
//
// Called LAST by writePayload: the restore side resolves each ref's location/area
// link through the ID mapping, which is only populated as the location documents
// are applied.
func (b *inbBuilder) writeFiles(pf plannedFiles) error {
	if !pf.present {
		return nil
	}
	if err := b.writeJSONMember(INBFilesName, pf.doc); err != nil {
		return err
	}
	for _, f := range pf.pending {
		if err := b.flushPendingFile(f); err != nil {
			return err
		}
	}
	return nil
}

// flushPendingFile streams one collected file's bytes into the tar. The byte
// count is informational here — the statistics were already accumulated in Pass
// 1 from the resolved size, so a stream error fails the export rather than
// silently under-counting.
func (b *inbBuilder) flushPendingFile(pf pendingFile) error {
	if _, err := b.writeFileMember(pf.archivePath, pf.blobKey, pf.size); err != nil {
		return err
	}
	return nil
}

// --- small free helpers ---

// commodityFileBuckets is the fixed iteration order for a commodity's attachment
// buckets, so the archive layout is deterministic.
var commodityFileBuckets = []string{"images", "invoices", "manuals"}

// isCommodityFileBucket reports whether a linked_entity_meta value is one of the
// three commodity attachment buckets.
func isCommodityFileBucket(meta string) bool {
	switch meta {
	case "images", "invoices", "manuals":
		return true
	default:
		return false
	}
}

// isEntityFileBucket reports whether a linked_entity_meta value is one of the two
// location/area attachment buckets (issue #2235). Anything else cannot pass model
// validation for a location/area file, so such a row is dropped rather than
// exported into an archive that would fail on restore.
func isEntityFileBucket(meta string) bool {
	switch meta {
	case "images", "files":
		return true
	default:
		return false
	}
}

// appendFileRef appends a file reference to the matching bucket slice.
func appendFileRef(com *INBCommodity, bucket string, ref INBFileRef) {
	switch bucket {
	case "images":
		com.Images = append(com.Images, ref)
	case "invoices":
		com.Invoices = append(com.Invoices, ref)
	case "manuals":
		com.Manuals = append(com.Manuals, ref)
	}
}

// incBucketStat increments the per-bucket stat counter.
func incBucketStat(stats *types.ExportStats, bucket string) {
	switch bucket {
	case "images":
		stats.ImageCount++
	case "invoices":
		stats.InvoiceCount++
	case "manuals":
		stats.ManualCount++
	}
}

// fileMemberName returns the basename to use for a file inside the archive:
// its Path (user-facing name without extension) plus its extension.
func fileMemberName(file *models.FileEntity) string {
	name := file.Path
	if name == "" {
		name = file.UUID
	}
	return name + file.Ext
}

// fileSizeHint returns the size to declare in the file member's tar header. The
// tar header needs an exact size, so we trust the recorded SizeBytes; rows with
// a zero/unknown size fall back to a probe via the bucket attributes.
func fileSizeHint(file *models.FileEntity) int64 {
	if file.File != nil && file.SizeBytes > 0 {
		return file.SizeBytes
	}
	return 0
}

// decimalString renders a decimal price, returning "" for the zero value so the
// omitempty JSON fields stay clean.
func decimalString(d interface{ String() string }) string {
	s := d.String()
	if s == "0" || s == "" {
		return ""
	}
	return s
}

// ptrDecimalString renders a nullable decimal (SalePrice / AcquisitionPrice).
// Unlike decimalString it preserves a legitimate zero value: the meaningful
// distinction for these columns is nil (absent) vs set, so a non-nil pointer
// holding 0 is emitted as "0" and only a nil pointer yields "" (the omitempty
// JSON field then disappears). The restore side mirrors this: empty → nil, any
// non-empty value → a set pointer.
func ptrDecimalString(d *decimal.Decimal) string {
	if d == nil {
		return ""
	}
	return d.String()
}

// urlStrings flattens a slice of *URL to their string forms.
func urlStrings(urls models.ValuerSlice[*models.URL]) []string {
	if len(urls) == 0 {
		return nil
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if u != nil {
			out = append(out, u.String())
		}
	}
	return out
}
