package postgres

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"
)

// totalCountColumn is what `COUNT(*) OVER()` is aliased to on a page query, so
// the page and its total arrive in one round trip instead of two (#1032).
const totalCountColumn = "COUNT(*) OVER() AS total_count"

// totalUnknown is what a page query reports when it returned no rows. The
// window function rides along on a row, so an empty page carries no total and
// the caller has to ask for it separately — otherwise an offset past the end
// would report zero rows out of zero rather than zero out of the real count.
const totalUnknown = -1

// countRows runs a plain COUNT(*) for the cases the window cannot cover.
func countRows(ctx context.Context, tx *sqlx.Tx, query string, args ...any) (int, error) {
	var total int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&total); err != nil {
		return 0, errxtrace.Wrap("failed to count rows", err)
	}
	return total, nil
}
