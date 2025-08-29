package types

import (
	"encoding/xml"
	"time"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
)

// RestoreOptions contains options for the restore operation
type RestoreOptions struct {
	Strategy        RestoreStrategy `json:"strategy"`
	IncludeFileData bool            `json:"include_file_data"`
	DryRun          bool            `json:"dry_run"`
}

// RestoreStrategy defines how to handle existing data during restore
type RestoreStrategy string

const (
	// RestoreStrategyFullReplace wipes the current database and restores everything from backup
	RestoreStrategyFullReplace RestoreStrategy = "full_replace"
	// RestoreStrategyMergeAdd only adds data from backup that is missing in current DB
	RestoreStrategyMergeAdd RestoreStrategy = "merge_add"
	// RestoreStrategyMergeUpdate creates if missing, updates if exists, leaves other records untouched
	RestoreStrategyMergeUpdate RestoreStrategy = "merge_update"
)

// RestoreStats tracks statistics during restore operation
type RestoreStats struct {
	LocationCount  int      `json:"location_count"`
	AreaCount      int      `json:"area_count"`
	CommodityCount int      `json:"commodity_count"`
	ImageCount     int      `json:"image_count"`
	InvoiceCount   int      `json:"invoice_count"`
	ManualCount    int      `json:"manual_count"`
	BinaryDataSize int64    `json:"binary_data_size"`
	ErrorCount     int      `json:"error_count"`
	Errors         []string `json:"errors"`
	SkippedCount   int      `json:"skipped_count"`
	UpdatedCount   int      `json:"updated_count"`
	CreatedCount   int      `json:"created_count"`
	DeletedCount   int      `json:"deleted_count"`
}

// IDMapping tracks the mapping from XML IDs to actual database IDs
type IDMapping struct {
	Locations   map[string]string // XML ID -> Database ID
	Areas       map[string]string // XML ID -> Database ID
	Commodities map[string]string // XML ID -> Database ID
	Images      map[string]string // XML ID -> Database ID
	Invoices    map[string]string // XML ID -> Database ID
	Manuals     map[string]string // XML ID -> Database ID
}

// PendingFileData holds file data that needs to be processed after commodity creation
type PendingFileData struct {
	FileType string    // "image", "invoice", "manual"
	XMLFiles []XMLFile // File data collected during parsing
}

// ExistingEntities tracks existing entities in the database
type ExistingEntities struct {
	Locations   map[string]*models.Location
	Areas       map[string]*models.Area
	Commodities map[string]*models.Commodity
	Images      map[string]*models.Image   // XML ID -> Image
	Invoices    map[string]*models.Invoice // XML ID -> Invoice
	Manuals     map[string]*models.Manual  // XML ID -> Manual
}

// XMLInventory represents the root element of the XML export
type XMLInventory struct {
	XMLName     xml.Name        `xml:"inventory"`
	Namespace   string          `xml:"xmlns,attr"`
	ExportDate  time.Time       `xml:"exportDate,attr"`
	ExportType  string          `xml:"exportType,attr"`
	Locations   *XMLLocations   `xml:"locations,omitempty"`
	Areas       *XMLAreas       `xml:"areas,omitempty"`
	Commodities *XMLCommodities `xml:"commodities,omitempty"`
}

// XMLLocations represents the locations section
type XMLLocations struct {
	XMLName   xml.Name      `xml:"locations"`
	Locations []XMLLocation `xml:"location"`
}

// XMLLocation represents a single location in XML
type XMLLocation struct {
	XMLName      xml.Name `xml:"location"`
	ID           string   `xml:"id,attr"`
	LocationName string   `xml:"locationName"`
	Address      string   `xml:"address,omitempty"`
}

// XMLAreas represents the areas section
type XMLAreas struct {
	XMLName xml.Name  `xml:"areas"`
	Areas   []XMLArea `xml:"area"`
}

// XMLArea represents a single area in XML
type XMLArea struct {
	XMLName    xml.Name `xml:"area"`
	ID         string   `xml:"id,attr"`
	AreaName   string   `xml:"areaName"`
	LocationID string   `xml:"locationId"`
}

// XMLCommodities represents the commodities section
type XMLCommodities struct {
	XMLName     xml.Name       `xml:"commodities"`
	Commodities []XMLCommodity `xml:"commodity"`
}

