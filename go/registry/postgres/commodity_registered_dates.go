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
// currency migration holding a row. An empty batch therefore does not prove
// that the work is done: a plain read, which takes no row lock and waits on
// none, then tells whether a skipped row is still undated (pending). See
// registry.CommodityRegistry for the contract.
//
// The lock is FOR NO KEY UPDATE, as registered_date is no key column. It
// does not conflict with the FOR KEY SHARE lock an insert into
// commodity_events takes on the item, so such an insert makes the batch skip
// no row.
func (r *CommodityRegistry) BackfillRegisteredDates(ctx context.Context) (dated int, pending bool, err error) {
	commodities := string(r.tableNames.Commodities())
	events := string(r.tableNames.CommodityEvents())
	undated := fmt.Sprintf(`
		(c.registered_date IS NULL OR c.registered_date = '')
		AND EXISTS (SELECT 1 FROM %[1]s e WHERE e.commodity_id = c.id AND e.kind = $1)`, events)
	// The day is taken in UTC, as Create takes it (models.NewDate).
	batchQuery := fmt.Sprintf(`
		WITH batch AS (
			SELECT c.id FROM %[1]s c
			WHERE %[3]s
			ORDER BY c.id
			LIMIT $2
			FOR NO KEY UPDATE SKIP LOCKED
		)
		UPDATE %[1]s c
		SET registered_date = to_char(
			(SELECT MIN(e.occurred_at) FROM %[2]s e WHERE e.commodity_id = c.id AND e.kind = $1) AT TIME ZONE 'UTC',
			'YYYY-MM-DD')
		FROM batch
		WHERE c.id = batch.id`, commodities, events, undated)
	pendingQuery := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %[1]s c WHERE %[2]s)`, commodities, undated)
	created := string(models.CommodityEventKindCreated)

	reg := r.newSQLRegistry()
	for {
		if err := ctx.Err(); err != nil {
			return dated, false, err
		}
		var batch int64
		err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			res, err := tx.ExecContext(ctx, batchQuery, created, registeredDateBatchSize)
			if err != nil {
				return err
			}
			batch, err = res.RowsAffected()
			return err
		})
		if err != nil {
			return dated, false, errxtrace.Wrap("failed to backfill registered dates", err)
		}
		dated += int(batch)
		if batch > 0 {
			continue
		}
		err = reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			return tx.GetContext(ctx, &pending, pendingQuery, created)
		})
		if err != nil {
			return dated, false, errxtrace.Wrap("failed to check for undated commodities", err)
		}
		return dated, pending, nil
	}
}
