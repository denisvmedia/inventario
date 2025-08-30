package postgres

import (
	"context"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.ImageRegistry = (*ImageRegistry)(nil)

type ImageRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

func NewImageRegistry(dbx *sqlx.DB) *ImageRegistry {
	return NewImageRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewImageRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *ImageRegistry {
	return &ImageRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *ImageRegistry) MustWithCurrentUser(ctx context.Context) registry.ImageRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *ImageRegistry) WithCurrentUser(ctx context.Context) (registry.ImageRegistry, error) {
	tmp := *r

	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}
	tmp.userID = user.ID
	tmp.tenantID = user.TenantID
	tmp.service = false
	return &tmp, nil
}

func (r *ImageRegistry) WithServiceAccount() registry.ImageRegistry {
	tmp := *r
	tmp.userID = ""
	tmp.tenantID = ""
	tmp.service = true
	return &tmp
}

func (r *ImageRegistry) Get(ctx context.Context, id string) (*models.Image, error) {
	return r.get(ctx, id)
}

func (r *ImageRegistry) List(ctx context.Context) ([]*models.Image, error) {
	var images []*models.Image

	reg := r.newSQLRegistry()

	// Query the database for all images (atomic operation)
	for image, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list images")
		}
		images = append(images, &image)
	}

	return images, nil
}

func (r *ImageRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count images")
	}

	return cnt, nil
}

func (r *ImageRegistry) Create(ctx context.Context, image models.Image) (*models.Image, error) {
	// Always generate a new server-side ID for security (ignore any user-provided ID)
	image.SetID(generateID())
	image.SetTenantID(r.tenantID)
	image.SetUserID(r.userID)

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, image, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", image.CommodityID), &commodity)
		if err != nil {
			return errkit.Wrap(err, "failed to get commodity")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create image")
	}

	return &image, nil
}

func (r *ImageRegistry) Update(ctx context.Context, image models.Image) (*models.Image, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, image, func(ctx context.Context, tx *sqlx.Tx, dbImage models.Image) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", image.CommodityID), &commodity)
		if err != nil {
			return errkit.Wrap(err, "failed to get commodity")
		}
		// TODO: what if commodity has changed, allow or not? (currently allowed)
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update image")
	}

	return &image, nil
}

func (r *ImageRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *ImageRegistry) newSQLRegistry() *store.RLSRepository[models.Image, *models.Image] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Image](r.dbx, r.tableNames.Images())
	}
	return store.NewUserAwareSQLRegistry[models.Image](r.dbx, r.userID, r.tenantID, r.tableNames.Images())
}

func (r *ImageRegistry) get(ctx context.Context, id string) (*models.Image, error) {
	var image models.Image
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &image)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get image")
	}

	return &image, nil
}
