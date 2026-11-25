package postgres

// White-box: the query builders below are unexported, and what these pin is
// the SQL they emit rather than anything reachable through the registries'
// exported surface. list_filters_test.go covers the rows the same queries
// return against PostgreSQL.

import (
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	sqlb "github.com/huandu/go-sqlbuilder"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/postgres/store"
)

type sqlCase struct {
	name string
	sql  string
	args []any
}

// countSQL renders where the way every paginated list renders its total, so a
// case pins the predicate alone.
func countSQL(table string, where *sqlb.WhereClause) (string, []any) {
	return addWhere(newSelect().Select("COUNT(*)").From(table), where).Build()
}

func checkSQL(c *qt.C, gotSQL string, gotArgs []any, want sqlCase) {
	c.Helper()
	c.Check(gotSQL, qt.Equals, want.sql)
	c.Check(gotArgs, qt.DeepEquals, want.args)
}

func TestAreaWhereSQL(t *testing.T) {
	for _, tc := range []struct {
		opts registry.AreaListOptions
		want sqlCase
	}{
		{want: sqlCase{name: "no filter yields no clause", sql: "SELECT COUNT(*) FROM areas"}},
		{
			opts: registry.AreaListOptions{LocationID: "loc-1"},
			want: sqlCase{name: "location", sql: "SELECT COUNT(*) FROM areas WHERE location_id = $1", args: []any{"loc-1"}},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			got, args := countSQL("areas", areaWhere(tc.opts))
			checkSQL(qt.New(t), got, args, tc.want)
		})
	}
}

func TestCommodityEventWhereSQL(t *testing.T) {
	for _, tc := range []struct {
		opts registry.CommodityEventListOptions
		want sqlCase
	}{
		{want: sqlCase{
			name: "the commodity alone",
			sql:  "SELECT COUNT(*) FROM commodity_events WHERE commodity_id = $1",
			args: []any{"com-1"},
		}},
		{
			opts: registry.CommodityEventListOptions{Kinds: []models.CommodityEventKind{"updated", "moved"}},
			want: sqlCase{
				name: "kinds become an IN list",
				sql:  "SELECT COUNT(*) FROM commodity_events WHERE commodity_id = $1 AND kind IN ($2, $3)",
				args: []any{"com-1", "updated", "moved"},
			},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			got, args := countSQL("commodity_events", commodityEventWhere("com-1", tc.opts))
			checkSQL(qt.New(t), got, args, tc.want)
		})
	}
}

func TestLoanAndServiceStateWhereSQL(t *testing.T) {
	now := time.Date(2026, 5, 10, 23, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		where *sqlb.WhereClause
		table string
		want  sqlCase
	}{
		{
			where: loanStateWhere(registry.LoanStateAll, now),
			table: "commodity_loans",
			want:  sqlCase{name: "all loans", sql: "SELECT COUNT(*) FROM commodity_loans"},
		},
		{
			where: loanStateWhere(registry.LoanStateOpen, now),
			table: "commodity_loans",
			want:  sqlCase{name: "open loans", sql: "SELECT COUNT(*) FROM commodity_loans WHERE returned_at IS NULL"},
		},
		{
			where: loanStateWhere(registry.LoanStateOverdue, now),
			table: "commodity_loans",
			want: sqlCase{
				name: "overdue loans",
				sql:  "SELECT COUNT(*) FROM commodity_loans WHERE returned_at IS NULL AND due_back_at IS NOT NULL AND due_back_at < $1",
				args: []any{"2026-05-10"},
			},
		},
		{
			where: loanStateWhere(registry.LoanStateReturned, now),
			table: "commodity_loans",
			want:  sqlCase{name: "returned loans", sql: "SELECT COUNT(*) FROM commodity_loans WHERE returned_at IS NOT NULL"},
		},
		{
			where: serviceStateWhere(registry.ServiceStateAll, now),
			table: "commodity_services",
			want:  sqlCase{name: "all services", sql: "SELECT COUNT(*) FROM commodity_services"},
		},
		{
			where: serviceStateWhere(registry.ServiceStateOpen, now),
			table: "commodity_services",
			want:  sqlCase{name: "open services", sql: "SELECT COUNT(*) FROM commodity_services WHERE returned_at IS NULL"},
		},
		{
			where: serviceStateWhere(registry.ServiceStateOverdue, now),
			table: "commodity_services",
			want: sqlCase{
				name: "overdue services",
				sql:  "SELECT COUNT(*) FROM commodity_services WHERE returned_at IS NULL AND expected_return_at IS NOT NULL AND expected_return_at < $1",
				args: []any{"2026-05-10"},
			},
		},
		{
			where: serviceStateWhere(registry.ServiceStateCompleted, now),
			table: "commodity_services",
			want:  sqlCase{name: "completed services", sql: "SELECT COUNT(*) FROM commodity_services WHERE returned_at IS NOT NULL"},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			got, args := countSQL(tc.table, tc.where)
			checkSQL(qt.New(t), got, args, tc.want)
		})
	}
}

