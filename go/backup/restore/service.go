package restore

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"

	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/internal/log"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

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
	Strategy        RestoreStrategy `json:"strategy"`
	IncludeFileData bool            `json:"include_file_data"`
	DryRun          bool            `json:"dry_run"`
}

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

// ExistingEntities tracks existing entities in the database
type ExistingEntities struct {
	Locations   map[string]*models.Location
	Areas       map[string]*models.Area
	Commodities map[string]*models.Commodity
	Images      map[string]*models.Image   // XML ID -> Image
	Invoices    map[string]*models.Invoice // XML ID -> Invoice
	Manuals     map[string]*models.Manual  // XML ID -> Manual
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

// ProcessRestoreOperation processes a restore operation in the background with detailed logging
func (s *RestoreService) ProcessRestoreOperation(ctx context.Context, restoreOperationID, uploadLocation string) error {
	return NewRestoreOperationProcessor(restoreOperationID, s.registrySet, uploadLocation).Process(ctx)
}

// RestoreOperationProcessor wraps the restore service to provide detailed logging
type RestoreOperationProcessor struct {
	restoreOperationID string
	registrySet        *registry.Set
	uploadLocation     string
}

func NewRestoreOperationProcessor(restoreOperationID string, registrySet *registry.Set, uploadLocation string) *RestoreOperationProcessor {
	return &RestoreOperationProcessor{
		restoreOperationID: restoreOperationID,
		registrySet:        registrySet,
		uploadLocation:     uploadLocation,
	}
}

// skipSection skips an entire XML section
func (l *RestoreOperationProcessor) skipSection(decoder *xml.Decoder, startElement *xml.StartElement) error {
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

// collectFiles collects file data from XML without processing it immediately
func (l *RestoreOperationProcessor) collectFiles(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats) ([]XMLFile, error) {
	var files []XMLFile

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, errkit.Wrap(err, "failed to read file token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "file" {
				xmlFile, err := l.collectFile(ctx, decoder, &t, stats)
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

// collectFile collects a single file's data without processing it immediately
func (l *RestoreOperationProcessor) collectFile(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats) (*XMLFile, error) {
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
				if err := l.decodeBase64ToFile(ctx, decoder, &xmlFile, stats); err != nil {
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

// decodeBase64ToFile streams base64 data and saves it to blob storage
func (l *RestoreOperationProcessor) decodeBase64ToFile(ctx context.Context, decoder *xml.Decoder, xmlFile *XMLFile, stats *RestoreStats) error {
	// Open blob bucket
	b, err := blob.OpenBucket(ctx, l.uploadLocation)
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

// validateOptions validates the restore options
func (*RestoreOperationProcessor) validateOptions(options RestoreOptions) error {
	switch options.Strategy {
	case RestoreStrategyFullReplace, RestoreStrategyMergeAdd, RestoreStrategyMergeUpdate:
		// Valid strategies
	default:
		return errors.New("invalid restore strategy")
	}
	return nil
}

// loadExistingEntities loads all existing entities from the database
func (l *RestoreOperationProcessor) loadExistingEntities(ctx context.Context, entities *ExistingEntities) error {
	entities.Locations = make(map[string]*models.Location)
	entities.Areas = make(map[string]*models.Area)
	entities.Commodities = make(map[string]*models.Commodity)
	entities.Images = make(map[string]*models.Image)
	entities.Invoices = make(map[string]*models.Invoice)
	entities.Manuals = make(map[string]*models.Manual)

	// Load locations - index by ID (which should be the same as XML ID for imported entities)
	locations, err := l.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing locations")
	}
	for _, location := range locations {
		entities.Locations[location.ID] = location
	}

	// Load areas - index by ID (which should be the same as XML ID for imported entities)
	areas, err := l.registrySet.AreaRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing areas")
	}
	for _, area := range areas {
		entities.Areas[area.ID] = area
	}

	// Load commodities - index by ID (which should be the same as XML ID for imported entities)
	commodities, err := l.registrySet.CommodityRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing commodities")
	}
	for _, commodity := range commodities {
		entities.Commodities[commodity.ID] = commodity
	}

	// Load images - index by ID (which should be the same as XML ID for imported entities)
	images, err := l.registrySet.ImageRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing images")
	}
	for _, image := range images {
		entities.Images[image.ID] = image
	}

	// Load invoices - index by ID (which should be the same as XML ID for imported entities)
	invoices, err := l.registrySet.InvoiceRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing invoices")
	}
	for _, invoice := range invoices {
		entities.Invoices[invoice.ID] = invoice
	}

	// Load manuals - index by ID (which should be the same as XML ID for imported entities)
	manuals, err := l.registrySet.ManualRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to load existing manuals")
	}
	for _, manual := range manuals {
		entities.Manuals[manual.ID] = manual
	}

	return nil
}

// clearExistingData removes all existing data for full replace strategy
func (l *RestoreOperationProcessor) clearExistingData(ctx context.Context) error {
	// Delete all locations recursively (this will also delete areas and commodities)
	locations, err := l.registrySet.LocationRegistry.List(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to list locations for deletion")
	}
	for _, location := range locations {
		if err := l.registrySet.LocationRegistry.DeleteRecursive(ctx, location.ID); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to delete location %s recursively", location.ID))
		}
	}

	return nil
}

