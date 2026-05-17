package postgres

import (
	"context"
	"fmt"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// SupplyLinkRegistryFactory creates SupplyLinkRegistry instances with proper context (#1369).
// Mirrors the loan/service registry factory shape exactly: a thin wrapper
// around (dbx, tableNames) that hands out group- or service-scoped registries.
type SupplyLinkRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// SupplyLinkRegistry is the postgres-backed group-scoped registry of
// commodity_supply_links. Group + tenant isolation is enforced by RLS;
// the in-process registry simply chooses the inventario_app vs
// inventario_background_worker role via newSQLRegistry below.
type SupplyLinkRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.SupplyLinkRegistry        = (*SupplyLinkRegistry)(nil)
	_ registry.SupplyLinkRegistryFactory = (*SupplyLinkRegistryFactory)(nil)
)

func NewSupplyLinkRegistry(dbx *sqlx.DB) *SupplyLinkRegistryFactory {
	return NewSupplyLinkRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewSupplyLinkRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *SupplyLinkRegistryFactory {
	return &SupplyLinkRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *SupplyLinkRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SupplyLinkRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SupplyLinkRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SupplyLinkRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &SupplyLinkRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *SupplyLinkRegistryFactory) CreateServiceRegistry() registry.SupplyLinkRegistry {
	return &SupplyLinkRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *SupplyLinkRegistry) newSQLRegistry() *store.RLSGroupRepository[models.SupplyLink, *models.SupplyLink] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.SupplyLink](r.dbx, r.tableNames.CommoditySupplyLinks())
	}
	return store.NewGroupAwareSQLRegistry[models.SupplyLink](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CommoditySupplyLinks())
}

func (r *SupplyLinkRegistry) Get(ctx context.Context, id string) (*models.SupplyLink, error) {
	var link models.SupplyLink
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &link); err != nil {
		return nil, errxtrace.Wrap("failed to get supply link", err)
	}
	return &link, nil
}

func (r *SupplyLinkRegistry) List(ctx context.Context) ([]*models.SupplyLink, error) {
	var links []*models.SupplyLink
	for link, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list supply links", err)
		}
		l := link
		links = append(links, &l)
	}
	return links, nil
}

func (r *SupplyLinkRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count supply links", err)
	}
	return cnt, nil
}

func (r *SupplyLinkRegistry) Create(ctx context.Context, link models.SupplyLink) (*models.SupplyLink, error) {
	now := time.Now()
	link.CreatedAt = now
	link.UpdatedAt = now
	created, err := r.newSQLRegistry().Create(ctx, link, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create supply link", err)
	}
	return &created, nil
}

func (r *SupplyLinkRegistry) Update(ctx context.Context, link models.SupplyLink) (*models.SupplyLink, error) {
	link.UpdatedAt = time.Now()
	if err := r.newSQLRegistry().Update(ctx, link, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update supply link", err)
	}
	return &link, nil
}

func (r *SupplyLinkRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

// ListByCommodity returns the supply links for one commodity, ordered
// by sort_order ASC, created_at ASC as a stable tiebreaker. The index
// idx_supply_links_commodity covers (commodity_id, sort_order).
func (r *SupplyLinkRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.SupplyLink, error) {
	var links []*models.SupplyLink
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE commodity_id = $1 ORDER BY sort_order ASC, created_at ASC`,
			r.tableNames.CommoditySupplyLinks())
		rows, err := tx.QueryxContext(ctx, query, commodityID)
		if err != nil {
			return errxtrace.Wrap("failed to query supply links", err)
		}
		defer rows.Close()
		for rows.Next() {
			var link models.SupplyLink
			if err := rows.StructScan(&link); err != nil {
				return errxtrace.Wrap("failed to scan supply link", err)
			}
			l := link
			links = append(links, &l)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list supply links for commodity", err)
	}
	return links, nil
}

// ReorderForCommodity densely renumbers sort_order = position for each
// id in orderedIDs (0..N-1) inside a single transaction. Ids not
// belonging to the commodity surface as ErrNotFound — no partial
// reorder, no silent skips. Ids of the commodity that the caller didn't
// list keep their prior sort_order (we don't reshuffle the rest).
func (r *SupplyLinkRegistry) ReorderForCommodity(ctx context.Context, commodityID string, orderedIDs []string) error {
	if len(orderedIDs) == 0 {
		return nil
	}
	reg := r.newSQLRegistry()
	return reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		now := time.Now()
		updateSQL := fmt.Sprintf(
			`UPDATE %s SET sort_order = $1, updated_at = $2
			 WHERE id = $3 AND commodity_id = $4`,
			r.tableNames.CommoditySupplyLinks())
		for i, id := range orderedIDs {
			res, err := tx.ExecContext(ctx, updateSQL, i, now, id, commodityID)
			if err != nil {
				return errxtrace.Wrap("failed to update supply link sort_order", err)
			}
			n, err := res.RowsAffected()
			if err != nil {
				return errxtrace.Wrap("failed to read rows affected", err)
			}
			if n == 0 {
				return registry.ErrNotFound
			}
		}
		return nil
	})
}

// CountByCommodity returns, for each id in commodityIDs, the total
// number of supply links. Empty input returns an empty map; missing
// ids map to 0. Mirrors CommodityLoanRegistry.CountOpenByCommodity.
func (r *SupplyLinkRegistry) CountByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
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
			 WHERE commodity_id = ANY($1)
			 GROUP BY commodity_id`,
			r.tableNames.CommoditySupplyLinks())
		rows, err := tx.QueryxContext(ctx, query, commodityIDs)
		if err != nil {
			return errxtrace.Wrap("failed to query supply link counts", err)
		}
		defer rows.Close()
		for rows.Next() {
			var commodityID string
			var cnt int
			if err := rows.Scan(&commodityID, &cnt); err != nil {
				return errxtrace.Wrap("failed to scan supply link count", err)
			}
			out[commodityID] = cnt
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count supply links by commodity", err)
	}
	return out, nil
}
