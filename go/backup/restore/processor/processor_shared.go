package processor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"gocloud.dev/blob"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/backup/restore/security"
	"go.5x5.cz/inventario/backup/restore/types"
	"go.5x5.cz/inventario/internal/backupsign"
	"go.5x5.cz/inventario/internal/blobkeys"
	"go.5x5.cz/inventario/internal/validationctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/services"
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

	// itemTypes maps archived item type slugs onto the destination group's
	// types (#2802); created on first use by itemTypeResolver.
	itemTypes *itemTypeResolver
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
		securityValidator:     security.NewRestoreSecurityValidator(logger),
		importSessionEntities: make(map[string]bool),
	}
}

// fileService exposes the shared-key guard (#2250) to the restore importer.
//
// The importer deletes blobs directly — it supersedes the old blob of a row it
// re-points during MergeUpdate — so it must ask the same question the
// FileService delete paths ask before removing any bytes: does another file row
// still reference this key? Pre-#2241 keys are shareable, and `files` has no
// soft-delete, so the answer is the difference between reclaiming a blob and
// destroying a live file.
//
// Constructed per call rather than cached: FileService is a thin façade over the
// factory set, and the restore path builds one at most once per superseded blob.
func (l *RestoreOperationProcessor) fileService() *services.FileService {
	return services.NewFileService(l.factorySet, l.uploadLocation)
}

// Process drives the full restore lifecycle: status/step bookkeeping, blob open,
// then the per-build decodeAndRestore (XML or `.inb`). The decode/verify/apply
// body differs per format; everything around it is format-agnostic.
func (l *RestoreOperationProcessor) Process(ctx context.Context) error {
	restoreOperationRegistry := l.factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry()
	restoreOperation, err := restoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return l.markRestoreFailed(ctx, err, "failed to get restore operation")
	}

	exportReg := l.factorySet.ExportRegistryFactory.CreateServiceRegistry()
	export, err := exportReg.Get(ctx, restoreOperation.ExportID)
	if err != nil {
		return l.markRestoreFailed(ctx, err, "failed to get export")
	}

	user, err := l.factorySet.UserRegistry.Get(ctx, export.CreatedByUserID)
	if err != nil {
		return l.markRestoreFailed(ctx, err, "failed to get user")
	}
	ctx = appctx.WithUser(ctx, user)

	// Resolve the export's group and inject it into ctx. The restore runs from
	// a background worker with no request middleware, so the group (and its
	// currency, needed by commodity validation) must be resolved here — mirrors
	// ProcessExport's group wiring. A non-empty GroupID that fails to resolve is
	// a hard failure: silently dropping it leaves group_currency unset, so every
	// commodity fails validation while the restore still reports Completed.
	if export.GroupID != "" {
		group, gerr := l.factorySet.LocationGroupRegistry.Get(ctx, export.GroupID)
		if gerr != nil {
			return l.markRestoreFailed(ctx, gerr, "failed to get restore group")
		}
		ctx = appctx.WithGroup(ctx, group)
	}

	restoreOperation.Status = models.RestoreStatusRunning
	restoreOperation.StartedDate = models.PNow()
	if _, err = restoreOperationRegistry.Update(ctx, *restoreOperation); err != nil {
		return l.markRestoreFailed(ctx, err, "failed to update restore status")
	}

	l.createRestoreStep(ctx, "Initializing restore", models.RestoreStepResultInProgress, "")

	// Defense-in-depth: the restore worker reads the export's blob key WITHOUT
	// RLS, and an `.inb` archive is signed against a tenant-AGNOSTIC server key.
	// Refuse a FilePath that lives outside the export's own tenant namespace
	// before opening it, so a row whose FilePath was somehow set to another
	// tenant's key cannot exfiltrate that tenant's data. An empty tenant yields
	// an empty prefix — guard it explicitly so HasPrefix can't pass vacuously.
	prefix := blobkeys.TenantPrefix(export.TenantID)
	if prefix == "" || !strings.HasPrefix(export.FilePath, prefix) {
		return l.markRestoreFailed(ctx,
			errx.NewSentinel("export file path is outside the export's tenant namespace"),
			"failed to open export file")
	}

	b, err := blob.OpenBucket(ctx, l.uploadLocation)
	if err != nil {
		return l.markRestoreFailed(ctx, err, "failed to open blob bucket")
	}
	defer b.Close()

	reader, err := b.NewReader(ctx, export.FilePath, nil)
	if err != nil {
		return l.markRestoreFailed(ctx, err, "failed to open export file")
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
		return l.markRestoreFailed(ctx, err, "restore failed")
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
		return l.markRestoreFailed(ctx, err, "failed to update restore completion status")
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
	entities.TrashedCommodities = make(map[string]string)
	entities.TrashedFiles = make(map[string]bool)

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
		if file.IsExportOwned() {
			continue
		}
		entities.Files[file.UUID] = file
	}

	return loadTrashedEntities(ctx, comReg, fileReg, entities)
}

