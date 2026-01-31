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

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/export/types"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ExtractTenantUserFromContext extracts tenant and user IDs from context
// Returns an error if context is missing required tenant/user information
func ExtractTenantUserFromContext(ctx context.Context) (tenantID, userID string, err error) {
	// Try to extract user from context using proper typed keys
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return "", "", errors.New("user context is required but not found")
	}

	// Extract user ID
	userID = user.ID
	if userID == "" {
		return "", "", errors.New("user ID is empty in context")
	}

	// Extract tenant ID from user
	tenantID = user.TenantID
	if tenantID == "" {
		return "", "", errors.New("tenant ID is empty in user context")
	}

	return tenantID, userID, nil
}

// ExportArgs contains arguments for export operations
type ExportArgs struct {
	IncludeFileData bool
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
	factorySet     *registry.FactorySet
	uploadLocation string
}

// NewExportService creates a new export service
func NewExportService(factorySet *registry.FactorySet, uploadLocation string) *ExportService {
	return &ExportService{
		factorySet:     factorySet,
		uploadLocation: uploadLocation,
	}
}

// ProcessExport processes an export request in the background
func (s *ExportService) ProcessExport(ctx context.Context, exportID string) error {
	// Get the export request
	export, err := s.factorySet.ExportRegistryFactory.CreateServiceRegistry().Get(ctx, exportID)
	if err != nil {
		return errxtrace.Wrap("failed to get export", err)
	}

	// Skip processing for imported exports - they are already completed
	if export.Type == models.ExportTypeImported {
		return nil
	}

	user, err := s.factorySet.UserRegistry.Get(ctx, export.UserID)
	if err != nil {
		return errxtrace.Wrap("failed to get user", err)
	}

	ctx = appctx.WithUser(ctx, user)

	// Update status to in_progress
	export.Status = models.ExportStatusInProgress
	expReg, err := s.factorySet.ExportRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get export registry", err)
	}
	_, err = expReg.Update(ctx, *export)
	if err != nil {
		return errxtrace.Wrap("failed to update export status", err)
	}

	// Generate the export and collect statistics using user context
	filePath, stats, err := s.generateExport(ctx, *export)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		_, expErr := s.factorySet.ExportRegistryFactory.CreateServiceRegistry().Update(ctx, *export)
		return errxtrace.Wrap("failed to generate export", errors.Join(err, expErr))
	}

	// Create file entity for the export using user context
	fileEntity, err := s.createExportFileEntity(ctx, export.ID, export.Description, filePath)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		_, updateErr := s.factorySet.ExportRegistryFactory.CreateServiceRegistry().Update(ctx, *export)
		return errxtrace.Wrap("failed to create export file entity", errors.Join(err, updateErr))
	}

	// Store statistics in export record
	export.LocationCount = stats.LocationCount
	export.AreaCount = stats.AreaCount
	export.CommodityCount = stats.CommodityCount
	export.ImageCount = stats.ImageCount
	export.InvoiceCount = stats.InvoiceCount
	export.ManualCount = stats.ManualCount
	export.BinaryDataSize = stats.BinaryDataSize

	// Get file size using user context
	if fileSize, err := s.getFileSize(ctx, filePath); err == nil {
		export.FileSize = fileSize
	}

	// Update status to completed using user context
	export.Status = models.ExportStatusCompleted
	export.FileID = &fileEntity.ID
	export.FilePath = filePath // Keep for backward compatibility during migration
	export.CompletedDate = models.PNow()
	export.ErrorMessage = ""

	userReg, err := s.factorySet.ExportRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get export registry", err)
	}

	_, err = userReg.Update(ctx, *export)
	if err != nil {
		return errxtrace.Wrap("failed to update export completion", err)
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

	// Extract tenant and user from context
	tenantID, userID, err := ExtractTenantUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to extract tenant/user context", err)
	}

	// Create file entity
	now := time.Now()
	fileEntity := models.FileEntity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   userID,
		},
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

	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get file registry", err)
	}

	// Create the file entity
	created, err := fileReg.Create(ctx, fileEntity)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create file entity", err)
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
		return errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			err = errxtrace.Wrap("failed to close blob bucket", closeErr)
		}
	}()

	// Delete the file
	err = b.Delete(ctx, filePath)
	if err != nil {
		return errxtrace.Wrap("failed to delete export file", err)
	}

	return nil
}

