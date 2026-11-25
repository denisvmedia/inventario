package services

import (
	"context"
	"errors"
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
// with a link to it, which purging the item later cleans up.
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
// as live: those entities have no Trash.
func hasLiveLinkBesides(ctx context.Context, comReg registry.CommodityRegistry, file *models.FileEntity, exceptCommodityID string) (bool, error) {
	for _, l := range file.Links {
		if l.EntityType != models.FileLinkCommodity {
			return true, nil
		}
		if l.EntityID == exceptCommodityID {
			continue
		}
		_, err := comReg.Get(ctx, l.EntityID)
		if err == nil {
			return true, nil
		}
		if !errors.Is(err, registry.ErrNotFound) {
			return false, errxtrace.Wrap("failed to look up linked item", err, errx.Attrs("commodity_id", l.EntityID))
		}
	}
	return false, nil
}

// CommodityRestoreResult describes a restored item.
type CommodityRestoreResult struct {
	Commodity *models.Commodity
	// AreaRemoved is true when the item's area was deleted while the item
	// was in Trash; the item came back without an area.
	AreaRemoved bool
	// FilesRestored counts the files that came back with the item.
	FilesRestored int
}

// RestoreCommodity brings a trashed item back with the files that went to
// Trash with an item and are linked to it, and records a "restored" event.
func (s *TrashService) RestoreCommodity(ctx context.Context, id string) (*CommodityRestoreResult, error) {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return nil, err
	}
	trashed, err := regs.commodities.GetTrashed(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get trashed item", err)
	}
	restored, err := regs.commodities.Restore(ctx, id)
	if err != nil {
		return nil, errxtrace.Wrap("failed to restore item", err)
	}
	result := &CommodityRestoreResult{
		Commodity:   restored,
		AreaRemoved: trashed.DeletedFromAreaID != nil && restored.AreaID == nil,
	}

	files, err := regs.files.ListTrashedLinkedTo(ctx, models.FileLinkCommodity, id)
	if err != nil {
		return result, errxtrace.Wrap("failed to list item files in Trash", err)
	}
	for _, file := range files {
		if file.DeletedWithCommodityID == nil {
			continue // deleted on its own; it stays in Trash
		}
		if err := regs.files.Restore(ctx, file.ID); err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				continue
			}
			return result, errxtrace.Wrap("failed to restore item file", err, errx.Attrs("file_id", file.ID))
		}
		result.FilesRestored++
	}

	s.events.EmitRestored(ctx, restored, RestoreDetails{AreaRemoved: result.AreaRemoved})
	return result, nil
}

// PurgeCommodity deletes a trashed item for good. ErrNotFound when the item is
// not in Trash.
func (s *TrashService) PurgeCommodity(ctx context.Context, id string) error {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return err
	}
	return s.purgeCommodity(ctx, regs, id)
}

func (s *TrashService) purgeCommodity(ctx context.Context, regs trashRegistries, id string) error {
	// Read before the links go: afterwards nothing ties the files to the item
	// but deleted_with_commodity_id.
	files, err := regs.files.ListTrashedLinkedTo(ctx, models.FileLinkCommodity, id)
	if err != nil {
		return errxtrace.Wrap("failed to list item files in Trash", err)
	}
	deleted, err := regs.commodities.PurgeTrashed(ctx, id)
	if err != nil {
		return errxtrace.Wrap("failed to purge item", err)
	}
	if !deleted {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity", "entity_id", id))
	}
	if _, err := regs.files.RemoveLinksToEntity(ctx, models.FileLinkCommodity, id); err != nil {
		return errxtrace.Wrap("failed to remove item file links", err, errx.Attrs("commodity_id", id))
	}
	for _, file := range files {
		if file.DeletedWithCommodityID == nil || *file.DeletedWithCommodityID != id {
			continue
		}
		if err := s.settleOrphanedTrashFile(ctx, regs, file.ID); err != nil {
			return err
		}
	}
	return nil
}

