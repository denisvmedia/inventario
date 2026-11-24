package postgres_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// The tests in this file pin which rows, in which order and with which total,
// the filtered list queries return. They exercise the SQL each registry builds
// from its options, so a change to how that SQL is assembled has to keep the
// same results.

func areaNames(areas []*models.Area) []string {
	out := make([]string, len(areas))
	for i, a := range areas {
		out[i] = a.Name
	}
	return out
}

func TestAreaRegistry_Postgres_ListPaginatedByLocation(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	reg := fx.groupASet.AreaRegistry

	loc, err := fx.groupASet.LocationRegistry.Create(fx.ctxA, models.Location{Name: "Workshop", Address: "addr"})
	c.Assert(err, qt.IsNil)
	for _, name := range []string{"Shelf B", "Shelf A"} {
		_, err := reg.Create(fx.ctxA, models.Area{Name: name, LocationID: loc.GetID()})
		c.Assert(err, qt.IsNil)
	}

	areas, total, err := reg.ListPaginated(fx.ctxA, 0, 10, registry.AreaListOptions{LocationID: loc.GetID()})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 2)
	c.Check(areaNames(areas), qt.DeepEquals, []string{"Shelf A", "Shelf B"})

	areas, total, err = reg.ListPaginated(fx.ctxA, 1, 1, registry.AreaListOptions{LocationID: loc.GetID()})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 2)
	c.Check(areaNames(areas), qt.DeepEquals, []string{"Shelf B"})

	// No location filter: the fixture's own area is listed too.
	_, total, err = reg.ListPaginated(fx.ctxA, 0, 10, registry.AreaListOptions{})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 3)
}

func TestAreaRegistry_Postgres_ListPaginatedByUnknownLocation(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)

	// An empty page takes the separate COUNT path, which must carry the same
	// filter as the page query.
	areas, total, err := fx.groupASet.AreaRegistry.ListPaginated(fx.ctxA, 0, 10, registry.AreaListOptions{LocationID: "no-such-location"})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 0)
	c.Check(areas, qt.HasLen, 0)
}

func TestCommodityEventRegistry_Postgres_ListByCommodityFiltersByKind(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	reg := fx.groupASet.CommodityEventRegistry

	commodityID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Drill")
	otherID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Saw")

	base := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	for i, kind := range []models.CommodityEventKind{
		models.CommodityEventKindCreated,
		models.CommodityEventKindUpdated,
		models.CommodityEventKindMoved,
	} {
		_, err := reg.Create(fx.ctxA, models.CommodityEvent{
			CommodityID: commodityID,
			Kind:        kind,
			OccurredAt:  base.Add(time.Duration(i) * time.Hour),
		})
		c.Assert(err, qt.IsNil)
	}
	_, err := reg.Create(fx.ctxA, models.CommodityEvent{
		CommodityID: otherID,
		Kind:        models.CommodityEventKindUpdated,
		OccurredAt:  base,
	})
	c.Assert(err, qt.IsNil)

	kinds := func(events []*models.CommodityEvent) []models.CommodityEventKind {
		out := make([]models.CommodityEventKind, len(events))
		for i, e := range events {
			out[i] = e.Kind
		}
		return out
	}

	events, total, err := reg.ListByCommodity(fx.ctxA, commodityID, 0, 10, registry.CommodityEventListOptions{})
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 3)
	c.Check(kinds(events), qt.DeepEquals, []models.CommodityEventKind{"moved", "updated", "created"})

	filter := registry.CommodityEventListOptions{Kinds: []models.CommodityEventKind{"updated", "moved"}}
	events, total, err = reg.ListByCommodity(fx.ctxA, commodityID, 0, 10, filter)
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 2)
	c.Check(kinds(events), qt.DeepEquals, []models.CommodityEventKind{"moved", "updated"})

	events, total, err = reg.ListByCommodity(fx.ctxA, commodityID, 1, 1, filter)
	c.Assert(err, qt.IsNil)
	c.Check(total, qt.Equals, 2)
	c.Check(kinds(events), qt.DeepEquals, []models.CommodityEventKind{"updated"})
}