// trashPageSize is how many Trash entries one read of loadTrashedEntities
// takes.
const trashPageSize = 500

// loadTrashedEntities records the UUIDs of the group's items and files in
// Trash: the files deleted on their own and the files that went with an item.
func loadTrashedEntities(ctx context.Context, comReg registry.CommodityRegistry, fileReg registry.FileRegistry, entities *types.ExistingEntities) error {
	var trashedItemIDs []string
	for offset := 0; ; offset += trashPageSize {
		page, total, err := comReg.ListTrashed(ctx, offset, trashPageSize)
		if err != nil {
			return errxtrace.Wrap("failed to load trashed commodities", err)
		}
		for _, c := range page {
			entities.TrashedCommodities[c.UUID] = c.ID
			trashedItemIDs = append(trashedItemIDs, c.ID)
		}
		if len(page) == 0 || offset+len(page) >= total {
			break
		}
	}
	for offset := 0; ; offset += trashPageSize {
		page, total, err := fileReg.ListTrashed(ctx, offset, trashPageSize)
		if err != nil {
			return errxtrace.Wrap("failed to load trashed files", err)
		}
		for _, f := range page {
			entities.TrashedFiles[f.UUID] = true
		}
		if len(page) == 0 || offset+len(page) >= total {
			break
		}
	}
	for _, id := range trashedItemIDs {
		files, err := fileReg.ListTrashedLinkedTo(ctx, models.FileLinkCommodity, id)
		if err != nil {
			return errxtrace.Wrap("failed to load trashed item files", err, errx.Attrs("commodity_id", id))
		}
		for _, f := range files {
			entities.TrashedFiles[f.UUID] = true
		}
	}
	// A file whose item left Trash before the file was settled is in no list
	// above, but its row still holds the UUID.
	stranded, err := fileReg.ListStrandedTrashed(ctx, maxStrandedOnRestore)
	if err != nil {
		return errxtrace.Wrap("failed to load stranded trashed files", err)
	}
	for _, f := range stranded {
		entities.TrashedFiles[f.UUID] = true
	}
	return nil
}

// maxStrandedOnRestore bounds the stranded-file read of loadTrashedEntities.
// The Trash sweep settles such files within one interval, so a group holds a
// handful at most.
const maxStrandedOnRestore = 10000

// parkFileInTrash moves a file this restore has just created to Trash with
// the item holderID. It is called for a new file whose only links point at
// items in Trash: deleting the item would have taken the file along, and
// restoring the item brings it back.
func (l *RestoreOperationProcessor) parkFileInTrash(ctx context.Context, fileDBID, holderID string) error {
	user := appctx.UserFromContext(ctx)
	if user == nil {
		return security.ErrNoUserContext
	}
	fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user file registry", err)
	}
	if err := fileReg.Trash(ctx, fileDBID, user.ID, holderID, time.Now().UTC()); err != nil {
		return errxtrace.Wrap("failed to move restored file to Trash", err, errx.Attrs("file_id", fileDBID))
	}
	return nil
}

// unparkFile brings back a file parkFileInTrash moved to Trash, once the
// restore links it to a live entity.
func (l *RestoreOperationProcessor) unparkFile(ctx context.Context, fileDBID string) error {
	fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user file registry", err)
	}
	if err := fileReg.Restore(ctx, fileDBID); err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to restore file from Trash", err, errx.Attrs("file_id", fileDBID))
	}
	return nil
}