// generateExport generates the XML export file using blob storage and returns statistics
func (s *ExportService) generateExport(ctx context.Context, export models.Export) (string, *types.ExportStats, error) {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer func() {
		if closeErr := b.Close(); closeErr != nil {
			err = errxtrace.Wrap("failed to close blob bucket", closeErr)
		}
	}()

	// Generate blob key (filename)
	timestamp := time.Now().Format("20060102_150405")
	blobKey := fmt.Sprintf("exports/export_%s_%s.xml", strings.ToLower(string(export.Type)), timestamp)

	// Create blob writer
	writer, err := b.NewWriter(ctx, blobKey, nil)
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to create blob writer", err)
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			err = errxtrace.Wrap("failed to close blob writer", closeErr)
		}
	}()

	// Stream XML generation with statistics tracking
	stats, err := s.streamXMLExport(ctx, export, writer)
	if err != nil {
		return "", nil, errxtrace.Wrap("failed to generate XML export", err)
	}

	return blobKey, stats, nil
}

// getFileSize gets the size of a file in blob storage
func (s *ExportService) getFileSize(ctx context.Context, filePath string) (int64, error) {
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open bucket", err)
	}
	defer b.Close()

	attrs, err := b.Attributes(ctx, filePath)
	if err != nil {
		return 0, errxtrace.Wrap("failed to get file attributes", err)
	}

	return attrs.Size, nil
}

// streamXMLExport streams XML data directly to the file writer and tracks statistics
func (s *ExportService) streamXMLExport(ctx context.Context, export models.Export, writer io.Writer) (*types.ExportStats, error) {
	stats := &types.ExportStats{}

	// Write XML header
	if _, err := writer.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")); err != nil {
		return nil, errxtrace.Wrap("failed to write XML header", err)
	}

	// Start root element
	exportDate := time.Now().Format("2006-01-02T15:04:05Z")
	rootStart := fmt.Sprintf(`<inventory exportDate="%s" exportType="%s">%s`, exportDate, string(export.Type), "\n")
	if _, err := writer.Write([]byte(rootStart)); err != nil {
		return nil, errxtrace.Wrap("failed to write root element", err)
	}

	user, err := s.factorySet.UserRegistry.Get(ctx, export.UserID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user", err)
	}
	ctx = appctx.WithUser(ctx, user)

	switch export.Type {
	case models.ExportTypeFullDatabase:
		if err := s.streamFullDatabase(ctx, writer, export, stats); err != nil {
			return nil, errxtrace.Wrap("failed to stream full database", err)
		}
	case models.ExportTypeLocations:
		if err := s.streamLocations(ctx, writer, export, stats); err != nil {
			return nil, errxtrace.Wrap("failed to stream locations", err)
		}
	case models.ExportTypeAreas:
		if err := s.streamAreas(ctx, writer, export, stats); err != nil {
			return nil, errxtrace.Wrap("failed to stream areas", err)
		}
	case models.ExportTypeCommodities:
		if err := s.streamCommodities(ctx, writer, export, stats); err != nil {
			return nil, errxtrace.Wrap("failed to stream commodities", err)
		}
	case models.ExportTypeSelectedItems:
		if err := s.streamSelectedItems(ctx, writer, export, stats); err != nil {
			return nil, errxtrace.Wrap("failed to stream selected items", err)
		}
	case models.ExportTypeImported:
		// Imported exports should not be processed through this function
		// They already have their XML file and should be marked as completed
		return nil, errx.Classify(ErrUnsupportedExportType, errx.Attrs("type", export.Type, "reason", "imported exports should not be processed"))
	default:
		return nil, errx.Classify(ErrUnsupportedExportType, errx.Attrs("type", export.Type))
	}

	// End root element
	if _, err := writer.Write([]byte("</inventory>\n")); err != nil {
		return nil, errxtrace.Wrap("failed to write closing root element", err)
	}

	return stats, nil
}

