package processor

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/security"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/filekit"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreOperationProcessor wraps the restore service to provide detailed logging
type RestoreOperationProcessor struct {
	restoreOperationID    string
	factorySet            *registry.FactorySet
	entityService         *services.EntityService
	uploadLocation        string
	securityValidator     security.SecurityValidator
	importSessionEntities map[string]bool // Track entities created in this session
}

func NewRestoreOperationProcessor(restoreOperationID string, factorySet *registry.FactorySet, entityService *services.EntityService, uploadLocation string) *RestoreOperationProcessor {
	logger := slog.Default()
	return &RestoreOperationProcessor{
		restoreOperationID:    restoreOperationID,
		factorySet:            factorySet,
		entityService:         entityService,
		uploadLocation:        uploadLocation,
		securityValidator:     security.NewRestoreSecurityValidator(factorySet, logger),
		importSessionEntities: make(map[string]bool),
	}
}

func (l *RestoreOperationProcessor) Process(ctx context.Context) error {
	restoreOperationRegistry := l.factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry()
	// Get the restore operation
	restoreOperation, err := restoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get restore operation: %v", err))
	}

	// Get the export to find the file path
	exportReg := l.factorySet.ExportRegistryFactory.CreateServiceRegistry()
	export, err := exportReg.Get(ctx, restoreOperation.ExportID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get export: %v", err))
	}

	user, err := l.factorySet.UserRegistry.Get(ctx, export.UserID)
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
			return errxtrace.Wrap("failed to read token while skipping section", err)
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
			return nil, errxtrace.Wrap("failed to read file token", err)
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
			return errxtrace.Wrap("failed to decode path", err)
		}
	case "originalPath":
		if err := decoder.DecodeElement(&xmlFile.OriginalPath, &t); err != nil {
			return errxtrace.Wrap("failed to decode original path", err)
		}
	case "extension":
		if err := decoder.DecodeElement(&xmlFile.Extension, &t); err != nil {
			return errxtrace.Wrap("failed to decode extension", err)
		}
	case "mimeType":
		if err := decoder.DecodeElement(&xmlFile.MimeType, &t); err != nil {
			return errxtrace.Wrap("failed to decode mime type", err)
		}
	case "data":
		// Stream and decode base64 data
		if err := l.decodeBase64ToFile(ctx, decoder, xmlFile, stats); err != nil {
			return errxtrace.Wrap("failed to decode base64 data", err)
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
			return nil, errxtrace.Wrap("failed to read file element token", err)
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
		return errxtrace.Wrap("failed to open blob bucket", err)
	}
	defer b.Close()

	// Generate unique filename using the same strategy as import service
	filename := filekit.UploadFileName(xmlFile.OriginalPath)

	// Create blob writer
	writer, err := b.NewWriter(ctx, filename, nil)
	if err != nil {
		return errxtrace.Wrap("failed to create blob writer", err)
	}
	defer func() {
		if closeErr := writer.Close(); closeErr != nil {
			err = errxtrace.Wrap("failed to close blob writer", closeErr)
		}
	}()

	// Read base64 data from XML and decode it directly to the blob
	var totalSize int64
	for {
		tok, err := decoder.Token()
		if err != nil {
			return errxtrace.Wrap("failed to read base64 token", err)
		}

		switch t := tok.(type) {
		case xml.CharData:
			// Decode base64 data in chunks
			decoded := make([]byte, len(t))
			n, err := base64.StdEncoding.Decode(decoded, t)
			if err != nil {
				return errxtrace.Wrap("failed to decode base64 data", err)
			}
			if n > 0 {
				if _, err := writer.Write(decoded[:n]); err != nil {
					return errxtrace.Wrap("failed to write decoded data", err)
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
	locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user location registry", err)
	}
	locations, err := locReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing locations", err)
	}
	for _, location := range locations {
		entities.Locations[location.ID] = location
	}

	// Load areas - index by ID (which should be the same as XML ID for imported entities)
	areaReg, err := l.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user area registry", err)
	}
	areas, err := areaReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing areas", err)
	}
	for _, area := range areas {
		entities.Areas[area.ID] = area
	}

	// Load commodities - index by ID (which should be the same as XML ID for imported entities)
	comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user commodity registry", err)
	}
	commodities, err := comReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing commodities", err)
	}
	for _, commodity := range commodities {
		entities.Commodities[commodity.ID] = commodity
	}

	// Load images - index by ID (which should be the same as XML ID for imported entities)
	imgReg, err := l.factorySet.ImageRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user image registry", err)
	}
	images, err := imgReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing images", err)
	}
	for _, image := range images {
		entities.Images[image.ID] = image
	}

	// Load invoices - index by ID (which should be the same as XML ID for imported entities)
	invReg, err := l.factorySet.InvoiceRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user invoice registry", err)
	}
	invoices, err := invReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing invoices", err)
	}
	for _, invoice := range invoices {
		entities.Invoices[invoice.ID] = invoice
	}

	// Load manuals - index by ID (which should be the same as XML ID for imported entities)
	manReg, err := l.factorySet.ManualRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user manual registry", err)
	}
	manuals, err := manReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing manuals", err)
	}
	for _, manual := range manuals {
		entities.Manuals[manual.ID] = manual
	}

	return nil
}

