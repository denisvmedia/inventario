package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// AreaRegistryFactory creates AreaRegistry instances with proper context
type AreaRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// AreaRegistry is a context-aware registry that can only be created through the factory
type AreaRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

var _ registry.AreaRegistry = (*AreaRegistry)(nil)
var _ registry.AreaRegistryFactory = (*AreaRegistryFactory)(nil)

func NewAreaRegistry(dbx *sqlx.DB) *AreaRegistryFactory {
	return NewAreaRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewAreaRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *AreaRegistryFactory {
	return &AreaRegistryFactory{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

// Factory methods implementing registry.AreaRegistryFactory

func (f *AreaRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.AreaRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *AreaRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.AreaRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get user ID from context")
	}

	return &AreaRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     user.ID,
		tenantID:   user.TenantID,
		service:    false,
	}, nil
}

func (f *AreaRegistryFactory) CreateServiceRegistry() registry.AreaRegistry {
	return &AreaRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		userID:     "",
		tenantID:   "",
		service:    true,
	}
}

func (r *AreaRegistry) Get(ctx context.Context, id string) (*models.Area, error) {
	return r.get(ctx, id)
}

func (r *AreaRegistry) List(ctx context.Context) ([]*models.Area, error) {
	var areas []*models.Area

	reg := r.newSQLRegistry()

	// Query the database for all locations (atomic operation)
	for area, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list areas")
		}
		areas = append(areas, &area)
	}

	return areas, nil
}

func (r *AreaRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count areas")
	}

	return cnt, nil
}

func (r *AreaRegistry) Create(ctx context.Context, area models.Area) (*models.Area, error) {
	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	reg := r.newSQLRegistry()

	createdArea, err := reg.Create(ctx, area, func(ctx context.Context, tx *sqlx.Tx) error {
		_, err := r.getLocation(ctx, tx, area.LocationID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to count areas")
	}

	return &createdArea, nil
}

func (r *AreaRegistry) Update(ctx context.Context, area models.Area) (*models.Area, error) {
	reg := r.newSQLRegistry()

	err := reg.Update(ctx, area, func(ctx context.Context, tx *sqlx.Tx, dbArea models.Area) error {
		_, err := r.getLocation(ctx, tx, area.LocationID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update area")
	}

	return &area, nil
}

func (r *AreaRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if the area has commodities
		commodities, err := r.getCommodities(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(commodities) > 0 {
			return errkit.Wrap(registry.ErrCannotDelete, "area has commodities")
		}
		return nil
	})

	return err
}

func (r *AreaRegistry) GetCommodities(ctx context.Context, areaID string) ([]string, error) {
	var commodities []string

	reg := r.newSQLRegistry()
	err := reg.DoWithEntityID(ctx, areaID, func(ctx context.Context, tx *sqlx.Tx, _ models.Area) error {
		var err error
		commodities, err = r.getCommodities(ctx, tx, areaID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list commodities")
	}

	return commodities, nil
}

// GetCommodityCount returns the number of commodities in an area
func (r *AreaRegistry) GetCommodityCount(ctx context.Context, areaID string) (int, error) {
	commodities, err := r.GetCommodities(ctx, areaID)
	if err != nil {
		return 0, err
	}
	return len(commodities), nil
}

// GetTotalValue calculates the total value of commodities in an area
func (r *AreaRegistry) GetTotalValue(ctx context.Context, areaID string, currency string) (float64, error) {
	var totalValue float64

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		sql := fmt.Sprintf(`
			SELECT COALESCE(SUM(COALESCE(converted_original_price, original_price)), 0)
			FROM %s
			WHERE area_id = $1
			AND (original_price_currency = $2 OR $2 = '')
			AND draft = false
	`, r.tableNames.Commodities())

		err := tx.GetContext(ctx, &totalValue, sql, areaID, currency)
		if err != nil {
			return errkit.Wrap(err, "failed to calculate total value")
		}
		return nil
	})
	if err != nil {
		return 0, errkit.Wrap(err, "failed to list commodities")
	}

	return totalValue, nil
}

// SearchByName searches areas by name using PostgreSQL text search
func (r *AreaRegistry) SearchByName(ctx context.Context, query string) ([]*models.Area, error) {
	var areas []*models.Area

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query = strings.ToLower(query)
		sql := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE LOWER(name) LIKE $1
			ORDER BY name
	`, r.tableNames.Areas())
		err := tx.SelectContext(ctx, &areas, sql, "%"+query+"%")
		if err != nil {
			return errkit.Wrap(err, "failed to search areas by name")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list commodities")
	}

	return areas, nil
}

func (r *AreaRegistry) newSQLRegistry() *store.RLSRepository[models.Area, *models.Area] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Area](r.dbx, r.tableNames.Areas())
	}
	slog.Info("Creating new user-aware SQL registry for areas", "userID", r.userID)
	return store.NewUserAwareSQLRegistry[models.Area](r.dbx, r.userID, r.tenantID, r.tableNames.Areas())
}

func (r *AreaRegistry) get(ctx context.Context, id string) (*models.Area, error) {
	var area models.Area
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &area)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get area")
	}

	return &area, nil
}

func (r *AreaRegistry) getCommodities(ctx context.Context, tx *sqlx.Tx, areaID string) ([]string, error) {
	var commodities []string

	comReg := store.NewTxRegistry[models.Commodity](tx, r.tableNames.Commodities())
	for commodity, err := range comReg.ScanByField(ctx, store.Pair("area_id", areaID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list commodities")
		}
		commodities = append(commodities, commodity.GetID())
	}

	return commodities, nil
}

func (r *AreaRegistry) getLocation(ctx context.Context, tx *sqlx.Tx, id string) (*models.Location, error) {
	var location models.Location
	txreg := store.NewTxRegistry[models.Location](tx, r.tableNames.Locations())
	err := txreg.ScanOneByField(ctx, store.Pair("id", id), &location)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get location")
	}
	return &location, nil
}