// XMLCommodity represents a single commodity in XML
type XMLCommodity struct {
	XMLName                xml.Name          `xml:"commodity"`
	ID                     string            `xml:"id,attr"`
	CommodityName          string            `xml:"commodityName"`
	ShortName              string            `xml:"shortName,omitempty"`
	AreaID                 string            `xml:"areaId"`
	Count                  int               `xml:"count,omitempty"`
	Status                 string            `xml:"status,omitempty"`
	Type                   string            `xml:"type,omitempty"`
	OriginalPrice          string            `xml:"originalPrice,omitempty"`
	OriginalCurrency       string            `xml:"originalPriceCurrency,omitempty"`
	ConvertedOriginalPrice string            `xml:"convertedOriginalPrice,omitempty"`
	CurrentPrice           string            `xml:"currentPrice,omitempty"`
	CurrentCurrency        string            `xml:"currentCurrency,omitempty"`
	SerialNumber           string            `xml:"serialNumber,omitempty"`
	ExtraSerialNumbers     *XMLSerialNumbers `xml:"extraSerialNumbers,omitempty"`
	Comments               string            `xml:"comments,omitempty"`
	Draft                  bool              `xml:"draft,omitempty"`
	PurchaseDate           string            `xml:"purchaseDate,omitempty"`
	RegisteredDate         string            `xml:"registeredDate,omitempty"`
	LastModifiedDate       string            `xml:"lastModifiedDate,omitempty"`
	PartNumbers            *XMLPartNumbers   `xml:"partNumbers,omitempty"`
	Tags                   *XMLTags          `xml:"tags,omitempty"`
	URLs                   *XMLURLs          `xml:"urls,omitempty"`
	Images                 *XMLImages        `xml:"images,omitempty"`
	Invoices               *XMLInvoices      `xml:"invoices,omitempty"`
	Manuals                *XMLManuals       `xml:"manuals,omitempty"`
}

// XMLSerialNumbers represents extra serial numbers
type XMLSerialNumbers struct {
	XMLName       xml.Name `xml:"extraSerialNumbers"`
	SerialNumbers []string `xml:"serialNumber"`
}

// XMLPartNumbers represents part numbers
type XMLPartNumbers struct {
	XMLName     xml.Name `xml:"partNumbers"`
	PartNumbers []string `xml:"partNumber"`
}

// XMLTags represents tags
type XMLTags struct {
	XMLName xml.Name `xml:"tags"`
	Tags    []string `xml:"tag"`
}

// XMLURLs represents URLs
type XMLURLs struct {
	XMLName xml.Name `xml:"urls"`
	URLs    []string `xml:"url"`
}

// XMLImages represents the images section
type XMLImages struct {
	XMLName xml.Name   `xml:"images"`
	Images  []XMLImage `xml:"image"`
}

// XMLImage represents a single image
type XMLImage struct {
	XMLName      xml.Name `xml:"image"`
	ID           string   `xml:"id,attr"`
	Path         string   `xml:"path"`
	OriginalPath string   `xml:"originalPath"`
	Extension    string   `xml:"extension"`
	MimeType     string   `xml:"mimeType"`
	Data         []byte   `xml:"data,omitempty"`
}

// XMLInvoices represents the invoices section
type XMLInvoices struct {
	XMLName  xml.Name     `xml:"invoices"`
	Invoices []XMLInvoice `xml:"invoice"`
}

// XMLInvoice represents a single invoice
type XMLInvoice struct {
	XMLName      xml.Name `xml:"invoice"`
	ID           string   `xml:"id,attr"`
	Path         string   `xml:"path"`
	OriginalPath string   `xml:"originalPath"`
	Extension    string   `xml:"extension"`
	MimeType     string   `xml:"mimeType"`
	Data         []byte   `xml:"data,omitempty"`
}

// XMLManuals represents the manuals section
type XMLManuals struct {
	XMLName xml.Name    `xml:"manuals"`
	Manuals []XMLManual `xml:"manual"`
}

// XMLManual represents a single manual
type XMLManual struct {
	XMLName      xml.Name `xml:"manual"`
	ID           string   `xml:"id,attr"`
	Path         string   `xml:"path"`
	OriginalPath string   `xml:"originalPath"`
	Extension    string   `xml:"extension"`
	MimeType     string   `xml:"mimeType"`
	Data         []byte   `xml:"data,omitempty"`
}

