//go:build !legacy_xml_backup

package export

// This file defines the JSON document schemas written into the inner tar of a
// signed `.inb` backup archive (issue #534). The same shapes are decoded on the
// restore side (see backup/restore/types/types_inb.go) — keep the two in sync.
//
// Identity rule: every id / foreign-key field carries the entity's IMMUTABLE
// UUID, never a volatile DB primary key, so a backup round-trips stably across
// databases.

// INBFormatVersion is the manifest `version` for the JSON `.inb` format. Bumped
// only on a breaking change to the document shapes.
const INBFormatVersion = "2.0"

// INBManifestName is the inner-tar member holding the manifest document.
const INBManifestName = "manifest.json"

// INBUnassignedName is the inner-tar member holding the area-less ("unassigned")
// commodities document (issue #1986). Commodities with no area have no location
// home in the location-centric layout, so they are emitted here as a flat list.
// The member is written ONLY when at least one unassigned commodity is in scope,
// keeping archives with no unassigned items byte-stable (no empty member).
const INBUnassignedName = "unassigned-commodities.json"

// INBFilesPrefix is the inner-tar path prefix under which commodity file bytes
// are stored: `files/<loc-slug>/<commodity-uuid>/<bucket>/<sanitized-name>`.
const INBFilesPrefix = "files/"

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
	UnassignedFile string           `json:"unassignedFile,omitempty"`
	Statistics     INBManifestStats `json:"statistics"`
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