// createFileRecord creates a file record in the appropriate registry with strategy support
func (l *RestoreOperationProcessor) createFileRecord(ctx context.Context, xmlFile *XMLFile, commodityID, fileType string, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	file := xmlFile.ConvertToFile()
	originalXMLID := xmlFile.ID

	switch fileType {
	case "image":
		// Apply strategy for images
		existingImage := existing.Images[originalXMLID]
		switch options.Strategy {
		case RestoreStrategyFullReplace:
			// Always create (database was cleared)
			if !options.DryRun {
				image := &models.Image{
					EntityID:    models.EntityID{ID: originalXMLID},
					CommodityID: commodityID,
					File:        file,
				}
				if err := image.ValidateWithContext(ctx); err != nil {
					return errkit.Wrap(err, "invalid image")
				}
				createdImage, err := l.registrySet.ImageRegistry.Create(ctx, *image)
				if err != nil {
					return errkit.Wrap(err, "failed to create image")
				}
				// Track the newly created image and store ID mapping
				existing.Images[originalXMLID] = createdImage
				idMapping.Images[originalXMLID] = createdImage.ID
			}
			stats.ImageCount++
		case RestoreStrategyMergeAdd:
			// Only create if doesn't exist
			if existingImage == nil {
				if !options.DryRun {
					image := &models.Image{
						EntityID:    models.EntityID{ID: originalXMLID},
						CommodityID: commodityID,
						File:        file,
					}
					if err := image.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid image")
					}
					createdImage, err := l.registrySet.ImageRegistry.Create(ctx, *image)
					if err != nil {
						return errkit.Wrap(err, "failed to create image")
					}
					// Track the newly created image and store ID mapping
					existing.Images[originalXMLID] = createdImage
					idMapping.Images[originalXMLID] = createdImage.ID
				}
				stats.ImageCount++
			} else {
				stats.SkippedCount++
			}
		case RestoreStrategyMergeUpdate:
			// Create if missing, update if exists
			if existingImage == nil {
				if !options.DryRun {
					image := &models.Image{
						EntityID:    models.EntityID{ID: originalXMLID},
						CommodityID: commodityID,
						File:        file,
					}
					if err := image.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid image")
					}
					createdImage, err := l.registrySet.ImageRegistry.Create(ctx, *image)
					if err != nil {
						return errkit.Wrap(err, "failed to create image")
					}
					// Track the newly created image and store ID mapping
					existing.Images[originalXMLID] = createdImage
					idMapping.Images[originalXMLID] = createdImage.ID
				}
				stats.ImageCount++
			} else {
				if !options.DryRun {
					// Update existing image
					image := &models.Image{
						EntityID:    existingImage.EntityID, // Keep the existing ID
						CommodityID: commodityID,
						File:        file,
					}
					if err := image.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid image")
					}
					updatedImage, err := l.registrySet.ImageRegistry.Update(ctx, *image)
					if err != nil {
						return errkit.Wrap(err, "failed to update image")
					}
					// Update the tracked image
					existing.Images[originalXMLID] = updatedImage
				}
				stats.UpdatedCount++
			}
		}
	case "invoice":
		// Apply strategy for invoices
		existingInvoice := existing.Invoices[originalXMLID]
		switch options.Strategy {
		case RestoreStrategyFullReplace:
			// Always create (database was cleared)
			if !options.DryRun {
				invoice := &models.Invoice{
					EntityID:    models.EntityID{ID: originalXMLID},
					CommodityID: commodityID,
					File:        file,
				}
				if err := invoice.ValidateWithContext(ctx); err != nil {
					return errkit.Wrap(err, "invalid invoice")
				}
				createdInvoice, err := l.registrySet.InvoiceRegistry.Create(ctx, *invoice)
				if err != nil {
					return errkit.Wrap(err, "failed to create invoice")
				}
				// Track the newly created invoice and store ID mapping
				existing.Invoices[originalXMLID] = createdInvoice
				idMapping.Invoices[originalXMLID] = createdInvoice.ID
			}
			stats.InvoiceCount++
		case RestoreStrategyMergeAdd:
			// Only create if doesn't exist
			if existingInvoice == nil {
				if !options.DryRun {
					invoice := &models.Invoice{
						EntityID:    models.EntityID{ID: originalXMLID},
						CommodityID: commodityID,
						File:        file,
					}
					if err := invoice.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid invoice")
					}
					createdInvoice, err := l.registrySet.InvoiceRegistry.Create(ctx, *invoice)
					if err != nil {
						return errkit.Wrap(err, "failed to create invoice")
					}
					// Track the newly created invoice and store ID mapping
					existing.Invoices[originalXMLID] = createdInvoice
					idMapping.Invoices[originalXMLID] = createdInvoice.ID
				}
				stats.InvoiceCount++
			} else {
				stats.SkippedCount++
			}
		case RestoreStrategyMergeUpdate:
			// Create if missing, update if exists
			if existingInvoice == nil {
				if !options.DryRun {
					invoice := &models.Invoice{
						EntityID:    models.EntityID{ID: originalXMLID},
						CommodityID: commodityID,
						File:        file,
					}
					if err := invoice.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid invoice")
					}
					createdInvoice, err := l.registrySet.InvoiceRegistry.Create(ctx, *invoice)
					if err != nil {
						return errkit.Wrap(err, "failed to create invoice")
					}
					// Track the newly created invoice and store ID mapping
					existing.Invoices[originalXMLID] = createdInvoice
					idMapping.Invoices[originalXMLID] = createdInvoice.ID
				}
				stats.InvoiceCount++
			} else {
				if !options.DryRun {
					// Update existing invoice
					invoice := &models.Invoice{
						EntityID:    existingInvoice.EntityID, // Keep the existing ID
						CommodityID: commodityID,
						File:        file,
					}
					if err := invoice.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid invoice")
					}
					updatedInvoice, err := l.registrySet.InvoiceRegistry.Update(ctx, *invoice)
					if err != nil {
						return errkit.Wrap(err, "failed to update invoice")
					}
					// Update the tracked invoice
					existing.Invoices[originalXMLID] = updatedInvoice
				}
				stats.UpdatedCount++
			}
		}
	case "manual":
		// Apply strategy for manuals
		existingManual := existing.Manuals[originalXMLID]
		switch options.Strategy {
		case RestoreStrategyFullReplace:
			// Always create (database was cleared)
			if !options.DryRun {
				manual := &models.Manual{
					EntityID:    models.EntityID{ID: originalXMLID},
					CommodityID: commodityID,
					File:        file,
				}
				if err := manual.ValidateWithContext(ctx); err != nil {
					return errkit.Wrap(err, "invalid manual")
				}
				createdManual, err := l.registrySet.ManualRegistry.Create(ctx, *manual)
				if err != nil {
					return errkit.Wrap(err, "failed to create manual")
				}
				// Track the newly created manual and store ID mapping
				existing.Manuals[originalXMLID] = createdManual
				idMapping.Manuals[originalXMLID] = createdManual.ID
			}
			stats.ManualCount++
		case RestoreStrategyMergeAdd:
			// Only create if doesn't exist
			if existingManual == nil {
				if !options.DryRun {
					manual := &models.Manual{
						EntityID:    models.EntityID{ID: originalXMLID},
						CommodityID: commodityID,
						File:        file,
					}
					if err := manual.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid manual")
					}
					createdManual, err := l.registrySet.ManualRegistry.Create(ctx, *manual)
					if err != nil {
						return errkit.Wrap(err, "failed to create manual")
					}
					// Track the newly created manual and store ID mapping
					existing.Manuals[originalXMLID] = createdManual
					idMapping.Manuals[originalXMLID] = createdManual.ID
				}
				stats.ManualCount++
			} else {
				stats.SkippedCount++
			}
		case RestoreStrategyMergeUpdate:
			// Create if missing, update if exists
			if existingManual == nil {
				if !options.DryRun {
					manual := &models.Manual{
						EntityID:    models.EntityID{ID: originalXMLID},
						CommodityID: commodityID,
						File:        file,
					}
					if err := manual.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid manual")
					}
					createdManual, err := l.registrySet.ManualRegistry.Create(ctx, *manual)
					if err != nil {
						return errkit.Wrap(err, "failed to create manual")
					}
					// Track the newly created manual and store ID mapping
					existing.Manuals[originalXMLID] = createdManual
					idMapping.Manuals[originalXMLID] = createdManual.ID
				}
				stats.ManualCount++
			} else {
				if !options.DryRun {
					// Update existing manual
					manual := &models.Manual{
						EntityID:    existingManual.EntityID, // Keep the existing ID
						CommodityID: commodityID,
						File:        file,
					}
					if err := manual.ValidateWithContext(ctx); err != nil {
						return errkit.Wrap(err, "invalid manual")
					}
					updatedManual, err := l.registrySet.ManualRegistry.Update(ctx, *manual)
					if err != nil {
						return errkit.Wrap(err, "failed to update manual")
					}
					// Update the tracked manual
					existing.Manuals[originalXMLID] = updatedManual
				}
				stats.UpdatedCount++
			}
		}
	default:
		return errors.New(fmt.Sprintf("unknown file type: %s", fileType))
	}

	return nil
}

