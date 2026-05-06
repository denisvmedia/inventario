package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// CommodityServiceRegistryFactory creates CommodityServiceRegistry instances with proper context.
type CommodityServiceRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// CommodityServiceRegistry is the postgres-backed group-scoped registry of services.
type CommodityServiceRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.CommodityServiceRegistry        = (*CommodityServiceRegistry)(nil)
	_ registry.CommodityServiceRegistryFactory = (*CommodityServiceRegistryFactory)(nil)
)

func NewCommodityServiceRegistry(dbx *sqlx.DB) *CommodityServiceRegistryFactory {
	return NewCommodityServiceRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCommodityServiceRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CommodityServiceRegistryFactory {
	return &CommodityServiceRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *CommodityServiceRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityServiceRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityServiceRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityServiceRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &CommodityServiceRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *CommodityServiceRegistryFactory) CreateServiceRegistry() registry.CommodityServiceRegistry {
	return &CommodityServiceRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *CommodityServiceRegistry) newSQLRegistry() *store.RLSGroupRepository[models.CommodityService, *models.CommodityService] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.CommodityService](r.dbx, r.tableNames.CommodityServices())
	}
	return store.NewGroupAwareSQLRegistry[models.CommodityService](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CommodityServices())
}

func (r *CommodityServiceRegistry) Get(ctx context.Context, id string) (*models.CommodityService, error) {
	var svc models.CommodityService
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &svc); err != nil {
		return nil, errxtrace.Wrap("failed to get commodity service", err)
	}
	return &svc, nil
}

func (r *CommodityServiceRegistry) List(ctx context.Context) ([]*models.CommodityService, error) {
	var services []*models.CommodityService
	for svc, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list commodity services", err)
		}
		s := svc
		services = append(services, &s)
	}
	return services, nil
}

func (r *CommodityServiceRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count commodity services", err)
	}
	return cnt, nil
}

func (r *CommodityServiceRegistry) Create(ctx context.Context, svc models.CommodityService) (*models.CommodityService, error) {
	now := time.Now()
	svc.CreatedAt = now
	svc.UpdatedAt = now
	created, err := r.newSQLRegistry().Create(ctx, svc, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity service", err)
	}
	return &created, nil
}

func (r *CommodityServiceRegistry) Update(ctx context.Context, svc models.CommodityService) (*models.CommodityService, error) {
	svc.UpdatedAt = time.Now()
	if err := r.newSQLRegistry().Update(ctx, svc, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update commodity service", err)
	}
	return &svc, nil
}

func (r *CommodityServiceRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

func (r *CommodityServiceRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityService, error) {
	var services []*models.CommodityService
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE commodity_id = $1 ORDER BY sent_at DESC, created_at DESC`,
			r.tableNames.CommodityServices())
		rows, err := tx.QueryxContext(ctx, query, commodityID)
		if err != nil {
			return errxtrace.Wrap("failed to query commodity services", err)
		}
		defer rows.Close()
		for rows.Next() {
			var svc models.CommodityService
			if err := rows.StructScan(&svc); err != nil {
				return errxtrace.Wrap("failed to scan commodity service", err)
			}
			s := svc
			services = append(services, &s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity services for commodity", err)
	}
	return services, nil
}

// GetOpenForCommodity returns the (at most one) open service row for a
// commodity or registry.ErrNotFound if none exists. ORDER BY sent_at DESC
// + LIMIT 1 makes this safe against the rare "two open rows somehow" case.
func (r *CommodityServiceRegistry) GetOpenForCommodity(ctx context.Context, commodityID string) (*models.CommodityService, error) {
	var svc models.CommodityService
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE commodity_id = $1 AND returned_at IS NULL ORDER BY sent_at DESC LIMIT 1`,
			r.tableNames.CommodityServices())
		err := tx.GetContext(ctx, &svc, query, commodityID)
		if errors.Is(err, sql.ErrNoRows) {
			return registry.ErrNotFound
		}
		if err != nil {
			return errxtrace.Wrap("failed to query open service", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &svc, nil
}

func (r *CommodityServiceRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.ServiceListOptions) ([]*models.CommodityService, int, error) {
	state := opts.State
	if state == "" {
		state = registry.ServiceStateAll
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	var conditions []string
	var args []any
	switch state {
	case registry.ServiceStateOpen:
		conditions = append(conditions, "returned_at IS NULL")
	case registry.ServiceStateOverdue:
		conditions = append(conditions, "returned_at IS NULL AND expected_return_at IS NOT NULL AND expected_return_at < $1")
		args = append(args, now.Format("2006-01-02"))
	case registry.ServiceStateCompleted:
		conditions = append(conditions, "returned_at IS NOT NULL")
	case registry.ServiceStateAll:
		// no filter
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var services []*models.CommodityService
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, r.tableNames.CommodityServices(), whereClause)
		if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return errxtrace.Wrap("failed to count commodity services", err)
		}

		dataArgs := append([]any{}, args...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(
			`SELECT * FROM %s %s ORDER BY sent_at DESC, created_at DESC LIMIT $%d OFFSET $%d`,
			r.tableNames.CommodityServices(), whereClause, len(args)+1, len(args)+2)
		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to query commodity services", err)
		}
		defer rows.Close()
		for rows.Next() {
			var svc models.CommodityService
			if err := rows.StructScan(&svc); err != nil {
				return errxtrace.Wrap("failed to scan commodity service", err)
			}
			s := svc
			services = append(services, &s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list paginated commodity services", err)
	}
	return services, total, nil
}

func (r *CommodityServiceRegistry) CountOpenByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
	out := make(map[string]int, len(commodityIDs))
	for _, id := range commodityIDs {
		out[id] = 0
	}
	if len(commodityIDs) == 0 {
		return out, nil
	}

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT commodity_id, COUNT(*)::int
			 FROM %s
			 WHERE commodity_id = ANY($1) AND returned_at IS NULL
			 GROUP BY commodity_id`,
			r.tableNames.CommodityServices())
		rows, err := tx.QueryxContext(ctx, query, commodityIDs)
		if err != nil {
			return errxtrace.Wrap("failed to query open service counts", err)
		}
		defer rows.Close()
		for rows.Next() {
			var commodityID string
			var cnt int
			if err := rows.Scan(&commodityID, &cnt); err != nil {
				return errxtrace.Wrap("failed to scan open service count", err)
			}
			out[commodityID] = cnt
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count open services by commodity", err)
	}
	return out, nil
}
