package processor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/backup/restore/security"
	"github.com/denisvmedia/inventario/backup/restore/types"
	"github.com/denisvmedia/inventario/internal/backupsign"
	"github.com/denisvmedia/inventario/internal/validationctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/services"
)

// RestoreOperationProcessor wraps the restore service to provide detailed logging.
//
// The signer field is used by the default `.inb` restorer (#534) to verify the
// archive signature before inflating. Under the legacy_xml_backup build the
// signer is accepted by the constructor but ignored — the constructor signature
// is identical across builds so the worker/bootstrap wiring never branches.
type RestoreOperationProcessor struct {
	restoreOperationID    string
	factorySet            *registry.FactorySet
	entityService         *services.EntityService
	tagService            *services.TagService
	uploadLocation        string
	signer                *backupsign.Signer
	securityValidator     security.SecurityValidator
	importSessionEntities map[string]bool // Track entities created in this session

	// commodityUUIDMap is a lazy-loaded cache of all pre-existing commodities
	// keyed by their immutable UUID. Populated once on the first call to
	// validateCommodityOwnershipInDB.
	commodityUUIDMap map[string]*models.Commodity
}

// NewRestoreOperationProcessor builds a processor. The signer is consumed by the
// default `.inb` restorer and ignored by the legacy XML restorer; the signature
// is identical across both builds on purpose.
func NewRestoreOperationProcessor(restoreOperationID string, factorySet *registry.FactorySet, entityService *services.EntityService, uploadLocation string, signer *backupsign.Signer) *RestoreOperationProcessor {
	logger := slog.Default()
	return &RestoreOperationProcessor{
		restoreOperationID:    restoreOperationID,
		factorySet:            factorySet,
		entityService:         entityService,
		tagService:            services.NewTagService(factorySet),
		uploadLocation:        uploadLocation,
		signer:                signer,
		securityValidator:     security.NewRestoreSecurityValidator(factorySet, logger),
		importSessionEntities: make(map[string]bool),
	}
}

// Process drives the full restore lifecycle: status/step bookkeeping, blob open,
// then the per-build decodeAndRestore (XML or `.inb`). The decode/verify/apply
// body differs per format; everything around it is format-agnostic.
func (l *RestoreOperationProcessor) Process(ctx context.Context) error {
	restoreOperationRegistry := l.factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry()
	restoreOperation, err := restoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to get restore operation: %v", err))
	}

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

	// Resolve the export's group and inject it into ctx. The restore runs from
	// a background worker with no request middleware, so the group (and its
	// currency, needed by commodity validation) must be resolved here — mirrors
	// ProcessExport's group wiring.
	if export.GroupID != "" {
		if group, gerr := l.factorySet.LocationGroupRegistry.Get(ctx, export.GroupID); gerr == nil {
			ctx = appctx.WithGroup(ctx, group)
		}
	}

	restoreOperation.Status = models.RestoreStatusRunning
	restoreOperation.StartedDate = models.PNow()
	if _, err = restoreOperationRegistry.Update(ctx, *restoreOperation); err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to update restore status: %v", err))
	}

	l.createRestoreStep(ctx, "Initializing restore", models.RestoreStepResultInProgress, "")

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

	l.updateRestoreStep(ctx, "Initializing restore", models.RestoreStepResultSuccess, "")
	l.createRestoreStep(ctx, "Reading backup file", models.RestoreStepResultInProgress, "")

	restoreOptions := types.RestoreOptions{
		Strategy:        types.RestoreStrategy(restoreOperation.Options.Strategy),
		IncludeFileData: restoreOperation.Options.IncludeFileData,
		DryRun:          restoreOperation.Options.DryRun,
	}

	l.updateRestoreStep(ctx, "Reading backup file", models.RestoreStepResultSuccess, "")

	stats, err := l.decodeAndRestore(ctx, reader, restoreOptions)
	if err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("restore failed: %v", err))
	}

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

	if _, err = restoreOperationRegistry.Update(ctx, *restoreOperation); err != nil {
		return l.markRestoreFailed(ctx, fmt.Sprintf("failed to update restore completion status: %v", err))
	}

	l.createRestoreStep(ctx, "Restore completed successfully", models.RestoreStepResultSuccess,
		fmt.Sprintf("Processed %d locations, %d areas, %d commodities with %d errors",
			stats.LocationCount, stats.AreaCount, stats.CommodityCount, stats.ErrorCount))

	return nil
}

