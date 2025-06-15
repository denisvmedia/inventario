package memory

import (
	"context"
	"strings"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.FileRegistry = (*FileRegistry)(nil)

type baseFileRegistry = Registry[models.FileEntity, *models.FileEntity]

type FileRegistry struct {
	*baseFileRegistry
}

func NewFileRegistry() *FileRegistry {
	return &FileRegistry{
		baseFileRegistry: NewRegistry[models.FileEntity, *models.FileEntity](),
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
