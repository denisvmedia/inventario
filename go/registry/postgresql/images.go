package postgresql

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ImageRegistry = (*ImageRegistry)(nil)

type ImageRegistry struct {
	pool              *pgxpool.Pool
	commodityRegistry registry.CommodityRegistry
}

func NewImageRegistry(pool *pgxpool.Pool, commodityRegistry registry.CommodityRegistry) *ImageRegistry {
	return &ImageRegistry{
		pool:              pool,
		commodityRegistry: commodityRegistry,
	}
}

func (r *ImageRegistry) Create(ctx context.Context, image models.Image) (*models.Image, error) {
	// Validate the image
	err := validation.Validate(&image)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the commodity exists
	_, err = r.commodityRegistry.Get(ctx, image.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	// Generate a new ID
	if image.ID == "" {
		image.SetID(generateID())
	}

	// Insert the image into the database
	_, err = r.pool.Exec(ctx, `
		INSERT INTO images (id, commodity_id, path, original_path, ext, mime_type)
		VALUES ($1, $2, $3, $4, $5, $6)
	`, image.ID, image.CommodityID, image.Path, image.OriginalPath, image.Ext, image.MIMEType)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create image")
	}

	// Add the image to the commodity
	err = r.commodityRegistry.AddImage(ctx, image.CommodityID, image.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to add image to commodity")
	}

	return &image, nil
}

func (r *ImageRegistry) Get(ctx context.Context, id string) (*models.Image, error) {
	var image models.Image
	image.File = &models.File{}

	// Query the database for the image
	err := r.pool.QueryRow(ctx, `
		SELECT id, commodity_id, path, original_path, ext, mime_type
		FROM images
		WHERE id = $1
	`, id).Scan(&image.ID, &image.CommodityID, &image.Path, &image.OriginalPath, &image.Ext, &image.MIMEType)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errkit.Wrap(registry.ErrNotFound, "image not found")
		}
		return nil, errkit.Wrap(err, "failed to get image")
	}

	return &image, nil
}

func (r *ImageRegistry) List(ctx context.Context) ([]*models.Image, error) {
	var images []*models.Image

	// Query the database for all images
	rows, err := r.pool.Query(ctx, `
		SELECT id, commodity_id, path, original_path, ext, mime_type
		FROM images
	`)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list images")
	}
	defer rows.Close()

	// Process each row
	for rows.Next() {
		var image models.Image
		image.File = &models.File{}
		if err := rows.Scan(&image.ID, &image.CommodityID, &image.Path, &image.OriginalPath, &image.Ext, &image.MIMEType); err != nil {
			return nil, errkit.Wrap(err, "failed to scan row")
		}
		images = append(images, &image)
	}

	if err := rows.Err(); err != nil {
		return nil, errkit.Wrap(err, "error iterating rows")
	}

	return images, nil
}

func (r *ImageRegistry) Update(ctx context.Context, image models.Image) (*models.Image, error) {
	// Validate the image
	err := validation.Validate(&image)
	if err != nil {
		return nil, errkit.Wrap(err, "validation failed")
	}

	// Check if the image exists
	existingImage, err := r.Get(ctx, image.ID)
	if err != nil {
		return nil, err
	}

	// Check if the commodity exists
	_, err = r.commodityRegistry.Get(ctx, image.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "commodity not found")
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// If the commodity ID has changed, update the commodity references
	if existingImage.CommodityID != image.CommodityID {
		// Remove the image from the old commodity
		err = r.commodityRegistry.DeleteImage(ctx, existingImage.CommodityID, image.ID)
		if err != nil {
			return nil, err
		}

		// Add the image to the new commodity
		err = r.commodityRegistry.AddImage(ctx, image.CommodityID, image.ID)
		if err != nil {
			return nil, err
		}
	}

	// Update the image in the database
	_, err = tx.Exec(ctx, `
		UPDATE images
		SET commodity_id = $1, path = $2, original_path = $3, ext = $4, mime_type = $5
		WHERE id = $6
	`, image.CommodityID, image.Path, image.OriginalPath, image.Ext, image.MIMEType, image.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update image")
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to commit transaction")
	}

	return &image, nil
}

func (r *ImageRegistry) Delete(ctx context.Context, id string) error {
	// Check if the image exists
	image, err := r.Get(ctx, id)
	if err != nil {
		return err
	}

	// Begin a transaction
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer tx.Rollback(ctx)

	// Delete the image from the database
	_, err = tx.Exec(ctx, `
		DELETE FROM images
		WHERE id = $1
	`, id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete image")
	}

	// Remove the image from the commodity
	err = r.commodityRegistry.DeleteImage(ctx, image.CommodityID, id)
	if err != nil {
		return err
	}

	// Commit the transaction
	err = tx.Commit(ctx)
	if err != nil {
		return errkit.Wrap(err, "failed to commit transaction")
	}

	return nil
}

func (r *ImageRegistry) Count(ctx context.Context) (int, error) {
	var count int

	// Query the database for the count
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM images
	`).Scan(&count)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count images")
	}

	return count, nil
}