// clearExistingData removes all existing data for full replace strategy. The
// group's Trash is emptied first: an item or file waiting there would otherwise
// keep its UUID and collide with the archive's copy. Then four passes, in this
// order:
//
//  1. PurgeLocationRecursive per location — cascades areas, their commodities,
//     and the commodity-/area-/location-linked files of that subtree.
//  2. DeleteCommodityRecursive per SURVIVING commodity. Commodities are
//     enumerated DIRECTLY because commodity.area_id is nullable since #1986: an
//     area-less commodity is not reachable through the location → area recursion
//     above, so before #2236 its row survived a full_replace while the file sweep
//     below still deleted its attachments (a zombie item with no files) — and its
//     preserved UUID then collided with the archive's own copy on re-create.
//  3. The group's drafts (#2812), in every state, with their file links. The
//     archive's open drafts take their place: the drafts member recreates
//     them, and the files member links their files to them again.
//  4. The group's intake sessions (#2810). Backups do not carry them, so a
//     replace leaves none; their files fall to the sweep unless the archive
//     brings them back.
//  5. A type-agnostic sweep of the remaining files (rows + blobs), which catches
//     standalone files and any orphan. linked_entity_type='export' is skipped:
//     that is the archive being restored plus the backup history, not inventory.
//
// Pass 2 runs before the sweep on purpose: DeleteCommodityRecursive cleans a
// commodity's own files (row + blob + thumbnails) instead of having them yanked
// out from under a live row by the sweep.
func (l *RestoreOperationProcessor) clearExistingData(ctx context.Context) error {
	if err := l.entityService.EmptyTrash(ctx); err != nil {
		return errxtrace.Wrap("failed to empty Trash", err)
	}

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
		if err := l.entityService.PurgeLocationRecursive(ctx, location.ID); err != nil {
			return errxtrace.Wrap("failed to delete location recursively", err, errx.Attrs("location_id", location.ID))
		}
	}

	if err := l.clearSurvivingCommodities(ctx); err != nil {
		return err
	}

	fileReg, err := l.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user file registry for clear", err)
	}
	if err := l.clearDrafts(ctx, fileReg); err != nil {
		return err
	}
	if err := l.clearIntake(ctx); err != nil {
		return err
	}

	files, err := fileReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list files for deletion", err)
	}
	fileService := services.NewFileService(l.factorySet, l.uploadLocation)
	for _, file := range files {
		if file.IsExportOwned() {
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

// clearDrafts deletes every draft of the group, open, promoted or being
// discarded, and the links of files to them. The files themselves are left
// to the sweep that follows. RLS-scoped registry, as for the locations.
func (l *RestoreOperationProcessor) clearDrafts(ctx context.Context, fileReg registry.FileRegistry) error {
	draftReg, err := l.factorySet.CommodityDraftRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user draft registry for clear", err)
	}
	drafts, err := draftReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list drafts for deletion", err)
	}
	for _, draft := range drafts {
		if _, err := fileReg.RemoveLinksToEntity(ctx, models.FileLinkDraft, draft.ID); err != nil {
			return errxtrace.Wrap("failed to remove draft file links", err, errx.Attrs("draft_id", draft.ID))
		}
		if err := draftReg.Delete(ctx, draft.ID); err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to delete draft", err, errx.Attrs("draft_id", draft.ID))
		}
	}
	return nil
}

// clearIntake deletes every intake session of the group with its sources,
// detected products and file links. Drafts made from them are drafts like
// any other. RLS-scoped registry, as for the drafts.
func (l *RestoreOperationProcessor) clearIntake(ctx context.Context) error {
	intakeReg, err := l.factorySet.IntakeRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user intake registry for clear", err)
	}
	sessions, err := intakeReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list intake sessions for deletion", err)
	}
	for _, session := range sessions {
		if err := intakeReg.Delete(ctx, session.ID); err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to delete intake session", err, errx.Attrs("session_id", session.ID))
		}
	}
	return nil
}

