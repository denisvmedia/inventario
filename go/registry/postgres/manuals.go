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

var _ registry.ManualRegistry = (*ManualRegistry)(nil)

type ManualRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

func NewManualRegistry(dbx *sqlx.DB) *ManualRegistry {
	return NewManualRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewManualRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *ManualRegistry {
	return &ManualRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *ManualRegistry) MustWithCurrentUser(ctx context.Context) registry.ManualRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *ManualRegistry) WithCurrentUser(ctx context.Context) (registry.ManualRegistry, error) {
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

func (r *ManualRegistry) WithServiceAccount() registry.ManualRegistry {
	tmp := *r
	tmp.userID = ""
	tmp.tenantID = ""
	tmp.service = true
	return &tmp
}

func (r *ManualRegistry) Get(ctx context.Context, id string) (*models.Manual, error) {
	return r.get(ctx, id)
}

func (r *ManualRegistry) List(ctx context.Context) ([]*models.Manual, error) {
	var manuals []*models.Manual

	reg := r.newSQLRegistry()

	// Query the database for all manuals (atomic operation)
	for manual, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list manuals")
		}
		manuals = append(manuals, &manual)
	}

	return manuals, nil
}

func (r *ManualRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count manuals")
	}

	return cnt, nil
}

func (r *ManualRegistry) Create(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// Generate a new ID if one is not already provided
	if manual.GetID() == "" {
		manual.SetID(generateID())
	}
	manual.SetTenantID(r.tenantID)
	manual.SetUserID(r.userID)

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, manual, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", manual.CommodityID), &commodity)
		if err != nil {
			return errkit.Wrap(err, "failed to get commodity")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create manual")
	}

	return &manual, nil
}

func (r *ManualRegistry) Update(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, manual, func(ctx context.Context, tx *sqlx.Tx, dbManual models.Manual) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", manual.CommodityID), &commodity)
		if err != nil {
			return errkit.Wrap(err, "failed to get commodity")
		}
		// TODO: what if commodity has changed, allow or not? (currently allowed)
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update manual")
	}

	return &manual, nil
}

func (r *ManualRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *ManualRegistry) newSQLRegistry() *store.RLSRepository[models.Manual, *models.Manual] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Manual](r.dbx, r.tableNames.Manuals())
	}
	return store.NewUserAwareSQLRegistry[models.Manual](r.dbx, r.userID, r.tenantID, r.tableNames.Manuals())
}

func (r *ManualRegistry) get(ctx context.Context, id string) (*models.Manual, error) {
	var manual models.Manual
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &manual)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get manual")
	}

	return &manual, nil
}
