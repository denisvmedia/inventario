package types

import (
	"path"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
)

// This file mirrors the JSON document schemas the `.inb` exporter writes (see
// backup/export/inb_types.go) for the restore-side decode (issue #534). The two
// packages can't share a type (export must not depend on restore and vice
// versa), so the shapes are duplicated and MUST stay in sync.

// INBManifestMember is the inner-tar member name of the manifest document. Kept
// in sync with backup/export.INBManifestName.
const INBManifestMember = "manifest.json"

// INBUnassignedMember is the inner-tar member name of the area-less ("unassigned")
// commodities document (issue #1986). Kept in sync with
// backup/export.INBUnassignedName. Absent from archives with no unassigned items.
const INBUnassignedMember = "unassigned-commodities.json"

// INBManifest is the decoded manifest.json. Restore only reads the statistics
// and the location index; the signature block is informational (verification
// uses the server's own key, never this).
type INBManifest struct {
	ExportDate  string           `json:"exportDate"`
	ExportType  string           `json:"exportType"`
	Version     string           `json:"version"`
	Format      string           `json:"format"`
	Compression string           `json:"compression"`
	Signature   INBSignatureInfo `json:"signature"`
	Locations   []INBManifestLoc `json:"locations"`
	// UnassignedFile names the area-less commodities member (issue #1986), or ""
	// when the archive carries none. Informational here — restore dispatches by
	// member name (handleMember), so older archives without this field and the
	// member round-trip unchanged.
	UnassignedFile string           `json:"unassignedFile,omitempty"`
	Statistics     INBManifestStats `json:"statistics"`
}

// INBSignatureInfo is the informational signature descriptor.
type INBSignatureInfo struct {
	Algorithm   string `json:"algorithm"`
	PublicKey   string `json:"publicKey"`
	Fingerprint string `json:"fingerprint"`
}

// INBManifestLoc is one manifest location index entry.
type INBManifestLoc struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	File string `json:"file"`
}

// INBManifestStats mirrors the export statistics.
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

// INBLocationDoc is a decoded per-location document.
type INBLocationDoc struct {
	Location    INBLocation    `json:"location"`
	Areas       []INBArea      `json:"areas"`
	Commodities []INBCommodity `json:"commodities"`
}

// INBUnassignedDoc is the decoded area-less commodities document (issue #1986):
// a flat list of commodities whose AreaID is "" (mapped to a nil model AreaID by
// ConvertToCommodity). Mirrors backup/export.INBUnassignedDoc.
type INBUnassignedDoc struct {
	Commodities []INBCommodity `json:"commodities"`
}

// INBLocation is a location row keyed by immutable UUID.
type INBLocation struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address,omitempty"`
}

// INBArea is an area row; LocationID is the parent location UUID.
type INBArea struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LocationID string `json:"locationId"`
}

// INBCommodity is a commodity row keyed by immutable UUID; AreaID is the parent
// area UUID. The three file-reference arrays carry the attached files.
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

	// Extended commodity fields (#534 round-trip parity). MUST keep json tags
	// identical to backup/export.INBCommodity. Warranty (#1554), terminal-status
	// metadata (#1611), acquisition provenance (#202) and the user-picked cover
	// photo (#1451) round-trip so a restored commodity is a lossless copy.
	WarrantyExpiresAt string `json:"warrantyExpiresAt,omitempty"`
	WarrantyNotes     string `json:"warrantyNotes,omitempty"`
	StatusDate        string `json:"statusDate,omitempty"`
	StatusNote        string `json:"statusNote,omitempty"`
	SalePrice         string `json:"salePrice,omitempty"`
	// AcquisitionPrice / AcquisitionCurrency are restored through the trusted
	// registry.WithRestoreAcquisition context seam (the normal write path nils
	// them), so ConvertToCommodity deliberately does NOT set them on the model —
	// the processor reads them off the DTO (RestoredAcquisition) and signals the
	// pair to Create via context, which writes it onto the fresh row.
	AcquisitionPrice    string `json:"acquisitionPrice,omitempty"`
	AcquisitionCurrency string `json:"acquisitionCurrency,omitempty"`
	// CoverFileID is the cover photo's immutable UUID; the processor re-resolves
	// it to the new file's DB id and patches the commodity after its files are
	// created (the file does not yet exist when the commodity row is created).
	CoverFileID string `json:"coverFileId,omitempty"`

	Images   []INBFileRef `json:"images,omitempty"`
	Invoices []INBFileRef `json:"invoices,omitempty"`
	Manuals  []INBFileRef `json:"manuals,omitempty"`
}

// INBFileRef references a commodity-attached file. Path is the file's location
// inside the inner tar; the bytes follow as a separate tar member at that path.
// Name is the file's user-facing basename (the original File.Path stem, without
// extension) — kept distinct from Title so a restore reproduces the original
// filename rather than collapsing it onto the display title or the UUID.
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