// clearSurvivingCommodities deletes every commodity the location recursion did
// not reach (issue #2236). Enumerating commodities DIRECTLY — rather than adding
// an `area_id IS NULL` sweep — needs no new registry API and stays correct for the
// next nullable-parent change. Most rows are already gone by now, so their
// recursive delete short-circuits on ErrNotFound.
//
// RLS-scoped registry, for the same reason as the location list above: a service
// registry here would recursively delete EVERY tenant's commodities.
func (l *RestoreOperationProcessor) clearSurvivingCommodities(ctx context.Context) error {
	comReg, err := l.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to create user commodity registry for clear", err)
	}
	commodities, err := comReg.List(ctx)
	if err != nil {
		return errxtrace.Wrap("failed to list commodities for deletion", err)
	}
	for _, commodity := range commodities {
		if err := l.entityService.DeleteCommodityRecursive(ctx, commodity.ID); err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				// Already removed by the location recursion.
				continue
			}
			return errxtrace.Wrap("failed to delete commodity recursively", err, errx.Attrs("commodity_id", commodity.ID))
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
		allCommodities, err := listCommoditiesWithTrash(ctx, serviceAccountRegistry)
		if err != nil {
			stats.ErrorCount++
			msg := fmt.Sprintf("Failed to list commodities for ownership validation of %s: %v", originalXMLID, err)
			stats.Errors = append(stats.Errors, msg)
			slog.Error("commodity ownership validation list failed",
				"restore_operation_id", l.restoreOperationID,
				"original_xml_id", originalXMLID,
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

// listCommoditiesWithTrash returns every commodity the registry sees, the
// ones in Trash included. A trashed item keeps its UUID, and a restore creates
// items under their archived UUIDs, so the ownership check has to see trashed
// items as well.
func listCommoditiesWithTrash(ctx context.Context, reg registry.CommodityRegistry) ([]*models.Commodity, error) {
	all, err := reg.List(ctx)
	if err != nil {
		return nil, err
	}
	for offset := 0; ; offset += trashPageSize {
		page, total, err := reg.ListTrashed(ctx, offset, trashPageSize)
		if err != nil {
			return nil, err
		}
		all = append(all, page...)
		if len(page) == 0 || offset+len(page) >= total {
			return all, nil
		}
	}
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

// markRestoreFailed flips the operation to Failed, records a flattened message
// on the row + a failure step, and returns the wrapped cause so callers retain
// the original error chain (errors.Is/As still work on classified failures such
// as bad signatures or ownership violations).
func (l *RestoreOperationProcessor) markRestoreFailed(ctx context.Context, cause error, errorMessage string) error {
	failure := errxtrace.Wrap(errorMessage, cause)

	restoreOperationRegistry := l.factorySet.RestoreOperationRegistryFactory.CreateServiceRegistry()

	restoreOperation, err := restoreOperationRegistry.Get(ctx, l.restoreOperationID)
	if err != nil {
		return err
	}

	restoreOperation.Status = models.RestoreStatusFailed
	restoreOperation.CompletedDate = models.PNow()
	restoreOperation.ErrorMessage = failure.Error()

	if _, err = restoreOperationRegistry.Update(ctx, *restoreOperation); err != nil {
		return err
	}

	l.createRestoreStep(ctx, "Restore failed", models.RestoreStepResultError, failure.Error())
	return failure
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
	if existing != nil && existing.TrashedCommodities[originalID] != "" {
		// The item is in Trash: a merge leaves it there.
		stats.SkippedCount++
		return nil
	}
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
// commodity that doesn't already exist in the target group. A backup carries
// slugs, not labels, so a created tag is labelled from its slug.
func (l *RestoreOperationProcessor) ensureCommodityTags(ctx context.Context, commodity *models.Commodity, originalID string) error {
	if len(commodity.Tags) == 0 {
		return nil
	}
	slugs, err := l.tagService.NormalizeAndEnsureRestoredSlugs(ctx, models.TagKindCommodity, []string(commodity.Tags))
	if err != nil {
		return errxtrace.Wrap("failed to ensure tags for restored commodity", err, errx.Attrs("original_commodity_id", originalID))
	}
	commodity.Tags = models.ValuerSlice[string](slugs)
	return nil
}

// fileAction is the create/update/skip decision a restore strategy makes for a
// single file, given whether the file already exists. It is the single source
// of truth shared by the persistence step (applyStrategyForFileModel) and the
// `.inb` walker's pre-stream gate, so the blob-write decision and the row-write
// decision can never disagree (issue #2125).
type fileAction int

const (
	fileActionCreate fileAction = iota
	fileActionUpdate
	fileActionSkip
)

// decideFileStrategyAction maps (strategy, existing-file) to the create/update/
// skip action. existingFile is nil when no matching row is present. This is the
// non-dry-run decision; dry-run is handled by the caller (the persist functions
// short-circuit their writes, and the `.inb` walker drains bytes regardless).
func decideFileStrategyAction(strategy types.RestoreStrategy, existingFile *models.FileEntity) fileAction {
	switch strategy {
	case types.RestoreStrategyMergeAdd:
		if existingFile != nil {
			return fileActionSkip
		}
		return fileActionCreate
	case types.RestoreStrategyMergeUpdate:
		if existingFile != nil {
			return fileActionUpdate
		}
		return fileActionCreate
	case types.RestoreStrategyFullReplace:
		return fileActionCreate
	}
	return fileActionCreate
}

// applyStrategyForFileModel persists a decoded file row per strategy. The blob
// bytes were already streamed to the bucket by the per-build decode pass (which
// also gates the write on the same decideFileStrategyAction decision, so a
// MergeAdd-skip never leaves an orphan blob behind — issue #2125).
func (l *RestoreOperationProcessor) applyStrategyForFileModel(
	ctx context.Context,
	fileEntity *models.FileEntity,
	originalID string,
	stats *types.RestoreStats,
	existing *types.ExistingEntities,
	idMapping *types.IDMapping,
	options types.RestoreOptions,
) error {
	if existing.TrashedFiles[originalID] {
		// The file is in Trash: a merge leaves it there.
		stats.SkippedCount++
		return nil
	}
	existingFile := existing.Files[originalID]

	switch decideFileStrategyAction(options.Strategy, existingFile) {
	case fileActionSkip:
		stats.SkippedCount++
		return nil
	case fileActionUpdate:
		fileEntity.SetID(existingFile.ID)
		fileEntity.UUID = existingFile.UUID
		return l.updateFile(ctx, fileEntity, originalID, stats, existing, options)
	case fileActionCreate:
		return l.createFile(ctx, fileEntity, originalID, stats, existing, idMapping, options)
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
