package export

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ExportArgs contains arguments for export operations
type ExportArgs struct {
	IncludeFileData bool
}

// ExportStats tracks statistics during export generation
type ExportStats struct {
	LocationCount  int
	AreaCount      int
	CommodityCount int
	ImageCount     int
	InvoiceCount   int
	ManualCount    int
	BinaryDataSize int64
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

// ExportService handles the background processing of export requests
type ExportService struct {
	registrySet    *registry.Set
	uploadLocation string
}

// NewExportService creates a new export service
func NewExportService(registrySet *registry.Set, uploadLocation string) *ExportService {
	return &ExportService{
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
	}
}

// CreateExportFromUserInput creates a new export record from user input.
// The export record is created with status "pending" and is ready for processing.
// It will be processed by the ExportWorker in the background.
func (s *ExportService) CreateExportFromUserInput(ctx context.Context, input *models.Export) (models.Export, error) {
	// Validate the export
	if err := input.ValidateWithContext(ctx); err != nil {
		return models.Export{}, errkit.Wrap(err, "failed to validate export")
	}

	export := models.NewExportFromUserInput(input)

	// Enrich selected items with names from the database
	if export.Type == models.ExportTypeSelectedItems && len(export.SelectedItems) > 0 {
		if err := s.enrichSelectedItemsWithNames(ctx, &export); err != nil {
			return models.Export{}, errkit.Wrap(err, "failed to enrich selected items with names")
		}
	}

	// Create the export
	created, err := s.registrySet.ExportRegistry.Create(ctx, export)
	if err != nil {
		return models.Export{}, errkit.Wrap(err, "failed to create export")
	}

	return *created, nil
}

// ProcessExport processes an export request in the background
func (s *ExportService) ProcessExport(ctx context.Context, exportID string) error {
	// Get the export request
	export, err := s.registrySet.ExportRegistry.Get(ctx, exportID)
	if err != nil {
		return errkit.Wrap(err, "failed to get export")
	}

	// Skip processing for imported exports - they are already completed
	if export.Type == models.ExportTypeImported {
		return nil
	}

	// Update status to in_progress
	export.Status = models.ExportStatusInProgress
	_, err = s.registrySet.ExportRegistry.Update(ctx, *export)
	if err != nil {
		return errkit.Wrap(err, "failed to update export status")
	}

	// Generate the export and collect statistics
	filePath, stats, err := s.generateExport(ctx, *export)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		s.registrySet.ExportRegistry.Update(ctx, *export)
		return errkit.Wrap(err, "failed to generate export")
	}

	// Create file entity for the export
	fileEntity, err := s.createExportFileEntity(ctx, export.ID, export.Description, filePath)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		s.registrySet.ExportRegistry.Update(ctx, *export)
		return errkit.Wrap(err, "failed to create export file entity")
	}

	// Store statistics in export record
	export.LocationCount = stats.LocationCount
	export.AreaCount = stats.AreaCount
	export.CommodityCount = stats.CommodityCount
	export.ImageCount = stats.ImageCount
	export.InvoiceCount = stats.InvoiceCount
	export.ManualCount = stats.ManualCount
	export.BinaryDataSize = stats.BinaryDataSize

	// Get file size
	if fileSize, err := s.getFileSize(ctx, filePath); err == nil {
		export.FileSize = fileSize
	}

	// Update status to completed
	export.Status = models.ExportStatusCompleted
	export.FileID = &fileEntity.ID
	export.FilePath = filePath // Keep for backward compatibility during migration
	export.CompletedDate = models.PNow()
	export.ErrorMessage = ""

	_, err = s.registrySet.ExportRegistry.Update(ctx, *export)
	if err != nil {
		return errkit.Wrap(err, "failed to update export completion")
	}

	return nil
}

