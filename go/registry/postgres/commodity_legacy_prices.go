package postgres

import (
	"context"
	"fmt"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"
)

// legacyPriceBatchSize bounds how many rows one BackfillLegacyPrices
// statement rewrites, so a large catalogue is rewritten in short
// transactions.
const legacyPriceBatchSize = 500

// BackfillLegacyPrices rewrites the rows flagged legacy_zero_prices in
// batches until none is left. A batch locks its rows with SKIP LOCKED, so
// replicas running the backfill at once split the work instead of waiting on
// each other, and a row a request is updating right now is left to that
// request, whose write clears the flag itself. See registry.CommodityRegistry
// for the contract.
func (r *CommodityRegistry) BackfillLegacyPrices(ctx context.Context) (int, error) {
	commodities := string(r.tableNames.Commodities())
	query := fmt.Sprintf(`
		WITH batch AS (
			SELECT id FROM %[1]s
			WHERE legacy_zero_prices
			ORDER BY id
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		UPDATE %[1]s c
		SET current_price = NULLIF(c.current_price, 0),
		    converted_original_price = NULLIF(c.converted_original_price, 0),
		    legacy_zero_prices = false,
		    revision = c.revision + 1
		FROM batch
		WHERE c.id = batch.id`, commodities)

	reg := r.newSQLRegistry()
	total := 0
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		var rewritten int64
		err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			res, err := tx.ExecContext(ctx, query, legacyPriceBatchSize)
			if err != nil {
				return err
			}
			rewritten, err = res.RowsAffected()
			return err
		})
		if err != nil {
			return total, errxtrace.Wrap("failed to backfill legacy prices", err)
		}
		total += int(rewritten)
		if rewritten == 0 {
			return total, nil
		}
	}
}
