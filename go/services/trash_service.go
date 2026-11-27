package services

import (
	"context"
	"errors"
	"slices"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// DefaultTrashRetention is how long an item or file stays in Trash before the
// purge worker deletes it for good.
const DefaultTrashRetention = 30 * 24 * time.Hour

// ErrTrashedWithItem is returned for a file that went to Trash with an item:
// it comes back, or goes for good, with that item.
var ErrTrashedWithItem = errx.NewSentinel("the file went to Trash with an item; restore or delete the item instead")

// Audit actions of the Trash (#2893). A restore, a delete for good (one entry
// or several) and an Empty Trash each write one row for the group; the purge
// worker writes one row per run for each group it deleted entries in.
const (
	AuditActionTrashRestore      = "trash_restore"
	AuditActionTrashPurge        = "trash_purge"
	AuditActionTrashEmpty        = "trash_empty"
	AuditActionTrashPurgeExpired = "trash_purge_expired"
)

// TrashAuditIDLimit is how many IDs of each kind an audit row lists. The
// counts are always exact; a row whose lists were cut says so in
// ids_truncated. The breadcrumb lives in a text column that audit scans read,
// and emptying a large Trash would otherwise write one row of megabytes.
const TrashAuditIDLimit = 1000

// TrashOutcome is what a Trash restore or delete for good did, by ID: the
// items and the files it restored or deleted. A file is listed only when the
// operation restored or deleted it; a file it left where it was (still used
// by a live entity, or handed to another item in Trash) is not.
type TrashOutcome struct {
	ItemIDs []string
	FileIDs []string
}

func (o *TrashOutcome) add(other TrashOutcome) {
	o.ItemIDs = append(o.ItemIDs, other.ItemIDs...)
	o.FileIDs = append(o.FileIDs, other.FileIDs...)
}

// Empty reports whether the operation restored or deleted nothing.
func (o TrashOutcome) Empty() bool {
	return len(o.ItemIDs) == 0 && len(o.FileIDs) == 0
}

// AuditExtra is the outcome as an audit row records it: the number and the
// IDs of the items and of the files (see TrashAuditIDLimit).
func (o TrashOutcome) AuditExtra() map[string]any {
	items, itemsCut := auditIDs(o.ItemIDs)
	files, filesCut := auditIDs(o.FileIDs)
	extra := map[string]any{
		"items":    len(o.ItemIDs),
		"item_ids": items,
		"files":    len(o.FileIDs),
		"file_ids": files,
	}
	if itemsCut || filesCut {
		extra["ids_truncated"] = true
	}
	return extra
}

// auditIDs returns ids cut to TrashAuditIDLimit, never nil so the row lists
// an empty array rather than null, and whether it cut them.
func auditIDs(ids []string) ([]string, bool) {
	if len(ids) > TrashAuditIDLimit {
		return ids[:TrashAuditIDLimit], true
	}
	if ids == nil {
		return []string{}, false
	}
	return ids, false
}

// GroupTrashOutcome is what the purge worker deleted in one group.
type GroupTrashOutcome struct {
	TenantID string
	GroupID  string
	TrashOutcome
}

// trashGroups collects outcomes per group, in the order the groups first
// deleted something.
type trashGroups struct {
	order []string
	by    map[string]*GroupTrashOutcome
}

func (g *trashGroups) add(tenantID, groupID string, out TrashOutcome) {
	if out.Empty() {
		return
	}
	if g.by == nil {
		g.by = map[string]*GroupTrashOutcome{}
	}
	key := tenantID + "/" + groupID
	entry, ok := g.by[key]
	if !ok {
		entry = &GroupTrashOutcome{TenantID: tenantID, GroupID: groupID}
		g.by[key] = entry
		g.order = append(g.order, key)
	}
	entry.add(out)
}

func (g *trashGroups) list() []GroupTrashOutcome {
	out := make([]GroupTrashOutcome, 0, len(g.order))
	for _, key := range g.order {
		out = append(out, *g.by[key])
	}
	return out
}

// TrashService moves items and files to Trash, restores them and deletes them
// for good.
//
// An item in Trash keeps its row, history, loans, service records, maintenance
// schedules and supply links; every ordinary registry read leaves it out (see
// registry.CommodityRegistry). Its files follow one rule: a file is live while
// it is linked to a live entity, or to nothing. Trashing an item therefore
// takes along the files no live entity still uses, and records the item in
// the file's deleted_with_commodity_id. A file shared with another live item,
// an area or a location stays where it is.
//
// Restoring an item brings back the files that went to Trash with an item and
// are linked to it. A file deleted on its own before stays in Trash: that was
// a separate decision.
//
// Purging an item deletes its row (the history and the other dependents go
// through ON DELETE CASCADE) and its links. A file that went with it is
// handed to another trashed item that still links it, made live when a live
// entity links it, and deleted with its blob otherwise. The blob delete keeps
// bytes that another row, live or trashed, still references.
type TrashService struct {
	factorySet  *registry.FactorySet
	fileService *FileService
	events      *CommodityEventService
	now         func() time.Time
}

// NewTrashService builds the service.
func NewTrashService(factorySet *registry.FactorySet, uploadLocation string) *TrashService {
	return &TrashService{
		factorySet:  factorySet,
		fileService: NewFileService(factorySet, uploadLocation),
		events:      NewCommodityEventService(factorySet),
		now:         time.Now,
	}
}

// WithClock replaces the clock; tests use it to age Trash entries.
func (s *TrashService) WithClock(now func() time.Time) *TrashService {
	s.now = now
	return s
}

// trashRegistries are the registries one operation works through: the
// caller's group-scoped registries for a request, service registries for the
// purge worker.
type trashRegistries struct {
	commodities registry.CommodityRegistry
	files       registry.FileRegistry
}

func (s *TrashService) userRegistries(ctx context.Context) (trashRegistries, error) {
	comReg, err := s.factorySet.CommodityRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return trashRegistries{}, errxtrace.Wrap("failed to create commodity registry", err)
	}
	fileReg, err := s.factorySet.FileRegistryFactory.CreateUserRegistry(ctx)
	if err != nil {
		return trashRegistries{}, errxtrace.Wrap("failed to create file registry", err)
	}
	return trashRegistries{commodities: comReg, files: fileReg}, nil
}