// streamFullDatabase streams all database content to the writer and tracks statistics
func (s *ExportService) streamFullDatabase(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats) error {
	if err := s.streamLocations(ctx, writer, export, stats); err != nil {
		return errxtrace.Wrap("failed to stream locations", err)
	}
	if err := s.streamAreas(ctx, writer, export, stats); err != nil {
		return errxtrace.Wrap("failed to stream areas", err)
	}
	if err := s.streamCommodities(ctx, writer, export, stats); err != nil {
		return errxtrace.Wrap("failed to stream commodities", err)
	}
	return nil
}

// streamLocations streams locations to the writer and tracks statistics
func (s *ExportService) streamLocations(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats) error { //nolint:dupl // streamLocations and streamAreas have similar structure but are specific to their types
	locReg, err := s.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get location registry", err)
	}
	locations, err := locReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get locations", err)
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Start locations element
	startElement := xml.StartElement{Name: xml.Name{Local: "locations"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errxtrace.Wrap("failed to encode locations start element", err)
	}

	for _, loc := range locations {
		xmlLoc := &Location{
			ID:      loc.ID,
			Name:    loc.Name,
			Address: loc.Address,
		}
		if err := encoder.Encode(xmlLoc); err != nil {
			return errxtrace.Wrap("failed to encode location", err)
		}
		stats.LocationCount++
	}

	// End locations element
	endElement := xml.EndElement{Name: xml.Name{Local: "locations"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errxtrace.Wrap("failed to encode locations end element", err)
	}

	if err := encoder.Flush(); err != nil {
		return errxtrace.Wrap("failed to flush encoder", err)
	}

	return nil
}

// streamAreas streams areas to the writer and tracks statistics
func (s *ExportService) streamAreas(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats) error { //nolint:dupl // streamLocations and streamAreas have similar structure but are specific to their types
	areaReg, err := s.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get area registry", err)
	}
	areas, err := areaReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get areas", err)
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Start areas element
	startElement := xml.StartElement{Name: xml.Name{Local: "areas"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errxtrace.Wrap("failed to encode areas start element", err)
	}

	for _, area := range areas {
		xmlArea := &Area{
			ID:         area.ID,
			Name:       area.Name,
			LocationID: area.LocationID,
		}
		if err := encoder.Encode(xmlArea); err != nil {
			return errxtrace.Wrap("failed to encode area", err)
		}
		stats.AreaCount++
	}

	// End areas element
	endElement := xml.EndElement{Name: xml.Name{Local: "areas"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errxtrace.Wrap("failed to encode areas end element", err)
	}

	if err := encoder.Flush(); err != nil {
		return errxtrace.Wrap("failed to flush encoder", err)
	}

	return nil
}

// streamCommodities streams commodities to the writer and tracks statistics
func (s *ExportService) streamCommodities(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodity registry", err)
	}
	commodities, err := comReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodities", err)
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Start commodities element
	startElement := xml.StartElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errxtrace.Wrap("failed to encode commodities start element", err)
	}

	for _, commodity := range commodities {
		// Use streaming approach for commodities with file data
		if export.IncludeFileData {
			if err := s.streamCommodityDirectly(ctx, encoder, commodity, export, stats); err != nil {
				return errxtrace.Wrap("failed to stream commodity", err)
			}
		} else {
			// Use traditional approach for commodities without file data
			xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, export, stats)
			if err != nil {
				return errxtrace.Wrap("failed to convert commodity to XML", err)
			}
			if err := encoder.Encode(xmlCommodity); err != nil {
				return errxtrace.Wrap("failed to encode commodity", err)
			}
		}
		stats.CommodityCount++
	}

	// End commodities element
	endElement := xml.EndElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errxtrace.Wrap("failed to encode commodities end element", err)
	}

	if err := encoder.Flush(); err != nil {
		return errxtrace.Wrap("failed to flush encoder", err)
	}

	return nil
}

