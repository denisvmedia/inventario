package internal

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// ExportService handles the background processing of export requests
type ExportService struct {
	registrySet    *registry.Set
	exportDir      string
	uploadLocation string
}

// NewExportService creates a new export service
func NewExportService(registrySet *registry.Set, exportDir, uploadLocation string) *ExportService {
	return &ExportService{
		registrySet:    registrySet,
		exportDir:      exportDir,
		uploadLocation: uploadLocation,
	}
}

// InventoryData represents the root XML structure for exports
type InventoryData struct {
	XMLName     xml.Name    `xml:"inventory"`
	ExportDate  string      `xml:"export_date,attr"`
	ExportType  string      `xml:"export_type,attr"`
	Locations   []*Location `xml:"locations>location,omitempty"`
	Areas       []*Area     `xml:"areas>area,omitempty"`
	Commodities []*Commodity `xml:"commodities>commodity,omitempty"`
}

type Location struct {
	XMLName xml.Name `xml:"location"`
	ID      string   `xml:"id,attr"`
	Name    string   `xml:"location_name"`
	Address string   `xml:"address"`
}

type Area struct {
	XMLName    xml.Name `xml:"area"`
	ID         string   `xml:"id,attr"`
	Name       string   `xml:"area_name"`
	LocationID string   `xml:"location_id"`
}

type Commodity struct {
	XMLName                xml.Name `xml:"commodity"`
	ID                     string   `xml:"id,attr"`
	Name                   string   `xml:"commodity_name"`
	ShortName              string   `xml:"short_name,omitempty"`
	Type                   string   `xml:"type"`
	AreaID                 string   `xml:"area_id"`
	Count                  int      `xml:"count"`
	OriginalPrice          string   `xml:"original_price,omitempty"`
	OriginalPriceCurrency  string   `xml:"original_price_currency,omitempty"`
	ConvertedOriginalPrice string   `xml:"converted_original_price,omitempty"`
	CurrentPrice           string   `xml:"current_price,omitempty"`
	SerialNumber           string   `xml:"serial_number,omitempty"`
	ExtraSerialNumbers     []string `xml:"extra_serial_numbers>serial_number,omitempty"`
	PartNumbers            []string `xml:"part_numbers>part_number,omitempty"`
	Tags                   []string `xml:"tags>tag,omitempty"`
	Status                 string   `xml:"status"`
	PurchaseDate           string   `xml:"purchase_date,omitempty"`
	RegisteredDate         string   `xml:"registered_date,omitempty"`
	LastModifiedDate       string   `xml:"last_modified_date,omitempty"`
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
	XMLName      xml.Name `xml:",any"`
	ID           string   `xml:"id,attr"`
	Path         string   `xml:"path"`
	OriginalPath string   `xml:"original_path"`
	Extension    string   `xml:"extension"`
	MimeType     string   `xml:"mime_type"`
	Data         string   `xml:"data,omitempty"` // Base64 encoded file data if include_file_data is true
}

// ProcessExport processes an export request in the background
func (s *ExportService) ProcessExport(ctx context.Context, exportID string) error {
	// Get the export request
	export, err := s.registrySet.ExportRegistry.Get(ctx, exportID)
	if err != nil {
		return fmt.Errorf("failed to get export: %w", err)
	}

	// Update status to in_progress
	export.Status = models.ExportStatusInProgress
	_, err = s.registrySet.ExportRegistry.Update(ctx, *export)
	if err != nil {
		return fmt.Errorf("failed to update export status: %w", err)
	}

	// Generate the export
	filePath, err := s.generateExport(ctx, *export)
	if err != nil {
		// Update status to failed
		export.Status = models.ExportStatusFailed
		export.ErrorMessage = err.Error()
		s.registrySet.ExportRegistry.Update(ctx, *export)
		return fmt.Errorf("failed to generate export: %w", err)
	}

	// Update status to completed
	export.Status = models.ExportStatusCompleted
	export.FilePath = filePath
	completedDate := models.Date(time.Now().Format("2006-01-02"))
	export.CompletedDate = &completedDate
	export.ErrorMessage = ""

	_, err = s.registrySet.ExportRegistry.Update(ctx, *export)
	if err != nil {
		return fmt.Errorf("failed to update export completion: %w", err)
	}

	return nil
}

// generateExport generates the XML export file
func (s *ExportService) generateExport(ctx context.Context, export models.Export) (string, error) {
	// Create export directory if it doesn't exist
	if err := os.MkdirAll(s.exportDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create export directory: %w", err)
	}

	// Generate filename
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("export_%s_%s.xml", strings.ToLower(string(export.Type)), timestamp)
	filePath := filepath.Join(s.exportDir, filename)

	// Generate XML data
	data, err := s.generateXMLData(ctx, export)
	if err != nil {
		return "", fmt.Errorf("failed to generate XML data: %w", err)
	}

	// Write XML to file
	file, err := os.Create(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to create export file: %w", err)
	}
	defer file.Close()

	// Write XML header
	file.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")

	// Marshal and write XML data
	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	if err := encoder.Encode(data); err != nil {
		return "", fmt.Errorf("failed to write XML data: %w", err)
	}

	return filePath, nil
}