// createExportFileEntity creates a file entity for an export file
func (s *ExportService) createExportFileEntity(ctx context.Context, exportID, description, filePath string) (*models.FileEntity, error) {
	// Extract filename from path for title
	filename := filepath.Base(filePath)
	if ext := filepath.Ext(filename); ext != "" {
		filename = strings.TrimSuffix(filename, ext)
	}

	// Create file entity
	now := time.Now()
	fileEntity := models.FileEntity{
		Title:            fmt.Sprintf("Export: %s", description),
		Description:      fmt.Sprintf("Export file generated on %s", now.Format("2006-01-02 15:04:05")),
		Type:             models.FileTypeDocument, // XML files are documents
		Tags:             []string{"export", "xml"},
		LinkedEntityType: "export",
		LinkedEntityID:   exportID,
		LinkedEntityMeta: "xml-1.0", // Mark as export file with version
		CreatedAt:        now,
		UpdatedAt:        now,
		File: &models.File{
			Path:         filename,
			OriginalPath: filePath,
			Ext:          ".xml",
			MIMEType:     "application/xml",
		},
	}

	// Create the file entity
	created, err := s.registrySet.FileRegistry.Create(ctx, fileEntity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create file entity")
	}

	return created, nil
}

// DeleteExportFile is deprecated - export files are now managed through the file entity system
// This method is kept for backward compatibility but should not be used for new exports
func (s *ExportService) DeleteExportFile(ctx context.Context, filePath string) error {
	if filePath == "" {
		return nil // Nothing to delete
	}

	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open blob bucket")
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			err = errkit.Wrap(closeErr, "failed to close blob bucket")
		}
	}()

	// Delete the file
	err = b.Delete(ctx, filePath)
	if err != nil {
		return errkit.Wrap(err, "failed to delete export file")
	}

	return nil
}

func (s *ExportService) parseTopLevelToken(t xml.StartElement, exportType *models.ExportType, decoder *xml.Decoder, stats *ExportStats) error {
	switch t.Name.Local {
	case "inventory":
		// Check export type attribute if present
		for _, attr := range t.Attr {
			if attr.Name.Local != "exportType" {
				continue
			}
			switch attr.Value {
			case "full_database":
				*exportType = models.ExportTypeFullDatabase
			case "selected_items":
				*exportType = models.ExportTypeSelectedItems
			case "locations":
				*exportType = models.ExportTypeLocations
			case "areas":
				*exportType = models.ExportTypeAreas
			case "commodities":
				*exportType = models.ExportTypeCommodities
			}
		}
	case "locations":
		if err := s.countLocations(decoder, stats); err != nil {
			return errkit.Wrap(err, "failed to count locations")
		}
	case "areas":
		if err := s.countAreas(decoder, stats); err != nil {
			return errkit.Wrap(err, "failed to count areas")
		}
	case "commodities":
		if err := s.countCommodities(decoder, stats); err != nil {
			return errkit.Wrap(err, "failed to count commodities")
		}
	default:
		return errkit.Wrap(ErrUnsupportedExportType, "unsupported top-level element", "element", t.Name.Local)
	}

	return nil
}

// ParseXMLMetadata parses XML file to extract statistics and determine export type
func (s *ExportService) ParseXMLMetadata(_ctx context.Context, reader io.Reader) (*ExportStats, models.ExportType, error) {
	stats := &ExportStats{}
	exportType := models.ExportTypeImported // Default to imported type

	decoder := xml.NewDecoder(reader)

	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, exportType, errkit.Wrap(err, "failed to read XML token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			err = s.parseTopLevelToken(t, &exportType, decoder, stats)
			if err != nil {
				return stats, exportType, err
			}
		default:
			// Skip other token types
			continue
		}
	}

	// The following was commented out because detecting the export type from the content is not
	// reliable. It's better to just set it to "imported" and let the user decide.
	//
	//// If export type wasn't specified in XML, try to infer it from content
	//
	// if exportType == models.ExportTypeImported {
	//	if hasLocations && hasAreas && hasCommodities {
	//		exportType = models.ExportTypeFullDatabase
	//	} else if hasLocations && !hasAreas && !hasCommodities {
	//		exportType = models.ExportTypeLocations
	//	} else if hasAreas && !hasLocations && !hasCommodities {
	//		exportType = models.ExportTypeAreas
	//	} else if hasCommodities && !hasLocations && !hasAreas {
	//		exportType = models.ExportTypeCommodities
	//	} else {
	//		exportType = models.ExportTypeSelectedItems
	//	}
	// }

	return stats, exportType, nil
}

// generateExport generates the XML export file using blob storage and returns statistics
func (s *ExportService) generateExport(ctx context.Context, export models.Export) (string, *ExportStats, error) {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return "", nil, errkit.Wrap(err, "failed to open blob bucket")
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
		return "", nil, errkit.Wrap(err, "failed to create blob writer")
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			err = errkit.Wrap(closeErr, "failed to close blob writer")
		}
	}()

	// Stream XML generation with statistics tracking
	stats, err := s.streamXMLExport(ctx, export, writer)
	if err != nil {
		return "", nil, errkit.Wrap(err, "failed to generate XML export")
	}

	return blobKey, stats, nil
}

