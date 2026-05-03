package processor

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/shopspring/decimal"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/security"
	"github.com/denisvmedia/inventario/backup/restore/types"
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
	tagService            *services.TagService
	uploadLocation        string
	securityValidator     security.SecurityValidator
	importSessionEntities map[string]bool // Track entities created in this session

	// commodityUUIDMap is a lazy-loaded cache of all pre-existing commodities keyed
	// by their immutable UUID. It is populated once on the first call to
	// validateCommodityOwnershipInDB, replacing repeated O(N) List() calls with a
	// single O(N) build followed by O(1) lookups.
	commodityUUIDMap map[string]*models.Commodity
}

func NewRestoreOperationProcessor(restoreOperationID string, factorySet *registry.FactorySet, entityService *services.EntityService, uploadLocation string) *RestoreOperationProcessor {
	logger := slog.Default()
	return &RestoreOperationProcessor{
		restoreOperationID:    restoreOperationID,
		factorySet:            factorySet,
		entityService:         entityService,
		tagService:            services.NewTagService(factorySet),
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

	user, err := l.factorySet.UserRegistry.Get(ctx, export.CreatedByUserID)
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
	restoreOperation.FileCount = stats.FileCount
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

// Legacy XML-attachment helpers (collectFiles / collectFile / processFile /
// decodeBase64ToFile / validateCommodityOwnership) were removed under #1421
// along with the legacy SQL tables they ultimately wrote to. The silent-skip
// helper that handled pre-cutover backups was dropped under #1485 along with
// the case in collectCommodityData — pre-cutover archives are explicitly not
// supported.

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

func (l *RestoreOperationProcessor) loadExistingEntities(ctx context.Context, entities *types.ExistingEntities) error {
	entities.Locations = make(map[string]*models.Location)
	entities.Areas = make(map[string]*models.Area)
	entities.Commodities = make(map[string]*models.Commodity)
	entities.Files = make(map[string]*models.FileEntity)

	// Load locations - index by immutable UUID (the stable XML identifier used in exports).
	locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user location registry", err)
	}
	locations, err := locReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing locations", err)
	}
	for _, location := range locations {
		entities.Locations[location.UUID] = location
	}

	// Load areas - index by immutable UUID.
	areaReg, err := l.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user area registry", err)
	}
	areas, err := areaReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing areas", err)
	}
	for _, area := range areas {
		entities.Areas[area.UUID] = area
	}

	// Load commodities - index by immutable UUID.
	comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user commodity registry", err)
	}
	commodities, err := comReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing commodities", err)
	}
	for _, commodity := range commodities {
		entities.Commodities[commodity.UUID] = commodity
	}

	// Load files - index by immutable UUID. Excludes export-linked files
	// (the export's own backup-bundle blobs); those aren't part of the
	// content surface that round-trips through restore and creating a
	// duplicate row for them would conflict with the export.FileID FK.
	fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user file registry", err)
	}
	files, err := fileReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to load existing files", err)
	}
	for _, file := range files {
		if file.LinkedEntityType == "export" {
			continue
		}
		entities.Files[file.UUID] = file
	}

	return nil
}

// clearExistingData removes all existing data for full replace strategy.
// DeleteLocationRecursive cascades through areas + commodities and the
// commodity step also deletes commodity-linked files (rows + physical
// blobs + thumbnails) via EntityService — but location-, area-, and
// standalone-linked files don't cascade and would remain as orphans
// (and worse: their UUIDs would collide with the restored rows, while
// their physical blobs would leak in the upload bucket forever). After
// the location sweep we make a second pass that calls FileService's
// row+blob delete for every non-export file.
//
// Export-linked files (the export's own backup-bundle blobs) are
// intentionally preserved — deleting them mid-restore would race with
// the export.FileID FK pointing at the bundle we're currently reading
// from.
func (l *RestoreOperationProcessor) clearExistingData(ctx context.Context) error {
	// Delete all locations recursively (this will also delete areas, commodities, and commodity-linked files+blobs).
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

	// Sweep remaining files (location-/area-linked + standalone) along
	// with their physical blobs and thumbnails — see the comment above
	// for why DeleteLocationRecursive isn't enough on its own.
	fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user file registry for clear", err)
	}
	files, err := fileReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list files for deletion", err)
	}
	fileService := services.NewFileService(l.factorySet, l.uploadLocation)
	for _, file := range files {
		if file.LinkedEntityType == "export" {
			continue
		}
		if err := fileService.DeleteFileWithPhysical(ctx, file.ID); err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				continue
			}
			return errxtrace.Wrap("failed to delete file row+blob", err, errx.Attrs("file_id", file.ID))
		}
	}

	return nil
}

