//go:build !legacy_xml_backup

package export

import (
	"archive/tar"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"gocloud.dev/blob"
	"gocloud.dev/gcerrors"

	"go.5x5.cz/inventario/backup/export/types"
	"go.5x5.cz/inventario/internal/blobkeys"
	"go.5x5.cz/inventario/internal/textutils"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
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

	// locUUIDByDBID / areaUUIDByDBID resolve a link's volatile entity id (a
	// DB key) to the linked entity's IMMUTABLE UUID, which is the only
	// identity that round-trips across databases (issue #2235). A link whose
	// entity id resolves to nothing (dangling link, out of RLS scope) is
	// dropped rather than emitted with a reference restore cannot map.
	locUUIDByDBID  map[string]string
	areaUUIDByDBID map[string]string
	// sellers are the group's sellers in name order; sellerUUIDByDBID
	// resolves an item's seller_id to the seller's UUID, and usedSellers
	// collects the DB ids of the sellers the planned items name.
	sellers          []*models.Seller
	sellerUUIDByDBID map[string]string
	usedSellers      map[string]bool
	// drafts are the group's open drafts in a whole-group export, oldest
	// first, and draftUUIDByDBID resolves a file link to one of them.
	drafts          []*models.CommodityDraft
	draftUUIDByDBID map[string]string
	// promotedItemByDraft maps a promoted draft's DB id to the item it
	// became, for a delivered document read before the promotion.
	promotedItemByDraft map[string]string
	// intakeDocuments are the documents other systems delivered, in a
	// whole-group export; commodityUUIDByDBID resolves the items they name.
	intakeDocuments []*models.IntakeDocument
	// itemTypes are the group's item types in list order; usedItemTypes
	// collects the slugs the planned items have.
	itemTypes     []*models.ItemType
	usedItemTypes map[models.CommodityType]bool
	// customFields are the group's custom fields by name; usedCustomFields
	// collects the IDs of the fields the planned items have a value of, and
	// commodityUUIDByDBID resolves the item an item link value or a
	// delivered document names.
	customFields        []*models.CustomFieldDefinition
	customFieldByID     map[string]*models.CustomFieldDefinition
	usedCustomFields    map[string]bool
	commodityUUIDByDBID map[string]string
	// entityFiles / standaloneFiles are the NON-commodity files (issue #2235),
	// size-resolved and in list order (orphans already dropped). They are
	// emitted in the dedicated files member, never nested under a commodity.
	entityFiles     []*entityCandidateFile
	standaloneFiles []*entityCandidateFile

	// sizeByFileID caches the resolved size of every file probed so far, so a
	// file attached to several entities is probed once.
	sizeByFileID map[string]resolvedSize
	// archivePathByFileID records the archive path of every file whose bytes
	// are already planned. A later reference to the same file reuses the path
	// and plans no second copy of the bytes.
	archivePathByFileID map[string]string
}

// resolvedSize is a cached resolveFileSize result.
type resolvedSize struct {
	size    int64
	present bool
}

// candidateFile is a commodity-attached file whose size has already been
// resolved (recorded SizeBytes or a one-time bucket probe). Orphans (missing or
// unsized blobs) are never turned into a candidateFile, so they are excluded
// from both the doc and the statistics.
type candidateFile struct {
	file *models.FileEntity
	size int64
}

