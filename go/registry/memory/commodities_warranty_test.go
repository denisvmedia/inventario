package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/go-extras/go-kit/must"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// TestCommodityRegistry_ListPaginated_WarrantyFilter pins the wiring of
// WarrantyStatuses + WarrantyExpiresBefore in CommodityListOptions to the
// computed status produced by models.ComputeWarrantyStatus. Regression
// guard for the FE-facing filter shape — the parameters are documented
// public API surface.
func TestCommodityRegistry_ListPaginated_WarrantyFilter(t *testing.T) {
	c := qt.New(t)
	ctx, regSet, areaID := newCommodityWarrantyFixture(c)

	// Frozen "now" so the test is independent of wall-clock drift.
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)

	mk := func(name string, expires *string) *models.Commodity {
		c.Helper()
		commodity := models.Commodity{
			AreaID:    areaID,
			Name:      name,
			ShortName: name,
			Status:    models.CommodityStatusInUse,
			Type:      models.CommodityTypeOther,
			Count:     1,
		}
		if expires != nil {
			d := models.Date(*expires)
			commodity.WarrantyExpiresAt = &d
		}
		out, err := regSet.CommodityRegistry.Create(ctx, commodity)
		c.Assert(err, qt.IsNil)
		return out
	}

	expiringDate := "2026-06-15" // 40 days out → "expiring" (<=60 days)
	activeDate := "2027-01-01"   // way out → "active"
	expiredDate := "2026-04-01"  // gone → "expired"

	mk("none", nil)
	mk("expiring", &expiringDate)
	mk("active", &activeDate)
	mk("expired", &expiredDate)

	tests := []struct {
		name   string
		opts   registry.CommodityListOptions
		expect []string
	}{
		{
			name: "filter active",
			opts: registry.CommodityListOptions{
				IncludeInactive:  true,
				WarrantyStatuses: []registry.WarrantyStatusFilter{registry.WarrantyStatusFilterActive},
				WarrantyNow:      now,
			},
			expect: []string{"active"},
		},
		{
			name: "filter expiring",
			opts: registry.CommodityListOptions{
				IncludeInactive:  true,
				WarrantyStatuses: []registry.WarrantyStatusFilter{registry.WarrantyStatusFilterExpiring},
				WarrantyNow:      now,
			},
			expect: []string{"expiring"},
		},
		{
			name: "filter expired",
			opts: registry.CommodityListOptions{
				IncludeInactive:  true,
				WarrantyStatuses: []registry.WarrantyStatusFilter{registry.WarrantyStatusFilterExpired},
				WarrantyNow:      now,
			},
			expect: []string{"expired"},
		},
		{
			name: "filter none",
			opts: registry.CommodityListOptions{
				IncludeInactive:  true,
				WarrantyStatuses: []registry.WarrantyStatusFilter{registry.WarrantyStatusFilterNone},
				WarrantyNow:      now,
			},
			expect: []string{"none"},
		},
		{
			name: "expires_before cutoff",
			opts: registry.CommodityListOptions{
				IncludeInactive:       true,
				WarrantyExpiresBefore: "2026-07-01",
				WarrantyNow:           now,
			},
			expect: []string{"expired", "expiring"},
		},
	}
	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			items, _, err := regSet.CommodityRegistry.ListPaginated(ctx, 0, 100, tc.opts)
			c.Assert(err, qt.IsNil)
			names := make([]string, len(items))
			for i, it := range items {
				names[i] = it.Name
			}
			c.Assert(names, qt.ContentEquals, tc.expect)
		})
	}
}

// TestComputeWarrantyStatus pins the boundary semantics for the
// computed warranty status — both ends of the "expiring" window are
// closed, expired starts strictly before today, none is the absence of
// a date.
func TestComputeWarrantyStatus(t *testing.T) {
	c := qt.New(t)
	now := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)

	mkDate := func(s string) models.PDate {
		d := models.Date(s)
		return &d
	}

	tests := []struct {
		name string
		date models.PDate
		want models.WarrantyStatus
	}{
		{"nil → none", nil, models.WarrantyStatusNone},
		{"empty → none", mkDate(""), models.WarrantyStatusNone},
		{"yesterday → expired", mkDate("2026-05-05"), models.WarrantyStatusExpired},
		{"today → expiring", mkDate("2026-05-06"), models.WarrantyStatusExpiring},
		{"60 days from today → expiring", mkDate("2026-07-05"), models.WarrantyStatusExpiring},
		{"61 days from today → active", mkDate("2026-07-06"), models.WarrantyStatusActive},
		{"far future → active", mkDate("2030-01-01"), models.WarrantyStatusActive},
	}
	for _, tc := range tests {
		c.Run(tc.name, func(c *qt.C) {
			got := models.ComputeWarrantyStatus(tc.date, now)
			c.Assert(got, qt.Equals, tc.want)
		})
	}
}

// newCommodityWarrantyFixture sets up a memory registry with an area
// ready to host commodities. Mirrors getCommodityRegistry but exposes
// the registry set + area id rather than a sample commodity.
func newCommodityWarrantyFixture(c *qt.C) (context.Context, *registry.Set, string) {
	c.Helper()
	factorySet := memory.NewFactorySet()
	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			EntityID: models.EntityID{ID: "warr-user"},
			TenantID: "warr-tenant",
		},
		Email: "warr@example.com",
		Name:  "Warranty Test User",
	}
	userReg := factorySet.CreateServiceRegistrySet().UserRegistry
	u, err := userReg.Create(context.Background(), user)
	c.Assert(err, qt.IsNil)

	ctx := appctx.WithUser(context.Background(), u)
	regSet := must.Must(factorySet.CreateUserRegistrySet(ctx))

	loc, err := regSet.LocationRegistry.Create(ctx, models.Location{Name: "L1"})
	c.Assert(err, qt.IsNil)
	area, err := regSet.AreaRegistry.Create(ctx, models.Area{Name: "A1", LocationID: loc.ID})
	c.Assert(err, qt.IsNil)
	return ctx, regSet, area.ID
}