// clearExistingData removes all existing data for full replace strategy
func (l *RestoreOperationProcessor) clearExistingData(ctx context.Context) error {
	// Delete all locations recursively (this will also delete areas and commodities)
	locReg := l.factorySet.LocationRegistryFactory.CreateServiceRegistry()
	locations, err := locReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list locations for deletion", err)
	}
	for _, location := range locations {
		if err := l.entityService.DeleteLocationRecursive(ctx, location.ID); err != nil {
			return errxtrace.Wrap("failed to delete location recursively", err, errx.Attrs("location_id", location.ID))
		}
	}

	return nil
}

// validateCommodityOwnership validates that the user can link files to the specified commodity
func (l *RestoreOperationProcessor) validateCommodityOwnership(
	ctx context.Context,
	commodityID, originalXMLID string,
	stats *types.RestoreStats,
	options types.RestoreOptions,
	fileType string,
) (string, error) {
	// For merge strategies, validate commodity ownership before linking
	if options.Strategy == types.RestoreStrategyMergeAdd || options.Strategy == types.RestoreStrategyMergeUpdate {
		err := l.securityValidator.ValidateImportScope(ctx, commodityID, l.restoreOperationID, l.importSessionEntities)
		if err != nil {
			// If entity not found, upload file as orphaned (not linked to any commodity)
			if errors.Is(err, registry.ErrNotFound) {
				stats.ErrorCount++
				stats.Errors = append(stats.Errors, fmt.Sprintf("%s %s uploaded as orphaned file: %v", fileType, originalXMLID, err))
				// Return empty commodityID to create orphaned file
				return "", nil
			}
			// For other security violations, fail completely
			stats.ErrorCount++
			stats.Errors = append(stats.Errors, fmt.Sprintf("Security validation failed for %s %s: %v", fileType, originalXMLID, err))
			return "", err
		}
	}
	return commodityID, nil
}

