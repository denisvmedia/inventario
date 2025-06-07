package export

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ExportService handles the background processing of export requests
type ExportService struct {
	registrySet    *registry.Set
	uploadLocation string
}

// ExportArgs contains arguments for export operations
type ExportArgs struct {
	IncludeFileData bool
}

// NewExportService creates a new export service
func NewExportService(registrySet *registry.Set, uploadLocation string) *ExportService {
	return &ExportService{
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
	}
}

// InventoryData represents the root XML structure for exports
type InventoryData struct {
	XMLName     xml.Name     `xml:"inventory"`
	ExportDate  string       `xml:"exportDate,attr"`
	ExportType  string       `xml:"exportType,attr"`
	Locations   []*Location  `xml:"locations>location,omitempty"`
	Areas       []*Area      `xml:"areas>area,omitempty"`
	Commodities []*Commodity `xml:"commodities>commodity,omitempty"`
}

type Location struct {
	XMLName xml.Name `xml:"location"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"locationName"`
	Address string   `xml:"address"`
}

type Area struct {
	XMLName    xml.Name `xml:"area"`
	ID         string   `xml:"id,attr"`
	Name       string   `xml:"areaName"`
	LocationID string   `xml:"locationId"`
}

type Commodity struct {
	XMLName                xml.Name `xml:"commodity"`
	ID                     string   `xml:"id,attr"`
	Name                   string   `xml:"commodityName"`
	ShortName              string   `xml:"shortName,omitempty"`
	Type                   string   `xml:"type"`
	AreaID                 string   `xml:"areaId"`
	Count                  int      `xml:"count"`
	OriginalPrice          string   `xml:"originalPrice,omitempty"`
	OriginalPriceCurrency  string   `xml:"originalPriceCurrency,omitempty"`
	ConvertedOriginalPrice string   `xml:"convertedOriginalPrice,omitempty"`
	CurrentPrice           string   `xml:"currentPrice,omitempty"`
	SerialNumber           string   `xml:"serialNumber,omitempty"`
	ExtraSerialNumbers     []string `xml:"extraSerialNumbers>serialNumber,omitempty"`
	PartNumbers            []string `xml:"partNumbers>partNumber,omitempty"`
	Tags                   []string `xml:"tags>tag,omitempty"`
	Status                 string   `xml:"status"`
	PurchaseDate           string   `xml:"purchaseDate,omitempty"`
	RegisteredDate         string   `xml:"registeredDate,omitempty"`
	LastModifiedDate       string   `xml:"lastModifiedDate,omitempty"`
	URLs                   []*URL   `xml:"urls>url,omitempty"`
	Comments               string   `xml:"comments,omitempty"`
	Draft                  bool     `xml:"draft"`
	Images                 []*File  `xml:"images>image,omitempty"`
	Invoices               []*File  `xml:"invoices>invoice,omitempty"`
	Manuals                []*File  `xml:"manuals>manual,omitempty"`
}