// getFileSize gets the size of a file in blob storage
func (s *ExportService) getFileSize(ctx context.Context, filePath string) (int64, error) {
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to open bucket")
	}
	defer b.Close()

	attrs, err := b.Attributes(ctx, filePath)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to get file attributes")
	}

	return attrs.Size, nil
}

// streamXMLExport streams XML data directly to the file writer and tracks statistics
func (s *ExportService) streamXMLExport(ctx context.Context, export models.Export, writer io.Writer) (*ExportStats, error) {
	stats := &ExportStats{}

	// Write XML header
	if _, err := writer.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")); err != nil {
		return nil, errkit.Wrap(err, "failed to write XML header")
	}

	// Start root element
	exportDate := time.Now().Format("2006-01-02T15:04:05Z")
	rootStart := fmt.Sprintf(`<inventory exportDate="%s" exportType="%s">%s`, exportDate, string(export.Type), "\n")
	if _, err := writer.Write([]byte(rootStart)); err != nil {
		return nil, errkit.Wrap(err, "failed to write root element")
	}

	switch export.Type {
	case models.ExportTypeFullDatabase:
		args := ExportArgs{IncludeFileData: export.IncludeFileData}
		if err := s.streamFullDatabase(ctx, writer, args, stats); err != nil {
			return nil, errkit.Wrap(err, "failed to stream full database")
		}
	case models.ExportTypeLocations:
		if err := s.streamLocations(ctx, writer, stats); err != nil {
			return nil, errkit.Wrap(err, "failed to stream locations")
		}
	case models.ExportTypeAreas:
		if err := s.streamAreas(ctx, writer, stats); err != nil {
			return nil, errkit.Wrap(err, "failed to stream areas")
		}
	case models.ExportTypeCommodities:
		args := ExportArgs{IncludeFileData: export.IncludeFileData}
		if err := s.streamCommodities(ctx, writer, args, stats); err != nil {
			return nil, errkit.Wrap(err, "failed to stream commodities")
		}
	case models.ExportTypeSelectedItems:
		args := ExportArgs{IncludeFileData: export.IncludeFileData}
		if err := s.streamSelectedItems(ctx, writer, export.SelectedItems, args, stats); err != nil {
			return nil, errkit.Wrap(err, "failed to stream selected items")
		}
	case models.ExportTypeImported:
		// Imported exports should not be processed through this function
		// They already have their XML file and should be marked as completed
		return nil, errkit.WithFields(ErrUnsupportedExportType, "type", export.Type, "reason", "imported exports should not be processed")
	default:
		return nil, errkit.WithFields(ErrUnsupportedExportType, "type", export.Type)
	}

	// End root element
	if _, err := writer.Write([]byte("</inventory>\n")); err != nil {
		return nil, errkit.Wrap(err, "failed to write closing root element")
	}

	return stats, nil
}

// streamFullDatabase streams all database content to the writer and tracks statistics
func (s *ExportService) streamFullDatabase(ctx context.Context, writer io.Writer, args ExportArgs, stats *ExportStats) error {
	if err := s.streamLocations(ctx, writer, stats); err != nil {
		return errkit.Wrap(err, "failed to stream locations")
	}
	if err := s.streamAreas(ctx, writer, stats); err != nil {
		return errkit.Wrap(err, "failed to stream areas")
	}
	if err := s.streamCommodities(ctx, writer, args, stats); err != nil {
		return errkit.Wrap(err, "failed to stream commodities")
	}
	return nil
}

