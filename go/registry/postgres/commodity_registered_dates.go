package postgres

import (
	"context"
	"fmt"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
)

// registeredDateBatchSize bounds how many rows one BackfillRegisteredDates
// statement dates, so a large catalogue is dated in short transactions.
const registeredDateBatchSize = 500

// BackfillRegisteredDates dates, in batches, the commodities that have no
// registered date and whose history holds a "created" event. A batch locks
// its rows with SKIP LOCKED, so replicas running the backfill at once split
// the work, and it never waits on, or deadlocks with, a request or a
// currency migration holding a row. See registry.CommodityRegistry for the
// contract.
func (r *CommodityRegistry) BackfillRegisteredDates(ctx context.Context) (int, error) {
	commodities := string(r.tableNames.Commodities())
	events := string(r.tableNames.CommodityEvents())
	// The day is taken in UTC, as Create takes it (models.NewDate).
	query := fmt.Sprintf(`
		WITH batch AS (
			SELECT c.id FROM %[1]s c
			WHERE (c.registered_date IS NULL OR c.registered_date = '')
			  AND EXISTS (SELECT 1 FROM %[2]s e WHERE e.commodity_id = c.id AND e.kind = $2)
			ORDER BY c.id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE %[1]s c
		SET registered_date = to_char(
			(SELECT MIN(e.occurred_at) FROM %[2]s e WHERE e.commodity_id = c.id AND e.kind = $2) AT TIME ZONE 'UTC',
			'YYYY-MM-DD')
		FROM batch
		WHERE c.id = batch.id`, commodities, events)

	reg := r.newSQLRegistry()
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		var dated int64
		err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			res, err := tx.ExecContext(ctx, query, registeredDateBatchSize, string(models.CommodityEventKindCreated))
			if err != nil {
				return err
			}
			dated, err = res.RowsAffected()
			return err
		})
		if err != nil {
			return total, errxtrace.Wrap("failed to backfill registered dates", err)
		}
		total += int(dated)
		if dated == 0 {
			return total, nil
		}
	}
}
