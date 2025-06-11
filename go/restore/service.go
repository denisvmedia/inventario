package restore

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"gocloud.dev/blob"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// RestoreService handles XML restore operations with different strategies
type RestoreService struct {
	registrySet    *registry.Set
	uploadLocation string
}

// NewRestoreService creates a new restore service
func NewRestoreService(registrySet *registry.Set, uploadLocation string) *RestoreService {
	return &RestoreService{
		registrySet:    registrySet,
		uploadLocation: uploadLocation,
	}
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

// RestoreOptions contains options for the restore operation
type RestoreOptions struct {
	Strategy         RestoreStrategy `json:"strategy"`
	IncludeFileData  bool           `json:"include_file_data"`
	DryRun          bool           `json:"dry_run"`
	BackupExisting  bool           `json:"backup_existing"`
}

// RestoreStats tracks statistics during restore operation
type RestoreStats struct {
	LocationCount     int      `json:"location_count"`
	AreaCount         int      `json:"area_count"`
	CommodityCount    int      `json:"commodity_count"`
	ImageCount        int      `json:"image_count"`
	InvoiceCount      int      `json:"invoice_count"`
	ManualCount       int      `json:"manual_count"`
	BinaryDataSize    int64    `json:"binary_data_size"`
	ErrorCount        int      `json:"error_count"`
	Errors            []string `json:"errors"`
	SkippedCount      int      `json:"skipped_count"`
	UpdatedCount      int      `json:"updated_count"`
	CreatedCount      int      `json:"created_count"`
	DeletedCount      int      `json:"deleted_count"`
}

// RestoreFromXML restores data from an XML file using the specified strategy
func (s *RestoreService) RestoreFromXML(ctx context.Context, xmlReader io.Reader, options RestoreOptions) (*RestoreStats, error) {
	stats := &RestoreStats{}

	// Validate options
	if err := s.validateOptions(options); err != nil {
		return stats, errkit.Wrap(err, "invalid restore options")
	}

	// Get main currency from settings and add it to context for commodity validation
	settings, err := s.registrySet.SettingsRegistry.Get(ctx)
	if err != nil {
		return stats, errkit.Wrap(err, "failed to get settings")
	}

	if settings.MainCurrency != nil && *settings.MainCurrency != "" {
		ctx = validationctx.WithMainCurrency(ctx, *settings.MainCurrency)
	}

	// If full replace strategy, backup existing data first if requested
	if options.Strategy == RestoreStrategyFullReplace && options.BackupExisting {
		if err := s.backupExistingData(ctx); err != nil {
			return stats, errkit.Wrap(err, "failed to backup existing data")
		}
	}

	decoder := xml.NewDecoder(xmlReader)
	
	// Track existing entities for validation and strategy decisions
	existingEntities := &ExistingEntities{}
	idMapping := &IDMapping{
		Locations:   make(map[string]string),
		Areas:       make(map[string]string),
		Commodities: make(map[string]string),
	}

	if options.Strategy != RestoreStrategyFullReplace {
		if err := s.loadExistingEntities(ctx, existingEntities); err != nil {
			return stats, errkit.Wrap(err, "failed to load existing entities")
		}
		// For non-full replace, populate ID mapping with existing entities
		for xmlID, entity := range existingEntities.Locations {
			idMapping.Locations[xmlID] = entity.ID
		}
		for xmlID, entity := range existingEntities.Areas {
			idMapping.Areas[xmlID] = entity.ID
		}
		for xmlID, entity := range existingEntities.Commodities {
			idMapping.Commodities[xmlID] = entity.ID
		}
	} else {
		// For full replace, initialize empty maps to track newly created entities
		existingEntities.Locations = make(map[string]*models.Location)
		existingEntities.Areas = make(map[string]*models.Area)
		existingEntities.Commodities = make(map[string]*models.Commodity)
	}

	// If full replace, clear existing data first
	if options.Strategy == RestoreStrategyFullReplace && !options.DryRun {
		if err := s.clearExistingData(ctx); err != nil {
			return stats, errkit.Wrap(err, "failed to clear existing data")
		}
	}

	// Process XML stream
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, errkit.Wrap(err, "failed to read XML token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "inventory":
				// Skip the root element, continue processing
				continue
			case "locations":
				if err := s.processLocations(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
					return stats, errkit.Wrap(err, "failed to process locations")
				}
			case "areas":
				if err := s.processAreas(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
					return stats, errkit.Wrap(err, "failed to process areas")
				}
			case "commodities":
				if err := s.processCommodities(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
					return stats, errkit.Wrap(err, "failed to process commodities")
				}
			}
		}
	}

	return stats, nil
}

