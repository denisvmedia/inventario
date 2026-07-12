package memory

import (
	"context"
	"errors"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// FileRegistryFactory creates FileRegistry instances with proper context
type FileRegistryFactory struct {
	baseFileRegistry *Registry[models.FileEntity, *models.FileEntity]

	// Sibling factories used ONLY by ListOrphanCandidates (#2237) to answer
	// "does the entity this file is linked to still exist?". Postgres asks
	// that with a SQL anti-join; the memory backend has no planner, so it
	// has to hold the other registries. Wired by NewFactorySet via
	// SetLinkedEntityFactories; nil for a bare NewFileRegistryFactory()
	// (unit tests that only exercise the file table), in which case
	// ListOrphanCandidates FAILS CLOSED with an error rather than
	// reporting "the entity is gone" on no evidence.
	commodityFactory *CommodityRegistryFactory
	areaFactory      *AreaRegistryFactory
	locationFactory  *LocationRegistryFactory
}

// FileRegistry is a context-aware registry that can only be created through the factory
type FileRegistry struct {
	*Registry[models.FileEntity, *models.FileEntity]

	userID string

	// Carried from the factory; see FileRegistryFactory. Only read by
	// ListOrphanCandidates.
	commodityFactory *CommodityRegistryFactory
	areaFactory      *AreaRegistryFactory
	locationFactory  *LocationRegistryFactory
}

var _ registry.FileRegistry = (*FileRegistry)(nil)
var _ registry.FileRegistryFactory = (*FileRegistryFactory)(nil)

func NewFileRegistryFactory() *FileRegistryFactory {
	return &FileRegistryFactory{
		baseFileRegistry: NewRegistry[models.FileEntity, *models.FileEntity](),
	}
}

// SetLinkedEntityFactories wires the sibling registries the orphan-candidate
// scan (#2237) probes for existence. Called once from NewFactorySet, after
// all three factories exist. Leaving them unset is safe: ListOrphanCandidates
// refuses to run rather than mis-report a live entity as missing.
func (f *FileRegistryFactory) SetLinkedEntityFactories(
	commodity *CommodityRegistryFactory,
	area *AreaRegistryFactory,
	location *LocationRegistryFactory,
) {
	f.commodityFactory = commodity
	f.areaFactory = area
	f.locationFactory = location
}

// Factory methods implementing registry.FileRegistryFactory

func (f *FileRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.FileRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *FileRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.FileRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	// Create a new registry with user context already set
	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := &Registry[models.FileEntity, *models.FileEntity]{
		items:   f.baseFileRegistry.items, // Share the data map
		lock:    f.baseFileRegistry.lock,  // Share the mutex pointer
		userID:  user.ID,                  // Set user-specific userID
		groupID: groupID,                  // Set group-specific groupID
	}

	return &FileRegistry{
		Registry:         userRegistry,
		userID:           user.ID,
		commodityFactory: f.commodityFactory,
		areaFactory:      f.areaFactory,
		locationFactory:  f.locationFactory,
	}, nil
}

func (f *FileRegistryFactory) CreateServiceRegistry() registry.FileRegistry {
	// Create a new registry with service account context (no user filtering)
	serviceRegistry := &Registry[models.FileEntity, *models.FileEntity]{
		items:  f.baseFileRegistry.items, // Share the data map
		lock:   f.baseFileRegistry.lock,  // Share the mutex pointer
		userID: "",                       // Clear userID to bypass user filtering
	}

	return &FileRegistry{
		Registry:         serviceRegistry,
		userID:           "", // Clear userID to bypass user filtering
		commodityFactory: f.commodityFactory,
		areaFactory:      f.areaFactory,
		locationFactory:  f.locationFactory,
	}
}

func (r *FileRegistry) ListByType(ctx context.Context, fileType models.FileType) ([]*models.FileEntity, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.FileEntity
	for _, file := range allFiles {
		if file.Type == fileType {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

// fileMatchesEnumFilters returns true when the file passes every
// non-nil enum-style filter (type, category, linked-entity pair).
// linkedEntityType + linkedEntityID must both be non-nil to apply;
// either alone is treated as "no filter" to mirror the postgres path.
func fileMatchesEnumFilters(file *models.FileEntity, fileType *models.FileType, fileCategory *models.FileCategory, linkedEntityType, linkedEntityID *string) bool {
	if fileType != nil && file.Type != *fileType {
		return false
	}
	if fileCategory != nil && file.Category != *fileCategory {
		return false
	}
	if linkedEntityType != nil && linkedEntityID != nil {
		if file.LinkedEntityType != *linkedEntityType || file.LinkedEntityID != *linkedEntityID {
			return false
		}
	}
	return true
}

//nolint:gocognit // TODO: refactor
func (r *FileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, fileCategory *models.FileCategory, tags []string, linkedEntityType, linkedEntityID *string) ([]*models.FileEntity, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.FileEntity

	for _, file := range allFiles {
		// Filter by type / category / linked-entity if specified. Each
		// helper returns true when the file passes; collapses what would
		// otherwise be three early-return guards into a single check and
		// keeps the surrounding cyclomatic complexity manageable.
		if !fileMatchesEnumFilters(file, fileType, fileCategory, linkedEntityType, linkedEntityID) {
			continue
		}

		// Filter by tags if specified
		if len(tags) > 0 {
			hasAllTags := true
			for _, requiredTag := range tags {
				found := false
				for _, fileTag := range file.Tags {
					if strings.EqualFold(fileTag, requiredTag) {
						found = true
						break
					}
				}
				if !found {
					hasAllTags = false
					break
				}
			}
			if !hasAllTags {
				continue
			}
		}

		// Search in title and description
		if query != "" {
			titleMatch := strings.Contains(strings.ToLower(file.Title), query)
			descMatch := strings.Contains(strings.ToLower(file.Description), query)
			pathMatch := strings.Contains(strings.ToLower(file.Path), query)
			originalPathMatch := strings.Contains(strings.ToLower(file.OriginalPath), query)

			if !titleMatch && !descMatch && !pathMatch && !originalPathMatch {
				continue
			}
		}

		filtered = append(filtered, file)
	}

	return filtered, nil
}

func (r *FileRegistry) ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType, fileCategory *models.FileCategory, linkedEntityType, linkedEntityID *string) ([]*models.FileEntity, int, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, 0, err
	}

	if fileType != nil || fileCategory != nil || (linkedEntityType != nil && linkedEntityID != nil) {
		filtered := allFiles[:0:0]
		for _, file := range allFiles {
			if !fileMatchesEnumFilters(file, fileType, fileCategory, linkedEntityType, linkedEntityID) {
				continue
			}
			filtered = append(filtered, file)
		}
		allFiles = filtered
	}

	// Match the postgres ListPaginated sort: newest first by
	// created_at. The base in-memory Registry.List returns insertion
	// order, which used to be fine when the seed produced ≤1 file
	// in dev mode — but post-#1658 the seed grows ~50 files, and
	// the e2e Files-page assertions depend on freshly-uploaded
	// rows landing at the top of page 1 like they do against
	// postgres.
	sort.SliceStable(allFiles, func(i, j int) bool {
		return allFiles[i].CreatedAt.After(allFiles[j].CreatedAt)
	})

	total := len(allFiles)

	// Apply pagination
	start := min(offset, total)

	end := min(start+limit, total)

	paginatedFiles := allFiles[start:end]
	return paginatedFiles, total, nil
}

// CountByCategory aggregates files matching the same filters as Search,
// grouped by Category. Always returns all four buckets (zero-filled),
// keeping the response shape stable for the FE tile renderer. The second
// returned map carries per-category byte totals (sum of size_bytes); the
// FE uses it to render the cumulative "{N} files · {Y} total" footer.
func (r *FileRegistry) CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string) (map[models.FileCategory]int, map[models.FileCategory]int64, error) {
	files, err := r.Search(ctx, query, fileType, nil, tags, nil, nil)
	if err != nil {
		return nil, nil, err
	}

	counts := map[models.FileCategory]int{
		models.FileCategoryImages:    0,
		models.FileCategoryDocuments: 0,
		models.FileCategoryOther:     0,
	}
	bytes := map[models.FileCategory]int64{
		models.FileCategoryImages:    0,
		models.FileCategoryDocuments: 0,
		models.FileCategoryOther:     0,
	}
	for _, file := range files {
		counts[file.Category]++
		bytes[file.Category] += file.SizeBytes
	}
	return counts, bytes, nil
}

// ListPendingSizeBackfill mirrors the postgres method — returns up to
// limit files where size_bytes == 0. The memory backend doesn't have a
// SQL planner so we iterate; production hits the postgres path. Runs
// in service mode (the backfill is RLS-blind).
func (r *FileRegistry) ListPendingSizeBackfill(_ context.Context, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	var pending []*models.FileEntity
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		file := pair.Value
		if file == nil || file.File == nil {
			continue
		}
		if file.SizeBytes != 0 {
			continue
		}
		pending = append(pending, file)
		if len(pending) >= limit {
			break
		}
	}
	return pending, nil
}

