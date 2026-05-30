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
type inbBuilder struct {
	svc    *ExportService
	ctx    context.Context
	export models.Export
	tenant string
	bucket *blob.Bucket
	tw     *tar.Writer
	stats  *types.ExportStats
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

// listLocations returns the locations to emit. For selected_items it also
// includes the locations that own a selected area or commodity, so a selected
// commodity's location document exists to hang it under.
func (b *inbBuilder) listLocations(scope *inbScope) ([]*models.Location, error) {
	locReg, err := b.svc.factorySet.LocationRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create location registry", err)
	}
	locations, err := locReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list locations", err)
	}
	if scope.allLocations {
		return locations, nil
	}

	// selected_items: a location is in scope if it was explicitly selected
	// OR it owns a selected area/commodity. We compute the implied parents by
	// walking areas/commodities once.
	implied, err := b.impliedLocationIDs(scope)
	if err != nil {
		return nil, err
	}
	var filtered []*models.Location
	for _, loc := range locations {
		if scope.locationIDs[loc.ID] || implied[loc.ID] {
			filtered = append(filtered, loc)
		}
	}
	return filtered, nil
}

// impliedLocationIDs computes the set of location DB IDs that own a selected
// area or commodity, so their documents are emitted even when the location
// itself was not directly selected.
func (b *inbBuilder) impliedLocationIDs(scope *inbScope) (map[string]bool, error) {
	implied := map[string]bool{}

	areaReg, err := b.svc.factorySet.AreaRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create area registry", err)
	}
	areas, err := areaReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list areas", err)
	}
	areaToLoc := map[string]string{}
	for _, area := range areas {
		areaToLoc[area.ID] = area.LocationID
		if scope.areaIDs[area.ID] {
			implied[area.LocationID] = true
		}
	}

	comReg, err := b.svc.factorySet.CommodityRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity registry", err)
	}
	commodities, err := comReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities", err)
	}
	for _, com := range commodities {
		if scope.commodityIDs[com.ID] {
			if locID, ok := areaToLoc[com.AreaID]; ok {
				implied[locID] = true
			}
		}
	}
	return implied, nil
}

// buildLocationDoc assembles the full JSON document for one location (its areas
// and their commodities, with each commodity's attached file references),
// writes the JSON member FIRST, then streams the referenced file bytes. Writing
// the JSON before the bytes lets the restore side register each file reference
// before the corresponding member arrives. Returns the in-archive member name
// for the manifest index.
func (b *inbBuilder) buildLocationDoc(loc *models.Location, scope *inbScope) (string, error) {
	areas, err := b.areasForLocation(loc, scope)
	if err != nil {
		return "", err
	}

	commoditiesByArea, err := b.commoditiesForAreas(areas, scope, loc)
	if err != nil {
		return "", err
	}

	doc := INBLocationDoc{
		Location: INBLocation{ID: loc.UUID, Name: loc.Name, Address: loc.Address},
	}
	b.stats.LocationCount++

	var pending []pendingFile
	for _, area := range areas {
		doc.Areas = append(doc.Areas, INBArea{
			ID:         area.UUID,
			Name:       area.Name,
			LocationID: loc.UUID,
		})
		b.stats.AreaCount++

		for _, com := range commoditiesByArea[area.ID] {
			inbCom, comPending, cErr := b.buildCommodity(loc, area, com)
			if cErr != nil {
				return "", cErr
			}
			doc.Commodities = append(doc.Commodities, inbCom)
			pending = append(pending, comPending...)
			b.stats.CommodityCount++
		}
	}

	member := fmt.Sprintf("location-%s-%s.json", textutils.CleanFilename(loc.Name), loc.UUID)
	member = blobkeys.SanitizeArchivePath(member)
	// Location JSON first…
	if err := b.writeJSONMember(member, doc); err != nil {
		return "", err
	}
	// …then the referenced file bytes.
	for _, pf := range pending {
		if err := b.flushPendingFile(pf); err != nil {
			// A missing blob (orphan row, manual delete) must not abort the
			// whole export — the ref was already omitted from the doc when the
			// size probe failed, so we only reach here for genuine stream
			// errors, which we surface.
			return "", err
		}
	}
	return member, nil
}

// flushPendingFile streams one collected file's bytes into the tar and updates
// stats.
func (b *inbBuilder) flushPendingFile(pf pendingFile) error {
	written, err := b.writeFileMember(pf.archivePath, pf.blobKey, pf.size)
	if err != nil {
		return err
	}
	b.stats.BinaryDataSize += written
	b.stats.FileCount++
	incBucketStat(b.stats, pf.bucket)
	return nil
}

// areasForLocation returns the in-scope areas of a location.
func (b *inbBuilder) areasForLocation(loc *models.Location, scope *inbScope) ([]*models.Area, error) {
	areaReg, err := b.svc.factorySet.AreaRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create area registry", err)
	}
	areas, err := areaReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list areas", err)
	}
	var result []*models.Area
	for _, area := range areas {
		if area.LocationID != loc.ID {
			continue
		}
		if scope.allAreas || scope.areaIDs[area.ID] || b.areaHasSelectedCommodity(area, scope) {
			result = append(result, area)
		}
	}
	return result, nil
}

