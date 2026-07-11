//go:build !legacy_xml_backup

package export

// This file defines the JSON document schemas written into the inner tar of a
// signed `.inb` backup archive (issue #534). The same shapes are decoded on the
// restore side (see backup/restore/types/types_inb.go) — keep the two in sync.
//
// Identity rule: every id / foreign-key field carries the entity's IMMUTABLE
// UUID, never a volatile DB primary key, so a backup round-trips stably across
// databases.

// INBFormatVersion is the manifest `version` for the JSON `.inb` format. The
// MAJOR component is the compatibility contract the restore side enforces (see
// backup/restore/processor.checkInbFormatVersion): a reader accepts any archive
// whose major version it knows and rejects a higher one outright. MINOR bumps
// are additive-only — a new OPTIONAL member plus an optional manifest pointer,
// dispatched by member name — so a 2.0 archive still restores unchanged.
//
// 2.1 (#2235) added the non-commodity files member (INBFilesName): files linked
// to locations and areas, plus standalone files.
const INBFormatVersion = "2.1"

// INBManifestName is the inner-tar member holding the manifest document.
const INBManifestName = "manifest.json"

// INBUnassignedName is the inner-tar member holding the area-less ("unassigned")
// commodities document (issue #1986). Commodities with no area have no location
// home in the location-centric layout, so they are emitted here as a flat list.
// The member is written ONLY when at least one unassigned commodity is in scope,
// keeping archives with no unassigned items byte-stable (no empty member).
const INBUnassignedName = "unassigned-commodities.json"

// INBFilesPrefix is the inner-tar path prefix under which file bytes are stored.
// A commodity attachment lives at
// `files/<loc-slug>/<commodity-uuid>/<bucket>/<file-uuid>/<sanitized-name>` —
// the `<file-uuid>` segment is what keeps two same-named attachments of one
// commodity from colliding on a single member name.
const INBFilesPrefix = "files/"

// INBFilesName is the inner-tar member holding the NON-commodity files document
// (issue #2235): every group file that is not a commodity attachment — files
// linked to a location or an area, and standalone (unlinked) files. Commodity
// attachments are NOT repeated here; they stay nested under their commodity
// (INBCommodity.Images/Invoices/Manuals).
//
// The member deliberately lives UNDER the `files/` prefix rather than at the top
// level. An older reader dispatches any other top-level `*.json` member to its
// location-document handler, which would fail the whole restore on a document
// that carries no location — and on full_replace that happens AFTER the existing
// data was already wiped. Under `files/` the same reader routes it to its file
// handler, which finds no matching reference, counts one error, and carries on.
//
// It cannot collide with a file-BYTES member either: every byte member nests at
// least one level deeper under `files/` — commodity attachments under
// `<location-slug>/<commodity-uuid>/<bucket>/<file-uuid>/`, non-commodity ones
// under `_entity/…` or `_standalone/<file-uuid>/` — so no byte member is ever a
// direct `files/<name>` child, which is exactly what this document is.
//
// Written ONLY when at least one non-commodity file is in scope, so an archive
// without any stays byte-stable (no empty member, no manifest pointer).
const INBFilesName = INBFilesPrefix + "_index.json"

// INBEntityFilesPrefix / INBStandaloneFilesPrefix are the archive homes of the
// non-commodity file BYTES (issue #2235):
//
//	files/_entity/<linked-type>/<entity-uuid>/<bucket>/<file-uuid>/<name>
//	files/_standalone/<file-uuid>/<name>
//
// Both keep a `<file-uuid>` segment for the same anti-collision reason as the
// commodity layout, and both are run through blobkeys.SanitizeArchivePath.
const (
	INBEntityFilesPrefix     = INBFilesPrefix + "_entity/"
	INBStandaloneFilesPrefix = INBFilesPrefix + "_standalone/"
)