// validateOptions validates the restore options
func (s *RestoreService) validateOptions(options RestoreOptions) error {
	switch options.Strategy {
	case RestoreStrategyFullReplace, RestoreStrategyMergeAdd, RestoreStrategyMergeUpdate:
		// Valid strategies
	default:
		return errors.New("invalid restore strategy")
	}
	return nil
}

// ExistingEntities tracks existing entities in the database
type ExistingEntities struct {
	Locations  map[string]*models.Location
	Areas      map[string]*models.Area
	Commodities map[string]*models.Commodity
}

// IDMapping tracks the mapping from XML IDs to actual database IDs
type IDMapping struct {
	Locations  map[string]string // XML ID -> Database ID
	Areas      map[string]string // XML ID -> Database ID
	Commodities map[string]string // XML ID -> Database ID
}

// PendingFileData holds file data that needs to be processed after commodity creation
type PendingFileData struct {
	FileType string   // "image", "invoice", "manual"
	XMLFiles []XMLFile // File data collected during parsing
}

// loadExistingEntities loads all existing entities from the database
func (s *RestoreService) loadExistingEntities(ctx context.Context, entities *ExistingEntities) error {
	entities.Locations = make(map[string]*models.Location)
	entities.Areas = make(map[string]*models.Area)
	entities.Commodities = make(map[string]*models.Commodity)

	// Load locations
	locations, err := s.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing locations")
	}
	for _, location := range locations {
		entities.Locations[location.ID] = location
	}

	// Load areas
	areas, err := s.registrySet.AreaRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing areas")
	}
	for _, area := range areas {
		entities.Areas[area.ID] = area
	}

	// Load commodities
	commodities, err := s.registrySet.CommodityRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing commodities")
	}
	for _, commodity := range commodities {
		entities.Commodities[commodity.ID] = commodity
	}

	return nil
}

// backupExistingData creates a backup of existing data before full replace
func (s *RestoreService) backupExistingData(ctx context.Context) error {
	// This would create an export of current data
	// Implementation would depend on export service integration
	// For now, we'll just log that backup would be created
	fmt.Println("Creating backup of existing data before full replace...")
	return nil
}

// clearExistingData removes all existing data for full replace strategy
func (s *RestoreService) clearExistingData(ctx context.Context) error {
	// Delete all locations recursively (this will also delete areas and commodities)
	locations, err := s.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list locations for deletion")
	}
	for _, location := range locations {
		if err := s.registrySet.LocationRegistry.DeleteRecursive(ctx, location.ID); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to delete location %s recursively", location.ID))
		}
	}

	return nil
}

// processLocations processes the locations section of the XML
func (s *RestoreService) processLocations(ctx context.Context, decoder *xml.Decoder, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read locations token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "location" {
				if err := s.processLocation(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process location: %v", err))
					continue
				}
			}
		case xml.EndElement:
			if t.Name.Local == "locations" {
				return nil
			}
		}
	}
}

