package postgres

import (
	"context"
	"fmt"
	"strings"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// CommodityEventRegistryFactory creates CommodityEventRegistry instances with proper context.
type CommodityEventRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// CommodityEventRegistry is the postgres-backed audit log for commodity state changes.
type CommodityEventRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.CommodityEventRegistry        = (*CommodityEventRegistry)(nil)
	_ registry.CommodityEventRegistryFactory = (*CommodityEventRegistryFactory)(nil)
)

func NewCommodityEventRegistry(dbx *sqlx.DB) *CommodityEventRegistryFactory {
	return NewCommodityEventRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCommodityEventRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CommodityEventRegistryFactory {
	return &CommodityEventRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *CommodityEventRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityEventRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityEventRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityEventRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &CommodityEventRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *CommodityEventRegistryFactory) CreateServiceRegistry() registry.CommodityEventRegistry {
	return &CommodityEventRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *CommodityEventRegistry) newSQLRegistry() *store.RLSGroupRepository[models.CommodityEvent, *models.CommodityEvent] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.CommodityEvent](r.dbx, r.tableNames.CommodityEvents())
	}
	return store.NewGroupAwareSQLRegistry[models.CommodityEvent](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CommodityEvents())
}

func (r *CommodityEventRegistry) Get(ctx context.Context, id string) (*models.CommodityEvent, error) {
	var event models.CommodityEvent
	err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &event)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get commodity event", err)
	}
	return &event, nil
}

func (r *CommodityEventRegistry) Create(ctx context.Context, event models.CommodityEvent) (*models.CommodityEvent, error) {
	created, err := r.newSQLRegistry().Create(ctx, event, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity event", err)
	}
	return &created, nil
}

// List returns every event in the current group, newest first. Mostly used
// by tests / future cross-commodity feeds; the FE always reads through
// ListByCommodity.
func (r *CommodityEventRegistry) List(ctx context.Context) ([]*models.CommodityEvent, error) {
	var events []*models.CommodityEvent
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf("SELECT * FROM %s ORDER BY occurred_at DESC, id ASC", r.tableNames.CommodityEvents())
		rows, err := tx.QueryxContext(ctx, query)
		if err != nil {
			return errxtrace.Wrap("failed to list commodity events", err)
		}
		defer rows.Close()
		for rows.Next() {
			var ev models.CommodityEvent
			if err := rows.StructScan(&ev); err != nil {
				return errxtrace.Wrap("failed to scan commodity event", err)
			}
			events = append(events, &ev)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity events", err)
	}
	return events, nil
}

func (r *CommodityEventRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count commodity events", err)
	}
	return cnt, nil
}

// Update is implemented for interface conformance only — events are
// append-only, so callers should never go through this. Returns the input
// untouched without writing to the DB.
func (r *CommodityEventRegistry) Update(_ context.Context, event models.CommodityEvent) (*models.CommodityEvent, error) {
	return &event, nil
}

func (r *CommodityEventRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

// ListByCommodity backs GET /commodities/{id}/events. Newest-first; the
// composite index (group_id, commodity_id, occurred_at) supports the sort
// without a separate ORDER BY pass. Pagination is offset/limit; total is
// the filtered post-Kinds count so the FE can render an accurate paginator.
func (r *CommodityEventRegistry) ListByCommodity(ctx context.Context, commodityID string, offset, limit int, opts registry.CommodityEventListOptions) ([]*models.CommodityEvent, int, error) {
	if offset < 0 {
		offset = 0
	}
	if limit < 0 {
		limit = 0
	}

	var events []*models.CommodityEvent
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		conds := []string{"commodity_id = $1"}
		args := []any{commodityID}
		idx := 2
		if len(opts.Kinds) > 0 {
			placeholders := make([]string, len(opts.Kinds))
			for i, k := range opts.Kinds {
				placeholders[i] = fmt.Sprintf("$%d", idx)
				args = append(args, string(k))
				idx++
			}
			conds = append(conds, fmt.Sprintf("kind IN (%s)", strings.Join(placeholders, ", ")))
		}
		whereClause := "WHERE " + strings.Join(conds, " AND ")

		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, r.tableNames.CommodityEvents(), whereClause)
		if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return errxtrace.Wrap("failed to count commodity events", err)
		}

		dataArgs := append([]any{}, args...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(`
			SELECT * FROM %s
			%s
			ORDER BY occurred_at DESC, id DESC
			LIMIT $%d OFFSET $%d`,
			r.tableNames.CommodityEvents(),
			whereClause,
			len(args)+1, len(args)+2,
		)

		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to list commodity events", err)
		}
		defer rows.Close()
		for rows.Next() {
			var ev models.CommodityEvent
			if err := rows.StructScan(&ev); err != nil {
				return errxtrace.Wrap("failed to scan commodity event", err)
			}
			events = append(events, &ev)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list commodity events by commodity", err)
	}

	return events, total, nil
}
