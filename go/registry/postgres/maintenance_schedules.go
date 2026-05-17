package postgres

import (
	"context"
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

// MaintenanceScheduleRegistryFactory creates
// MaintenanceScheduleRegistry instances with proper context (#1368).
type MaintenanceScheduleRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// MaintenanceScheduleRegistry is the postgres-backed group-scoped
// registry of maintenance schedules.
type MaintenanceScheduleRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.MaintenanceScheduleRegistry        = (*MaintenanceScheduleRegistry)(nil)
	_ registry.MaintenanceScheduleRegistryFactory = (*MaintenanceScheduleRegistryFactory)(nil)
)

func NewMaintenanceScheduleRegistry(dbx *sqlx.DB) *MaintenanceScheduleRegistryFactory {
	return NewMaintenanceScheduleRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewMaintenanceScheduleRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *MaintenanceScheduleRegistryFactory {
	return &MaintenanceScheduleRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *MaintenanceScheduleRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.MaintenanceScheduleRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *MaintenanceScheduleRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.MaintenanceScheduleRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &MaintenanceScheduleRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
		service:         false,
	}, nil
}

func (f *MaintenanceScheduleRegistryFactory) CreateServiceRegistry() registry.MaintenanceScheduleRegistry {
	return &MaintenanceScheduleRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *MaintenanceScheduleRegistry) newSQLRegistry() *store.RLSGroupRepository[models.MaintenanceSchedule, *models.MaintenanceSchedule] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.MaintenanceSchedule](r.dbx, r.tableNames.MaintenanceSchedules())
	}
	return store.NewGroupAwareSQLRegistry[models.MaintenanceSchedule](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.MaintenanceSchedules())
}

func (r *MaintenanceScheduleRegistry) Get(ctx context.Context, id string) (*models.MaintenanceSchedule, error) {
	var schedule models.MaintenanceSchedule
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &schedule); err != nil {
		return nil, errxtrace.Wrap("failed to get maintenance schedule", err)
	}
	return &schedule, nil
}

func (r *MaintenanceScheduleRegistry) List(ctx context.Context) ([]*models.MaintenanceSchedule, error) {
	var schedules []*models.MaintenanceSchedule
	for schedule, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list maintenance schedules", err)
		}
		s := schedule
		schedules = append(schedules, &s)
	}
	return schedules, nil
}

func (r *MaintenanceScheduleRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count maintenance schedules", err)
	}
	return cnt, nil
}

func (r *MaintenanceScheduleRegistry) Create(ctx context.Context, schedule models.MaintenanceSchedule) (*models.MaintenanceSchedule, error) {
	now := time.Now()
	schedule.CreatedAt = now
	schedule.UpdatedAt = now
	created, err := r.newSQLRegistry().Create(ctx, schedule, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create maintenance schedule", err)
	}
	return &created, nil
}

func (r *MaintenanceScheduleRegistry) Update(ctx context.Context, schedule models.MaintenanceSchedule) (*models.MaintenanceSchedule, error) {
	schedule.UpdatedAt = time.Now()
	if err := r.newSQLRegistry().Update(ctx, schedule, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update maintenance schedule", err)
	}
	return &schedule, nil
}

func (r *MaintenanceScheduleRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

func (r *MaintenanceScheduleRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.MaintenanceSchedule, error) {
	var schedules []*models.MaintenanceSchedule
	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE commodity_id = $1 ORDER BY next_due_at ASC, title ASC`,
			r.tableNames.MaintenanceSchedules())
		rows, err := tx.QueryxContext(ctx, query, commodityID)
		if err != nil {
			return errxtrace.Wrap("failed to query maintenance schedules", err)
		}
		defer rows.Close()
		for rows.Next() {
			var schedule models.MaintenanceSchedule
			if err := rows.StructScan(&schedule); err != nil {
				return errxtrace.Wrap("failed to scan maintenance schedule", err)
			}
			s := schedule
			schedules = append(schedules, &s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list maintenance schedules for commodity", err)
	}
	return schedules, nil
}

func (r *MaintenanceScheduleRegistry) ListPaginated(ctx context.Context, offset, limit int, opts registry.MaintenanceListOptions) ([]*models.MaintenanceSchedule, int, error) {
	var conditions []string
	var args []any
	if opts.EnabledOnly {
		conditions = append(conditions, "enabled = true")
	}
	if opts.DueBefore != "" {
		conditions = append(conditions, fmt.Sprintf("next_due_at <= $%d", len(args)+1))
		args = append(args, opts.DueBefore)
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var schedules []*models.MaintenanceSchedule
	var total int

	reg := r.newSQLRegistry()
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, r.tableNames.MaintenanceSchedules(), whereClause)
		if err := tx.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
			return errxtrace.Wrap("failed to count maintenance schedules", err)
		}

		dataArgs := append([]any{}, args...)
		dataArgs = append(dataArgs, limit, offset)
		dataQuery := fmt.Sprintf(
			`SELECT * FROM %s %s ORDER BY next_due_at ASC, title ASC LIMIT $%d OFFSET $%d`,
			r.tableNames.MaintenanceSchedules(), whereClause, len(args)+1, len(args)+2)
		rows, err := tx.QueryxContext(ctx, dataQuery, dataArgs...)
		if err != nil {
			return errxtrace.Wrap("failed to query maintenance schedules", err)
		}
		defer rows.Close()
		for rows.Next() {
			var schedule models.MaintenanceSchedule
			if err := rows.StructScan(&schedule); err != nil {
				return errxtrace.Wrap("failed to scan maintenance schedule", err)
			}
			s := schedule
			schedules = append(schedules, &s)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, 0, errxtrace.Wrap("failed to list paginated maintenance schedules", err)
	}
	return schedules, total, nil
}

func (r *MaintenanceScheduleRegistry) CountByCommodity(ctx context.Context, commodityIDs []string) (map[string]int, error) {
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
			r.tableNames.MaintenanceSchedules())
		rows, err := tx.QueryxContext(ctx, query, commodityIDs)
		if err != nil {
			return errxtrace.Wrap("failed to query maintenance counts", err)
		}
		defer rows.Close()
		for rows.Next() {
			var commodityID string
			var cnt int
			if err := rows.Scan(&commodityID, &cnt); err != nil {
				return errxtrace.Wrap("failed to scan maintenance count", err)
			}
			out[commodityID] = cnt
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count maintenance schedules by commodity", err)
	}
	return out, nil
}