// createRestoreStep creates a new restore step
func (l *RestoreOperationProcessor) createRestoreStep(ctx context.Context, name string, result models.RestoreStepResult, reason string) {
	step := models.RestoreStep{
		RestoreOperationID: l.restoreOperationID,
		Name:               name,
		Result:             result,
		Reason:             reason,
		CreatedDate:        models.PNow(),
	}

	_, err := l.registrySet.RestoreStepRegistry.Create(ctx, step)
	if err != nil {
		// Log error but don't fail the restore operation
		log.WithError(err).Error("Failed to create restore step")
	}
}

// updateRestoreStep updates an existing restore step
func (l *RestoreOperationProcessor) updateRestoreStep(ctx context.Context, name string, result models.RestoreStepResult, reason string) {
	// Get all steps for this restore operation
	steps, err := l.registrySet.RestoreStepRegistry.ListByRestoreOperation(ctx, l.restoreOperationID)
	if err != nil {
		// If we can't get steps, create a new one
		l.createRestoreStep(ctx, name, result, reason)
		return
	}

	// Find the step with the matching name
	for _, step := range steps {
		if step.Name == name {
			step.Result = result
			step.Reason = reason
			_, err := l.registrySet.RestoreStepRegistry.Update(ctx, *step)
			if err != nil {
				// Log error but don't fail the restore operation
				log.WithError(err).Error("Failed to update restore step")
			}
			return
		}
	}

	// If step not found, create it
	l.createRestoreStep(ctx, name, result, reason)
}