// validateCommodityOwnershipInDB validates that a commodity in the database belongs to the current user
func (l *RestoreOperationProcessor) validateCommodityOwnershipInDB(
	ctx context.Context,
	originalXMLID string,
	currentUser *models.User,
	existing *types.ExistingEntities,
	stats *types.RestoreStats,
) error {
	existingCommodity := existing.Commodities[originalXMLID]
	if existingCommodity != nil {
		return nil // Already validated in existing entities
	}

	// Check if commodity exists in database but not in our existing entities map
	// Use service account registry to see all entities across users
	serviceAccountRegistry := l.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
	existingDBCommodity, err := serviceAccountRegistry.Get(ctx, originalXMLID)
	if err != nil {
		return nil // Commodity doesn't exist in DB, which is fine
	}

	if existingDBCommodity.UserID != currentUser.ID {
		err := l.securityValidator.ValidateEntityOwnership(ctx, originalXMLID, currentUser.ID)
		if err != nil {
			stats.ErrorCount++
			stats.Errors = append(stats.Errors, fmt.Sprintf("Security validation failed for commodity %s: %v", originalXMLID, err))
			return err
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
	// Security validation: Check if user can link files to this commodity
	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}

	// Validate commodity ownership and handle security violations
	validatedCommodityID, err := l.validateCommodityOwnership(ctx, commodityID, originalXMLID, stats, options, "Image")
	if err != nil {
		return err
	}
	commodityID = validatedCommodityID

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
			return errxtrace.Wrap("invalid image", err)
		}
		imgReg := l.factorySet.ImageRegistryFactory.CreateServiceRegistry()
		createdImage, err := imgReg.Create(ctx, *image)
		if err != nil {
			return errxtrace.Wrap("failed to create image", err)
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
			return errxtrace.Wrap("invalid image", err)
		}
		imgReg := l.factorySet.ImageRegistryFactory.CreateServiceRegistry()
		createdImage, err := imgReg.Create(ctx, *image)
		if err != nil {
			return errxtrace.Wrap("failed to create image", err)
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
					return errxtrace.Wrap("invalid image", err)
				}
				imgReg := l.factorySet.ImageRegistryFactory.CreateServiceRegistry()
				createdImage, err := imgReg.Create(ctx, *image)
				if err != nil {
					return errxtrace.Wrap("failed to create image", err)
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
			return errxtrace.Wrap("invalid image", err)
		}
		imgReg := l.factorySet.ImageRegistryFactory.CreateServiceRegistry()
		updatedImage, err := imgReg.Update(ctx, *image)
		if err != nil {
			return errxtrace.Wrap("failed to update image", err)
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
	// Security validation: Check if user can link files to this commodity
	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}

	// Validate commodity ownership and handle security violations
	validatedCommodityID, err := l.validateCommodityOwnership(ctx, commodityID, originalXMLID, stats, options, "Invoice")
	if err != nil {
		return err
	}
	commodityID = validatedCommodityID

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
			return errxtrace.Wrap("invalid invoice", err)
		}
		invReg := l.factorySet.InvoiceRegistryFactory.CreateServiceRegistry()
		createdInvoice, err := invReg.Create(ctx, *invoice)
		if err != nil {
			return errxtrace.Wrap("failed to create invoice", err)
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
			return errxtrace.Wrap("invalid invoice", err)
		}
		invReg := l.factorySet.InvoiceRegistryFactory.CreateServiceRegistry()
		createdInvoice, err := invReg.Create(ctx, *invoice)
		if err != nil {
			return errxtrace.Wrap("failed to create invoice", err)
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
					return errxtrace.Wrap("invalid invoice", err)
				}
				invReg := l.factorySet.InvoiceRegistryFactory.CreateServiceRegistry()
				createdInvoice, err := invReg.Create(ctx, *invoice)
				if err != nil {
					return errxtrace.Wrap("failed to create invoice", err)
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
			return errxtrace.Wrap("invalid invoice", err)
		}
		invReg := l.factorySet.InvoiceRegistryFactory.CreateServiceRegistry()
		updatedInvoice, err := invReg.Update(ctx, *invoice)
		if err != nil {
			return errxtrace.Wrap("failed to update invoice", err)
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
	// Security validation: Check if user can link files to this commodity
	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}

	// Validate commodity ownership and handle security violations
	validatedCommodityID, err := l.validateCommodityOwnership(ctx, commodityID, originalXMLID, stats, options, "Manual")
	if err != nil {
		return err
	}
	commodityID = validatedCommodityID

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
			return errxtrace.Wrap("invalid manual", err)
		}
		manReg := l.factorySet.ManualRegistryFactory.CreateServiceRegistry()
		createdManual, err := manReg.Create(ctx, *manual)
		if err != nil {
			return errxtrace.Wrap("failed to create manual", err)
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
			return errxtrace.Wrap("invalid manual", err)
		}
		manReg := l.factorySet.ManualRegistryFactory.CreateServiceRegistry()
		createdManual, err := manReg.Create(ctx, *manual)
		if err != nil {
			return errxtrace.Wrap("failed to create manual", err)
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
				return errxtrace.Wrap("invalid manual", err)
			}
			manReg := l.factorySet.ManualRegistryFactory.CreateServiceRegistry()
			createdManual, err := manReg.Create(ctx, *manual)
			if err != nil {
				return errxtrace.Wrap("failed to create manual", err)
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
			return errxtrace.Wrap("invalid manual", err)
		}
		manReg := l.factorySet.ManualRegistryFactory.CreateServiceRegistry()
		updatedManual, err := manReg.Update(ctx, *manual)
		if err != nil {
			return errxtrace.Wrap("failed to update manual", err)
		}
		// Update the tracked manual
		existing.Manuals[originalXMLID] = updatedManual
		stats.UpdatedCount++
	}

	return nil
}