func TestCommodityServiceRegistry_Postgres_StateFilter(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	reg := fx.groupASet.CommodityServiceRegistry
	commodityID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Mower")

	create := func(reason string, sent models.Date, expected, returned models.PDate) {
		c.Helper()
		_, err := reg.Create(fx.ctxA, models.CommodityService{
			CommodityID:      commodityID,
			ProviderName:     "Repair shop",
			Reason:           reason,
			SentAt:           sent,
			ExpectedReturnAt: expected,
			ReturnedAt:       returned,
			CostCurrency:     "USD",
		})
		c.Assert(err, qt.IsNil)
	}
	create("overdue", "2026-04-01", models.ToPDate("2026-04-15"), nil)
	create("open", "2026-05-01", models.ToPDate("2026-06-01"), nil)
	create("completed", "2026-03-01", nil, models.ToPDate("2026-03-10"))

	reasons := func(rows []*models.CommodityService) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = r.Reason
		}
		return out
	}
	now := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)

	for _, tc := range []struct {
		state  registry.ServiceState
		offset int
		limit  int
		total  int
		want   []string
	}{
		{state: registry.ServiceStateAll, limit: 10, total: 3, want: []string{"open", "overdue", "completed"}},
		{state: registry.ServiceStateAll, offset: 1, limit: 1, total: 3, want: []string{"overdue"}},
		{state: registry.ServiceStateOpen, limit: 10, total: 2, want: []string{"open", "overdue"}},
		{state: registry.ServiceStateOverdue, limit: 10, total: 1, want: []string{"overdue"}},
		{state: registry.ServiceStateCompleted, limit: 10, total: 1, want: []string{"completed"}},
	} {
		rows, total, err := reg.ListPaginated(fx.ctxA, tc.offset, tc.limit, registry.ServiceListOptions{State: tc.state, Now: now})
		c.Assert(err, qt.IsNil, qt.Commentf("state %s", tc.state))
		c.Check(total, qt.Equals, tc.total, qt.Commentf("state %s", tc.state))
		c.Check(reasons(rows), qt.DeepEquals, tc.want, qt.Commentf("state %s", tc.state))
	}
}

func TestCommodityLoanRegistry_Postgres_ListPendingReminders(t *testing.T) {
	c := qt.New(t)
	fx := newLoanPGFixture(t)
	reg := fx.groupASet.CommodityLoanRegistry
	serviceReg := fx.factorySet.CreateServiceRegistrySet().CommodityLoanRegistry

	create := func(borrower string, due, returned models.PDate) string {
		c.Helper()
		loan, err := reg.Create(fx.ctxA, models.CommodityLoan{
			CommodityID:  fx.commAID,
			BorrowerName: borrower,
			LentAt:       "2026-04-01",
			DueBackAt:    due,
			ReturnedAt:   returned,
		})
		c.Assert(err, qt.IsNil)
		return loan.ID
	}
	create("late", models.ToPDate("2026-05-01"), nil)
	create("later", models.ToPDate("2026-04-25"), nil)
	create("soon", models.ToPDate("2026-05-12"), nil)
	remindedID := create("reminded", models.ToPDate("2026-05-11"), nil)
	create("far", models.ToPDate("2026-05-20"), nil)
	create("back", models.ToPDate("2026-04-20"), models.ToPDate("2026-04-21"))
	create("no due date", nil, nil)

	sent, err := serviceReg.MarkReminderSent(context.Background(), remindedID, registry.LoanReminderKindDueSoon)
	c.Assert(err, qt.IsNil)
	c.Assert(sent, qt.IsTrue)

	borrowers := func(loans []*models.CommodityLoan) []string {
		out := make([]string, len(loans))
		for i, l := range loans {
			out[i] = l.BorrowerName
		}
		return out
	}
	now := time.Date(2026, 5, 10, 9, 0, 0, 0, time.UTC)

	overdue, err := serviceReg.ListPendingReminders(context.Background(), registry.LoanReminderKindOverdue, now, 3)
	c.Assert(err, qt.IsNil)
	c.Check(borrowers(overdue), qt.DeepEquals, []string{"later", "late"})

	dueSoon, err := serviceReg.ListPendingReminders(context.Background(), registry.LoanReminderKindDueSoon, now, 3)
	c.Assert(err, qt.IsNil)
	c.Check(borrowers(dueSoon), qt.DeepEquals, []string{"soon"})
}