func TestPendingLoanRemindersSQL(t *testing.T) {
	// 01:00 in UTC+2 is still the previous day in UTC, and the query compares
	// UTC calendar dates.
	now := time.Date(2026, 5, 11, 1, 0, 0, 0, time.FixedZone("UTC+2", 2*60*60))

	for _, tc := range []struct {
		kind registry.LoanReminderKind
		want sqlCase
	}{
		{
			kind: registry.LoanReminderKindOverdue,
			want: sqlCase{
				name: "overdue",
				sql: "SELECT * FROM commodity_loans WHERE returned_at IS NULL AND due_back_at IS NOT NULL AND " +
					"EXISTS (SELECT 1 FROM commodities tc WHERE tc.id = commodity_loans.commodity_id AND tc.deleted_at IS NULL) AND " +
					"due_back_at < $1 AND reminder_sent_overdue = false ORDER BY due_back_at ASC, id ASC",
				args: []any{"2026-05-10"},
			},
		},
		{
			kind: registry.LoanReminderKindDueSoon,
			want: sqlCase{
				name: "due soon",
				sql: "SELECT * FROM commodity_loans WHERE returned_at IS NULL AND due_back_at IS NOT NULL AND " +
					"EXISTS (SELECT 1 FROM commodities tc WHERE tc.id = commodity_loans.commodity_id AND tc.deleted_at IS NULL) AND " +
					"due_back_at >= $1 AND due_back_at <= $2 AND reminder_sent_due_soon = false ORDER BY due_back_at ASC, id ASC",
				args: []any{"2026-05-10", "2026-05-13"},
			},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			c := qt.New(t)
			got, args, err := pendingLoanRemindersQuery("commodity_loans", "commodities", tc.kind, now, 3)
			c.Assert(err, qt.IsNil)
			checkSQL(c, got, args, tc.want)
		})
	}
}

func TestPendingLoanRemindersSQLRejectsUnknownKind(t *testing.T) {
	c := qt.New(t)

	_, _, err := pendingLoanRemindersQuery("commodity_loans", "commodities", "weekly", time.Now(), 3)
	c.Assert(err, qt.ErrorIs, registry.ErrInvalidInput)
}

func TestMaintenanceWhereSQL(t *testing.T) {
	for _, tc := range []struct {
		opts registry.MaintenanceListOptions
		want sqlCase
	}{
		{want: sqlCase{name: "no filter yields no clause", sql: "SELECT COUNT(*) FROM maintenance_schedules"}},
		{
			opts: registry.MaintenanceListOptions{EnabledOnly: true},
			want: sqlCase{name: "enabled only", sql: "SELECT COUNT(*) FROM maintenance_schedules WHERE enabled = true"},
		},
		{
			opts: registry.MaintenanceListOptions{EnabledOnly: true, DueBefore: "2026-06-01"},
			want: sqlCase{
				name: "due before is inclusive",
				sql:  "SELECT COUNT(*) FROM maintenance_schedules WHERE enabled = true AND next_due_at <= $1",
				args: []any{"2026-06-01"},
			},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			got, args := countSQL("maintenance_schedules", maintenanceWhere(tc.opts))
			checkSQL(qt.New(t), got, args, tc.want)
		})
	}
}

