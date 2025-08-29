package memory

import (
	"context"
	"strings"
	"time"

	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.FileRegistry = (*FileRegistry)(nil)

type baseFileRegistry = Registry[models.FileEntity, *models.FileEntity]

type FileRegistry struct {
	*baseFileRegistry

	userID string
}

func NewFileRegistry() *FileRegistry {
	return &FileRegistry{
		baseFileRegistry: NewRegistry[models.FileEntity, *models.FileEntity](),
	}
}

func (r *FileRegistry) MustWithCurrentUser(ctx context.Context) registry.FileRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *FileRegistry) WithCurrentUser(ctx context.Context) (registry.FileRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user from context")
	}
	tmp.userID = user.ID
	return &tmp, nil
}

func (r *FileRegistry) WithServiceAccount() registry.FileRegistry {
	// For memory registries, service account access is the same as regular access
	// since memory registries don't enforce RLS restrictions
	return r
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

//nolint:gocognit // TODO: refactor
func (r *FileRegistry) Search(ctx context.Context, query string, fileType *models.FileType, tags []string) ([]*models.FileEntity, error) {
	allFiles, err := r.List(ctx)
	if err != nil {
		return nil, err
	}

	query = strings.ToLower(query)
	var filtered []*models.FileEntity

	for _, file := range allFiles {
		// Filter by type if specified
		if fileType != nil && file.Type != *fileType {
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

func (r *FileRegistry) ListPaginated(ctx context.Context, offset, limit int, fileType *models.FileType) ([]*models.FileEntity, int, error) {
	var allFiles []*models.FileEntity
	var err error

	if fileType != nil {
		allFiles, err = r.ListByType(ctx, *fileType)
	} else {
		allFiles, err = r.List(ctx)
	}

	if err != nil {
		return nil, 0, err
	}

	total := len(allFiles)

	// Apply pagination
	start := offset
	if start > total {
		start = total
	}

	end := start + limit
	if end > total {
		end = total
	}

	paginatedFiles := allFiles[start:end]
	return paginatedFiles, total, nil
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
	files, err := r.Search(ctx, query, fileType, nil)
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
		for _, mimeType := range mimeTypes {
			if file.MIMEType == mimeType {
				filtered = append(filtered, file)
				break
			}
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
		return nil, errkit.Wrap(err, "invalid start date format")
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return nil, errkit.Wrap(err, "invalid end date format")
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