// validateCommodityOwnership validates that the user can link files to the specified commodity

// validateCommodityOwnershipInDB validates that a commodity in the database belongs to the current user.
//
// The UUID→commodity index is built lazily on the first invocation and reused for all subsequent calls,
// reducing the per-commodity O(N) List() pattern to a single O(N) build followed by O(1) lookups.
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

	// Build the UUID→commodity cache on first access.
	// We use the service account registry (bypasses tenant/user filtering) so that
	// commodities belonging to other users are also visible for ownership checks.
	if l.commodityUUIDMap == nil {
		serviceAccountRegistry := l.factorySet.CommodityRegistryFactory.CreateServiceRegistry()
		allCommodities, err := serviceAccountRegistry.List(ctx)
		if err != nil {
			stats.ErrorCount++
			msg := fmt.Sprintf("Failed to list commodities for ownership validation of %s: %v", originalXMLID, err)
			stats.Errors = append(stats.Errors, msg)
			slog.Error("commodity ownership validation list failed",
				"restoreOperationID", l.restoreOperationID,
				"originalXMLID", originalXMLID,
				"error", err,
			)
			return err
		}
		l.commodityUUIDMap = make(map[string]*models.Commodity, len(allCommodities))
		for _, c := range allCommodities {
			l.commodityUUIDMap[c.UUID] = c
		}
	}

	existingDBCommodity := l.commodityUUIDMap[originalXMLID]
	if existingDBCommodity == nil {
		return nil // No existing commodity with this UUID; creating a new one is fine
	}

	if existingDBCommodity.CreatedByUserID != currentUser.ID {
		// Ownership mismatch is already confirmed from the UUID map; log the audit event
		// directly using the DB primary key so the audit trail references the correct entity.
		l.securityValidator.LogUnauthorizedAttempt(ctx, security.UnauthorizedAttempt{
			UserID:         currentUser.ID,
			TargetEntityID: existingDBCommodity.ID,
			EntityType:     "commodity",
			Operation:      "restore",
			AttemptType:    "cross_user_access",
			Timestamp:      time.Now(),
		})
		stats.ErrorCount++
		stats.Errors = append(stats.Errors, fmt.Sprintf("Security validation failed for commodity %s: commodity belongs to a different user", originalXMLID))
		// Return the sentinel so callers can distinguish ownership violations from operational DB errors.
		return errxtrace.Classify(security.ErrOwnershipViolation, errx.Attrs("xml_id", originalXMLID, "commodity_id", existingDBCommodity.ID))
	}

	return nil
}

// Legacy createImageRecord / createInvoiceRecord / createManualRecord were
// removed under #1421 along with the SQL tables they wrote to. Commodity
// attachment restore now belongs to the unified `files` surface — that work
// is tracked separately. The XML <images>/<invoices>/<manuals> sections in
// older backups are silently skipped during restore.

// trackCreatedEntity tracks entities created in this import session for security validation
func (l *RestoreOperationProcessor) trackCreatedEntity(entityID string) {
	if l.importSessionEntities == nil {
		l.importSessionEntities = make(map[string]bool)
	}
	l.importSessionEntities[entityID] = true
}

