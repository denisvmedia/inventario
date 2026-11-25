package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
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

// SavedViewRegistryFactory creates SavedViewRegistry instances with proper
// context.
type SavedViewRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// SavedViewRegistry is the postgres registry of saved_views. RLS limits a
// user registry to the rows of its tenant and group; the visibility rule of
// registry.SavedViewRegistry (own views plus shared ones) is applied in the
// queries below, because the group transaction carries no user context.
type SavedViewRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.SavedViewRegistry        = (*SavedViewRegistry)(nil)
	_ registry.SavedViewRegistryFactory = (*SavedViewRegistryFactory)(nil)
)

func NewSavedViewRegistry(dbx *sqlx.DB) *SavedViewRegistryFactory {
	return NewSavedViewRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewSavedViewRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *SavedViewRegistryFactory {
	return &SavedViewRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *SavedViewRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.SavedViewRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *SavedViewRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.SavedViewRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &SavedViewRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *SavedViewRegistryFactory) CreateServiceRegistry() registry.SavedViewRegistry {
	return &SavedViewRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *SavedViewRegistry) newSQLRegistry() *store.RLSGroupRepository[models.SavedView, *models.SavedView] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.SavedView](r.dbx, r.tableNames.SavedViews())
	}
	return store.NewGroupAwareSQLRegistry[models.SavedView](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.SavedViews())
}

// visibility returns the WHERE fragment that applies the visibility rule and
// its argument, numbered from argN. A service registry sees every row.
func (r *SavedViewRegistry) visibility(argN int) (string, []any) {
	if r.service {
		return "TRUE", nil
	}
	return fmt.Sprintf("(shared OR created_by_user_id = $%d)", argN), []any{r.createdByUserID}
}

func (r *SavedViewRegistry) visible(v *models.SavedView) bool {
	return r.service || v.VisibleTo(r.createdByUserID)
}

// lockGroup serializes the writes that read positions of a group before
// changing them. The lock is released at the end of the transaction.
func lockSavedViewGroup(ctx context.Context, tx *sqlx.Tx, groupID string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('saved_views'), hashtext($1))`, groupID)
	if err != nil {
		return errxtrace.Wrap("failed to acquire saved view advisory lock", err, errx.Attrs("group_id", groupID))
	}
	return nil
}

func (r *SavedViewRegistry) Create(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
	now := time.Now()
	view.CreatedAt = now
	view.UpdatedAt = now
	created, err := r.newSQLRegistry().CreateWithPrepare(ctx, view, func(ctx context.Context, tx *sqlx.Tx, v *models.SavedView) error {
		if err := lockSavedViewGroup(ctx, tx, v.GroupID); err != nil {
			return err
		}
		query := fmt.Sprintf(
			`SELECT COALESCE(MAX(sort_order) + 1, 0) FROM %s WHERE tenant_id = $1 AND group_id = $2`,
			r.tableNames.SavedViews())
		if err := tx.GetContext(ctx, &v.SortOrder, query, v.TenantID, v.GroupID); err != nil {
			return errxtrace.Wrap("failed to read the next saved view position", err)
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create saved view", err)
	}
	return &created, nil
}

func (r *SavedViewRegistry) Get(ctx context.Context, id string) (*models.SavedView, error) {
	var view models.SavedView
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &view); err != nil {
		return nil, errxtrace.Wrap("failed to get saved view", err)
	}
	if !r.visible(&view) {
		return nil, errxtrace.Wrap("failed to get saved view", registry.ErrNotFound)
	}
	return &view, nil
}

// List returns the visible views ordered by position, then name without
// regard to case, then id.
func (r *SavedViewRegistry) List(ctx context.Context) ([]*models.SavedView, error) {
	var views []*models.SavedView
	cond, args := r.visibility(1)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE %s ORDER BY sort_order ASC, lower(name) ASC, id ASC`,
			r.tableNames.SavedViews(), cond)
		return tx.SelectContext(ctx, &views, query, args...)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list saved views", err)
	}
	return views, nil
}