func TestMaintenanceScheduleRegistry_Postgres_ListPaginatedFilters(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	reg := fx.groupASet.MaintenanceScheduleRegistry
	commodityID := seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Car")

	for _, s := range []struct {
		title   string
		due     models.Date
		enabled bool
	}{
		{"Filter", "2026-05-01", true},
		{"Belt", "2026-05-01", true},
		{"Oil", "2026-06-01", false},
		{"Tires", "2026-07-01", true},
	} {
		_, err := reg.Create(fx.ctxA, models.MaintenanceSchedule{
			CommodityID:  commodityID,
			Title:        s.title,
			IntervalDays: 30,
			NextDueAt:    s.due,
			Enabled:      s.enabled,
		})
		c.Assert(err, qt.IsNil)
	}

	titles := func(rows []*models.MaintenanceSchedule) []string {
		out := make([]string, len(rows))
		for i, r := range rows {
			out[i] = r.Title
		}
		return out
	}

	for _, tc := range []struct {
		name   string
		opts   registry.MaintenanceListOptions
		offset int
		total  int
		want   []string
	}{
		{name: "no filter", total: 4, want: []string{"Belt", "Filter", "Oil", "Tires"}},
		{name: "enabled only", opts: registry.MaintenanceListOptions{EnabledOnly: true}, total: 3, want: []string{"Belt", "Filter", "Tires"}},
		{name: "due before is inclusive", opts: registry.MaintenanceListOptions{DueBefore: "2026-06-01"}, total: 3, want: []string{"Belt", "Filter", "Oil"}},
		{name: "both", opts: registry.MaintenanceListOptions{EnabledOnly: true, DueBefore: "2026-06-01"}, total: 2, want: []string{"Belt", "Filter"}},
		{name: "page past the filter", opts: registry.MaintenanceListOptions{EnabledOnly: true}, offset: 2, total: 3, want: []string{"Tires"}},
	} {
		rows, total, err := reg.ListPaginated(fx.ctxA, tc.offset, 10, tc.opts)
		c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
		c.Check(total, qt.Equals, tc.total, qt.Commentf(tc.name))
		c.Check(titles(rows), qt.DeepEquals, tc.want, qt.Commentf(tc.name))
	}
}

func TestTagRegistry_Postgres_ListPaginatedSearchAndSort(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	reg := fx.groupASet.TagRegistry

	for _, tag := range []models.Tag{
		{Kind: models.TagKindCommodity, Slug: "kitchen", Label: "Kitchen"},
		{Kind: models.TagKindCommodity, Slug: "kit-bag", Label: "Kit bag"},
		{Kind: models.TagKindCommodity, Slug: "garage", Label: "Garage"},
		{Kind: models.TagKindFile, Slug: "kitchen", Label: "Kitchen files"},
	} {
		tag.Color = models.DefaultTagColor
		_, err := reg.Create(fx.ctxA, tag)
		c.Assert(err, qt.IsNil)
	}
	seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Pan", "kitchen")
	seedTagCommodity(c, fx.groupASet, fx.ctxA, fx.areaAID, "Knife", "kitchen", "kit-bag")

	labels := func(tags []*models.Tag) []string {
		out := make([]string, len(tags))
		for i, t := range tags {
			out[i] = t.Label
		}
		return out
	}

	for _, tc := range []struct {
		name  string
		opts  registry.TagListOptions
		total int
		want  []string
	}{
		{
			name:  "search matches label or slug within the kind",
			opts:  registry.TagListOptions{Search: "kit", Kind: models.TagKindCommodity},
			total: 2,
			want:  []string{"Kit bag", "Kitchen"},
		},
		{
			name:  "search across kinds",
			opts:  registry.TagListOptions{Search: "kitchen"},
			total: 2,
			want:  []string{"Kitchen", "Kitchen files"},
		},
		{
			name:  "label descending",
			opts:  registry.TagListOptions{Kind: models.TagKindCommodity, SortDesc: true},
			total: 3,
			want:  []string{"Kitchen", "Kit bag", "Garage"},
		},
		{
			name:  "usage descending counts the kind's own table",
			opts:  registry.TagListOptions{Kind: models.TagKindCommodity, SortField: registry.TagSortUsage, SortDesc: true},
			total: 3,
			want:  []string{"Kitchen", "Kit bag", "Garage"},
		},
		{
			name:  "usage ascending",
			opts:  registry.TagListOptions{Kind: models.TagKindCommodity, SortField: registry.TagSortUsage},
			total: 3,
			want:  []string{"Garage", "Kit bag", "Kitchen"},
		},
	} {
		tags, total, err := reg.ListPaginated(fx.ctxA, 0, 10, tc.opts)
		c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
		c.Check(total, qt.Equals, tc.total, qt.Commentf(tc.name))
		c.Check(labels(tags), qt.DeepEquals, tc.want, qt.Commentf(tc.name))
	}

	tags, err := reg.Search(fx.ctxA, "kit", 10, models.TagKindCommodity)
	c.Assert(err, qt.IsNil)
	c.Check(labels(tags), qt.DeepEquals, []string{"Kitchen", "Kit bag"})

	tags, err = reg.Search(fx.ctxA, "", 2, models.TagKindCommodity)
	c.Assert(err, qt.IsNil)
	c.Check(labels(tags), qt.DeepEquals, []string{"Kitchen", "Kit bag"})
}