// restorePrep bundles the state prepareRestore produces: the currency-augmented
// context plus the existing-entity snapshot and ID mapping the strategy handlers
// thread through. Returning a struct keeps prepareRestore within the project's
// 3-result function limit.
type restorePrep struct {
	ctx       context.Context
	existing  *types.ExistingEntities
	idMapping *types.IDMapping
}

// prepareRestore validates options, seeds the group currency into ctx, and
// loads existing entities + the ID mapping per strategy. Shared by both the XML
// and `.inb` decode paths so the strategy bookkeeping stays identical.
func (l *RestoreOperationProcessor) prepareRestore(
	ctx context.Context,
	options types.RestoreOptions,
) (*restorePrep, error) {
	if err := l.validateOptions(options); err != nil {
		return nil, errxtrace.Wrap("invalid restore options", err)
	}

	if group := appctx.GroupFromContext(ctx); group != nil && group.GroupCurrency != "" {
		ctx = validationctx.WithGroupCurrency(ctx, string(group.GroupCurrency))
	}

	existingEntities := &types.ExistingEntities{}
	idMapping := &types.IDMapping{
		Locations:   make(map[string]string),
		Areas:       make(map[string]string),
		Commodities: make(map[string]string),
		Files:       make(map[string]string),
	}

	if options.Strategy != types.RestoreStrategyFullReplace {
		if err := l.loadExistingEntities(ctx, existingEntities); err != nil {
			return nil, errxtrace.Wrap("failed to load existing entities", err)
		}
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
		existingEntities.Locations = make(map[string]*models.Location)
		existingEntities.Areas = make(map[string]*models.Area)
		existingEntities.Commodities = make(map[string]*models.Commodity)
		existingEntities.Files = make(map[string]*models.FileEntity)
	}

	if options.Strategy == types.RestoreStrategyFullReplace && !options.DryRun {
		if err := l.clearExistingData(ctx); err != nil {
			return nil, errxtrace.Wrap("failed to clear existing data", err)
		}
	}

	return &restorePrep{ctx: ctx, existing: existingEntities, idMapping: idMapping}, nil
}

// validateOptions validates the restore options.
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

// clearExistingData removes all existing data for full replace strategy. See
// the original comment chain — DeleteLocationRecursive cascades commodities and
// commodity-linked files; a second sweep removes location-/area-/standalone
// files (rows + blobs) so nothing orphans or collides on restore.
func (l *RestoreOperationProcessor) clearExistingData(ctx context.Context) error {
	// RLS-scoped registry: a full_replace restore must only wipe the restoring
	// user's own locations. CreateServiceRegistry bypasses RLS and would
	// enumerate (and recursively delete) EVERY tenant's locations — a
	// cross-tenant data wipe. Mirror the file sweep below, which already uses
	// the user registry.
	locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user location registry for clear", err)
	}
	locations, err := locReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list locations for deletion", err)
	}
	for _, location := range locations {
		if err := l.entityService.DeleteLocationRecursive(ctx, location.ID); err != nil {
			return errxtrace.Wrap("failed to delete location recursively", err, errx.Attrs("location_id", location.ID))
		}
	}

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

// validateCommodityOwnershipInDB validates that a commodity in the database
// belongs to the current user. Identical behaviour to the original.
func (l *RestoreOperationProcessor) validateCommodityOwnershipInDB(
	ctx context.Context,
	originalXMLID string,
	currentUser *models.User,
	existing *types.ExistingEntities,
	stats *types.RestoreStats,
) error {
	existingCommodity := existing.Commodities[originalXMLID]
	if existingCommodity != nil {
		return nil
	}

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
		return nil
	}

	if existingDBCommodity.CreatedByUserID != currentUser.ID {
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
		return errxtrace.Classify(security.ErrOwnershipViolation, errx.Attrs("xml_id", originalXMLID, "commodity_id", existingDBCommodity.ID))
	}

	return nil
}