// XMLFile represents a single file with embedded base64 data
type XMLFile struct {
	ID           string `xml:"id,attr"`
	Path         string `xml:"path"`
	OriginalPath string `xml:"originalPath"`
	Extension    string `xml:"extension"`
	MimeType     string `xml:"mimeType"`
	Data         []byte `xml:"data,omitempty"`
}

// ConvertToLocation converts XMLLocation to models.Location
func (xl *XMLLocation) ConvertToLocation() *models.Location {
	return &models.Location{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: xl.ID},
			TenantID: "default-tenant", // TODO: Get from context
		},
		Name:    xl.LocationName,
		Address: xl.Address,
	}
}

// ConvertToArea converts XMLArea to models.Area
func (xa *XMLArea) ConvertToArea() *models.Area {
	return &models.Area{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: xa.ID},
			TenantID: "default-tenant", // TODO: Get from context
		},
		Name:       xa.AreaName,
		LocationID: xa.LocationID,
	}
}

// ConvertToCommodity converts XMLCommodity to models.Commodity
func (xc *XMLCommodity) ConvertToCommodity() (*models.Commodity, error) {
	commodity := &models.Commodity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: xc.ID},
			TenantID: "default-tenant", // TODO: Get from context
		},
		Name:      xc.CommodityName,
		ShortName: xc.ShortName,
		AreaID:    xc.AreaID,
		Count:     xc.Count,
		Status:    models.CommodityStatus(xc.Status),
		Comments:  xc.Comments,
		Draft:     xc.Draft,
	}

	// Convert commodity type
	commodity.Type = models.CommodityType(xc.Type)

	// Convert prices
	if xc.OriginalPrice != "" {
		if price, err := decimal.NewFromString(xc.OriginalPrice); err == nil {
			commodity.OriginalPrice = price
		}
	}
	if xc.ConvertedOriginalPrice != "" {
		if price, err := decimal.NewFromString(xc.ConvertedOriginalPrice); err == nil {
			commodity.ConvertedOriginalPrice = price
		}
	}
	if xc.CurrentPrice != "" {
		if price, err := decimal.NewFromString(xc.CurrentPrice); err == nil {
			commodity.CurrentPrice = price
		}
	}

	// Convert currencies
	if xc.OriginalCurrency != "" {
		commodity.OriginalPriceCurrency = models.Currency(xc.OriginalCurrency)
	}

	// Convert serial numbers
	commodity.SerialNumber = xc.SerialNumber
	if xc.ExtraSerialNumbers != nil {
		commodity.ExtraSerialNumbers = models.ValuerSlice[string](xc.ExtraSerialNumbers.SerialNumbers)
	}

	// Convert part numbers
	if xc.PartNumbers != nil {
		commodity.PartNumbers = models.ValuerSlice[string](xc.PartNumbers.PartNumbers)
	}

	// Convert tags
	if xc.Tags != nil {
		commodity.Tags = models.ValuerSlice[string](xc.Tags.Tags)
	}

	// Convert URLs
	if xc.URLs != nil {
		urls := make([]*models.URL, 0, len(xc.URLs.URLs))
		for _, urlStr := range xc.URLs.URLs {
			if url, err := models.URLParse(urlStr); err == nil {
				urls = append(urls, url)
			}
		}
		commodity.URLs = models.ValuerSlice[*models.URL](urls)
	}

	// Convert dates
	if xc.PurchaseDate != "" {
		date := models.Date(xc.PurchaseDate)
		commodity.PurchaseDate = models.ToPDate(date)
	}
	if xc.RegisteredDate != "" {
		date := models.Date(xc.RegisteredDate)
		commodity.RegisteredDate = models.ToPDate(date)
	}
	if xc.LastModifiedDate != "" {
		date := models.Date(xc.LastModifiedDate)
		commodity.LastModifiedDate = models.ToPDate(date)
	}

	return commodity, nil
}

// ConvertToFile converts XMLFile to models.File
func (xf *XMLFile) ConvertToFile() *models.File {
	return &models.File{
		Path:         xf.Path,
		OriginalPath: xf.OriginalPath,
		Ext:          xf.Extension,
		MIMEType:     xf.MimeType,
	}
}
