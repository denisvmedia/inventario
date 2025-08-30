package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.LocationRegistry = (*LocationRegistry)(nil)

type LocationRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
	userID     string
	tenantID   string
	service    bool
}

func NewLocationRegistry(dbx *sqlx.DB) *LocationRegistry {
	return NewLocationRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewLocationRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *LocationRegistry {
	return &LocationRegistry{
		dbx:        dbx,
		tableNames: tableNames,
	}
}

func (r *LocationRegistry) MustWithCurrentUser(ctx context.Context) registry.LocationRegistry {
	return must.Must(r.WithCurrentUser(ctx))
}

func (r *LocationRegistry) WithCurrentUser(ctx context.Context) (registry.LocationRegistry, error) {
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

func (r *LocationRegistry) WithServiceAccount() registry.LocationRegistry {
	tmp := *r
	tmp.userID = ""
	tmp.tenantID = ""
	tmp.service = true
	return &tmp
}

func (r *LocationRegistry) Get(ctx context.Context, id string) (*models.Location, error) {
	return r.get(ctx, id)
}

func (r *LocationRegistry) List(ctx context.Context) ([]*models.Location, error) {
	var locations []*models.Location

	reg := r.newSQLRegistry()

	// Query the database for all locations (atomic operation)
	for location, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list locations")
		}
		locations = append(locations, &location)
	}

	return locations, nil
}

func (r *LocationRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	cnt, err := reg.Count(ctx)
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count locations")
	}

	return cnt, nil
}

func (r *LocationRegistry) Create(ctx context.Context, location models.Location) (*models.Location, error) {
	if location.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	// ID, TenantID, and UserID are now set automatically by RLSRepository.Create

	reg := r.newSQLRegistry()

	err := reg.Create(ctx, location, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to create location")
	}

	return &location, nil
}

func (r *LocationRegistry) Update(ctx context.Context, location models.Location) (*models.Location, error) {
	if location.Name == "" {
		return nil, errkit.WithStack(registry.ErrFieldRequired,
			"field_name", "Name",
		)
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, location, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to update location")
	}

	return &location, nil
}

func (r *LocationRegistry) Delete(ctx context.Context, id string) error {
	reg := r.newSQLRegistry()
	err := reg.Delete(ctx, id, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check if the location has areas
		areas, err := r.getAreas(ctx, tx, id)
		if err != nil {
			return err
		}
		if len(areas) > 0 {
			return errkit.Wrap(registry.ErrCannotDelete, "location has areas")
		}
		return nil
	})

	return err
}

func (r *LocationRegistry) GetAreas(ctx context.Context, locationID string) ([]string, error) {
	var areas []string

	reg := r.newSQLRegistry()
	err := reg.DoWithEntityID(ctx, locationID, func(ctx context.Context, tx *sqlx.Tx, _ models.Location) error {
		var err error
		areas, err = r.getAreas(ctx, tx, locationID)
		return err
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to list areas")
	}

	return areas, nil
}

func (r *LocationRegistry) newSQLRegistry() *store.RLSRepository[models.Location, *models.Location] {
	if r.service {
		return store.NewServiceSQLRegistry[models.Location](r.dbx, r.tableNames.Locations())
	}
	return store.NewUserAwareSQLRegistry[models.Location](r.dbx, r.userID, r.tenantID, r.tableNames.Locations())
}

func (r *LocationRegistry) get(ctx context.Context, id string) (*models.Location, error) {
	var location models.Location
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &location)
	if err != nil {
		return nil, errkit.Wrap(err, "failed to get location")
	}

	return &location, nil
}

func (r *LocationRegistry) getAreas(ctx context.Context, tx *sqlx.Tx, locationID string) ([]string, error) {
	var areas []string

	areaReg := store.NewTxRegistry[models.Area](tx, r.tableNames.Areas())
	for area, err := range areaReg.ScanByField(ctx, store.Pair("location_id", locationID)) {
		if err != nil {
			return nil, errkit.Wrap(err, "failed to list areas")
		}
		areas = append(areas, area.GetID())
	}

	return areas, nil
}

// GetAreaCount returns the number of areas in a location
func (r *LocationRegistry) GetAreaCount(ctx context.Context, locationID string) (int, error) {
	areas, err := r.GetAreas(ctx, locationID)
	if err != nil {
		return 0, err
	}
	return len(areas), nil
}

// GetTotalCommodityCount returns the total number of commodities across all areas in a location
func (r *LocationRegistry) GetTotalCommodityCount(ctx context.Context, locationID string) (int, error) {
	var totalCount int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		sql := fmt.Sprintf(`
			SELECT COUNT(c.id)
			FROM %s c
			JOIN %s a ON c.area_id = a.id
			WHERE a.location_id = $1
			AND c.draft = false
		`, r.tableNames.Commodities(), r.tableNames.Areas())

		err := tx.GetContext(ctx, &totalCount, sql, locationID)
		if err != nil {
			return errkit.Wrap(err, "failed to count commodities in location")
		}
		return nil
	})
	if err != nil {
		return 0, errkit.Wrap(err, "failed to count commodities")
	}

	return totalCount, nil
}

// SearchByName searches locations by name using PostgreSQL text search
func (r *LocationRegistry) SearchByName(ctx context.Context, query string) ([]*models.Location, error) {
	var locations []*models.Location

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query = strings.ToLower(query)
		sql := fmt.Sprintf(`
			SELECT * FROM %s
			WHERE LOWER(name) LIKE $1
			ORDER BY name
		`, r.tableNames.Locations())
		err := tx.SelectContext(ctx, &locations, sql, "%"+query+"%")
		if err != nil {
			return errkit.Wrap(err, "failed to search locations by name")
		}
		return nil
	})
	if err != nil {
		return nil, errkit.Wrap(err, "failed to search locations")
	}

	return locations, nil
}
