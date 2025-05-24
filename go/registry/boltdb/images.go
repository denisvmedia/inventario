package boltdb

import (
	"context"

	bolt "go.etcd.io/bbolt"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const (
	entityNameImage = "image"

	bucketNameImages         = "images"
	bucketNameImagesChildren = "images-children"
)

var _ registry.ImageRegistry = (*ImageRegistry)(nil)

type ImageRegistry struct {
	db                *bolt.DB
	base              *dbx.BaseRepository[models.Image, *models.Image]
	registry          *Registry[models.Image, *models.Image]
	commodityRegistry registry.CommodityRegistry
}

func NewImageRegistry(db *bolt.DB, commodityRegistry registry.CommodityRegistry) *ImageRegistry {
	base := dbx.NewBaseRepository[models.Image, *models.Image](bucketNameImages)

	return &ImageRegistry{
		db:   db,
		base: base,
		registry: NewRegistry[models.Image, *models.Image](
			db,
			base,
			entityNameImage,
			bucketNameImagesChildren,
		),
		commodityRegistry: commodityRegistry,
	}
}

func (r *ImageRegistry) Create(ctx context.Context, m models.Image) (*models.Image, error) {
	result, err := r.registry.Create(m, func(_tx dbx.TransactionOrBucket, _image *models.Image) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _image *models.Image) error {
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create image")
	}

	err = r.commodityRegistry.AddImage(ctx, result.CommodityID, result.ID)
	if err != nil {
		return result, errkit.Wrap(err, "failed to add image to commodity")
	}

	return result, nil
}

func (r *ImageRegistry) Get(_ context.Context, id string) (*models.Image, error) {
	return r.registry.Get(id)
}

func (r *ImageRegistry) List(_ context.Context) ([]*models.Image, error) {
	return r.registry.List()
}

func (r *ImageRegistry) Update(_ context.Context, m models.Image) (*models.Image, error) {
	return r.registry.Update(m, func(_tx dbx.TransactionOrBucket, _image *models.Image) error {
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.Image) error {
		return nil
	})
}

func (r *ImageRegistry) Delete(ctx context.Context, id string) error {
	var commodityID string
	err := r.registry.Delete(id, func(_tx dbx.TransactionOrBucket, image *models.Image) error {
		commodityID = image.CommodityID
		return nil
	}, func(_tx dbx.TransactionOrBucket, _result *models.Image) error {
		return nil
	})
	if err != nil {
		return errkit.Wrap(err, "failed to delete image")
	}

	err = r.commodityRegistry.DeleteImage(ctx, commodityID, id)
	if err != nil {
		return errkit.Wrap(err, "failed to remove image from commodity")
	}

	return nil
}

func (r *ImageRegistry) Count(_ context.Context) (int, error) {
	return r.registry.Count()
}
