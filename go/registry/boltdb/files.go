package boltdb

import (
	"context"
	"strings"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameFile = "file"

	bucketNameFiles         = "files"
	bucketNameFilesChildren = "files-children"
)

var _ registry.FileRegistry = (*FileRegistry)(nil)

type FileRegistry struct {
	db       *bolt.DB
	base     *dbx.BaseRepository[models.FileEntity, *models.FileEntity]
	registry *Registry[models.FileEntity, *models.FileEntity]
}

func NewFileRegistry(db *bolt.DB) *FileRegistry {
	base := dbx.NewBaseRepository[models.FileEntity, *models.FileEntity](bucketNameFiles)

	return &FileRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.FileEntity, *models.FileEntity](
			db,
			base,
			entityNameFile,
			bucketNameFilesChildren,
		),
	}
}

func (r *FileRegistry) Create(ctx context.Context, m models.FileEntity) (*models.FileEntity, error) {
	result, err := r.registry.Create(m, func(_tx dbx.TransactionOrBucket, _file *models.FileEntity) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _file *models.FileEntity) error {
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create file")
	}

	return result, nil
}

func (r *FileRegistry) Get(_ context.Context, id string) (*models.FileEntity, error) {
	return r.registry.Get(id)
}

func (r *FileRegistry) List(_ context.Context) ([]*models.FileEntity, error) {
	return r.registry.List()
}

func (r *FileRegistry) Update(_ context.Context, m models.FileEntity) (*models.FileEntity, error) {
	return r.registry.Update(m, func(_tx dbx.TransactionOrBucket, _file *models.FileEntity) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.FileEntity) error {
		return nil
	})
}

func (r *FileRegistry) Delete(ctx context.Context, id string) error {
	err := r.registry.Delete(id, func(_tx dbx.TransactionOrBucket, file *models.FileEntity) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.FileEntity) error {
		return nil
	})
	if err != nil {
		return errkit.Wrap(err, "failed to delete file")
	}

	return nil
}

func (r *FileRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
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