// streamLocations streams locations to the writer and tracks statistics
func (s *ExportService) streamLocations(ctx context.Context, writer io.Writer, stats *ExportStats) error { //nolint:dupl // streamLocations and streamAreas have similar structure but are specific to their types
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
		stats.LocationCount++
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

// streamAreas streams areas to the writer and tracks statistics
func (s *ExportService) streamAreas(ctx context.Context, writer io.Writer, stats *ExportStats) error { //nolint:dupl // streamLocations and streamAreas have similar structure but are specific to their types
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
		stats.AreaCount++
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

// streamCommodities streams commodities to the writer and tracks statistics
func (s *ExportService) streamCommodities(ctx context.Context, writer io.Writer, args ExportArgs, stats *ExportStats) error {
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
			if err := s.streamCommodityDirectly(ctx, encoder, commodity, args, stats); err != nil {
				return errkit.Wrap(err, "failed to stream commodity")
			}
		} else {
			// Use traditional approach for commodities without file data
			xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, args, stats)
			if err != nil {
				return errkit.Wrap(err, "failed to convert commodity to XML")
			}
			if err := encoder.Encode(xmlCommodity); err != nil {
				return errkit.Wrap(err, "failed to encode commodity")
			}
		}
		stats.CommodityCount++
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

// streamSelectedItems streams selected items (locations, areas, commodities) to the writer and tracks statistics
func (s *ExportService) streamSelectedItems(ctx context.Context, writer io.Writer, selectedItems []models.ExportSelectedItem, args ExportArgs, stats *ExportStats) error {
	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Group items by type for better organization
	locations, areas, commodities := s.groupSelectedItemsByType(selectedItems)

	// Export each type of item with statistics tracking
	if err := s.streamSelectedLocations(ctx, encoder, locations, stats); err != nil {
		return err
	}
	if err := s.streamSelectedAreas(ctx, encoder, areas, stats); err != nil {
		return err
	}
	if err := s.streamSelectedCommodities(ctx, encoder, commodities, args, stats); err != nil {
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

// streamSelectedLocations streams location data to the XML encoder and tracks statistics
func (s *ExportService) streamSelectedLocations(ctx context.Context, encoder *xml.Encoder, locationIDs []string, stats *ExportStats) error {
	return s.streamEntitySection(ctx, encoder, "locations", locationIDs, stats, func(ctx context.Context, id string) (any, error) {
		location, err := s.registrySet.LocationRegistry.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		stats.LocationCount++
		return &Location{
			ID:      location.ID,
			Name:    location.Name,
			Address: location.Address,
		}, nil
	})
}

// streamSelectedAreas streams area data to the XML encoder and tracks statistics
func (s *ExportService) streamSelectedAreas(ctx context.Context, encoder *xml.Encoder, areaIDs []string, stats *ExportStats) error {
	return s.streamEntitySection(ctx, encoder, "areas", areaIDs, stats, func(ctx context.Context, id string) (any, error) {
		area, err := s.registrySet.AreaRegistry.Get(ctx, id)
		if err != nil {
			return nil, err
		}
		stats.AreaCount++
		return &Area{
			ID:         area.ID,
			Name:       area.Name,
			LocationID: area.LocationID,
		}, nil
	})
}

// streamEntitySection streams a section of entities to the XML encoder and tracks statistics
func (s *ExportService) streamEntitySection(ctx context.Context, encoder *xml.Encoder, sectionName string, entityIDs []string, stats *ExportStats, entityLoader func(context.Context, string) (any, error)) error {
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

// streamSelectedCommodities streams commodity data to the XML encoder and tracks statistics
func (s *ExportService) streamSelectedCommodities(ctx context.Context, encoder *xml.Encoder, commodityIDs []string, args ExportArgs, stats *ExportStats) error {
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
			if err := s.streamCommodityDirectly(ctx, encoder, commodity, args, stats); err != nil {
				return errkit.Wrap(err, "failed to stream commodity")
			}
		} else {
			// Use traditional approach for commodities without file data
			xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, args, stats)
			if err != nil {
				return errkit.Wrap(err, "failed to convert commodity to XML")
			}
			if err := encoder.Encode(xmlCommodity); err != nil {
				return errkit.Wrap(err, "failed to encode commodity")
			}
		}
		stats.CommodityCount++
	}

	endElement := xml.EndElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errkit.Wrap(err, "failed to encode commodities end element")
	}

	return nil
}

// convertCommodityToXML converts a commodity to XML format and tracks statistics
func (s *ExportService) convertCommodityToXML(ctx context.Context, commodity *models.Commodity, args ExportArgs, stats *ExportStats) (*Commodity, error) {
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

	// Handle file attachments (images, invoices, manuals) with statistics tracking
	if err := s.addFileAttachments(ctx, commodity.ID, xmlCommodity, args, stats); err != nil {
		return nil, errkit.Wrap(err, "failed to add file attachments")
	}

	return xmlCommodity, nil
}