func (s *TrashService) serviceRegistries() trashRegistries {
	return trashRegistries{
		commodities: s.factorySet.CommodityRegistryFactory.CreateServiceRegistry(),
		files:       s.factorySet.FileRegistryFactory.CreateServiceRegistry(),
	}
}

func actorID(ctx context.Context) (string, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return "", errxtrace.Wrap("failed to get user from context", err)
	}
	return user.ID, nil
}

// TrashCommodity moves a live item to Trash with the files no live entity
// still uses, and records a "deleted" event in its history. The item row goes
// first: if moving a file fails, the item is in Trash and the file stays live
// with a link to it. Purging the item removes that link and leaves the file
// live, as a standalone file.
func (s *TrashService) TrashCommodity(ctx context.Context, id string) (*models.Commodity, error) {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return nil, err
	}
	userID, err := actorID(ctx)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()

	commodity, err := regs.commodities.Trash(ctx, id, userID, now)
	if err != nil {
		return nil, errxtrace.Wrap("failed to move item to Trash", err)
	}
	s.events.EmitDeleted(ctx, commodity)

	files, err := regs.files.ListByLinkedEntity(ctx, models.FileLinkCommodity, id)
	if err != nil {
		return commodity, errxtrace.Wrap("failed to list item files", err)
	}
	for _, file := range files {
		keep, err := hasLiveLinkBesides(ctx, regs.commodities, file, id)
		if err != nil {
			return commodity, err
		}
		if keep {
			continue
		}
		if err := regs.files.Trash(ctx, file.ID, userID, id, now); err != nil && !errors.Is(err, registry.ErrNotFound) {
			return commodity, errxtrace.Wrap("failed to move item file to Trash", err, errx.Attrs("file_id", file.ID))
		}
	}
	return commodity, nil
}