// trackCreatedEntity tracks entities created in this import session.
func (l *RestoreOperationProcessor) trackCreatedEntity(entityID string) {
	if l.importSessionEntities == nil {
		l.importSessionEntities = make(map[string]bool)
	}
	l.importSessionEntities[entityID] = true
}

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
	if _, err := stepReg.Create(ctx, step); err != nil {
		slog.Error("Failed to create restore step", "error", err)
	}
}

func (l *RestoreOperationProcessor) updateRestoreStep(ctx context.Context, name string, result models.RestoreStepResult, reason string) {
	stepReg := l.factorySet.RestoreStepRegistryFactory.CreateServiceRegistry()
	steps, err := stepReg.ListByRestoreOperation(ctx, l.restoreOperationID)
	if err != nil {
		l.createRestoreStep(ctx, name, result, reason)
		return
	}

	for _, step := range steps {
		if step.Name == name {
			step.Result = result
			step.Reason = reason
			if _, uerr := stepReg.Update(ctx, *step); uerr != nil {
				slog.Error("Failed to update restore step", "error", uerr)
			}
			return
		}
	}

	l.createRestoreStep(ctx, name, result, reason)
}

func (l *RestoreOperationProcessor) markRestoreFailed(ctx context.Context, errorMessage string) error {
	restoreOperationRegistry := l.factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry()

	restoreOperation, err := restoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return err
	}

	restoreOperation.Status = models.RestoreStatusFailed
	restoreOperation.CompletedDate = models.PNow()
	restoreOperation.ErrorMessage = errorMessage

	if _, err = restoreOperationRegistry.Update(ctx, *restoreOperation); err != nil {
		return err
	}

	l.createRestoreStep(ctx, "Restore failed", models.RestoreStepResultError, errorMessage)
	return fmt.Errorf("%s", errorMessage)
}

// --- model-level strategy handlers (format-agnostic) ---

// applyStrategyForLocationModel applies the restore strategy for a location
// model. displayName is used purely for step messages so the handler is free of
// any format-specific struct.
func (l *RestoreOperationProcessor) applyStrategyForLocationModel(
	ctx context.Context,
	location *models.Location,
	existingLocation *models.Location,
	originalID, displayName string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace, types.RestoreStrategyMergeAdd:
		if options.Strategy == types.RestoreStrategyMergeAdd && existingLocation != nil {
			stats.SkippedCount++
			return nil
		}
		return l.createLocation(ctx, location, originalID, displayName, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeUpdate:
		if existingLocation == nil {
			return l.createLocation(ctx, location, originalID, displayName, stats, existing, idMapping, options)
		}
		location.ID = existingLocation.ID
		location.UUID = existingLocation.UUID
		return l.updateLocation(ctx, location, existingLocation, originalID, displayName, stats, existing, options)
	}
	return nil
}

func (l *RestoreOperationProcessor) createLocation(
	ctx context.Context,
	location *models.Location,
	originalID, displayName string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user location registry", err)
		}
		location.UUID = originalID
		created, err := locReg.Create(ctx, *location)
		if err != nil {
			l.updateRestoreStep(ctx, locationStep(displayName), models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to create location", err, errx.Attrs("xml_id", originalID))
		}
		existing.Locations[originalID] = created
		idMapping.Locations[originalID] = created.ID
		l.trackCreatedEntity(created.ID)
	}
	stats.CreatedCount++
	stats.LocationCount++
	return nil
}

func (l *RestoreOperationProcessor) updateLocation(
	ctx context.Context,
	location *models.Location,
	_ *models.Location,
	originalID, displayName string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		locReg, err := l.factorySet.LocationRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user location registry", err)
		}
		updated, err := locReg.Update(ctx, *location)
		if err != nil {
			l.updateRestoreStep(ctx, locationStep(displayName), models.RestoreStepResultError, err.Error())
			return errxtrace.Wrap("failed to update location", err, errx.Attrs("xml_id", originalID))
		}
		existing.Locations[originalID] = updated
	}
	stats.UpdatedCount++
	stats.LocationCount++
	return nil
}

