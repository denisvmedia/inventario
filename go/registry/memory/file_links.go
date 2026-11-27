package memory

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/google/uuid"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// fileLinkStore holds the file links of the memory backend, keyed by file id.
// It is guarded by the file registry's lock, so a link check and a file delete
// can happen under one critical section the way the postgres registry does
// them under one row lock.
type fileLinkStore struct {
	byFile map[string][]models.FileLink
}

func newFileLinkStore() *fileLinkStore {
	return &fileLinkStore{byFile: make(map[string][]models.FileLink)}
}

func sortLinks(links []models.FileLink) {
	slices.SortStableFunc(links, func(a, b models.FileLink) int {
		if c := a.CreatedAt.Compare(b.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(a.ID, b.ID)
	})
}

// attachLinks fills the Links of copies handed to a caller. The stored slices
// are never modified in place, so a clone is enough to keep callers from
// sharing them.
func (r *FileRegistry) attachLinks(files []*models.FileEntity) []*models.FileEntity {
	r.lock.RLock()
	defer r.lock.RUnlock()
	for _, f := range files {
		if f == nil {
			continue
		}
		f.Links = slices.Clone(r.links.byFile[f.ID])
		if f.Links == nil {
			f.Links = []models.FileLink{}
		}
	}
	return files
}

// newLinkLocked builds the stored form of a link of file. Caller holds the
// write lock.
func (r *FileRegistry) newLinkLocked(file *models.FileEntity, link models.FileLink) models.FileLink {
	createdBy := r.userID
	if createdBy == "" {
		createdBy = file.CreatedByUserID
	}
	if link.CreatedAt.IsZero() {
		link.CreatedAt = time.Now()
	}
	link.ID = uuid.NewString()
	link.UUID = uuid.NewString()
	link.TenantID = file.TenantID
	link.GroupID = file.GroupID
	link.CreatedByUserID = createdBy
	link.FileID = file.ID
	link.EntityName = ""
	return link
}

// addLinkLocked stores link unless the file already links to that entity.
// Caller holds the write lock.
func (r *FileRegistry) addLinkLocked(file *models.FileEntity, link models.FileLink) (models.FileLink, bool) {
	current := r.links.byFile[file.ID]
	for _, l := range current {
		if l.EntityType == link.EntityType && l.EntityID == link.EntityID {
			return l, false
		}
	}
	stored := r.newLinkLocked(file, link)
	next := make([]models.FileLink, 0, len(current)+1)
	next = append(next, current...)
	next = append(next, stored)
	sortLinks(next)
	r.links.byFile[file.ID] = next
	return stored, true
}

// Get returns the file with its links.
func (r *FileRegistry) Get(ctx context.Context, id string) (*models.FileEntity, error) {
	file, err := r.Registry.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.attachLinks([]*models.FileEntity{file})[0], nil
}

// List returns the visible files with their links.
func (r *FileRegistry) List(ctx context.Context) ([]*models.FileEntity, error) {
	files, err := r.Registry.List(ctx)
	if err != nil {
		return nil, err
	}
	return r.attachLinks(files), nil
}

// Create stores the file and the links it carries.
func (r *FileRegistry) Create(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	links := file.Links
	file.Links = nil
	file.ClearTrashState()
	created, err := r.Registry.Create(ctx, file)
	if err != nil {
		return nil, err
	}

	r.lock.Lock()
	for _, l := range links {
		r.addLinkLocked(created, l)
	}
	r.lock.Unlock()

	out := *created
	return r.attachLinks([]*models.FileEntity{&out})[0], nil
}

// Update writes the file's own fields. Links and the legacy link fields are
// kept as stored, and so is the checksum when file carries none and the
// thumbnail orientation mark.
func (r *FileRegistry) Update(ctx context.Context, file models.FileEntity) (*models.FileEntity, error) {
	r.lock.RLock()
	existing, ok := r.items.Get(file.ID)
	if ok {
		file.LegacyFileLink = existing.LegacyFileLink
		if file.File != nil && file.SHA256 == "" && existing.File != nil {
			f := *file.File
			f.SHA256 = existing.SHA256
			file.File = &f
		}
		if file.File != nil && existing.File != nil && existing.ThumbnailsOriented {
			f := *file.File
			f.ThumbnailsOriented = true
			file.File = &f
		}
	}
	r.lock.RUnlock()
	file.Links = nil
	// The trash fields belong to Trash and Restore. The base update refuses a
	// trashed file, as the postgres registry does.
	file.ClearTrashState()

	updated, err := r.Registry.Update(ctx, file)
	if err != nil {
		return nil, err
	}
	out := *updated
	return r.attachLinks([]*models.FileEntity{&out})[0], nil
}

// Delete removes the file row and its links.
func (r *FileRegistry) Delete(ctx context.Context, id string) error {
	if err := r.Registry.Delete(ctx, id); err != nil {
		return err
	}
	r.lock.Lock()
	delete(r.links.byFile, id)
	r.lock.Unlock()
	return nil
}

// AddLink mirrors the postgres method.
func (r *FileRegistry) AddLink(_ context.Context, link models.FileLink) (*models.FileLink, bool, error) {
	return r.addLink(link, isLiveFile)
}

// AddLinkIncludingTrashed mirrors the postgres method.
func (r *FileRegistry) AddLinkIncludingTrashed(_ context.Context, link models.FileLink) (*models.FileLink, bool, error) {
	return r.addLink(link, func(*models.FileEntity) bool { return true })
}

// addLink writes the link when inState holds for the file. The state is read
// under the write lock that Trash takes, as the postgres method reads it under
// the row lock.
func (r *FileRegistry) addLink(link models.FileLink, inState func(*models.FileEntity) bool) (*models.FileLink, bool, error) {
	if link.FileID == "" {
		return nil, false, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "FileID"))
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	file, ok := r.items.Get(link.FileID)
	if !ok || !r.isItemVisible(file) || !inState(file) {
		return nil, false, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", link.FileID))
	}
	stored, created := r.addLinkLocked(file, link)
	return &stored, created, nil
}