// orphanCandidateLinkTypes is the positive ALLOWLIST of linked_entity_type
// values the orphan-file GC (#2237) understands. It mirrors the IN (...) list
// in the postgres ListOrphanCandidates query verbatim, and it is an allowlist
// on purpose:
//
//   - ""       → a STANDALONE file. First-class since #2235 — it has no link,
//     so it can never have a DANGLING one. Excluded structurally.
//   - "export" → the backup subsystem's own artifact, with its own lifecycle.
//     Excluded structurally, so the `deleted_at IS NULL` filter on the exports
//     registry can never be misread as "the export is gone".
//   - anything else (a future link type; the registries do NOT enforce
//     models.FileEntity.ValidateWithContext, so the DB is a superset of the
//     validator's enumeration) → excluded, fails closed.
var orphanCandidateLinkTypes = map[string]bool{
	"commodity": true,
	"area":      true,
	"location":  true,
}

// ListOrphanCandidates mirrors the postgres method — file rows in the
// linked-entity allowlist whose target no longer exists and whose BOTH
// timestamps are older than olderThan (#2237). The memory backend has no SQL
// planner, so the anti-join is three Get probes against the sibling service
// registries.
//
// Runs in service mode (the GC is RLS-blind by necessity: a file may be
// legitimately linked to an entity in another group, and a group-scoped probe
// would read that LIVE entity as missing). Fails closed when the sibling
// factories were never wired.
func (r *FileRegistry) ListOrphanCandidates(ctx context.Context, olderThan time.Time, after registry.OrphanCandidateCursor, limit int) ([]*models.FileEntity, error) {
	if limit <= 0 {
		return nil, nil
	}
	if r.commodityFactory == nil || r.areaFactory == nil || r.locationFactory == nil {
		// No evidence available ⇒ no deletions. Never guess.
		return nil, errxtrace.Classify(registry.ErrFieldRequired,
			errx.Attrs("field_name", "linkedEntityFactories"))
	}

	// Snapshot the shape-eligible rows under the read lock, then probe the
	// sibling registries with the lock released (they own their own mutexes;
	// this just keeps the critical section short and honest).
	var shortlist []*models.FileEntity
	r.lock.RLock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		file := pair.Value
		if file == nil {
			continue
		}
		if !orphanCandidateLinkTypes[file.LinkedEntityType] {
			continue // allowlist: standalone (#2235), export, and unknown types are never candidates
		}
		if file.LinkedEntityID == "" {
			continue // malformed link, not garbage — the worker reports it, never deletes it
		}
		if !file.CreatedAt.Before(olderThan) || !file.UpdatedAt.Before(olderThan) {
			continue // BOTH timestamps must clear the age gate
		}
		if !afterOrphanCursor(file, after) {
			continue // already covered by an earlier page of this scan
		}
		v := *file
		shortlist = append(shortlist, &v)
	}
	r.lock.RUnlock()

	// (created_at, id) ascending — the same total order the postgres keyset
	// query produces, so a cursor handed back by one backend means the same
	// thing in the other.
	sort.SliceStable(shortlist, func(i, j int) bool {
		return lessOrphanKeyset(shortlist[i], shortlist[j])
	})

	comReg := r.commodityFactory.CreateServiceRegistry()
	areaReg := r.areaFactory.CreateServiceRegistry()
	locReg := r.locationFactory.CreateServiceRegistry()

	var orphans []*models.FileEntity
	for _, file := range shortlist {
		var err error
		switch file.LinkedEntityType {
		case "commodity":
			_, err = comReg.Get(ctx, file.LinkedEntityID)
		case "area":
			_, err = areaReg.Get(ctx, file.LinkedEntityID)
		case "location":
			_, err = locReg.Get(ctx, file.LinkedEntityID)
		default:
			continue // unreachable: the allowlist above already filtered
		}
		switch {
		case err == nil:
			continue // the entity is alive — not an orphan
		case errors.Is(err, registry.ErrNotFound):
			orphans = append(orphans, file)
		default:
			// A transient probe failure must never read as "gone".
			return nil, errxtrace.Wrap("failed to probe linked entity existence", err)
		}
		if len(orphans) >= limit {
			break
		}
	}
	return orphans, nil
}

