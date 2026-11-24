package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// assetNumberLockKey is the first half of the transaction-scoped advisory
// lock that serializes every write of commodities.asset_number inside one
// group; the second half is the group id. Create and BackfillAssetNumbers
// both take it before they read MAX(asset_number), so two transactions can
// never compute the same next number. The unique index on
// (group_id, asset_number) backs the lock up.
const assetNumberLockKey = "commodity_asset_number"

func lockGroupAssetNumbers(ctx context.Context, tx *sqlx.Tx, groupID string) error {
	_, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))`, assetNumberLockKey, groupID)
	if err != nil {
		return errxtrace.Wrap("failed to acquire asset number lock", err, errx.Attrs("group_id", groupID))
	}
	return nil
}

// assignAssetNumberInTx sets row.AssetNumber inside the insert transaction.
// requested is the archived number a restore asks for (0 for none): it is
// kept when no other commodity of the group holds it. Otherwise the row gets
// the group's highest number plus one. The group lock is held until the
// transaction ends, so the INSERT that follows is part of the same critical
// section.
func assignAssetNumberInTx(ctx context.Context, tx *sqlx.Tx, table store.TableName, row *models.Commodity, requested int) error {
	groupID := row.GetGroupID()
	if groupID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}
	if err := lockGroupAssetNumbers(ctx, tx, groupID); err != nil {
		return err
	}

	if requested > 0 {
		var taken bool
		query := fmt.Sprintf(`SELECT EXISTS (SELECT 1 FROM %s WHERE group_id = $1 AND asset_number = $2)`, table)
		if err := tx.GetContext(ctx, &taken, query, groupID, requested); err != nil {
			return errxtrace.Wrap("failed to check asset number", err, errx.Attrs("group_id", groupID))
		}
		if !taken {
			row.AssetNumber = new(requested)
			return nil
		}
	}

	var next int
	query := fmt.Sprintf(`SELECT COALESCE(MAX(asset_number), 0) + 1 FROM %s WHERE group_id = $1`, table)
	if err := tx.GetContext(ctx, &next, query, groupID); err != nil {
		return errxtrace.Wrap("failed to compute next asset number", err, errx.Attrs("group_id", groupID))
	}
	row.AssetNumber = new(next)
	return nil
}

// keepAssetNumberInTx copies the stored asset number onto the row an Update
// is about to write, so the caller's value never reaches the column. FOR
// UPDATE makes the read wait for a backfill that is numbering this row right
// now, instead of reading NULL and writing it back over the new number.
func keepAssetNumberInTx(ctx context.Context, tx *sqlx.Tx, table store.TableName, row *models.Commodity) error {
	var stored sql.NullInt64
	query := fmt.Sprintf(`SELECT asset_number FROM %s WHERE id = $1 FOR UPDATE`, table)
	if err := tx.GetContext(ctx, &stored, query, row.GetID()); err != nil {
		return errxtrace.Wrap("failed to read asset number", err, errx.Attrs("commodity_id", row.GetID()))
	}
	row.AssetNumber = nil
	if stored.Valid {
		row.AssetNumber = new(int(stored.Int64))
	}
	return nil
}

// BackfillAssetNumbers numbers the commodities that have no asset number, one
// group per transaction. Inside a group the new numbers continue after the
// group's highest one, in creation order: the commodity's earliest history
// event, then its registered date, then its id as the final tie-breaker
// (commodities carry no creation timestamp of their own, and the history
// rail records the "created" event). See registry.CommodityRegistry for the
// contract.
func (r *CommodityRegistry) BackfillAssetNumbers(ctx context.Context) (int, error) {
	commodities := string(r.tableNames.Commodities())
	events := string(r.tableNames.CommodityEvents())
	reg := r.newSQLRegistry()

	var groupIDs []string
	err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
		query := fmt.Sprintf(`SELECT DISTINCT group_id FROM %s WHERE asset_number IS NULL ORDER BY group_id`, commodities)
		return tx.SelectContext(ctx, &groupIDs, query)
	})
	if err != nil {
		return 0, errxtrace.Wrap("failed to list groups with unnumbered commodities", err)
	}

	numberQuery := fmt.Sprintf(`
		WITH top AS (
			SELECT COALESCE(MAX(asset_number), 0) AS n FROM %[1]s WHERE group_id = $1
		), ordered AS (
			SELECT c.id, ROW_NUMBER() OVER (
				ORDER BY
					(SELECT MIN(e.occurred_at) FROM %[2]s e WHERE e.commodity_id = c.id) ASC NULLS LAST,
					NULLIF(c.registered_date, '') ASC NULLS LAST,
					c.id ASC
			) AS rn
			FROM %[1]s c
			WHERE c.group_id = $1 AND c.asset_number IS NULL
		)
		UPDATE %[1]s c
		SET asset_number = top.n + ordered.rn
		FROM ordered, top
		WHERE c.id = ordered.id`, commodities, events)

	total := 0
	for _, groupID := range groupIDs {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		var numbered int64
		err := reg.Do(ctx, func(ctx context.Context, tx *sqlx.Tx) error {
			if err := lockGroupAssetNumbers(ctx, tx, groupID); err != nil {
				return err
			}
			res, err := tx.ExecContext(ctx, numberQuery, groupID)
			if err != nil {
				return errxtrace.Wrap("failed to number commodities", err, errx.Attrs("group_id", groupID))
			}
			numbered, err = res.RowsAffected()
			return err
		})
		if err != nil {
			return total, errxtrace.Wrap("failed to backfill asset numbers", err, errx.Attrs("group_id", groupID))
		}
		total += int(numbered)
	}
	return total, nil
}
