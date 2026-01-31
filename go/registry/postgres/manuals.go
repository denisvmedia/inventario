package postgres

import (
	"context"

	"github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// ManualRegistryFactory creates ManualRegistry instances with proper context
type ManualRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// ManualRegistry is a context-aware registry that can only be created through the factory
type ManualRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.ManualRegistry = (*ManualRegistry)(nil)
var _ registry.ManualRegistryFactory = (*ManualRegistryFactory)(nil)

func NewManualRegistry(dbx *sqlx.DB) *ManualRegistryFactory {
	return NewManualRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewManualRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *ManualRegistryFactory {
	return &ManualRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.ManualRegistryFactory

func (f *ManualRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.ManualRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *ManualRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.ManualRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, stacktrace.Wrap("failed to get user ID from context", err)
	}

	return &ManualRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *ManualRegistryFactory) CreateServiceRegistry() registry.ManualRegistry {
	return &ManualRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
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
			return nil, stacktrace.Wrap("failed to list manuals", err)
		}
		manuals = append(manuals, &manual)
	}

	return manuals, nil
}

func (r *ManualRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, stacktrace.Wrap("failed to count manuals", err)
	}

	return cnt, nil
}

func (r *ManualRegistry) Create(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	reg := r.newSQLRegistry()

	createdManual, err := reg.Create(ctx, manual, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", manual.CommodityID), &commodity)
		if err != nil {
			return stacktrace.Wrap("failed to get commodity", err)
		}
		return nil
	})
	if err != nil {
		return nil, stacktrace.Wrap("failed to create manual", err)
	}

	return &createdManual, nil
}

func (r *ManualRegistry) Update(ctx context.Context, manual models.Manual) (*models.Manual, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, manual, func(ctx context.Context, tx *sqlx.Tx, dbManual models.Manual) error {
		// Check if the commodity exists
		var commodity models.Commodity
		commodityReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
		err := commodityReg.ScanOneByField(ctx, store.Pair("id", manual.CommodityID), &commodity)
		if err != nil {
			return stacktrace.Wrap("failed to get commodity", err)
		}
		// TODO: what if commodity has changed, allow or not? (currently allowed)
		return nil
	})
	if err != nil {
		return nil, stacktrace.Wrap("failed to update manual", err)
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
		return nil, stacktrace.Wrap("failed to get manual", err)
	}

	return &manual, nil
}