// INBManifest is the top-level descriptor written as manifest.json. It records
// the format, the signing key the archive was signed with (informational only —
// restore verifies against the server's own key, never this), the per-location
// member index, and aggregate statistics.
//
// compressedSize is intentionally OMITTED: the manifest lives inside the very
// gzip stream whose size it would describe, so it is unknowable at write time.
// The final `.inb` size is recorded on Export.FileSize instead.
type INBManifest struct {
	ExportDate  string           `json:"exportDate"`
	ExportType  string           `json:"exportType"`
	Version     string           `json:"version"`
	Format      string           `json:"format"`
	Compression string           `json:"compression"`
	Signature   INBSignatureInfo `json:"signature"`
	Locations   []INBManifestLoc `json:"locations"`
	// UnassignedFile names the inner-tar member holding the area-less
	// commodities document (issue #1986), or "" when the archive contains no
	// unassigned commodities (then the member is absent entirely). Restore
	// keys off member names and tolerates its absence, so older archives
	// without this field round-trip unchanged.
	UnassignedFile string `json:"unassignedFile,omitempty"`
	// FilesFile names the inner-tar member holding the non-commodity files
	// document (issue #2235), or "" when the archive carries no location-,
	// area-linked or standalone file (then the member is absent entirely).
	// Restore keys off member names, so a 2.0 archive without this field
	// round-trips unchanged.
	FilesFile  string           `json:"filesFile,omitempty"`
	Statistics INBManifestStats `json:"statistics"`
}

// INBSignatureInfo records the signing algorithm + the public key (base64) and
// its fingerprint. Purely informational: it lets an offline tool identify which
// key signed the archive, but restore always verifies against the server's own
// configured key.
type INBSignatureInfo struct {
	Algorithm   string `json:"algorithm"`
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
}

// INBManifestLoc is the manifest's per-location index entry: the location UUID,
// its display name, and the inner-tar member holding its full JSON document.
type INBManifestLoc struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
}

// INBManifestStats mirrors the export statistics surfaced on the Export record.
type INBManifestStats struct {
	LocationCount  int   `json:"locationCount"`
	AreaCount      int   `json:"areaCount"`
	CommodityCount int   `json:"commodityCount"`
	ImageCount     int   `json:"imageCount"`
	InvoiceCount   int   `json:"invoiceCount"`
	ManualCount    int   `json:"manualCount"`
	FileCount      int   `json:"fileCount"`
	TotalFileSize  int64 `json:"totalFileSize"`
}

// INBLocationDoc is the document written as a per-location tar member
// (`location-<slug>-<uuid>.json`). It carries the location, its areas, and the
// commodities under those areas — each commodity bundling its image/invoice/
// manual file references.
type INBLocationDoc struct {
	Location    INBLocation    `json:"location"`
	Areas       []INBArea      `json:"areas"`
	Commodities []INBCommodity `json:"commodities"`
}

// INBUnassignedDoc is the document written as the INBUnassignedName member: a
// flat list of area-less commodities (issue #1986). Each INBCommodity has its
// AreaID set to "" (the empty wire value for "no area"); the restore side maps
// that back to a nil model AreaID via ConvertToCommodity.
type INBUnassignedDoc struct {
	Commodities []INBCommodity `json:"commodities"`
}

// INBLocation is a location row keyed by its immutable UUID.
type INBLocation struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
}

// INBArea is an area row; LocationID is the parent location's immutable UUID.
type INBArea struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LocationID string `json:"locationId"`
}