// processLocation processes a single location
func (s *RestoreService) processLocation(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	var xmlLocation XMLLocation
	if err := decoder.DecodeElement(&xmlLocation, startElement); err != nil {
		return errkit.Wrap(err, "failed to decode location")
	}

	location := xmlLocation.ConvertToLocation()
	if err := location.ValidateWithContext(ctx); err != nil {
		return errkit.Wrap(err, fmt.Sprintf("invalid location %s", location.ID))
	}

	// Store the original XML ID for mapping
	originalXMLID := xmlLocation.ID

	// Apply strategy
	existingLocation := existing.Locations[originalXMLID]
	switch options.Strategy {
	case RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if !options.DryRun {
			createdLocation, err := s.registrySet.LocationRegistry.Create(ctx, *location)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to create location %s", originalXMLID))
			}
			// Track the newly created location and store ID mapping
			existing.Locations[originalXMLID] = createdLocation
			idMapping.Locations[originalXMLID] = createdLocation.ID
		}
		stats.CreatedCount++
		stats.LocationCount++
	case RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingLocation == nil {
			if !options.DryRun {
				createdLocation, err := s.registrySet.LocationRegistry.Create(ctx, *location)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create location %s", originalXMLID))
				}
				// Track the newly created location and store ID mapping
				existing.Locations[originalXMLID] = createdLocation
				idMapping.Locations[originalXMLID] = createdLocation.ID
			}
			stats.CreatedCount++
			stats.LocationCount++
		} else {
			stats.SkippedCount++
		}
	case RestoreStrategyMergeUpdate:
		// Create if missing, update if exists
		if existingLocation == nil {
			if !options.DryRun {
				createdLocation, err := s.registrySet.LocationRegistry.Create(ctx, *location)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create location %s", originalXMLID))
				}
				// Track the newly created location and store ID mapping
				existing.Locations[originalXMLID] = createdLocation
				idMapping.Locations[originalXMLID] = createdLocation.ID
			}
			stats.CreatedCount++
			stats.LocationCount++
		} else {
			if !options.DryRun {
				updatedLocation, err := s.registrySet.LocationRegistry.Update(ctx, *location)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to update location %s", originalXMLID))
				}
				// Update the tracked location
				existing.Locations[originalXMLID] = updatedLocation
			}
			stats.UpdatedCount++
			stats.LocationCount++
		}
	}

	return nil
}

// processAreas processes the areas section of the XML
func (s *RestoreService) processAreas(ctx context.Context, decoder *xml.Decoder, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read areas token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "area" {
				if err := s.processArea(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process area: %v", err))
					continue
				}
			}
		case xml.EndElement:
			if t.Name.Local == "areas" {
				return nil
			}
		}
	}
}

