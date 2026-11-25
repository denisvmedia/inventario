package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// sellerNameKeyIndex is the unique index on (group_id, name_key). A unique
// violation on it is a duplicate seller name, reported as
// registry.ErrAlreadyExists.
const sellerNameKeyIndex = "idx_sellers_group_name_key"

// SellerRegistryFactory creates SellerRegistry instances with proper context
// (#2804).
type SellerRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// SellerRegistry is the postgres-backed group-scoped registry of sellers.
type SellerRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.SellerRegistry        = (*SellerRegistry)(nil)
	_ registry.SellerRegistryFactory = (*SellerRegistryFactory)(nil)
)

func NewSellerRegistry(dbx *sqlx.DB) *SellerRegistryFactory {
	return NewSellerRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewSellerRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *SellerRegistryFactory {
	return &SellerRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *SellerRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SellerRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SellerRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SellerRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &SellerRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *SellerRegistryFactory) CreateServiceRegistry() registry.SellerRegistry {
	return &SellerRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *SellerRegistry) newSQLRegistry() *store.RLSGroupRepository[models.Seller, *models.Seller] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.Seller](r.dbx, r.tableNames.Sellers())
	}
	return store.NewGroupAwareSQLRegistry[models.Seller](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.Sellers())
}

func (r *SellerRegistry) Get(ctx context.Context, id string) (*models.Seller, error) {
	var seller models.Seller
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &seller); err != nil {
		return nil, errxtrace.Wrap("failed to get seller", err)
	}
	return &seller, nil
}

func (r *SellerRegistry) GetByName(ctx context.Context, name string) (*models.Seller, error) {
	var seller models.Seller
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("name_key", models.SellerNameKey(name)), &seller); err != nil {
		return nil, errxtrace.Wrap("failed to get seller by name", err)
	}
	return &seller, nil
}

// List returns the sellers sorted by registry.SortSellers.
func (r *SellerRegistry) List(ctx context.Context) ([]*models.Seller, error) {
	var sellers []*models.Seller
	for seller, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list sellers", err)
		}
		sellers = append(sellers, new(seller))
	}
	registry.SortSellers(sellers)
	return sellers, nil
}

func (r *SellerRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count sellers", err)
	}
	return cnt, nil
}

func (r *SellerRegistry) Create(ctx context.Context, seller models.Seller) (*models.Seller, error) {
	seller.Normalize()
	now := time.Now()
	seller.CreatedAt = now
	seller.UpdatedAt = now
	created, err := r.newSQLRegistry().Create(ctx, seller, nil)
	if err != nil {
		if isSellerNameUniqueViolation(err) {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", seller.Name))
		}
		return nil, errxtrace.Wrap("failed to create seller", err)
	}
	return &created, nil
}

func (r *SellerRegistry) Update(ctx context.Context, seller models.Seller) (*models.Seller, error) {
	seller.Normalize()
	existing, err := r.Get(ctx, seller.ID)
	if err != nil {
		return nil, err
	}
	seller.CreatedAt = existing.CreatedAt
	seller.UpdatedAt = time.Now()
	if err := r.newSQLRegistry().Update(ctx, seller, nil); err != nil {
		if isSellerNameUniqueViolation(err) {
			return nil, errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs("name", seller.Name))
		}
		return nil, errxtrace.Wrap("failed to update seller", err)
	}
	return r.Get(ctx, seller.ID)
}

// Delete removes the seller. The foreign key from commodities.seller_id is
// ON DELETE SET NULL, so its items lose the seller in the same statement.
func (r *SellerRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

func (r *SellerRegistry) ItemCounts(ctx context.Context) (map[string]int, error) {
	counts := map[string]int{}
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT seller_id, COUNT(*) FROM %s WHERE seller_id IS NOT NULL AND %s GROUP BY seller_id`,
			r.tableNames.Commodities(), liveRow)
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return errxtrace.Wrap("failed to count items per seller", err)
		}
		defer rows.Close()
		for rows.Next() {
			var (
				id    string
				count int
			)
			if err := rows.Scan(&id, &count); err != nil {
				return errxtrace.Wrap("failed to scan item count", err)
			}
			counts[id] = count
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count items per seller", err)
	}
	return counts, nil
}

// isSellerNameUniqueViolation reports whether err is a unique violation
// (SQLSTATE 23505) on sellerNameKeyIndex. A unique violation on any other
// index is not a duplicate name and is returned as it is.
func isSellerNameUniqueViolation(err error) bool {
	type sqlStater interface{ SQLState() string }
	type constrainter interface{ ConstraintName() string }
	var s sqlStater
	if !errors.As(err, &s) || s.SQLState() != "23505" {
		return false
	}
	var c constrainter
	if errors.As(err, &c) {
		return c.ConstraintName() == sellerNameKeyIndex
	}
	return strings.Contains(err.Error(), sellerNameKeyIndex)
}
