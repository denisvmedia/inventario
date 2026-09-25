package postgres

// White-box: commodityWhere and applyCommodityOrder are unexported, and what
// this pins is the SQL they emit rather than anything reachable through the
// registry's exported surface.

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// The list query is assembled from a dozen optional filters, and a wrong
// placeholder or a dropped predicate reads as "no results" rather than as an
// error. These pin the text so a change to the builder has to be deliberate.
func TestCommodityListSQL(t *testing.T) {
	lentOut := true
	notLentOut := false
	const selectPrefix = "SELECT *, COUNT(*) OVER() AS total_count FROM commodities WHERE "

	for _, tc := range []struct {
		name string
		opts registry.CommodityListOptions
		want string
		args []any
	}{
		{
			name: "default view hides drafts and non-in-use rows",
			want: selectPrefix + "draft = $1 AND status = $2 ORDER BY LOWER(name) ASC, id ASC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
		{
			// The implicit in_use goes away once the caller names statuses.
			name: "explicit statuses replace the implicit one",
			opts: registry.CommodityListOptions{
				Statuses: []models.CommodityStatus{"in_use", "sold"},
			},
			want: selectPrefix + "draft = $1 AND status IN ($2, $3) ORDER BY LOWER(name) ASC, id ASC LIMIT $4 OFFSET $5",
			args: []any{false, "in_use", "sold", 24, 0},
		},
		{
			name: "IncludeInactive drops both defaults",
			opts: registry.CommodityListOptions{IncludeInactive: true},
			want: "SELECT *, COUNT(*) OVER() AS total_count FROM commodities ORDER BY LOWER(name) ASC, id ASC LIMIT $1 OFFSET $2",
			args: []any{24, 0},
		},
		{
			// The tiebreaker has to carry the same direction as the leading
			// column, or page boundaries move between backends on rows that
			// share a sort value.
			name: "descending sort carries the id tiebreaker",
			opts: registry.CommodityListOptions{SortField: registry.CommoditySortName, SortDesc: true},
			want: selectPrefix + "draft = $1 AND status = $2 ORDER BY LOWER(name) DESC, id DESC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
		{
			// Only name sorts are case-folded; the functional index is on
			// LOWER(name) alone.
			name: "non-name sorts are not case-folded",
			opts: registry.CommodityListOptions{SortField: registry.CommoditySortCurrentPrice},
			want: selectPrefix + "draft = $1 AND status = $2 ORDER BY current_price ASC, id ASC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
		{
			name: "search folds the pattern and matches both name columns",
			opts: registry.CommodityListOptions{Search: "  Drill  "},
			want: selectPrefix + "draft = $1 AND status = $2 AND (LOWER(name) LIKE $3 OR LOWER(short_name) LIKE $4) ORDER BY LOWER(name) ASC, id ASC LIMIT $5 OFFSET $6",
			args: []any{false, "in_use", "%drill%", "%drill%", 24, 0},
		},
		{
			name: "types and statuses become IN lists",
			opts: registry.CommodityListOptions{
				Types:    []models.CommodityType{"equipment", "consumable"},
				Statuses: []models.CommodityStatus{"in_use"},
			},
			want: selectPrefix + "draft = $1 AND type IN ($2, $3) AND status IN ($4) ORDER BY LOWER(name) ASC, id ASC LIMIT $5 OFFSET $6",
			args: []any{false, "equipment", "consumable", "in_use", 24, 0},
		},
		{
			name: "an explicit area wins over Unassigned",
			opts: registry.CommodityListOptions{AreaID: "area-1", Unassigned: true},
			want: selectPrefix + "draft = $1 AND status = $2 AND area_id = $3 ORDER BY LOWER(name) ASC, id ASC LIMIT $4 OFFSET $5",
			args: []any{false, "in_use", "area-1", 24, 0},
		},
		{
			name: "Unassigned alone takes no argument",
			opts: registry.CommodityListOptions{Unassigned: true},
			want: selectPrefix + "draft = $1 AND status = $2 AND area_id IS NULL ORDER BY LOWER(name) ASC, id ASC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
		{
			// Every non-none branch guards against the empty string, which
			// reaches the column via the PDate zero value and would otherwise
			// satisfy expired's `<`.
			name: "warranty statuses are OR-ed with empty-string guards",
			opts: registry.CommodityListOptions{
				WarrantyStatuses: []registry.WarrantyStatusFilter{"expired", "expiring", "active", "none"},
				WarrantyNow:      time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			},
			want: selectPrefix + "draft = $1 AND status = $2 AND " +
				"((warranty_expires_at IS NOT NULL AND warranty_expires_at <> '' AND warranty_expires_at < $3) OR " +
				"(warranty_expires_at <> '' AND warranty_expires_at >= $4 AND warranty_expires_at <= $5) OR " +
				"(warranty_expires_at <> '' AND warranty_expires_at > $6) OR " +
				"(warranty_expires_at IS NULL OR warranty_expires_at = $7)) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $8 OFFSET $9",
			args: []any{false, "in_use", "2026-01-15", "2026-01-15", "2026-03-16", "2026-03-16", "", 24, 0},
		},
		{
			name: "WarrantyExpiresBefore guards the empty string too",
			opts: registry.CommodityListOptions{WarrantyExpiresBefore: "2026-03-01"},
			want: selectPrefix + "draft = $1 AND status = $2 AND " +
				"(warranty_expires_at IS NOT NULL AND warranty_expires_at <> '' AND warranty_expires_at < $3) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $4 OFFSET $5",
			args: []any{false, "in_use", "2026-03-01", 24, 0},
		},
		{
			// The subquery's outer reference is qualified with the table name
			// passed in, so a TableNames override reaches it.
			name: "LentOut true becomes EXISTS",
			opts: registry.CommodityListOptions{LentOut: &lentOut},
			want: selectPrefix + "draft = $1 AND status = $2 AND " +
				"EXISTS (SELECT 1 FROM commodity_loans WHERE commodity_id = commodities.id AND returned_at IS NULL) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
		{
			name: "LentOut false becomes NOT EXISTS",
			opts: registry.CommodityListOptions{LentOut: &notLentOut},
			want: selectPrefix + "draft = $1 AND status = $2 AND " +
				"NOT EXISTS (SELECT 1 FROM commodity_loans WHERE commodity_id = commodities.id AND returned_at IS NULL) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)

			where := commodityWhere(tc.opts, "commodities", "commodity_loans")
			sb := newSelect().
				Select("*", totalCountColumn).
				From("commodities").
				Limit(24).
				Offset(0)
			if where != nil {
				sb.AddWhereClause(where)
			}
			applyCommodityOrder(sb, tc.opts)

			got, args := sb.Build()
			c.Check(got, qt.Equals, tc.want)
			c.Check(args, qt.DeepEquals, tc.args)
		})
	}
}

// The count query has to carry the same predicate as the data query, or the
// total describes a different set than the page — the reason the clause is
// built once and shared.
func TestCommodityCountSQLSharesThePredicate(t *testing.T) {
	c := qt.New(t)

	opts := registry.CommodityListOptions{
		Search:   "drill",
		Statuses: []models.CommodityStatus{"in_use"},
	}
	where := commodityWhere(opts, "commodities", "commodity_loans")

	countSB := newSelect().Select("COUNT(*)").From("commodities")
	countSB.AddWhereClause(where)
	countSQL, countArgs := countSB.Build()

	c.Check(countSQL, qt.Equals,
		"SELECT COUNT(*) FROM commodities WHERE draft = $1 AND status IN ($2) AND "+
			"(LOWER(name) LIKE $3 OR LOWER(short_name) LIKE $4)")
	c.Check(countArgs, qt.DeepEquals, []any{false, "in_use", "%drill%", "%drill%"})
}
