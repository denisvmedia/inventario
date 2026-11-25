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
			want: selectPrefix + "draft = $1 AND status = $2 ORDER BY current_price ASC NULLS LAST, id ASC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
		{
			// An unknown date or price sorts after every known one in both
			// directions; PostgreSQL alone would put NULL first on DESC.
			name: "descending sort on a nullable column keeps unknowns last",
			opts: registry.CommodityListOptions{SortField: registry.CommoditySortPurchaseDate, SortDesc: true},
			want: selectPrefix + "draft = $1 AND status = $2 ORDER BY purchase_date DESC NULLS LAST, id DESC LIMIT $3 OFFSET $4",
			args: []any{false, "in_use", 24, 0},
		},
		{
			// One ILIKE against the generated column covers most text
			// fields; brand and model are matched on their own columns.
			// A trigram index serves each of the three.
			name: "a search term is an ILIKE against search_text, brand and model",
			opts: registry.CommodityListOptions{Search: "  Drill  "},
			want: selectPrefix + "draft = $1 AND status = $2 AND (search_text ILIKE $3 OR brand ILIKE $4 OR model ILIKE $5) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $6 OFFSET $7",
			args: []any{false, "in_use", "%Drill%", "%Drill%", "%Drill%", 24, 0},
		},
		{
			// Terms are AND-ed; within a term the alternatives are OR-ed: a
			// tag found by label, and the asset number when the term reads
			// as an asset ID.
			name: "every term is its own predicate with its own alternatives",
			opts: registry.CommodityListOptions{
				IncludeInactive: true,
				Search:          `A-0042 "red drill"`,
				SearchTagLabels: map[string][]string{"red drill": {"power-tools", "red"}},
			},
			want: "SELECT *, COUNT(*) OVER() AS total_count FROM commodities WHERE " +
				"(search_text ILIKE $1 OR brand ILIKE $2 OR model ILIKE $3 OR asset_number = $4) AND " +
				"(search_text ILIKE $5 OR brand ILIKE $6 OR model ILIKE $7 OR tags ?| $8::text[]) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $9 OFFSET $10",
			args: []any{
				"%A-0042%", "%A-0042%", "%A-0042%", 42,
				"%red drill%", "%red drill%", "%red drill%", []string{"power-tools", "red"},
				24, 0,
			},
		},
		{
			// A % or _ the user typed matches itself, as strings.Contains
			// does in the memory backend.
			name: "LIKE wildcards in a term are escaped",
			opts: registry.CommodityListOptions{IncludeInactive: true, Search: `50%_off\`},
			want: "SELECT *, COUNT(*) OVER() AS total_count FROM commodities WHERE " +
				"(search_text ILIKE $1 OR brand ILIKE $2 OR model ILIKE $3) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $4 OFFSET $5",
			args: []any{`%50\%\_off\\%`, `%50\%\_off\\%`, `%50\%\_off\\%`, 24, 0},
		},
		{
			name: "tag filter: all, exclusion",
			opts: registry.CommodityListOptions{
				IncludeInactive: true,
				Tags:            registry.CommodityTagFilter{Include: []string{"a", "b"}, Exclude: []string{"c"}},
			},
			want: "SELECT *, COUNT(*) OVER() AS total_count FROM commodities WHERE " +
				"tags ?& $1::text[] AND NOT (COALESCE(tags, '[]'::jsonb) ?| $2::text[]) " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $3 OFFSET $4",
			args: []any{[]string{"a", "b"}, []string{"c"}, 24, 0},
		},
		{
			name: "tag filter: any",
			opts: registry.CommodityListOptions{
				IncludeInactive: true,
				Tags:            registry.CommodityTagFilter{Include: []string{"a", "b"}, Mode: registry.TagMatchAny},
			},
			want: "SELECT *, COUNT(*) OVER() AS total_count FROM commodities WHERE tags ?| $1::text[] " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $2 OFFSET $3",
			args: []any{[]string{"a", "b"}, 24, 0},
		},
		{
			// NULL and [] both read as "no tags".
			name: "tag filter: untagged takes no argument",
			opts: registry.CommodityListOptions{
				IncludeInactive: true,
				Tags:            registry.CommodityTagFilter{Untagged: true},
			},
			want: "SELECT *, COUNT(*) OVER() AS total_count FROM commodities WHERE " +
				"jsonb_array_length(COALESCE(tags, '[]'::jsonb)) = 0 " +
				"ORDER BY LOWER(name) ASC, id ASC LIMIT $1 OFFSET $2",
			args: []any{24, 0},
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
			"(search_text ILIKE $3 OR brand ILIKE $4 OR model ILIKE $5)")
	c.Check(countArgs, qt.DeepEquals, []any{false, "in_use", "%drill%", "%drill%", "%drill%"})
}

func TestCommodityTagCountsSQL(t *testing.T) {
	c := qt.New(t)

	opts := registry.CommodityListOptions{Search: "drill"}
	sb := commodityTagCountsSelect("commodities")
	query, args := addWhere(sb, commodityWhere(opts, "commodities", "commodity_loans")).Build()

	c.Check(query, qt.Equals,
		"SELECT t.slug, COUNT(DISTINCT commodities.id) FROM commodities "+
			"LEFT JOIN LATERAL jsonb_array_elements_text(COALESCE(commodities.tags, '[]'::jsonb)) AS t(slug) ON true "+
			"WHERE draft = $1 AND status = $2 AND (search_text ILIKE $3 OR brand ILIKE $4 OR model ILIKE $5) GROUP BY t.slug")
	c.Check(args, qt.DeepEquals, []any{false, "in_use", "%drill%", "%drill%", "%drill%"})
}
