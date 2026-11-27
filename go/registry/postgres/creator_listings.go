package postgres

import (
	"context"
	"fmt"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// The creator listings (#2934) read what List reads, with the creator as
// one more condition of the WHERE clause, so the database returns only the
// user's rows. The group still comes from the transaction's RLS context.

// createdByCondition is the condition a creator listing adds for table;
// userID is its first argument.
func createdByCondition(table store.TableName) string {
	return fmt.Sprintf("%s.created_by_user_id = $1", table)
}

// txRunner runs fn in a transaction with a registry's RLS context: the Do
// method of the registry's SQL repository.
type txRunner func(ctx context.Context, fn func(context.Context, *sqlx.Tx) error) error

// listCreatedBy runs SELECT * FROM table WHERE where AND the creator
// condition in a transaction of do.
func listCreatedBy[T any](ctx context.Context, do txRunner, table store.TableName, where, userID string) ([]*T, error) {
	conditions := createdByCondition(table)
	if where != "" {
		conditions = where + " AND " + conditions
	}
	var out []*T
	err := do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		var err error
		out, err = scanAll[T](ctx, tx, fmt.Sprintf(`SELECT * FROM %s WHERE %s`, table, conditions), userID)
		return err
	})
	return out, err
}

// listRefs reads columns into R, a struct whose db tags name them, from
// every row of table in a transaction of do.
func listRefs[R any](ctx context.Context, do txRunner, columns string, table store.TableName) ([]R, error) {
	var out []R
	err := do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		rows, err := scanAll[R](ctx, tx, fmt.Sprintf(`SELECT %s FROM %s`, columns, table))
		if err != nil {
			return err
		}
		out = make([]R, 0, len(rows))
		for _, row := range rows {
			out = append(out, *row)
		}
		return nil
	})
	return out, err
}

const entityRefColumns = "id, uuid, name"

// ListCreatedBy returns the user's live items with their custom field values.
func (r *CommodityRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.Commodity, error) {
	out, err := r.listCommoditiesCreatedBy(ctx, liveRow, userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's commodities", err)
	}
	return out, nil
}

// ListTrashedCreatedBy returns the user's items in Trash with their custom
// field values.
func (r *CommodityRegistry) ListTrashedCreatedBy(ctx context.Context, userID string) ([]*models.Commodity, error) {
	out, err := r.listCommoditiesCreatedBy(ctx, trashedRow, userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's trashed commodities", err)
	}
	return out, nil
}

func (r *CommodityRegistry) listCommoditiesCreatedBy(ctx context.Context, state, userID string) ([]*models.Commodity, error) {
	table := r.tableNames.Commodities()
	var out []*models.Commodity
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE %s AND %s`, table, state, createdByCondition(table))
		var err error
		out, err = scanAll[models.Commodity](ctx, tx, query, userID)
		if err != nil {
			return err
		}
		return loadCustomFieldValuesInTx(ctx, tx, r.tableNames, out...)
	})
	return out, err
}

// ListRefs names every item of the group, live or in Trash.
func (r *CommodityRegistry) ListRefs(ctx context.Context) ([]registry.EntityRef, error) {
	out, err := listRefs[registry.EntityRef](ctx, r.newSQLRegistry().Do, entityRefColumns, r.tableNames.Commodities())
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity names", err)
	}
	return out, nil
}

// ListCreatedBy returns the user's live files with their links.
func (r *FileRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.FileEntity, error) {
	out, err := r.listFilesCreatedBy(ctx, liveRow, userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's files", err)
	}
	return out, nil
}

// ListTrashedCreatedBy returns the user's files in Trash, those deleted with
// their item included, with their links.
func (r *FileRegistry) ListTrashedCreatedBy(ctx context.Context, userID string) ([]*models.FileEntity, error) {
	out, err := r.listFilesCreatedBy(ctx, trashedRow, userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's trashed files", err)
	}
	return out, nil
}

func (r *FileRegistry) listFilesCreatedBy(ctx context.Context, state, userID string) ([]*models.FileEntity, error) {
	table := r.tableNames.Files()
	var out []*models.FileEntity
	err := r.newSQLRegistry().Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT * FROM %s WHERE %s AND %s`, table, state, createdByCondition(table))
		var err error
		out, err = r.queryFiles(ctx, tx, query, userID)
		return err
	})
	return out, err
}