// trackCreatedEntity tracks entities created in this import session for security validation
func (l *RestoreOperationProcessor) trackCreatedEntity(entityID string) {
	if l.importSessionEntities == nil {
		l.importSessionEntities = make(map[string]bool)
	}
	l.importSessionEntities[entityID] = true
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
			return errxtrace.Wrap("failed to create image record", err)
		}
	case "invoice":
		err := l.createInvoiceRecord(ctx, file, commodityID, originalXMLID, stats, existing, idMapping, options)
		if err != nil {
			return errxtrace.Wrap("failed to create invoice record", err)
		}
	case "manual":
		err := l.createManualRecord(ctx, file, commodityID, originalXMLID, stats, existing, idMapping, options)
		if err != nil {
			return errxtrace.Wrap("failed to create manual record", err)
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

	stepReg := l.factorySet.RestoreStepRegistryFactory.CreateServiceRegistry()
	_, err := stepReg.Create(ctx, step)
	if err != nil {
		// Log error but don't fail the restore operation
		slog.Error("Failed to create restore step", "error", err)
	}
}

// updateRestoreStep updates an existing restore step
func (l *RestoreOperationProcessor) updateRestoreStep(ctx context.Context, name string, result models.RestoreStepResult, reason string) {
	// Get all steps for this restore operation
	stepReg := l.factorySet.RestoreStepRegistryFactory.CreateServiceRegistry()
	steps, err := stepReg.ListByRestoreOperation(ctx, l.restoreOperationID)
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
			stepReg := l.factorySet.RestoreStepRegistryFactory.CreateServiceRegistry()
			_, err := stepReg.Update(ctx, *step)
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
	restoreOperationRegistry := l.factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry()

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
			return errxtrace.Wrap("failed to process locations", err)
		}
		l.updateRestoreStep(ctx, "Processing locations", models.RestoreStepResultSuccess,
			fmt.Sprintf("Processed %d locations", stats.LocationCount))
	case "areas":
		l.createRestoreStep(ctx, "Processing areas", models.RestoreStepResultInProgress, "")
		if err := l.processAreasWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
			l.updateRestoreStep(ctx, "Processing areas", models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to process areas", err)
		}
		l.updateRestoreStep(ctx, "Processing areas", models.RestoreStepResultSuccess,
			fmt.Sprintf("Processed %d areas", stats.AreaCount))
	case "commodities":
		l.createRestoreStep(ctx, "Processing commodities", models.RestoreStepResultInProgress, "")
		if err := l.processCommoditiesWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
			l.updateRestoreStep(ctx, "Processing commodities", models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to process commodities", err)
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
		return stats, errxtrace.Wrap("invalid restore options", err)
	}

	// Get main currency from settings and add it to context for commodity validation
	// Use user-aware settings registry to get user-specific settings
	settingsReg, err := l.factorySet.SettingsRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return stats, errxtrace.Wrap("failed to create user settings registry", err)
	}
	settings, err := settingsReg.Get(ctx)
	if err != nil {
		return stats, errxtrace.Wrap("failed to get settings", err)
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
			return stats, errxtrace.Wrap("failed to load existing entities", err)
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
			return stats, errxtrace.Wrap("failed to clear existing data", err)
		}
	}

	// Process XML stream with logging
	for {
		tok, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return stats, errxtrace.Wrap("failed to read XML token", err)
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
			return stats, errxtrace.ClassifyNew("unexpected token type", errx.Attrs("token_type", fmt.Sprintf("%T", t)))
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
			return errxtrace.Wrap("failed to read locations token", err)
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
		return errxtrace.Wrap("failed to decode location", err)
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
		return errxtrace.Wrap("invalid location", err, errx.Attrs("location_id", location.ID))
	}

	// Store the original XML ID for mapping
	originalXMLID := xmlLocation.ID

	// Apply strategy
	existingLocation := existing.Locations[originalXMLID]
	err := l.applyStrategyForLocation(ctx, location, existingLocation, originalXMLID, stats, existing, idMapping, options, emoji, &xmlLocation)
	if err != nil {
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
		return errxtrace.Wrap("failed to apply strategy for location", err)
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
			return errxtrace.Wrap("failed to read areas token", err)
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

// applyStrategyForLocation applies the restore strategy for a location
func (l *RestoreOperationProcessor) applyStrategyForLocation(
	ctx context.Context,
	location *models.Location,
	existingLocation *models.Location,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
	emoji string,
	xmlLocation *types.XMLLocation,
) error {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		return l.handleLocationFullReplace(ctx, location, originalXMLID, stats, existing, idMapping, options, emoji, xmlLocation)
	case types.RestoreStrategyMergeAdd:
		return l.handleLocationMergeAdd(ctx, location, existingLocation, originalXMLID, stats, existing, idMapping, options, emoji, xmlLocation)
	case types.RestoreStrategyMergeUpdate:
		return l.handleLocationMergeUpdate(ctx, location, existingLocation, originalXMLID, stats, existing, idMapping, options, emoji, xmlLocation)
	}
	return nil
}

