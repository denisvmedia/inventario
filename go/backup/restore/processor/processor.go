package processor

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreOperationProcessor wraps the restore service to provide detailed logging
type RestoreOperationProcessor struct {
	restoreOperationID string
	registrySet        *registry.Set
	entityService      *services.EntityService
	uploadLocation     string
}

func NewRestoreOperationProcessor(restoreOperationID string, registrySet *registry.Set, entityService *services.EntityService, uploadLocation string) *RestoreOperationProcessor {
	return &RestoreOperationProcessor{
		restoreOperationID: restoreOperationID,
		registrySet:        registrySet,
		entityService:      entityService,
		uploadLocation:     uploadLocation,
	}
}

func (l *RestoreOperationProcessor) Process(ctx context.Context) error {
	restoreOperationRegistry := l.registrySet.RestoreOperationRegistry.WithServiceAccount()
	// Get the restore operation
	restoreOperation, err := restoreOperationRegistry.WithServiceAccount().Get(ctx, l.restoreOperationID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get restore operation: %v", err))
	}

	// Get the export to find the file path
	export, err := l.registrySet.ExportRegistry.WithServiceAccount().Get(ctx, restoreOperation.ExportID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get export: %v", err))
	}

	user, err := l.registrySet.UserRegistry.Get(ctx, export.UserID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get user: %v", err))
	}

	ctx = appctx.WithUser(ctx, user)

	// Update status to running
	restoreOperation.Status = models.RestoreStatusRunning
	restoreOperation.StartedDate = models.PNow()
	_, err = restoreOperationRegistry.Update(ctx, *restoreOperation)
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
	restoreOptions := types.RestoreOptions{
		Strategy:        types.RestoreStrategy(restoreOperation.Options.Strategy),
		IncludeFileData: restoreOperation.Options.IncludeFileData,
		DryRun:          restoreOperation.Options.DryRun,
	}

	// Update step to processing
	l.updateRestoreStep(ctx, "Reading XML file", models.RestoreStepResultSuccess, "")

	// Process with detailed logging
	stats, err := l.processRestore(ctx, reader, restoreOptions)
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

	_, err = restoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to update restore completion status: %v", err))
	}

	l.createRestoreStep(ctx, "Restore completed successfully", models.RestoreStepResultSuccess,
		fmt.Sprintf("Processed %d locations, %d areas, %d commodities with %d errors",
			stats.LocationCount, stats.AreaCount, stats.CommodityCount, stats.ErrorCount))

	return nil
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
func (l *RestoreOperationProcessor) collectFiles(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
) ([]types.XMLFile, error) {
	var files []types.XMLFile

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

func (l *RestoreOperationProcessor) processFile(
	ctx context.Context,
	t xml.StartElement,
	decoder *xml.Decoder,
	xmlFile *types.XMLFile,
	stats *types.RestoreStats,
) error {
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
		if err := l.decodeBase64ToFile(ctx, decoder, xmlFile, stats); err != nil {
			return errkit.Wrap(err, "failed to decode base64 data")
		}
	}

	return nil
}

// collectFile collects a single file's data without processing it immediately
func (l *RestoreOperationProcessor) collectFile(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
) (*types.XMLFile, error) {
	var xmlFile types.XMLFile

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
			err = l.processFile(ctx, t, decoder, &xmlFile, stats)
			if err != nil {
				return nil, err
			}
		case xml.EndElement:
			if t.Name.Local == "file" {
				return &xmlFile, nil
			}
		}
	}
}

// decodeBase64ToFile streams base64 data and saves it to blob storage
func (l *RestoreOperationProcessor) decodeBase64ToFile(
	ctx context.Context,
	decoder *xml.Decoder,
	xmlFile *types.XMLFile,
	stats *types.RestoreStats,
) error {
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
func (*RestoreOperationProcessor) validateOptions(options types.RestoreOptions) error {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace,
		types.RestoreStrategyMergeAdd,
		types.RestoreStrategyMergeUpdate:
		// Valid strategies
	default:
		return errors.New("invalid restore strategy")
	}
	return nil
}

