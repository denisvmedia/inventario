//go:build !legacy_xml_backup

package export

import (
	"archive/tar"
	"context"
	"fmt"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

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
	// filesByCommodityID maps a commodity DB ID → bucket (images/invoices/
	// manuals) → its in-scope, size-resolved candidate files (orphans already
	// dropped).
	filesByCommodityID map[string]map[string][]*candidateFile
}

// candidateFile is a commodity-attached file whose size has already been
// resolved (recorded SizeBytes or a one-time bucket probe). Orphans (missing or
// unsized blobs) are never turned into a candidateFile, so they are excluded
// from both the doc and the statistics.
type candidateFile struct {
	file *models.FileEntity
	size int64
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

// inbScope captures the selection filter for an export. For whole-class export
// types every in-scope entity is included; for ExportTypeSelectedItems the
// allow-sets restrict which locations/areas/commodities round-trip.
//
// The `.inb` format is location-centric: every commodity is emitted under its
// location's document. An "areas" or "commodities" export therefore still walks
// locations, but only emits the areas/commodities the user asked for (and the
// locations that contain them).
type inbScope struct {
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
		return &inbScope{allLocations: true, allAreas: true, allCommodities: true}, nil
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

	areas, err := b.loadAreas()
	if err != nil {
		return err
	}
	b.areasByLocationID = make(map[string][]*models.Area, len(areas))
	for _, area := range areas {
		b.areasByLocationID[area.LocationID] = append(b.areasByLocationID[area.LocationID], area)
	}

	commodities, err := b.loadCommodities()
	if err != nil {
		return err
	}
	b.commoditiesByAreaID = make(map[string][]*models.Commodity, len(commodities))
	for _, com := range commodities {
		b.commoditiesByAreaID[com.AreaID] = append(b.commoditiesByAreaID[com.AreaID], com)
	}

	if err := b.indexCommodityFiles(); err != nil {
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

// indexCommodityFiles lists files once and groups the commodity attachments by
// commodity DB ID and bucket, resolving each file's size up front. Orphan rows
// (missing/unsized blob) are dropped here so they never appear in the doc or the
// statistics.
func (b *inbBuilder) indexCommodityFiles() error {
	fileReg, err := b.svc.factorySet.FileRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create file registry", err)
	}
	files, err := fileReg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list files", err)
	}

	b.filesByCommodityID = map[string]map[string][]*candidateFile{}
	for _, file := range files {
		if file == nil || file.File == nil {
			continue
		}
		if file.LinkedEntityType != "commodity" {
			continue
		}
		bucket := file.LinkedEntityMeta
		if !isCommodityFileBucket(bucket) {
			continue
		}

		size, ok := b.resolveFileSize(file)
		if !ok {
			// A missing/unsized blob (orphan row, manual delete) is dropped so
			// it is excluded from both the doc and the statistics.
			continue
		}

		byBucket := b.filesByCommodityID[file.LinkedEntityID]
		if byBucket == nil {
			byBucket = map[string][]*candidateFile{}
			b.filesByCommodityID[file.LinkedEntityID] = byBucket
		}
		byBucket[bucket] = append(byBucket[bucket], &candidateFile{file: file, size: size})
	}
	return nil
}

// resolveFileSize returns the byte size to declare for a file member: the
// recorded SizeBytes when >0, else a single bucket attributes probe. ok=false
// when the blob is missing/unsized so the caller skips it (orphan).
func (b *inbBuilder) resolveFileSize(file *models.FileEntity) (int64, bool) {
	if size := fileSizeHint(file); size > 0 {
		return size, true
	}
	// Probe the bucket for the exact size — the tar header needs it. A missing
	// blob fails the probe and the file is treated as an orphan.
	attrs, err := b.bucket.Attributes(b.ctx, file.OriginalPath)
	if err != nil || attrs.Size <= 0 {
		return 0, false
	}
	return attrs.Size, true
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
			inbCom, comPending := b.planCommodity(loc, area, com)
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
func (b *inbBuilder) planCommodity(loc *models.Location, area *models.Area, com *models.Commodity) (INBCommodity, []pendingFile) {
	inbCom := INBCommodity{
		ID:                     com.UUID,
		Name:                   com.Name,
		ShortName:              com.ShortName,
		Type:                   string(com.Type),
		AreaID:                 area.UUID,
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

	pending := b.planCommodityFiles(loc, com, &inbCom)
	return inbCom, pending
}

// planCommodityFiles records the commodity's image/invoice/manual file
// references on inbCom (from the preloaded, size-resolved index) and returns the
// corresponding ordered pendingFile list. Bucket/file/totalFileSize statistics
// are accumulated here.
func (b *inbBuilder) planCommodityFiles(loc *models.Location, com *models.Commodity, inbCom *INBCommodity) []pendingFile {
	byBucket := b.filesByCommodityID[com.ID]
	if byBucket == nil {
		return nil
	}

	var pending []pendingFile
	locSlug := textutils.CleanFilename(loc.Name)
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
	archivePath := blobkeys.SanitizeArchivePath(
		fmt.Sprintf("%s%s/%s/%s/%s", INBFilesPrefix, locSlug, com.UUID, bucket, name),
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
// run returns the per-location plan and the manifest location index; it does NOT
// write anything itself, so writePayload can interleave the manifest first.
func (b *inbBuilder) run() ([]plannedLocation, []INBManifestLoc, error) {
	scope, err := b.resolveScope()
	if err != nil {
		return nil, nil, err
	}
	if err := b.preload(); err != nil {
		return nil, nil, err
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
	return planned, manifestLocs, nil
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