// addFileAttachments adds file attachments (images, invoices, manuals) to the XML commodity and tracks statistics
func (s *ExportService) addFileAttachments(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs, stats *ExportStats) error {
	// Only count and add files if file data is included
	if !args.IncludeFileData {
		return nil
	}
	// Add images
	if err := s.addImages(ctx, commodityID, xmlCommodity, args, stats); err != nil {
		return errkit.Wrap(err, "failed to add images")
	}

	// Add invoices
	if err := s.addInvoices(ctx, commodityID, xmlCommodity, args, stats); err != nil {
		return errkit.Wrap(err, "failed to add invoices")
	}

	// Add manuals
	if err := s.addManuals(ctx, commodityID, xmlCommodity, args, stats); err != nil {
		return errkit.Wrap(err, "failed to add manuals")
	}

	return nil
}

// addImages adds images to the XML commodity and tracks statistics
func (s *ExportService) addImages(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs, stats *ExportStats) error {
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

		if args.IncludeFileData {
			if err := s.loadFileDataStreaming(ctx, xmlFile, stats); err != nil {
				// Don't fail the entire export if one file can't be read
				continue
			}
		}

		xmlCommodity.Images = append(xmlCommodity.Images, xmlFile)
		stats.ImageCount++
	}

	return nil
}

// addInvoices adds invoices to the XML commodity and tracks statistics
func (s *ExportService) addInvoices(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs, stats *ExportStats) error {
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
			if err := s.loadFileDataStreaming(ctx, xmlFile, stats); err != nil {
				// Don't fail the entire export if one file can't be read
				continue
			}
		}

		xmlCommodity.Invoices = append(xmlCommodity.Invoices, xmlFile)
		stats.InvoiceCount++
	}

	return nil
}

// addManuals adds manuals to the XML commodity and tracks statistics
func (s *ExportService) addManuals(ctx context.Context, commodityID string, xmlCommodity *Commodity, args ExportArgs, stats *ExportStats) error {
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
			if err := s.loadFileDataStreaming(ctx, xmlFile, stats); err != nil {
				// Don't fail the entire export if one file can't be read
				continue
			}
		}

		xmlCommodity.Manuals = append(xmlCommodity.Manuals, xmlFile)
		stats.ManualCount++
	}

	return nil
}

// loadFileDataStreaming loads file data using a memory-efficient streaming approach and tracks base64 size
func (s *ExportService) loadFileDataStreaming(ctx context.Context, xmlFile *File, stats *ExportStats) error {
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

	// Encode to base64 and track the encoded size
	encodedData := base64.StdEncoding.EncodeToString(fileData)
	xmlFile.Data = encodedData

	// Add the base64 encoded size to statistics
	stats.BinaryDataSize += int64(len(encodedData))

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

// streamFileCollectionDirectly is a generic helper for streaming file collections
func (s *ExportService) streamFileCollectionDirectly(ctx context.Context, encoder *xml.Encoder, elementName string, fileIDs []string, reg any, stats *ExportStats, counter *int) error {
	if len(fileIDs) == 0 {
		return nil
	}

	// Start element
	startElement := xml.StartElement{Name: xml.Name{Local: elementName}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errkit.Wrap(err, "failed to encode "+elementName+" start element")
	}

	for _, fileID := range fileIDs {
		var file any
		var err error

		// Get file based on registry type
		switch r := reg.(type) {
		case registry.ImageRegistry:
			file, err = r.Get(ctx, fileID)
		case registry.InvoiceRegistry:
			file, err = r.Get(ctx, fileID)
		case registry.ManualRegistry:
			file, err = r.Get(ctx, fileID)
		default:
			continue
		}

		if err != nil {
			continue // Skip files that can't be found
		}

		// Convert to XML File struct based on type
		var xmlFile *File
		switch f := file.(type) {
		case *models.Image:
			xmlFile = &File{
				ID:           f.ID,
				Path:         f.Path,
				OriginalPath: f.OriginalPath,
				Extension:    f.Ext,
				MimeType:     f.MIMEType,
			}
		case *models.Invoice:
			xmlFile = &File{
				ID:           f.ID,
				Path:         f.Path,
				OriginalPath: f.OriginalPath,
				Extension:    f.Ext,
				MimeType:     f.MIMEType,
			}
		case *models.Manual:
			xmlFile = &File{
				ID:           f.ID,
				Path:         f.Path,
				OriginalPath: f.OriginalPath,
				Extension:    f.Ext,
				MimeType:     f.MIMEType,
			}
		default:
			continue
		}

		// Stream file data directly
		if err := s.streamFileDataDirectly(ctx, encoder, xmlFile, stats); err != nil {
			continue // Don't fail the entire export if one file can't be read
		}

		if counter != nil {
			*counter++
		}
	}

	// End element
	endElement := xml.EndElement{Name: xml.Name{Local: elementName}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errkit.Wrap(err, "failed to encode "+elementName+" end element")
	}

	return nil
}

// encodeCommodityMetadata encodes commodity metadata elements
func (s *ExportService) encodeCommodityMetadata(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity) error {
	if err := s.encodeBasicCommodityFields(encoder, commodity); err != nil {
		return err
	}

	if err := s.encodeCommodityPricing(encoder, commodity); err != nil {
		return err
	}

	if err := s.encodeCommodityStatus(encoder, commodity); err != nil {
		return err
	}

	if err := s.encodeCommodityDates(encoder, commodity); err != nil {
		return err
	}

	if err := s.encodeCommodityArrays(encoder, commodity); err != nil {
		return err
	}

	return s.encodeURLs(encoder, commodity.URLs)
}

// encodeBasicCommodityFields encodes basic commodity fields
func (s *ExportService) encodeBasicCommodityFields(encoder *xml.Encoder, commodity *models.Commodity) error {
	encodeTextElement := s.createTextElementEncoder(encoder)

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
	return encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "count"}})
}

