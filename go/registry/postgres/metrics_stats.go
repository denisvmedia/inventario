package postgres

import (
	"context"
	"fmt"

	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

// newSystemStatsFunc returns the installation-wide stats source wired
// onto FactorySet.SystemStats for the business-metrics collector
// (#843). Every read runs under the inventario_background_worker role
// (via store.DoAsBackgroundWorker), which is NOT bound by the per-tenant
// RLS policies — so the counts and storage totals span every tenant and
// group, exactly what an installation-wide gauge needs. This is the same
// RLS-bypass posture the housekeeping sweeps use; it must never be
// reachable from a user-facing request path.
//
// All reads share a single transaction so the snapshot is internally
// consistent and pays one round of role setup rather than one per query.
func newSystemStatsFunc(dbx *sqlx.DB) registry.SystemStatsFunc {
	// Table names are resolved once from the defaults, matching the
	// constructor that builds every other registry with
	// store.DefaultTableNames.
	tableNames := store.DefaultTableNames

	return func(ctx context.Context) (registry.SystemStats, error) {
		var stats registry.SystemStats

		err := store.DoAsBackgroundWorker(ctx, dbx, func(ctx context.Context, tx *sqlx.Tx) error {
			// Entity counts. Each is a plain installation-wide
			// COUNT(*) — the background-worker role sees rows in every
			// tenant, so no tenant/group filter is applied.
			counts := []struct {
				table store.TableName
				dst   *int64
			}{
				{tableNames.Tenants(), &stats.Tenants},
				{tableNames.Users(), &stats.Users},
				{tableNames.LocationGroups(), &stats.LocationGroups},
				{tableNames.Locations(), &stats.Locations},
				{tableNames.Areas(), &stats.Areas},
				{tableNames.Commodities(), &stats.Commodities},
				{tableNames.Files(), &stats.Files},
			}
			for _, c := range counts {
				query := fmt.Sprintf("SELECT count(*) FROM %s", c.table)
				if err := tx.QueryRowxContext(ctx, query).Scan(c.dst); err != nil {
					return errxtrace.Wrap("failed to count rows", err)
				}
			}

			// Storage breakdown. Same CASE/SUM logic as
			// FileRegistry.SumSizeBreakdown, but installation-wide (no
			// tenant/group WHERE clause): export bundles
			// (linked_entity_type='export') are split out of the
			// category buckets so "exports" is a distinct series and the
			// other buckets don't double-count it. COALESCE keeps an
			// empty table from returning NULL.
			storageQuery := fmt.Sprintf(`
				SELECT
					COALESCE(SUM(CASE WHEN linked_entity_type = 'export' THEN size_bytes ELSE 0 END), 0) AS exports,
					COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'images' THEN size_bytes ELSE 0 END), 0) AS images,
					COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'documents' THEN size_bytes ELSE 0 END), 0) AS documents,
					COALESCE(SUM(CASE WHEN linked_entity_type IS DISTINCT FROM 'export' AND category = 'other' THEN size_bytes ELSE 0 END), 0) AS other
				FROM %s`, tableNames.Files())

			row := tx.QueryRowxContext(ctx, storageQuery)
			if err := row.Scan(
				&stats.StorageExports,
				&stats.StorageImages,
				&stats.StorageDocuments,
				&stats.StorageOther,
			); err != nil {
				return errxtrace.Wrap("failed to scan storage breakdown", err)
			}

			return nil
		})
		if err != nil {
			return registry.SystemStats{}, errxtrace.Wrap("failed to collect system stats", err)
		}

		return stats, nil
	}
}