// lessOrphanKeyset orders two candidates by the (created_at, id) keyset the
// paged scan resumes on.
func lessOrphanKeyset(a, b *models.FileEntity) bool {
	if a.CreatedAt.Equal(b.CreatedAt) {
		return a.ID < b.ID
	}
	return a.CreatedAt.Before(b.CreatedAt)
}

// afterOrphanCursor reports whether file sorts strictly after the cursor in the
// (created_at, id) keyset order. A zero cursor accepts every row.
func afterOrphanCursor(file *models.FileEntity, after registry.OrphanCandidateCursor) bool {
	if after.IsZero() {
		return true
	}
	if file.CreatedAt.Equal(after.CreatedAt) {
		return file.ID > after.ID
	}
	return file.CreatedAt.After(after.CreatedAt)
}

// CountByOriginalPath mirrors the postgres method — how many file rows, across
// EVERY tenant and group, reference the given blob key (#2237). Service-mode
// only: it reads the shared item map directly rather than through the
// visibility filter, because the whole point is to see rows the caller cannot.
func (r *FileRegistry) CountByOriginalPath(_ context.Context, originalPath string) (int, error) {
	if originalPath == "" {
		return 0, nil
	}
	count := 0
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		file := pair.Value
		if file == nil || file.File == nil {
			continue
		}
		if file.OriginalPath == originalPath {
			count++
		}
	}
	return count, nil
}