// loadExistingEntities loads all existing entities from the database
func (l *RestoreOperationProcessor) loadExistingEntities(ctx context.Context, entities *types.ExistingEntities) error {
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
		if err := l.entityService.DeleteLocationRecursive(ctx, location.ID); err != nil {
			return errkit.Wrap(err, fmt.Sprintf("failed to delete location %s recursively", location.ID))
		}
	}

	return nil
}

//nolint:dupl,gocognit // Similar but not the same as other create*Record functions (and is readable enough)
func (l *RestoreOperationProcessor) createImageRecord(
	ctx context.Context,
	file *models.File,
	commodityID, originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	// Apply strategy for images
	existingImage := existing.Images[originalXMLID]
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if options.DryRun {
			stats.ImageCount++
			break
		}
		image := &models.Image{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: originalXMLID},
				TenantID: "default-tenant", // TODO: Get from context
			},
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
		stats.ImageCount++
	case types.RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingImage != nil {
			stats.SkippedCount++
			break
		}
		if options.DryRun {
			stats.ImageCount++
			break
		}
		image := &models.Image{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: originalXMLID},
				TenantID: "default-tenant", // TODO: Get from context
			},
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
		stats.ImageCount++
	case types.RestoreStrategyMergeUpdate:
		// Create if missing, update if exists
		if existingImage == nil {
			if !options.DryRun {
				image := &models.Image{
					TenantAwareEntityID: models.TenantAwareEntityID{
						EntityID: models.EntityID{ID: originalXMLID},
						TenantID: "default-tenant", // TODO: Get from context
					},
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
			break
		}

		// Update existing invoice
		if options.DryRun {
			stats.UpdatedCount++
			break
		}
		image := &models.Image{
			TenantAwareEntityID: existingImage.TenantAwareEntityID, // Keep the existing ID and tenant
			CommodityID:         commodityID,
			File:                file,
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
		stats.UpdatedCount++
	}

	return nil
}

//nolint:dupl,gocognit // Similar but not the same as other create*Record functions (and is readable enough)
func (l *RestoreOperationProcessor) createInvoiceRecord(
	ctx context.Context,
	file *models.File, commodityID,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	// Apply strategy for invoices
	existingInvoice := existing.Invoices[originalXMLID]
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if options.DryRun {
			stats.InvoiceCount++
			break
		}
		invoice := &models.Invoice{
			TenantAwareEntityID: models.TenantAwareEntityID{
				EntityID: models.EntityID{ID: originalXMLID},
				TenantID: "default-tenant", // TODO: Get from context
			},
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
		stats.InvoiceCount++
	case types.RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingInvoice != nil {
			stats.SkippedCount++
			break
		}
		if options.DryRun {
			stats.InvoiceCount++
			break
		}
		invoice := &models.Invoice{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: originalXMLID}, TenantID: "default-tenant"},
			CommodityID:         commodityID,
			File:                file,
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
		stats.InvoiceCount++
	case types.RestoreStrategyMergeUpdate:
		// Create if missing, update if exists
		if existingInvoice == nil {
			if !options.DryRun {
				invoice := &models.Invoice{
					TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: originalXMLID}, TenantID: "default-tenant"},
					CommodityID:         commodityID,
					File:                file,
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
			break
		}
		if options.DryRun {
			stats.UpdatedCount++
			break
		}
		// Update existing invoice
		invoice := &models.Invoice{
			TenantAwareEntityID: existingInvoice.TenantAwareEntityID, // Keep the existing ID
			CommodityID:         commodityID,
			File:                file,
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
		stats.UpdatedCount++
	}

	return nil
}

//nolint:gocognit // readable enough
func (l *RestoreOperationProcessor) createManualRecord(
	ctx context.Context,
	file *models.File,
	commodityID,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	// Apply strategy for manuals
	existingManual := existing.Manuals[originalXMLID]
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if options.DryRun {
			stats.ManualCount++
			break
		}
		manual := &models.Manual{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: originalXMLID}, TenantID: "default-tenant"},
			CommodityID:         commodityID,
			File:                file,
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
		stats.ManualCount++
	case types.RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingManual != nil {
			stats.SkippedCount++
			break
		}
		if options.DryRun {
			stats.ManualCount++
			break
		}
		manual := &models.Manual{
			TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: originalXMLID}, TenantID: "default-tenant"},
			CommodityID:         commodityID,
			File:                file,
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
		stats.ManualCount++
	case types.RestoreStrategyMergeUpdate:
		// Create if missing, update if exists
		if existingManual == nil {
			if options.DryRun {
				stats.ManualCount++
				break
			}
			manual := &models.Manual{
				TenantAwareEntityID: models.TenantAwareEntityID{EntityID: models.EntityID{ID: originalXMLID}, TenantID: "default-tenant"},
				CommodityID:         commodityID,
				File:                file,
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
			stats.ManualCount++
			break
		}
		// Update existing manual
		if options.DryRun {
			stats.UpdatedCount++
			break
		}
		manual := &models.Manual{
			TenantAwareEntityID: existingManual.TenantAwareEntityID, // Keep the existing ID
			CommodityID:         commodityID,
			File:                file,
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
		stats.UpdatedCount++
	}

	return nil
}