// hasLiveLinkBesides reports whether the file is linked to a live entity other
// than the item exceptCommodityID. Links to areas, locations and exports count
// as live: those entities have no Trash. So does a link to an item that is
// neither live nor in Trash (see linkedItemState).
func hasLiveLinkBesides(ctx context.Context, comReg registry.CommodityRegistry, file *models.FileEntity, exceptCommodityID string) (bool, error) {
	for _, l := range file.Links {
		if l.EntityType != models.FileLinkCommodity {
			return true, nil
		}
		if l.EntityID == exceptCommodityID {
			continue
		}
		state, err := linkedItemState(ctx, comReg, l.EntityID)
		if err != nil {
			return false, err
		}
		if state != itemInTrash {
			return true, nil
		}
	}
	return false, nil
}

type itemState int

const (
	itemLive itemState = iota
	itemInTrash
	itemUnknown
)

// linkedItemState looks up an item that a file links to.
//
// Every link of a file points at an entity of the file's own group: an upload
// links to the entity of its request, FileLinkService.Attach checks the target
// in the caller's group (requireTarget), and a restore links a file only to
// entities it restored into the same group. A group-scoped registry therefore
// sees every item a file links to, and an item it cannot find was purged. The
// callers do not rely on that alone: they treat an unknown item like a live
// one, so a file is never moved to Trash or purged because a lookup missed.
func linkedItemState(ctx context.Context, comReg registry.CommodityRegistry, id string) (itemState, error) {
	_, err := comReg.Get(ctx, id)
	if err == nil {
		return itemLive, nil
	}
	if !errors.Is(err, registry.ErrNotFound) {
		return 0, errxtrace.Wrap("failed to look up linked item", err, errx.Attrs("commodity_id", id))
	}
	_, err = comReg.GetTrashed(ctx, id)
	if err == nil {
		return itemInTrash, nil
	}
	if !errors.Is(err, registry.ErrNotFound) {
		return 0, errxtrace.Wrap("failed to look up linked item in Trash", err, errx.Attrs("commodity_id", id))
	}
	return itemUnknown, nil
}

// CommodityRestoreResult describes a restored item.
type CommodityRestoreResult struct {
	Commodity *models.Commodity
	// AreaRemoved is true when the item's area was deleted while the item
	// was in Trash; the item came back without an area.
	AreaRemoved bool
	// FilesRestored counts the files that came back with the item.
	FilesRestored int
	// FileIDs are the files that came back with the item.
	FileIDs []string
}

// RestoreCommodity brings a trashed item back with the files that went to
// Trash with an item and are linked to it, and records a "restored" event. On
// an error the result, when not nil, lists the files restored before it.
//
// The files come back first and the item last, so a failure leaves the item in
// Trash and a retry restores the rest. A file that a concurrent delete moves
// to Trash after the item came back is left with a live item; the stranded
// file pass (settleStrandedFiles) restores it.
func (s *TrashService) RestoreCommodity(ctx context.Context, id string) (*CommodityRestoreResult, error) {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return nil, err
	}
	trashed, err := regs.commodities.GetTrashed(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get trashed item", err)
	}

	files, err := regs.files.ListTrashedLinkedTo(ctx, models.FileLinkCommodity, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list item files in Trash", err)
	}
	var fileIDs []string
	for _, file := range files {
		if file.DeletedWithCommodityID == nil {
			continue // deleted on its own; it stays in Trash
		}
		if err := regs.files.Restore(ctx, file.ID); err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				continue
			}
			return &CommodityRestoreResult{FilesRestored: len(fileIDs), FileIDs: fileIDs},
				errxtrace.Wrap("failed to restore item file", err, errx.Attrs("file_id", file.ID))
		}
		fileIDs = append(fileIDs, file.ID)
	}

	restored, err := regs.commodities.Restore(ctx, id)
	if err != nil {
		return &CommodityRestoreResult{FilesRestored: len(fileIDs), FileIDs: fileIDs}, errxtrace.Wrap("failed to restore item", err)
	}
	result := &CommodityRestoreResult{
		Commodity:     restored,
		AreaRemoved:   trashed.DeletedFromAreaID != nil && restored.AreaID == nil,
		FilesRestored: len(fileIDs),
		FileIDs:       fileIDs,
	}
	s.events.EmitRestored(ctx, restored, RestoreDetails{AreaRemoved: result.AreaRemoved})
	return result, nil
}

