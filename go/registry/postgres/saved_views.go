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
	"github.com/google/uuid"
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

// lockSavedViewGroup serializes the writes of a group that read its views
// before changing them: positions, the limits, the rewrite of queries. The
// lock is released at the end of the transaction.
func lockSavedViewGroup(ctx context.Context, tx *sqlx.Tx, groupID string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext('saved_views'), hashtext($1))`, groupID)
	if err != nil {
		return errxtrace.Wrap("failed to acquire saved view advisory lock", err, errx.Attrs("group_id", groupID))
	}
	return nil
}

// nextSortOrder returns one past the highest position of the group.
func (r *SavedViewRegistry) nextSortOrder(ctx context.Context, tx *sqlx.Tx, tenantID, groupID string) (int, error) {
	var next int
	query := fmt.Sprintf(
		`SELECT COALESCE(MAX(sort_order) + 1, 0) FROM %s WHERE tenant_id = $1 AND group_id = $2`,
		r.tableNames.SavedViews())
	if err := tx.GetContext(ctx, &next, query, tenantID, groupID); err != nil {
		return 0, errxtrace.Wrap("failed to read the next saved view position", err)
	}
	return next, nil
}

// tally counts the views of the registry's group for the limits.
func (r *SavedViewRegistry) tally(ctx context.Context, tx *sqlx.Tx) (registry.SavedViewTally, error) {
	var row struct {
		Own    int `db:"own"`
		Shared int `db:"shared"`
	}
	query := fmt.Sprintf(
		`SELECT COUNT(*) FILTER (WHERE created_by_user_id = $3) AS own, COUNT(*) FILTER (WHERE shared) AS shared
		 FROM %s WHERE tenant_id = $1 AND group_id = $2`,
		r.tableNames.SavedViews())
	if err := tx.GetContext(ctx, &row, query, r.tenantID, r.groupID, r.createdByUserID); err != nil {
		return registry.SavedViewTally{}, errxtrace.Wrap("failed to count saved views", err)
	}
	return registry.SavedViewTally{Own: row.Own, Shared: row.Shared}, nil
}

func (r *SavedViewRegistry) Create(ctx context.Context, view models.SavedView) (*models.SavedView, error) {
	now := time.Now()
	view.CreatedAt = now
	view.UpdatedAt = now
	created, err := r.newSQLRegistry().CreateWithPrepare(ctx, view, func(ctx context.Context, tx *sqlx.Tx, v *models.SavedView) error {
		if err := lockSavedViewGroup(ctx, tx, v.GroupID); err != nil {
			return err
		}
		next, err := r.nextSortOrder(ctx, tx, v.TenantID, v.GroupID)
		if err != nil {
			return err
		}
		v.SortOrder = next
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create saved view", err)
	}
	return &created, nil
}

func (r *SavedViewRegistry) CreateChecked(
	ctx context.Context,
	view models.SavedView,
	placement *registry.SavedViewPlacementPatch,
	check func(registry.SavedViewTally) error,
) (*models.SavedView, error) {
	if r.service {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "user"))
	}
	now := time.Now()
	view.ID = uuid.New().String()
	view.UUID = uuid.New().String()
	view.TenantID = r.tenantID
	view.GroupID = r.groupID
	view.CreatedByUserID = r.createdByUserID
	view.CreatedAt = now
	view.UpdatedAt = now
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := lockSavedViewGroup(ctx, tx, r.groupID); err != nil {
			return err
		}
		tally, err := r.tally(ctx, tx)
		if err != nil {
			return err
		}
		if check != nil {
			if err := check(tally); err != nil {
				return err
			}
		}
		if view.SortOrder, err = r.nextSortOrder(ctx, tx, r.tenantID, r.groupID); err != nil {
			return err
		}
		if err := store.NewTxRegistry[models.SavedView](tx, r.tableNames.SavedViews()).Insert(ctx, view); err != nil {
			return errxtrace.Wrap("failed to insert saved view", err)
		}
		if placement != nil {
			if _, err := r.upsertPlacement(ctx, tx, view.ID, *placement); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create saved view", err)
	}
	return &view, nil
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
		next.SortOrder = existing.SortOrder
		view = *next
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update saved view", err)
	}
	return &view, nil
}

func (r *SavedViewRegistry) UpdateFunc(
	ctx context.Context,
	id string,
	mutate func(view *models.SavedView, tally registry.SavedViewTally) error,
) (*models.SavedView, error) {
	if r.service {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "user"))
	}
	var updated models.SavedView
	cond, args := r.visibility(2)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		if err := lockSavedViewGroup(ctx, tx, r.groupID); err != nil {
			return err
		}
		var existing models.SavedView
		query := fmt.Sprintf(`SELECT * FROM %s WHERE id = $1 AND %s FOR UPDATE`, r.tableNames.SavedViews(), cond)
		if err := tx.GetContext(ctx, &existing, query, append([]any{id}, args...)...); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return registry.ErrNotFound
			}
			return errxtrace.Wrap("failed to read saved view", err)
		}
		tally, err := r.tally(ctx, tx)
		if err != nil {
			return err
		}
		next := existing
		if err := mutate(&next, tally); err != nil {
			return err
		}
		next.EntityID = existing.EntityID
		next.TenantID = existing.TenantID
		next.GroupID = existing.GroupID
		next.CreatedByUserID = existing.CreatedByUserID
		next.SortOrder = existing.SortOrder
		next.CreatedAt = existing.CreatedAt
		next.UpdatedAt = time.Now()
		if err := store.NewTxRegistry[models.SavedView](tx, r.tableNames.SavedViews()).UpdateByField(ctx, store.Pair("id", id), next); err != nil {
			return errxtrace.Wrap("failed to update saved view", err)
		}
		if existing.Shared && !next.Shared {
			// A personal view is the creator's alone again: nobody else
			// keeps it in their sidebar or on their dashboard.
			unpin := fmt.Sprintf(`DELETE FROM %s WHERE saved_view_id = $1 AND user_id <> $2`, r.tableNames.SavedViewPlacements())
			if _, err := tx.ExecContext(ctx, unpin, id, next.CreatedByUserID); err != nil {
				return errxtrace.Wrap("failed to remove saved view placements", err)
			}
		}
		updated = next
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to update saved view", err)
	}
	return &updated, nil
}

// upsertPlacement writes the registry user's placement of viewID. A nil
// field of patch keeps the stored value, or false on insert.
func (r *SavedViewRegistry) upsertPlacement(
	ctx context.Context,
	tx *sqlx.Tx,
	viewID string,
	patch registry.SavedViewPlacementPatch,
) (*models.SavedViewPlacement, error) {
	now := time.Now()
	var placement models.SavedViewPlacement
	// uuid is left to the column default, as in the other upserts.
	query := fmt.Sprintf(
		`INSERT INTO %[1]s (id, tenant_id, group_id, saved_view_id, user_id, show_in_sidebar, show_on_dashboard, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, COALESCE($6, FALSE), COALESCE($7, FALSE), $8, $8)
		 ON CONFLICT (saved_view_id, user_id) DO UPDATE SET
		   show_in_sidebar = COALESCE($6, %[1]s.show_in_sidebar),
		   show_on_dashboard = COALESCE($7, %[1]s.show_on_dashboard),
		   updated_at = $8
		 RETURNING *`,
		r.tableNames.SavedViewPlacements())
	err := tx.QueryRowxContext(ctx, query,
		uuid.New().String(), r.tenantID, r.groupID, viewID, r.createdByUserID,
		patch.ShowInSidebar, patch.ShowOnDashboard, now,
	).StructScan(&placement)
	if err != nil {
		return nil, errxtrace.Wrap("failed to store saved view placement", err)
	}
	return &placement, nil
}

func (r *SavedViewRegistry) Placements(ctx context.Context) (map[string]models.SavedViewPlacement, error) {
	if r.service {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "user"))
	}
	var rows []models.SavedViewPlacement
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT p.* FROM %s p JOIN %s v ON v.id = p.saved_view_id
			 WHERE p.user_id = $1 AND (v.shared OR v.created_by_user_id = $1)`,
			r.tableNames.SavedViewPlacements(), r.tableNames.SavedViews())
		return tx.SelectContext(ctx, &rows, query, r.createdByUserID)
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list saved view placements", err)
	}
	out := make(map[string]models.SavedViewPlacement, len(rows))
	for _, p := range rows {
		out[p.SavedViewID] = p
	}
	return out, nil
}