// ConvertToLocation converts an INBLocation to a models.Location. The ID is the
// immutable UUID; the caller stamps tenant/group/user context.
func (l *INBLocation) ConvertToLocation() *models.Location {
	return &models.Location{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{UUID: l.ID},
		},
		Name:    l.Name,
		Address: l.Address,
	}
}

// ConvertToArea converts an INBArea to a models.Area. LocationID is left as the
// source UUID; the caller resolves it to the destination DB ID via IDMapping.
func (a *INBArea) ConvertToArea() *models.Area {
	return &models.Area{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{UUID: a.ID},
		},
		Name:       a.Name,
		LocationID: a.LocationID,
	}
}

// ConvertToCommodity converts an INBCommodity to a models.Commodity. AreaID is
// left as the source UUID (mapped at the boundary: "" → nil for an area-less
// commodity, issue #1986; non-empty → a set pointer the caller then resolves to
// the destination DB ID). A malformed numeric field surfaces an error (tagged
// with the field name and the commodity UUID) rather than being silently coerced
// to zero, so a corrupt archive fails the restore instead of restoring wrong data.
func (c *INBCommodity) ConvertToCommodity() (*models.Commodity, error) {
	originalPrice, err := parseDecimal(c.OriginalPrice)
	if err != nil {
		return nil, errxtrace.Wrap("invalid originalPrice", err, errx.Attrs("commodity_id", c.ID))
	}
	currentPrice, err := parseDecimal(c.CurrentPrice)
	if err != nil {
		return nil, errxtrace.Wrap("invalid currentPrice", err, errx.Attrs("commodity_id", c.ID))
	}

	commodity := &models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{UUID: c.ID},
		},
		Name:                  c.Name,
		ShortName:             c.ShortName,
		Type:                  models.CommodityType(c.Type),
		AreaID:                inbAreaIDPtr(c.AreaID),
		Count:                 c.Count,
		OriginalPrice:         originalPrice,
		OriginalPriceCurrency: models.Currency(c.OriginalPriceCurrency),
		CurrentPrice:          currentPrice,
		SerialNumber:          c.SerialNumber,
		Status:                models.CommodityStatus(c.Status),
		Comments:              c.Comments,
		Draft:                 c.Draft,
		ExtraSerialNumbers:    models.ValuerSlice[string](c.ExtraSerialNumbers),
		PartNumbers:           models.ValuerSlice[string](c.PartNumbers),
		Tags:                  models.ValuerSlice[string](c.Tags),
	}
	if c.ConvertedOriginalPrice != "" {
		converted, cErr := parseDecimal(c.ConvertedOriginalPrice)
		if cErr != nil {
			return nil, errxtrace.Wrap("invalid convertedOriginalPrice", cErr, errx.Attrs("commodity_id", c.ID))
		}
		commodity.ConvertedOriginalPrice = converted
	}
	if len(c.URLs) > 0 {
		urls := make([]*models.URL, 0, len(c.URLs))
		for _, raw := range c.URLs {
			if u, err := models.URLParse(raw); err == nil {
				urls = append(urls, u)
			}
		}
		commodity.URLs = models.ValuerSlice[*models.URL](urls)
	}
	if c.PurchaseDate != "" {
		commodity.PurchaseDate = models.ToPDate(models.Date(c.PurchaseDate))
	}
	if c.RegisteredDate != "" {
		commodity.RegisteredDate = models.ToPDate(models.Date(c.RegisteredDate))
	}
	if c.LastModifiedDate != "" {
		commodity.LastModifiedDate = models.ToPDate(models.Date(c.LastModifiedDate))
	}

	// Extended commodity fields (#534). Warranty + terminal-status metadata are
	// plain columns the registry writes verbatim. Acquisition and cover are
	// handled by the processor (restore-only seam / post-files patch), so they
	// are intentionally NOT set here.
	if c.WarrantyExpiresAt != "" {
		commodity.WarrantyExpiresAt = models.ToPDate(models.Date(c.WarrantyExpiresAt))
	}
	commodity.WarrantyNotes = c.WarrantyNotes
	if c.StatusDate != "" {
		commodity.StatusDate = models.ToPDate(models.Date(c.StatusDate))
	}
	commodity.StatusNote = c.StatusNote
	salePrice, err := parseDecimalPtr(c.SalePrice)
	if err != nil {
		return nil, errxtrace.Wrap("invalid salePrice", err, errx.Attrs("commodity_id", c.ID))
	}
	commodity.SalePrice = salePrice

	return commodity, nil
}

