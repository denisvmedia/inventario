package memory

import (
	"context"
	"slices"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// FileRegistryFactory creates FileRegistry instances with proper context
type FileRegistryFactory struct {
	baseFileRegistry *Registry[models.FileEntity, *models.FileEntity]
}

// FileRegistry is a context-aware registry that can only be created through the factory
type FileRegistry struct {
	*Registry[models.FileEntity, *models.FileEntity]

	userID string
}

var _ registry.FileRegistry = (*FileRegistry)(nil)
var _ registry.FileRegistryFactory = (*FileRegistryFactory)(nil)

func NewFileRegistryFactory() *FileRegistryFactory {
	return &FileRegistryFactory{
		baseFileRegistry: NewRegistry[models.FileEntity, *models.FileEntity](),
	}
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
		Registry: userRegistry,
		userID:   user.ID,
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
		Registry: serviceRegistry,
		userID:   "", // Clear userID to bypass user filtering
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

	total := len(allFiles)

	// Apply pagination
	start := min(offset, total)

	end := min(start+limit, total)

	paginatedFiles := allFiles[start:end]
	return paginatedFiles, total, nil
}

// CountByCategory aggregates files matching the same filters as Search,
// grouped by Category. Always returns all four buckets (zero-filled),
// keeping the response shape stable for the FE tile renderer.
func (r *FileRegistry) CountByCategory(ctx context.Context, query string, fileType *models.FileType, tags []string) (map[models.FileCategory]int, error) {
	files, err := r.Search(ctx, query, fileType, nil, tags, nil, nil)
	if err != nil {
		return nil, err
	}

	counts := map[models.FileCategory]int{
		models.FileCategoryPhotos:    0,
		models.FileCategoryInvoices:  0,
		models.FileCategoryDocuments: 0,
		models.FileCategoryOther:     0,
	}
	for _, file := range files {
		counts[file.Category]++
	}
	return counts, nil
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

type baseCommodityAndUserAwareRegistry[T any, P registry.PIDable[T]] struct {
	*Registry[T, P]

	userID            string
	commodityRegistry *CommodityRegistry // required dependency for relationship tracking
}

// createUserRegistry creates a new registry with user context from the provided context
func createUserRegistry[T any, P registry.PIDable[T]](ctx context.Context, userRegistryFactory func(userID string) *Registry[T, P], comRegFactory *CommodityRegistryFactory) (res *baseCommodityAndUserAwareRegistry[T, P], err error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}

	// Create a new registry with user context already set
	groupID := appctx.GroupIDFromContext(ctx)
	userRegistry := userRegistryFactory(user.ID)
	userRegistry.groupID = groupID // Set group-specific groupID

	// Create user-aware commodity registry
	commodityRegistryInterface, err := comRegFactory.CreateUserRegistry(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create user commodity registry", err)
	}

	// Cast to concrete type for relationship management
	commodityRegistry, ok := commodityRegistryInterface.(*CommodityRegistry)
	if !ok {
		return nil, errxtrace.ClassifyNew("failed to cast commodity registry to concrete type")
	}

	return &baseCommodityAndUserAwareRegistry[T, P]{
		Registry:          userRegistry,
		userID:            user.ID,
		commodityRegistry: commodityRegistry,
	}, nil
}