// encodeCommodityPricing encodes commodity pricing fields
func (s *ExportService) encodeCommodityPricing(encoder *xml.Encoder, commodity *models.Commodity) error {
	encodeTextElement := s.createTextElementEncoder(encoder)

	if err := encodeTextElement("originalPrice", commodity.OriginalPrice.String()); err != nil {
		return err
	}
	if err := encodeTextElement("originalPriceCurrency", string(commodity.OriginalPriceCurrency)); err != nil {
		return err
	}
	if err := encodeTextElement("convertedOriginalPrice", commodity.ConvertedOriginalPrice.String()); err != nil {
		return err
	}
	return encodeTextElement("currentPrice", commodity.CurrentPrice.String())
}

// encodeCommodityStatus encodes commodity status and other simple fields
func (s *ExportService) encodeCommodityStatus(encoder *xml.Encoder, commodity *models.Commodity) error {
	encodeTextElement := s.createTextElementEncoder(encoder)

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
	return encoder.EncodeToken(xml.EndElement{Name: xml.Name{Local: "draft"}})
}

// encodeCommodityDates encodes commodity date fields
func (s *ExportService) encodeCommodityDates(encoder *xml.Encoder, commodity *models.Commodity) error {
	encodeTextElement := s.createTextElementEncoder(encoder)

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
	return nil
}

// encodeCommodityArrays encodes commodity array fields
func (s *ExportService) encodeCommodityArrays(encoder *xml.Encoder, commodity *models.Commodity) error {
	if err := s.encodeStringArray(encoder, "extraSerialNumbers", "serialNumber", commodity.ExtraSerialNumbers); err != nil {
		return err
	}
	if err := s.encodeStringArray(encoder, "partNumbers", "partNumber", commodity.PartNumbers); err != nil {
		return err
	}
	return s.encodeStringArray(encoder, "tags", "tag", commodity.Tags)
}