// createFileRecord creates a file record in the appropriate registry with strategy support
// createFileRecord was the dispatcher for the three legacy create*Record
// functions removed under #1421. It is gone too — see processCommodityData
// for the no-op path that replaces it.

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
	case "files":
		l.createRestoreStep(ctx, "Processing files", models.RestoreStepResultInProgress, "")
		if err := l.processFilesWithLogging(ctx, decoder, stats, existingEntities, idMapping, options); err != nil {
			l.updateRestoreStep(ctx, "Processing files", models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to process files", err)
		}
		l.updateRestoreStep(ctx, "Processing files", models.RestoreStepResultSuccess,
			fmt.Sprintf("Processed %d files", stats.FileCount))
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

// restoreFromXML processes the restore with detailed logging using streaming approach.
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

	// Get main currency from the group in context and add it to the validation
	// context for commodity validation. The restore runs scoped to a single
	// group; its currency is the only currency that matters here.
	if group := appctx.GroupFromContext(ctx); group != nil && group.MainCurrency != "" {
		ctx = validationctx.WithMainCurrency(ctx, string(group.MainCurrency))
	}

	decoder := xml.NewDecoder(xmlReader)

	// Track existing entities for validation and strategy decisions
	existingEntities := &types.ExistingEntities{}
	idMapping := &types.IDMapping{
		Locations:   make(map[string]string),
		Areas:       make(map[string]string),
		Commodities: make(map[string]string),
		Files:       make(map[string]string),
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
		for xmlID, entity := range existingEntities.Files {
			idMapping.Files[xmlID] = entity.ID
		}
	} else {
		// For full replace, initialize empty maps to track newly created entities
		existingEntities.Locations = make(map[string]*models.Location)
		existingEntities.Areas = make(map[string]*models.Area)
		existingEntities.Commodities = make(map[string]*models.Commodity)
		existingEntities.Files = make(map[string]*models.FileEntity)
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
		// Preserve the immutable UUID from XML so the entity is stable across restores.
		location.UUID = originalXMLID
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
		// Preserve the immutable UUID from XML so the entity is stable across restores.
		location.UUID = originalXMLID
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
			// Preserve the immutable UUID from XML so the entity is stable across restores.
			location.UUID = originalXMLID
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

	// Update existing location: restore the DB ID and preserve the immutable UUID.
	location.ID = existingLocation.ID
	location.UUID = existingLocation.UUID
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
		// Preserve the immutable UUID from XML so the entity is stable across restores.
		area.UUID = originalXMLID
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

	// Update existing area: restore the DB ID and preserve the immutable UUID.
	area.SetID(existingArea.ID)
	area.UUID = existingArea.UUID
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
	case "images", "invoices", "manuals":
		// Legacy commodity-scoped attachment sections were removed under
		// #1421 along with the SQL tables they wrote to. Pre-cutover
		// archives are not supported per ops (no production data
		// pre-dates the cutover) — but a pre-cutover backup walked
		// through the decoder would otherwise just silently drop its
		// attachment data while reporting ErrorCount=0. We loud-fail
		// here so the operator notices: the rest of the restore proceeds
		// (commodities and other top-level sections still get their
		// chance), but the operation ends with a clear error in
		// stats.Errors and a nonzero ErrorCount.
		stats.ErrorCount++
		stats.Errors = append(stats.Errors, fmt.Sprintf(
			"unsupported pre-cutover attachment section <%s> on commodity %s; restore the data via a backup created after PR #1485 instead",
			t.Name.Local, xmlCommodity.ID,
		))
		// Skip the section's body so the surrounding <commodity> loop
		// doesn't trip on the children as unknown elements.
		if err := skipElement(decoder, t.Name.Local); err != nil {
			return errxtrace.Wrap("failed to skip legacy attachment section", err)
		}
	}

	return nil
}

