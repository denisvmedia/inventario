package postgres

import (
	"context"
	"errors"

	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ImageRegistry = (*ImageRegistry)(nil)

type ImageRegistry struct {
	dbx        *sqlx.DB
	tableNames TableNames
}

func NewImageRegistry(dbx *sqlx.DB) *ImageRegistry {
	return NewImageRegistryWithTableNames(dbx, DefaultTableNames)
}

func NewImageRegistryWithTableNames(dbx *sqlx.DB, tableNames TableNames) *ImageRegistry {
	return &ImageRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// SetUserContext sets the user context for RLS policies
func (r *ImageRegistry) SetUserContext(ctx context.Context, userID string) error {
	return SetUserContext(ctx, r.dbx, userID)
}

// WithUserContext executes a function with user context set
func (r *ImageRegistry) WithUserContext(ctx context.Context, userID string, fn func(context.Context) error) error {
	return WithUserContext(ctx, r.dbx, userID, fn)
}

func (r *ImageRegistry) Create(ctx context.Context, image models.Image) (*models.Image, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the commodity exists
	var commodity models.Commodity
	err = ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", image.CommodityID, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// Generate a new ID if one is not already provided
	if image.GetID() == "" {
		image.SetID(generateID())
	}

	err = InsertEntity(ctx, tx, r.tableNames.Images(), image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &image, nil
}

func (r *ImageRegistry) Get(ctx context.Context, id string) (*models.Image, error) {
	return r.get(ctx, r.dbx, id)
}

func (r *ImageRegistry) List(ctx context.Context) ([]*models.Image, error) {
	var images []*models.Image

	// Query the database for all locations (atomic operation)
	for image, err := range ScanEntities[models.Image](ctx, r.dbx, r.tableNames.Images()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list images")
		}
		images = append(images, &image)
	}

	return images, nil
}

func (r *ImageRegistry) Update(ctx context.Context, image models.Image) (*models.Image, error) {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the image exists
	_, err = r.get(ctx, tx, image.ID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get image")
	}

	// Check if the commodity exists
	_, err = r.getCommodity(ctx, tx, image.CommodityID)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	// TODO: what if commodity has changed, allow or not? (currently allowed)

	err = UpdateEntityByField(ctx, tx, r.tableNames.Images(), "id", image.ID, image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update image")
	}

	return &image, nil
}

func (r *ImageRegistry) Delete(ctx context.Context, id string) error {
	// Begin a transaction (atomic operation)
	tx, err := r.dbx.Beginx()
	if err != nil {
		return errkit.Wrap(err, "failed to begin transaction")
	}
	defer func() {
		err = errors.Join(err, RollbackOrCommit(tx, err))
	}()

	// Check if the image exists
	_, err = r.get(ctx, tx, id)
	if err != nil {
		return err
	}

	// Finally, delete the image
	err = DeleteEntityByField(ctx, tx, r.tableNames.Images(), "id", id)
	if err != nil {
		return errkit.Wrap(err, "failed to delete image")
	}

	return nil
}

func (r *ImageRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := CountEntities(ctx, r.dbx, r.tableNames.Images())
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count images")
	}

	return cnt, nil
}

func (r *ImageRegistry) get(ctx context.Context, tx sqlx.ExtContext, id string) (*models.Image, error) {
	var image models.Image
	err := ScanEntityByField(ctx, tx, r.tableNames.Images(), "id", id, &image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get image")
	}

	return &image, nil
}

func (r *ImageRegistry) getCommodity(ctx context.Context, tx sqlx.ExtContext, commodityID string) (*models.Commodity, error) {
	var commodity models.Commodity
	err := ScanEntityByField(ctx, tx, r.tableNames.Commodities(), "id", commodityID, &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	return &commodity, nil
}

// User-aware methods that automatically use user context from the request context

// CreateWithUser creates an image with user context
func (r *ImageRegistry) CreateWithUser(ctx context.Context, image models.Image) (*models.Image, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user_id on the image
	image.SetUserID(userID)

	// Generate a new ID if one is not already provided
	if image.GetID() == "" {
		image.SetID(generateID())
	}

	// Set user context for RLS and insert the image
	err := InsertEntityWithUser(ctx, r.dbx, r.tableNames.Images(), image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to insert entity")
	}

	return &image, nil
}

// GetWithUser gets an image with user context
func (r *ImageRegistry) GetWithUser(ctx context.Context, id string) (*models.Image, error) {
	var image models.Image
	err := ScanEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Images(), "id", id, &image)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, errkit.WithStack(registry.ErrNotFound,
				"entity_type", "Image",
				"entity_id", id,
			)
		}
		return nil, errkit.Wrap(err, "failed to get entity")
	}

	return &image, nil
}

// ListWithUser lists images with user context
func (r *ImageRegistry) ListWithUser(ctx context.Context) ([]*models.Image, error) {
	var images []*models.Image

	// Query the database for all images with user context
	for image, err := range ScanEntitiesWithUser[models.Image](ctx, r.dbx, r.tableNames.Images()) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list images")
		}
		images = append(images, &image)
	}

	return images, nil
}

// UpdateWithUser updates an image with user context
func (r *ImageRegistry) UpdateWithUser(ctx context.Context, image models.Image) (*models.Image, error) {
	// Extract user ID from context
	userID := registry.UserIDFromContext(ctx)
	if userID == "" {
		return nil, errkit.WithStack(registry.ErrUserContextRequired)
	}

	// Set user_id on the image
	image.SetUserID(userID)

	// Update the image with user context
	err := UpdateEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Images(), "id", image.ID, image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update entity")
	}

	return &image, nil
}

// DeleteWithUser deletes an image with user context
func (r *ImageRegistry) DeleteWithUser(ctx context.Context, id string) error {
	return DeleteEntityByFieldWithUser(ctx, r.dbx, r.tableNames.Images(), "id", id)
}

// CountWithUser counts images with user context
func (r *ImageRegistry) CountWithUser(ctx context.Context) (int, error) {
	return CountEntitiesWithUser(ctx, r.dbx, r.tableNames.Images())
}