// PurgeCommodity deletes a trashed item for good and reports what it deleted,
// also when it fails partway. ErrNotFound when the item is not in Trash.
func (s *TrashService) PurgeCommodity(ctx context.Context, id string) (TrashOutcome, error) {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return TrashOutcome{}, err
	}
	return s.purgeCommodity(ctx, regs, id)
}

// purgeCommodity deletes a trashed item and settles the files that went to
// Trash with it. It never moves a live file to Trash or deletes one: a live
// file linked to the item (restored on its own while the item was in Trash,
// linked by a merge restore, or missed by the delete) only loses its link to
// the item and stays live. The outcome lists what was deleted before an
// error too.
func (s *TrashService) purgeCommodity(ctx context.Context, regs trashRegistries, id string) (TrashOutcome, error) {
	var out TrashOutcome
	if _, err := regs.commodities.GetTrashed(ctx, id); err != nil {
		return out, errxtrace.Wrap("failed to get trashed item", err)
	}
	// Read before the links go: afterwards nothing ties the files to the item
	// but deleted_with_commodity_id.
	files, err := regs.files.ListTrashedLinkedTo(ctx, models.FileLinkCommodity, id)
	if err != nil {
		return out, errxtrace.Wrap("failed to list item files in Trash", err)
	}
	deleted, err := regs.commodities.PurgeTrashed(ctx, id)
	if err != nil {
		return out, errxtrace.Wrap("failed to purge item", err)
	}
	if !deleted {
		return out, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity", "entity_id", id))
	}
	out.ItemIDs = []string{id}
	if _, err := regs.files.RemoveLinksToEntity(ctx, models.FileLinkCommodity, id); err != nil {
		return out, errxtrace.Wrap("failed to remove item file links", err, errx.Attrs("commodity_id", id))
	}
	for _, file := range files {
		if file.DeletedWithCommodityID == nil || *file.DeletedWithCommodityID != id {
			continue
		}
		gone, err := s.settleTrashedFile(ctx, regs, file.ID)
		if err != nil {
			return out, err
		}
		if gone {
			out.FileIDs = append(out.FileIDs, file.ID)
		}
	}
	return out, nil
}

// settleTrashedFile decides the fate of a file that went to Trash with an item
// that is no longer in Trash, and reports whether it deleted the file. A live
// entity that links it makes it live again, which also covers an item that
// came back without it; another trashed item that links it takes it over;
// with neither it is deleted with its blob. A link to an item that is neither
// live nor in Trash keeps the file (see linkedItemState), except a link to the
// item the file went with: that item was purged, and its link is one the purge
// did not get to remove.
func (s *TrashService) settleTrashedFile(ctx context.Context, regs trashRegistries, fileID string) (bool, error) {
	file, err := regs.files.GetTrashed(ctx, fileID)
	if errors.Is(err, registry.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, errxtrace.Wrap("failed to get trashed file", err, errx.Attrs("file_id", fileID))
	}
	if file.DeletedWithCommodityID == nil {
		return false, nil // a Trash entry of its own
	}

	ownerID := *file.DeletedWithCommodityID
	owner, err := linkedItemState(ctx, regs.commodities, ownerID)
	if err != nil {
		return false, err
	}
	switch owner {
	case itemInTrash:
		return false, nil // the item still holds it
	case itemUnknown:
		if file.LinkTo(models.FileLinkCommodity, ownerID) != nil {
			if _, err := regs.files.RemoveLinksToEntity(ctx, models.FileLinkCommodity, ownerID); err != nil {
				return false, errxtrace.Wrap("failed to remove links to a purged item", err, errx.Attrs("commodity_id", ownerID))
			}
			file.Links = slices.DeleteFunc(file.Links, func(l models.FileLink) bool {
				return l.EntityType == models.FileLinkCommodity && l.EntityID == ownerID
			})
		}
	case itemLive:
	}

	holder := ""
	for _, l := range file.Links {
		if l.EntityType != models.FileLinkCommodity {
			return false, s.restoreFile(ctx, regs, file.ID)
		}
		state, err := linkedItemState(ctx, regs.commodities, l.EntityID)
		if err != nil {
			return false, err
		}
		if state != itemInTrash {
			return false, s.restoreFile(ctx, regs, file.ID)
		}
		if holder == "" {
			holder = l.EntityID
		}
	}
	if holder != "" {
		if err := regs.files.SetTrashedWith(ctx, file.ID, holder); err != nil && !errors.Is(err, registry.ErrNotFound) {
			return false, errxtrace.Wrap("failed to hand file to another trashed item", err, errx.Attrs("file_id", file.ID))
		}
		return false, nil
	}
	deleted, err := s.fileService.PurgeTrashedFile(ctx, regs.files, file)
	if err != nil {
		return false, errxtrace.Wrap("failed to purge file", err, errx.Attrs("file_id", file.ID))
	}
	return deleted, nil
}