// markRestoreFailed marks a restore operation as failed with an error message
func (l *RestoreOperationProcessor) markRestoreFailed(ctx context.Context, errorMessage string) error {
	restoreOperation, err := l.registrySet.RestoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return err
	}

	restoreOperation.Status = models.RestoreStatusFailed
	restoreOperation.CompletedDate = models.PNow()
	restoreOperation.ErrorMessage = errorMessage

	_, err = l.registrySet.RestoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		return err
	}

	l.createRestoreStep(ctx, "Restore failed", models.RestoreStepResultError, errorMessage)
	return fmt.Errorf("%s", errorMessage)
}

// processRestoreWithDetailedLogging processes the restore with detailed step-by-step logging
func (l *RestoreOperationProcessor) processRestoreWithDetailedLogging(ctx context.Context, reader io.Reader, options RestoreOptions) (*RestoreStats, error) {
	// Create step for loading existing data
	l.createRestoreStep(ctx, "Loading existing data", models.RestoreStepResultInProgress, "")

	// Create a custom restore service with logging callbacks
	stats, err := l.RestoreFromXML(ctx, reader, options)
	if err != nil {
		l.updateRestoreStep(ctx, "Loading existing data", models.RestoreStepResultError, err.Error())
		return stats, err
	}

	l.updateRestoreStep(ctx, "Loading existing data", models.RestoreStepResultSuccess, "")

	return stats, nil
}
func (l *RestoreOperationProcessor) Process(ctx context.Context) error {
	// Get the restore operation
	restoreOperation, err := l.registrySet.RestoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get restore operation: %v", err))
	}

	// Get the export to find the file path
	export, err := l.registrySet.ExportRegistry.Get(ctx, restoreOperation.ExportID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get export: %v", err))
	}

	// Update status to running
	restoreOperation.Status = models.RestoreStatusRunning
	restoreOperation.StartedDate = models.PNow()
	_, err = l.registrySet.RestoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to update restore status: %v", err))
	}

	// Create initial restore steps
	l.createRestoreStep(ctx, "Initializing restore", models.RestoreStepResultInProgress, "")

	// Open the export file
	b, err := blob.OpenBucket(ctx, l.uploadLocation)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to open blob bucket: %v", err))
	}
	defer b.Close()

	reader, err := b.NewReader(ctx, export.FilePath, nil)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to open export file: %v", err))
	}
	defer reader.Close()

	// Update step to processing
	l.updateRestoreStep(ctx, "Initializing restore", models.RestoreStepResultSuccess, "")
	l.createRestoreStep(ctx, "Reading XML file", models.RestoreStepResultInProgress, "")

	// Convert options to restore service format
	restoreOptions := RestoreOptions{
		Strategy:        RestoreStrategy(restoreOperation.Options.Strategy),
		IncludeFileData: restoreOperation.Options.IncludeFileData,
		DryRun:          restoreOperation.Options.DryRun,
	}

	// Update step to processing
	l.updateRestoreStep(ctx, "Reading XML file", models.RestoreStepResultSuccess, "")

	// Process with detailed logging
	stats, err := l.processRestoreWithDetailedLogging(ctx, reader, restoreOptions)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("restore failed: %v", err))
	}

	// Mark restore as completed
	restoreOperation.Status = models.RestoreStatusCompleted
	restoreOperation.CompletedDate = models.PNow()
	restoreOperation.LocationCount = stats.LocationCount
	restoreOperation.AreaCount = stats.AreaCount
	restoreOperation.CommodityCount = stats.CommodityCount
	restoreOperation.ImageCount = stats.ImageCount
	restoreOperation.InvoiceCount = stats.InvoiceCount
	restoreOperation.ManualCount = stats.ManualCount
	restoreOperation.BinaryDataSize = stats.BinaryDataSize
	restoreOperation.ErrorCount = stats.ErrorCount

	_, err = l.registrySet.RestoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to update restore completion status: %v", err))
	}

	l.createRestoreStep(ctx, "Restore completed successfully", models.RestoreStepResultSuccess,
		fmt.Sprintf("Processed %d locations, %d areas, %d commodities with %d errors",
			stats.LocationCount, stats.AreaCount, stats.CommodityCount, stats.ErrorCount))

	return nil
}