// createFileRecord creates a file record in the appropriate registry with strategy support
func (l *RestoreOperationProcessor) createFileRecord(
	ctx context.Context,
	xmlFile *types.XMLFile,
	commodityID, fileType string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	file := xmlFile.ConvertToFile()
	originalXMLID := xmlFile.ID

	switch fileType {
	case "image":
		err := l.createImageRecord(ctx, file, commodityID, originalXMLID, stats, existing, idMapping, options)
		if err != nil {
			return errkit.Wrap(err, "failed to create image record")
		}
	case "invoice":
		err := l.createInvoiceRecord(ctx, file, commodityID, originalXMLID, stats, existing, idMapping, options)
		if err != nil {
			return errkit.Wrap(err, "failed to create invoice record")
		}
	case "manual":
		err := l.createManualRecord(ctx, file, commodityID, originalXMLID, stats, existing, idMapping, options)
		if err != nil {
			return errkit.Wrap(err, "failed to create manual record")
		}
	default:
		return fmt.Errorf("unknown file type: %s", fileType)
	}

	return nil
}

// createRestoreStep creates a new restore step
func (l *RestoreOperationProcessor) createRestoreStep(
	ctx context.Context,
	name string,
	result models.RestoreStepResult,
	reason string,
) {
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
		slog.Error("Failed to create restore step", "error", err)
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
				slog.Error("Failed to update restore step", "error", err)
			}
			return
		}
	}

	// If step not found, create it
	l.createRestoreStep(ctx, name, result, reason)
}

// markRestoreFailed marks a restore operation as failed with an error message
func (l *RestoreOperationProcessor) markRestoreFailed(ctx context.Context, errorMessage string) error {
	restoreOperationRegistry := l.registrySet.RestoreOperationRegistry.WithServiceAccount()

	restoreOperation, err := restoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return err
	}

	restoreOperation.Status = models.RestoreStatusFailed
	restoreOperation.CompletedDate = models.PNow()
	restoreOperation.ErrorMessage = errorMessage

	_, err = restoreOperationRegistry.Update(ctx, *restoreOperation)
	if err != nil {
		return err
	}

	l.createRestoreStep(ctx, "Restore failed", models.RestoreStepResultError, errorMessage)
	return fmt.Errorf("%s", errorMessage)
}

// processRestore processes the restore with detailed step-by-step logging
func (l *RestoreOperationProcessor) processRestore(ctx context.Context, reader io.Reader, options types.RestoreOptions) (*types.RestoreStats, error) {
	// Create step for loading existing data
	l.createRestoreStep(ctx, "Loading existing data", models.RestoreStepResultInProgress, "")

	// Create a custom restore service with logging callbacks
	stats, err := l.restoreFromXML(ctx, reader, options)
	if err != nil {
		l.updateRestoreStep(ctx, "Loading existing data", models.RestoreStepResultError, err.Error())
		return stats, err
	}

	l.updateRestoreStep(ctx, "Loading existing data", models.RestoreStepResultSuccess, "")

	return stats, nil
}