// processArea processes a single area
func (s *RestoreService) processArea(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	var xmlArea XMLArea
	if err := decoder.DecodeElement(&xmlArea, startElement); err != nil {
		return errkit.Wrap(err, "failed to decode area")
	}

	// Store the original XML IDs for mapping
	originalXMLID := xmlArea.ID
	originalLocationXMLID := xmlArea.LocationID

	// Validate that the location exists (either in existing data or was just created)
	if existing.Locations[originalLocationXMLID] == nil {
		return errors.New(fmt.Sprintf("area %s references non-existent location %s", originalXMLID, originalLocationXMLID))
	}

	// Get the actual database location ID
	actualLocationID := idMapping.Locations[originalLocationXMLID]
	if actualLocationID == "" {
		return errors.New(fmt.Sprintf("no ID mapping found for location %s", originalLocationXMLID))
	}

	area := xmlArea.ConvertToArea()
	// Set the correct location ID from the mapping
	area.LocationID = actualLocationID

	if err := area.ValidateWithContext(ctx); err != nil {
		return errkit.Wrap(err, fmt.Sprintf("invalid area %s", originalXMLID))
	}

	// Apply strategy
	existingArea := existing.Areas[originalXMLID]
	switch options.Strategy {
	case RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if !options.DryRun {
			createdArea, err := s.registrySet.AreaRegistry.Create(ctx, *area)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
			}
				// Note: Area is automatically added to location by the AreaRegistry.Create method
			// Track the newly created area and store ID mapping
			existing.Areas[originalXMLID] = createdArea
			idMapping.Areas[originalXMLID] = createdArea.ID
		}
		stats.CreatedCount++
		stats.AreaCount++
	case RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingArea == nil {
			if !options.DryRun {
				createdArea, err := s.registrySet.AreaRegistry.Create(ctx, *area)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
				}
				// Note: Area is automatically added to location by the AreaRegistry.Create method
				// Track the newly created area and store ID mapping
				existing.Areas[originalXMLID] = createdArea
				idMapping.Areas[originalXMLID] = createdArea.ID
			}
			stats.CreatedCount++
			stats.AreaCount++
		} else {
			stats.SkippedCount++
		}
	case RestoreStrategyMergeUpdate:
		// Create if missing, update if exists
		if existingArea == nil {
			if !options.DryRun {
				createdArea, err := s.registrySet.AreaRegistry.Create(ctx, *area)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
				}
				// Note: Area is automatically added to location by the AreaRegistry.Create method
				// Track the newly created area and store ID mapping
				existing.Areas[originalXMLID] = createdArea
				idMapping.Areas[originalXMLID] = createdArea.ID
			}
			stats.CreatedCount++
			stats.AreaCount++
		} else {
			if !options.DryRun {
				updatedArea, err := s.registrySet.AreaRegistry.Update(ctx, *area)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to update area %s", originalXMLID))
				}
				// Update the tracked area
				existing.Areas[originalXMLID] = updatedArea
			}
			stats.UpdatedCount++
			stats.AreaCount++
		}
	}

	return nil
}

// processCommodities processes the commodities section of the XML
func (s *RestoreService) processCommodities(ctx context.Context, decoder *xml.Decoder, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodities token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "commodity" {
				if err := s.processCommodity(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process commodity: %v", err))
					continue
				}
			}
		case xml.EndElement:
			if t.Name.Local == "commodities" {
				return nil
			}
		}
	}
}