// INBCommodity is a commodity row keyed by its immutable UUID; AreaID is the
// parent area's immutable UUID, or "" for an area-less ("unassigned") commodity
// emitted in the INBUnassignedDoc member (issue #1986). The three file-reference
// arrays carry the commodity's attached images / invoices / manuals.
type INBCommodity struct {
	ID                     string   `json:"id"`
	Name                   string   `json:"name"`
	ShortName              string   `json:"shortName,omitempty"`
	Type                   string   `json:"type,omitempty"`
	AreaID                 string   `json:"areaId"`
	Count                  int      `json:"count"`
	OriginalPrice          string   `json:"originalPrice,omitempty"`
	OriginalPriceCurrency  string   `json:"originalPriceCurrency,omitempty"`
	ConvertedOriginalPrice string   `json:"convertedOriginalPrice,omitempty"`
	CurrentPrice           string   `json:"currentPrice,omitempty"`
	SerialNumber           string   `json:"serialNumber,omitempty"`
	ExtraSerialNumbers     []string `json:"extraSerialNumbers,omitempty"`
	PartNumbers            []string `json:"partNumbers,omitempty"`
	Tags                   []string `json:"tags,omitempty"`
	Status                 string   `json:"status,omitempty"`
	PurchaseDate           string   `json:"purchaseDate,omitempty"`
	RegisteredDate         string   `json:"registeredDate,omitempty"`
	LastModifiedDate       string   `json:"lastModifiedDate,omitempty"`
	URLs                   []string `json:"urls,omitempty"`
	Comments               string   `json:"comments,omitempty"`
	Draft                  bool     `json:"draft"`

	// Extended commodity fields (#534 round-trip parity). Warranty (#1554),
	// terminal-status metadata (#1611), acquisition provenance (#202), and the
	// user-picked cover photo (#1451) all round-trip so a restored commodity is
	// a lossless copy of the exported one.
	WarrantyExpiresAt string `json:"warrantyExpiresAt,omitempty"`
	WarrantyNotes     string `json:"warrantyNotes,omitempty"`
	StatusDate        string `json:"statusDate,omitempty"`
	StatusNote        string `json:"statusNote,omitempty"`
	SalePrice         string `json:"salePrice,omitempty"`
	// AcquisitionPrice / AcquisitionCurrency are the write-once acquisition
	// provenance pair (#202). They are restored through the trusted
	// registry.WithRestoreAcquisition context seam, never the normal write path.
	AcquisitionPrice    string `json:"acquisitionPrice,omitempty"`
	AcquisitionCurrency string `json:"acquisitionCurrency,omitempty"`
	// CoverFileID is the IMMUTABLE UUID of the commodity's user-picked cover
	// photo (#1451). Stored as a UUID (not the volatile cover_file_id DB key)
	// so it round-trips stably; the restore side re-resolves it to the new
	// file's DB id after the commodity's files are recreated.
	CoverFileID string `json:"coverFileId,omitempty"`

	Images   []INBFileRef `json:"images,omitempty"`
	Invoices []INBFileRef `json:"invoices,omitempty"`
	Manuals  []INBFileRef `json:"manuals,omitempty"`
}

// INBFileRef is a reference to a commodity-attached file. Path is the file's
// location inside the inner tar (`files/<loc>/<commodity>/<bucket>/<name>`);
// the actual bytes follow as a separate tar member at that path. Name is the
// file's user-facing basename (the original File.Path stem) preserved so a
// restore reproduces the original filename rather than the display Title or the
// UUID. OriginalPath records the source blob key (informational) — the restore
// side never reuses it as a destination, always re-minting a key under the
// importing tenant.
type INBFileRef struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	Name         string   `json:"name,omitempty"`
	OriginalPath string   `json:"originalPath,omitempty"`
	Extension    string   `json:"extension,omitempty"`
	MimeType     string   `json:"mimeType,omitempty"`
	Title        string   `json:"title,omitempty"`
	Description  string   `json:"description,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	CreatedAt    string   `json:"createdAt,omitempty"`
	UpdatedAt    string   `json:"updatedAt,omitempty"`
}

// INBFilesDoc is the document written as the INBFilesName member (issue #2235):
// the group's NON-commodity files. Commodity attachments are not repeated here —
// they stay nested under their commodity.
type INBFilesDoc struct {
	Files []INBEntityFileRef `json:"files"`
}

// INBEntityFileRef is a file reference that carries its OWN polymorphic link, so
// the restore side can recreate the row with the original linked_entity_type /
// linked_entity_id / linked_entity_meta instead of assuming a commodity
// attachment (issue #2235).
//
// LinkedEntityID is the linked entity's IMMUTABLE UUID — never the volatile
// files.linked_entity_id DB key, which is regenerated on restore. Restore maps
// it back to the destination DB id through the location/area ID mapping.
//
// Type and Category are carried EXPLICITLY: models.FileCategoryFromContext has
// no "area" and no standalone branch, so re-deriving them on the restore side
// would silently rewrite the user-visible category.
type INBEntityFileRef struct {
	INBFileRef

	// LinkedEntityType is "location", "area", or "" for a standalone file.
	LinkedEntityType string `json:"linkedEntityType,omitempty"`
	// LinkedEntityID is the linked location/area UUID; "" for a standalone file.
	LinkedEntityID string `json:"linkedEntityId,omitempty"`
	// LinkedEntityMeta is the location/area bucket ("images" or "files"); "" for
	// a standalone file.
	LinkedEntityMeta string `json:"linkedEntityMeta,omitempty"`
	// Type / Category are the models.FileType / models.FileCategory values as
	// stored, carried verbatim so they round-trip losslessly.
	Type     string `json:"type,omitempty"`
	Category string `json:"category,omitempty"`
}