// ListCreatedBy returns the user's loans, those of items in Trash included.
func (r *CommodityLoanRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityLoan, error) {
	out, err := listCreatedBy[models.CommodityLoan](ctx, r.newSQLRegistry().Do, r.tableNames.CommodityLoans(), "", userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's commodity loans", err)
	}
	return out, nil
}

// ListCreatedBy returns the user's maintenance schedules, those of items in
// Trash included.
func (r *MaintenanceScheduleRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.MaintenanceSchedule, error) {
	out, err := listCreatedBy[models.MaintenanceSchedule](ctx, r.newSQLRegistry().Do, r.tableNames.MaintenanceSchedules(), "", userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's maintenance schedules", err)
	}
	return out, nil
}

// ListCreatedBy returns the user's service records, those of items in Trash
// included.
func (r *CommodityServiceRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityService, error) {
	out, err := listCreatedBy[models.CommodityService](ctx, r.newSQLRegistry().Do, r.tableNames.CommodityServices(), "", userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's commodity services", err)
	}
	return out, nil
}

// ListCreatedBy returns the user's reminders.
func (r *CommodityReminderRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityReminder, error) {
	out, err := listCreatedBy[models.CommodityReminder](ctx, r.newSQLRegistry().Do, r.tableNames.CommodityReminders(), "", userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's commodity reminders", err)
	}
	return out, nil
}

// ListCreatedBy returns the user's supply links.
func (r *SupplyLinkRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.SupplyLink, error) {
	out, err := listCreatedBy[models.SupplyLink](ctx, r.newSQLRegistry().Do, r.tableNames.CommoditySupplyLinks(), "", userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's supply links", err)
	}
	return out, nil
}

// ListCreatedBy returns the user's drafts.
func (r *CommodityDraftRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityDraft, error) {
	out, err := listCreatedBy[models.CommodityDraft](ctx, r.newSQLRegistry().Do, r.tableNames.CommodityDrafts(), "", userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's commodity drafts", err)
	}
	return out, nil
}

// ListRefs gives the id and the UUID of every draft of the group.
func (r *CommodityDraftRegistry) ListRefs(ctx context.Context) ([]registry.EntityRef, error) {
	out, err := listRefs[registry.EntityRef](ctx, r.newSQLRegistry().Do, "id, uuid, '' AS name", r.tableNames.CommodityDrafts())
	if err != nil {
		return nil, errxtrace.Wrap("failed to list commodity draft ids", err)
	}
	return out, nil
}

// ListCreatedBy returns the history entries the user's actions wrote.
func (r *CommodityEventRegistry) ListCreatedBy(ctx context.Context, userID string) ([]*models.CommodityEvent, error) {
	out, err := listCreatedBy[models.CommodityEvent](ctx, r.newSQLRegistry().Do, r.tableNames.CommodityEvents(), "", userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list the user's commodity events", err)
	}
	return out, nil
}

// ListRefs names every area of the group with its location.
func (r *AreaRegistry) ListRefs(ctx context.Context) ([]registry.AreaRef, error) {
	out, err := listRefs[registry.AreaRef](ctx, r.newSQLRegistry().Do, entityRefColumns+", location_id", r.tableNames.Areas())
	if err != nil {
		return nil, errxtrace.Wrap("failed to list area names", err)
	}
	return out, nil
}

// ListRefs names every location of the group.
func (r *LocationRegistry) ListRefs(ctx context.Context) ([]registry.EntityRef, error) {
	out, err := listRefs[registry.EntityRef](ctx, r.newSQLRegistry().Do, entityRefColumns, r.tableNames.Locations())
	if err != nil {
		return nil, errxtrace.Wrap("failed to list location names", err)
	}
	return out, nil
}