// skipElement walks the decoder until the matching </name> closes the
// element identified by `name`. Used for explicit skip-with-error paths
// where we want to consume the unknown element's body and continue
// rather than abort.
func skipElement(decoder *xml.Decoder, name string) error {
	depth := 1
	for depth > 0 {
		tok, err := decoder.Token()
		if err != nil {
			return errxtrace.Wrap("failed to read token while skipping element", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 0 && t.Name.Local == name {
				return nil
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
) error {
	return l.createOrUpdateCommodity(ctx, xmlCommodity, stats, existing, idMapping, options)
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
			err = l.collectCommodityData(ctx, stepName, t, decoder, stats, &xmlCommodity, options)
			if err != nil {
				l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, err.Error())
				return err
			}
		case xml.EndElement:
			if t.Name.Local != "commodity" {
				continue
			}

			err = l.processCommodityData(ctx, &xmlCommodity, stats, existing, idMapping, options)
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

// createCommodityIfNotDryRun creates a commodity if not in dry run mode.
// Before creating, it verifies that no other user already owns a commodity with the same immutable UUID,
// preventing a malicious restore from hijacking another user's entity.
func (l *RestoreOperationProcessor) createCommodityIfNotDryRun(
	ctx context.Context,
	commodity *models.Commodity,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	// Check ownership before creating: if a commodity with this UUID already exists in
	// the database but belongs to a different user, skip creation but do NOT propagate
	// the error — security violations are counted in stats, and restore continues.
	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}
	if err := l.validateCommodityOwnershipInDB(ctx, originalXMLID, currentUser, existing, stats); err != nil {
		if errors.Is(err, security.ErrOwnershipViolation) {
			// Security violation already recorded in stats.ErrorCount; skip this commodity.
			return nil
		}
		// Propagate operational/system errors so they are not silently swallowed.
		return errxtrace.Wrap("failed to validate commodity ownership in DB", err, errx.Attrs("xml_id", originalXMLID))
	}

	if !options.DryRun {
		comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user commodity registry", err)
		}
		// Preserve the immutable UUID from XML so the entity is stable across restores.
		commodity.UUID = originalXMLID
		if err := l.ensureCommodityTags(ctx, commodity, originalXMLID); err != nil {
			return err
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

// ensureCommodityTags auto-creates `tags` rows for any slug carried on the
// restored commodity that doesn't already exist in the target group, and
// rewrites commodity.Tags to the normalized slug list. Mirrors the apiserver
// hook (see services.TagService.NormalizeAndEnsureSlugs callers in
// apiserver/commodities.go and apiserver/files.go). Caller must guard with
// !options.DryRun so a preview does not mutate the tags table; we don't
// re-check it here because both call sites are already inside that branch
// and we want the helper to be loud if a future caller forgets.
func (l *RestoreOperationProcessor) ensureCommodityTags(ctx context.Context, commodity *models.Commodity, originalXMLID string) error {
	if len(commodity.Tags) == 0 {
		return nil
	}
	slugs, err := l.tagService.NormalizeAndEnsureSlugs(ctx, []string(commodity.Tags))
	if err != nil {
		return errxtrace.Wrap("failed to ensure tags for restored commodity", err, errx.Attrs("original_commodity_id", originalXMLID))
	}
	commodity.Tags = models.ValuerSlice[string](slugs)
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

	// Restore the DB ID and preserve the immutable UUID before updating.
	commodity.SetID(existingCommodity.ID)
	commodity.UUID = existingCommodity.UUID
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
		if err := l.ensureCommodityTags(ctx, commodity, originalXMLID); err != nil {
			return err
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

// processFilesWithLogging walks the <file> children of <files>, decoding
// each into a types.XMLFile and dispatching to processFile for strategy
// resolution. Mirrors the locations / areas / commodities sweep pattern.
func (l *RestoreOperationProcessor) processFilesWithLogging(
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
			return errxtrace.Wrap("failed to read files token", err)
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "file" {
				if err := l.processFile(ctx, decoder, &t, stats, existing, idMapping, options); err != nil {
					stats.ErrorCount++
					stats.Errors = append(stats.Errors, fmt.Sprintf("failed to process file: %v", err))
					continue
				}
			}
		case xml.EndElement:
			if t.Name.Local == "files" {
				return nil
			}
		}
	}
}

// processFile decodes a single <file> element and dispatches it to the
// strategy handler. The XML id attribute carries the immutable file UUID;
// linkedEntityId references the linked commodity/location/area by their
// UUID, resolved here to the destination DB ID via idMapping.
//
// We decode metadata via a manual token loop rather than DecodeElement so
// the <data> element can be streamed directly from the XML decoder into
// the destination blob via base64.NewDecoder. Buffering the whole
// <data> chardata before writing — the post-#1421 stub's behavior — would
// have inflated worker memory in proportion to the largest single blob.
func (l *RestoreOperationProcessor) processFile(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	xmlFile, blobBytes, err := l.decodeFileElement(ctx, decoder, startElement, existing, idMapping, options)
	if err != nil {
		return errxtrace.Wrap("failed to decode file element", err)
	}
	stats.BinaryDataSize += blobBytes

	originalXMLID := xmlFile.ID

	action := l.predictFileAction(xmlFile.ID, options, existing)
	emoji := l.getEmojiForAction(action)
	actionDesc := l.getActionDescription(action, options)
	displayName := xmlFile.Title
	if displayName == "" {
		displayName = xmlFile.Path
	}
	stepName := fmt.Sprintf("%s File: %s", emoji, displayName)
	l.createRestoreStep(ctx, stepName, models.RestoreStepResultInProgress, actionDesc)

	// Resolve the linked entity to a destination DB ID via the IDMapping.
	// Files referencing entities that weren't in the export (or that didn't
	// resolve in this restore) are dropped with an error rather than create
	// a row that points at nothing — the unique-on-UUID index would still
	// allow restoring it later when the entity exists.
	linkedDBID, ok := l.resolveLinkedEntityDBID(xmlFile.LinkedEntityType, xmlFile.LinkedEntityID, idMapping)
	if !ok {
		stats.ErrorCount++
		msg := fmt.Sprintf("file %s references unknown %s %s", originalXMLID, xmlFile.LinkedEntityType, xmlFile.LinkedEntityID)
		stats.Errors = append(stats.Errors, msg)
		l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, msg)
		return nil
	}

	fileEntity := xmlFile.ConvertToFileEntity(linkedDBID)

	// Stamp tenant/group/created-by from current context. The user-aware
	// registry would do this on Create, but for MergeUpdate we may go
	// through Update instead, which preserves the existing entity's
	// fields rather than re-populating from context.
	user := appctx.UserFromContext(ctx)
	if user == nil {
		stats.ErrorCount++
		l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, "missing user context")
		return security.ErrNoUserContext
	}
	fileEntity.TenantID = user.TenantID
	fileEntity.CreatedByUserID = user.ID
	if group := appctx.GroupFromContext(ctx); group != nil {
		fileEntity.GroupID = group.ID
	}

	if err := l.applyStrategyForFile(ctx, fileEntity, xmlFile, originalXMLID, stats, existing, idMapping, options); err != nil {
		l.updateRestoreStep(ctx, stepName, models.RestoreStepResultError, err.Error())
		return err
	}

	l.updateRestoreStep(ctx, stepName, models.RestoreStepResultSuccess, "Completed")
	return nil
}

// resolveLinkedEntityDBID maps an XML linked-entity reference (UUID) to the
// destination DB ID using the IDMapping. Returns ("", true) for standalone
// files (no linked entity to resolve) and ("", false) when the entity was
// expected but not found in the mapping.
func (l *RestoreOperationProcessor) resolveLinkedEntityDBID(
	linkedEntityType, linkedEntityXMLID string,
	idMapping *types.IDMapping,
) (string, bool) {
	if linkedEntityType == "" || linkedEntityXMLID == "" {
		return "", true
	}
	switch linkedEntityType {
	case "commodity":
		id, ok := idMapping.Commodities[linkedEntityXMLID]
		return id, ok
	case "location":
		id, ok := idMapping.Locations[linkedEntityXMLID]
		return id, ok
	case "area":
		id, ok := idMapping.Areas[linkedEntityXMLID]
		return id, ok
	default:
		// Unknown linked-entity-type: pass through the XML value verbatim
		// and accept whatever the registry does with it. Avoids hard
		// failures on linked-entity types added in future versions.
		return linkedEntityXMLID, true
	}
}

// predictFileAction is the file-specific equivalent of predictAction: it
// keys on the in-memory existing-files map (populated for non-FullReplace
// strategies) so the predicted action lines up with what
// applyStrategyForFile will actually do.
func (l *RestoreOperationProcessor) predictFileAction(xmlID string, options types.RestoreOptions, existing *types.ExistingEntities) string {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		return "create"
	case types.RestoreStrategyMergeAdd:
		if _, ok := existing.Files[xmlID]; ok {
			return "skip"
		}
		return "create"
	case types.RestoreStrategyMergeUpdate:
		if _, ok := existing.Files[xmlID]; ok {
			return "update"
		}
		return "create"
	default:
		return "unknown"
	}
}

// applyStrategyForFile dispatches a decoded XML file row to the matching
// FullReplace / MergeAdd / MergeUpdate handler. The file's UUID acts as
// the dedup key — DB-level idempotency is provided by the unique
// idx_files_uuid index even when in-memory tracking misses.
func (l *RestoreOperationProcessor) applyStrategyForFile(
	ctx context.Context,
	fileEntity *models.FileEntity,
	xmlFile *types.XMLFile,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	existingFile := existing.Files[originalXMLID]

	switch options.Strategy {
	case types.RestoreStrategyFullReplace:
		return l.handleFileFullReplace(ctx, fileEntity, xmlFile, originalXMLID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeAdd:
		if existingFile != nil {
			stats.SkippedCount++
			return nil
		}
		return l.handleFileFullReplace(ctx, fileEntity, xmlFile, originalXMLID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeUpdate:
		if existingFile == nil {
			return l.handleFileFullReplace(ctx, fileEntity, xmlFile, originalXMLID, stats, existing, idMapping, options)
		}
		// Restore the destination DB ID + UUID before updating.
		fileEntity.SetID(existingFile.ID)
		fileEntity.UUID = existingFile.UUID
		return l.handleFileMergeUpdate(ctx, fileEntity, xmlFile, originalXMLID, stats, existing, options)
	}
	return nil
}

// handleFileFullReplace creates a new file row (used for FullReplace and
// the create branch of MergeAdd / MergeUpdate). Preserves the immutable
// XML UUID. The blob bytes were already streamed to the bucket by the
// decode pass — this function only persists the row.
func (l *RestoreOperationProcessor) handleFileFullReplace(
	ctx context.Context,
	fileEntity *models.FileEntity,
	xmlFile *types.XMLFile,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	_ = xmlFile // blob already streamed to bucket during decode
	if !options.DryRun {
		fileEntity.UUID = originalXMLID
		fileEntity.Tags = ensureFileTags(fileEntity.Tags)
		fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user file registry", err)
		}
		created, err := fileReg.Create(ctx, *fileEntity)
		if err != nil {
			return errxtrace.Wrap("failed to create file row", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Files[originalXMLID] = created
		idMapping.Files[originalXMLID] = created.ID
		l.trackCreatedEntity(created.ID)
	}
	stats.CreatedCount++
	stats.FileCount++
	return nil
}

// handleFileMergeUpdate updates an existing file row in place. The file's
// UUID and DB ID were already restored on fileEntity by the caller, and
// the blob bytes were streamed during decode.
func (l *RestoreOperationProcessor) handleFileMergeUpdate(
	ctx context.Context,
	fileEntity *models.FileEntity,
	xmlFile *types.XMLFile,
	originalXMLID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	options types.RestoreOptions,
) error {
	_ = xmlFile // blob already streamed to bucket during decode
	if !options.DryRun {
		fileEntity.Tags = ensureFileTags(fileEntity.Tags)
		fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user file registry", err)
		}
		updated, err := fileReg.Update(ctx, *fileEntity)
		if err != nil {
			return errxtrace.Wrap("failed to update file row", err, errx.Attrs("xml_id", originalXMLID))
		}
		existing.Files[originalXMLID] = updated
	}
	stats.UpdatedCount++
	stats.FileCount++
	return nil
}

// decodeFileElement walks the children of a <file> StartElement, decoding
// each metadata sub-element via DecodeElement and streaming the <data>
// chardata directly into a blob writer in chunks. Returns the populated
// XMLFile (without the bulk Data field — the bytes never reside in memory)
// and the raw decoded blob byte count for stats.
//
// Ordering invariant: <originalPath> must precede <data>. The export side
// emits in that order; if a future format violates it, decodeFileElement
// fails loudly rather than corrupt the bucket with an unknown key.
//
// Strategy-aware blob handling: by the time we hit <data>, every metadata
// element (including linkedEntityType / linkedEntityId / the file's own
// XML UUID) has already been decoded, so we have enough state to decide
// up-front whether the blob is worth writing. The chardata is drained
// into io.Discard rather than the bucket when:
//   - DryRun is set, OR
//   - linkedEntityType is set but its linkedEntityId doesn't resolve via
//     IDMapping (the row would be rejected with an "unknown commodity"
//     error in processFile and the blob would be an orphan), OR
//   - strategy is MergeAdd and a file with the same UUID already exists
//     in the destination (the row will be skipped — overwriting its
//     blob with our copy would clobber whatever the destination's
//     custodian had at that key).
//
// For MergeUpdate-with-existing the blob IS rewritten — that path is
// the explicit "overwrite" semantic the strategy promises. For
// FullReplace the destination's existing.Files map is empty so the
// MergeAdd guard naturally falls through and the blob writes.
func (l *RestoreOperationProcessor) decodeFileElement(
	ctx context.Context,
	decoder *xml.Decoder,
	startElement *xml.StartElement,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) (*types.XMLFile, int64, error) {
	var xmlFile types.XMLFile
	for _, attr := range startElement.Attr {
		if attr.Name.Local == "id" {
			xmlFile.ID = attr.Value
			break
		}
	}

	var rawSize int64

	for {
		tok, err := decoder.Token()
		if err != nil {
			return nil, 0, errxtrace.Wrap("failed to read file element token", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "data" {
				if xmlFile.OriginalPath == "" {
					return nil, 0, errors.New("malformed export: <data> element preceded <originalPath>")
				}
				size, err := l.handleFileDataElement(ctx, decoder, &xmlFile, existing, idMapping, options)
				if err != nil {
					return nil, 0, errxtrace.Wrap("failed to stream file data", err, errx.Attrs("xml_id", xmlFile.ID))
				}
				rawSize = size
				continue
			}
			if err := decodeFileChild(decoder, &xmlFile, &t); err != nil {
				return nil, 0, err
			}
		case xml.EndElement:
			if t.Name.Local == "file" {
				return &xmlFile, rawSize, nil
			}
		}
	}
}

// handleFileDataElement is the dispatch point for <data> chardata: it
// uses the strategy + existing-files map + IDMapping to decide whether
// to stream the blob to the upload bucket or just count the decoded
// byte size while draining to io.Discard. See decodeFileElement's
// doc comment for the full decision matrix.
func (l *RestoreOperationProcessor) handleFileDataElement(
	ctx context.Context,
	decoder *xml.Decoder,
	xmlFile *types.XMLFile,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) (int64, error) {
	if shouldWriteFileBlob(xmlFile, existing, idMapping, options) {
		return l.streamDecodeFileData(ctx, decoder, xmlFile.OriginalPath, options)
	}
	// Drain the chardata into io.Discard so the byte count still flows
	// into BinaryDataSize for the operator's preview / size estimate.
	return drainFileDataElement(decoder)
}

// shouldWriteFileBlob is the predicate behind handleFileDataElement.
// Pulled out so processFile can reuse it for symmetric error handling
// (e.g., asserting that a discard happened when expected). Returning
// false for non-decidable cases (no linked entity to resolve) errs on
// the side of "write" — matches the legacy pre-#1421 behavior.
func shouldWriteFileBlob(
	xmlFile *types.XMLFile,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) bool {
	if options.DryRun {
		return false
	}
	if existing != nil && existing.Files != nil {
		if _, dupe := existing.Files[xmlFile.ID]; dupe && options.Strategy == types.RestoreStrategyMergeAdd {
			return false
		}
	}
	// Linked-entity resolution: if the row will be rejected in
	// processFile because the referenced commodity / location / area
	// isn't in the IDMapping, the blob would be a permanent orphan.
	if xmlFile.LinkedEntityType != "" && xmlFile.LinkedEntityID != "" {
		if !linkedEntityResolves(xmlFile.LinkedEntityType, xmlFile.LinkedEntityID, idMapping) {
			return false
		}
	}
	return true
}

// linkedEntityResolves mirrors resolveLinkedEntityDBID's lookup for the
// known linked-entity types but returns just the boolean — used by
// shouldWriteFileBlob to short-circuit the blob write when the row
// would have been rejected anyway. Unknown linked-entity types pass
// through (forward-compat: a future format may add new types and we
// don't want to silently drop their blobs).
func linkedEntityResolves(linkedType, linkedXMLID string, idMapping *types.IDMapping) bool {
	if idMapping == nil {
		return true
	}
	switch linkedType {
	case "commodity":
		_, ok := idMapping.Commodities[linkedXMLID]
		return ok
	case "location":
		_, ok := idMapping.Locations[linkedXMLID]
		return ok
	case "area":
		_, ok := idMapping.Areas[linkedXMLID]
		return ok
	}
	return true
}

// drainFileDataElement consumes the chardata of a <data> element via
// the streaming base64 decoder into io.Discard, returning the raw
// decoded byte count for stats. Used when decodeFileElement decides
// the blob isn't worth writing (DryRun, MergeAdd dedup, unresolved
// linkage).
func drainFileDataElement(decoder *xml.Decoder) (int64, error) {
	chardataReader := &xmlChardataReader{decoder: decoder, terminator: "data"}
	base64Reader := base64.NewDecoder(base64.StdEncoding, chardataReader)
	n, err := io.Copy(io.Discard, base64Reader)
	if err != nil {
		return 0, errxtrace.Wrap("failed to drain base64 stream", err)
	}
	if !chardataReader.done {
		return 0, errors.New("malformed export: <data> stream ended before </data>")
	}
	return n, nil
}

// decodeFileChild dispatches a single non-<data> sub-element of <file> to
// the matching XMLFile field. Unknown elements are skipped via the
// decoder's built-in element-skip — DecodeElement on a *struct{} consumes
// the entire element subtree.
func decodeFileChild(decoder *xml.Decoder, xmlFile *types.XMLFile, t *xml.StartElement) error {
	switch t.Name.Local {
	case "linkedEntityType":
		return decoder.DecodeElement(&xmlFile.LinkedEntityType, t)
	case "linkedEntityId":
		return decoder.DecodeElement(&xmlFile.LinkedEntityID, t)
	case "linkedEntityMeta":
		return decoder.DecodeElement(&xmlFile.LinkedEntityMeta, t)
	case "type":
		return decoder.DecodeElement(&xmlFile.Type, t)
	case "category":
		return decoder.DecodeElement(&xmlFile.Category, t)
	case "title":
		return decoder.DecodeElement(&xmlFile.Title, t)
	case "description":
		return decoder.DecodeElement(&xmlFile.Description, t)
	case "tags":
		return decoder.DecodeElement(&xmlFile.Tags, t)
	case "path":
		return decoder.DecodeElement(&xmlFile.Path, t)
	case "originalPath":
		return decoder.DecodeElement(&xmlFile.OriginalPath, t)
	case "extension":
		return decoder.DecodeElement(&xmlFile.Extension, t)
	case "mimeType":
		return decoder.DecodeElement(&xmlFile.MimeType, t)
	case "createdAt":
		return decoder.DecodeElement(&xmlFile.CreatedAt, t)
	case "updatedAt":
		return decoder.DecodeElement(&xmlFile.UpdatedAt, t)
	default:
		// Skip unknown sub-elements (forward-compat with newer formats).
		var ignored struct{}
		return decoder.DecodeElement(&ignored, t)
	}
}

// streamDecodeFileData consumes the chardata of a <data> element through a
// streaming base64 decoder into the upload bucket under blobKey. Returns
// the raw decoded byte count read out of the base64 decoder.
//
// Caller must have already decided that the blob is worth writing —
// DryRun, MergeAdd-dedup, and unresolved-linkage cases route through
// drainFileDataElement instead so they don't open the bucket at all.
//
// The upload bucket is opened per-file rather than amortized across the
// import — same tradeoff as the export side. Imports of thousands of
// files might want to hold the bucket open; that's a future optimization
// once we have a benchmark.
func (l *RestoreOperationProcessor) streamDecodeFileData(
	ctx context.Context,
	decoder *xml.Decoder,
	blobKey string,
	options types.RestoreOptions,
) (int64, error) {
	_ = options // strategy/dry-run already filtered by shouldWriteFileBlob
	if l.uploadLocation == "" {
		// Test fixtures may run without an upload bucket configured;
		// fall through to a drain so the byte count still flows.
		return drainFileDataElement(decoder)
	}

	chardataReader := &xmlChardataReader{decoder: decoder, terminator: "data"}
	base64Reader := base64.NewDecoder(base64.StdEncoding, chardataReader)

	bucket, err := blob.OpenBucket(ctx, l.uploadLocation)
	if err != nil {
		return 0, errxtrace.Wrap("failed to open blob bucket for streaming write", err)
	}
	defer bucket.Close()

	writer, err := bucket.NewWriter(ctx, blobKey, nil)
	if err != nil {
		return 0, errxtrace.Wrap("failed to create blob writer", err, errx.Attrs("blob_key", blobKey))
	}

	n, copyErr := io.Copy(writer, base64Reader)
	closeErr := writer.Close()
	if copyErr != nil {
		return 0, errxtrace.Wrap("failed to stream blob bytes", copyErr, errx.Attrs("blob_key", blobKey))
	}
	if closeErr != nil {
		return 0, errxtrace.Wrap("failed to close blob writer", closeErr, errx.Attrs("blob_key", blobKey))
	}
	if !chardataReader.done {
		return 0, errors.New("malformed export: <data> stream ended before </data>")
	}
	return n, nil
}

// xmlChardataReader exposes the decoder's chardata tokens (until the
// matching `</terminator>` end element) as an io.Reader. Whitespace
// introduced by pretty-printing is stripped on the fly so the underlying
// base64 decoder sees a contiguous stream. Callers wrap this in
// `base64.NewDecoder` and io.Copy into a destination writer.
type xmlChardataReader struct {
	decoder    *xml.Decoder
	terminator string
	buf        []byte // pending chardata bytes (whitespace-stripped)
	done       bool
}

func (r *xmlChardataReader) Read(p []byte) (int, error) {
	for len(r.buf) == 0 && !r.done {
		tok, err := r.decoder.Token()
		if err != nil {
			return 0, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			for _, b := range t {
				switch b {
				case ' ', '\n', '\r', '\t':
					continue
				}
				r.buf = append(r.buf, b)
			}
		case xml.EndElement:
			if t.Name.Local == r.terminator {
				r.done = true
			}
		}
	}
	if r.done && len(r.buf) == 0 {
		return 0, io.EOF
	}
	n := copy(p, r.buf)
	r.buf = r.buf[n:]
	return n, nil
}

// ensureFileTags normalizes nil to empty so the registry's NOT NULL JSONB
// constraint accepts the value. Mirrors the upload-time behavior in
// apiserver/uploads.go where Tags is always at least [].
func ensureFileTags(tags models.StringSlice) models.StringSlice {
	if tags == nil {
		return models.StringSlice{}
	}
	return tags
}
