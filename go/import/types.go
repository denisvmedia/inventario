package importpkg

import (
	"encoding/xml"
	"time"

	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
)

// XMLInventory represents the root element of the XML export
type XMLInventory struct {
	XMLName    xml.Name       `xml:"inventory"`
	Namespace  string         `xml:"xmlns,attr"`
	ExportDate time.Time      `xml:"exportDate,attr"`
	ExportType string         `xml:"exportType,attr"`
	Locations  *XMLLocations  `xml:"locations,omitempty"`
	Areas      *XMLAreas      `xml:"areas,omitempty"`
	Commodities *XMLCommodities `xml:"commodities,omitempty"`
}

// XMLLocations represents the locations section
type XMLLocations struct {
	XMLName   xml.Name      `xml:"locations"`
	Locations []XMLLocation `xml:"location"`
}

// XMLLocation represents a single location
type XMLLocation struct {
	XMLName      xml.Name `xml:"location"`
	ID           string   `xml:"id,attr"`
	LocationName string   `xml:"locationName"`
	Address      string   `xml:"address"`
}

// XMLAreas represents the areas section
type XMLAreas struct {
	XMLName xml.Name  `xml:"areas"`
	Areas   []XMLArea `xml:"area"`
}

// XMLArea represents a single area
type XMLArea struct {
	XMLName      xml.Name `xml:"area"`
	ID           string   `xml:"id,attr"`
	LocationID   string   `xml:"locationId"`
	AreaName     string   `xml:"areaName"`
	Description  string   `xml:"description"`
}

// XMLCommodities represents the commodities section
type XMLCommodities struct {
	XMLName     xml.Name       `xml:"commodities"`
	Commodities []XMLCommodity `xml:"commodity"`
}

// XMLCommodity represents a single commodity
type XMLCommodity struct {
	XMLName            xml.Name           `xml:"commodity"`
	ID                 string             `xml:"id,attr"`
	AreaID             string             `xml:"areaId"`
	Name               string             `xml:"name"`
	Description        string             `xml:"description"`
	Category           string             `xml:"category"`
	Status             string             `xml:"status"`
	Condition          string             `xml:"condition"`
	Brand              string             `xml:"brand"`
	Model              string             `xml:"model"`
	SerialNumber       string             `xml:"serialNumber"`
	OriginalPrice      *XMLPrice          `xml:"originalPrice,omitempty"`
	CurrentPrice       *XMLPrice          `xml:"currentPrice,omitempty"`
	PurchaseDate       string             `xml:"purchaseDate,omitempty"`
	RegisteredDate     string             `xml:"registeredDate,omitempty"`
	LastModifiedDate   string             `xml:"lastModifiedDate,omitempty"`
	PartNumbers        *XMLPartNumbers    `xml:"partNumbers,omitempty"`
	Tags               *XMLTags           `xml:"tags,omitempty"`
	URLs               *XMLURLs           `xml:"urls,omitempty"`
	Images             *XMLImages         `xml:"images,omitempty"`
	Invoices           *XMLInvoices       `xml:"invoices,omitempty"`
	Manuals            *XMLManuals        `xml:"manuals,omitempty"`
}

// XMLPrice represents a price with currency
type XMLPrice struct {
	XMLName  xml.Name        `xml:",any"`
	Amount   decimal.Decimal `xml:"amount"`
	Currency string          `xml:"currency"`
}

// XMLPartNumbers represents a collection of part numbers
type XMLPartNumbers struct {
	XMLName     xml.Name `xml:"partNumbers"`
	PartNumbers []string `xml:"partNumber"`
}

// XMLTags represents a collection of tags
type XMLTags struct {
	XMLName xml.Name `xml:"tags"`
	Tags    []string `xml:"tag"`
}

// XMLURLs represents a collection of URLs
type XMLURLs struct {
	XMLName xml.Name `xml:"urls"`
	URLs    []string `xml:"url"`
}

// XMLImages represents a collection of images
type XMLImages struct {
	XMLName xml.Name  `xml:"images"`
	Images  []XMLFile `xml:"image"`
}

// XMLInvoices represents a collection of invoices
type XMLInvoices struct {
	XMLName  xml.Name  `xml:"invoices"`
	Invoices []XMLFile `xml:"invoice"`
}

// XMLManuals represents a collection of manuals
type XMLManuals struct {
	XMLName xml.Name  `xml:"manuals"`
	Manuals []XMLFile `xml:"manual"`
}

// XMLFile represents a single file with embedded base64 data
type XMLFile struct {
	XMLName      xml.Name `xml:",any"`
	ID           string   `xml:"id,attr"`
	Path         string   `xml:"path"`
	OriginalPath string   `xml:"originalPath"`
	Extension    string   `xml:"extension"`
	MimeType     string   `xml:"mimeType"`
	Data         []byte   `xml:"data,omitempty"`
}

// ImportStats tracks statistics during import
type ImportStats struct {
	LocationCount  int
	AreaCount      int
	CommodityCount int
	ImageCount     int
	InvoiceCount   int
	ManualCount    int
	BinaryDataSize int64
	ErrorCount     int
	Errors         []string
}

// ImportProgress represents the current progress of an import operation
type ImportProgress struct {
	Phase           string  `json:"phase"`
	CurrentItem     string  `json:"current_item"`
	ProcessedItems  int     `json:"processed_items"`
	TotalItems      int     `json:"total_items"`
	PercentComplete float64 `json:"percent_complete"`
	Stats           ImportStats `json:"stats"`
}

// ConvertToLocation converts XMLLocation to models.Location
func (xl *XMLLocation) ConvertToLocation() *models.Location {
	return &models.Location{
		EntityID: models.EntityID{ID: xl.ID},
		Name:     xl.LocationName,
		Address:  xl.Address,
	}
}

// ConvertToArea converts XMLArea to models.Area
func (xa *XMLArea) ConvertToArea() *models.Area {
	return &models.Area{
		EntityID:   models.EntityID{ID: xa.ID},
		LocationID: xa.LocationID,
		Name:       xa.AreaName,
	}
}

// ConvertToCommodity converts XMLCommodity to models.Commodity
func (xc *XMLCommodity) ConvertToCommodity() (*models.Commodity, error) {
	commodity := &models.Commodity{
		EntityID:     models.EntityID{ID: xc.ID},
		AreaID:       xc.AreaID,
		Name:         xc.Name,
		ShortName:    xc.Name, // Use name as short name for now
		Type:         models.CommodityTypeOther, // Default type
		Status:       models.CommodityStatus(xc.Status),
		SerialNumber: xc.SerialNumber,
		Count:        1, // Default count
		Comments:     xc.Description,
	}

	// Convert prices
	if xc.OriginalPrice != nil {
		commodity.OriginalPrice = xc.OriginalPrice.Amount
		commodity.OriginalPriceCurrency = models.Currency(xc.OriginalPrice.Currency)
	}
	if xc.CurrentPrice != nil {
		commodity.CurrentPrice = xc.CurrentPrice.Amount
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