// strandedBatch is how many stranded files one round of settleStrandedFiles
// reads.
const strandedBatch = 100

// settleStrandedFiles settles up to limit trashed files whose item is no
// longer in Trash (registry.FileRegistry.ListStrandedTrashed): the files an
// interrupted purge, restore or delete left with no Trash entry to show them.
// A file that fails is skipped and its error is joined into the result. It
// reports how many files it looked at and the ones it deleted.
func (s *TrashService) settleStrandedFiles(ctx context.Context, regs trashRegistries, limit int) (seen int, deleted []*models.FileEntity, err error) {
	files, err := regs.files.ListStrandedTrashed(ctx, limit)
	if err != nil {
		return 0, nil, errxtrace.Wrap("failed to list stranded trashed files", err)
	}
	var errs []error
	for _, f := range files {
		gone, err := s.settleTrashedFile(ctx, regs, f.ID)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if gone {
			deleted = append(deleted, f)
		}
	}
	return len(files), deleted, errors.Join(errs...)
}

func (s *TrashService) restoreFile(ctx context.Context, regs trashRegistries, fileID string) error {
	if err := regs.files.Restore(ctx, fileID); err != nil && !errors.Is(err, registry.ErrNotFound) {
		return errxtrace.Wrap("failed to restore file", err, errx.Attrs("file_id", fileID))
	}
	return nil
}

// TrashFile moves a live file to Trash on its own. Its links stay, so a
// restore puts it back where it was. An export bundle is not inventory and has
// no Trash: it is deleted for good.
func (s *TrashService) TrashFile(ctx context.Context, id string) error {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return err
	}
	userID, err := actorID(ctx)
	if err != nil {
		return err
	}
	file, err := regs.files.Get(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get file", err)
	}
	if file.IsExportOwned() {
		return s.fileService.DeleteFileWithPhysical(ctx, id)
	}
	if err := regs.files.Trash(ctx, id, userID, "", s.now().UTC()); err != nil {
		return errxtrace.Wrap("failed to move file to Trash", err)
	}
	return nil
}

// RestoreFile brings back a file that was deleted on its own.
// ErrTrashedWithItem for a file that went to Trash with an item.
func (s *TrashService) RestoreFile(ctx context.Context, id string) error {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return err
	}
	file, err := regs.files.GetTrashed(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get trashed file", err)
	}
	if file.DeletedWithCommodityID != nil {
		return errxtrace.Classify(ErrTrashedWithItem, errx.Attrs("file_id", id))
	}
	if err := regs.files.Restore(ctx, id); err != nil {
		return errxtrace.Wrap("failed to restore file", err)
	}
	return nil
}

// PurgeFile deletes a file that was deleted on its own for good.
// ErrTrashedWithItem for a file that went to Trash with an item.
func (s *TrashService) PurgeFile(ctx context.Context, id string) error {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return err
	}
	file, err := regs.files.GetTrashed(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to get trashed file", err)
	}
	if file.DeletedWithCommodityID != nil {
		return errxtrace.Classify(ErrTrashedWithItem, errx.Attrs("file_id", id))
	}
	deleted, err := s.fileService.PurgeTrashedFile(ctx, regs.files, file)
	if err != nil {
		return errxtrace.Wrap("failed to purge file", err)
	}
	if !deleted {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
	}
	return nil
}

// emptyTrashBatch is how many entries one round of EmptyTrash reads.
const emptyTrashBatch = 100

