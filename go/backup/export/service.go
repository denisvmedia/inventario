package export

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"slices"
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
	// Legacy commodity-scoped attachment sections (`<images>`, `<invoices>`,
	// `<manuals>`) were removed under #1421 along with the legacy SQL tables
	// they sourced. Including file attachments in new exports is tracked as
	// a follow-up — once it lands it will export from the unified `files`
	// surface, not from these per-bucket types.
}

type URL struct {
	XMLName xml.Name `xml:"url"`
	Name    string   `xml:"name,attr"`
	Value   string   `xml:",chardata"`
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

	user, err := s.factorySet.UserRegistry.Get(ctx, export.CreatedByUserID)
	if err != nil {
		return errxtrace.Wrap("failed to get user", err)
	}

	// The export worker drives ProcessExport from a background context, so
	// no request-time middleware has populated user/group context. Resolve
	// the export's group now and inject both into ctx — the downstream
	// registry factories and createExportFileEntity read them from there.
	group, err := s.factorySet.LocationGroupRegistry.Get(ctx, export.GroupID)
	if err != nil {
		return errxtrace.Wrap("failed to get export group", err)
	}

	ctx = appctx.WithUser(ctx, user)
	ctx = appctx.WithGroup(ctx, group)

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

	// FileEntity is group-scoped (group_id NOT NULL + FK on PostgreSQL),
	// so the export's group must be on the context — exports themselves
	// are always created inside a group-scoped request.
	groupID := appctx.GroupIDFromContext(ctx)
	if groupID == "" {
		return nil, errors.New("group context is required but not found")
	}

	// Create file entity
	now := time.Now()
	fileEntity := models.FileEntity{
		TenantGroupAwareEntityID: models.TenantGroupAwareEntityID{
			TenantID:        tenantID,
			GroupID:         groupID,
			CreatedByUserID: userID,
		},
		Title:            fmt.Sprintf("Export: %s", description),
		Description:      fmt.Sprintf("Export file generated on %s", now.Format("2006-01-02 15:04:05")),
		Type:             models.FileTypeDocument,  // XML files are documents
		Category:         models.FileCategoryOther, // Export bundles aren't user-facing files; they live outside the four UI tiles
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

// ExportXML streams a single export to the provided writer and returns
// the corresponding statistics. Mirrors ProcessExport's per-export flow
// without the blob persistence + status updates — intended for tests
// (round-trip and table-driven) that want the XML bytes in hand without
// going through the on-disk bucket. Production code should still drive
// the worker via ProcessExport.
func ExportXML(s *ExportService, ctx context.Context, exp models.Export, writer io.Writer) (*types.ExportStats, error) {
	return s.streamXMLExport(ctx, exp, writer)
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

	user, err := s.factorySet.UserRegistry.Get(ctx, export.CreatedByUserID)
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
		locUUIDMap, err := s.buildLocationUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build location UUID map", err)
		}
		areaUUIDMap, err := s.buildAreaUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build area UUID map", err)
		}
		commodityUUIDMap, err := s.buildCommodityUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build commodity UUID map", err)
		}
		if err := s.streamLocations(ctx, writer, export, stats); err != nil {
			return nil, errxtrace.Wrap("failed to stream locations", err)
		}
		if err := s.streamFiles(ctx, writer, export, stats, locUUIDMap, areaUUIDMap, commodityUUIDMap, nil); err != nil {
			return nil, errxtrace.Wrap("failed to stream files", err)
		}
	case models.ExportTypeAreas:
		locUUIDMap, err := s.buildLocationUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build location UUID map", err)
		}
		areaUUIDMap, err := s.buildAreaUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build area UUID map", err)
		}
		commodityUUIDMap, err := s.buildCommodityUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build commodity UUID map", err)
		}
		if err := s.streamAreas(ctx, writer, export, stats, locUUIDMap); err != nil {
			return nil, errxtrace.Wrap("failed to stream areas", err)
		}
		if err := s.streamFiles(ctx, writer, export, stats, locUUIDMap, areaUUIDMap, commodityUUIDMap, nil); err != nil {
			return nil, errxtrace.Wrap("failed to stream files", err)
		}
	case models.ExportTypeCommodities:
		locUUIDMap, err := s.buildLocationUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build location UUID map", err)
		}
		areaUUIDMap, err := s.buildAreaUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build area UUID map", err)
		}
		commodityUUIDMap, err := s.buildCommodityUUIDMap(ctx)
		if err != nil {
			return nil, errxtrace.Wrap("failed to build commodity UUID map", err)
		}
		if err := s.streamCommodities(ctx, writer, export, stats, areaUUIDMap); err != nil {
			return nil, errxtrace.Wrap("failed to stream commodities", err)
		}
		if err := s.streamFiles(ctx, writer, export, stats, locUUIDMap, areaUUIDMap, commodityUUIDMap, nil); err != nil {
			return nil, errxtrace.Wrap("failed to stream files", err)
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
	// Build all three UUID maps once here so streamAreas, streamCommodities and
	// streamFiles can reuse them without issuing redundant List() calls.
	locUUIDMap, err := s.buildLocationUUIDMap(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to build location UUID map", err)
	}
	areaUUIDMap, err := s.buildAreaUUIDMap(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to build area UUID map", err)
	}
	commodityUUIDMap, err := s.buildCommodityUUIDMap(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to build commodity UUID map", err)
	}

	if err := s.streamLocations(ctx, writer, export, stats); err != nil {
		return errxtrace.Wrap("failed to stream locations", err)
	}
	if err := s.streamAreas(ctx, writer, export, stats, locUUIDMap); err != nil {
		return errxtrace.Wrap("failed to stream areas", err)
	}
	if err := s.streamCommodities(ctx, writer, export, stats, areaUUIDMap); err != nil {
		return errxtrace.Wrap("failed to stream commodities", err)
	}
	if err := s.streamFiles(ctx, writer, export, stats, locUUIDMap, areaUUIDMap, commodityUUIDMap, nil); err != nil {
		return errxtrace.Wrap("failed to stream files", err)
	}
	return nil
}