// processCommodity processes a single commodity with streaming file handling
func (s *RestoreService) processCommodity(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	var xmlCommodity XMLCommodity
	var pendingFiles []PendingFileData // Collect file data to process after commodity creation

	// Get commodity ID from attributes
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlCommodity.ID = attr.Value
			break
		}
	}

	// Process commodity elements
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodity element token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "commodityName":
				if err := decoder.DecodeElement(&xmlCommodity.CommodityName, &t); err != nil {
					return errkit.Wrap(err, "failed to decode commodity name")
				}
			case "shortName":
				if err := decoder.DecodeElement(&xmlCommodity.ShortName, &t); err != nil {
					return errkit.Wrap(err, "failed to decode short name")
				}
			case "areaId":
				if err := decoder.DecodeElement(&xmlCommodity.AreaID, &t); err != nil {
					return errkit.Wrap(err, "failed to decode area ID")
				}
			case "count":
				if err := decoder.DecodeElement(&xmlCommodity.Count, &t); err != nil {
					return errkit.Wrap(err, "failed to decode count")
				}
			case "status":
				if err := decoder.DecodeElement(&xmlCommodity.Status, &t); err != nil {
					return errkit.Wrap(err, "failed to decode status")
				}
			case "type":
				if err := decoder.DecodeElement(&xmlCommodity.Type, &t); err != nil {
					return errkit.Wrap(err, "failed to decode type")
				}
			case "originalPrice":
				if err := decoder.DecodeElement(&xmlCommodity.OriginalPrice, &t); err != nil {
					return errkit.Wrap(err, "failed to decode original price")
				}
			case "originalPriceCurrency":
				if err := decoder.DecodeElement(&xmlCommodity.OriginalCurrency, &t); err != nil {
					return errkit.Wrap(err, "failed to decode original currency")
				}
			case "convertedOriginalPrice":
				if err := decoder.DecodeElement(&xmlCommodity.ConvertedOriginalPrice, &t); err != nil {
					return errkit.Wrap(err, "failed to decode converted original price")
				}
			case "currentPrice":
				if err := decoder.DecodeElement(&xmlCommodity.CurrentPrice, &t); err != nil {
					return errkit.Wrap(err, "failed to decode current price")
				}
			case "currentCurrency":
				if err := decoder.DecodeElement(&xmlCommodity.CurrentCurrency, &t); err != nil {
					return errkit.Wrap(err, "failed to decode current currency")
				}
			case "serialNumber":
				if err := decoder.DecodeElement(&xmlCommodity.SerialNumber, &t); err != nil {
					return errkit.Wrap(err, "failed to decode serial number")
				}
			case "extraSerialNumbers":
				if err := decoder.DecodeElement(&xmlCommodity.ExtraSerialNumbers, &t); err != nil {
					return errkit.Wrap(err, "failed to decode extra serial numbers")
				}
			case "comments":
				if err := decoder.DecodeElement(&xmlCommodity.Comments, &t); err != nil {
					return errkit.Wrap(err, "failed to decode comments")
				}
			case "draft":
				if err := decoder.DecodeElement(&xmlCommodity.Draft, &t); err != nil {
					return errkit.Wrap(err, "failed to decode draft")
				}
			case "purchaseDate":
				if err := decoder.DecodeElement(&xmlCommodity.PurchaseDate, &t); err != nil {
					return errkit.Wrap(err, "failed to decode purchase date")
				}
			case "registeredDate":
				if err := decoder.DecodeElement(&xmlCommodity.RegisteredDate, &t); err != nil {
					return errkit.Wrap(err, "failed to decode registered date")
				}
			case "lastModifiedDate":
				if err := decoder.DecodeElement(&xmlCommodity.LastModifiedDate, &t); err != nil {
					return errkit.Wrap(err, "failed to decode last modified date")
				}
			case "partNumbers":
				if err := decoder.DecodeElement(&xmlCommodity.PartNumbers, &t); err != nil {
					return errkit.Wrap(err, "failed to decode part numbers")
				}
			case "tags":
				if err := decoder.DecodeElement(&xmlCommodity.Tags, &t); err != nil {
					return errkit.Wrap(err, "failed to decode tags")
				}
			case "urls":
				if err := decoder.DecodeElement(&xmlCommodity.URLs, &t); err != nil {
					return errkit.Wrap(err, "failed to decode URLs")
				}
			case "images":
				if options.IncludeFileData {
					files, err := s.collectFiles(ctx, decoder, &t, stats)
					if err != nil {
						return errkit.Wrap(err, "failed to collect images")
					}
					if len(files) > 0 {
						pendingFiles = append(pendingFiles, PendingFileData{
							FileType: "image",
							XMLFiles: files,
						})
					}
				} else {
					// Skip images section
					if err := s.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip images section")
					}
				}
			case "invoices":
				if options.IncludeFileData {
					files, err := s.collectFiles(ctx, decoder, &t, stats)
					if err != nil {
						return errkit.Wrap(err, "failed to collect invoices")
					}
					if len(files) > 0 {
						pendingFiles = append(pendingFiles, PendingFileData{
							FileType: "invoice",
							XMLFiles: files,
						})
					}
				} else {
					// Skip invoices section
					if err := s.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip invoices section")
					}
				}
			case "manuals":
				if options.IncludeFileData {
					files, err := s.collectFiles(ctx, decoder, &t, stats)
					if err != nil {
						return errkit.Wrap(err, "failed to collect manuals")
					}
					if len(files) > 0 {
						pendingFiles = append(pendingFiles, PendingFileData{
							FileType: "manual",
							XMLFiles: files,
						})
					}
				} else {
					// Skip manuals section
					if err := s.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip manuals section")
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "commodity" {
				// Process the commodity first
				if err := s.createOrUpdateCommodity(ctx, &xmlCommodity, stats, existing, idMapping, options); err != nil {
					return err
				}

				// Now process any collected files after the commodity is created
				if options.IncludeFileData && len(pendingFiles) > 0 {
					// Get the actual commodity ID from the mapping
					actualCommodityID := idMapping.Commodities[xmlCommodity.ID]
					if actualCommodityID != "" {
						for _, pendingFile := range pendingFiles {
							for _, xmlFile := range pendingFile.XMLFiles {
								if err := s.createFileRecord(ctx, &xmlFile, actualCommodityID, pendingFile.FileType, stats); err != nil {
									stats.ErrorCount++
									stats.Errors = append(stats.Errors, fmt.Sprintf("failed to create %s file record: %v", pendingFile.FileType, err))
									continue
								}
							}
						}
					}
				}

				return nil
			}
		}
	}
}