// streamSelectedItems streams selected items (locations, areas, commodities) to the writer and tracks statistics
func (s *ExportService) streamSelectedItems(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats) error {
	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Group items by type for better organization
	locations, areas, commodities := s.groupSelectedItemsByType(export.SelectedItems)

	// Export each type of item with statistics tracking
	if err := s.streamSelectedLocations(ctx, encoder, locations, stats); err != nil {
		return err
	}
	if err := s.streamSelectedAreas(ctx, encoder, areas, stats); err != nil {
		return err
	}
	if err := s.streamSelectedCommodities(ctx, encoder, commodities, export, stats); err != nil {
		return err
	}

	if err := encoder.Flush(); err != nil {
		return errxtrace.Wrap("failed to flush encoder", err)
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
func (s *ExportService) streamSelectedLocations(ctx context.Context, encoder *xml.Encoder, locationIDs []string, stats *types.ExportStats) error {
	return s.streamEntitySection(ctx, encoder, "locations", locationIDs, stats, func(ctx context.Context, id string) (any, error) {
		locReg, err := s.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return nil, err
		}
		location, err := locReg.Get(ctx, id)
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
func (s *ExportService) streamSelectedAreas(ctx context.Context, encoder *xml.Encoder, areaIDs []string, stats *types.ExportStats) error {
	return s.streamEntitySection(ctx, encoder, "areas", areaIDs, stats, func(ctx context.Context, id string) (any, error) {
		areaReg, err := s.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return nil, err
		}
		area, err := areaReg.Get(ctx, id)
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
func (s *ExportService) streamEntitySection(ctx context.Context, encoder *xml.Encoder, sectionName string, entityIDs []string, stats *types.ExportStats, entityLoader func(context.Context, string) (any, error)) error {
	if len(entityIDs) == 0 {
		return nil
	}

	startElement := xml.StartElement{Name: xml.Name{Local: sectionName}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errxtrace.Wrap("failed to encode "+sectionName+" start element", err)
	}

	for _, entityID := range entityIDs {
		entity, err := entityLoader(ctx, entityID)
		if err != nil {
			continue // Skip items that can't be found
		}

		if err := encoder.Encode(entity); err != nil {
			return errxtrace.Wrap("failed to encode "+sectionName+" entity", err)
		}
	}

	endElement := xml.EndElement{Name: xml.Name{Local: sectionName}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errxtrace.Wrap("failed to encode "+sectionName+" end element", err)
	}

	return nil
}

// streamSelectedCommodities streams commodity data to the XML encoder and tracks statistics
func (s *ExportService) streamSelectedCommodities(ctx context.Context, encoder *xml.Encoder, commodityIDs []string, export models.Export, stats *types.ExportStats) error {
	if len(commodityIDs) == 0 {
		return nil
	}

	startElement := xml.StartElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errxtrace.Wrap("failed to encode commodities start element", err)
	}

	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodity registry", err)
	}
	for _, commodityID := range commodityIDs {
		commodity, err := comReg.Get(ctx, commodityID)
		if err != nil {
			continue // Skip items that can't be found
		}

		// Use streaming approach for commodities with file data
		if export.IncludeFileData {
			if err := s.streamCommodityDirectly(ctx, encoder, commodity, export, stats); err != nil {
				return errxtrace.Wrap("failed to stream commodity", err)
			}
		} else {
			// Use traditional approach for commodities without file data
			xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, export, stats)
			if err != nil {
				return errxtrace.Wrap("failed to convert commodity to XML", err)
			}
			if err := encoder.Encode(xmlCommodity); err != nil {
				return errxtrace.Wrap("failed to encode commodity", err)
			}
		}
		stats.CommodityCount++
	}

	endElement := xml.EndElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errxtrace.Wrap("failed to encode commodities end element", err)
	}

	return nil
}

// convertCommodityToXML converts a commodity to XML format and tracks statistics
func (s *ExportService) convertCommodityToXML(ctx context.Context, commodity *models.Commodity, export models.Export, stats *types.ExportStats) (*Commodity, error) {
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
	if err := s.addFileAttachments(ctx, commodity.ID, xmlCommodity, export, stats); err != nil {
		return nil, errxtrace.Wrap("failed to add file attachments", err)
	}

	return xmlCommodity, nil
}

// addFileAttachments adds file attachments (images, invoices, manuals) to the XML commodity and tracks statistics
func (s *ExportService) addFileAttachments(ctx context.Context, commodityID string, xmlCommodity *Commodity, export models.Export, stats *types.ExportStats) error {
	// Only count and add files if file data is included
	if !export.IncludeFileData {
		return nil
	}
	// Add images
	if err := s.addImages(ctx, commodityID, xmlCommodity, export, stats); err != nil {
		return errxtrace.Wrap("failed to add images", err)
	}

	// Add invoices
	if err := s.addInvoices(ctx, commodityID, xmlCommodity, export, stats); err != nil {
		return errxtrace.Wrap("failed to add invoices", err)
	}

	// Add manuals
	if err := s.addManuals(ctx, commodityID, xmlCommodity, export, stats); err != nil {
		return errxtrace.Wrap("failed to add manuals", err)
	}

	return nil
}

// addImages adds images to the XML commodity and tracks statistics
func (s *ExportService) addImages(ctx context.Context, commodityID string, xmlCommodity *Commodity, export models.Export, stats *types.ExportStats) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return err
	}
	// Use the commodity registry to get related image IDs
	imageIDs, err := comReg.GetImages(ctx, commodityID)
	if err != nil {
		return errxtrace.Wrap("failed to get images", err)
	}

	imgReg, err := s.factorySet.ImageRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get image registry", err)
	}

	files, err := s.addFileCollection(ctx, imageIDs, imgReg, export, stats)
	if err != nil {
		return err
	}

	xmlCommodity.Images = files
	stats.ImageCount += len(files)
	return nil
}

