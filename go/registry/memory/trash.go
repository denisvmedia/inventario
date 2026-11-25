package memory

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// listAll returns every visible row, trashed rows included. The ordinary List
// leaves trashed rows out; the purgers, the ownership checker and the tag
// rewrites must reach them too.
func (r *Registry[_, P]) listAll() []P {
	r.lock.RLock()
	defer r.lock.RUnlock()
	items := make([]P, 0, r.items.Len())
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if !r.isItemVisible(pair.Value) {
			continue
		}
		v := *pair.Value
		items = append(items, &v)
	}
	return items
}

// baseRegistry gives the purgers the embedded base registry of a concrete
// memory registry.
func (r *Registry[T, P]) baseRegistry() *Registry[T, P] {
	return r
}

// serviceRows returns a list function that reaches every row of reg, trashed
// rows and the rows of trashed items included. The purgers and the ownership
// checker use it: they must not leave such rows behind. A registry that is not
// a memory registry falls back to its own List.
func serviceRows[T any, P registry.PIDable[T]](reg any, fallback func(context.Context) ([]P, error)) func(context.Context) ([]P, error) {
	b, ok := reg.(interface{ baseRegistry() *Registry[T, P] })
	if !ok {
		return fallback
	}
	base := b.baseRegistry()
	return func(context.Context) ([]P, error) {
		return base.listAll(), nil
	}
}

// trashedCommodities answers whether an item is in Trash for the registries of
// its loans, service records and maintenance schedules, whose group-wide reads
// leave the rows of trashed items out. Nil (a registry built outside
// NewFactorySet) treats every item as live.
type trashedCommodities struct {
	factory *CommodityRegistryFactory
}

func (t *trashedCommodities) isTrashed(commodityID string) bool {
	if t == nil || t.factory == nil {
		return false
	}
	base := t.factory.baseCommodityRegistry
	base.lock.RLock()
	defer base.lock.RUnlock()
	c, ok := base.items.Get(commodityID)
	return ok && c != nil && c.IsTrashed()
}

// dropRowsOfTrashed removes the rows whose item is in Trash.
func dropRowsOfTrashed[T any](rows []T, commodityID func(T) string, trashed *trashedCommodities) []T {
	if trashed == nil || trashed.factory == nil {
		return rows
	}
	return slices.DeleteFunc(rows, func(row T) bool {
		return trashed.isTrashed(commodityID(row))
	})
}

// GetTrashed returns a trashed item.
func (r *CommodityRegistry) GetTrashed(_ context.Context, id string) (*models.Commodity, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()
	c, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(c) || !c.IsTrashed() {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity", "entity_id", id))
	}
	cp := *c
	return &cp, nil
}

// trashedItems returns copies of the visible trashed items that pass keep.
func (r *CommodityRegistry) trashedItems(keep func(*models.Commodity) bool) []*models.Commodity {
	r.lock.RLock()
	defer r.lock.RUnlock()
	var out []*models.Commodity
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		if c == nil || !c.IsTrashed() || !r.isItemVisible(c) || !keep(c) {
			continue
		}
		cp := *c
		out = append(out, &cp)
	}
	return out
}