// settleOrphanedTrashFile decides the fate of a file whose item was purged:
// another trashed item that links it takes it over, a live entity that links
// it makes it live again, and with neither it is deleted with its blob.
func (s *TrashService) settleOrphanedTrashFile(ctx context.Context, regs trashRegistries, fileID string) error {
	file, err := regs.files.GetTrashed(ctx, fileID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil
	}
	if err != nil {
		return errxtrace.Wrap("failed to get trashed file", err, errx.Attrs("file_id", fileID))
	}

	holder := ""
	for _, l := range file.Links {
		if l.EntityType != models.FileLinkCommodity {
			return s.restoreFile(ctx, regs, file.ID)
		}
		if _, err := regs.commodities.Get(ctx, l.EntityID); err == nil {
			return s.restoreFile(ctx, regs, file.ID)
		} else if !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to look up linked item", err, errx.Attrs("commodity_id", l.EntityID))
		}
		if holder == "" {
			if _, err := regs.commodities.GetTrashed(ctx, l.EntityID); err == nil {
				holder = l.EntityID
			}
		}
	}
	if holder != "" {
		if err := regs.files.SetTrashedWith(ctx, file.ID, holder); err != nil && !errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Wrap("failed to hand file to another trashed item", err, errx.Attrs("file_id", file.ID))
		}
		return nil
	}
	if _, err := s.fileService.PurgeTrashedFile(ctx, regs.files, file); err != nil {
		return errxtrace.Wrap("failed to purge file", err, errx.Attrs("file_id", file.ID))
	}
	return nil
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

// EmptyTrash deletes every item and file in the group's Trash for good and
// reports how many of each it deleted.
func (s *TrashService) EmptyTrash(ctx context.Context) (items, files int, err error) {
	regs, err := s.userRegistries(ctx)
	if err != nil {
		return 0, 0, err
	}
	// Each round deletes what it read, so the next round reads from the top
	// again. An entry that cannot be deleted ends the loop with the error
	// instead of being read over and over.
	for {
		page, _, err := regs.commodities.ListTrashed(ctx, 0, emptyTrashBatch)
		if err != nil {
			return items, files, errxtrace.Wrap("failed to list trashed items", err)
		}
		if len(page) == 0 {
			break
		}
		for _, c := range page {
			if err := s.purgeCommodity(ctx, regs, c.ID); err != nil {
				if errors.Is(err, registry.ErrNotFound) {
					continue
				}
				return items, files, err
			}
			items++
		}
	}
	for {
		page, _, err := regs.files.ListTrashed(ctx, 0, emptyTrashBatch)
		if err != nil {
			return items, files, errxtrace.Wrap("failed to list trashed files", err)
		}
		if len(page) == 0 {
			break
		}
		progressed := false
		for _, f := range page {
			deleted, err := s.fileService.PurgeTrashedFile(ctx, regs.files, f)
			if err != nil {
				return items, files, errxtrace.Wrap("failed to purge file", err, errx.Attrs("file_id", f.ID))
			}
			if deleted {
				files++
				progressed = true
			}
		}
		if !progressed {
			break
		}
	}
	return items, files, nil
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
// batch files that went to Trash before cutoff. It runs on service registries
// and is safe to repeat: every step re-reads its state and an entry that is
// gone or restored in the meantime is skipped.
func (s *TrashService) PurgeExpired(ctx context.Context, cutoff time.Time, batch int) (items, files int, err error) {
	regs := s.serviceRegistries()
	expiredItems, err := regs.commodities.ListTrashedBefore(ctx, cutoff, batch)
	if err != nil {
		return 0, 0, errxtrace.Wrap("failed to list expired trashed items", err)
	}
	for _, c := range expiredItems {
		if err := s.purgeCommodity(ctx, regs, c.ID); err != nil {
			if errors.Is(err, registry.ErrNotFound) {
				continue
			}
			return items, files, errxtrace.Wrap("failed to purge expired item", err, errx.Attrs("commodity_id", c.ID))
		}
		items++
	}

	expiredFiles, err := regs.files.ListTrashedBefore(ctx, cutoff, batch)
	if err != nil {
		return items, files, errxtrace.Wrap("failed to list expired trashed files", err)
	}
	for _, f := range expiredFiles {
		if f.DeletedWithCommodityID != nil {
			// Its item is gone (ListTrashedBefore returns only those): an
			// earlier purge stopped before it settled the file.
			if err := s.settleOrphanedTrashFile(ctx, regs, f.ID); err != nil {
				return items, files, err
			}
			files++
			continue
		}
		deleted, err := s.fileService.PurgeTrashedFile(ctx, regs.files, f)
		if err != nil {
			return items, files, errxtrace.Wrap("failed to purge expired file", err, errx.Attrs("file_id", f.ID))
		}
		if deleted {
			files++
		}
	}
	return items, files, nil
}