// RemoveLink mirrors the postgres method.
func (r *FileRegistry) RemoveLink(_ context.Context, fileID, linkID string) (*models.FileLink, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	file, ok := r.items.Get(fileID)
	if ok && r.isItemVisible(file) {
		current := r.links.byFile[fileID]
		for i, l := range current {
			if l.ID != linkID {
				continue
			}
			next := make([]models.FileLink, 0, len(current)-1)
			next = append(next, current[:i]...)
			next = append(next, current[i+1:]...)
			r.links.byFile[fileID] = next
			return &l, nil
		}
	}
	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file_link", "entity_id", linkID))
}

// RemoveLinksToEntity mirrors the postgres method.
func (r *FileRegistry) RemoveLinksToEntity(_ context.Context, entityType, entityID string) ([]string, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var fileIDs []string
	for fileID, current := range r.links.byFile {
		file, ok := r.items.Get(fileID)
		if ok && !r.isItemVisible(file) {
			continue
		}
		next := make([]models.FileLink, 0, len(current))
		for _, l := range current {
			if l.EntityType == entityType && l.EntityID == entityID {
				continue
			}
			next = append(next, l)
		}
		if len(next) != len(current) {
			r.links.byFile[fileID] = next
			fileIDs = append(fileIDs, fileID)
		}
	}
	slices.Sort(fileIDs)
	return fileIDs, nil
}

// moveLinks points every link to (fromType, fromID) at (toType, toID)
// instead, for the files this registry can see. It mirrors the UPDATE of
// file_links the postgres draft promotion runs.
func (r *FileRegistry) moveLinks(fromType, fromID, toType, toID string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for fileID, current := range r.links.byFile {
		file, ok := r.items.Get(fileID)
		if ok && !r.isItemVisible(file) {
			continue
		}
		var next []models.FileLink
		for i, l := range current {
			if l.EntityType != fromType || l.EntityID != fromID {
				continue
			}
			if next == nil {
				next = slices.Clone(current)
			}
			next[i].EntityType = toType
			next[i].EntityID = toID
		}
		if next != nil {
			r.links.byFile[fileID] = next
		}
	}
}