// entityCandidateFile is one size-resolved NON-commodity reference (issue
// #2235): a file's link to a location, an area or a draft, or a standalone
// (unlinked) file. linkedType is "location", "area", "draft", or "" for
// standalone; entityUUID is
// the linked entity's IMMUTABLE UUID (empty for standalone), already resolved
// from the link's volatile entity id, which entityDBID keeps for scoping.
type entityCandidateFile struct {
	candidateFile
	linkedType string
	entityUUID string
	entityDBID string
	meta       string
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

// plannedDrafts is the plan for the drafts member: the document encoded as it
// is written. present is false when no draft is in scope, so the member is
// omitted.
type plannedDrafts struct {
	present bool
	data    []byte
}

// plannedIntakeDocuments is the plan for the delivered documents member, as
// plannedDrafts is for the drafts.
type plannedIntakeDocuments struct {
	present bool
	data    []byte
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
func (b *inbBuilder) preload(scope *inbScope) error {
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

	// The delivered documents are read before the drafts. A delivery makes
	// its draft before it records it on the document, so a draft a document
	// read here names is already there for the drafts read.
	if err := b.loadIntakeDocuments(scope); err != nil {
		return err
	}

	// The drafts are read before the items, in a transaction of their own.
	// A promotion that commits between the two reads then leaves the archive
	// with both the draft and the item it became, which restores as a
	// duplicate the user can discard. Read the other way round, the item
	// would be missing from the items read and the draft from the drafts
	// read, and the archive would lose it.
	if err := b.loadDrafts(scope); err != nil {
		return err
	}

	commodities, err := b.loadCommodities()
	if err != nil {
		return err
	}
	// A promotion that committed between the drafts read and the items
	// read put the item in the archive; the delivered document names it.
	if err := b.loadPromotions(scope); err != nil {
		return err
	}
	b.commoditiesByAreaID = make(map[string][]*models.Commodity, len(commodities))
	b.commodityUUIDByDBID = make(map[string]string, len(commodities))
	for _, com := range commodities {
		b.commodityUUIDByDBID[com.ID] = com.UUID
		// Area is optional (issue #1986): only index commodities that have an
		// area under it; collect the area-less ones for the unassigned member.
		if com.AreaID != nil && *com.AreaID != "" {
			b.commoditiesByAreaID[*com.AreaID] = append(b.commoditiesByAreaID[*com.AreaID], com)
		} else {
			b.unassignedCommodities = append(b.unassignedCommodities, com)
		}
	}

	if err := b.loadSellers(); err != nil {
		return err
	}

	if err := b.loadItemTypes(); err != nil {
		return err
	}

	if err := b.loadCustomFields(); err != nil {
		return err
	}

	// After the items, the drafts and the fields, whose links it reads.
	if err := b.loadTrashedLinkTargets(commodities); err != nil {
		return err
	}

	// After the drafts, which the files link to.
	if err := b.indexFiles(); err != nil {
		return err
	}
	return nil
}

// loadSellers lists the group's sellers once via the RLS-scoped user
// registry and indexes their UUIDs.
func (b *inbBuilder) loadSellers() error {
	b.sellerUUIDByDBID = map[string]string{}
	b.usedSellers = map[string]bool{}
	if b.svc.factorySet.SellerRegistryFactory == nil {
		return nil
	}
	reg, err := b.svc.factorySet.SellerRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create seller registry", err)
	}
	sellers, err := reg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list sellers", err)
	}
	b.sellers = sellers
	for _, s := range sellers {
		b.sellerUUIDByDBID[s.ID] = s.UUID
	}
	return nil
}

// loadDrafts lists the group's open drafts once via the RLS-scoped user
// registry. Only a whole-group export carries drafts: a selection names
// locations, areas and items, and a draft is none of them. A promoted draft
// is an item already, and one being discarded is on its way out, so neither
// is exported.
func (b *inbBuilder) loadDrafts(scope *inbScope) error {
	b.draftUUIDByDBID = map[string]string{}
	b.promotedItemByDraft = map[string]string{}
	if !scope.wholeClass || b.svc.factorySet.CommodityDraftRegistryFactory == nil {
		return nil
	}
	reg, err := b.svc.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create draft registry", err)
	}
	drafts, err := reg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list drafts", err)
	}
	for _, d := range drafts {
		if d != nil && d.PromotedCommodityID != nil {
			b.promotedItemByDraft[d.ID] = *d.PromotedCommodityID
		}
		if d == nil || !d.IsOpen() {
			continue
		}
		b.drafts = append(b.drafts, d)
		b.draftUUIDByDBID[d.ID] = d.UUID
	}
	slices.SortFunc(b.drafts, func(x, y *models.CommodityDraft) int {
		if c := x.CreatedAt.Compare(y.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(x.UUID, y.UUID)
	})
	return nil
}

// loadPromotions reads the drafts again for the items they became, after
// the items were read. The open drafts the archive carries stay the ones
// loadDrafts read.
func (b *inbBuilder) loadPromotions(scope *inbScope) error {
	if !scope.wholeClass || len(b.intakeDocuments) == 0 || b.svc.factorySet.CommodityDraftRegistryFactory == nil {
		return nil
	}
	reg, err := b.svc.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create draft registry", err)
	}
	drafts, err := reg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list drafts", err)
	}
	for _, d := range drafts {
		if d != nil && d.PromotedCommodityID != nil {
			b.promotedItemByDraft[d.ID] = *d.PromotedCommodityID
		}
	}
	return nil
}