// createTextElementEncoder creates a reusable function for encoding text elements
func (s *ExportService) createTextElementEncoder(encoder *xml.Encoder) func(name, value string) error {
	return func(name, value string) error {
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

// streamCommodityDirectly streams a commodity with file attachments directly to XML encoder and tracks statistics
func (s *ExportService) streamCommodityDirectly(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, args ExportArgs, stats *ExportStats) error {
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
		if err := s.streamFileAttachmentsDirectly(ctx, encoder, commodity, args, stats); err != nil {
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

// streamFileAttachmentsDirectly streams file attachments directly to XML encoder for large files and tracks statistics
func (s *ExportService) streamFileAttachmentsDirectly(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, args ExportArgs, stats *ExportStats) error {
	if !args.IncludeFileData {
		return nil // No file data to stream
	}

	// Stream images
	if err := s.streamImagesDirectly(ctx, encoder, commodity.ID, stats); err != nil {
		return errkit.Wrap(err, "failed to stream images")
	}

	// Stream invoices
	if err := s.streamInvoicesDirectly(ctx, encoder, commodity.ID, stats); err != nil {
		return errkit.Wrap(err, "failed to stream invoices")
	}

	// Stream manuals
	if err := s.streamManualsDirectly(ctx, encoder, commodity.ID, stats); err != nil {
		return errkit.Wrap(err, "failed to stream manuals")
	}

	return nil
}

// streamImagesDirectly streams images directly to XML encoder and tracks statistics
func (s *ExportService) streamImagesDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string, stats *ExportStats) error {
	imageIDs, err := s.registrySet.CommodityRegistry.GetImages(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get images")
	}

	return s.streamFileCollectionDirectly(ctx, encoder, "images", imageIDs, s.registrySet.ImageRegistry, stats, &stats.ImageCount)
}

// streamInvoicesDirectly streams invoices directly to XML encoder and tracks statistics
func (s *ExportService) streamInvoicesDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string, stats *ExportStats) error {
	invoiceIDs, err := s.registrySet.CommodityRegistry.GetInvoices(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get invoices")
	}

	return s.streamFileCollectionDirectly(ctx, encoder, "invoices", invoiceIDs, s.registrySet.InvoiceRegistry, stats, &stats.InvoiceCount)
}

// streamManualsDirectly streams manuals directly to XML encoder and tracks statistics
func (s *ExportService) streamManualsDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string, stats *ExportStats) error {
	manualIDs, err := s.registrySet.CommodityRegistry.GetManuals(ctx, commodityID)
	if err != nil {
		return errkit.Wrap(err, "failed to get manuals")
	}

	return s.streamFileCollectionDirectly(ctx, encoder, "manuals", manualIDs, s.registrySet.ManualRegistry, stats, &stats.ManualCount)
}

// streamFileDataDirectly streams file data directly to XML encoder without loading into memory and tracks base64 size
func (s *ExportService) streamFileDataDirectly(ctx context.Context, encoder *xml.Encoder, xmlFile *File, stats *ExportStats) error {
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

	// Stream file content as base64 encoded chunks and track size
	base64Size, err := s.streamBase64Content(encoder, reader)
	if err != nil {
		return errkit.Wrap(err, "failed to stream file content")
	}

	// Add the base64 encoded size to statistics
	stats.BinaryDataSize += base64Size

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

// streamBase64Content streams file content as base64 encoded data in chunks directly to XML and tracks size
func (s *ExportService) streamBase64Content(encoder *xml.Encoder, reader *blob.Reader) (int64, error) {
	// Create a custom writer that writes base64 chunks directly to XML encoder and tracks size
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
				writeErr = errors.Join(writeErr, base64Encoder.Close())
				return 0, errkit.Wrap(writeErr, "failed to write chunk to base64 encoder")
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			err = errors.Join(err, base64Encoder.Close())
			return 0, errkit.Wrap(err, "failed to read chunk")
		}
	}

	// Close the base64 encoder to flush any remaining data
	if err := base64Encoder.Close(); err != nil {
		return 0, errkit.Wrap(err, "failed to close base64 encoder")
	}

	return xmlWriter.totalSize, nil
}

func (s *ExportService) enrichSelectedItemsWithNames(ctx context.Context, export *models.Export) error {
	for i, item := range export.SelectedItems {
		var name string
		var locationID, areaID string
		var err error

		switch item.Type {
		case models.ExportSelectedItemTypeLocation:
			location, getErr := s.registrySet.LocationRegistry.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Location " + item.ID + "]"
			} else {
				name = location.Name
			}
		case models.ExportSelectedItemTypeArea:
			area, getErr := s.registrySet.AreaRegistry.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Area " + item.ID + "]"
			} else {
				name = area.Name
				locationID = area.LocationID // Store the relationship
			}
		case models.ExportSelectedItemTypeCommodity:
			commodity, getErr := s.registrySet.CommodityRegistry.Get(ctx, item.ID)
			if getErr != nil {
				// If item doesn't exist, use a fallback name
				name = "[Deleted Commodity " + item.ID + "]"
			} else {
				name = commodity.Name
				areaID = commodity.AreaID // Store the relationship
			}
		default:
			name = "[Unknown Item " + item.ID + "]"
		}

		if err != nil {
			return errkit.Wrap(err, "failed to fetch item name")
		}

		// Update the item with the name and relationships
		export.SelectedItems[i].Name = name
		export.SelectedItems[i].LocationID = locationID
		export.SelectedItems[i].AreaID = areaID
	}

	return nil
}