// addInvoices adds invoices to the XML commodity and tracks statistics
func (s *ExportService) addInvoices(ctx context.Context, commodityID string, xmlCommodity *Commodity, export models.Export, stats *types.ExportStats) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodity registry", err)
	}
	invoiceIDs, err := comReg.GetInvoices(ctx, commodityID)
	if err != nil {
		return errxtrace.Wrap("failed to get invoices", err)
	}

	invReg, err := s.factorySet.InvoiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get invoice registry", err)
	}

	files, err := s.addFileCollection(ctx, invoiceIDs, invReg, export, stats)
	if err != nil {
		return err
	}

	xmlCommodity.Invoices = files
	stats.InvoiceCount += len(files)
	return nil
}

// addManuals adds manuals to the XML commodity and tracks statistics
func (s *ExportService) addManuals(ctx context.Context, commodityID string, xmlCommodity *Commodity, export models.Export, stats *types.ExportStats) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodity registry", err)
	}
	manualIDs, err := comReg.GetManuals(ctx, commodityID)
	if err != nil {
		return errxtrace.Wrap("failed to get manuals", err)
	}

	manReg, err := s.factorySet.ManualRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get manual registry", err)
	}

	files, err := s.addFileCollection(ctx, manualIDs, manReg, export, stats)
	if err != nil {
		return err
	}

	xmlCommodity.Manuals = files
	stats.ManualCount += len(files)
	return nil
}