// handleLocationFullReplace handles full replace strategy for locations
func (l *RestoreOperationProcessor) handleLocationFullReplace(
	ctx context.Context,
	location *models.Location,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
	emoji string,
	xmlLocation *types.XMLLocation,
) error {
	if !options.DryRun {
		locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user location registry", err)
		}
		createdLocation, err := locReg.Create(ctx, *location)
		if err != nil {
			l.updateRestoreStep(ctx,
				fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to create location", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Locations[originalXMLID] = createdLocation
		idMapping.Locations[originalXMLID] = createdLocation.ID
		l.trackCreatedEntity(createdLocation.ID)
	}
	stats.CreatedCount++
	stats.LocationCount++
	return nil
}

// handleLocationMergeAdd handles merge add strategy for locations
func (l *RestoreOperationProcessor) handleLocationMergeAdd(
	ctx context.Context,
	location *models.Location,
	existingLocation *models.Location,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
	emoji string,
	xmlLocation *types.XMLLocation,
) error {
	if existingLocation != nil {
		stats.SkippedCount++
		return nil
	}
	if !options.DryRun {
		locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user location registry", err)
		}
		createdLocation, err := locReg.Create(ctx, *location)
		if err != nil {
			l.updateRestoreStep(ctx,
				fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to create location", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Locations[originalXMLID] = createdLocation
		idMapping.Locations[originalXMLID] = createdLocation.ID
		l.trackCreatedEntity(createdLocation.ID)
	}
	stats.CreatedCount++
	stats.LocationCount++
	return nil
}

// handleLocationMergeUpdate handles merge update strategy for locations
func (l *RestoreOperationProcessor) handleLocationMergeUpdate(
	ctx context.Context,
	location *models.Location,
	existingLocation *models.Location,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
	emoji string,
	xmlLocation *types.XMLLocation,
) error {
	if existingLocation == nil {
		if !options.DryRun {
			locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
			if err != nil {
				return errxtrace.Wrap("failed to create user location registry", err)
			}
			createdLocation, err := locReg.Create(ctx, *location)
			if err != nil {
				l.updateRestoreStep(ctx,
					fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
				return errxtrace.Wrap("failed to create location", err, errx.Attrs("xml_id", originalXMLID))
			}
			existing.Locations[originalXMLID] = createdLocation
			idMapping.Locations[originalXMLID] = createdLocation.ID
			l.trackCreatedEntity(createdLocation.ID)
		}
		stats.CreatedCount++
		stats.LocationCount++
		return nil
	}

	// Update existing location
	location.ID = existingLocation.ID
	if !options.DryRun {
		locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user location registry", err)
		}
		updatedLocation, err := locReg.Update(ctx, *location)
		if err != nil {
			l.updateRestoreStep(ctx,
				fmt.Sprintf("%s Location: %s", emoji, xmlLocation.LocationName), models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to update location", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Locations[originalXMLID] = updatedLocation
	}
	stats.UpdatedCount++
	stats.LocationCount++
	return nil
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
		return l.handleAreaFullReplace(ctx, area, originalXMLID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeAdd:
		return l.handleAreaMergeAdd(ctx, area, existingArea, originalXMLID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeUpdate:
		return l.handleAreaMergeUpdate(ctx, area, existingArea, originalXMLID, stats, existing, idMapping, options)
	}
	return nil
}

// handleAreaFullReplace handles full replace strategy for areas
func (l *RestoreOperationProcessor) handleAreaFullReplace(
	ctx context.Context,
	area *models.Area,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	return l.createAreaIfNotDryRun(ctx, area, originalXMLID, stats, existing, idMapping, options)
}

// handleAreaMergeAdd handles merge add strategy for areas
func (l *RestoreOperationProcessor) handleAreaMergeAdd(
	ctx context.Context,
	area *models.Area,
	existingArea *models.Area,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if existingArea != nil {
		stats.SkippedCount++
		return nil
	}

	return l.createAreaIfNotDryRun(ctx, area, originalXMLID, stats, existing, idMapping, options)
}

// createAreaIfNotDryRun creates an area if not in dry run mode
func (l *RestoreOperationProcessor) createAreaIfNotDryRun(
	ctx context.Context,
	area *models.Area,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		areaReg, err := l.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user area registry", err)
		}
		createdArea, err := areaReg.Create(ctx, *area)
		if err != nil {
			return errxtrace.Wrap("failed to create area", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Areas[originalXMLID] = createdArea
		idMapping.Areas[originalXMLID] = createdArea.ID
		l.trackCreatedEntity(createdArea.ID)
	}
	stats.CreatedCount++
	stats.AreaCount++
	return nil
}

// handleAreaMergeUpdate handles merge update strategy for areas
func (l *RestoreOperationProcessor) handleAreaMergeUpdate(
	ctx context.Context,
	area *models.Area,
	existingArea *models.Area,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if existingArea == nil {
		return l.createAreaIfNotDryRun(ctx, area, originalXMLID, stats, existing, idMapping, options)
	}

	// Update existing area
	if !options.DryRun {
		areaReg, err := l.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user area registry", err)
		}
		updatedArea, err := areaReg.Update(ctx, *area)
		if err != nil {
			return errxtrace.Wrap("failed to update area", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Areas[originalXMLID] = updatedArea
	}
	stats.UpdatedCount++
	stats.AreaCount++
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
		return errxtrace.Wrap("failed to decode area", err)
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
		return errxtrace.Wrap("invalid area", err, errx.Attrs("xml_id", originalXMLID))
	}

	// Apply strategy
	existingArea := existing.Areas[originalXMLID]
	err := l.applyStrategyForArea(ctx, area, existingArea, originalXMLID, stats, existing, idMapping, options, emoji, &xmlArea)
	if err != nil {
		l.updateRestoreStep(ctx,
			fmt.Sprintf("%s Area: %s", emoji, xmlArea.AreaName), models.RestoreStepResultError, err.Error())
		return errxtrace.Wrap("failed to apply strategy for area", err)
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
			return errxtrace.Wrap("failed to read commodities token", err)
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
			return errxtrace.Wrap("failed to decode commodity name", err)
		}
		// Update step description with actual commodity name
		l.updateRestoreStep(ctx, stepName, models.RestoreStepResultInProgress,
			fmt.Sprintf("Processing %s", xmlCommodity.CommodityName))
	case "shortName":
		if err := decoder.DecodeElement(&xmlCommodity.ShortName, &t); err != nil {
			return errxtrace.Wrap("failed to decode short name", err)
		}
	case "areaId":
		if err := decoder.DecodeElement(&xmlCommodity.AreaID, &t); err != nil {
			return errxtrace.Wrap("failed to decode area ID", err)
		}
	case "type":
		if err := decoder.DecodeElement(&xmlCommodity.Type, &t); err != nil {
			return errxtrace.Wrap("failed to decode type", err)
		}
	case "count":
		if err := decoder.DecodeElement(&xmlCommodity.Count, &t); err != nil {
			return errxtrace.Wrap("failed to decode count", err)
		}
	case "status":
		if err := decoder.DecodeElement(&xmlCommodity.Status, &t); err != nil {
			return errxtrace.Wrap("failed to decode status", err)
		}
	case "originalPrice":
		if err := decoder.DecodeElement(&xmlCommodity.OriginalPrice, &t); err != nil {
			return errxtrace.Wrap("failed to decode original price", err)
		}
	case "originalPriceCurrency":
		if err := decoder.DecodeElement(&xmlCommodity.OriginalCurrency, &t); err != nil {
			return errxtrace.Wrap("failed to decode original price currency", err)
		}
	case "convertedOriginalPrice":
		if err := decoder.DecodeElement(&xmlCommodity.ConvertedOriginalPrice, &t); err != nil {
			return errxtrace.Wrap("failed to decode converted original price", err)
		}
	case "currentPrice":
		if err := decoder.DecodeElement(&xmlCommodity.CurrentPrice, &t); err != nil {
			return errxtrace.Wrap("failed to decode current price", err)
		}
	case "currentCurrency":
		if err := decoder.DecodeElement(&xmlCommodity.CurrentCurrency, &t); err != nil {
			return errxtrace.Wrap("failed to decode current currency", err)
		}
	case "serialNumber":
		if err := decoder.DecodeElement(&xmlCommodity.SerialNumber, &t); err != nil {
			return errxtrace.Wrap("failed to decode serial number", err)
		}
	case "extraSerialNumbers":
		if err := decoder.DecodeElement(&xmlCommodity.ExtraSerialNumbers, &t); err != nil {
			return errxtrace.Wrap("failed to decode extra serial numbers", err)
		}
	case "comments":
		if err := decoder.DecodeElement(&xmlCommodity.Comments, &t); err != nil {
			return errxtrace.Wrap("failed to decode comments", err)
		}
	case "draft":
		if err := decoder.DecodeElement(&xmlCommodity.Draft, &t); err != nil {
			return errxtrace.Wrap("failed to decode draft", err)
		}
	case "purchaseDate":
		if err := decoder.DecodeElement(&xmlCommodity.PurchaseDate, &t); err != nil {
			return errxtrace.Wrap("failed to decode purchase date", err)
		}
	case "registeredDate":
		if err := decoder.DecodeElement(&xmlCommodity.RegisteredDate, &t); err != nil {
			return errxtrace.Wrap("failed to decode registered date", err)
		}
	case "lastModifiedDate":
		if err := decoder.DecodeElement(&xmlCommodity.LastModifiedDate, &t); err != nil {
			return errxtrace.Wrap("failed to decode last modified date", err)
		}
	case "partNumbers":
		if err := decoder.DecodeElement(&xmlCommodity.PartNumbers, &t); err != nil {
			return errxtrace.Wrap("failed to decode part numbers", err)
		}
	case "tags":
		if err := decoder.DecodeElement(&xmlCommodity.Tags, &t); err != nil {
			return errxtrace.Wrap("failed to decode tags", err)
		}
	case "urls":
		if err := decoder.DecodeElement(&xmlCommodity.URLs, &t); err != nil {
			return errxtrace.Wrap("failed to decode URLs", err)
		}
	case "images":
		if options.IncludeFileData {
			files, err := l.collectFiles(ctx, decoder, &t, stats)
			if err != nil {
				return errxtrace.Wrap("failed to collect images", err)
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
				return errxtrace.Wrap("failed to skip images section", err)
			}
		}
	case "invoices":
		if options.IncludeFileData {
			files, err := l.collectFiles(ctx, decoder, &t, stats)
			if err != nil {
				return errxtrace.Wrap("failed to collect invoices", err)
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
				return errxtrace.Wrap("failed to skip invoices section", err)
			}
		}
	case "manuals":
		if options.IncludeFileData {
			files, err := l.collectFiles(ctx, decoder, &t, stats)
			if err != nil {
				return errxtrace.Wrap("failed to collect manuals", err)
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
				return errxtrace.Wrap("failed to skip manuals section", err)
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
			return errxtrace.Wrap("failed to read commodity element token", err)
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
		return l.handleCommodityFullReplace(ctx, commodity, originalXMLID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeAdd:
		return l.handleCommodityMergeAdd(ctx, commodity, existingCommodity, originalXMLID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeUpdate:
		return l.handleCommodityMergeUpdate(ctx, commodity, existingCommodity, originalXMLID, stats, existing, idMapping, options)
	}
	return nil
}

// handleCommodityFullReplace handles full replace strategy for commodities
func (l *RestoreOperationProcessor) handleCommodityFullReplace(
	ctx context.Context,
	commodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	return l.createCommodityIfNotDryRun(ctx, commodity, originalXMLID, stats, existing, idMapping, options)
}

// handleCommodityMergeAdd handles merge add strategy for commodities
func (l *RestoreOperationProcessor) handleCommodityMergeAdd(
	ctx context.Context,
	commodity *models.Commodity,
	existingCommodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if existingCommodity != nil {
		stats.SkippedCount++
		return nil
	}

	return l.createCommodityIfNotDryRun(ctx, commodity, originalXMLID, stats, existing, idMapping, options)
}

// createCommodityIfNotDryRun creates a commodity if not in dry run mode
func (l *RestoreOperationProcessor) createCommodityIfNotDryRun(
	ctx context.Context,
	commodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user commodity registry", err)
		}
		createdCommodity, err := comReg.Create(ctx, *commodity)
		if err != nil {
			return errxtrace.Wrap("failed to create commodity", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Commodities[originalXMLID] = createdCommodity
		idMapping.Commodities[originalXMLID] = createdCommodity.ID
		l.trackCreatedEntity(createdCommodity.ID)
	}
	stats.CreatedCount++
	stats.CommodityCount++
	return nil
}

// handleCommodityMergeUpdate handles merge update strategy for commodities
func (l *RestoreOperationProcessor) handleCommodityMergeUpdate(
	ctx context.Context,
	commodity *models.Commodity,
	existingCommodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if existingCommodity == nil {
		return l.createCommodityForMergeUpdate(ctx, commodity, originalXMLID, stats, existing, idMapping, options)
	}

	return l.updateExistingCommodity(ctx, commodity, originalXMLID, stats, existing, options)
}

// createCommodityForMergeUpdate creates a new commodity during merge update
func (l *RestoreOperationProcessor) createCommodityForMergeUpdate(
	ctx context.Context,
	commodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	return l.createCommodityIfNotDryRun(ctx, commodity, originalXMLID, stats, existing, idMapping, options)
}

// updateExistingCommodity updates an existing commodity
func (l *RestoreOperationProcessor) updateExistingCommodity(
	ctx context.Context,
	commodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user commodity registry", err)
		}
		updatedCommodity, err := comReg.Update(ctx, *commodity)
		if err != nil {
			return errxtrace.Wrap("failed to update commodity", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Commodities[originalXMLID] = updatedCommodity
	}
	stats.UpdatedCount++
	stats.CommodityCount++
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
		return errxtrace.ClassifyNew("commodity references non-existent area", errx.Attrs(
			"original_commodity_id", originalXMLID,
			"original_area_id", originalAreaXMLID,
		))
	}

	// Get the actual database area ID
	actualAreaID := idMapping.Areas[originalAreaXMLID]
	if actualAreaID == "" {
		return errxtrace.ClassifyNew("no ID mapping found for area", errx.Attrs("original_area_id", originalAreaXMLID))
	}

	commodity, err := xmlCommodity.ConvertToCommodity()
	if err != nil {
		return errxtrace.Wrap("failed to convert commodity", err, errx.Attrs("original_commodity_id", originalXMLID))
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
		return errxtrace.Wrap("invalid commodity", err, errx.Attrs("original_commodity_id", originalXMLID))
	}

	// Security validation: Check if user is trying to use an existing commodity ID that belongs to another user
	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}

	// For all strategies, check if the commodity ID already exists and belongs to another user
	err = l.validateCommodityOwnershipInDB(ctx, originalXMLID, currentUser, existing, stats)
	if err != nil {
		return err
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
		locReg := l.factorySet.LocationRegistryFactory.CreateServiceRegistry()
		_, err := locReg.Get(ctx, entityID)
		return err == nil
	case "area":
		areaReg := l.factorySet.AreaRegistryFactory.CreateServiceRegistry()
		_, err := areaReg.Get(ctx, entityID)
		return err == nil
	case "commodity":
		comReg := l.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
		_, err := comReg.Get(ctx, entityID)
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