// applyStrategyForAreaModel applies the restore strategy for an area model.
func (l *RestoreOperationProcessor) applyStrategyForAreaModel(
	ctx context.Context,
	area *models.Area,
	existingArea *models.Area,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace, types.RestoreStrategyMergeAdd:
		if options.Strategy == types.RestoreStrategyMergeAdd && existingArea != nil {
			stats.SkippedCount++
			return nil
		}
		return l.createArea(ctx, area, originalID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeUpdate:
		if existingArea == nil {
			return l.createArea(ctx, area, originalID, stats, existing, idMapping, options)
		}
		area.SetID(existingArea.ID)
		area.UUID = existingArea.UUID
		return l.updateArea(ctx, area, originalID, stats, existing, options)
	}
	return nil
}

func (l *RestoreOperationProcessor) createArea(
	ctx context.Context,
	area *models.Area,
	originalID string,
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
		area.UUID = originalID
		created, err := areaReg.Create(ctx, *area)
		if err != nil {
			return errxtrace.Wrap("failed to create area", err, errx.Attrs("xml_id", originalID))
		}
		existing.Areas[originalID] = created
		idMapping.Areas[originalID] = created.ID
		l.trackCreatedEntity(created.ID)
	}
	stats.CreatedCount++
	stats.AreaCount++
	return nil
}

func (l *RestoreOperationProcessor) updateArea(
	ctx context.Context,
	area *models.Area,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		areaReg, err := l.factorySet.AreaRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user area registry", err)
		}
		updated, err := areaReg.Update(ctx, *area)
		if err != nil {
			return errxtrace.Wrap("failed to update area", err, errx.Attrs("xml_id", originalID))
		}
		existing.Areas[originalID] = updated
	}
	stats.UpdatedCount++
	stats.AreaCount++
	return nil
}

// applyStrategyForCommodityModel applies the restore strategy for a commodity
// model. Ownership has already been validated by the caller.
func (l *RestoreOperationProcessor) applyStrategyForCommodityModel(
	ctx context.Context,
	commodity *models.Commodity,
	existingCommodity *models.Commodity,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	switch options.Strategy {
	case types.RestoreStrategyFullReplace, types.RestoreStrategyMergeAdd:
		if options.Strategy == types.RestoreStrategyMergeAdd && existingCommodity != nil {
			stats.SkippedCount++
			return nil
		}
		return l.createCommodity(ctx, commodity, originalID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeUpdate:
		if existingCommodity == nil {
			return l.createCommodity(ctx, commodity, originalID, stats, existing, idMapping, options)
		}
		commodity.SetID(existingCommodity.ID)
		commodity.UUID = existingCommodity.UUID
		return l.updateCommodity(ctx, commodity, originalID, stats, existing, options)
	}
	return nil
}

// createCommodity creates a commodity, after re-checking ownership and ensuring
// its tags exist.
func (l *RestoreOperationProcessor) createCommodity(
	ctx context.Context,
	commodity *models.Commodity,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	currentUser := appctx.UserFromContext(ctx)
	if currentUser == nil {
		return security.ErrNoUserContext
	}
	if err := l.validateCommodityOwnershipInDB(ctx, originalID, currentUser, existing, stats); err != nil {
		if errors.Is(err, security.ErrOwnershipViolation) {
			return nil
		}
		return errxtrace.Wrap("failed to validate commodity ownership in DB", err, errx.Attrs("xml_id", originalID))
	}

	if !options.DryRun {
		comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user commodity registry", err)
		}
		commodity.UUID = originalID
		if err := l.ensureCommodityTags(ctx, commodity, originalID); err != nil {
			return err
		}
		created, err := comReg.Create(ctx, *commodity)
		if err != nil {
			return errxtrace.Wrap("failed to create commodity", err, errx.Attrs("xml_id", originalID))
		}
		existing.Commodities[originalID] = created
		idMapping.Commodities[originalID] = created.ID
		l.trackCreatedEntity(created.ID)
	}
	stats.CreatedCount++
	stats.CommodityCount++
	return nil
}

