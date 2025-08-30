package postgres

import (
	"context"
	"log/slog"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.CommodityRegistry = (*CommodityRegistry)(nil)

type CommodityRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

func NewCommodityRegistry(dbx *sqlx.DB) *CommodityRegistry {
	return NewCommodityRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCommodityRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CommodityRegistry {
	return &CommodityRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *CommodityRegistry) MustWithCurrentUser(ctx context.Context) registry.CommodityRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *CommodityRegistry) WithCurrentUser(ctx context.Context) (registry.CommodityRegistry, error) {
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

func (r *CommodityRegistry) WithServiceAccount() registry.CommodityRegistry {
	tmp := *r
	tmp.userID = ""
	tmp.tenantID = ""
	tmp.service = true
	return &tmp
}

func (r *CommodityRegistry) Get(ctx context.Context, id string) (*models.Commodity, error) {
	slog.Info("Getting commodity", "commodity_id", id, "user_id", r.userID, "tenant_id", r.tenantID, "service_mode", r.service)
	return r.get(ctx, id)
}

func (r *CommodityRegistry) Create(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, commodity, func(ctx context.Context, tx *sqlx.Tx) error {
		_, err := r.getArea(ctx, tx, commodity.AreaID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create commodity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) GetByName(ctx context.Context, name string) (*models.Commodity, error) {
	var commodity models.Commodity
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("name", name), &commodity)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) List(ctx context.Context) ([]*models.Commodity, error) {
	var commodities []*models.Commodity

	reg := r.newSQLRegistry()

	// Query the database for all commodities (atomic operation)
	for commodity, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list commodities")
		}
		commodities = append(commodities, &commodity)
	}

	return commodities, nil
}

func (r *CommodityRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count commodities")
	}

	return cnt, nil
}

func (r *CommodityRegistry) Update(ctx context.Context, commodity models.Commodity) (*models.Commodity, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, commodity, func(ctx context.Context, tx *sqlx.Tx, dbCommodity models.Commodity) error {
		_, err := r.getArea(ctx, tx, commodity.AreaID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update commodity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, nil)
	return err
}

func (r *CommodityRegistry) newSQLRegistry() *store.RLSRepository[models.Commodity, *models.Commodity] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Commodity](r.dbx, r.tableNames.Commodities())
	}
	return store.NewUserAwareSQLRegistry[models.Commodity](r.dbx, r.userID, r.tenantID, r.tableNames.Commodities())
}

func (r *CommodityRegistry) get(ctx context.Context, id string) (*models.Commodity, error) {
	slog.Info("Getting commodity", "commodity_id", id, "user_id", r.userID, "tenant_id", r.tenantID, "service_mode", r.service)

	var commodity models.Commodity
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &commodity)
	if err != nil {
		// Add debug logging for RLS issues
		slog.Warn("Commodity not found - possible RLS issue",
			"commodity_id", id,
			"user_id", r.userID,
			"tenant_id", r.tenantID,
			"service_mode", r.service,
		)
		return nil, errkit.Wrap(err, "failed to get commodity")
	}

	return &commodity, nil
}

func (r *CommodityRegistry) getArea(ctx context.Context, tx *sqlx.Tx, areaID string) (*models.Area, error) {
	var area models.Area
	areaReg := store.NewTxRegistry[models.Area](tx, r.tableNames.Areas())
	err := areaReg.ScanOneByField(ctx, store.Pair("id", areaID), &area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	return &area, nil
}

// File-related methods

func (r *CommodityRegistry) GetImages(ctx context.Context, commodityID string) ([]string, error) {
	var images []string

	reg := r.newSQLRegistry()
	err := reg.DoWithEntityID(ctx, commodityID, func(ctx context.Context, tx *sqlx.Tx, _ models.Commodity) error {
		var err error
		images, err = r.getImages(ctx, tx, commodityID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list images")
	}

	return images, nil
}

func (r *CommodityRegistry) getImages(ctx context.Context, tx *sqlx.Tx, commodityID string) ([]string, error) {
	var images []string

	imageReg := store.NewTxRegistry[models.Image](tx, r.tableNames.Images())
	for image, err := range imageReg.ScanByField(ctx, store.Pair("commodity_id", commodityID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list images")
		}
		images = append(images, image.GetID())
	}

	return images, nil
}

func (r *CommodityRegistry) GetManuals(ctx context.Context, commodityID string) ([]string, error) {
	var manuals []string

	reg := r.newSQLRegistry()
	err := reg.DoWithEntityID(ctx, commodityID, func(ctx context.Context, tx *sqlx.Tx, _ models.Commodity) error {
		var err error
		manuals, err = r.getManuals(ctx, tx, commodityID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list manuals")
	}

	return manuals, nil
}

func (r *CommodityRegistry) getManuals(ctx context.Context, tx *sqlx.Tx, commodityID string) ([]string, error) {
	var manuals []string

	manualReg := store.NewTxRegistry[models.Manual](tx, r.tableNames.Manuals())
	for manual, err := range manualReg.ScanByField(ctx, store.Pair("commodity_id", commodityID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list manuals")
		}
		manuals = append(manuals, manual.GetID())
	}

	return manuals, nil
}

func (r *CommodityRegistry) GetInvoices(ctx context.Context, commodityID string) ([]string, error) {
	var invoices []string

	reg := r.newSQLRegistry()
	err := reg.DoWithEntityID(ctx, commodityID, func(ctx context.Context, tx *sqlx.Tx, _ models.Commodity) error {
		var err error
		invoices, err = r.getInvoices(ctx, tx, commodityID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list invoices")
	}

	return invoices, nil
}

func (r *CommodityRegistry) getInvoices(ctx context.Context, tx *sqlx.Tx, commodityID string) ([]string, error) {
	var invoices []string

	invoiceReg := store.NewTxRegistry[models.Invoice](tx, r.tableNames.Invoices())
	for invoice, err := range invoiceReg.ScanByField(ctx, store.Pair("commodity_id", commodityID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list invoices")
		}
		invoices = append(invoices, invoice.GetID())
	}

	return invoices, nil
}