func TestTagListSQL(t *testing.T) {
	r := &TagRegistry{tableNames: store.DefaultTableNames}
	const usage = "(SELECT COUNT(*) FROM commodities WHERE tags @> jsonb_build_array(t.slug) AND deleted_at IS NULL)"

	for _, tc := range []struct {
		opts registry.TagListOptions
		want sqlCase
	}{
		{want: sqlCase{
			name: "no filter sorts by label, case-folded",
			sql:  "SELECT t.* FROM tags t ORDER BY LOWER(t.label) ASC, t.id ASC LIMIT $1 OFFSET $2",
			args: []any{20, 0},
		}},
		{
			opts: registry.TagListOptions{Search: "kit", Kind: models.TagKindCommodity, SortDesc: true},
			want: sqlCase{
				name: "search and kind",
				sql: "SELECT t.* FROM tags t WHERE (t.label ILIKE $1 OR t.slug ILIKE $2) AND t.kind = $3 " +
					"ORDER BY LOWER(t.label) DESC, t.id ASC LIMIT $4 OFFSET $5",
				args: []any{"%kit%", "%kit%", models.TagKindCommodity, 20, 0},
			},
		},
		{
			// Only the two concrete kinds filter; anything else lists across
			// both.
			opts: registry.TagListOptions{Kind: "other"},
			want: sqlCase{
				name: "an unknown kind adds no condition",
				sql:  "SELECT t.* FROM tags t ORDER BY LOWER(t.label) ASC, t.id ASC LIMIT $1 OFFSET $2",
				args: []any{20, 0},
			},
		},
		{
			opts: registry.TagListOptions{Kind: models.TagKindCommodity, SortField: registry.TagSortUsage, SortDesc: true},
			want: sqlCase{
				name: "usage counts the kind's own table",
				sql:  "SELECT t.* FROM tags t WHERE t.kind = $1 ORDER BY " + usage + " DESC, t.label ASC LIMIT $2 OFFSET $3",
				args: []any{models.TagKindCommodity, 20, 0},
			},
		},
		{
			opts: registry.TagListOptions{SortField: registry.TagSortCreatedAt},
			want: sqlCase{
				name: "created_at",
				sql:  "SELECT t.* FROM tags t ORDER BY t.created_at ASC, t.label ASC LIMIT $1 OFFSET $2",
				args: []any{20, 0},
			},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			sb := newSelect().Select("t.*").From("tags t")
			addWhere(sb, tagWhere(tc.opts.Search, tc.opts.Kind))
			r.applyTagOrder(sb, tc.opts)
			got, args := paginate(sb, 0, 20).Build()
			checkSQL(qt.New(t), got, args, tc.want)
		})
	}
}

// A negative LIMIT would make go-sqlbuilder drop the clause and return every
// row instead of an empty page.
func TestPaginateClampsNegativeValues(t *testing.T) {
	c := qt.New(t)

	got, args := paginate(newSelect().Select("*").From("tags"), -5, -1).Build()
	c.Check(got, qt.Equals, "SELECT * FROM tags LIMIT $1 OFFSET $2")
	c.Check(args, qt.DeepEquals, []any{0, 0})
}

func TestAdminTenantListSQL(t *testing.T) {
	c := qt.New(t)

	countSB, pageSB := adminTenantListQueries(store.DefaultTableNames, registry.AdminTenantListOptions{
		Query:    " widget ",
		Page:     2,
		PerPage:  10,
		SortDesc: true,
	})
	count, countArgs := countSB.Build()
	page, pageArgs := pageSB.Build()

	pattern := "%widget%"
	c.Check(count, qt.Equals, "SELECT COUNT(*) FROM tenants AS t WHERE (t.name ILIKE $1 OR t.slug ILIKE $2 OR t.domain ILIKE $3)")
	c.Check(countArgs, qt.DeepEquals, []any{pattern, pattern, pattern})
	c.Check(page, qt.Equals, "SELECT t.*, "+
		"(SELECT COUNT(*) FROM users AS u WHERE u.tenant_id = t.id) AS _user_count, "+
		"(SELECT COUNT(*) FROM location_groups AS g WHERE g.tenant_id = t.id) AS _group_count "+
		"FROM tenants AS t WHERE (t.name ILIKE $1 OR t.slug ILIKE $2 OR t.domain ILIKE $3) "+
		"ORDER BY t.name DESC, t.id ASC LIMIT $4 OFFSET $5")
	c.Check(pageArgs, qt.DeepEquals, []any{pattern, pattern, pattern, 10, 10})
}