// areaHasSelectedCommodity reports whether a selected_items export picked a
// commodity that lives in this area (so the area must be emitted to host it).
func (b *inbBuilder) areaHasSelectedCommodity(area *models.Area, scope *inbScope) bool {
	if len(scope.commodityIDs) == 0 {
		return false
	}
	comReg := b.svc.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	commodities, err := comReg.List(b.ctx)
	if err != nil {
		return false
	}
	for _, com := range commodities {
		if com.AreaID == area.ID && scope.commodityIDs[com.ID] {
			return true
		}
	}
	return false
}

// commoditiesForAreas returns the in-scope commodities for the given areas,
// keyed by area DB ID.
func (b *inbBuilder) commoditiesForAreas(areas []*models.Area, scope *inbScope, _ *models.Location) (map[string][]*models.Commodity, error) {
	comReg, err := b.svc.factorySet.CommodityRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity registry", err)
	}
	commodities, err := comReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities", err)
	}
	areaIDs := map[string]bool{}
	for _, a := range areas {
		areaIDs[a.ID] = true
	}
	out := map[string][]*models.Commodity{}
	for _, com := range commodities {
		if !areaIDs[com.AreaID] {
			continue
		}
		if scope.allCommodities || scope.commodityIDs[com.ID] {
			out[com.AreaID] = append(out[com.AreaID], com)
		}
	}
	return out, nil
}

// buildCommodity converts a commodity row to its INBCommodity shape and collects
// (without yet streaming) its attached image/invoice/manual file references. The
// bytes are flushed by buildLocationDoc AFTER the location JSON is written.
func (b *inbBuilder) buildCommodity(loc *models.Location, area *models.Area, com *models.Commodity) (INBCommodity, []pendingFile, error) {
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

	pending, err := b.collectCommodityFiles(loc, com, &inbCom)
	if err != nil {
		return INBCommodity{}, nil, err
	}
	return inbCom, pending, nil
}

// collectCommodityFiles records the commodity's image/invoice/manual file
// references on inbCom and returns the corresponding pendingFile list (bytes to
// be streamed after the location JSON). The unified files table is the single
// source: rows linked to this commodity (linked_entity_type="commodity") are
// bucketed by linked_entity_meta (images/invoices/manuals).
func (b *inbBuilder) collectCommodityFiles(loc *models.Location, com *models.Commodity, inbCom *INBCommodity) ([]pendingFile, error) {
	fileReg, err := b.svc.factorySet.FileRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file registry", err)
	}
	files, err := fileReg.List(b.ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list files", err)
	}

	var pending []pendingFile
	locSlug := textutils.CleanFilename(loc.Name)
	for _, file := range files {
		if file == nil || file.File == nil {
			continue
		}
		if file.LinkedEntityType != "commodity" || file.LinkedEntityID != com.ID {
			continue
		}
		bucket := file.LinkedEntityMeta
		if !isCommodityFileBucket(bucket) {
			continue
		}

		ref, pf, ok := b.prepareCommodityFile(locSlug, com, bucket, file)
		if !ok {
			// A missing/unsized blob (orphan row, manual delete) is omitted from
			// both the doc and the stream so the export stays fail-soft.
			continue
		}
		appendFileRef(inbCom, bucket, ref)
		pending = append(pending, pf)
	}
	return pending, nil
}

// prepareCommodityFile builds a file reference + its pending stream entry,
// probing the blob size for the tar header. Returns ok=false when the blob is
// missing/unsized so the caller can skip it.
func (b *inbBuilder) prepareCommodityFile(locSlug string, com *models.Commodity, bucket string, file *models.FileEntity) (INBFileRef, pendingFile, bool) {
	name := blobkeys.SanitizeArchivePath(fileMemberName(file))
	archivePath := blobkeys.SanitizeArchivePath(
		fmt.Sprintf("%s%s/%s/%s/%s", INBFilesPrefix, locSlug, com.UUID, bucket, name),
	)

	size := fileSizeHint(file)
	if size <= 0 {
		// Probe the bucket for the exact size — the tar header needs it.
		attrs, err := b.bucket.Attributes(b.ctx, file.OriginalPath)
		if err != nil {
			return INBFileRef{}, pendingFile{}, false
		}
		size = attrs.Size
	}

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
	pf := pendingFile{archivePath: archivePath, blobKey: file.OriginalPath, size: size, bucket: bucket}
	return ref, pf, true
}

// run walks the in-scope locations and emits one JSON member per location plus
// each commodity's attached file bytes. Returns the manifest location index.
func (b *inbBuilder) run() ([]INBManifestLoc, error) {
	scope, err := b.resolveScope()
	if err != nil {
		return nil, err
	}
	locations, err := b.listLocations(scope)
	if err != nil {
		return nil, err
	}
	var manifestLocs []INBManifestLoc
	for _, loc := range locations {
		member, mErr := b.buildLocationDoc(loc, scope)
		if mErr != nil {
			return nil, mErr
		}
		manifestLocs = append(manifestLocs, INBManifestLoc{
			ID:   loc.UUID,
			Name: loc.Name,
			File: member,
		})
	}
	return manifestLocs, nil
}

// --- small free helpers ---

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