// ListIDsByTenant mirrors the postgres method — the ids of every file row
// owned by tenantID, across all its groups (#2237). Service-mode only.
func (r *FileRegistry) ListIDsByTenant(_ context.Context, tenantID string) ([]string, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}
	var ids []string
	r.lock.RLock()
	defer r.lock.RUnlock()
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		file := pair.Value
		if file == nil || file.TenantID != tenantID {
			continue
		}
		ids = append(ids, file.ID)
	}
	return ids, nil
}

// SumSizeBreakdown mirrors the postgres aggregator: per-category byte
// totals with export bundles split out of "other". The shared file map
// is already group-scoped via the user registry's groupID, so this
// iterates the group's files only.
func (r *FileRegistry) SumSizeBreakdown(ctx context.Context) (registry.StorageBreakdown, error) {
	files, err := r.List(ctx)
	if err != nil {
		return registry.StorageBreakdown{}, err
	}

	var breakdown registry.StorageBreakdown
	for _, file := range files {
		if file.File == nil {
			continue
		}
		size := file.SizeBytes
		if file.LinkedEntityType == "export" {
			breakdown.Exports += size
			continue
		}
		switch file.Category {
		case models.FileCategoryImages:
			breakdown.Images += size
		case models.FileCategoryDocuments:
			breakdown.Documents += size
		default:
			breakdown.Other += size
		}
	}
	return breakdown, nil
}

