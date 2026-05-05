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

// CommodityLoanRegistryFactory creates CommodityLoanRegistry instances with proper context.
type CommodityLoanRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// CommodityLoanRegistry is the postgres-backed group-scoped registry of loans.
type CommodityLoanRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.CommodityLoanRegistry        = (*CommodityLoanRegistry)(nil)
	_ registry.CommodityLoanRegistryFactory = (*CommodityLoanRegistryFactory)(nil)
)

func NewCommodityLoanRegistry(dbx *sqlx.DB) *CommodityLoanRegistryFactory {
	return NewCommodityLoanRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCommodityLoanRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CommodityLoanRegistryFactory {
	return &CommodityLoanRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *CommodityLoanRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityLoanRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityLoanRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityLoanRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &CommodityLoanRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *CommodityLoanRegistryFactory) CreateServiceRegistry() registry.CommodityLoanRegistry {
	return &CommodityLoanRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *CommodityLoanRegistry) newSQLRegistry() *store.RLSGroupRepository[models.CommodityLoan, *models.CommodityLoan] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.CommodityLoan](r.dbx, r.tableNames.CommodityLoans())
	}
	return store.NewGroupAwareSQLRegistry[models.CommodityLoan](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CommodityLoans())
}

func (r *CommodityLoanRegistry) Get(ctx context.Context, id string) (*models.CommodityLoan, error) {
	var loan models.CommodityLoan
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &loan); err != nil {
		return nil, errxtrace.Wrap("failed to get commodity loan", err)
	}
	return &loan, nil
}

func (r *CommodityLoanRegistry) List(ctx context.Context) ([]*models.CommodityLoan, error) {
	var loans []*models.CommodityLoan
	for loan, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list commodity loans", err)
		}
		l := loan
		loans = append(loans, &l)
	}
	return loans, nil
}

func (r *CommodityLoanRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count commodity loans", err)
	}
	return cnt, nil
}

func (r *CommodityLoanRegistry) Create(ctx context.Context, loan models.CommodityLoan) (*models.CommodityLoan, error) {
	now := time.Now()
	loan.CreatedAt = now
	loan.UpdatedAt = now
	created, err := r.newSQLRegistry().Create(ctx, loan, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity loan", err)
	}
	return &created, nil
}

func (r *CommodityLoanRegistry) Update(ctx context.Context, loan models.CommodityLoan) (*models.CommodityLoan, error) {
	loan.UpdatedAt = time.Now()
	if err := r.newSQLRegistry().Update(ctx, loan, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update commodity loan", err)
	}
	return &loan, nil
}

func (r *CommodityLoanRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

func (r *CommodityLoanRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityLoan, error) {
	var loans []*models.CommodityLoan
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE commodity_id = $1 ORDER BY lent_at DESC, created_at DESC`,
			r.tableNames.CommodityLoans())
		rows, err := tx.QueryxContext(ctx, query, commodityID)
		if err != nil {
			return errxtrace.Wrap("failed to query commodity loans", err)
		}
		defer rows.Close()
		for rows.Next() {
			var loan models.CommodityLoan
			if err := rows.StructScan(&loan); err != nil {
				return errxtrace.Wrap("failed to scan commodity loan", err)
			}
			l := loan
			loans = append(loans, &l)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity loans for commodity", err)
	}
	return loans, nil
}

// GetOpenForCommodity returns the (at most one) open loan for a commodity
// or registry.ErrNotFound if none exists. ORDER BY lent_at DESC + LIMIT 1
// makes this safe against the rare "two open rows somehow" case — picks
// the most recent, matching the memory backend's tiebreaker.
func (r *CommodityLoanRegistry) GetOpenForCommodity(ctx context.Context, commodityID string) (*models.CommodityLoan, error) {
	var loan models.CommodityLoan
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE commodity_id = $1 AND returned_at IS NULL ORDER BY lent_at DESC LIMIT 1`,
			r.tableNames.CommodityLoans())
		err := tx.GetContext(ctx, &loan, query, commodityID)
		if errors.Is(err, sql.ErrNoRows) {
			return registry.ErrNotFound
		}
		if err != nil {
			return errxtrace.Wrap("failed to query open loan", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &loan, nil
}

func (r *CommodityLoanRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.LoanListOptions) ([]*models.CommodityLoan, int, error) {
	state := opts.State
	if state == "" {
		state = registry.LoanStateAll
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	var conditions []string
	var args []any
	switch state {
	case registry.LoanStateOpen:
		conditions = append(conditions, "returned_at IS NULL")
	case registry.LoanStateOverdue:
		conditions = append(conditions, "returned_at IS NULL AND due_back_at IS NOT NULL AND due_back_at < $1")
		args = append(args, now.Format("2006-01-02"))
	case registry.LoanStateReturned:
		conditions = append(conditions, "returned_at IS NOT NULL")
	case registry.LoanStateAll:
		// no filter
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var loans []*models.CommodityLoan
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, r.tableNames.CommodityLoans(), whereClause)
		if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return errxtrace.Wrap("failed to count commodity loans", err)
		}

		dataArgs := append([]any{}, args...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(
			`SELECT * FROM %s %s ORDER BY lent_at DESC, created_at DESC LIMIT $%d OFFSET $%d`,
			r.tableNames.CommodityLoans(), whereClause, len(args)+1, len(args)+2)
		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to query commodity loans", err)
		}
		defer rows.Close()
		for rows.Next() {
			var loan models.CommodityLoan
			if err := rows.StructScan(&loan); err != nil {
				return errxtrace.Wrap("failed to scan commodity loan", err)
			}
			l := loan
			loans = append(loans, &l)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list paginated commodity loans", err)
	}
	return loans, total, nil
}

func (r *CommodityLoanRegistry) CountOpenByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
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
			r.tableNames.CommodityLoans())
		rows, err := tx.QueryxContext(ctx, query, commodityIDs)
		if err != nil {
			return errxtrace.Wrap("failed to query open loan counts", err)
		}
		defer rows.Close()
		for rows.Next() {
			var commodityID string
			var cnt int
			if err := rows.Scan(&commodityID, &cnt); err != nil {
				return errxtrace.Wrap("failed to scan open loan count", err)
			}
			out[commodityID] = cnt
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count open loans by commodity", err)
	}
	return out, nil
}