type URL struct {
	XMLName xml.Name `xml:"url"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
}

type File struct {
	XMLName      xml.Name `xml:"file"`
	ID           string   `xml:"id,attr"`
	Path         string   `xml:"path"`
	OriginalPath string   `xml:"originalPath"`
	Extension    string   `xml:"extension"`
	MimeType     string   `xml:"mimeType"`
	Data         string   `xml:"data,omitempty"` // Base64 encoded file data if include_file_data is true
}

// ProcessExport processes an export request in the background
func (s *ExportService) ProcessExport(ctx context.Context, exportID string) error {
	// Get the export request
	export, err := s.registrySet.ExportRegistry.Get(ctx, exportID)
	if err != nil {
		return errkit.Wrap(err, "failed to get export")
	}

	// Update status to in_progress
	export.Status = models.ExportStatusInProgress
	_, err = s.registrySet.ExportRegistry.Update(ctx, *export)
	if err != nil {
		return errkit.Wrap(err, "failed to update export status")
	}

	// Generate the export
	filePath, err := s.generateExport(ctx, *export)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		s.registrySet.ExportRegistry.Update(ctx, *export)
		return errkit.Wrap(err, "failed to generate export")
	}

	// Update status to completed
	export.Status = models.ExportStatusCompleted
	export.FilePath = filePath
	completedDate := models.Date(time.Now().Format("2006-01-02"))
	export.CompletedDate = &completedDate
	export.ErrorMessage = ""

	_, err = s.registrySet.ExportRegistry.Update(ctx, *export)
	if err != nil {
		return errkit.Wrap(err, "failed to update export completion")
	}

	return nil
}

// generateExport generates the XML export file using blob storage
func (s *ExportService) generateExport(ctx context.Context, export models.Export) (string, error) {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return "", errkit.Wrap(err, "failed to open blob bucket")
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			err = errkit.Wrap(closeErr, "failed to close blob bucket")
		}
	}()

	// Generate blob key (filename)
	timestamp := time.Now().Format("20060102_150405")
	blobKey := fmt.Sprintf("exports/export_%s_%s.xml", strings.ToLower(string(export.Type)), timestamp)

	// Create blob writer
	writer, err := b.NewWriter(ctx, blobKey, nil)
	if err != nil {
		return "", errkit.Wrap(err, "failed to create blob writer")
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			err = errkit.Wrap(closeErr, "failed to close blob writer")
		}
	}()

	// Stream XML generation
	if err := s.streamXMLExport(ctx, export, writer); err != nil {
		return "", errkit.Wrap(err, "failed to generate XML export")
	}

	return blobKey, nil
}

// streamXMLExport streams XML data directly to the file writer
func (s *ExportService) streamXMLExport(ctx context.Context, export models.Export, writer io.Writer) error {
	// Write XML header
	if _, err := writer.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")); err != nil {
		return errkit.Wrap(err, "failed to write XML header")
	}

	// Start root element
	exportDate := time.Now().Format("2006-01-02T15:04:05Z")
	rootStart := fmt.Sprintf(`<inventory exportDate="%s" exportType="%s">%s`, exportDate, string(export.Type), "\n")
	if _, err := writer.Write([]byte(rootStart)); err != nil {
		return errkit.Wrap(err, "failed to write root element")
	}

	switch export.Type {
	case models.ExportTypeFullDatabase:
		args := ExportArgs{IncludeFileData: export.IncludeFileData}
		if err := s.streamFullDatabase(ctx, writer, args); err != nil {
			return errkit.Wrap(err, "failed to stream full database")
		}
	case models.ExportTypeLocations:
		if err := s.streamLocations(ctx, writer); err != nil {
			return errkit.Wrap(err, "failed to stream locations")
		}
	case models.ExportTypeAreas:
		if err := s.streamAreas(ctx, writer); err != nil {
			return errkit.Wrap(err, "failed to stream areas")
		}
	case models.ExportTypeCommodities:
		args := ExportArgs{IncludeFileData: export.IncludeFileData}
		if err := s.streamCommodities(ctx, writer, args); err != nil {
			return errkit.Wrap(err, "failed to stream commodities")
		}
	case models.ExportTypeSelectedItems:
		args := ExportArgs{IncludeFileData: export.IncludeFileData}
		if err := s.streamSelectedItems(ctx, writer, export.SelectedItems, args); err != nil {
			return errkit.Wrap(err, "failed to stream selected items")
		}
	default:
		return errkit.WithFields(ErrUnsupportedExportType, "type", export.Type)
	}

	// End root element
	if _, err := writer.Write([]byte("</inventory>\n")); err != nil {
		return errkit.Wrap(err, "failed to write closing root element")
	}

	return nil
}

// streamFullDatabase streams all database content to the writer
func (s *ExportService) streamFullDatabase(ctx context.Context, writer io.Writer, args ExportArgs) error {
	if err := s.streamLocations(ctx, writer); err != nil {
		return errkit.Wrap(err, "failed to stream locations")
	}
	if err := s.streamAreas(ctx, writer); err != nil {
		return errkit.Wrap(err, "failed to stream areas")
	}
	if err := s.streamCommodities(ctx, writer, args); err != nil {
		return errkit.Wrap(err, "failed to stream commodities")
	}
	return nil
}

// streamLocations streams locations to the writer
//
//nolint:dupl // streamLocations and streamAreas have similar structure but are specific to their types
func (s *ExportService) streamLocations(ctx context.Context, writer io.Writer) error {
	locations, err := s.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to get locations")
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Start locations element
	startElement := xml.StartElement{Name: xml.Name{Local: "locations"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errkit.Wrap(err, "failed to encode locations start element")
	}

	for _, loc := range locations {
		xmlLoc := &Location{
			ID:      loc.ID,
			Name:    loc.Name,
			Address: loc.Address,
		}
		if err := encoder.Encode(xmlLoc); err != nil {
			return errkit.Wrap(err, "failed to encode location")
		}
	}

	// End locations element
	endElement := xml.EndElement{Name: xml.Name{Local: "locations"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errkit.Wrap(err, "failed to encode locations end element")
	}

	if err := encoder.Flush(); err != nil {
		return errkit.Wrap(err, "failed to flush encoder")
	}

	return nil
}

// streamAreas streams areas to the writer
//
//nolint:dupl // streamLocations and streamAreas have similar structure but are specific to their types
func (s *ExportService) streamAreas(ctx context.Context, writer io.Writer) error {
	areas, err := s.registrySet.AreaRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to get areas")
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Start areas element
	startElement := xml.StartElement{Name: xml.Name{Local: "areas"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errkit.Wrap(err, "failed to encode areas start element")
	}

	for _, area := range areas {
		xmlArea := &Area{
			ID:         area.ID,
			Name:       area.Name,
			LocationID: area.LocationID,
		}
		if err := encoder.Encode(xmlArea); err != nil {
			return errkit.Wrap(err, "failed to encode area")
		}
	}

	// End areas element
	endElement := xml.EndElement{Name: xml.Name{Local: "areas"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errkit.Wrap(err, "failed to encode areas end element")
	}

	if err := encoder.Flush(); err != nil {
		return errkit.Wrap(err, "failed to flush encoder")
	}

	return nil
}

// streamCommodities streams commodities to the writer
func (s *ExportService) streamCommodities(ctx context.Context, writer io.Writer, args ExportArgs) error {
	commodities, err := s.registrySet.CommodityRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to get commodities")
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Start commodities element
	startElement := xml.StartElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errkit.Wrap(err, "failed to encode commodities start element")
	}

	for _, commodity := range commodities {
		// Use streaming approach for commodities with file data
		if args.IncludeFileData {
			if err := s.streamCommodityDirectly(ctx, encoder, commodity, args); err != nil {
				return errkit.Wrap(err, "failed to stream commodity")
			}
		} else {
			// Use traditional approach for commodities without file data
			xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, args)
			if err != nil {
				return errkit.Wrap(err, "failed to convert commodity to XML")
			}
			if err := encoder.Encode(xmlCommodity); err != nil {
				return errkit.Wrap(err, "failed to encode commodity")
			}
		}
	}

	// End commodities element
	endElement := xml.EndElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errkit.Wrap(err, "failed to encode commodities end element")
	}

	if err := encoder.Flush(); err != nil {
		return errkit.Wrap(err, "failed to flush encoder")
	}

	return nil
}

// streamSelectedItems streams selected items (locations, areas, commodities) to the writer
func (s *ExportService) streamSelectedItems(ctx context.Context, writer io.Writer, selectedItems []models.ExportSelectedItem, args ExportArgs) error {
	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Group items by type for better organization
	locations, areas, commodities := s.groupSelectedItemsByType(selectedItems)

	// Export each type of item
	if err := s.streamSelectedLocations(ctx, encoder, locations); err != nil {
		return err
	}
	if err := s.streamSelectedAreas(ctx, encoder, areas); err != nil {
		return err
	}
	if err := s.streamSelectedCommodities(ctx, encoder, commodities, args); err != nil {
		return err
	}

	if err := encoder.Flush(); err != nil {
		return errkit.Wrap(err, "failed to flush encoder")
	}

	return nil
}

// groupSelectedItemsByType groups selected items by their type
func (s *ExportService) groupSelectedItemsByType(selectedItems []models.ExportSelectedItem) (locations, areas, commodities []string) {
	for _, item := range selectedItems {
		switch item.Type {
		case models.ExportSelectedItemTypeLocation:
			locations = append(locations, item.ID)
		case models.ExportSelectedItemTypeArea:
			areas = append(areas, item.ID)
		case models.ExportSelectedItemTypeCommodity:
			commodities = append(commodities, item.ID)
		}
	}

	return locations, areas, commodities
}

// streamSelectedLocations streams location data to the XML encoder
func (s *ExportService) streamSelectedLocations(ctx context.Context, encoder *xml.Encoder, locationIDs []string) error {
	return s.streamEntitySection(ctx, encoder, "locations", locationIDs, func(ctx context.Context, id string) (any, error) {
		location, err := s.registrySet.LocationRegistry.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return &Location{
			ID:      location.ID,
			Name:    location.Name,
			Address: location.Address,
		}, nil
	})
}

// streamSelectedAreas streams area data to the XML encoder
func (s *ExportService) streamSelectedAreas(ctx context.Context, encoder *xml.Encoder, areaIDs []string) error {
	return s.streamEntitySection(ctx, encoder, "areas", areaIDs, func(ctx context.Context, id string) (any, error) {
		area, err := s.registrySet.AreaRegistry.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		return &Area{
			ID:         area.ID,
			Name:       area.Name,
			LocationID: area.LocationID,
		}, nil
	})
}

// streamEntitySection streams a section of entities to the XML encoder
func (s *ExportService) streamEntitySection(ctx context.Context, encoder *xml.Encoder, sectionName string, entityIDs []string, entityLoader func(context.Context, string) (any, error)) error {
	if len(entityIDs) == 0 {
		return nil
	}

	startElement := xml.StartElement{Name: xml.Name{Local: sectionName}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errkit.Wrap(err, "failed to encode "+sectionName+" start element")
	}

	for _, entityID := range entityIDs {
		entity, err := entityLoader(ctx, entityID)
		if err != nil {
			continue // Skip items that can't be found
		}

		if err := encoder.Encode(entity); err != nil {
			return errkit.Wrap(err, "failed to encode "+sectionName+" entity")
		}
	}

	endElement := xml.EndElement{Name: xml.Name{Local: sectionName}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errkit.Wrap(err, "failed to encode "+sectionName+" end element")
	}

	return nil
}

// streamSelectedCommodities streams commodity data to the XML encoder
func (s *ExportService) streamSelectedCommodities(ctx context.Context, encoder *xml.Encoder, commodityIDs []string, args ExportArgs) error {
	if len(commodityIDs) == 0 {
		return nil
	}

	startElement := xml.StartElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errkit.Wrap(err, "failed to encode commodities start element")
	}

	for _, commodityID := range commodityIDs {
		commodity, err := s.registrySet.CommodityRegistry.Get(ctx, commodityID)
		if err != nil {
			continue // Skip items that can't be found
		}

		// Use streaming approach for commodities with file data
		if args.IncludeFileData {
			if err := s.streamCommodityDirectly(ctx, encoder, commodity, args); err != nil {
				return errkit.Wrap(err, "failed to stream commodity")
			}
		} else {
			// Use traditional approach for commodities without file data
			xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, args)
			if err != nil {
				return errkit.Wrap(err, "failed to convert commodity to XML")
			}
			if err := encoder.Encode(xmlCommodity); err != nil {
				return errkit.Wrap(err, "failed to encode commodity")
			}
		}
	}

	endElement := xml.EndElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errkit.Wrap(err, "failed to encode commodities end element")
	}

	return nil
}

func (s *ExportService) convertCommodityToXML(ctx context.Context, commodity *models.Commodity, args ExportArgs) (*Commodity, error) {
	xmlCommodity := &Commodity{
		ID:                     commodity.ID,
		Name:                   commodity.Name,
		ShortName:              commodity.ShortName,
		Type:                   string(commodity.Type),
		AreaID:                 commodity.AreaID,
		Count:                  commodity.Count,
		OriginalPrice:          commodity.OriginalPrice.String(),
		OriginalPriceCurrency:  string(commodity.OriginalPriceCurrency),
		ConvertedOriginalPrice: commodity.ConvertedOriginalPrice.String(),
		CurrentPrice:           commodity.CurrentPrice.String(),
		SerialNumber:           commodity.SerialNumber,
		Status:                 string(commodity.Status),
		Comments:               commodity.Comments,
		Draft:                  commodity.Draft,
	}

	// Convert slices
	if commodity.ExtraSerialNumbers != nil {
		xmlCommodity.ExtraSerialNumbers = commodity.ExtraSerialNumbers
	}
	if commodity.PartNumbers != nil {
		xmlCommodity.PartNumbers = commodity.PartNumbers
	}
	if commodity.Tags != nil {
		xmlCommodity.Tags = commodity.Tags
	}

	// Convert dates
	if commodity.PurchaseDate != nil {
		xmlCommodity.PurchaseDate = string(*commodity.PurchaseDate)
	}
	if commodity.RegisteredDate != nil {
		xmlCommodity.RegisteredDate = string(*commodity.RegisteredDate)
	}
	if commodity.LastModifiedDate != nil {
		xmlCommodity.LastModifiedDate = string(*commodity.LastModifiedDate)
	}

	// Convert URLs
	for _, u := range commodity.URLs {
		if u != nil {
			xmlCommodity.URLs = append(xmlCommodity.URLs, &URL{
				Name:  "", // URL model doesn't have a Name field
				Value: u.String(),
			})
		}
	}

	// Handle file attachments (images, invoices, manuals)
	if err := s.addFileAttachments(ctx, commodity.ID, xmlCommodity, args); err != nil {
		return nil, errkit.Wrap(err, "failed to add file attachments")
	}

	return xmlCommodity, nil
}

// addFileAttachments adds file attachments (images, invoices, manuals) to the XML commodity
func (s *ExportService) addFileAttachments(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs) error {
	// Add images
	if err := s.addImages(ctx, commodityID, xmlCommodity, args); err != nil {
		return errkit.Wrap(err, "failed to add images")
	}

	// Add invoices
	if err := s.addInvoices(ctx, commodityID, xmlCommodity, args); err != nil {
		return errkit.Wrap(err, "failed to add invoices")
	}

	// Add manuals
	if err := s.addManuals(ctx, commodityID, xmlCommodity, args); err != nil {
		return errkit.Wrap(err, "failed to add manuals")
	}

	return nil
}

// addImages adds images to the XML commodity
func (s *ExportService) addImages(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs) error {
	imageIDs, err := s.registrySet.CommodityRegistry.GetImages(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get images")
	}

	for _, imageID := range imageIDs {
		image, err := s.registrySet.ImageRegistry.Get(ctx, imageID)
		if err != nil {
			continue // Skip images that can't be found
		}

		xmlFile := &File{
			ID:           image.ID,
			Path:         image.Path,
			OriginalPath: image.OriginalPath,
			Extension:    image.Ext,
			MimeType:     image.MIMEType,
		}

		// Note: File data loading is skipped here when using streaming approach
		// The streamFileAttachmentsDirectly method handles file data streaming
		// This traditional approach is only used for non-streaming exports
		if args.IncludeFileData {
			if err := s.loadFileDataStreaming(ctx, xmlFile); err != nil {
				// Don't fail the entire export if one file can't be read
				continue
			}
		}

		xmlCommodity.Images = append(xmlCommodity.Images, xmlFile)
	}

	return nil
}

// addInvoices adds invoices to the XML commodity
func (s *ExportService) addInvoices(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs) error {
	invoiceIDs, err := s.registrySet.CommodityRegistry.GetInvoices(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoices")
	}

	for _, invoiceID := range invoiceIDs {
		invoice, err := s.registrySet.InvoiceRegistry.Get(ctx, invoiceID)
		if err != nil {
			continue // Skip invoices that can't be found
		}

		xmlFile := &File{
			ID:           invoice.ID,
			Path:         invoice.Path,
			OriginalPath: invoice.OriginalPath,
			Extension:    invoice.Ext,
			MimeType:     invoice.MIMEType,
		}

		// Note: File data loading is skipped here when using streaming approach
		// This traditional approach is only used for non-streaming exports
		if args.IncludeFileData {
			if err := s.loadFileDataStreaming(ctx, xmlFile); err != nil {
				// Don't fail the entire export if one file can't be read
				continue
			}
		}

		xmlCommodity.Invoices = append(xmlCommodity.Invoices, xmlFile)
	}

	return nil
}

// addManuals adds manuals to the XML commodity
func (s *ExportService) addManuals(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs) error {
	manualIDs, err := s.registrySet.CommodityRegistry.GetManuals(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get manuals")
	}

	for _, manualID := range manualIDs {
		manual, err := s.registrySet.ManualRegistry.Get(ctx, manualID)
		if err != nil {
			continue // Skip manuals that can't be found
		}

		xmlFile := &File{
			ID:           manual.ID,
			Path:         manual.Path,
			OriginalPath: manual.OriginalPath,
			Extension:    manual.Ext,
			MimeType:     manual.MIMEType,
		}

		// Note: File data loading is skipped here when using streaming approach
		// This traditional approach is only used for non-streaming exports
		if args.IncludeFileData {
			if err := s.loadFileDataStreaming(ctx, xmlFile); err != nil {
				// Don't fail the entire export if one file can't be read
				continue
			}
		}

		xmlCommodity.Manuals = append(xmlCommodity.Manuals, xmlFile)
	}

	return nil
}

// loadFileDataStreaming loads file data using a memory-efficient streaming approach
// This is only used for small files or when backward compatibility is needed
func (s *ExportService) loadFileDataStreaming(ctx context.Context, xmlFile *File) error {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open blob bucket")
	}
	defer b.Close()

	// Open file reader
	reader, err := b.NewReader(ctx, xmlFile.OriginalPath, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to create file reader")
	}
	defer reader.Close()

	// For backward compatibility, still load small files into memory
	// Large files should use streamFileDataDirectly instead
	fileData, err := io.ReadAll(reader)
	if err != nil {
		return errkit.Wrap(err, "failed to read file data")
	}

	xmlFile.Data = base64.StdEncoding.EncodeToString(fileData)
	return nil
}

// streamFileDataDirectly streams file data directly to XML encoder without loading into memory.
// This is the core of the memory-efficient streaming solution that reads files in chunks
// and encodes them to base64 on-the-fly, writing directly to the XML output stream.
// Unlike the traditional approach, this method never stores the complete file content in memory.
func (s *ExportService) streamFileDataDirectly(ctx context.Context, encoder *xml.Encoder, xmlFile *File) error {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open blob bucket")
	}
	defer b.Close()

	// Open file reader
	reader, err := b.NewReader(ctx, xmlFile.OriginalPath, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to create file reader")
	}
	defer reader.Close()

	// Start the file element without the data attribute
	fileStart := xml.StartElement{
		Name: xml.Name{Local: "file"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: xmlFile.ID},
		},
	}
	if err := encoder.EncodeToken(fileStart); err != nil {
		return errkit.Wrap(err, "failed to encode file start element")
	}

	// Encode file metadata elements
	if err := s.encodeFileMetadata(encoder, xmlFile); err != nil {
		return errkit.Wrap(err, "failed to encode file metadata")
	}

	// Start data element
	dataStart := xml.StartElement{Name: xml.Name{Local: "data"}}
	if err := encoder.EncodeToken(dataStart); err != nil {
		return errkit.Wrap(err, "failed to encode data start element")
	}

	// Stream file content as base64 encoded chunks
	if err := s.streamBase64Content(encoder, reader); err != nil {
		return errkit.Wrap(err, "failed to stream file content")
	}

	// End data element
	dataEnd := xml.EndElement{Name: xml.Name{Local: "data"}}
	if err := encoder.EncodeToken(dataEnd); err != nil {
		return errkit.Wrap(err, "failed to encode data end element")
	}

	// End file element
	fileEnd := xml.EndElement{Name: xml.Name{Local: "file"}}
	if err := encoder.EncodeToken(fileEnd); err != nil {
		return errkit.Wrap(err, "failed to encode file end element")
	}

	return nil
}

// encodeFileMetadata encodes file metadata elements (path, originalPath, extension, mimeType)
func (s *ExportService) encodeFileMetadata(encoder *xml.Encoder, xmlFile *File) error {
	// Encode path
	pathElement := xml.StartElement{Name: xml.Name{Local: "path"}}
	if err := encoder.EncodeToken(pathElement); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.CharData(xmlFile.Path)); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "path"}}); err != nil {
		return err
	}

	// Encode originalPath
	originalPathElement := xml.StartElement{Name: xml.Name{Local: "originalPath"}}
	if err := encoder.EncodeToken(originalPathElement); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.CharData(xmlFile.OriginalPath)); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "originalPath"}}); err != nil {
		return err
	}

	// Encode extension
	extensionElement := xml.StartElement{Name: xml.Name{Local: "extension"}}
	if err := encoder.EncodeToken(extensionElement); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.CharData(xmlFile.Extension)); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "extension"}}); err != nil {
		return err
	}

	// Encode mimeType
	mimeTypeElement := xml.StartElement{Name: xml.Name{Local: "mimeType"}}
	if err := encoder.EncodeToken(mimeTypeElement); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.CharData(xmlFile.MimeType)); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "mimeType"}}); err != nil {
		return err
	}

	return nil
}

// streamBase64Content streams file content as base64 encoded data in chunks directly to XML
func (s *ExportService) streamBase64Content(encoder *xml.Encoder, reader *blob.Reader) error {
	// Create a custom writer that writes base64 chunks directly to XML encoder
	xmlWriter := &xmlBase64Writer{encoder: encoder}

	// Create base64 encoder that writes directly to XML
	base64Encoder := base64.NewEncoder(base64.StdEncoding, xmlWriter)

	// Copy data from reader to base64 encoder in chunks
	const chunkSize = 32768 // 32KB chunks for efficient memory usage
	buf := make([]byte, chunkSize)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			if _, writeErr := base64Encoder.Write(buf[:n]); writeErr != nil {
				err = errors.Join(writeErr, base64Encoder.Close())
				return errkit.Wrap(writeErr, "failed to write chunk to base64 encoder")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			err = errors.Join(err, base64Encoder.Close())
			return errkit.Wrap(err, "failed to read chunk")
		}
	}

	// Close the base64 encoder to flush any remaining data
	if err := base64Encoder.Close(); err != nil {
		return errkit.Wrap(err, "failed to close base64 encoder")
	}

	return nil
}

// xmlBase64Writer is a custom writer that writes base64 data directly to XML encoder.
// This is the key component that enables true streaming without memory accumulation.
// Each Write() call immediately writes the data to the XML encoder as CharData,
// ensuring that no data is buffered or accumulated in memory.
type xmlBase64Writer struct {
	encoder *xml.Encoder
}

// Write implements io.Writer interface to write base64 chunks directly to XML.
// This method ensures that base64-encoded data is written immediately to the XML stream
// without any intermediate buffering, making it truly memory-efficient for large files.
func (w *xmlBase64Writer) Write(p []byte) (n int, err error) {
	if err := w.encoder.EncodeToken(xml.CharData(p)); err != nil {
		return 0, err
	}
	return len(p), nil
}

// streamFileAttachmentsDirectly streams file attachments directly to XML encoder for large files
func (s *ExportService) streamFileAttachmentsDirectly(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, args ExportArgs) error {
	if !args.IncludeFileData {
		return nil // No file data to stream
	}

	// Stream images
	if err := s.streamImagesDirectly(ctx, encoder, commodity.ID); err != nil {
		return errkit.Wrap(err, "failed to stream images")
	}

	// Stream invoices
	if err := s.streamInvoicesDirectly(ctx, encoder, commodity.ID); err != nil {
		return errkit.Wrap(err, "failed to stream invoices")
	}

	// Stream manuals
	if err := s.streamManualsDirectly(ctx, encoder, commodity.ID); err != nil {
		return errkit.Wrap(err, "failed to stream manuals")
	}

	return nil
}

// streamImagesDirectly streams images directly to XML encoder
func (s *ExportService) streamImagesDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string) error {
	imageIDs, err := s.registrySet.CommodityRegistry.GetImages(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get images")
	}

	if len(imageIDs) == 0 {
		return nil
	}

	// Start images element
	imagesStart := xml.StartElement{Name: xml.Name{Local: "images"}}
	if err := encoder.EncodeToken(imagesStart); err != nil {
		return errkit.Wrap(err, "failed to encode images start element")
	}

	for _, imageID := range imageIDs {
		image, err := s.registrySet.ImageRegistry.Get(ctx, imageID)
		if err != nil {
			continue // Skip images that can't be found
		}

		xmlFile := &File{
			ID:           image.ID,
			Path:         image.Path,
			OriginalPath: image.OriginalPath,
			Extension:    image.Ext,
			MimeType:     image.MIMEType,
		}

		// Stream file data directly
		if err := s.streamFileDataDirectly(ctx, encoder, xmlFile); err != nil {
			// Don't fail the entire export if one file can't be read
			continue
		}
	}

	// End images element
	imagesEnd := xml.EndElement{Name: xml.Name{Local: "images"}}
	if err := encoder.EncodeToken(imagesEnd); err != nil {
		return errkit.Wrap(err, "failed to encode images end element")
	}

	return nil
}

// streamInvoicesDirectly streams invoices directly to XML encoder
func (s *ExportService) streamInvoicesDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string) error {
	invoiceIDs, err := s.registrySet.CommodityRegistry.GetInvoices(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoices")
	}

	if len(invoiceIDs) == 0 {
		return nil
	}

	// Start invoices element
	invoicesStart := xml.StartElement{Name: xml.Name{Local: "invoices"}}
	if err := encoder.EncodeToken(invoicesStart); err != nil {
		return errkit.Wrap(err, "failed to encode invoices start element")
	}

	for _, invoiceID := range invoiceIDs {
		invoice, err := s.registrySet.InvoiceRegistry.Get(ctx, invoiceID)
		if err != nil {
			continue // Skip invoices that can't be found
		}

		xmlFile := &File{
			ID:           invoice.ID,
			Path:         invoice.Path,
			OriginalPath: invoice.OriginalPath,
			Extension:    invoice.Ext,
			MimeType:     invoice.MIMEType,
		}

		// Stream file data directly
		if err := s.streamFileDataDirectly(ctx, encoder, xmlFile); err != nil {
			// Don't fail the entire export if one file can't be read
			continue
		}
	}

	// End invoices element
	invoicesEnd := xml.EndElement{Name: xml.Name{Local: "invoices"}}
	if err := encoder.EncodeToken(invoicesEnd); err != nil {
		return errkit.Wrap(err, "failed to encode invoices end element")
	}

	return nil
}

// streamManualsDirectly streams manuals directly to XML encoder
func (s *ExportService) streamManualsDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string) error {
	manualIDs, err := s.registrySet.CommodityRegistry.GetManuals(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get manuals")
	}

	if len(manualIDs) == 0 {
		return nil
	}

	// Start manuals element
	manualsStart := xml.StartElement{Name: xml.Name{Local: "manuals"}}
	if err := encoder.EncodeToken(manualsStart); err != nil {
		return errkit.Wrap(err, "failed to encode manuals start element")
	}

	for _, manualID := range manualIDs {
		manual, err := s.registrySet.ManualRegistry.Get(ctx, manualID)
		if err != nil {
			continue // Skip manuals that can't be found
		}

		xmlFile := &File{
			ID:           manual.ID,
			Path:         manual.Path,
			OriginalPath: manual.OriginalPath,
			Extension:    manual.Ext,
			MimeType:     manual.MIMEType,
		}

		// Stream file data directly
		if err := s.streamFileDataDirectly(ctx, encoder, xmlFile); err != nil {
			// Don't fail the entire export if one file can't be read
			continue
		}
	}

	// End manuals element
	manualsEnd := xml.EndElement{Name: xml.Name{Local: "manuals"}}
	if err := encoder.EncodeToken(manualsEnd); err != nil {
		return errkit.Wrap(err, "failed to encode manuals end element")
	}

	return nil
}

// streamCommodityDirectly streams a commodity with file attachments directly to XML encoder
func (s *ExportService) streamCommodityDirectly(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, args ExportArgs) error {
	// Start commodity element
	commodityStart := xml.StartElement{
		Name: xml.Name{Local: "commodity"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: commodity.ID},
		},
	}
	if err := encoder.EncodeToken(commodityStart); err != nil {
		return errkit.Wrap(err, "failed to encode commodity start element")
	}

	// Encode commodity metadata
	if err := s.encodeCommodityMetadata(ctx, encoder, commodity); err != nil {
		return errkit.Wrap(err, "failed to encode commodity metadata")
	}

	// Stream file attachments if requested - this bypasses the traditional File.Data approach
	if args.IncludeFileData {
		if err := s.streamFileAttachmentsDirectly(ctx, encoder, commodity, args); err != nil {
			return errkit.Wrap(err, "failed to stream file attachments")
		}
	}

	// End commodity element
	commodityEnd := xml.EndElement{Name: xml.Name{Local: "commodity"}}
	if err := encoder.EncodeToken(commodityEnd); err != nil {
		return errkit.Wrap(err, "failed to encode commodity end element")
	}

	return nil
}

// encodeCommodityMetadata encodes commodity metadata elements
func (s *ExportService) encodeCommodityMetadata(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity) error {
	// Helper function to encode simple text elements
	encodeTextElement := func(name, value string) error {
		if value == "" {
			return nil // Skip empty values
		}
		start := xml.StartElement{Name: xml.Name{Local: name}}
		if err := encoder.EncodeToken(start); err != nil {
			return err
		}
		if err := encoder.EncodeToken(xml.CharData(value)); err != nil {
			return err
		}
		return encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: name}})
	}

	// Encode basic fields
	if err := encodeTextElement("commodityName", commodity.Name); err != nil {
		return err
	}
	if err := encodeTextElement("shortName", commodity.ShortName); err != nil {
		return err
	}
	if err := encodeTextElement("type", string(commodity.Type)); err != nil {
		return err
	}
	if err := encodeTextElement("areaId", commodity.AreaID); err != nil {
		return err
	}

	// Encode count
	countStart := xml.StartElement{Name: xml.Name{Local: "count"}}
	if err := encoder.EncodeToken(countStart); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.CharData(fmt.Sprintf("%d", commodity.Count))); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "count"}}); err != nil {
		return err
	}

	// Encode price fields
	if err := encodeTextElement("originalPrice", commodity.OriginalPrice.String()); err != nil {
		return err
	}
	if err := encodeTextElement("originalPriceCurrency", string(commodity.OriginalPriceCurrency)); err != nil {
		return err
	}
	if err := encodeTextElement("convertedOriginalPrice", commodity.ConvertedOriginalPrice.String()); err != nil {
		return err
	}
	if err := encodeTextElement("currentPrice", commodity.CurrentPrice.String()); err != nil {
		return err
	}

	// Encode other fields
	if err := encodeTextElement("serialNumber", commodity.SerialNumber); err != nil {
		return err
	}
	if err := encodeTextElement("status", string(commodity.Status)); err != nil {
		return err
	}
	if err := encodeTextElement("comments", commodity.Comments); err != nil {
		return err
	}

	// Encode draft
	draftStart := xml.StartElement{Name: xml.Name{Local: "draft"}}
	if err := encoder.EncodeToken(draftStart); err != nil {
		return err
	}
	draftValue := "false"
	if commodity.Draft {
		draftValue = "true"
	}
	if err := encoder.EncodeToken(xml.CharData(draftValue)); err != nil {
		return err
	}
	if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "draft"}}); err != nil {
		return err
	}

	// Encode dates
	if commodity.PurchaseDate != nil {
		if err := encodeTextElement("purchaseDate", string(*commodity.PurchaseDate)); err != nil {
			return err
		}
	}
	if commodity.RegisteredDate != nil {
		if err := encodeTextElement("registeredDate", string(*commodity.RegisteredDate)); err != nil {
			return err
		}
	}
	if commodity.LastModifiedDate != nil {
		if err := encodeTextElement("lastModifiedDate", string(*commodity.LastModifiedDate)); err != nil {
			return err
		}
	}

	// Encode arrays
	if err := s.encodeStringArray(encoder, "extraSerialNumbers", "serialNumber", commodity.ExtraSerialNumbers); err != nil {
		return err
	}
	if err := s.encodeStringArray(encoder, "partNumbers", "partNumber", commodity.PartNumbers); err != nil {
		return err
	}
	if err := s.encodeStringArray(encoder, "tags", "tag", commodity.Tags); err != nil {
		return err
	}

	// Encode URLs
	if err := s.encodeURLs(encoder, commodity.URLs); err != nil {
		return err
	}

	return nil
}

// encodeStringArray encodes a string array as XML elements
func (s *ExportService) encodeStringArray(encoder *xml.Encoder, containerName, elementName string, values []string) error {
	if len(values) == 0 {
		return nil
	}

	// Start container element
	containerStart := xml.StartElement{Name: xml.Name{Local: containerName}}
	if err := encoder.EncodeToken(containerStart); err != nil {
		return err
	}

	// Encode each value
	for _, value := range values {
		elementStart := xml.StartElement{Name: xml.Name{Local: elementName}}
		if err := encoder.EncodeToken(elementStart); err != nil {
			return err
		}
		if err := encoder.EncodeToken(xml.CharData(value)); err != nil {
			return err
		}
		if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: elementName}}); err != nil {
			return err
		}
	}

	// End container element
	containerEnd := xml.EndElement{Name: xml.Name{Local: containerName}}
	return encoder.EncodeToken(containerEnd)
}

// encodeURLs encodes URLs as XML elements
func (s *ExportService) encodeURLs(encoder *xml.Encoder, urls []*models.URL) error {
	if len(urls) == 0 {
		return nil
	}

	// Start urls element
	urlsStart := xml.StartElement{Name: xml.Name{Local: "urls"}}
	if err := encoder.EncodeToken(urlsStart); err != nil {
		return err
	}

	// Encode each URL
	for _, u := range urls {
		if u != nil {
			urlStart := xml.StartElement{
				Name: xml.Name{Local: "url"},
				Attr: []xml.Attr{
					{Name: xml.Name{Local: "name"}, Value: ""}, // URL model doesn't have a Name field
				},
			}
			if err := encoder.EncodeToken(urlStart); err != nil {
				return err
			}
			if err := encoder.EncodeToken(xml.CharData(u.String())); err != nil {
				return err
			}
			if err := encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "url"}}); err != nil {
				return err
			}
		}
	}

	// End urls element
	urlsEnd := xml.EndElement{Name: xml.Name{Local: "urls"}}
	return encoder.EncodeToken(urlsEnd)
}