// loadFileDataStreaming loads file data using a memory-efficient streaming approach and tracks base64 size
func (s *ExportService) loadFileDataStreaming(ctx context.Context, xmlFile *File, stats *types.ExportStats) error {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer b.Close()

	// Open file reader
	reader, err := b.NewReader(ctx, xmlFile.OriginalPath, nil)
	if err != nil {
		return errxtrace.Wrap("failed to create file reader", err)
	}
	defer reader.Close()

	// For backward compatibility, still load small files into memory
	// Large files should use streamFileDataDirectly instead
	fileData, err := io.ReadAll(reader)
	if err != nil {
		return errxtrace.Wrap("failed to read file data", err)
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

// addFileCollection is a generic helper for adding file collections to XML commodities
func (s *ExportService) addFileCollection(ctx context.Context, fileIDs []string, fileRegistry any, export models.Export, stats *types.ExportStats) ([]*File, error) {
	var files []*File

	for _, fileID := range fileIDs {
		var file any
		var err error

		// Use type assertion to call Get method on the registry
		switch reg := fileRegistry.(type) {
		case interface {
			Get(context.Context, string) (*models.Image, error)
		}:
			file, err = reg.Get(ctx, fileID)
		case interface {
			Get(context.Context, string) (*models.Invoice, error)
		}:
			file, err = reg.Get(ctx, fileID)
		case interface {
			Get(context.Context, string) (*models.Manual, error)
		}:
			file, err = reg.Get(ctx, fileID)
		default:
			continue // Skip unknown registry types
		}

		if err != nil {
			continue // Skip files that can't be found
		}

		var xmlFile *File

		// Convert to XML file based on type
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
			continue // Skip unknown file types
		}

		if export.IncludeFileData {
			if err := s.loadFileDataStreaming(ctx, xmlFile, stats); err != nil {
				// Don't fail the entire export if one file can't be read
				continue
			}
		}

		files = append(files, xmlFile)
	}

	return files, nil
}

// streamFileCollectionDirectly is a generic helper for streaming file collections
func (s *ExportService) streamFileCollectionDirectly(ctx context.Context, encoder *xml.Encoder, elementName string, fileIDs []string, reg any, stats *types.ExportStats, counter *int) error {
	if len(fileIDs) == 0 {
		return nil
	}

	// Start element
	startElement := xml.StartElement{Name: xml.Name{Local: elementName}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errxtrace.Wrap("failed to encode "+elementName+" start element", err)
	}

	var fileGetter func(context.Context, string) (any, error)
	switch r := reg.(type) {
	case registry.ImageRegistryFactory:
		imgReg, err := r.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create image registry", err)
		}
		fileGetter = func(ctx context.Context, id string) (any, error) {
			return imgReg.Get(ctx, id)
		}
	case registry.InvoiceRegistryFactory:
		invReg, err := r.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create invoice registry", err)
		}
		fileGetter = func(ctx context.Context, id string) (any, error) {
			return invReg.Get(ctx, id)
		}
	case registry.ManualRegistryFactory:
		manReg, err := r.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create manual registry", err)
		}
		fileGetter = func(ctx context.Context, id string) (any, error) {
			return manReg.Get(ctx, id)
		}
	default:
		return fmt.Errorf("unsupported file registry factory type: %T", reg)
	}

	for _, fileID := range fileIDs {
		var file any
		var err error

		// Get file based on registry type
		file, err = fileGetter(ctx, fileID)

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
		return errxtrace.Wrap("failed to encode "+elementName+" end element", err)
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
func (s *ExportService) streamCommodityDirectly(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, export models.Export, stats *types.ExportStats) error {
	// Start commodity element
	commodityStart := xml.StartElement{
		Name: xml.Name{Local: "commodity"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: commodity.ID},
		},
	}
	if err := encoder.EncodeToken(commodityStart); err != nil {
		return errxtrace.Wrap("failed to encode commodity start element", err)
	}

	// Encode commodity metadata
	if err := s.encodeCommodityMetadata(ctx, encoder, commodity); err != nil {
		return errxtrace.Wrap("failed to encode commodity metadata", err)
	}

	// Stream file attachments if requested - this bypasses the traditional File.Data approach
	if export.IncludeFileData {
		if err := s.streamFileAttachmentsDirectly(ctx, encoder, commodity, export, stats); err != nil {
			return errxtrace.Wrap("failed to stream file attachments", err)
		}
	}

	// End commodity element
	commodityEnd := xml.EndElement{Name: xml.Name{Local: "commodity"}}
	if err := encoder.EncodeToken(commodityEnd); err != nil {
		return errxtrace.Wrap("failed to encode commodity end element", err)
	}

	return nil
}

// streamFileAttachmentsDirectly streams file attachments directly to XML encoder for large files and tracks statistics
func (s *ExportService) streamFileAttachmentsDirectly(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, export models.Export, stats *types.ExportStats) error {
	if !export.IncludeFileData {
		return nil // No file data to stream
	}

	// Stream images
	if err := s.streamImagesDirectly(ctx, encoder, commodity.ID, export, stats); err != nil {
		return errxtrace.Wrap("failed to stream images", err)
	}

	// Stream invoices
	if err := s.streamInvoicesDirectly(ctx, encoder, commodity.ID, stats); err != nil {
		return errxtrace.Wrap("failed to stream invoices", err)
	}

	// Stream manuals
	if err := s.streamManualsDirectly(ctx, encoder, commodity.ID, stats); err != nil {
		return errxtrace.Wrap("failed to stream manuals", err)
	}

	return nil
}