func (r *SavedViewRegistry) Count(ctx context.Context) (int, error) {
	var n int
	cond, args := r.visibility(1)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s`, r.tableNames.SavedViews(), cond)
		return tx.GetContext(ctx, &n, query, args...)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to count saved views", err)
	}
	return n, nil
}

func (r *SavedViewRegistry) Update(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
	view.UpdatedAt = time.Now()
	err := r.newSQLRegistry().UpdateWithPrepare(ctx, view, func(_ context.Context, _ *sqlx.Tx, existing models.SavedView, next *models.SavedView) error {
		if !r.visible(&existing) {
			return registry.ErrNotFound
		}
		next.CreatedAt = existing.CreatedAt
		view = *next
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update saved view", err)
	}
	return &view, nil
}

func (r *SavedViewRegistry) Delete(ctx context.Context, id string) error {
	cond, args := r.visibility(2)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1 AND %s`, r.tableNames.SavedViews(), cond)
		res, err := tx.ExecContext(ctx, query, append([]any{id}, args...)...)
		if err != nil {
			return errxtrace.Wrap("failed to delete saved view", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return errxtrace.Wrap("failed to read rows affected", err)
		}
		if n == 0 {
			return registry.ErrNotFound
		}
		return nil
	})
	if err != nil {
		return errxtrace.Wrap("failed to delete saved view", err)
	}
	return nil
}

func (r *SavedViewRegistry) Reorder(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	cond, args := r.visibility(2)
	return r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if !r.service {
			if err := lockSavedViewGroup(ctx, tx, r.groupID); err != nil {
				return err
			}
		}
		selectSQL := fmt.Sprintf(`SELECT sort_order FROM %s WHERE id = $1 AND %s`, r.tableNames.SavedViews(), cond)
		seen := make(map[string]struct{}, len(ids))
		current := make([]int, 0, len(ids))
		for _, id := range ids {
			if _, dup := seen[id]; dup {
				return registry.ErrNotFound
			}
			seen[id] = struct{}{}
			var pos int
			if err := tx.GetContext(ctx, &pos, selectSQL, append([]any{id}, args...)...); err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return registry.ErrNotFound
				}
				return errxtrace.Wrap("failed to read saved view position", err)
			}
			current = append(current, pos)
		}
		sorted := slices.Clone(current)
		slices.Sort(sorted)
		now := time.Now()
		updateSQL := fmt.Sprintf(`UPDATE %s SET sort_order = $1, updated_at = $2 WHERE id = $3`, r.tableNames.SavedViews())
		for i, id := range ids {
			if current[i] == sorted[i] {
				continue
			}
			if _, err := tx.ExecContext(ctx, updateSQL, sorted[i], now, id); err != nil {
				return errxtrace.Wrap("failed to update saved view position", err)
			}
		}
		return nil
	})
}

func (r *SavedViewRegistry) RewriteQueries(ctx context.Context, rewrite func(view models.SavedView) (string, bool)) (int, error) {
	changed := 0
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if !r.service {
			if err := lockSavedViewGroup(ctx, tx, r.groupID); err != nil {
				return err
			}
		}
		var views []models.SavedView
		// Every view of the group: RLS keeps a user registry to its group,
		// and the personal views of other members have to follow too.
		query := fmt.Sprintf(`SELECT * FROM %s ORDER BY id`, r.tableNames.SavedViews())
		if err := tx.SelectContext(ctx, &views, query); err != nil {
			return errxtrace.Wrap("failed to list saved views", err)
		}
		now := time.Now()
		updateSQL := fmt.Sprintf(`UPDATE %s SET query = $1, updated_at = $2 WHERE id = $3`, r.tableNames.SavedViews())
		for _, v := range views {
			next, ok := rewrite(v)
			if !ok {
				continue
			}
			if _, err := tx.ExecContext(ctx, updateSQL, next, now, v.ID); err != nil {
				return errxtrace.Wrap("failed to rewrite saved view query", err)
			}
			changed++
		}
		return nil
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to rewrite saved view queries", err)
	}
	return changed, nil
}
