package memory

import (
	"context"
	"slices"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// Patch writes the bulk-editable columns of a live item. It holds the lock
// Update holds across its read and write, so the two run one after the
// other, and it copies the columns onto the item as stored at the moment of
// the write, so a writer that does not take that lock, such as SetDraft,
// keeps its change too.
func (r *CommodityRegistry) Patch(ctx context.Context, id string, addTags []string, change func(models.Commodity) (models.Commodity, bool, error)) (before, after *models.Commodity, err error) {
	r.assetNumbers.Lock()
	defer r.assetNumbers.Unlock()

	locked, err := r.Registry.Get(ctx, id)
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to get commodity", err, errx.Attrs("commodity_id", id))
	}
	next, changed, err := change(*copyOut(locked))
	if err != nil {
		return nil, nil, err
	}
	if !changed {
		return locked, nil, nil
	}
	patched, err := registry.PatchedCommodity(*locked, next, addTags)
	if err != nil {
		return nil, nil, err
	}
	if patched.Type != locked.Type {
		// Read-locked until the write, as in Update, so a type delete
		// cannot move the item in between.
		unlockTypes, known := r.lockItemTypesForUpdate(ctx, patched)
		defer unlockTypes()
		if !known {
			return nil, nil, errxtrace.Classify(registry.ErrUnknownItemType, errx.Attrs("type", string(patched.Type)))
		}
	}
	if area := commodityAreaID(&patched); area != "" && area != commodityAreaID(locked) {
		if _, err := r.areaRegistry.Get(ctx, area); err != nil {
			return nil, nil, errxtrace.Wrap("failed to get area", err, errx.Attrs("area_id", area))
		}
	}

	stored, oldArea, err := r.writePatch(id, patched)
	if err != nil {
		return nil, nil, err
	}
	if newArea := commodityAreaID(stored); oldArea != newArea {
		if oldArea != "" {
			_ = r.areaRegistry.DeleteCommodity(ctx, oldArea, id)
		}
		if newArea != "" {
			_ = r.areaRegistry.AddCommodity(ctx, newArea, id)
		}
	}
	return locked, stored, nil
}

// writePatch copies the patched columns onto the item as stored now and
// returns it with the area it had before.
func (r *CommodityRegistry) writePatch(id string, patched models.Commodity) (*models.Commodity, string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	c, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(c) || c.IsTrashed() {
		return nil, "", errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "commodity", "entity_id", id))
	}
	// The tags were checked against the item as Patch read it; here they
	// are only copied, so every one of them counts as named.
	next, err := registry.PatchedCommodity(*copyOut(c), patched, patched.Tags)
	if err != nil {
		return nil, "", err
	}
	next.Revision = c.Revision + 1
	next.LastModifiedDate = new(models.NewDate(time.Now()))
	r.items.Set(id, &next)
	return copyOut(&next), commodityAreaID(c), nil
}

// commodityAreaID is the area of c, or "" when it has none.
func commodityAreaID(c *models.Commodity) string {
	if c.AreaID == nil {
		return ""
	}
	return *c.AreaID
}

// PatchTags writes the tags of a live file. The read, change and write run
// under the registry's write lock.
func (r *FileRegistry) PatchTags(_ context.Context, id string, addTags []string, change func(models.FileEntity) (models.FileEntity, bool, error)) (before, after *models.FileEntity, err error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	f, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(f) || f.IsTrashed() {
		return nil, nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
	}
	locked := r.withLinksLocked(f)
	next, changed, err := change(*r.withLinksLocked(f))
	if err != nil {
		return nil, nil, err
	}
	if !changed {
		return locked, nil, nil
	}
	patched, err := registry.PatchedFileTags(*f, next, addTags)
	if err != nil {
		return nil, nil, err
	}
	patched.UpdatedAt = time.Now()
	r.items.Set(id, &patched)
	return locked, r.withLinksLocked(&patched), nil
}

// withLinksLocked returns a copy of f with its links. The caller holds the
// registry lock.
func (r *FileRegistry) withLinksLocked(f *models.FileEntity) *models.FileEntity {
	out := *f
	out.Tags = slices.Clone(f.Tags)
	out.Links = slices.Clone(r.links.byFile[f.ID])
	if out.Links == nil {
		out.Links = []models.FileLink{}
	}
	return &out
}