// EmptyTrashResult is what EmptyTrash deleted.
type EmptyTrashResult struct {
	TrashOutcome
	// FileEntries counts the deleted files that were not deleted with an
	// item: the files deleted on their own, and those an interrupted purge
	// left behind. With len(ItemIDs) it is the number of entries the Trash
	// page reports; FileIDs also lists the files that went with an item.
	FileEntries int
}

// EmptyTrash deletes every item and file in the group's Trash for good and
// reports what it deleted, also when it fails partway.
func (s *TrashService) EmptyTrash(ctx context.Context) (EmptyTrashResult, error) {
	var out TrashOutcome
	withItems := 0
	result := func() EmptyTrashResult {
		return EmptyTrashResult{TrashOutcome: out, FileEntries: len(out.FileIDs) - withItems}
	}
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return result(), err
	}
	err = s.purgeAllItems(ctx, regs, &out)
	withItems = len(out.FileIDs)
	if err != nil {
		return result(), err
	}
	if err := s.purgeAllFiles(ctx, regs, &out); err != nil {
		return result(), err
	}
	// Files an interrupted purge left behind are in neither list.
	for {
		seen, deleted, err := s.settleStrandedFiles(ctx, regs, strandedBatch)
		for _, f := range deleted {
			out.FileIDs = append(out.FileIDs, f.ID)
		}
		if err != nil {
			return result(), err
		}
		if seen < strandedBatch {
			return result(), nil
		}
	}
}

// purgeAllItems purges every item in the group's Trash into out. Each round
// deletes what it read, so the next round reads from the top again. An entry
// that cannot be deleted ends the loop with the error instead of being read
// over and over.
func (s *TrashService) purgeAllItems(ctx context.Context, regs trashRegistries, out *TrashOutcome) error {
	for {
		page, _, err := regs.commodities.ListTrashed(ctx, 0, emptyTrashBatch)
		if err != nil {
			return errxtrace.Wrap("failed to list trashed items", err)
		}
		if len(page) == 0 {
			return nil
		}
		for _, c := range page {
			purged, err := s.purgeCommodity(ctx, regs, c.ID)
			out.add(purged)
			if err != nil {
				if errors.Is(err, registry.ErrNotFound) {
					continue
				}
				return err
			}
		}
	}
}

// purgeAllFiles purges every file deleted on its own in the group's Trash
// into out.
func (s *TrashService) purgeAllFiles(ctx context.Context, regs trashRegistries, out *TrashOutcome) error {
	for {
		page, _, err := regs.files.ListTrashed(ctx, 0, emptyTrashBatch)
		if err != nil {
			return errxtrace.Wrap("failed to list trashed files", err)
		}
		if len(page) == 0 {
			return nil
		}
		progressed := false
		for _, f := range page {
			deleted, err := s.fileService.PurgeTrashedFile(ctx, regs.files, f)
			if err != nil {
				return errxtrace.Wrap("failed to purge file", err, errx.Attrs("file_id", f.ID))
			}
			if deleted {
				out.FileIDs = append(out.FileIDs, f.ID)
				progressed = true
			}
		}
		if !progressed {
			return nil
		}
	}
}

// TrashedCommodity is one item in the Trash list.
type TrashedCommodity struct {
	Commodity *models.Commodity
	// DeletedByName is the name of the member who deleted the item, empty
	// when that user no longer exists.
	DeletedByName string
	// ExpiresAt is when the purge worker deletes the item for good.
	ExpiresAt time.Time
	// Files counts the files that went to Trash with the item.
	Files registry.TrashedFilesStats
}

// TrashedFile is one file in the Trash list.
type TrashedFile struct {
	File          *models.FileEntity
	DeletedByName string
	ExpiresAt     time.Time
}

// ListTrashedCommodities returns a page of the group's Trash items with the
// details the Trash page shows.
func (s *TrashService) ListTrashedCommodities(ctx context.Context, offset, limit int, retention time.Duration) ([]TrashedCommodity, int, error) {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return nil, 0, err
	}
	items, total, err := regs.commodities.ListTrashed(ctx, offset, limit)
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list trashed items", err)
	}
	ids := make([]string, 0, len(items))
	for _, c := range items {
		ids = append(ids, c.ID)
	}
	stats, err := regs.files.CountTrashedWith(ctx, ids)
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to count item files in Trash", err)
	}
	names := s.userNames(ctx)
	out := make([]TrashedCommodity, 0, len(items))
	for _, c := range items {
		out = append(out, TrashedCommodity{
			Commodity:     c,
			DeletedByName: names(c.DeletedBy),
			ExpiresAt:     expiresAt(c.DeletedAt, retention),
			Files:         stats[c.ID],
		})
	}
	return out, total, nil
}