// streamImagesDirectly streams images directly to XML encoder and tracks statistics
func (s *ExportService) streamImagesDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string, export models.Export, stats *types.ExportStats) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodity registry", err)
	}
	imageIDs, err := comReg.GetImages(ctx, commodityID)
	if err != nil {
		return errxtrace.Wrap("failed to get images", err)
	}

	return s.streamFileCollectionDirectly(ctx, encoder, "images", imageIDs, s.factorySet.ImageRegistryFactory, stats, &stats.ImageCount)
}

// streamInvoicesDirectly streams invoices directly to XML encoder and tracks statistics
func (s *ExportService) streamInvoicesDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string, stats *types.ExportStats) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodity registry", err)
	}

	invoiceIDs, err := comReg.GetInvoices(ctx, commodityID)
	if err != nil {
		return errxtrace.Wrap("failed to get invoices", err)
	}

	return s.streamFileCollectionDirectly(ctx, encoder, "invoices", invoiceIDs, s.factorySet.InvoiceRegistryFactory, stats, &stats.InvoiceCount)
}

// streamManualsDirectly streams manuals directly to XML encoder and tracks statistics
func (s *ExportService) streamManualsDirectly(ctx context.Context, encoder *xml.Encoder, commodityID string, stats *types.ExportStats) error {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get commodity registry", err)
	}

	manualIDs, err := comReg.GetManuals(ctx, commodityID)
	if err != nil {
		return errxtrace.Wrap("failed to get manuals", err)
	}

	return s.streamFileCollectionDirectly(ctx, encoder, "manuals", manualIDs, s.factorySet.ManualRegistryFactory, stats, &stats.ManualCount)
}

// streamFileDataDirectly streams file data directly to XML encoder without loading into memory and tracks base64 size
func (s *ExportService) streamFileDataDirectly(ctx context.Context, encoder *xml.Encoder, xmlFile *File, stats *types.ExportStats) error {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer b.Close()

	// Open file reader
	reader, err := b.NewReader(ctx, xmlFile.OriginalPath, nil)
	if err != nil {
		return errxtrace.Wrap("failed to create file reader", err)
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
		return errxtrace.Wrap("failed to encode file start element", err)
	}

	// Encode file metadata elements
	if err := s.encodeFileMetadata(encoder, xmlFile); err != nil {
		return errxtrace.Wrap("failed to encode file metadata", err)
	}

	// Start data element
	dataStart := xml.StartElement{Name: xml.Name{Local: "data"}}
	if err := encoder.EncodeToken(dataStart); err != nil {
		return errxtrace.Wrap("failed to encode data start element", err)
	}

	// Stream file content as base64 encoded chunks and track size
	base64Size, err := s.streamBase64Content(encoder, reader)
	if err != nil {
		return errxtrace.Wrap("failed to stream file content", err)
	}

	// Add the base64 encoded size to statistics
	stats.BinaryDataSize += base64Size

	// End data element
	dataEnd := xml.EndElement{Name: xml.Name{Local: "data"}}
	if err := encoder.EncodeToken(dataEnd); err != nil {
		return errxtrace.Wrap("failed to encode data end element", err)
	}

	// End file element
	fileEnd := xml.EndElement{Name: xml.Name{Local: "file"}}
	if err := encoder.EncodeToken(fileEnd); err != nil {
		return errxtrace.Wrap("failed to encode file end element", err)
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
				return 0, errxtrace.Wrap("failed to write chunk to base64 encoder", writeErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			err = errors.Join(err, base64Encoder.Close())
			return 0, errxtrace.Wrap("failed to read chunk", err)
		}
	}

	// Close the base64 encoder to flush any remaining data
	if err := base64Encoder.Close(); err != nil {
		return 0, errxtrace.Wrap("failed to close base64 encoder", err)
	}

	return xmlWriter.totalSize, nil
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