// DeleteIfUnlinked mirrors the postgres method.
func (r *FileRegistry) DeleteIfUnlinked(_ context.Context, id, ignoreEntityType, ignoreEntityID string) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	file, ok := r.items.Get(id)
	if !ok || !r.isItemVisible(file) {
		return false, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "file", "entity_id", id))
	}
	for _, l := range r.links.byFile[id] {
		if l.EntityType != ignoreEntityType || l.EntityID != ignoreEntityID {
			return false, nil
		}
	}
	r.items.Delete(id)
	delete(r.links.byFile, id)
	return true, nil
}

// ListByChecksum mirrors the postgres method.
func (r *FileRegistry) ListByChecksum(ctx context.Context, sha256 string) ([]*models.FileEntity, error) {
	if sha256 == "" {
		return nil, nil
	}
	all, err := r.List(ctx)
	if err != nil {
		return nil, err
	}
	var out []*models.FileEntity
	for _, f := range all {
		if f.File != nil && f.SHA256 == sha256 {
			out = append(out, f)
		}
	}
	slices.SortStableFunc(out, func(a, b *models.FileEntity) int {
		if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
			return c
		}
		return strings.Compare(b.ID, a.ID)
	})
	return out, nil
}

// ListPendingChecksum mirrors the postgres method.
func (r *FileRegistry) ListPendingChecksum(_ context.Context, afterID string, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	r.lock.RLock()
	var pending []*models.FileEntity
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		f := pair.Value
		if f == nil || f.File == nil || f.SHA256 != "" || f.OriginalPath == "" || f.ID <= afterID {
			continue
		}
		v := *f
		pending = append(pending, &v)
	}
	r.lock.RUnlock()
	slices.SortFunc(pending, func(a, b *models.FileEntity) int { return strings.Compare(a.ID, b.ID) })
	if len(pending) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}

// SetChecksum mirrors the postgres method.
func (r *FileRegistry) SetChecksum(_ context.Context, id, sha256 string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	f, ok := r.items.Get(id)
	if !ok || f.File == nil || f.SHA256 != "" {
		return nil
	}
	nf := *f.File
	nf.SHA256 = sha256
	next := *f
	next.File = &nf
	r.items.Set(id, &next)
	return nil
}

// ListPendingThumbnailOrientation mirrors the postgres method.
func (r *FileRegistry) ListPendingThumbnailOrientation(_ context.Context, afterID string, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	r.lock.RLock()
	var pending []*models.FileEntity
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		f := pair.Value
		if f == nil || f.File == nil || f.ThumbnailsOriented || f.MIMEType != "image/jpeg" || f.OriginalPath == "" || f.ID <= afterID {
			continue
		}
		v := *f
		pending = append(pending, &v)
	}
	r.lock.RUnlock()
	slices.SortFunc(pending, func(a, b *models.FileEntity) int { return strings.Compare(a.ID, b.ID) })
	if len(pending) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}

// SetThumbnailsOriented mirrors the postgres method.
func (r *FileRegistry) SetThumbnailsOriented(_ context.Context, id string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	f, ok := r.items.Get(id)
	if !ok || f.File == nil || f.ThumbnailsOriented {
		return nil
	}
	nf := *f.File
	nf.ThumbnailsOriented = true
	next := *f
	next.File = &nf
	r.items.Set(id, &next)
	return nil
}

// MoveLegacyLinks mirrors the postgres method.
func (r *FileRegistry) MoveLegacyLinks(_ context.Context, limit int) (int, error) {
	if limit <= 0 {
		return 0, nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()
	moved := 0
	for pair := r.items.Oldest(); pair != nil && moved < limit; pair = pair.Next() {
		f := pair.Value
		if f == nil || f.LegacyLinkedEntityType == "" {
			continue
		}
		if f.LegacyLinkedEntityID != "" {
			r.addLinkLocked(f, models.FileLink{
				EntityType: f.LegacyLinkedEntityType,
				EntityID:   f.LegacyLinkedEntityID,
				Meta:       f.LegacyLinkedEntityMeta,
				CreatedAt:  f.CreatedAt,
			})
		}
		next := *f
		next.LegacyFileLink = models.LegacyFileLink{}
		r.items.Set(f.ID, &next)
		moved++
	}
	return moved, nil
}