// RestoredAcquisition returns the acquisition provenance pair (#202) decoded
// from the archive, or (nil, nil) when the archive carried neither. A
// non-empty-but-unparseable price surfaces an error so a corrupt archive fails
// the restore rather than silently dropping the frozen acquisition history. The
// caller signals the pair to Create via the trusted registry.WithRestoreAcquisition
// context seam, never the normal write path.
func (c *INBCommodity) RestoredAcquisition() (*decimal.Decimal, *models.Currency, error) {
	price, err := parseDecimalPtr(c.AcquisitionPrice)
	if err != nil {
		return nil, nil, errxtrace.Wrap("invalid acquisitionPrice", err, errx.Attrs("commodity_id", c.ID))
	}
	hasPrice := price != nil
	hasCurrency := c.AcquisitionCurrency != ""
	switch {
	case !hasPrice && !hasCurrency:
		// No acquisition provenance in the archive.
		return nil, nil, nil
	case hasPrice != hasCurrency:
		// The DB CHECK enforces both-or-neither, so a one-sided pair is a corrupt
		// archive — fail the restore rather than silently dropping the provenance.
		return nil, nil, errxtrace.ClassifyNew(
			"acquisition provenance is half-present (need both acquisitionPrice and acquisitionCurrency)",
			errx.Attrs("commodity_id", c.ID),
		)
	}
	currency := models.Currency(c.AcquisitionCurrency)
	return price, &currency, nil
}

// ConvertToFileEntity builds a models.FileEntity from an INBFileRef for a
// commodity attachment. linkedDBID is the destination commodity DB ID; bucket
// is the linked_entity_meta (images/invoices/manuals). blobKey is the re-minted
// destination blob key (under the importing tenant). The file's immutable UUID
// is preserved. A malformed createdAt/updatedAt timestamp surfaces an error
// (tagged with the field name and the file UUID) rather than being coerced to
// time.Now(), so a corrupt archive fails the restore instead of restoring wrong
// metadata.
func (r *INBFileRef) ConvertToFileEntity(linkedDBID, bucket, blobKey string) (*models.FileEntity, error) {
	createdAt, err := parseInbTimestamp(r.CreatedAt)
	if err != nil {
		return nil, errxtrace.Wrap("invalid createdAt", err, errx.Attrs("file_id", r.ID))
	}
	updatedAt, err := parseInbTimestamp(r.UpdatedAt)
	if err != nil {
		return nil, errxtrace.Wrap("invalid updatedAt", err, errx.Attrs("file_id", r.ID))
	}

	fileType := models.FileTypeFromMIME(r.MimeType)
	category := models.FileCategoryFromContext("commodity", bucket, r.MimeType)

	return &models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{UUID: r.ID},
		},
		Title:            r.Title,
		Description:      r.Description,
		Type:             fileType,
		Category:         category,
		Tags:             models.StringSlice(r.Tags),
		LinkedEntityType: "commodity",
		LinkedEntityID:   linkedDBID,
		LinkedEntityMeta: bucket,
		CreatedAt:        createdAt,
		UpdatedAt:        updatedAt,
		File: &models.File{
			Path:         fileBaseName(r),
			OriginalPath: blobKey,
			Ext:          r.Extension,
			MIMEType:     r.MimeType,
		},
	}, nil
}

// inbAreaIDPtr maps the wire AreaID to the model's nullable *string (issue
// #1986): "" (an area-less commodity) becomes nil, a UUID becomes a set pointer
// the caller resolves to the destination DB ID. The processor mirrors this at
// the model level (nil → unassigned path, set → area-mapped path).
func inbAreaIDPtr(areaUUID string) *string {
	if areaUUID == "" {
		return nil
	}
	return &areaUUID
}

// parseDecimal parses a decimal price string. An absent value (empty string) is
// valid and yields decimal.Zero; a non-empty but unparseable value surfaces an
// error rather than being silently coerced to zero, which would corrupt the
// restored price.
func parseDecimal(s string) (decimal.Decimal, error) {
	if s == "" {
		return decimal.Zero, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero, errxtrace.Wrap("failed to parse decimal", err, errx.Attrs("value", s))
	}
	return d, nil
}

// parseDecimalPtr parses a nullable decimal column (SalePrice / AcquisitionPrice).
// An absent value (empty string) yields a nil pointer — the column stays NULL —
// while a non-empty but unparseable value surfaces an error rather than being
// coerced. A legitimate "0" round-trips as a set pointer holding zero, mirroring
// the exporter's nil-vs-zero distinction.
func parseDecimalPtr(s string) (*decimal.Decimal, error) {
	if s == "" {
		return nil, nil
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return nil, errxtrace.Wrap("failed to parse decimal", err, errx.Attrs("value", s))
	}
	return &d, nil
}

// parseInbTimestamp parses an RFC3339 timestamp. An absent value (empty string)
// is valid and yields the zero time.Time; a non-empty but unparseable value
// surfaces an error rather than being silently coerced to time.Now(), which
// would fabricate a restored timestamp.
func parseInbTimestamp(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}, errxtrace.Wrap("failed to parse timestamp", err, errx.Attrs("value", s))
	}
	return t, nil
}

// fileBaseName derives the user-facing Path (filename without extension) for a
// restored file. The exporter records the original stem in Name; if an older
// archive lacks it we recover the stem from the archive member basename (Path
// minus extension), then fall back to the display Title, then the UUID.
func fileBaseName(r *INBFileRef) string {
	if r.Name != "" {
		return r.Name
	}
	if base := strings.TrimSuffix(path.Base(r.Path), r.Extension); base != "" && base != "." && base != "/" {
		return base
	}
	if r.Title != "" {
		return r.Title
	}
	return r.ID
}