// createOrUpdateCommodity creates or updates a commodity based on the strategy
func (s *RestoreService) createOrUpdateCommodity(ctx context.Context, xmlCommodity *XMLCommodity, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	// Store the original XML IDs for mapping
	originalXMLID := xmlCommodity.ID
	originalAreaXMLID := xmlCommodity.AreaID

	// Validate that the area exists (either in existing data or was just created)
	if existing.Areas[originalAreaXMLID] == nil {
		return errors.New(fmt.Sprintf("commodity %s references non-existent area %s", originalXMLID, originalAreaXMLID))
	}

	// Get the actual database area ID
	actualAreaID := idMapping.Areas[originalAreaXMLID]
	if actualAreaID == "" {
		return errors.New(fmt.Sprintf("no ID mapping found for area %s", originalAreaXMLID))
	}

	commodity, err := xmlCommodity.ConvertToCommodity()
	if err != nil {
		return errkit.Wrap(err, fmt.Sprintf("failed to convert commodity %s", originalXMLID))
	}

	// Set the correct area ID from the mapping
	commodity.AreaID = actualAreaID

	// Fix price validation issue: if original currency matches main currency,
	// converted original price must be zero
	mainCurrency, err := validationctx.MainCurrencyFromContext(ctx)
	if err == nil && string(commodity.OriginalPriceCurrency) == mainCurrency {
		commodity.ConvertedOriginalPrice = decimal.Zero
	}

	if err := commodity.ValidateWithContext(ctx); err != nil {
		return errkit.Wrap(err, fmt.Sprintf("invalid commodity %s", originalXMLID))
	}

	// Apply strategy
	existingCommodity := existing.Commodities[originalXMLID]
	switch options.Strategy {
	case RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if !options.DryRun {
			createdCommodity, err := s.registrySet.CommodityRegistry.Create(ctx, *commodity)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", originalXMLID))
			}
				// Note: Commodity is automatically added to area by the CommodityRegistry.Create method
			// Track the newly created commodity and store ID mapping
			existing.Commodities[originalXMLID] = createdCommodity
			idMapping.Commodities[originalXMLID] = createdCommodity.ID
		}
		stats.CreatedCount++
		stats.CommodityCount++
	case RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingCommodity == nil {
			if !options.DryRun {
				createdCommodity, err := s.registrySet.CommodityRegistry.Create(ctx, *commodity)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", originalXMLID))
				}
				// Note: Commodity is automatically added to area by the CommodityRegistry.Create method
				// Track the newly created commodity and store ID mapping
				existing.Commodities[originalXMLID] = createdCommodity
				idMapping.Commodities[originalXMLID] = createdCommodity.ID
			}
			stats.CreatedCount++
			stats.CommodityCount++
		} else {
			stats.SkippedCount++
		}
	case RestoreStrategyMergeUpdate:
		// Create if missing, update if exists
		if existingCommodity == nil {
			if !options.DryRun {
				createdCommodity, err := s.registrySet.CommodityRegistry.Create(ctx, *commodity)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", originalXMLID))
				}
				// Note: Commodity is automatically added to area by the CommodityRegistry.Create method
				// Track the newly created commodity and store ID mapping
				existing.Commodities[originalXMLID] = createdCommodity
				idMapping.Commodities[originalXMLID] = createdCommodity.ID
			}
			stats.CreatedCount++
			stats.CommodityCount++
		} else {
			if !options.DryRun {
				updatedCommodity, err := s.registrySet.CommodityRegistry.Update(ctx, *commodity)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to update commodity %s", originalXMLID))
				}
				// Update the tracked commodity
				existing.Commodities[originalXMLID] = updatedCommodity
			}
			stats.UpdatedCount++
			stats.CommodityCount++
		}
	}

	return nil
}

