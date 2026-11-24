package postgres

// White-box: the property below has to see the SQL text each builder emits,
// and the builders are unexported.

import (
	"strconv"
	"strings"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	sqlb "github.com/huandu/go-sqlbuilder"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

// sqlInjectionPayloads are the classic shapes an attacker types into a search
// box or an id parameter: quote breaking, comment truncation, stacked
// statements, UNION, and go-sqlbuilder's own placeholder syntax, which must
// stay inert inside a value.
var sqlInjectionPayloads = []string{
	"' OR '1'='1",
	"' OR 1=1 --",
	"'; DROP TABLE commodities; --",
	`"; DROP TABLE tags; --`,
	"') OR ('a'='a",
	"1; SELECT pg_sleep(10)",
	"' UNION SELECT id, email FROM users --",
	`\'; --`,
	"/* comment */ OR 1=1",
	"$1",
	"$0 $? $$",
	"${name}",
	"admin'--",
}

// listQuery renders one list query with v in every value slot a caller can
// fill from a request: search terms, ids, statuses, kinds and dates.
type listQuery struct {
	name  string
	build func(v string) (string, []any)
}

// listQueryProbes returns, per query name, the reference value v is compared
// against when it is not "probe". Most queries take any non-blank value the
// same way. The item search does not: it splits v into terms, one predicate
// each, and a term that reads as an asset ID adds the asset number to its
// predicate. Its probe keeps that shape, so the property still holds for the
// characters.
var listQueryProbes = map[string]func(v string) string{
	"commodities": searchProbe,
}

// searchProbe returns a search with the shape of v: one placeholder term per
// term of v, a number where the term reads as an asset ID. A v without terms
// gets "\"\"", which is not blank yet has no terms either.
func searchProbe(v string) string {
	terms := registry.SplitSearchTerms(v)
	if len(terms) == 0 {
		return `""`
	}
	parts := make([]string, len(terms))
	for i, term := range terms {
		if _, ok := models.ParseAssetID(term); ok {
			parts[i] = strconv.Itoa(i + 1)
		} else {
			parts[i] = "probe" + strconv.Itoa(i)
		}
	}
	return strings.Join(parts, " ")
}

// searchLabels resolves every term of search to the slug v, the way
// services/commodityquery fills SearchTagLabels.
func searchLabels(search, v string) map[string][]string {
	labels := make(map[string][]string)
	for _, term := range registry.SplitSearchTerms(search) {
		labels[term] = []string{v}
	}
	return labels
}

func listQueries() []listQuery {
	tables := store.DefaultTableNames
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	where := func(table string, wc *sqlb.WhereClause) (string, []any) {
		return addWhere(newSelect().Select("*").From(table), wc).Build()
	}

	return []listQuery{
		{"commodities", func(v string) (string, []any) {
			return where("commodities", commodityWhere(registry.CommodityListOptions{
				Search:                v,
				SearchTagLabels:       searchLabels(v, v),
				Tags:                  registry.CommodityTagFilter{Include: []string{v}, Exclude: []string{v}},
				AreaID:                v,
				Types:                 []models.CommodityType{models.CommodityType(v)},
				Statuses:              []models.CommodityStatus{models.CommodityStatus(v)},
				WarrantyExpiresBefore: v,
			}, "commodities", "commodity_loans"))
		}},
		{"files", func(v string) (string, []any) {
			fileType := models.FileType(v)
			category := models.FileCategory(v)
			return where("files", fileWhere(v, &fileType, &category, []string{v}, &v, &v))
		}},
		{"areas", func(v string) (string, []any) {
			return where("areas", areaWhere(registry.AreaListOptions{LocationID: v}))
		}},
		{"commodity events", func(v string) (string, []any) {
			return where("commodity_events", commodityEventWhere(v, registry.CommodityEventListOptions{
				Kinds: []models.CommodityEventKind{models.CommodityEventKind(v)},
			}))
		}},
		{"maintenance schedules", func(v string) (string, []any) {
			return where("maintenance_schedules", maintenanceWhere(registry.MaintenanceListOptions{DueBefore: v, EnabledOnly: true}))
		}},
		{"tags", func(v string) (string, []any) {
			return where("tags t", tagWhere(v, models.TagKindCommodity))
		}},
		{"admin tenants", func(v string) (string, []any) {
			_, page := adminTenantListQueries(tables, registry.AdminTenantListOptions{Query: v})
			return page.Build()
		}},
		{"admin users", func(v string) (string, []any) {
			_, page := adminUserListQueries(tables, v, registry.AdminUserListOptions{Query: v})
			return page.Build()
		}},
		{"admin groups", func(v string) (string, []any) {
			_, page := adminGroupListQueries(tables, registry.AdminGroupListOptions{Query: v, TenantID: v, Status: v})
			return page.Build()
		}},
		{"owned content", func(v string) (string, []any) {
			return ownedRowExistsQuery("commodities", "created_by_user_id", "group_id", v, v, []string{v, v})
		}},
		{"orphan candidates", func(v string) (string, []any) {
			return orphanCandidatesQuery(tables, now, registry.OrphanCandidateCursor{CreatedAt: now, ID: v}, 10)
		}},
	}
}

// The SQL text of every list query must not depend on the values in it. When
// it does not, a value can only reach PostgreSQL as an argument, and no input
// can change what the statement does. The argument count is checked too, so a
// value that silently drops out of the query does not pass for a safe one.
//
// The seed corpus runs as part of `go test`; `go test -fuzz` explores further.
func FuzzListQueriesKeepValuesOutOfSQL(f *testing.F) {
	for _, p := range sqlInjectionPayloads {
		f.Add(p)
	}

	queries := listQueries()
	f.Fuzz(func(t *testing.T, v string) {
		// An empty or blank value switches a filter off, which changes the SQL
		// by design.
		if strings.TrimSpace(v) == "" {
			t.Skip()
		}
		c := qt.New(t)

		for _, q := range queries {
			probe := "probe"
			if shape, ok := listQueryProbes[q.name]; ok {
				probe = shape(v)
			}
			wantSQL, wantArgs := q.build(probe)
			gotSQL, gotArgs := q.build(v)
			c.Check(gotSQL, qt.Equals, wantSQL, qt.Commentf("%s: the value changed the SQL text", q.name))
			c.Check(gotArgs, qt.HasLen, len(wantArgs), qt.Commentf("%s", q.name))
		}
	})
}