// planDrafts builds the drafts member. The area and the seller a draft names
// are written as their UUIDs, which the restore maps to the restored area
// and seller; one the export cannot resolve is left out, so the archive never
// carries a DB id. The document is encoded here, in the planning pass, so a
// document larger than a restore accepts fails the export with
// ErrDraftsTooLarge before anything is written.
func (b *inbBuilder) planDrafts() (plannedDrafts, error) {
	if len(b.drafts) == 0 {
		return plannedDrafts{}, nil
	}
	doc := INBDraftsDoc{Drafts: make([]INBDraft, 0, len(b.drafts))}
	for _, d := range b.drafts {
		values := make(map[string]any, len(d.Values))
		maps.Copy(values, d.Values)
		b.exportDraftReference(values, "area_id", b.areaUUIDByDBID)
		b.exportDraftReference(values, "seller_id", b.sellerUUIDByDBID)
		b.exportDraftCustomFields(values)
		doc.Drafts = append(doc.Drafts, INBDraft{
			ID:        d.UUID,
			ClientKey: d.ClientKey,
			Values:    values,
			Revision:  d.Revision,
			CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt: d.UpdatedAt.UTC().Format(time.RFC3339),
		})
	}
	data, err := encodeJSONMember(INBDraftsName, doc)
	if err != nil {
		return plannedDrafts{}, err
	}
	if limit := jsonDocLimit(); int64(len(data)) > limit {
		return plannedDrafts{}, errx.Classify(ErrDraftsTooLarge,
			errx.Attrs("member", INBDraftsName, "size", len(data), "max", limit, "drafts", len(doc.Drafts)))
	}
	return plannedDrafts{present: true, data: data}, nil
}

// loadIntakeDocuments lists the documents other systems delivered to the
// group. Like the drafts, only a whole-group export carries them.
func (b *inbBuilder) loadIntakeDocuments(scope *inbScope) error {
	if !scope.wholeClass || b.svc.factorySet.IntakeDocumentRegistryFactory == nil {
		return nil
	}
	reg, err := b.svc.factorySet.IntakeDocumentRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create intake document registry", err)
	}
	docs, err := reg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list intake documents", err)
	}
	slices.SortFunc(docs, func(x, y *models.IntakeDocument) int {
		return cmp.Or(x.CreatedAt.Compare(y.CreatedAt), cmp.Compare(x.ID, y.ID))
	})
	b.intakeDocuments = docs
	return nil
}

// planIntakeDocuments builds the delivered documents member. The draft and
// the item a document names are written as their UUIDs, and left out when
// the archive does not carry them. A document read before its draft was
// promoted names the item the drafts read found.
func (b *inbBuilder) planIntakeDocuments() (plannedIntakeDocuments, error) {
	if len(b.intakeDocuments) == 0 {
		return plannedIntakeDocuments{}, nil
	}
	doc := INBIntakeDocumentsDoc{Documents: make([]INBIntakeDocument, 0, len(b.intakeDocuments))}
	for _, d := range b.intakeDocuments {
		entry := INBIntakeDocument{
			ID:                 d.UUID,
			ExternalSystem:     d.ExternalSystem,
			ExternalInstance:   d.ExternalInstance,
			ExternalDocumentID: d.ExternalDocumentID,
			SourceName:         d.SourceName,
			SourceURL:          d.SourceURL,
			PayloadHash:        d.PayloadHash,
			DraftMade:          d.DraftID != nil,
			CreatedAt:          d.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:          d.UpdatedAt.UTC().Format(time.RFC3339),
		}
		if d.DraftID != nil {
			entry.DraftID = b.draftUUIDByDBID[*d.DraftID]
		}
		commodityID := d.CommodityID
		if commodityID == nil && d.DraftID != nil {
			if item, ok := b.promotedItemByDraft[*d.DraftID]; ok {
				commodityID = &item
			}
		}
		if commodityID != nil {
			entry.CommodityID = b.commodityUUIDByDBID[*commodityID]
		}
		doc.Documents = append(doc.Documents, entry)
	}
	data, err := encodeJSONMember(INBIntakeDocumentsName, doc)
	if err != nil {
		return plannedIntakeDocuments{}, err
	}
	if limit := jsonDocLimit(); int64(len(data)) > limit {
		return plannedIntakeDocuments{}, errx.Classify(ErrIntakeDocumentsTooLarge,
			errx.Attrs("member", INBIntakeDocumentsName, "size", len(data), "max", limit, "documents", len(doc.Documents)))
	}
	return plannedIntakeDocuments{present: true, data: data}, nil
}