func TestUserContentOwnershipChecker_Postgres_HasRetainedOwnedContent(t *testing.T) {
	c := qt.New(t)
	fx := newTagPGFixture(t)
	checker := fx.factorySet.UserContentOwnershipChecker

	// The fixture gives the user a location and an area in each of its two
	// groups, and the user created both groups.
	for _, tc := range []struct {
		name   string
		purged []string
		want   bool
	}{
		{name: "nothing purged", purged: nil, want: true},
		{name: "group B retained", purged: []string{fx.groupAID}, want: true},
		{name: "group A retained", purged: []string{fx.groupBID}, want: true},
		{name: "every group purged", purged: []string{fx.groupAID, fx.groupBID}, want: false},
	} {
		got, err := checker.HasRetainedOwnedContent(context.Background(), fx.user.TenantID, fx.user.ID, tc.purged)
		c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
		c.Check(got, qt.Equals, tc.want, qt.Commentf(tc.name))
	}
}

func TestAdminListQueries_Postgres_Filters(t *testing.T) {
	set, _ := setupTestRegistrySet(t)
	c := qt.New(t)
	ctx := context.Background()

	harnessUser := getTestUser(c, set)
	tenantA := harnessUser.TenantID

	tenantB, err := set.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Second Organization",
		Slug:   "second-org",
		Domain: new("widgets.example.com"),
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)
	_, err = set.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Acme Widgets",
		Slug:   "acme",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	for _, u := range []models.User{
		{TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantA}, Email: "carol@test-org.com", Name: "Carol Admin", IsActive: true},
		{TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantA}, Email: "dave@test-org.com", Name: "Dave", IsActive: false},
	} {
		_, err := set.UserRegistry.Create(ctx, u)
		c.Assert(err, qt.IsNil)
	}
	userB, err := set.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantB.ID},
		Email:               "admin@widgets.example.com",
		Name:                "B Admin",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)

	createGroup := func(tenantID, createdBy, name string, status models.LocationGroupStatus) {
		c.Helper()
		slug, err := models.GenerateGroupSlug()
		c.Assert(err, qt.IsNil)
		_, err = set.LocationGroupRegistry.Create(ctx, models.LocationGroup{
			TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
			Name:                name,
			Slug:                slug,
			Status:              status,
			CreatedBy:           createdBy,
			GroupCurrency:       models.Currency("USD"),
		})
		c.Assert(err, qt.IsNil)
	}
	createGroup(tenantA, harnessUser.ID, "Garage Group", models.LocationGroupStatusActive)
	createGroup(tenantB.ID, userB.ID, "Workshop", models.LocationGroupStatusPendingDeletion)

	c.Run("tenants", func(c *qt.C) {
		names := func(items []*registry.AdminTenantListItem) []string {
			out := make([]string, len(items))
			for i, it := range items {
				out[i] = it.Tenant.Name
			}
			return out
		}
		for _, tc := range []struct {
			name  string
			opts  registry.AdminTenantListOptions
			total int
			want  []string
		}{
			{name: "query matches name or domain", opts: registry.AdminTenantListOptions{Query: "widget"}, total: 2, want: []string{"Acme Widgets", "Second Organization"}},
			{name: "descending", opts: registry.AdminTenantListOptions{Query: "widget", SortDesc: true}, total: 2, want: []string{"Second Organization", "Acme Widgets"}},
			{name: "query matches slug", opts: registry.AdminTenantListOptions{Query: "test-org"}, total: 1, want: []string{"Test Organization"}},
			{name: "second page", opts: registry.AdminTenantListOptions{Query: "widget", Page: 2, PerPage: 1}, total: 2, want: []string{"Second Organization"}},
		} {
			items, total, err := set.TenantRegistry.ListAdmin(ctx, tc.opts)
			c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
			c.Check(total, qt.Equals, tc.total, qt.Commentf(tc.name))
			c.Check(names(items), qt.DeepEquals, tc.want, qt.Commentf(tc.name))
		}
	})

	c.Run("users", func(c *qt.C) {
		emails := func(items []*registry.AdminUserListItem) []string {
			out := make([]string, len(items))
			for i, it := range items {
				out[i] = it.User.Email
			}
			return out
		}
		inactive := false
		for _, tc := range []struct {
			name  string
			opts  registry.AdminUserListOptions
			total int
			want  []string
		}{
			{name: "query matches email or name within the tenant", opts: registry.AdminUserListOptions{Query: "admin"}, total: 2, want: []string{"admin@test-org.com", "carol@test-org.com"}},
			{name: "inactive only", opts: registry.AdminUserListOptions{IsActive: &inactive}, total: 1, want: []string{"dave@test-org.com"}},
			{name: "query and inactive", opts: registry.AdminUserListOptions{Query: "admin", IsActive: &inactive}, total: 0, want: []string{}},
			{name: "name descending", opts: registry.AdminUserListOptions{SortField: registry.AdminUserSortName, SortDesc: true}, total: 3, want: []string{"admin@test-org.com", "dave@test-org.com", "carol@test-org.com"}},
		} {
			items, total, err := set.UserRegistry.ListAdminByTenant(ctx, tenantA, tc.opts)
			c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
			c.Check(total, qt.Equals, tc.total, qt.Commentf(tc.name))
			c.Check(emails(items), qt.DeepEquals, tc.want, qt.Commentf(tc.name))
		}
	})

	c.Run("groups", func(c *qt.C) {
		names := func(items []*registry.AdminGroupListItem) []string {
			out := make([]string, len(items))
			for i, it := range items {
				out[i] = it.Group.Name
			}
			return out
		}
		for _, tc := range []struct {
			name  string
			opts  registry.AdminGroupListOptions
			total int
			want  []string
		}{
			{name: "query", opts: registry.AdminGroupListOptions{Query: "group"}, total: 2, want: []string{"Garage Group", "Test Group"}},
			{name: "tenant", opts: registry.AdminGroupListOptions{TenantID: tenantB.ID}, total: 1, want: []string{"Workshop"}},
			{name: "status", opts: registry.AdminGroupListOptions{Status: string(models.LocationGroupStatusPendingDeletion)}, total: 1, want: []string{"Workshop"}},
			{name: "tenant and status descending", opts: registry.AdminGroupListOptions{TenantID: tenantA, Status: "active", SortDesc: true}, total: 2, want: []string{"Test Group", "Garage Group"}},
		} {
			items, total, err := set.LocationGroupRegistry.ListAdmin(ctx, tc.opts)
			c.Assert(err, qt.IsNil, qt.Commentf(tc.name))
			c.Check(total, qt.Equals, tc.total, qt.Commentf(tc.name))
			c.Check(names(items), qt.DeepEquals, tc.want, qt.Commentf(tc.name))
		}
	})
}