// buildLocationUUIDMap builds a map from location DB ID to immutable UUID.
// This is used during export to write stable UUIDs in XML rather than volatile DB IDs.
func (s *ExportService) buildLocationUUIDMap(ctx context.Context) (map[string]string, error) {
	locReg, err := s.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create location registry for UUID map", err)
	}
	locations, err := locReg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list locations for UUID map", err)
	}
	m := make(map[string]string, len(locations))
	for _, loc := range locations {
		m[loc.ID] = loc.UUID
	}
	return m, nil
}

// buildAreaUUIDMap builds a map from area DB ID to immutable UUID.
// This is used during export to write stable UUIDs in XML rather than volatile DB IDs.
func (s *ExportService) buildAreaUUIDMap(ctx context.Context) (map[string]string, error) {
	areaReg, err := s.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create area registry for UUID map", err)
	}
	areas, err := areaReg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list areas for UUID map", err)
	}
	m := make(map[string]string, len(areas))
	for _, area := range areas {
		m[area.ID] = area.UUID
	}
	return m, nil
}

// streamLocations streams locations to the writer and tracks statistics
func (s *ExportService) streamLocations(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats) error {
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
			ID:      loc.UUID, // Use immutable UUID as the stable XML identifier
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

// streamAreas streams areas to the writer and tracks statistics.
// locUUIDMap is a pre-built location DB ID → UUID map used to resolve FK references;
// it is built by the caller to avoid redundant List() round-trips.
func (s *ExportService) streamAreas(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats, locUUIDMap map[string]string) error {
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
			ID:         area.UUID, // Use immutable UUID as the stable XML identifier
			Name:       area.Name,
			LocationID: locUUIDMap[area.LocationID], // Resolve FK to location's immutable UUID
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

// streamCommodities streams commodities to the writer and tracks statistics.
// areaUUIDMap is a pre-built area DB ID → UUID map used to resolve FK references;
// it is built by the caller to avoid redundant List() round-trips.
func (s *ExportService) streamCommodities(ctx context.Context, writer io.Writer, export models.Export, stats *types.ExportStats, areaUUIDMap map[string]string) error {
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
		areaUUID := areaUUIDMap[commodity.AreaID] // Resolve area DB ID → immutable UUID
		// Single token-stream path. The previous `if export.IncludeFileData`
		// branch picked between this and `convertCommodityToXML`; both
		// produce identical output now that legacy attachment streaming is
		// gone (#1421), so the branch was misleading and was dropped.
		if err := s.streamCommodityDirectly(ctx, encoder, commodity, areaUUID); err != nil {
			return errxtrace.Wrap("failed to stream commodity", err)
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
	// Build all three UUID maps once so the entity streams and streamFiles
	// can reuse them without issuing redundant List() calls.
	locUUIDMap, err := s.buildLocationUUIDMap(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to build location UUID map", err)
	}
	areaUUIDMap, err := s.buildAreaUUIDMap(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to build area UUID map", err)
	}
	commodityUUIDMap, err := s.buildCommodityUUIDMap(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to build commodity UUID map", err)
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	// Group items by type for better organization
	locations, areas, commodities := s.groupSelectedItemsByType(export.SelectedItems)

	// Export each type of item with statistics tracking
	if err := s.streamSelectedLocations(ctx, encoder, locations, stats); err != nil {
		return err
	}
	if err := s.streamSelectedAreas(ctx, encoder, areas, stats, locUUIDMap); err != nil {
		return err
	}
	if err := s.streamSelectedCommodities(ctx, encoder, commodities, export, stats, areaUUIDMap); err != nil {
		return err
	}

	if err := encoder.Flush(); err != nil {
		return errxtrace.Wrap("failed to flush encoder", err)
	}

	// Files for selected_items are filtered to those linked to the selected
	// entities. Standalone files (LinkedEntityType="") are intentionally not
	// included — the user explicitly picked locations/areas/commodities and
	// wouldn't expect unrelated standalone uploads to ride along.
	scope := selectedFileScope{
		Locations:   locations,
		Areas:       areas,
		Commodities: commodities,
	}
	if err := s.streamFiles(ctx, writer, export, stats, locUUIDMap, areaUUIDMap, commodityUUIDMap, &scope); err != nil {
		return errxtrace.Wrap("failed to stream files", err)
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
			ID:      location.UUID, // Use immutable UUID as the stable XML identifier
			Name:    location.Name,
			Address: location.Address,
		}, nil
	})
}

// streamSelectedAreas streams area data to the XML encoder and tracks statistics.
// locUUIDMap is a pre-built location DB ID → UUID map used to resolve FK references;
// it is built by the caller to avoid redundant List() round-trips.
func (s *ExportService) streamSelectedAreas(ctx context.Context, encoder *xml.Encoder, areaIDs []string, stats *types.ExportStats, locUUIDMap map[string]string) error {
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
			ID:         area.UUID, // Use immutable UUID as the stable XML identifier
			Name:       area.Name,
			LocationID: locUUIDMap[area.LocationID], // Resolve FK to location's immutable UUID
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

// streamSelectedCommodities streams commodity data to the XML encoder and tracks statistics.
// areaUUIDMap is a pre-built area DB ID → UUID map used to resolve FK references;
// it is built by the caller to avoid redundant List() round-trips.
func (s *ExportService) streamSelectedCommodities(ctx context.Context, encoder *xml.Encoder, commodityIDs []string, export models.Export, stats *types.ExportStats, areaUUIDMap map[string]string) error {
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

		areaUUID := areaUUIDMap[commodity.AreaID] // Resolve area DB ID → immutable UUID
		// Single token-stream path. The previous `if export.IncludeFileData`
		// branch picked between this and `convertCommodityToXML`; both
		// produce identical output now that legacy attachment streaming is
		// gone (#1421), so the branch was misleading and was dropped.
		if err := s.streamCommodityDirectly(ctx, encoder, commodity, areaUUID); err != nil {
			return errxtrace.Wrap("failed to stream commodity", err)
		}
		stats.CommodityCount++
	}

	endElement := xml.EndElement{Name: xml.Name{Local: "commodities"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errxtrace.Wrap("failed to encode commodities end element", err)
	}

	return nil
}

// Legacy commodity-scoped attachment helpers (`addFileAttachments`,
// `addImages`, `addInvoices`, `addManuals`, `addFileCollection`) were removed
// under #1421 along with the `images`/`invoices`/`manuals` SQL tables they
// queried. The same applies to the streaming variants further down. Re-adding
// file export from the unified `files` surface is tracked separately.

// loadFileDataStreaming loads file data using a memory-efficient streaming approach and tracks base64 size

// encodeFileMetadata encodes file metadata elements (path, originalPath, extension, mimeType)

// addFileCollection / streamFileCollectionDirectly were removed under #1421
// alongside the legacy commodity-scoped image / invoice / manual registries
// they switched on. New file export from the unified `files` surface is a
// follow-up issue.

// encodeCommodityMetadata encodes commodity metadata elements.
// areaUUID is the immutable UUID of the referenced area (resolved by the caller).
func (s *ExportService) encodeCommodityMetadata(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, areaUUID string) error {
	if err := s.encodeBasicCommodityFields(encoder, commodity, areaUUID); err != nil {
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

// encodeBasicCommodityFields encodes basic commodity fields.
// areaUUID is the immutable UUID of the referenced area (resolved by the caller from the DB ID→UUID map).
func (s *ExportService) encodeBasicCommodityFields(encoder *xml.Encoder, commodity *models.Commodity, areaUUID string) error {
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
	// Write the area's immutable UUID so the FK reference is stable across exports/imports.
	if err := encodeTextElement("areaId", areaUUID); err != nil {
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

// streamCommodityDirectly streams a commodity to the XML encoder.
// areaUUID is the immutable UUID of the referenced area (resolved by the
// caller from the DB ID→UUID map). Legacy attachment streaming was removed
// under #1421 — re-introducing file export from `files` is a follow-up.
func (s *ExportService) streamCommodityDirectly(ctx context.Context, encoder *xml.Encoder, commodity *models.Commodity, areaUUID string) error {
	// Start commodity element using the immutable UUID as the stable XML identifier.
	commodityStart := xml.StartElement{
		Name: xml.Name{Local: "commodity"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "id"}, Value: commodity.UUID},
		},
	}
	if err := encoder.EncodeToken(commodityStart); err != nil {
		return errxtrace.Wrap("failed to encode commodity start element", err)
	}

	// Encode commodity metadata
	if err := s.encodeCommodityMetadata(ctx, encoder, commodity, areaUUID); err != nil {
		return errxtrace.Wrap("failed to encode commodity metadata", err)
	}

	// End commodity element
	commodityEnd := xml.EndElement{Name: xml.Name{Local: "commodity"}}
	if err := encoder.EncodeToken(commodityEnd); err != nil {
		return errxtrace.Wrap("failed to encode commodity end element", err)
	}

	return nil
}

// XMLFile mirrors the on-disk shape of a single <file> element inside the
// export's <files> section. The export side marshals from this struct;
// the restore side decodes into restore/types.XMLFile (kept separate so
// the restore package doesn't pull export-package transitively).
type XMLFile struct {
	XMLName          xml.Name `xml:"file"`
	ID               string   `xml:"id,attr"`
	LinkedEntityType string   `xml:"linkedEntityType,omitempty"`
	LinkedEntityID   string   `xml:"linkedEntityId,omitempty"`
	LinkedEntityMeta string   `xml:"linkedEntityMeta,omitempty"`
	Type             string   `xml:"type,omitempty"`
	Category         string   `xml:"category,omitempty"`
	Title            string   `xml:"title,omitempty"`
	Description      string   `xml:"description,omitempty"`
	Tags             []string `xml:"tags>tag,omitempty"`
	Path             string   `xml:"path"`
	OriginalPath     string   `xml:"originalPath"`
	Extension        string   `xml:"extension,omitempty"`
	MimeType         string   `xml:"mimeType,omitempty"`
	CreatedAt        string   `xml:"createdAt,omitempty"`
	UpdatedAt        string   `xml:"updatedAt,omitempty"`
	Data             string   `xml:"data,omitempty"` // Base64-encoded blob, only when IncludeFileData is true.
}

// selectedFileScope narrows the file export to a specific set of selected
// entities for ExportTypeSelectedItems. Files are included iff their
// linked_entity_type matches one of the categories below and their
// linked_entity_id is in the corresponding DB-ID list.
type selectedFileScope struct {
	Locations   []string // DB IDs
	Areas       []string // DB IDs
	Commodities []string // DB IDs
}

// buildCommodityUUIDMap builds a map from commodity DB ID to immutable UUID,
// used by streamFiles to resolve linked_entity_id (DB-id) → stable UUID for
// the XML emission. Mirrors the existing location/area UUID-map helpers.
func (s *ExportService) buildCommodityUUIDMap(ctx context.Context) (map[string]string, error) {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity registry for UUID map", err)
	}
	commodities, err := comReg.List(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodities for UUID map", err)
	}
	m := make(map[string]string, len(commodities))
	for _, c := range commodities {
		m[c.ID] = c.UUID
	}
	return m, nil
}

// streamFiles emits the <files> XML section. Reads from the unified `files`
// table (group-scoped via the user registry's RLS context) and writes one
// <file> element per row whose linked entity is in scope.
//
// Files linked to the export itself (linked_entity_type="export") are always
// excluded — those are the export's own backup-bundle blobs and re-emitting
// them in their own export would create a self-reference.
//
// When `scope` is nil all eligible files are emitted; when non-nil only files
// whose linked entity (commodity/location/area, by DB ID) matches the scope
// are included. Standalone files (LinkedEntityType="") are emitted only when
// scope is nil.
//
// When export.IncludeFileData is true, each file's blob content is read from
// the upload bucket, base64-encoded, and inlined as a <data> element. The
// blob payload is loaded into memory in full per file — that matches the
// pre-#1421 code path; chunked streaming is a future optimization.
func (s *ExportService) streamFiles(
	ctx context.Context,
	writer io.Writer,
	export models.Export,
	stats *types.ExportStats,
	locUUIDMap, areaUUIDMap, commodityUUIDMap map[string]string,
	scope *selectedFileScope,
) error {
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to get file registry", err)
	}
	files, err := fileReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list files", err)
	}

	encoder := xml.NewEncoder(writer)
	encoder.Indent("  ", "  ")

	startElement := xml.StartElement{Name: xml.Name{Local: "files"}}
	if err := encoder.EncodeToken(startElement); err != nil {
		return errxtrace.Wrap("failed to encode files start element", err)
	}

	// Open the bucket lazily — only when at least one file needs blob data.
	var bucket *blob.Bucket
	defer func() {
		if bucket != nil {
			_ = bucket.Close()
		}
	}()

	for _, file := range files {
		if file == nil || file.File == nil {
			continue
		}

		// Skip the export's own backup blobs to avoid self-reference.
		if file.LinkedEntityType == "export" {
			continue
		}

		// Apply the selected_items scope, if any. Standalone files (no
		// linked entity) are only included for full/whole-class exports.
		if scope != nil {
			if !inSelectedScope(file, scope) {
				continue
			}
		}

		linkedID, ok := resolveLinkedEntityUUID(file, locUUIDMap, areaUUIDMap, commodityUUIDMap)
		if !ok {
			// The linked entity wasn't visible to this user (RLS may have
			// hidden it) or its UUID couldn't be resolved. Skip rather
			// than emit a dangling reference.
			continue
		}

		xmlFile := XMLFile{
			ID:               file.UUID,
			LinkedEntityType: file.LinkedEntityType,
			LinkedEntityID:   linkedID,
			LinkedEntityMeta: file.LinkedEntityMeta,
			Type:             string(file.Type),
			Category:         string(file.Category),
			Title:            file.Title,
			Description:      file.Description,
			Tags:             []string(file.Tags),
			Path:             file.Path,
			OriginalPath:     file.OriginalPath,
			Extension:        file.Ext,
			MimeType:         file.MIMEType,
			CreatedAt:        file.CreatedAt.UTC().Format(time.RFC3339),
			UpdatedAt:        file.UpdatedAt.UTC().Format(time.RFC3339),
		}

		if export.IncludeFileData {
			if bucket == nil {
				bucket, err = blob.OpenBucket(ctx, s.uploadLocation)
				if err != nil {
					return errxtrace.Wrap("failed to open blob bucket for file data", err)
				}
			}
			data, size, err := s.readFileBase64(ctx, bucket, file.OriginalPath)
			if err != nil {
				// Missing blobs are common during dev (manual deletes,
				// orphan rows). Log on stats but don't fail the entire
				// export — surface the row metadata without <data>.
				stats.FileCount++
				if encErr := encoder.Encode(&xmlFile); encErr != nil {
					return errxtrace.Wrap("failed to encode file metadata", encErr)
				}
				continue
			}
			xmlFile.Data = data
			stats.BinaryDataSize += size
		}

		if err := encoder.Encode(&xmlFile); err != nil {
			return errxtrace.Wrap("failed to encode file", err)
		}
		stats.FileCount++
	}

	endElement := xml.EndElement{Name: xml.Name{Local: "files"}}
	if err := encoder.EncodeToken(endElement); err != nil {
		return errxtrace.Wrap("failed to encode files end element", err)
	}
	if err := encoder.Flush(); err != nil {
		return errxtrace.Wrap("failed to flush encoder", err)
	}
	return nil
}

// readFileBase64 reads a blob fully into memory and returns it base64-encoded
// alongside the raw byte size (for BinaryDataSize stats).
func (s *ExportService) readFileBase64(ctx context.Context, bucket *blob.Bucket, blobKey string) (string, int64, error) {
	reader, err := bucket.NewReader(ctx, blobKey, nil)
	if err != nil {
		return "", 0, errxtrace.Wrap("failed to open blob reader", err, errx.Attrs("blob_key", blobKey))
	}
	defer reader.Close()

	raw, err := io.ReadAll(reader)
	if err != nil {
		return "", 0, errxtrace.Wrap("failed to read blob content", err, errx.Attrs("blob_key", blobKey))
	}
	return base64.StdEncoding.EncodeToString(raw), int64(len(raw)), nil
}

// resolveLinkedEntityUUID maps a file's linked_entity_id (a DB primary key)
// to the corresponding immutable UUID using the per-type UUID maps. Returns
// (uuid, true) on success, ("", false) when the linked entity is unknown
// (pruned, deleted, or never existed). Standalone files (LinkedEntityType="")
// always return ("", true) since there's nothing to resolve.
func resolveLinkedEntityUUID(
	file *models.FileEntity,
	locUUIDMap, areaUUIDMap, commodityUUIDMap map[string]string,
) (string, bool) {
	if file.LinkedEntityType == "" || file.LinkedEntityID == "" {
		return "", true
	}
	switch file.LinkedEntityType {
	case "commodity":
		uuid, ok := commodityUUIDMap[file.LinkedEntityID]
		return uuid, ok
	case "location":
		uuid, ok := locUUIDMap[file.LinkedEntityID]
		return uuid, ok
	case "area":
		uuid, ok := areaUUIDMap[file.LinkedEntityID]
		return uuid, ok
	default:
		// Unknown linked-entity-type means we can't safely round-trip it.
		// Treat as standalone — but emit the type/meta so the restore
		// side can decide what to do (likely also skip).
		return "", true
	}
}

// inSelectedScope reports whether a file should be included given the
// selected_items scope. Only commodity/area/location-linked files are
// considered; standalone files are excluded from selected exports because
// the user picked specific entities.
func inSelectedScope(file *models.FileEntity, scope *selectedFileScope) bool {
	if file.LinkedEntityType == "" || file.LinkedEntityID == "" {
		return false
	}
	switch file.LinkedEntityType {
	case "commodity":
		return slices.Contains(scope.Commodities, file.LinkedEntityID)
	case "area":
		return slices.Contains(scope.Areas, file.LinkedEntityID)
	case "location":
		return slices.Contains(scope.Locations, file.LinkedEntityID)
	}
	return false
}