// exportDraftReference replaces the DB id in values[key] with the UUID
// uuidByDBID gives for it. An empty value stays, meaning nothing chosen; a
// value that is not text or names nothing in the group is removed.
func (*inbBuilder) exportDraftReference(values map[string]any, key string, uuidByDBID map[string]string) {
	raw, ok := values[key]
	if !ok {
		return
	}
	id, isText := raw.(string)
	switch {
	case isText && id == "":
	case isText && uuidByDBID[id] != "":
		values[key] = uuidByDBID[id]
	default:
		delete(values, key)
	}
}

// manifestSellers returns the sellers the manifest carries: all of them in
// a whole-group export, otherwise the ones the planned items name.
func (b *inbBuilder) manifestSellers(scope *inbScope) []INBSeller {
	out := make([]INBSeller, 0, len(b.sellers))
	for _, s := range b.sellers {
		if !scope.wholeClass && !b.usedSellers[s.ID] {
			continue
		}
		out = append(out, INBSeller{ID: s.UUID, Name: s.Name, Website: s.Website, Contact: s.Contact, Note: s.Note})
	}
	return out
}

// loadItemTypes lists the group's item types once via the RLS-scoped user
// registry.
func (b *inbBuilder) loadItemTypes() error {
	b.usedItemTypes = map[models.CommodityType]bool{}
	if b.svc.factorySet.ItemTypeRegistryFactory == nil {
		return nil
	}
	reg, err := b.svc.factorySet.ItemTypeRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create item type registry", err)
	}
	list, err := reg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list item types", err)
	}
	b.itemTypes = list
	return nil
}

// manifestItemTypes returns the item types the manifest carries: all of them
// in a whole-group export, otherwise the ones the planned items have.
func (b *inbBuilder) manifestItemTypes(scope *inbScope) []INBItemType {
	out := make([]INBItemType, 0, len(b.itemTypes))
	for _, t := range b.itemTypes {
		if !scope.wholeClass && !b.usedItemTypes[t.Slug] {
			continue
		}
		out = append(out, INBItemType{Slug: string(t.Slug), Name: t.Name, Icon: t.Icon})
	}
	return out
}

// loadCustomFields lists the group's custom fields once via the RLS-scoped
// user registry.
func (b *inbBuilder) loadCustomFields() error {
	b.customFieldByID = map[string]*models.CustomFieldDefinition{}
	b.usedCustomFields = map[string]bool{}
	if b.svc.factorySet.CustomFieldRegistryFactory == nil {
		return nil
	}
	reg, err := b.svc.factorySet.CustomFieldRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create custom field registry", err)
	}
	list, err := reg.List(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list custom fields", err)
	}
	b.customFields = list
	for _, d := range list {
		b.customFieldByID[d.ID] = d
	}
	return nil
}

// loadTrashedLinkTargets adds to commodityUUIDByDBID the items in Trash that
// an item link of commodities or of a draft names, so the archive keeps the
// link. A restore sets it when the destination has the item outside Trash,
// and counts it as an error otherwise.
func (b *inbBuilder) loadTrashedLinkTargets(commodities []*models.Commodity) error {
	missing := map[string]bool{}
	note := func(id string) string {
		if id != "" && b.commodityUUIDByDBID[id] == "" {
			missing[id] = true
		}
		return id
	}
	for _, com := range commodities {
		for id, raw := range com.CustomFields {
			var target string
			if d := b.customFieldByID[id]; d != nil && d.Type == models.CustomFieldTypeItemLink && json.Unmarshal(raw, &target) == nil {
				note(target)
			}
		}
	}
	for _, draft := range b.drafts {
		entries, _ := draft.Values["custom_fields"].(map[string]any)
		for id, entry := range entries {
			if d := b.customFieldByID[id]; d != nil && d.Type == models.CustomFieldTypeItemLink {
				models.MapDraftCustomFieldReference(entry, note)
			}
		}
	}
	if len(missing) == 0 {
		return nil
	}
	comReg, err := b.svc.factorySet.CommodityRegistryFactory.CreateUserRegistry(b.ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create commodity registry", err)
	}
	for _, id := range slices.Sorted(maps.Keys(missing)) {
		trashed, err := comReg.GetTrashed(b.ctx, id)
		switch {
		case errors.Is(err, registry.ErrNotFound):
			continue
		case err != nil:
			return errxtrace.Wrap("failed to look up a linked item in Trash", err, errx.Attrs("commodity_id", id))
		}
		b.commodityUUIDByDBID[trashed.ID] = trashed.UUID
	}
	return nil
}