func TestAdminTenantListSQLDefaults(t *testing.T) {
	c := qt.New(t)

	// An invalid sort field falls back to name, and a missing page size to 50.
	_, pageSB := adminTenantListQueries(store.DefaultTableNames, registry.AdminTenantListOptions{SortField: "password"})
	page, args := pageSB.Build()

	c.Check(page, qt.Equals, "SELECT t.*, "+
		"(SELECT COUNT(*) FROM users AS u WHERE u.tenant_id = t.id) AS _user_count, "+
		"(SELECT COUNT(*) FROM location_groups AS g WHERE g.tenant_id = t.id) AS _group_count "+
		"FROM tenants AS t ORDER BY t.name ASC, t.id ASC LIMIT $1 OFFSET $2")
	c.Check(args, qt.DeepEquals, []any{50, 0})
}

func TestAdminUserListSQL(t *testing.T) {
	c := qt.New(t)

	active := true
	countSB, pageSB := adminUserListQueries(store.DefaultTableNames, "tenant-1", registry.AdminUserListOptions{
		Query:     "ann",
		IsActive:  &active,
		SortField: registry.AdminUserSortName,
	})
	count, countArgs := countSB.Build()
	page, pageArgs := pageSB.Build()

	const where = "WHERE u.tenant_id = $1 AND (u.email ILIKE $2 OR u.name ILIKE $3) AND u.is_active = $4"
	c.Check(count, qt.Equals, "SELECT COUNT(*) FROM users AS u "+where)
	c.Check(countArgs, qt.DeepEquals, []any{"tenant-1", "%ann%", "%ann%", true})
	c.Check(page, qt.Equals, "SELECT u.*, "+
		"(SELECT COUNT(*) FROM group_memberships AS m WHERE m.member_user_id = u.id AND m.tenant_id = u.tenant_id) AS _group_membership_count "+
		"FROM users AS u "+where+" ORDER BY u.name ASC, u.id ASC LIMIT $5 OFFSET $6")
	c.Check(pageArgs, qt.DeepEquals, []any{"tenant-1", "%ann%", "%ann%", true, 50, 0})
}

func TestAdminUserListSQLTenantOnly(t *testing.T) {
	c := qt.New(t)

	// The tenant predicate is always there, whatever else is filtered.
	countSB, _ := adminUserListQueries(store.DefaultTableNames, "tenant-1", registry.AdminUserListOptions{Query: "   "})
	count, args := countSB.Build()

	c.Check(count, qt.Equals, "SELECT COUNT(*) FROM users AS u WHERE u.tenant_id = $1")
	c.Check(args, qt.DeepEquals, []any{"tenant-1"})
}

func TestAdminGroupListSQL(t *testing.T) {
	c := qt.New(t)

	countSB, pageSB := adminGroupListQueries(store.DefaultTableNames, registry.AdminGroupListOptions{
		Query:     "garage",
		TenantID:  "tenant-1",
		Status:    "active",
		SortField: registry.AdminGroupSortCreatedAt,
		SortDesc:  true,
	})
	count, countArgs := countSB.Build()
	page, pageArgs := pageSB.Build()

	const where = "WHERE (g.name ILIKE $1 OR g.slug ILIKE $2) AND g.tenant_id = $3 AND g.status = $4"
	c.Check(count, qt.Equals, "SELECT COUNT(*) FROM location_groups AS g "+where)
	c.Check(countArgs, qt.DeepEquals, []any{"%garage%", "%garage%", "tenant-1", "active"})
	c.Check(page, qt.Equals, "SELECT g.*, "+
		"(SELECT COUNT(*) FROM group_memberships AS m WHERE m.group_id = g.id) AS _member_count, "+
		"t.id AS _tenant_id, t.name AS _tenant_name, t.slug AS _tenant_slug "+
		"FROM location_groups AS g LEFT JOIN tenants AS t ON t.id = g.tenant_id "+
		where+" ORDER BY g.created_at DESC, g.id ASC LIMIT $5 OFFSET $6")
	c.Check(pageArgs, qt.DeepEquals, []any{"%garage%", "%garage%", "tenant-1", "active", 50, 0})
}

