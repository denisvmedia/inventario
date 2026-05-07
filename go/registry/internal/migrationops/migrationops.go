// Package migrationops is the worker-only entry point for the
// write-once acquisition-price provenance columns added in #1550 (epic
// #202). It lives under registry/internal so Go's `internal/` boundary
// keeps it inaccessible to anything outside the registry tree — only the
// migration worker (in go/services/, importing through the registry
// adapter that PR 3 will add) is allowed to call SetAcquisition.
//
// The runtime guard inside SetAcquisition is defence in depth: it
// re-reads the row inside the worker's TX2 and refuses to overwrite if
// either column is already non-NULL. So a buggy second call from the
// worker itself is caught loudly rather than silently corrupting the
// "frozen forever" provenance contract.
package migrationops

import (
	"context"
	"errors"
	"fmt"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"
	"github.com/shopspring/decimal"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SetAcquisition writes commodities.acquisition_price /
// acquisition_currency for `commodityID` in the same transaction `tx`
// the worker is using for TX2. Returns registry.ErrAcquisitionAlreadySet
// if either column is already non-NULL, registry.ErrNotFound if the row
// does not exist (or RLS hides it from `tx`), and any wrapped DB error
// otherwise.
//
// The function does NOT begin or commit a transaction — it expects to
// run inside the worker's existing tx so the audit row, the commodity
// price update, and the acquisition fill all commit together (or all
// roll back together if anything down-stream fails).
//
// The CHECK constraint commodities_acquisition_pair enforces the
// "both NULL or both set" invariant at the schema level; this helper
// always writes both columns together.
func SetAcquisition(ctx context.Context, tx *sqlx.Tx, table string, commodityID string, price decimal.Decimal, currency models.Currency) error {
	if commodityID == "" {
		return errxtrace.Wrap("commodity id is required", registry.ErrFieldRequired)
	}
	if currency == "" {
		return errxtrace.Wrap("currency is required", registry.ErrFieldRequired)
	}

	// 1. Read the current acquisition columns under tx isolation.
	var (
		dbPrice    *decimal.Decimal
		dbCurrency *models.Currency
	)
	selectQuery := fmt.Sprintf(
		`SELECT acquisition_price, acquisition_currency FROM %s WHERE id = $1`,
		table,
	)
	if err := tx.QueryRowxContext(ctx, selectQuery, commodityID).Scan(&dbPrice, &dbCurrency); err != nil {
		if errors.Is(err, sqlNoRows) {
			return registry.ErrNotFound
		}
		return errxtrace.Wrap("failed to read commodity acquisition columns", err)
	}

	// 2. Write-once guard. CHECK constraint guarantees both-or-neither,
	// so checking either column is sufficient — but we check both for
	// defence in depth in case the constraint ever drifts.
	if dbPrice != nil || dbCurrency != nil {
		return registry.ErrAcquisitionAlreadySet
	}

	// 3. Write both columns atomically. Pair them in the SET so the
	// CHECK constraint can never observe a half-state during the row
	// write.
	updateQuery := fmt.Sprintf(
		`UPDATE %s
		    SET acquisition_price = $2,
		        acquisition_currency = $3
		  WHERE id = $1
		    AND acquisition_price IS NULL
		    AND acquisition_currency IS NULL`,
		table,
	)
	res, err := tx.ExecContext(ctx, updateQuery, commodityID, price, string(currency))
	if err != nil {
		return errxtrace.Wrap("failed to write commodity acquisition columns", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return errxtrace.Wrap("failed to read RowsAffected on acquisition write", err)
	}
	if rows == 0 {
		// Either the row vanished after the SELECT (unlikely under
		// SERIALIZABLE / REPEATABLE READ inside the same tx) or
		// concurrent worker-mode wrote between the read and the
		// guarded UPDATE. Be loud either way.
		return registry.ErrAcquisitionAlreadySet
	}
	return nil
}