// generateXMLData generates the XML data structure based on export type
func (s *ExportService) generateXMLData(ctx context.Context, export models.Export) (*InventoryData, error) {
	data := &InventoryData{
		ExportDate: time.Now().Format("2006-01-02T15:04:05Z"),
		ExportType: string(export.Type),
	}

	switch export.Type {
	case models.ExportTypeFullDatabase:
		return s.exportFullDatabase(ctx, data, export.IncludeFileData)
	case models.ExportTypeLocations:
		return s.exportLocations(ctx, data)
	case models.ExportTypeAreas:
		return s.exportAreas(ctx, data)
	case models.ExportTypeCommodities:
		return s.exportCommodities(ctx, data, export.IncludeFileData)
	case models.ExportTypeSelectedItems:
		return s.exportSelectedItems(ctx, data, export.SelectedItemIDs, export.IncludeFileData)
	default:
		return nil, fmt.Errorf("unsupported export type: %s", export.Type)
	}
}

func (s *ExportService) exportFullDatabase(ctx context.Context, data *InventoryData, includeFileData bool) (*InventoryData, error) {
	// Export locations
	locations, err := s.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get locations: %w", err)
	}
	for _, loc := range locations {
		data.Locations = append(data.Locations, &Location{
			ID:      loc.ID,
			Name:    loc.Name,
			Address: loc.Address,
		})
	}

	// Export areas
	areas, err := s.registrySet.AreaRegistry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get areas: %w", err)
	}
	for _, area := range areas {
		data.Areas = append(data.Areas, &Area{
			ID:         area.ID,
			Name:       area.Name,
			LocationID: area.LocationID,
		})
	}

	// Export commodities
	commodities, err := s.registrySet.CommodityRegistry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get commodities: %w", err)
	}
	for _, commodity := range commodities {
		xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, includeFileData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert commodity to XML: %w", err)
		}
		data.Commodities = append(data.Commodities, xmlCommodity)
	}

	return data, nil
}

func (s *ExportService) exportLocations(ctx context.Context, data *InventoryData) (*InventoryData, error) {
	locations, err := s.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get locations: %w", err)
	}
	for _, loc := range locations {
		data.Locations = append(data.Locations, &Location{
			ID:      loc.ID,
			Name:    loc.Name,
			Address: loc.Address,
		})
	}
	return data, nil
}

func (s *ExportService) exportAreas(ctx context.Context, data *InventoryData) (*InventoryData, error) {
	areas, err := s.registrySet.AreaRegistry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get areas: %w", err)
	}
	for _, area := range areas {
		data.Areas = append(data.Areas, &Area{
			ID:         area.ID,
			Name:       area.Name,
			LocationID: area.LocationID,
		})
	}
	return data, nil
}

func (s *ExportService) exportCommodities(ctx context.Context, data *InventoryData, includeFileData bool) (*InventoryData, error) {
	commodities, err := s.registrySet.CommodityRegistry.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get commodities: %w", err)
	}
	for _, commodity := range commodities {
		xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, includeFileData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert commodity to XML: %w", err)
		}
		data.Commodities = append(data.Commodities, xmlCommodity)
	}
	return data, nil
}

func (s *ExportService) exportSelectedItems(ctx context.Context, data *InventoryData, selectedIDs []string, includeFileData bool) (*InventoryData, error) {
	for _, id := range selectedIDs {
		commodity, err := s.registrySet.CommodityRegistry.Get(ctx, id)
		if err != nil {
			// Skip items that don't exist
			continue
		}
		xmlCommodity, err := s.convertCommodityToXML(ctx, commodity, includeFileData)
		if err != nil {
			return nil, fmt.Errorf("failed to convert commodity to XML: %w", err)
		}
		data.Commodities = append(data.Commodities, xmlCommodity)
	}
	return data, nil
}

func (s *ExportService) convertCommodityToXML(ctx context.Context, commodity *models.Commodity, includeFileData bool) (*Commodity, error) {
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
	if commodity.URLs != nil {
		for _, url := range commodity.URLs {
			if url != nil {
				xmlCommodity.URLs = append(xmlCommodity.URLs, &URL{
					Name:  "", // URL model doesn't have a Name field
					Value: url.String(),
				})
			}
		}
	}

	// TODO: Add file handling when file data is needed
	// This would require implementing file retrieval from the upload location

	return xmlCommodity, nil
}