func TestOwnedRowExistsSQL(t *testing.T) {
	for _, tc := range []struct {
		purged []string
		want   sqlCase
	}{
		{
			// Nothing purged, so any owned row counts as retained.
			want: sqlCase{
				name: "no exclusion",
				sql:  "SELECT EXISTS (SELECT 1 FROM commodities WHERE tenant_id = $1 AND created_by_user_id = $2)",
				args: []any{"tenant-1", "user-1"},
			},
		},
		{
			purged: []string{"group-1", "group-2"},
			want: sqlCase{
				name: "purged groups are excluded",
				sql:  "SELECT EXISTS (SELECT 1 FROM commodities WHERE tenant_id = $1 AND created_by_user_id = $2 AND group_id NOT IN ($3, $4))",
				args: []any{"tenant-1", "user-1", "group-1", "group-2"},
			},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			got, args := ownedRowExistsQuery("commodities", "created_by_user_id", "group_id", "tenant-1", "user-1", tc.purged)
			checkSQL(qt.New(t), got, args, tc.want)
		})
	}
}

func TestOrphanCandidatesSQL(t *testing.T) {
	olderThan := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	cursorAt := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)

	// A candidate has at least one link and no link that keeps it: one of a
	// type outside the allowlist, one with an empty entity id, one younger than
	// the cutoff, or one whose entity still exists.
	head := "SELECT f.* FROM files f WHERE f.deleted_at IS NULL AND f.created_at < $1 AND f.updated_at < $2 AND "
	links := func(cutoff string) string {
		return "EXISTS (SELECT 1 FROM file_links l WHERE l.file_id = f.id) " +
			"AND NOT EXISTS (SELECT 1 FROM file_links l WHERE l.file_id = f.id AND (" +
			"l.entity_type NOT IN ('commodity', 'area', 'location') OR l.entity_id = '' OR l.created_at >= " + cutoff + " " +
			"OR (l.entity_type = 'commodity' AND EXISTS (SELECT 1 FROM commodities c WHERE c.id = l.entity_id)) " +
			"OR (l.entity_type = 'area' AND EXISTS (SELECT 1 FROM areas a WHERE a.id = l.entity_id)) " +
			"OR (l.entity_type = 'location' AND EXISTS (SELECT 1 FROM locations lo WHERE lo.id = l.entity_id)))) " +
			"ORDER BY f.created_at ASC, f.id ASC "
	}

	for _, tc := range []struct {
		after registry.OrphanCandidateCursor
		want  sqlCase
	}{
		{
			want: sqlCase{
				name: "first page",
				sql:  head + links("$3") + "LIMIT $4",
				args: []any{olderThan, olderThan, olderThan, 25},
			},
		},
		{
			after: registry.OrphanCandidateCursor{CreatedAt: cursorAt, ID: "file-9"},
			want: sqlCase{
				name: "resumes after the cursor",
				sql:  head + "(f.created_at, f.id) > ($3, $4) AND " + links("$5") + "LIMIT $6",
				args: []any{olderThan, olderThan, cursorAt, "file-9", olderThan, 25},
			},
		},
	} {
		t.Run(tc.want.name, func(t *testing.T) {
			got, args := orphanCandidatesQuery(store.DefaultTableNames, olderThan, tc.after, 25)
			checkSQL(qt.New(t), got, args, tc.want)
		})
	}
}