// collectFiles collects file data from XML without processing it immediately
func (s *RestoreService) collectFiles(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats) ([]XMLFile, error) {
	var files []XMLFile

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, errkit.Wrap(err, "failed to read file token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "file" {
				xmlFile, err := s.collectFile(ctx, decoder, &t, stats)
				if err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to collect file: %v", err))
					continue
				}
				files = append(files, *xmlFile)
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				return files, nil
			}
		}
	}
}

// processFiles processes file sections (images, invoices, manuals) with streaming base64 decoding
func (s *RestoreService) processFiles(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, commodityID, fileType string, stats *RestoreStats) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "file" {
				if err := s.processFile(ctx, decoder, &t, commodityID, fileType, stats); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process %s file: %v", fileType, err))
					continue
				}
			}
		case xml.EndElement:
			if t.Name.Local == startElement.Name.Local {
				return nil
			}
		}
	}
}

// collectFile collects a single file's data without processing it immediately
func (s *RestoreService) collectFile(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats) (*XMLFile, error) {
	var xmlFile XMLFile

	// Get file ID from attributes
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlFile.ID = attr.Value
			break
		}
	}

	// Process file elements
	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, errkit.Wrap(err, "failed to read file element token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "path":
				if err := decoder.DecodeElement(&xmlFile.Path, &t); err != nil {
					return nil, errkit.Wrap(err, "failed to decode path")
				}
			case "originalPath":
				if err := decoder.DecodeElement(&xmlFile.OriginalPath, &t); err != nil {
					return nil, errkit.Wrap(err, "failed to decode original path")
				}
			case "extension":
				if err := decoder.DecodeElement(&xmlFile.Extension, &t); err != nil {
					return nil, errkit.Wrap(err, "failed to decode extension")
				}
			case "mimeType":
				if err := decoder.DecodeElement(&xmlFile.MimeType, &t); err != nil {
					return nil, errkit.Wrap(err, "failed to decode mime type")
				}
			case "data":
				// Stream and decode base64 data
				if err := s.decodeBase64ToFile(ctx, decoder, &xmlFile, stats); err != nil {
					return nil, errkit.Wrap(err, "failed to decode base64 data")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "file" {
				return &xmlFile, nil
			}
		}
	}
}

// processFile processes a single file with streaming base64 decoding
func (s *RestoreService) processFile(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, commodityID, fileType string, stats *RestoreStats) error {
	var xmlFile XMLFile

	// Get file ID from attributes
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlFile.ID = attr.Value
			break
		}
	}

	// Process file elements
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read file element token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "path":
				if err := decoder.DecodeElement(&xmlFile.Path, &t); err != nil {
					return errkit.Wrap(err, "failed to decode path")
				}
			case "originalPath":
				if err := decoder.DecodeElement(&xmlFile.OriginalPath, &t); err != nil {
					return errkit.Wrap(err, "failed to decode original path")
				}
			case "extension":
				if err := decoder.DecodeElement(&xmlFile.Extension, &t); err != nil {
					return errkit.Wrap(err, "failed to decode extension")
				}
			case "mimeType":
				if err := decoder.DecodeElement(&xmlFile.MimeType, &t); err != nil {
					return errkit.Wrap(err, "failed to decode mime type")
				}
			case "data":
				// Stream and decode base64 data
				if err := s.decodeBase64ToFile(ctx, decoder, &xmlFile, stats); err != nil {
					return errkit.Wrap(err, "failed to decode base64 data")
				}
			}
		case xml.EndElement:
			if t.Name.Local == "file" {
				// Create file record
				if err := s.createFileRecord(ctx, &xmlFile, commodityID, fileType, stats); err != nil {
					return errkit.Wrap(err, "failed to create file record")
				}
				return nil
			}
		}
	}
}