// RestoreFromXML processes the restore with detailed logging using streaming approach
func (l *RestoreOperationProcessor) RestoreFromXML(ctx context.Context, reader io.Reader, options RestoreOptions) (*RestoreStats, error) {
	// Override the processing methods to include logging
	return l.restoreFromXMLWithStreamingLogging(ctx, reader, options)
}

// restoreFromXMLWithStreamingLogging processes the restore with detailed logging using streaming approach
func (l *RestoreOperationProcessor) restoreFromXMLWithStreamingLogging(ctx context.Context, xmlReader io.Reader, options RestoreOptions) (*RestoreStats, error) {
	stats := &RestoreStats{}

	// Validate options
	if err := l.validateOptions(options); err != nil {
		return stats, errkit.Wrap(err, "invalid restore options")
	}

	// Get main currency from settings and add it to context for commodity validation
	settings, err := l.registrySet.SettingsRegistry.Get(ctx)
	if err != nil {
		return stats, errkit.Wrap(err, "failed to get settings")
	}

	if settings.MainCurrency != nil && *settings.MainCurrency != "" {
		ctx = validationctx.WithMainCurrency(ctx, *settings.MainCurrency)
	}

	decoder := xml.NewDecoder(xmlReader)

	// Track existing entities for validation and strategy decisions
	existingEntities := &ExistingEntities{}
	idMapping := &IDMapping{
		Locations:   make(map[string]string),
		Areas:       make(map[string]string),
		Commodities: make(map[string]string),
		Images:      make(map[string]string),
		Invoices:    make(map[string]string),
		Manuals:     make(map[string]string),
	}

	if options.Strategy != RestoreStrategyFullReplace {
		if err := l.loadExistingEntities(ctx, existingEntities); err != nil {
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
		for xmlID, entity := range existingEntities.Images {
			idMapping.Images[xmlID] = entity.ID
		}
		for xmlID, entity := range existingEntities.Invoices {
			idMapping.Invoices[xmlID] = entity.ID
		}
		for xmlID, entity := range existingEntities.Manuals {
			idMapping.Manuals[xmlID] = entity.ID
		}
	} else {
		// For full replace, initialize empty maps to track newly created entities
		existingEntities.Locations = make(map[string]*models.Location)
		existingEntities.Areas = make(map[string]*models.Area)
		existingEntities.Commodities = make(map[string]*models.Commodity)
		existingEntities.Images = make(map[string]*models.Image)
		existingEntities.Invoices = make(map[string]*models.Invoice)
		existingEntities.Manuals = make(map[string]*models.Manual)
	}

	// If full replace, clear existing data first
	if options.Strategy == RestoreStrategyFullReplace && !options.DryRun {
		if err := l.clearExistingData(ctx); err != nil {
			return stats, errkit.Wrap(err, "failed to clear existing data")
		}
	}

	// Process XML stream with logging
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
				l.createRestoreStep(ctx, "Processing locations", models.RestoreStepResultInProgress, "")
				if err := l.processLocationsWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
					l.updateRestoreStep(ctx, "Processing locations", models.RestoreStepResultError, err.Error())
					return stats, errkit.Wrap(err, "failed to process locations")
				}
				l.updateRestoreStep(ctx, "Processing locations", models.RestoreStepResultSuccess,
					fmt.Sprintf("Processed %d locations", stats.LocationCount))
			case "areas":
				l.createRestoreStep(ctx, "Processing areas", models.RestoreStepResultInProgress, "")
				if err := l.processAreasWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
					l.updateRestoreStep(ctx, "Processing areas", models.RestoreStepResultError, err.Error())
					return stats, errkit.Wrap(err, "failed to process areas")
				}
				l.updateRestoreStep(ctx, "Processing areas", models.RestoreStepResultSuccess,
					fmt.Sprintf("Processed %d areas", stats.AreaCount))
			case "commodities":
				l.createRestoreStep(ctx, "Processing commodities", models.RestoreStepResultInProgress, "")
				if err := l.processCommoditiesWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
					l.updateRestoreStep(ctx, "Processing commodities", models.RestoreStepResultError, err.Error())
					return stats, errkit.Wrap(err, "failed to process commodities")
				}
				l.updateRestoreStep(ctx, "Processing commodities", models.RestoreStepResultSuccess,
					fmt.Sprintf("Processed %d commodities", stats.CommodityCount))
			}
		case xml.ProcInst, xml.Directive, xml.Comment, xml.CharData, xml.EndElement:
			// Skip processing instructions, directives, comments, character data, and end elements at root level
			continue
		default:
			return stats, errkit.WithFields(errors.New("unexpected token type"), "token_type", fmt.Sprintf("%T", t))
		}
	}

	return stats, nil
}

