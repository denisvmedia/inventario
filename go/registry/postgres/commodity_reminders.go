package postgres

import (
	"context"
	"fmt"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/go-extras/go-kit/must"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/appctx"
	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// CommodityReminderRegistryFactory creates CommodityReminderRegistry
// instances with proper context (#2805).
type CommodityReminderRegistryFactory struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

// CommodityReminderRegistry is the postgres-backed group-scoped registry of
// item reminders.
type CommodityReminderRegistry struct {
	dbx             *sqlx.DB
	tableNames      store.TableNames
	tenantID        string
	groupID         string
	createdByUserID string
	service         bool
}

var (
	_ registry.CommodityReminderRegistry        = (*CommodityReminderRegistry)(nil)
	_ registry.CommodityReminderRegistryFactory = (*CommodityReminderRegistryFactory)(nil)
)

func NewCommodityReminderRegistry(dbx *sqlx.DB) *CommodityReminderRegistryFactory {
	return NewCommodityReminderRegistryWithTableNames(dbx, store.DefaultTableNames)
}

func NewCommodityReminderRegistryWithTableNames(dbx *sqlx.DB, tableNames store.TableNames) *CommodityReminderRegistryFactory {
	return &CommodityReminderRegistryFactory{dbx: dbx, tableNames: tableNames}
}

func (f *CommodityReminderRegistryFactory) MustCreateUserRegistry(ctx context.Context) registry.CommodityReminderRegistry {
	return must.Must(f.CreateUserRegistry(ctx))
}

func (f *CommodityReminderRegistryFactory) CreateUserRegistry(ctx context.Context) (registry.CommodityReminderRegistry, error) {
	user, err := appctx.RequireUserFromContext(ctx)
	if err != nil {
		return nil, errxtrace.Wrap("failed to get user from context", err)
	}
	return &CommodityReminderRegistry{
		dbx:             f.dbx,
		tableNames:      f.tableNames,
		tenantID:        user.TenantID,
		groupID:         appctx.GroupIDFromContext(ctx),
		createdByUserID: user.ID,
	}, nil
}

func (f *CommodityReminderRegistryFactory) CreateServiceRegistry() registry.CommodityReminderRegistry {
	return &CommodityReminderRegistry{
		dbx:        f.dbx,
		tableNames: f.tableNames,
		service:    true,
	}
}

func (r *CommodityReminderRegistry) newSQLRegistry() *store.RLSGroupRepository[models.CommodityReminder, *models.CommodityReminder] {
	if r.service {
		return store.NewGroupServiceSQLRegistry[models.CommodityReminder](r.dbx, r.tableNames.CommodityReminders())
	}
	return store.NewGroupAwareSQLRegistry[models.CommodityReminder](r.dbx, r.tenantID, r.groupID, r.createdByUserID, r.tableNames.CommodityReminders())
}

func (r *CommodityReminderRegistry) Get(ctx context.Context, id string) (*models.CommodityReminder, error) {
	var reminder models.CommodityReminder
	if err := r.newSQLRegistry().ScanOneByField(ctx, store.Pair("id", id), &reminder); err != nil {
		return nil, errxtrace.Wrap("failed to get commodity reminder", err)
	}
	return &reminder, nil
}

func (r *CommodityReminderRegistry) List(ctx context.Context) ([]*models.CommodityReminder, error) {
	var reminders []*models.CommodityReminder
	for reminder, err := range r.newSQLRegistry().Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list commodity reminders", err)
		}
		reminders = append(reminders, new(reminder))
	}
	return reminders, nil
}

func (r *CommodityReminderRegistry) Count(ctx context.Context) (int, error) {
	cnt, err := r.newSQLRegistry().Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count commodity reminders", err)
	}
	return cnt, nil
}

func (r *CommodityReminderRegistry) Create(ctx context.Context, reminder models.CommodityReminder) (*models.CommodityReminder, error) {
	now := time.Now()
	reminder.CreatedAt = now
	reminder.UpdatedAt = now
	created, err := r.newSQLRegistry().Create(ctx, reminder, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create commodity reminder", err)
	}
	return &created, nil
}

func (r *CommodityReminderRegistry) Update(ctx context.Context, reminder models.CommodityReminder) (*models.CommodityReminder, error) {
	reminder.UpdatedAt = time.Now()
	if err := r.newSQLRegistry().Update(ctx, reminder, nil); err != nil {
		return nil, errxtrace.Wrap("failed to update commodity reminder", err)
	}
	return &reminder, nil
}

func (r *CommodityReminderRegistry) Delete(ctx context.Context, id string) error {
	return r.newSQLRegistry().Delete(ctx, id, nil)
}

func (r *CommodityReminderRegistry) ListByCommodity(ctx context.Context, commodityID string) ([]*models.CommodityReminder, error) {
	var reminders []*models.CommodityReminder
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(
			`SELECT * FROM %s WHERE commodity_id = $1 ORDER BY due_on ASC, title ASC`,
			r.tableNames.CommodityReminders())
		rows, err := tx.QueryxContext(ctx, query, commodityID)
		if err != nil {
			return errxtrace.Wrap("failed to query commodity reminders", err)
		}
		defer rows.Close()
		for rows.Next() {
			var reminder models.CommodityReminder
			if err := rows.StructScan(&reminder); err != nil {
				return errxtrace.Wrap("failed to scan commodity reminder", err)
			}
			reminders = append(reminders, new(reminder))
		}
		return rows.Err()
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity reminders for commodity", err)
	}
	return reminders, nil
}
