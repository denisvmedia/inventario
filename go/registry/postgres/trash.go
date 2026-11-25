package postgres

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"

	"go.5x5.cz/inventario/registry"
)

// Trash filters. Commodities and files carry deleted_at; a NULL value is a live
// row. The ordinary reads of both registries add liveRow; the trash reads add
// trashedRow. A query that reaches commodities from another table (loans,
// services, maintenance schedules) keeps the rows of trashed items out with
// liveCommodityOf.
const (
	liveRow    = "deleted_at IS NULL"
	trashedRow = "deleted_at IS NOT NULL"
)

// liveCommodityOf returns an EXISTS predicate that holds when the commodity
// referenced by outerTable.column is live. Both names come from code (a
// TableNames method and a column constant), never from a request.
func liveCommodityOf(commoditiesTable, outerTable, column string) string {
	return fmt.Sprintf("EXISTS (SELECT 1 FROM %s tc WHERE tc.id = %s.%s AND tc.deleted_at IS NULL)",
		commoditiesTable, outerTable, column)
}

// scanOne runs a query expected to return at most one row and scans it into
// dest. No row is registry.ErrNotFound; a failure while reading is returned as
// is, so a transient fault is not mistaken for an absent row.
func scanOne[T any](ctx context.Context, tx *sqlx.Tx, dest *T, query string, args ...any) error {
	rows, err := tx.QueryxContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return err
		}
		return registry.ErrNotFound
	}
	return rows.StructScan(dest)
}

// scanAll runs a query and scans every row into a fresh T.
func scanAll[T any](ctx context.Context, tx *sqlx.Tx, query string, args ...any) ([]*T, error) {
	rows, err := tx.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*T
	for rows.Next() {
		var item T
		if err := rows.StructScan(&item); err != nil {
			return nil, err
		}
		out = append(out, &item)
	}
	return out, rows.Err()
}

// execAffected runs a statement and reports whether it changed a row.
func execAffected(ctx context.Context, tx *sqlx.Tx, query string, args ...any) (bool, error) {
	res, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