// processLocationsWithLogging processes the locations section with detailed logging
func (l *RestoreOperationProcessor) processLocationsWithLogging(ctx context.Context, decoder *xml.Decoder, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read locations token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "location" {
				if err := l.processLocationWithLogging(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
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

// processLocationWithLogging processes a single location with detailed logging
func (l *RestoreOperationProcessor) processLocationWithLogging(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	var xmlLocation XMLLocation
	if err := decoder.DecodeElement(&xmlLocation, startElement); err != nil {
		return errkit.Wrap(err, "failed to decode location")
	}

	// Predict action and log it
	action := l.predictAction(ctx, "location", xmlLocation.ID, options)
	emoji := l.getEmojiForAction(action)
	actionDesc := l.getActionDescription(action, options.DryRun)

	l.createRestoreStep(ctx,
		fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultInProgress, actionDesc)

	// Process the location using the original service logic
	location := xmlLocation.ConvertToLocation()
	if err := location.ValidateWithContext(ctx); err != nil {
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
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
			createdLocation, err := l.registrySet.LocationRegistry.Create(ctx, *location)
			if err != nil {
				l.updateRestoreStep(ctx,
					fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
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
				createdLocation, err := l.registrySet.LocationRegistry.Create(ctx, *location)
				if err != nil {
					l.updateRestoreStep(ctx,
						fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
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
				createdLocation, err := l.registrySet.LocationRegistry.Create(ctx, *location)
				if err != nil {
					l.updateRestoreStep(ctx,
						fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
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
				updatedLocation, err := l.registrySet.LocationRegistry.Update(ctx, *location)
				if err != nil {
					l.updateRestoreStep(ctx,
						fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
					return errkit.Wrap(err, fmt.Sprintf("failed to update location %s", originalXMLID))
				}
				// Update the tracked location
				existing.Locations[originalXMLID] = updatedLocation
			}
			stats.UpdatedCount++
			stats.LocationCount++
		}
	}

	l.updateRestoreStep(ctx,
		fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultSuccess, "Completed")

	return nil
}

// processAreasWithLogging processes the areas section with detailed logging
func (l *RestoreOperationProcessor) processAreasWithLogging(ctx context.Context, decoder *xml.Decoder, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read areas token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "area" {
				if err := l.processAreaWithLogging(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
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

// processAreaWithLogging processes a single area with detailed logging
func (l *RestoreOperationProcessor) processAreaWithLogging(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	var xmlArea XMLArea
	if err := decoder.DecodeElement(&xmlArea, startElement); err != nil {
		return errkit.Wrap(err, "failed to decode area")
	}

	// Predict action and log it
	action := l.predictAction(ctx, "area", xmlArea.ID, options)
	emoji := l.getEmojiForAction(action)
	actionDesc := l.getActionDescription(action, options.DryRun)

	l.createRestoreStep(ctx,
		fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultInProgress, actionDesc)

	// Store the original XML IDs for mapping
	originalXMLID := xmlArea.ID
	originalLocationXMLID := xmlArea.LocationID

	// Validate that the location exists (either in existing data or was just created)
	if existing.Locations[originalLocationXMLID] == nil {
		err := errors.New(fmt.Sprintf("area %s references non-existent location %s", originalXMLID, originalLocationXMLID))
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
		return err
	}

	// Get the actual database location ID
	actualLocationID := idMapping.Locations[originalLocationXMLID]
	if actualLocationID == "" {
		err := errors.New(fmt.Sprintf("no ID mapping found for location %s", originalLocationXMLID))
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
		return err
	}

	area := xmlArea.ConvertToArea()
	// Set the correct location ID from the mapping
	area.LocationID = actualLocationID

	if err := area.ValidateWithContext(ctx); err != nil {
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
		return errkit.Wrap(err, fmt.Sprintf("invalid area %s", originalXMLID))
	}

	// Apply strategy
	existingArea := existing.Areas[originalXMLID]
	switch options.Strategy {
	case RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if !options.DryRun {
			createdArea, err := l.registrySet.AreaRegistry.Create(ctx, *area)
			if err != nil {
				l.updateRestoreStep(ctx,
					fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
				return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
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
				createdArea, err := l.registrySet.AreaRegistry.Create(ctx, *area)
				if err != nil {
					l.updateRestoreStep(ctx,
						fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
					return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
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
				createdArea, err := l.registrySet.AreaRegistry.Create(ctx, *area)
				if err != nil {
					l.updateRestoreStep(ctx,
						fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
					return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
				}
				// Track the newly created area and store ID mapping
				existing.Areas[originalXMLID] = createdArea
				idMapping.Areas[originalXMLID] = createdArea.ID
			}
			stats.CreatedCount++
			stats.AreaCount++
		} else {
			if !options.DryRun {
				updatedArea, err := l.registrySet.AreaRegistry.Update(ctx, *area)
				if err != nil {
					l.updateRestoreStep(ctx,
						fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
					return errkit.Wrap(err, fmt.Sprintf("failed to update area %s", originalXMLID))
				}
				// Update the tracked area
				existing.Areas[originalXMLID] = updatedArea
			}
			stats.UpdatedCount++
			stats.AreaCount++
		}
	}

	l.updateRestoreStep(ctx,
		fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultSuccess, "Completed")

	return nil
}

// processCommoditiesWithLogging processes the commodities section with detailed logging
func (l *RestoreOperationProcessor) processCommoditiesWithLogging(ctx context.Context, decoder *xml.Decoder, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodities token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "commodity" {
				if err := l.processCommodityWithLogging(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
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

// processCommodityWithLogging processes a single commodity with detailed logging
func (l *RestoreOperationProcessor) processCommodityWithLogging(ctx context.Context, decoder *xml.Decoder, startElement *xml.StartElement, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
	var xmlCommodity XMLCommodity
	var pendingFiles []PendingFileData // Collect file data to process after commodity creation

	// Get commodity ID from attributes
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlCommodity.ID = attr.Value
			break
		}
	}

	// Predict action and log it early (we'll update it later)
	action := l.predictAction(ctx, "commodity", xmlCommodity.ID, options)
	emoji := l.getEmojiForAction(action)
	actionDesc := l.getActionDescription(action, options.DryRun)

	// We'll use a consistent step name throughout - start with ID and update description when we get the name
	stepName := fmt.Sprintf("%s Commodity: %s", emoji, xmlCommodity.ID)
	l.createRestoreStep(ctx, stepName, models.RestoreStepResultInProgress, actionDesc)

	// Process commodity elements
	for {
		tok, err := decoder.Token()
		if err != nil {
			l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, err.Error())
			return errkit.Wrap(err, "failed to read commodity element token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "commodityName":
				if err := decoder.DecodeElement(&xmlCommodity.CommodityName, &t); err != nil {
					l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, err.Error())
					return errkit.Wrap(err, "failed to decode commodity name")
				}
				// Update step description with actual commodity name
				l.updateRestoreStep(ctx, stepName, models.RestoreStepResultInProgress,
					fmt.Sprintf("Processing %s", xmlCommodity.CommodityName))
			case "shortName":
				if err := decoder.DecodeElement(&xmlCommodity.ShortName, &t); err != nil {
					return errkit.Wrap(err, "failed to decode short name")
				}
			case "areaId":
				if err := decoder.DecodeElement(&xmlCommodity.AreaID, &t); err != nil {
					return errkit.Wrap(err, "failed to decode area ID")
				}
			case "type":
				if err := decoder.DecodeElement(&xmlCommodity.Type, &t); err != nil {
					return errkit.Wrap(err, "failed to decode type")
				}
			case "count":
				if err := decoder.DecodeElement(&xmlCommodity.Count, &t); err != nil {
					return errkit.Wrap(err, "failed to decode count")
				}
			case "status":
				if err := decoder.DecodeElement(&xmlCommodity.Status, &t); err != nil {
					return errkit.Wrap(err, "failed to decode status")
				}
			case "originalPrice":
				if err := decoder.DecodeElement(&xmlCommodity.OriginalPrice, &t); err != nil {
					return errkit.Wrap(err, "failed to decode original price")
				}
			case "originalPriceCurrency":
				if err := decoder.DecodeElement(&xmlCommodity.OriginalCurrency, &t); err != nil {
					return errkit.Wrap(err, "failed to decode original price currency")
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
					files, err := l.collectFiles(ctx, decoder, &t, stats)
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
					if err := l.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip images section")
					}
				}
			case "invoices":
				if options.IncludeFileData {
					files, err := l.collectFiles(ctx, decoder, &t, stats)
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
					if err := l.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip invoices section")
					}
				}
			case "manuals":
				if options.IncludeFileData {
					files, err := l.collectFiles(ctx, decoder, &t, stats)
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
					if err := l.skipSection(decoder, &t); err != nil {
						return errkit.Wrap(err, "failed to skip manuals section")
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "commodity" {
				// Process the commodity first
				if err := l.createOrUpdateCommodityWithLogging(ctx, &xmlCommodity, stats, existing, idMapping, options); err != nil {
					l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, err.Error())
					return err
				}

				// Process pending files after commodity creation
				if len(pendingFiles) > 0 {
					commodityID := idMapping.Commodities[xmlCommodity.ID]
					if commodityID == "" {
						commodityID = xmlCommodity.ID // Fallback to XML ID
					}
					for _, fileData := range pendingFiles {
						for _, xmlFile := range fileData.XMLFiles {
							if err := l.createFileRecord(ctx, &xmlFile, commodityID, fileData.FileType, stats, existing, idMapping, options); err != nil {
								// Log file processing errors but don't fail the entire commodity
								stats.ErrorCount++
								stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process %s file for commodity %s: %v", fileData.FileType, xmlCommodity.ID, err))
							}
						}
					}
				}

				l.updateRestoreStep(ctx, stepName, models.RestoreStepResultSuccess, "Completed")
				return nil
			}
		}
	}
}

// createOrUpdateCommodityWithLogging creates or updates a commodity with detailed logging
func (l *RestoreOperationProcessor) createOrUpdateCommodityWithLogging(ctx context.Context, xmlCommodity *XMLCommodity, stats *RestoreStats, existing *ExistingEntities, idMapping *IDMapping, options RestoreOptions) error {
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
			createdCommodity, err := l.registrySet.CommodityRegistry.Create(ctx, *commodity)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", originalXMLID))
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
				createdCommodity, err := l.registrySet.CommodityRegistry.Create(ctx, *commodity)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", originalXMLID))
				}
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
				createdCommodity, err := l.registrySet.CommodityRegistry.Create(ctx, *commodity)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create commodity %s", originalXMLID))
				}
				// Track the newly created commodity and store ID mapping
				existing.Commodities[originalXMLID] = createdCommodity
				idMapping.Commodities[originalXMLID] = createdCommodity.ID
			}
			stats.CreatedCount++
			stats.CommodityCount++
		} else {
			if !options.DryRun {
				updatedCommodity, err := l.registrySet.CommodityRegistry.Update(ctx, *commodity)
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

// predictAction predicts what action will be taken for an entity based on strategy
func (l *RestoreOperationProcessor) predictAction(ctx context.Context, entityType, entityID string, options RestoreOptions) string {
	switch options.Strategy {
	case RestoreStrategyFullReplace:
		return "create"
	case RestoreStrategyMergeAdd:
		// Check if entity exists
		exists := l.entityExists(ctx, entityType, entityID)
		if exists {
			return "skip"
		}
		return "create"
	case RestoreStrategyMergeUpdate:
		// Check if entity exists
		exists := l.entityExists(ctx, entityType, entityID)
		if exists {
			return "update"
		}
		return "create"
	default:
		return "unknown"
	}
}

// entityExists checks if an entity exists in the database
func (l *RestoreOperationProcessor) entityExists(ctx context.Context, entityType, entityID string) bool {
	switch entityType {
	case "location":
		_, err := l.registrySet.LocationRegistry.Get(ctx, entityID)
		return err == nil
	case "area":
		_, err := l.registrySet.AreaRegistry.Get(ctx, entityID)
		return err == nil
	case "commodity":
		_, err := l.registrySet.CommodityRegistry.Get(ctx, entityID)
		return err == nil
	default:
		return false
	}
}

// getEmojiForAction returns an emoji for the action
func (l *RestoreOperationProcessor) getEmojiForAction(action string) string {
	switch action {
	case "create":
		return "📝"
	case "update":
		return "🔄"
	case "skip":
		return "⏭️"
	default:
		return "❓"
	}
}

// getActionDescription returns a description for the action
func (l *RestoreOperationProcessor) getActionDescription(action string, dryRun bool) string {
	prefix := ""
	if dryRun {
		prefix = "[DRY RUN] Would "
	} else {
		prefix = "Will "
	}

	switch action {
	case "create":
		return prefix + "create new entity"
	case "update":
		return prefix + "update existing entity"
	case "skip":
		return prefix + "skip (already exists)"
	default:
		return prefix + "perform unknown action"
	}
}