func (l *RestoreOperationProcessor) updateCommodity(
	ctx context.Context,
	commodity *models.Commodity,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user commodity registry", err)
		}
		if err := l.ensureCommodityTags(ctx, commodity, originalID); err != nil {
			return err
		}
		updated, err := comReg.Update(ctx, *commodity)
		if err != nil {
			return errxtrace.Wrap("failed to update commodity", err, errx.Attrs("xml_id", originalID))
		}
		existing.Commodities[originalID] = updated
	}
	stats.UpdatedCount++
	stats.CommodityCount++
	return nil
}

// ensureCommodityTags auto-creates tag rows for any slug carried on the restored
// commodity that doesn't already exist in the target group.
func (l *RestoreOperationProcessor) ensureCommodityTags(ctx context.Context, commodity *models.Commodity, originalID string) error {
	if len(commodity.Tags) == 0 {
		return nil
	}
	slugs, err := l.tagService.NormalizeAndEnsureSlugs(ctx, models.TagKindCommodity, []string(commodity.Tags))
	if err != nil {
		return errxtrace.Wrap("failed to ensure tags for restored commodity", err, errx.Attrs("original_commodity_id", originalID))
	}
	commodity.Tags = models.ValuerSlice[string](slugs)
	return nil
}

// applyStrategyForFileModel persists a decoded file row per strategy. The blob
// bytes were already streamed to the bucket by the per-build decode pass.
func (l *RestoreOperationProcessor) applyStrategyForFileModel(
	ctx context.Context,
	fileEntity *models.FileEntity,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	existingFile := existing.Files[originalID]

	switch options.Strategy {
	case types.RestoreStrategyFullReplace, types.RestoreStrategyMergeAdd:
		if options.Strategy == types.RestoreStrategyMergeAdd && existingFile != nil {
			stats.SkippedCount++
			return nil
		}
		return l.createFile(ctx, fileEntity, originalID, stats, existing, idMapping, options)
	case types.RestoreStrategyMergeUpdate:
		if existingFile == nil {
			return l.createFile(ctx, fileEntity, originalID, stats, existing, idMapping, options)
		}
		fileEntity.SetID(existingFile.ID)
		fileEntity.UUID = existingFile.UUID
		return l.updateFile(ctx, fileEntity, originalID, stats, existing, options)
	}
	return nil
}

func (l *RestoreOperationProcessor) createFile(
	ctx context.Context,
	fileEntity *models.FileEntity,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		fileEntity.UUID = originalID
		fileEntity.Tags = ensureFileTags(fileEntity.Tags)
		fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user file registry", err)
		}
		created, err := fileReg.Create(ctx, *fileEntity)
		if err != nil {
			return errxtrace.Wrap("failed to create file row", err, errx.Attrs("xml_id", originalID))
		}
		existing.Files[originalID] = created
		idMapping.Files[originalID] = created.ID
		l.trackCreatedEntity(created.ID)
	}
	stats.CreatedCount++
	stats.FileCount++
	return nil
}

func (l *RestoreOperationProcessor) updateFile(
	ctx context.Context,
	fileEntity *models.FileEntity,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	options types.RestoreOptions,
) error {
	if !options.DryRun {
		fileEntity.Tags = ensureFileTags(fileEntity.Tags)
		fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
		if err != nil {
			return errxtrace.Wrap("failed to create user file registry", err)
		}
		updated, err := fileReg.Update(ctx, *fileEntity)
		if err != nil {
			return errxtrace.Wrap("failed to update file row", err, errx.Attrs("xml_id", originalID))
		}
		existing.Files[originalID] = updated
	}
	stats.UpdatedCount++
	stats.FileCount++
	return nil
}

// ensureFileTags normalizes nil to empty so the registry's NOT NULL JSONB
// constraint accepts the value.
func ensureFileTags(tags models.StringSlice) models.StringSlice {
	if tags == nil {
		return models.StringSlice{}
	}
	return tags
}

// locationStep returns the canonical step name for a location.
func locationStep(displayName string) string {
	return fmt.Sprintf("Location: %s", displayName)
}