func (l *RestoreOperationProcessor) restoreTopLevelElements(
	ctx context.Context,
	t xml.StartElement,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existingEntities *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	switch t.Name.Local {
	case "inventory":
		// Skip the root element, continue processing
		return nil
	case "locations":
		l.createRestoreStep(ctx, "Processing locations", models.RestoreStepResultInProgress, "")
		if err := l.processLocationsWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
			l.updateRestoreStep(ctx, "Processing locations", models.RestoreStepResultError, err.Error())
			return errkit.Wrap(err, "failed to process locations")
		}
		l.updateRestoreStep(ctx, "Processing locations", models.RestoreStepResultSuccess,
			fmt.Sprintf("Processed %d locations", stats.LocationCount))
	case "areas":
		l.createRestoreStep(ctx, "Processing areas", models.RestoreStepResultInProgress, "")
		if err := l.processAreasWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
			l.updateRestoreStep(ctx, "Processing areas", models.RestoreStepResultError, err.Error())
			return errkit.Wrap(err, "failed to process areas")
		}
		l.updateRestoreStep(ctx, "Processing areas", models.RestoreStepResultSuccess,
			fmt.Sprintf("Processed %d areas", stats.AreaCount))
	case "commodities":
		l.createRestoreStep(ctx, "Processing commodities", models.RestoreStepResultInProgress, "")
		if err := l.processCommoditiesWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
			l.updateRestoreStep(ctx, "Processing commodities", models.RestoreStepResultError, err.Error())
			return errkit.Wrap(err, "failed to process commodities")
		}
		l.updateRestoreStep(ctx, "Processing commodities", models.RestoreStepResultSuccess,
			fmt.Sprintf("Processed %d commodities", stats.CommodityCount))
	}
	return nil
}

func (l *RestoreOperationProcessor) RestoreFromXML(
	ctx context.Context,
	xmlReader io.Reader,
	options types.RestoreOptions,
) (*types.RestoreStats, error) {
	return l.processRestore(ctx, xmlReader, options)
}

