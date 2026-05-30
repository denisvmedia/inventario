package types

import (
	"path"
	"strings"
	"time"

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
	Statistics  INBManifestStats `json:"statistics"`
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
	ID                     string       `json:"id"`
	Name                   string       `json:"name"`
	ShortName              string       `json:"shortName,omitempty"`
	Type                   string       `json:"type,omitempty"`
	AreaID                 string       `json:"areaId"`
	Count                  int          `json:"count"`
	OriginalPrice          string       `json:"originalPrice,omitempty"`
	OriginalPriceCurrency  string       `json:"originalPriceCurrency,omitempty"`
	ConvertedOriginalPrice string       `json:"convertedOriginalPrice,omitempty"`
	CurrentPrice           string       `json:"currentPrice,omitempty"`
	SerialNumber           string       `json:"serialNumber,omitempty"`
	ExtraSerialNumbers     []string     `json:"extraSerialNumbers,omitempty"`
	PartNumbers            []string     `json:"partNumbers,omitempty"`
	Tags                   []string     `json:"tags,omitempty"`
	Status                 string       `json:"status,omitempty"`
	PurchaseDate           string       `json:"purchaseDate,omitempty"`
	RegisteredDate         string       `json:"registeredDate,omitempty"`
	LastModifiedDate       string       `json:"lastModifiedDate,omitempty"`
	URLs                   []string     `json:"urls,omitempty"`
	Comments               string       `json:"comments,omitempty"`
	Draft                  bool         `json:"draft"`
	Images                 []INBFileRef `json:"images,omitempty"`
	Invoices               []INBFileRef `json:"invoices,omitempty"`
	Manuals                []INBFileRef `json:"manuals,omitempty"`
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
// left as the source UUID; the caller resolves it to the destination DB ID.
func (c *INBCommodity) ConvertToCommodity() *models.Commodity {
	commodity := &models.Commodity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			EntityID: models.EntityID{UUID: c.ID},
		},
		Name:                  c.Name,
		ShortName:             c.ShortName,
		Type:                  models.CommodityType(c.Type),
		AreaID:                c.AreaID,
		Count:                 c.Count,
		OriginalPrice:         parseDecimal(c.OriginalPrice),
		OriginalPriceCurrency: models.Currency(c.OriginalPriceCurrency),
		CurrentPrice:          parseDecimal(c.CurrentPrice),
		SerialNumber:          c.SerialNumber,
		Status:                models.CommodityStatus(c.Status),
		Comments:              c.Comments,
		Draft:                 c.Draft,
		ExtraSerialNumbers:    models.ValuerSlice[string](c.ExtraSerialNumbers),
		PartNumbers:           models.ValuerSlice[string](c.PartNumbers),
		Tags:                  models.ValuerSlice[string](c.Tags),
	}
	if c.ConvertedOriginalPrice != "" {
		commodity.ConvertedOriginalPrice = parseDecimal(c.ConvertedOriginalPrice)
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
	return commodity
}

// ConvertToFileEntity builds a models.FileEntity from an INBFileRef for a
// commodity attachment. linkedDBID is the destination commodity DB ID; bucket
// is the linked_entity_meta (images/invoices/manuals). blobKey is the re-minted
// destination blob key (under the importing tenant). The file's immutable UUID
// is preserved.
func (r *INBFileRef) ConvertToFileEntity(linkedDBID, bucket, blobKey string) *models.FileEntity {
	createdAt := parseInbTimestamp(r.CreatedAt)
	updatedAt := parseInbTimestamp(r.UpdatedAt)

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
	}
}

func parseDecimal(s string) decimal.Decimal {
	if s == "" {
		return decimal.Zero
	}
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

func parseInbTimestamp(s string) time.Time {
	if s == "" {
		return time.Now()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return time.Now()
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