// countLocations counts location elements in XML
func (s *ExportService) countLocations(decoder *xml.Decoder, stats *ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read location token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "location" {
				stats.LocationCount++
			}
		case xml.EndElement:
			if t.Name.Local == "locations" {
				return nil
			}
		}
	}
}

// countAreas counts area elements in XML
func (s *ExportService) countAreas(decoder *xml.Decoder, stats *ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read area token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "area" {
				stats.AreaCount++
			}
		case xml.EndElement:
			if t.Name.Local == "areas" {
				return nil
			}
		}
	}
}

// countCommodities counts commodity elements and their files in XML
func (s *ExportService) countCommodities(decoder *xml.Decoder, stats *ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodity token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "commodity":
				stats.CommodityCount++
			case "images":
				if err := s.countFiles(decoder, "images", &stats.ImageCount, stats); err != nil {
					return errkit.Wrap(err, "failed to count images")
				}
			case "invoices":
				if err := s.countFiles(decoder, "invoices", &stats.InvoiceCount, stats); err != nil {
					return errkit.Wrap(err, "failed to count invoices")
				}
			case "manuals":
				if err := s.countFiles(decoder, "manuals", &stats.ManualCount, stats); err != nil {
					return errkit.Wrap(err, "failed to count manuals")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "commodities" {
				return nil
			}
		}
	}
}

// countFiles counts file elements within a file section and estimates binary data size
func (s *ExportService) countFiles(decoder *xml.Decoder, sectionName string, counter *int, stats *ExportStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file section token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "file" {
				*counter++
				// Process the file element to detect if it has data
				if err := s.processFileElement(decoder, stats); err != nil {
					return errkit.Wrap(err, "failed to process file element")
				}
			}
		case xml.EndElement:
			if t.Name.Local == sectionName {
				return nil
			}
		case xml.CharData:
			// Skip character data at this level
			continue
		}
	}
}

// processFileElement processes a file element to detect and estimate binary data size
func (s *ExportService) processFileElement(decoder *xml.Decoder, stats *ExportStats) error {
	depth := 1 // We're inside a file element

	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file element token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
			if t.Name.Local == "data" {
				// We found a data element - estimate its size efficiently
				if err := s.estimateDataElementSize(decoder, stats); err != nil {
					return errkit.Wrap(err, "failed to estimate data element size")
				}
				depth-- // estimateDataElementSize consumes the end element
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			// Skip character data at this level
			continue
		}
	}

	return nil
}

// estimateDataElementSize efficiently estimates the size of a data element without loading all content
func (s *ExportService) estimateDataElementSize(decoder *xml.Decoder, stats *ExportStats) error {
	var totalBase64Length int64
	const maxSampleSize = 1024 * 1024 // Sample up to 1MB to estimate size
	var sampledLength int64

	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read data element token")
		}

		switch t := tok.(type) {
		case xml.CharData:
			dataLength := int64(len(t))
			totalBase64Length += dataLength

			// Only sample the first part for efficiency
			if sampledLength < maxSampleSize {
				sampledLength += dataLength
			}
		case xml.EndElement:
			if t.Name.Local == "data" {
				// Convert base64 length to original binary size (approximately 3/4 of base64 length)
				if totalBase64Length > 0 {
					originalSize := (totalBase64Length * 3) / 4
					stats.BinaryDataSize += originalSize
				}
				return nil
			}
		}
	}
}

// xmlBase64Writer is a custom writer that writes base64 data directly to XML encoder and tracks size
type xmlBase64Writer struct {
	encoder   *xml.Encoder
	totalSize int64
}

// Write implements io.Writer interface to write base64 chunks directly to XML and track size
func (w *xmlBase64Writer) Write(p []byte) (n int, err error) {
	if err := w.encoder.EncodeToken(xml.CharData(p)); err != nil {
		return 0, err
	}
	w.totalSize += int64(len(p))
	return len(p), nil
}