// restoreFromXML processes the restore with detailed logging using streaming approach
//
//nolint:gocognit,gocyclo // readable enough
func (l *RestoreOperationProcessor) restoreFromXML(
	ctx context.Context,
	xmlReader io.Reader,
	options types.RestoreOptions,
) (*types.RestoreStats, error) {
	stats := &types.RestoreStats{}

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
	existingEntities := &types.ExistingEntities{}
	idMapping := &types.IDMapping{
		Locations:   make(map[string]string),
		Areas:       make(map[string]string),
		Commodities: make(map[string]string),
		Images:      make(map[string]string),
		Invoices:    make(map[string]string),
		Manuals:     make(map[string]string),
	}

	if options.Strategy != types.RestoreStrategyFullReplace {
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
	if options.Strategy == types.RestoreStrategyFullReplace && !options.DryRun {
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
			err := l.restoreTopLevelElements(ctx, t, decoder, stats, existingEntities, idMapping, options)
			if err != nil {
				return stats, err
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
func (l *RestoreOperationProcessor) processLocationsWithLogging(
	ctx context.Context,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read locations token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "location" {
				if err := l.processLocation(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
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

// processLocation processes a single location with detailed logging
func (l *RestoreOperationProcessor) processLocation(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	var xmlLocation types.XMLLocation
	if err := decoder.DecodeElement(&xmlLocation, startElement); err != nil {
		return errkit.Wrap(err, "failed to decode location")
	}

	// Predict action and log it
	action := l.predictAction(ctx, "location", xmlLocation.ID, options)
	emoji := l.getEmojiForAction(action)
	actionDesc := l.getActionDescription(action, options)

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
	case types.RestoreStrategyFullReplace:
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
	case types.RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingLocation != nil {
			stats.SkippedCount++
			break
		}
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
	case types.RestoreStrategyMergeUpdate:
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
			break
		}
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

	l.updateRestoreStep(ctx,
		fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultSuccess, "Completed")

	return nil
}

// processAreasWithLogging processes the areas section with detailed logging
func (l *RestoreOperationProcessor) processAreasWithLogging(
	ctx context.Context,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read areas token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "area" {
				if err := l.processArea(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
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

func (l *RestoreOperationProcessor) applyStrategyForArea(
	ctx context.Context,
	area *models.Area,
	existingArea *models.Area,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
	emoji string,
	xmlArea *types.XMLArea,
) error {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		// Always create (database was cleared)
		if !options.DryRun {
			createdArea, err := l.registrySet.AreaRegistry.Create(ctx, *area)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
			}
			// Track the newly created area and store ID mapping
			existing.Areas[originalXMLID] = createdArea
			idMapping.Areas[originalXMLID] = createdArea.ID
		}
		stats.CreatedCount++
		stats.AreaCount++
	case types.RestoreStrategyMergeAdd:
		// Only create if doesn't exist
		if existingArea == nil {
			if !options.DryRun {
				createdArea, err := l.registrySet.AreaRegistry.Create(ctx, *area)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
				}
				// Track the newly created area and store ID mapping
				existing.Areas[originalXMLID] = createdArea
				idMapping.Areas[originalXMLID] = createdArea.ID
			}
			stats.CreatedCount++
			stats.AreaCount++
			break
		}
		stats.SkippedCount++
	case types.RestoreStrategyMergeUpdate:
		// Create if missing, update if exists
		if existingArea == nil {
			if !options.DryRun {
				createdArea, err := l.registrySet.AreaRegistry.Create(ctx, *area)
				if err != nil {
					return errkit.Wrap(err, fmt.Sprintf("failed to create area %s", originalXMLID))
				}
				// Track the newly created area and store ID mapping
				existing.Areas[originalXMLID] = createdArea
				idMapping.Areas[originalXMLID] = createdArea.ID
			}
			stats.CreatedCount++
			stats.AreaCount++
			break
		}
		if !options.DryRun {
			updatedArea, err := l.registrySet.AreaRegistry.Update(ctx, *area)
			if err != nil {
				return errkit.Wrap(err, fmt.Sprintf("failed to update area %s", originalXMLID))
			}
			// Update the tracked area
			existing.Areas[originalXMLID] = updatedArea
		}
		stats.UpdatedCount++
		stats.AreaCount++
	}
	return nil
}

// processArea processes a single area with detailed logging
func (l *RestoreOperationProcessor) processArea(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	var xmlArea types.XMLArea
	if err := decoder.DecodeElement(&xmlArea, startElement); err != nil {
		return errkit.Wrap(err, "failed to decode area")
	}

	// Predict action and log it
	action := l.predictAction(ctx, "area", xmlArea.ID, options)
	emoji := l.getEmojiForAction(action)
	actionDesc := l.getActionDescription(action, options)

	l.createRestoreStep(ctx,
		fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultInProgress, actionDesc)

	// Store the original XML IDs for mapping
	originalXMLID := xmlArea.ID
	originalLocationXMLID := xmlArea.LocationID

	// Validate that the location exists (either in existing data or was just created)
	if existing.Locations[originalLocationXMLID] == nil {
		err := fmt.Errorf("area %s references non-existent location %s", originalXMLID, originalLocationXMLID)
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
		return err
	}

	// Get the actual database location ID
	actualLocationID := idMapping.Locations[originalLocationXMLID]
	if actualLocationID == "" {
		err := fmt.Errorf("no ID mapping found for location %s", originalLocationXMLID)
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
	err := l.applyStrategyForArea(ctx, area, existingArea, originalXMLID, stats, existing, idMapping, options, emoji, &xmlArea)
	if err != nil {
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
		return errkit.Wrap(err, "failed to apply strategy for area")
	}

	l.updateRestoreStep(ctx,
		fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultSuccess, "Completed")

	return nil
}

// processCommoditiesWithLogging processes the commodities section with detailed logging
func (l *RestoreOperationProcessor) processCommoditiesWithLogging(ctx context.Context, decoder *xml.Decoder, stats *types.RestoreStats, existing *types.ExistingEntities, idMapping *types.IDMapping, options types.RestoreOptions) error {
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errkit.Wrap(err, "failed to read commodities token")
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "commodity" {
				if err := l.processCommodity(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
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

//nolint:gocognit,gocyclo // readable enough
func (l *RestoreOperationProcessor) collectCommodityData(
	ctx context.Context,
	stepName string,
	t xml.StartElement,
	decoder *xml.Decoder,
	stats *types.RestoreStats,
	pendingFiles *[]types.PendingFileData,
	xmlCommodity *types.XMLCommodity,
	options types.RestoreOptions,
) error {
	switch t.Name.Local {
	case "commodityName":
		if err := decoder.DecodeElement(&xmlCommodity.CommodityName, &t); err != nil {
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
				*pendingFiles = append(*pendingFiles, types.PendingFileData{
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
				*pendingFiles = append(*pendingFiles, types.PendingFileData{
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
				*pendingFiles = append(*pendingFiles, types.PendingFileData{
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

	return nil
}

func (l *RestoreOperationProcessor) processCommodityData(
	ctx context.Context,
	xmlCommodity *types.XMLCommodity,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
	pendingFiles []types.PendingFileData,
) error {
	// Process the commodity first
	if err := l.createOrUpdateCommodity(ctx, xmlCommodity, stats, existing, idMapping, options); err != nil {
		return err
	}

	// Process pending files after commodity creation
	if len(pendingFiles) == 0 {
		return nil
	}

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

	return nil
}

// processCommodity processes a single commodity with detailed logging
func (l *RestoreOperationProcessor) processCommodity(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	var xmlCommodity types.XMLCommodity
	var pendingFiles []types.PendingFileData // Collect file data to process after commodity creation

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
	actionDesc := l.getActionDescription(action, options)

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
			err = l.collectCommodityData(ctx, stepName, t, decoder, stats, &pendingFiles, &xmlCommodity, options)
			if err != nil {
				l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, err.Error())
				return err
			}
		case xml.EndElement:
			if t.Name.Local != "commodity" {
				continue
			}

			err = l.processCommodityData(ctx, &xmlCommodity, stats, existing, idMapping, options, pendingFiles)
			if err != nil {
				l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, err.Error())
				return err
			}

			l.updateRestoreStep(ctx, stepName, models.RestoreStepResultSuccess, "Completed")
			return nil
		}
	}
}

func (l *RestoreOperationProcessor) applyStrategyForCommodity(
	ctx context.Context,
	commodity *models.Commodity,
	existingCommodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
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
	case types.RestoreStrategyMergeAdd:
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
	case types.RestoreStrategyMergeUpdate:
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
			return nil
		}
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

	return nil
}

// createOrUpdateCommodity creates or updates a commodity with detailed logging
func (l *RestoreOperationProcessor) createOrUpdateCommodity(
	ctx context.Context,
	xmlCommodity *types.XMLCommodity,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	// Store the original XML IDs for mapping
	originalXMLID := xmlCommodity.ID
	originalAreaXMLID := xmlCommodity.AreaID

	// Validate that the area exists (either in existing data or was just created)
	if existing.Areas[originalAreaXMLID] == nil {
		return errkit.Wrap(errors.New("commodity references non-existent area"), "original_commodity_id", originalXMLID, "original_area_id", originalAreaXMLID)
	}

	// Get the actual database area ID
	actualAreaID := idMapping.Areas[originalAreaXMLID]
	if actualAreaID == "" {
		return errkit.Wrap(errors.New("no ID mapping found for area"), "original_area_id", originalAreaXMLID)
	}

	commodity, err := xmlCommodity.ConvertToCommodity()
	if err != nil {
		return errkit.Wrap(err, "failed to convert commodity", "original_commodity_id", originalXMLID)
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
		return errkit.Wrap(err, "invalid commodity", "original_commodity_id", originalXMLID)
	}

	// Apply strategy
	existingCommodity := existing.Commodities[originalXMLID]
	err = l.applyStrategyForCommodity(ctx, commodity, existingCommodity, originalXMLID, stats, existing, idMapping, options)
	if err != nil {
		return err
	}

	return nil
}

// predictAction predicts what action will be taken for an entity based on strategy
func (l *RestoreOperationProcessor) predictAction(ctx context.Context, entityType, entityID string, options types.RestoreOptions) string {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		return "create"
	case types.RestoreStrategyMergeAdd:
		// Check if entity exists
		exists := l.entityExists(ctx, entityType, entityID)
		if exists {
			return "skip"
		}
		return "create"
	case types.RestoreStrategyMergeUpdate:
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
func (l *RestoreOperationProcessor) getActionDescription(action string, options types.RestoreOptions) string {
	prefix := ""
	if options.DryRun {
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