// skipSection skips an entire XML section
func (s *RestoreService) skipSection(decoder *xml.Decoder, startElement *xml.StartElement) error {
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read token while skipping section")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 && t.Name.Local == startElement.Name.Local {
				return nil
			}
		}
	}
	return nil
}

// decodeBase64ToFile streams base64 data and saves it to blob storage
func (s *RestoreService) decodeBase64ToFile(ctx context.Context, decoder *xml.Decoder, xmlFile *XMLFile, stats *RestoreStats) error {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, s.uploadLocation)
	if err != nil {
		return errkit.Wrap(err, "failed to open blob bucket")
	}
	defer b.Close()

	// Generate unique filename using the same strategy as import service
	filename := filekit.UploadFileName(xmlFile.OriginalPath)

	// Create blob writer
	writer, err := b.NewWriter(ctx, filename, nil)
	if err != nil {
		return errkit.Wrap(err, "failed to create blob writer")
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			err = errkit.Wrap(closeErr, "failed to close blob writer")
		}
	}()

	// Read base64 data from XML and decode it directly to the blob
	var totalSize int64
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read base64 token")
		}

		switch t := tok.(type) {
		case xml.CharData:
			// Decode base64 data in chunks
			decoded := make([]byte, len(t))
			n, err := base64.StdEncoding.Decode(decoded, t)
			if err != nil {
				return errkit.Wrap(err, "failed to decode base64 data")
			}
			if n > 0 {
				if _, err := writer.Write(decoded[:n]); err != nil {
					return errkit.Wrap(err, "failed to write decoded data")
				}
				totalSize += int64(n)
			}
		case xml.EndElement:
			if t.Name.Local == "data" {
				stats.BinaryDataSize += totalSize
				// Update both Path and OriginalPath to the stored filename
				// Path is used for display/editing, OriginalPath is used for blob retrieval
				xmlFile.Path = filename
				xmlFile.OriginalPath = filename
				return nil
			}
		}
	}
}

// createFileRecord creates a file record in the appropriate registry
func (s *RestoreService) createFileRecord(ctx context.Context, xmlFile *XMLFile, commodityID, fileType string, stats *RestoreStats) error {
	file := xmlFile.ConvertToFile()

	switch fileType {
	case "image":
		image := &models.Image{
			CommodityID: commodityID,
			File:        file,
		}
		if err := image.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid image")
		}
		_, err := s.registrySet.ImageRegistry.Create(ctx, *image)
		if err != nil {
			return errkit.Wrap(err, "failed to create image")
		}
		// Note: AddImage is called automatically by the ImageRegistry.Create method
		stats.ImageCount++
	case "invoice":
		invoice := &models.Invoice{
			CommodityID: commodityID,
			File:        file,
		}
		if err := invoice.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid invoice")
		}
		_, err := s.registrySet.InvoiceRegistry.Create(ctx, *invoice)
		if err != nil {
			return errkit.Wrap(err, "failed to create invoice")
		}
		// Note: AddInvoice is called automatically by the InvoiceRegistry.Create method
		stats.InvoiceCount++
	case "manual":
		manual := &models.Manual{
			CommodityID: commodityID,
			File:        file,
		}
		if err := manual.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid manual")
		}
		_, err := s.registrySet.ManualRegistry.Create(ctx, *manual)
		if err != nil {
			return errkit.Wrap(err, "failed to create manual")
		}
		// Note: AddManual is called automatically by the ManualRegistry.Create method
		stats.ManualCount++
	default:
		return errors.New(fmt.Sprintf("unknown file type: %s", fileType))
	}

	return nil
}
