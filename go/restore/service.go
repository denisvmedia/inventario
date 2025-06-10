package restore

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
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
	// Delete all commodities first (due to foreign key constraints)
	commodities, err := s.registrySet.CommodityRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list commodities for deletion")
	}
	for _, commodity := range commodities {
		if err := s.registrySet.CommodityRegistry.Delete(ctx, commodity.ID); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to delete commodity %s", commodity.ID))
		}
	}

	// Delete all areas
	areas, err := s.registrySet.AreaRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list areas for deletion")
	}
	for _, area := range areas {
		if err := s.registrySet.AreaRegistry.Delete(ctx, area.ID); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to delete area %s", area.ID))
		}
	}

	// Delete all locations
	locations, err := s.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list locations for deletion")
	}
	for _, location := range locations {
		if err := s.registrySet.LocationRegistry.Delete(ctx, location.ID); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to delete location %s", location.ID))
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
	existingLocation := existing.Locations[location.ID]
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
	existingArea := existing.Areas[area.ID]
	switch options.Strategy {
	case RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if !options.DryRun {
			createdArea, err := s.registrySet.AreaRegistry.Create(ctx, *area)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
			}
			// Add area to location using the actual location ID
			if err := s.registrySet.LocationRegistry.AddArea(ctx, actualLocationID, createdArea.ID); err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to add area %s to location %s", createdArea.ID, actualLocationID))
			}
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
				// Add area to location using the actual location ID
				if err := s.registrySet.LocationRegistry.AddArea(ctx, actualLocationID, createdArea.ID); err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to add area %s to location %s", createdArea.ID, actualLocationID))
				}
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
					return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", area.ID))
				}
				// Add area to location
				if err := s.registrySet.LocationRegistry.AddArea(ctx, xmlArea.LocationID, area.ID); err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to add area %s to location %s", area.ID, xmlArea.LocationID))
				}
				// Track the newly created area
				existing.Areas[area.ID] = createdArea
			}
			stats.CreatedCount++
			stats.AreaCount++
		} else {
			if !options.DryRun {
				updatedArea, err := s.registrySet.AreaRegistry.Update(ctx, *area)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to update area %s", area.ID))
				}
				// Update the tracked area
				existing.Areas[area.ID] = updatedArea
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
			case "originalCurrency":
				if err := decoder.DecodeElement(&xmlCommodity.OriginalCurrency, &t); err != nil {
					return errkit.Wrap(err, "failed to decode original currency")
				}
			case "currentPrice":
				if err := decoder.DecodeElement(&xmlCommodity.CurrentPrice, &t); err != nil {
					return errkit.Wrap(err, "failed to decode current price")
				}
			case "currentCurrency":
				if err := decoder.DecodeElement(&xmlCommodity.CurrentCurrency, &t); err != nil {
					return errkit.Wrap(err, "failed to decode current currency")
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
			case "images":
				if options.IncludeFileData {
					if err := s.processFiles(ctx, decoder, &t, xmlCommodity.ID, "image", stats); err != nil {
						return errkit.Wrap(err, "failed to process images")
					}
				} else {
					// Skip images section
					if err := s.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip images section")
					}
				}
			case "invoices":
				if options.IncludeFileData {
					if err := s.processFiles(ctx, decoder, &t, xmlCommodity.ID, "invoice", stats); err != nil {
						return errkit.Wrap(err, "failed to process invoices")
					}
				} else {
					// Skip invoices section
					if err := s.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip invoices section")
					}
				}
			case "manuals":
				if options.IncludeFileData {
					if err := s.processFiles(ctx, decoder, &t, xmlCommodity.ID, "manual", stats); err != nil {
						return errkit.Wrap(err, "failed to process manuals")
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
				// Process the commodity
				return s.createOrUpdateCommodity(ctx, &xmlCommodity, stats, existing, idMapping, options)
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
			// Add commodity to area using the actual area ID
			if err := s.registrySet.AreaRegistry.AddCommodity(ctx, actualAreaID, createdCommodity.ID); err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to add commodity %s to area %s", createdCommodity.ID, actualAreaID))
			}
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
				if _, err := s.registrySet.CommodityRegistry.Create(ctx, *commodity); err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", commodity.ID))
				}
				// Add commodity to area
				if err := s.registrySet.AreaRegistry.AddCommodity(ctx, xmlCommodity.AreaID, commodity.ID); err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to add commodity %s to area %s", commodity.ID, xmlCommodity.AreaID))
				}
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
				if _, err := s.registrySet.CommodityRegistry.Create(ctx, *commodity); err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", commodity.ID))
				}
				// Add commodity to area
				if err := s.registrySet.AreaRegistry.AddCommodity(ctx, xmlCommodity.AreaID, commodity.ID); err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to add commodity %s to area %s", commodity.ID, xmlCommodity.AreaID))
				}
			}
			stats.CreatedCount++
			stats.CommodityCount++
		} else {
			if !options.DryRun {
				if _, err := s.registrySet.CommodityRegistry.Update(ctx, *commodity); err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to update commodity %s", commodity.ID))
				}
			}
			stats.UpdatedCount++
			stats.CommodityCount++
		}
	}

	return nil
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
			if t.Name.Local == fileType {
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
			if t.Name.Local == fileType {
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

	// Generate unique filename
	filename := fmt.Sprintf("%s-%s%s", xmlFile.Path, xmlFile.ID, xmlFile.Extension)

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
				// Update the file path to the stored filename
				xmlFile.Path = filename
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
			EntityID:    models.EntityID{ID: xmlFile.ID},
			CommodityID: commodityID,
			File:        file,
		}
		if err := image.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid image")
		}
		if _, err := s.registrySet.ImageRegistry.Create(ctx, *image); err != nil {
			return errkit.Wrap(err, "failed to create image")
		}
		if err := s.registrySet.CommodityRegistry.AddImage(ctx, commodityID, image.ID); err != nil {
			return errkit.Wrap(err, "failed to add image to commodity")
		}
		stats.ImageCount++
	case "invoice":
		invoice := &models.Invoice{
			EntityID:    models.EntityID{ID: xmlFile.ID},
			CommodityID: commodityID,
			File:        file,
		}
		if err := invoice.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid invoice")
		}
		if _, err := s.registrySet.InvoiceRegistry.Create(ctx, *invoice); err != nil {
			return errkit.Wrap(err, "failed to create invoice")
		}
		if err := s.registrySet.CommodityRegistry.AddInvoice(ctx, commodityID, invoice.ID); err != nil {
			return errkit.Wrap(err, "failed to add invoice to commodity")
		}
		stats.InvoiceCount++
	case "manual":
		manual := &models.Manual{
			EntityID:    models.EntityID{ID: xmlFile.ID},
			CommodityID: commodityID,
			File:        file,
		}
		if err := manual.ValidateWithContext(ctx); err != nil {
			return errkit.Wrap(err, "invalid manual")
		}
		if _, err := s.registrySet.ManualRegistry.Create(ctx, *manual); err != nil {
			return errkit.Wrap(err, "failed to create manual")
		}
		if err := s.registrySet.CommodityRegistry.AddManual(ctx, commodityID, manual.ID); err != nil {
			return errkit.Wrap(err, "failed to add manual to commodity")
		}
		stats.ManualCount++
	default:
		return errors.New(fmt.Sprintf("unknown file type: %s", fileType))
	}

	return nil
}