// ListTrashed returns a page of trashed items, most recently deleted first.
func (r *CommodityRegistry) ListTrashed(_ context.Context, offset, limit int) ([]*models.Commodity, int, error) {
	all := r.trashedItems(func(*models.Commodity) bool { return true })
	slices.SortStableFunc(all, func(a, b *models.Commodity) int {
		if c := b.DeletedAt.Compare(*a.DeletedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	return page(all, offset, limit), len(all), nil
}

// ListTrashedBefore returns up to limit items deleted before cutoff, oldest
// first.
func (r *CommodityRegistry) ListTrashedBefore(_ context.Context, cutoff time.Time, limit int) ([]*models.Commodity, error) {
	if limit <= 0 {
		return nil, nil
	}
	all := r.trashedItems(func(c *models.Commodity) bool { return c.DeletedAt.Before(cutoff) })
	slices.SortStableFunc(all, func(a, b *models.Commodity) int {
		if c := a.DeletedAt.Compare(*b.DeletedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
	return page(all, 0, limit), nil
}

// Trash moves a live item to Trash. It also drops the item from its area's
// index, so a trashed item does not keep the area from being deleted.
func (r *CommodityRegistry) Trash(ctx context.Context, id, deletedBy string, at time.Time) (*models.Commodity, error) {
	r.lock.Lock()
	c, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(c) || c.IsTrashed() {
		r.lock.Unlock()
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity", "entity_id", id))
	}
	next := *c
	next.DeletedAt = new(at.UTC())
	next.DeletedBy = new(deletedBy)
	next.DeletedFromAreaID = nil
	if c.AreaID != nil {
		next.DeletedFromAreaID = new(*c.AreaID)
	}
	r.items.Set(id, &next)
	r.lock.Unlock()

	if next.AreaID != nil && *next.AreaID != "" {
		_ = r.areaRegistry.DeleteCommodity(ctx, *next.AreaID, id)
	}
	out := next
	return &out, nil
}

// Restore makes a trashed item live again. An item whose area was deleted
// while it was in Trash comes back unassigned, as in the postgres registry,
// where deleting the area clears area_id of its trashed items.
func (r *CommodityRegistry) Restore(ctx context.Context, id string) (*models.Commodity, error) {
	r.lock.RLock()
	c, ok := r.items.Get(id)
	visible := ok && r.isItemVisible(c) && c.IsTrashed()
	var areaID string
	if visible && c.AreaID != nil {
		areaID = *c.AreaID
	}
	r.lock.RUnlock()
	if !visible {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity", "entity_id", id))
	}
	areaExists := false
	if areaID != "" {
		_, err := r.areaRegistry.Get(ctx, areaID)
		areaExists = err == nil
	}

	r.lock.Lock()
	c, ok = r.items.Get(id)
	if !ok || !c.IsTrashed() {
		r.lock.Unlock()
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity", "entity_id", id))
	}
	next := *c
	next.ClearTrashState()
	if !areaExists {
		next.AreaID = nil
	}
	r.items.Set(id, &next)
	r.lock.Unlock()

	if areaExists {
		_ = r.areaRegistry.AddCommodity(ctx, areaID, id)
	}
	out := next
	return &out, nil
}

// PurgeTrashed deletes a trashed item's row, and the links other items have
// to it.
func (r *CommodityRegistry) PurgeTrashed(_ context.Context, id string) (bool, error) {
	if !r.purgeTrashedRow(id) {
		return false, nil
	}
	if r.customFields != nil {
		r.customFields.dropLinksTo(id)
	}
	return true, nil
}

func (r *CommodityRegistry) purgeTrashedRow(id string) bool {
	r.lock.Lock()
	defer r.lock.Unlock()
	c, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(c) || !c.IsTrashed() {
		return false
	}
	r.items.Delete(id)
	return true
}

// rewriteTrashedTags applies fn to the tags of every visible trashed item. Tag
// renames and deletes cover trashed items too, so a restored item carries no
// stale slug.
func (r *CommodityRegistry) rewriteTrashedTags(fn func(models.ValuerSlice[string]) (bool, models.ValuerSlice[string])) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		c := pair.Value
		if c == nil || !c.IsTrashed() || !r.isItemVisible(c) {
			continue
		}
		if changed, tags := fn(c.Tags); changed {
			next := *c
			next.Tags = tags
			r.items.Set(c.ID, &next)
		}
	}
}

// GetTrashed returns a trashed file with its links.
func (r *FileRegistry) GetTrashed(_ context.Context, id string) (*models.FileEntity, error) {
	r.lock.RLock()
	f, ok := r.items.Get(id)
	visible := ok && r.isItemVisible(f) && f.IsTrashed()
	var cp models.FileEntity
	if visible {
		cp = *f
	}
	r.lock.RUnlock()
	if !visible {
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
	}
	return r.attachLinks([]*models.FileEntity{&cp})[0], nil
}

// trashedFiles returns copies of the visible trashed files that pass keep,
// with their links.
func (r *FileRegistry) trashedFiles(keep func(*models.FileEntity, []models.FileLink) bool) []*models.FileEntity {
	r.lock.RLock()
	var out []*models.FileEntity
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		f := pair.Value
		if f == nil || !f.IsTrashed() || !r.isItemVisible(f) || !keep(f, r.links.byFile[f.ID]) {
			continue
		}
		cp := *f
		out = append(out, &cp)
	}
	r.lock.RUnlock()
	return r.attachLinks(out)
}

// ListTrashed returns a page of the files deleted on their own, most recently
// deleted first.
func (r *FileRegistry) ListTrashed(_ context.Context, offset, limit int) ([]*models.FileEntity, int, error) {
	all := r.trashedFiles(func(f *models.FileEntity, _ []models.FileLink) bool {
		return f.DeletedWithCommodityID == nil
	})
	slices.SortStableFunc(all, func(a, b *models.FileEntity) int {
		if c := b.DeletedAt.Compare(*a.DeletedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	return page(all, offset, limit), len(all), nil
}

// ListTrashedLinkedTo returns the trashed files linked to the entity, newest
// first.
func (r *FileRegistry) ListTrashedLinkedTo(_ context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	out := r.trashedFiles(func(_ *models.FileEntity, links []models.FileLink) bool {
		for _, l := range links {
			if l.EntityType == entityType && l.EntityID == entityID {
				return true
			}
		}
		return false
	})
	sortFilesNewestFirst(out)
	return out, nil
}

// CountTrashedWith counts the files that went to Trash with each item.
func (r *FileRegistry) CountTrashedWith(_ context.Context, commodityIDs []string) (map[string]registry.TrashedFilesStats, error) {
	out := make(map[string]registry.TrashedFilesStats, len(commodityIDs))
	if len(commodityIDs) == 0 {
		return out, nil
	}
	want := make(map[string]bool, len(commodityIDs))
	for _, id := range commodityIDs {
		want[id] = true
	}
	for _, f := range r.trashedFiles(func(f *models.FileEntity, _ []models.FileLink) bool {
		return f.DeletedWithCommodityID != nil && want[*f.DeletedWithCommodityID]
	}) {
		stats := out[*f.DeletedWithCommodityID]
		stats.Count++
		stats.Bytes += f.SizeBytes
		out[*f.DeletedWithCommodityID] = stats
	}
	return out, nil
}

// ListTrashedBefore returns up to limit files deleted on their own before
// cutoff, oldest first.
func (r *FileRegistry) ListTrashedBefore(_ context.Context, cutoff time.Time, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	candidates := r.trashedFiles(func(f *models.FileEntity, _ []models.FileLink) bool {
		return f.DeletedWithCommodityID == nil && f.DeletedAt.Before(cutoff)
	})
	sortFilesOldestDeletedFirst(candidates)
	return page(candidates, 0, limit), nil
}

// ListStrandedTrashed returns up to limit trashed files whose item is no
// longer in Trash, oldest first.
func (r *FileRegistry) ListStrandedTrashed(_ context.Context, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	candidates := r.trashedFiles(func(f *models.FileEntity, _ []models.FileLink) bool {
		return f.DeletedWithCommodityID != nil && !r.itemInTrash(*f.DeletedWithCommodityID)
	})
	sortFilesOldestDeletedFirst(candidates)
	return page(candidates, 0, limit), nil
}

func sortFilesOldestDeletedFirst(files []*models.FileEntity) {
	slices.SortStableFunc(files, func(a, b *models.FileEntity) int {
		if c := a.DeletedAt.Compare(*b.DeletedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
}

// isLiveFile reports whether a file is out of Trash.
func isLiveFile(f *models.FileEntity) bool { return !f.IsTrashed() }

// setTrashState changes the trash columns of one file under the write lock.
// The change applies only when inState holds for the stored file.
func (r *FileRegistry) setTrashState(id string, inState func(*models.FileEntity) bool, change func(*models.FileEntity)) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	f, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(f) || !inState(f) {
		return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
	}
	next := *f
	change(&next)
	r.items.Set(id, &next)
	return nil
}

// Trash moves a live file to Trash.
func (r *FileRegistry) Trash(_ context.Context, id, deletedBy, withCommodityID string, at time.Time) error {
	return r.setTrashState(id, isLiveFile, func(f *models.FileEntity) {
		f.DeletedAt = new(at.UTC())
		f.DeletedBy = new(deletedBy)
		f.DeletedWithCommodityID = nil
		if withCommodityID != "" {
			f.DeletedWithCommodityID = new(withCommodityID)
		}
	})
}

// Restore makes a trashed file live again.
func (r *FileRegistry) Restore(_ context.Context, id string) error {
	return r.setTrashState(id, (*models.FileEntity).IsTrashed, func(f *models.FileEntity) {
		f.ClearTrashState()
	})
}

// SetTrashedWith changes the item a trashed file went to Trash with.
func (r *FileRegistry) SetTrashedWith(_ context.Context, id, commodityID string) error {
	return r.setTrashState(id, (*models.FileEntity).IsTrashed, func(f *models.FileEntity) {
		f.DeletedWithCommodityID = nil
		if commodityID != "" {
			f.DeletedWithCommodityID = new(commodityID)
		}
	})
}

// PurgeTrashed deletes a trashed file's row and links.
func (r *FileRegistry) PurgeTrashed(_ context.Context, id string) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	f, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(f) || !f.IsTrashed() {
		return false, nil
	}
	r.items.Delete(id)
	delete(r.links.byFile, id)
	return true, nil
}

// rewriteTrashedTags applies fn to the tags of every visible trashed file.
func (r *FileRegistry) rewriteTrashedTags(fn func(models.StringSlice) (bool, models.StringSlice)) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		f := pair.Value
		if f == nil || !f.IsTrashed() || !r.isItemVisible(f) {
			continue
		}
		if changed, tags := fn(f.Tags); changed {
			next := *f
			next.Tags = tags
			r.items.Set(f.ID, &next)
		}
	}
}

// page returns items[offset:offset+limit], clamped.
func page[T any](items []T, offset, limit int) []T {
	offset = max(offset, 0)
	limit = max(limit, 0)
	start := min(offset, len(items))
	end := min(start+limit, len(items))
	return items[start:end]
}