// ListTrashedFiles returns a page of the files deleted on their own.
func (s *TrashService) ListTrashedFiles(ctx context.Context, offset, limit int, retention time.Duration) ([]TrashedFile, int, error) {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return nil, 0, err
	}
	files, total, err := regs.files.ListTrashed(ctx, offset, limit)
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list trashed files", err)
	}
	names := s.userNames(ctx)
	out := make([]TrashedFile, 0, len(files))
	for _, f := range files {
		out = append(out, TrashedFile{
			File:          f,
			DeletedByName: names(f.DeletedBy),
			ExpiresAt:     expiresAt(f.DeletedAt, retention),
		})
	}
	return out, total, nil
}

func expiresAt(deletedAt *time.Time, retention time.Duration) time.Time {
	if deletedAt == nil {
		return time.Time{}
	}
	return deletedAt.Add(retention).UTC()
}

// userNames returns a lookup of user display names, cached for one listing.
func (s *TrashService) userNames(ctx context.Context) func(*string) string {
	cache := map[string]string{}
	return func(id *string) string {
		if id == nil || *id == "" || s.factorySet.UserRegistry == nil {
			return ""
		}
		if name, ok := cache[*id]; ok {
			return name
		}
		name := ""
		if user, err := s.factorySet.UserRegistry.Get(ctx, *id); err == nil && user != nil {
			name = user.Name
			if name == "" {
				name = user.Email
			}
		}
		cache[*id] = name
		return name
	}
}

// PurgeExpired deletes, across every tenant and group, up to batch items and
// batch files that went to Trash before cutoff, and settles up to batch files
// whose item is no longer in Trash (settleStrandedFiles). It runs on service
// registries and is safe to repeat: every step re-reads its state and an entry
// that is gone or restored in the meantime is skipped. An entry that fails is
// skipped too, so one bad row does not hold up the rest; the errors are joined
// into the result. It reports what it deleted in each group.
func (s *TrashService) PurgeExpired(ctx context.Context, cutoff time.Time, batch int) ([]GroupTrashOutcome, error) {
	regs := s.serviceRegistries()
	var groups trashGroups
	var errs []error

	expiredItems, err := regs.commodities.ListTrashedBefore(ctx, cutoff, batch)
	if err != nil {
		errs = append(errs, errxtrace.Wrap("failed to list expired trashed items", err))
	}
	for _, c := range expiredItems {
		purged, err := s.purgeCommodity(ctx, regs, c.ID)
		groups.add(c.TenantID, c.GroupID, purged)
		if err != nil && !errors.Is(err, registry.ErrNotFound) {
			errs = append(errs, errxtrace.Wrap("failed to purge expired item", err, errx.Attrs("commodity_id", c.ID)))
		}
	}

	_, stranded, err := s.settleStrandedFiles(ctx, regs, batch)
	for _, f := range stranded {
		groups.add(f.TenantID, f.GroupID, TrashOutcome{FileIDs: []string{f.ID}})
	}
	if err != nil {
		errs = append(errs, err)
	}

	expiredFiles, err := regs.files.ListTrashedBefore(ctx, cutoff, batch)
	if err != nil {
		errs = append(errs, errxtrace.Wrap("failed to list expired trashed files", err))
	}
	for _, f := range expiredFiles {
		deleted, err := s.fileService.PurgeTrashedFile(ctx, regs.files, f)
		if err != nil {
			errs = append(errs, errxtrace.Wrap("failed to purge expired file", err, errx.Attrs("file_id", f.ID)))
			continue
		}
		if deleted {
			groups.add(f.TenantID, f.GroupID, TrashOutcome{FileIDs: []string{f.ID}})
		}
	}
	return groups.list(), errors.Join(errs...)
}