// SumSizeBreakdownByGroup mirrors SumSizeBreakdown but for an explicit
// (tenant_id, group_id) tuple — see the interface doc. Iterates the
// shared file map filtered by the tuple so the worker can compute
// per-group usage without instantiating a request-scoped registry.
func (r *FileRegistry) SumSizeBreakdownByGroup(ctx context.Context, tenantID, groupID string) (registry.StorageBreakdown, error) {
	files, err := r.ListByGroup(ctx, tenantID, groupID)
	if err != nil {
		return registry.StorageBreakdown{}, err
	}

	var breakdown registry.StorageBreakdown
	for _, file := range files {
		if file.File == nil {
			continue
		}
		size := file.SizeBytes
		if file.LinkedEntityType == "export" {
			breakdown.Exports += size
			continue
		}
		switch file.Category {
		case models.FileCategoryImages:
			breakdown.Images += size
		case models.FileCategoryDocuments:
			breakdown.Documents += size
		default:
			breakdown.Other += size
		}
	}
	return breakdown, nil
}

// ListByLinkedEntity returns files linked to a specific entity
func (r *FileRegistry) ListByLinkedEntity(ctx context.Context, entityType, entityID string) ([]*models.FileEntity, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.FileEntity
	for _, file := range allFiles {
		if file.LinkedEntityType == entityType && file.LinkedEntityID == entityID {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

// ListByGroup returns every file belonging to the given (tenant_id, group_id)
// tuple, bypassing the registry's user/group filtering. Mirrors the postgres
// counterpart used by GroupPurgeService to avoid a full file-table scan.
func (r *FileRegistry) ListByGroup(_ context.Context, tenantID, groupID string) ([]*models.FileEntity, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var filtered []*models.FileEntity
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		file := pair.Value
		if file.TenantID == tenantID && file.GroupID == groupID {
			v := *file
			filtered = append(filtered, &v)
		}
	}
	return filtered, nil
}

// ListByLinkedEntityAndMeta returns files linked to a specific entity with specific metadata
func (r *FileRegistry) ListByLinkedEntityAndMeta(ctx context.Context, entityType, entityID, entityMeta string) ([]*models.FileEntity, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.FileEntity
	for _, file := range allFiles {
		if file.LinkedEntityType == entityType && file.LinkedEntityID == entityID && file.LinkedEntityMeta == entityMeta {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

// Enhanced methods with simplified in-memory implementations

// FullTextSearch performs simple text search on files (simplified)
func (r *FileRegistry) FullTextSearch(ctx context.Context, query string, fileType *models.FileType, options ...registry.SearchOption) ([]*models.FileEntity, error) {
	// Use the existing search method as a simplified implementation
	files, err := r.Search(ctx, query, fileType, nil, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	// Apply options
	opts := &registry.SearchOptions{Limit: len(files)}
	for _, opt := range options {
		opt(opts)
	}

	if opts.Offset > 0 && opts.Offset < len(files) {
		files = files[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(files) {
		files = files[:opts.Limit]
	}

	return files, nil
}

// FindByMimeType finds files by MIME types (simplified)
func (r *FileRegistry) FindByMimeType(ctx context.Context, mimeTypes []string) ([]*models.FileEntity, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	var filtered []*models.FileEntity
	for _, file := range allFiles {
		if slices.Contains(mimeTypes, file.MIMEType) {
			filtered = append(filtered, file)
		}
	}

	return filtered, nil
}

// FindByDateRange finds files within a date range (simplified)
func (r *FileRegistry) FindByDateRange(ctx context.Context, startDate, endDate string) ([]*models.FileEntity, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return nil, errxtrace.Wrap("invalid start date format", err)
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, errxtrace.Wrap("invalid end date format", err)
	}

	var filtered []*models.FileEntity
	for _, file := range allFiles {
		if !file.CreatedAt.IsZero() {
			if (file.CreatedAt.Equal(start) || file.CreatedAt.After(start)) &&
				(file.CreatedAt.Equal(end) || file.CreatedAt.Before(end)) {
				filtered = append(filtered, file)
			}
		}
	}

	return filtered, nil
}

// FindLargeFiles finds files larger than the specified size (simplified)
func (r *FileRegistry) FindLargeFiles(ctx context.Context, minSizeBytes int64) ([]*models.FileEntity, error) {
	// Note: File size is not currently tracked in the FileEntity model
	// This is a placeholder implementation that returns empty results
	// In a full implementation, you would add a size field to the files table
	return []*models.FileEntity{}, nil
}