// archiveCustomFields returns com's custom field values keyed by the field's
// UUID. An item link value names the linked item's UUID instead of its DB
// id, also when the linked item is in Trash (see loadTrashedLinkTargets).
func (b *inbBuilder) archiveCustomFields(com *models.Commodity) map[string]json.RawMessage {
	if len(com.CustomFields) == 0 {
		return nil
	}
	out := make(map[string]json.RawMessage, len(com.CustomFields))
	for id, raw := range com.CustomFields {
		d := b.customFieldByID[id]
		if d == nil || models.IsNullJSON(raw) {
			continue
		}
		if d.Type == models.CustomFieldTypeItemLink {
			var target string
			if json.Unmarshal(raw, &target) != nil || b.commodityUUIDByDBID[target] == "" {
				continue
			}
			raw = must.Must(json.Marshal(b.commodityUUIDByDBID[target]))
		}
		out[d.UUID] = raw
		b.usedCustomFields[d.ID] = true
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// exportDraftCustomFields archives the custom field values a draft holds
// under "custom_fields", a map from field id to an entry as the form keeps
// it. Like an item's values (see archiveCustomFields), they are keyed by the
// field's UUID, and a link names the linked item's UUID. An entry of a field
// the group no longer has, or a link the export cannot resolve, is left out,
// and so is the key when nothing is left or it holds no such map.
func (b *inbBuilder) exportDraftCustomFields(values map[string]any) {
	raw, ok := values["custom_fields"]
	if !ok {
		return
	}
	entries, _ := raw.(map[string]any)
	out := make(map[string]any, len(entries))
	for id, entry := range entries {
		d := b.customFieldByID[id]
		if d == nil || entry == nil {
			continue
		}
		if d.Type == models.CustomFieldTypeItemLink {
			entry = models.MapDraftCustomFieldReference(entry, func(id string) string { return b.commodityUUIDByDBID[id] })
			if entry == nil {
				continue
			}
		}
		out[d.UUID] = entry
	}
	if len(out) == 0 {
		delete(values, "custom_fields")
		return
	}
	values["custom_fields"] = out
}

// manifestCustomFields returns the custom fields the manifest carries: all of
// them in a whole-group export, otherwise the ones the planned items have a
// value of. A template names its item type by slug.
func (b *inbBuilder) manifestCustomFields(scope *inbScope) []INBCustomField {
	typeSlugByID := make(map[string]string, len(b.itemTypes))
	for _, t := range b.itemTypes {
		typeSlugByID[t.ID] = string(t.Slug)
	}
	out := make([]INBCustomField, 0, len(b.customFields))
	for _, d := range b.customFields {
		if !scope.wholeClass && !b.usedCustomFields[d.ID] {
			continue
		}
		f := INBCustomField{ID: d.UUID, Name: d.Name, Type: string(d.Type)}
		for _, o := range d.Options {
			f.Options = append(f.Options, INBCustomFieldOption{ID: o.ID, Label: o.Label})
		}
		for _, t := range d.Templates {
			slug := typeSlugByID[t.ItemTypeID]
			if slug == "" {
				continue
			}
			entry := INBCustomFieldTemplate{ItemType: slug}
			if !t.DefaultValue.IsNull() {
				entry.DefaultValue = json.RawMessage(t.DefaultValue)
			}
			f.Templates = append(f.Templates, entry)
		}
		out = append(out, f)
	}
	return out
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

// indexFiles lists the group's files ONCE and routes every link to its home:
// commodity attachments are grouped by commodity DB ID and bucket, while
// location-, area- and draft-linked and standalone files are collected for the
// dedicated files member (issue #2235). A file with several links is routed
// once per link. Export artifacts (files with an export link) are skipped — a
// backup must never embed previous backups. A link to a draft the export does
// not carry (any draft in a selection, a draft being discarded) is left out,
// and a file with no other link is exported as a standalone file, so its bytes
// survive a restore.
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
	b.sizeByFileID = make(map[string]resolvedSize, len(files))
	b.archivePathByFileID = make(map[string]string, len(files))
	for _, file := range files {
		if file == nil || file.File == nil || file.IsExportOwned() {
			// An export bundle is not user inventory — skip it.
			continue
		}
		// A blob probe that cannot determine a file's state aborts the export:
		// an incomplete-but-successful backup is worse than a failed one.
		links := b.exportedFileLinks(file.Links)
		if len(links) == 0 {
			if err := b.indexStandaloneFile(file); err != nil {
				return err
			}
			continue
		}
		for _, link := range links {
			var indexErr error
			switch link.EntityType {
			case models.FileLinkCommodity:
				indexErr = b.indexCommodityFile(file, link)
			case models.FileLinkLocation, models.FileLinkArea, models.FileLinkDraft:
				indexErr = b.indexEntityFile(file, link)
			default:
				// A link type this build does not know is not exported.
				continue
			}
			if indexErr != nil {
				return indexErr
			}
		}
	}
	return nil
}

// exportedFileLinks returns the links a backup carries: every link except
// the links to drafts the backup does not contain and the links to intake
// sources. Intake sessions (#2810) are a work queue and stay out of backups;
// a file only an intake source holds is exported as a standalone file.
func (b *inbBuilder) exportedFileLinks(links []models.FileLink) []models.FileLink {
	out := make([]models.FileLink, 0, len(links))
	for _, link := range links {
		switch {
		case link.EntityType == models.FileLinkIntake:
		case link.EntityType == models.FileLinkDraft && b.draftUUIDByDBID[link.EntityID] == "":
		default:
			out = append(out, link)
		}
	}
	return out
}

// cachedFileSize resolves a file's size once per export.
func (b *inbBuilder) cachedFileSize(file *models.FileEntity) (int64, bool, error) {
	if cached, ok := b.sizeByFileID[file.ID]; ok {
		return cached.size, cached.present, nil
	}
	size, present, err := b.resolveFileSize(file)
	if err != nil {
		return 0, false, err
	}
	b.sizeByFileID[file.ID] = resolvedSize{size: size, present: present}
	return size, present, nil
}

// indexCommodityFile files one commodity attachment under its commodity + bucket.
// A bucket outside images/documents cannot pass model validation, so such a
// link is skipped rather than emitted.
func (b *inbBuilder) indexCommodityFile(file *models.FileEntity, link models.FileLink) error {
	bucket := link.Meta
	if !isCommodityFileBucket(bucket) {
		return nil
	}

	// Record the DB-id → UUID mapping for EVERY commodity-attached file,
	// before the orphan-size filter below: the cover-file cross-reference
	// only needs the stable UUID, and resolving it for a file whose blob
	// later proves missing is harmless (the restore drops the cover if its
	// target file never lands).
	b.fileUUIDByDBID[file.ID] = file.UUID

	size, present, err := b.cachedFileSize(file)
	if err != nil {
		return err
	}
	if !present {
		// A confirmed-missing/unsized blob (orphan row, manual delete) is dropped
		// so it is excluded from both the doc and the statistics.
		return nil
	}

	byBucket := b.filesByCommodityID[link.EntityID]
	if byBucket == nil {
		byBucket = map[string][]*candidateFile{}
		b.filesByCommodityID[link.EntityID] = byBucket
	}
	byBucket[bucket] = append(byBucket[bucket], &candidateFile{file: file, size: size})
	return nil
}

// indexEntityFile collects one location, area or draft link of a file (issue
// #2235), resolving the link's volatile entity id to the linked entity's
// immutable UUID. A link whose entity does not resolve (dangling link, out of
// RLS scope) or whose bucket is not one of the entity type's is skipped rather
// than emitted with a reference the restore side could not map.
func (b *inbBuilder) indexEntityFile(file *models.FileEntity, link models.FileLink) error {
	if !slices.Contains(models.FileLinkMetaValues(link.EntityType), link.Meta) {
		return nil
	}

	var entityUUID string
	switch link.EntityType {
	case models.FileLinkLocation:
		entityUUID = b.locUUIDByDBID[link.EntityID]
	case models.FileLinkArea:
		entityUUID = b.areaUUIDByDBID[link.EntityID]
	case models.FileLinkDraft:
		entityUUID = b.draftUUIDByDBID[link.EntityID]
	}
	if entityUUID == "" {
		return nil
	}

	size, present, err := b.cachedFileSize(file)
	if err != nil {
		return err
	}
	if !present {
		return nil
	}
	b.entityFiles = append(b.entityFiles, &entityCandidateFile{
		candidateFile: candidateFile{file: file, size: size},
		linkedType:    link.EntityType,
		entityUUID:    entityUUID,
		entityDBID:    link.EntityID,
		meta:          link.Meta,
	})
	return nil
}

// indexStandaloneFile collects one unlinked file (issue #2235). linkedType and
// entityUUID stay empty.
func (b *inbBuilder) indexStandaloneFile(file *models.FileEntity) error {
	size, present, err := b.cachedFileSize(file)
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
// #2235): every location-/area link and standalone file in scope, each carrying
// its own polymorphic link so restore recreates the link with the original
// entity type, entity and meta.
//
// Scope (legacy XML parity): a whole-class export carries every entity file plus
// the standalone files; a selected_items export carries only the files of the
// locations/areas the user EXPLICITLY selected, and NO standalone files (they have
// no parent that could imply them into the selection). See entityFileInScope.
//
// Statistics: these files bump FileCount + BinaryDataSize only, once per stored
// file. ImageCount / InvoiceCount / ManualCount are legacy COMMODITY-scoped
// counters (see export/types.ExportStats), so a location image must not inflate
// imageCount.
func (b *inbBuilder) planFiles(scope *inbScope) plannedFiles {
	var doc INBFilesDoc
	var pending []pendingFile

	appendCandidate := func(cand *entityCandidateFile) {
		ref, pf, fresh := b.buildEntityFileRef(cand)
		doc.Files = append(doc.Files, ref)
		if fresh {
			pending = append(pending, pf)
			b.stats.FileCount++
			b.stats.BinaryDataSize += cand.size
		}
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
	// The link's entity id is a DB key, and so are the selection sets.
	switch cand.linkedType {
	case models.FileLinkLocation:
		return scope.locationIDs[cand.entityDBID]
	case models.FileLinkArea:
		return scope.areaIDs[cand.entityDBID]
	}
	return false
}

// claimArchivePath returns the archive path for a file's bytes. The first
// reference to a file claims candidate and reports fresh=true; every later one
// gets the claimed path and fresh=false, because the bytes are written once.
func (b *inbBuilder) claimArchivePath(file *models.FileEntity, candidate string) (archivePath string, fresh bool) {
	if claimed, ok := b.archivePathByFileID[file.ID]; ok {
		return claimed, false
	}
	b.archivePathByFileID[file.ID] = candidate
	return candidate, true
}

// buildEntityFileRef builds a non-commodity file reference + its pending stream
// entry (issue #2235). The archive member keeps a <file-uuid> segment for the
// same anti-collision reason as the commodity layout (two files of one entity can
// share a basename), and the whole path goes through SanitizeArchivePath — the
// restore side rejects any member whose sanitized form differs from its raw name.
func (b *inbBuilder) buildEntityFileRef(cand *entityCandidateFile) (INBEntityFileRef, pendingFile, bool) {
	file := cand.file
	name := blobkeys.SanitizeArchivePath(fileMemberName(file))

	var archivePath string
	if cand.linkedType == "" {
		archivePath = fmt.Sprintf("%s%s/%s", INBStandaloneFilesPrefix, file.UUID, name)
	} else {
		archivePath = fmt.Sprintf("%s%s/%s/%s/%s/%s",
			INBEntityFilesPrefix, cand.linkedType, cand.entityUUID, cand.meta, file.UUID, name)
	}
	archivePath, fresh := b.claimArchivePath(file, blobkeys.SanitizeArchivePath(archivePath))

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
		LinkedEntityMeta: cand.meta,
		Type:             string(file.Type),
		Category:         string(file.Category),
	}

	pf := pendingFile{archivePath: archivePath, blobKey: file.OriginalPath, size: cand.size}
	return ref, pf, fresh
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
	b.usedItemTypes[com.Type] = true
	inbCom := INBCommodity{
		ID:                     com.UUID,
		Name:                   com.Name,
		ShortName:              com.ShortName,
		Brand:                  com.Brand,
		Model:                  com.Model,
		Type:                   string(com.Type),
		AreaID:                 areaUUID,
		Count:                  com.Count,
		OriginalPrice:          ptrDecimalString(com.OriginalPrice),
		OriginalPriceCurrency:  string(com.OriginalPriceCurrency),
		ConvertedOriginalPrice: ptrDecimalString(com.ConvertedOriginalPrice),
		CurrentPrice:           ptrDecimalString(com.CurrentPrice),
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
	if com.AssetNumber != nil {
		inbCom.AssetNumber = *com.AssetNumber
	}
	inbCom.CustomFields = b.archiveCustomFields(com)
	if com.SellerID != nil {
		if sellerUUID := b.sellerUUIDByDBID[*com.SellerID]; sellerUUID != "" {
			inbCom.SellerID = sellerUUID
			b.usedSellers[*com.SellerID] = true
		}
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
			ref, pf, fresh := b.buildFileRef(locSlug, com, bucket, cand)
			appendFileRef(inbCom, bucket, ref)
			if fresh {
				pending = append(pending, pf)
				b.stats.BinaryDataSize += cand.size
				b.stats.FileCount++
			}
			// The bucket counters count attachments, so a receipt attached to
			// three items counts three times.
			incBucketStat(b.stats, bucket, ref.Tags)
		}
	}
	return pending
}

// buildFileRef builds a file reference + its pending stream entry from a
// size-resolved candidate file. No probing happens here — the size was resolved
// once in preload. fresh is false when an earlier reference already planned the
// file's bytes; the reference then names that earlier path.
func (b *inbBuilder) buildFileRef(locSlug string, com *models.Commodity, bucket string, cand *candidateFile) (INBFileRef, pendingFile, bool) {
	file := cand.file
	name := blobkeys.SanitizeArchivePath(fileMemberName(file))
	// Disambiguate by the file's immutable UUID: two files attached to the same
	// commodity bucket can share a user-facing basename (e.g. two "invoice.pdf").
	// Without the UUID segment they would collide on one archive member name and
	// restore would silently drop one file's bytes (and mis-assign the survivor's
	// metadata) while still reporting Completed. The UUID guarantees uniqueness;
	// the original filename is preserved in the ref's Name/OriginalPath.
	archivePath, fresh := b.claimArchivePath(file, blobkeys.SanitizeArchivePath(
		fmt.Sprintf("%s%s/%s/%s/%s/%s", INBFilesPrefix, locSlug, com.UUID, bucket, file.UUID, name),
	))

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
	return ref, pf, fresh
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
	drafts       plannedDrafts
	intakeDocs   plannedIntakeDocuments
	files        plannedFiles
	sellers      []INBSeller
	itemTypes    []INBItemType
	customFields []INBCustomField
}

// run returns the Pass-1 plan; it does NOT write anything itself, so writePayload
// can interleave the manifest first.
func (b *inbBuilder) run() (inbPlan, error) {
	scope, err := b.resolveScope()
	if err != nil {
		return inbPlan{}, err
	}
	if err := b.preload(scope); err != nil {
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

	drafts, err := b.planDrafts()
	if err != nil {
		return inbPlan{}, err
	}
	intakeDocs, err := b.planIntakeDocuments()
	if err != nil {
		return inbPlan{}, err
	}

	return inbPlan{
		locations:    planned,
		manifestLocs: manifestLocs,
		unassigned:   unassigned,
		drafts:       drafts,
		intakeDocs:   intakeDocs,
		files:        b.planFiles(scope),
		// After every item is planned, so usedSellers is complete.
		sellers: b.manifestSellers(scope),
		// After every item is planned, so usedItemTypes is complete.
		itemTypes: b.manifestItemTypes(scope),
		// After every item is planned, so usedCustomFields is complete.
		customFields: b.manifestCustomFields(scope),
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

// writeDrafts writes the drafts member; a no-op when no draft is in scope.
func (b *inbBuilder) writeDrafts(pd plannedDrafts) error {
	if !pd.present {
		return nil
	}
	return b.writeMember(INBDraftsName, pd.data)
}

// writeIntakeDocuments writes the delivered documents member; a no-op when no
// document is in scope.
func (b *inbBuilder) writeIntakeDocuments(pd plannedIntakeDocuments) error {
	if !pd.present {
		return nil
	}
	return b.writeMember(INBIntakeDocumentsName, pd.data)
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
var commodityFileBuckets = []string{"images", "documents"}

// isCommodityFileBucket reports whether a linked_entity_meta value is one of the
// two commodity attachment buckets.
func isCommodityFileBucket(meta string) bool {
	switch meta {
	case "images", "documents":
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
	case "documents":
		com.Documents = append(com.Documents, ref)
	}
}

// incBucketStat increments the per-bucket stat counter.
//
// InvoiceCount and ManualCount outlive the buckets they were named after: the
// archive now carries one `documents` section and the kind lives on a tag, but
// both numbers are on the exports screen, so they are derived rather than
// dropped. A document with neither tag counts towards neither, which is what
// the old buckets did for a file nobody filed.
func incBucketStat(stats *types.ExportStats, bucket string, tags []string) {
	switch bucket {
	case "images":
		stats.ImageCount++
	case "documents":
		if slices.Contains(tags, models.FileTagInvoice) {
			stats.InvoiceCount++
		}
		if slices.Contains(tags, models.FileTagManual) {
			stats.ManualCount++
		}
	}
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