func (r *SavedViewRegistry) SetPlacement(
	ctx context.Context,
	viewID string,
	patch registry.SavedViewPlacementPatch,
) (*models.SavedViewPlacement, error) {
	if r.service {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field", "user"))
	}
	var placement *models.SavedViewPlacement
	cond, args := r.visibility(2)
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		// FOR SHARE holds the view row until the placement is stored: an
		// unshare or a delete that comes in between waits for this
		// transaction, and one that came first leaves no visible row.
		var one int
		query := fmt.Sprintf(`SELECT 1 FROM %s WHERE id = $1 AND %s FOR SHARE`, r.tableNames.SavedViews(), cond)
		if err := tx.GetContext(ctx, &one, query, append([]any{viewID}, args...)...); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return registry.ErrNotFound
			}
			return errxtrace.Wrap("failed to read saved view", err)
		}
		var err error
		placement, err = r.upsertPlacement(ctx, tx, viewID, patch)
		if isForeignKeyViolation(err) {
			return errxtrace.Classify(registry.ErrNotFound, errx.Attrs("saved_view_id", viewID))
		}
		return err
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to store saved view placement", err)
	}
	return placement, nil
}

// isForeignKeyViolation reports whether err is PostgreSQL's
// foreign_key_violation (23503).
func isForeignKeyViolation(err error) bool {
	type sqlStater interface{ SQLState() string }
	var s sqlStater
	return errors.As(err, &s) && s.SQLState() == "23503"
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
