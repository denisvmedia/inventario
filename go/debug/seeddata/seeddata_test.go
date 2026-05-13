package seeddata_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSeedData(t *testing.T) {
	c := qt.New(t)

	// Create an in-memory registry for testing
	factorySet := memory.NewFactorySet()

	// Test that seed data creation works without errors
	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsFalse)

	// Verify that a tenant was created
	registrySet := factorySet.CreateServiceRegistrySet()
	ctx := context.Background()
	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.HasLen, 1)

	tenant := tenants[0]
	c.Assert(tenant.Name, qt.Equals, "Test Organization")
	c.Assert(tenant.Slug, qt.Equals, "test-org")
	c.Assert(tenant.Status, qt.Equals, models.TenantStatusActive)

	// Five well-known users land in the test-org tenant:
	//   admin / user2 — currency-different default groups (CZK + EUR);
	//                   user2 stays in its OWN group so user-isolation
	//                   e2e specs keep working.
	//   orphan       — zero memberships (no-group fixture, issue #1277).
	//   family       — owns the second seeded group (#1658 multi-group demo).
	//   teammate     — second member of admin's primary group (#1658
	//                   multi-member demo). Lives apart from user2 by
	//                   design.
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 5)

	for _, user := range users {
		c.Assert(user.TenantID, qt.Equals, tenant.ID)
	}

	emails := map[string]*models.User{}
	for _, u := range users {
		emails[u.Email] = u
	}
	c.Assert(emails["admin@test-org.com"], qt.IsNotNil)
	c.Assert(emails["admin@test-org.com"].Name, qt.Equals, "Test Administrator")
	c.Assert(emails["admin@test-org.com"].IsActive, qt.IsTrue)

	c.Assert(emails["user2@test-org.com"], qt.IsNotNil)
	c.Assert(emails["user2@test-org.com"].Name, qt.Equals, "Test User 2")
	c.Assert(emails["user2@test-org.com"].IsActive, qt.IsTrue)

	c.Assert(emails["family@test-org.com"], qt.IsNotNil)
	c.Assert(emails["family@test-org.com"].IsActive, qt.IsTrue)

	c.Assert(emails["teammate@test-org.com"], qt.IsNotNil)
	c.Assert(emails["teammate@test-org.com"].IsActive, qt.IsTrue)

	// Orphan must be active so it can authenticate, but must hold zero
	// group memberships so e2e tests exercise the real `/api/v1/groups`
	// empty-collection response (issue #1277).
	orphan := emails["orphan@test-org.com"]
	c.Assert(orphan, qt.IsNotNil)
	c.Assert(orphan.IsActive, qt.IsTrue)
	memberships, err := registrySet.GroupMembershipRegistry.ListByUser(ctx, tenant.ID, orphan.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(memberships, qt.HasLen, 0)
}

// TestSeedDataSurfaceCoverage asserts that every feature surface called
// out by issue #1658 has at least one bundled fixture in the in-memory
// dataset — the regression-net for "did we accidentally shrink the
// seed?".
func TestSeedDataSurfaceCoverage(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsFalse)

	registrySet := factorySet.CreateServiceRegistrySet()
	ctx := context.Background()

	// Inventory tree.
	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, 3, qt.Commentf("expected 3 seeded locations"))

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(areas) >= 10, qt.IsTrue, qt.Commentf("expected ≥10 areas, got %d", len(areas)))

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(commodities) >= 30, qt.IsTrue, qt.Commentf("expected ≥30 commodities, got %d", len(commodities)))

	// Warranty bucket distribution — at least one row in each bucket
	// (none / active / expiring / expired). Computed live against
	// model logic.
	warrantyBuckets := map[models.WarrantyStatus]int{}
	now := referenceNow()
	statusMix := map[models.CommodityStatus]int{}
	for _, com := range commodities {
		warrantyBuckets[models.ComputeWarrantyStatus(com.WarrantyExpiresAt, now)]++
		statusMix[com.Status]++
	}
	c.Assert(warrantyBuckets[models.WarrantyStatusActive] >= 1, qt.IsTrue, qt.Commentf("≥1 active warranty"))
	c.Assert(warrantyBuckets[models.WarrantyStatusExpiring] >= 1, qt.IsTrue, qt.Commentf("≥1 expiring warranty"))
	c.Assert(warrantyBuckets[models.WarrantyStatusExpired] >= 1, qt.IsTrue, qt.Commentf("≥1 expired warranty"))
	c.Assert(warrantyBuckets[models.WarrantyStatusNone] >= 1, qt.IsTrue, qt.Commentf("≥1 no-warranty row"))

	// Status mix — at least one inactive variant so the Inactive
	// toggle has filter content.
	inactiveCount := statusMix[models.CommodityStatusSold] +
		statusMix[models.CommodityStatusLost] +
		statusMix[models.CommodityStatusDisposed] +
		statusMix[models.CommodityStatusWrittenOff]
	c.Assert(inactiveCount >= 1, qt.IsTrue, qt.Commentf("≥1 inactive (sold/lost/disposed/written_off)"))

	// Tag catalogue.
	tags, err := registrySet.TagRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(tags) >= 8, qt.IsTrue, qt.Commentf("≥8 tags in catalogue, got %d", len(tags)))

	// Files — at least one per category that the issue calls out.
	files, err := registrySet.FileRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	categoryMix := map[models.FileCategory]int{}
	for _, f := range files {
		categoryMix[f.Category]++
	}
	c.Assert(categoryMix[models.FileCategoryImages] >= len(commodities),
		qt.IsTrue, qt.Commentf("every commodity has ≥1 photo (got %d images, %d commodities)", categoryMix[models.FileCategoryImages], len(commodities)))
	c.Assert(categoryMix[models.FileCategoryInvoices] >= 5,
		qt.IsTrue, qt.Commentf("≥5 invoice files, got %d", categoryMix[models.FileCategoryInvoices]))
	c.Assert(categoryMix[models.FileCategoryDocuments] >= 1,
		qt.IsTrue, qt.Commentf("≥1 manual/document file"))

	// Loans — open / overdue / returned mix.
	loans, _, err := registrySet.CommodityLoanRegistry.ListPaginated(ctx, 0, 1000, registry.LoanListOptions{})
	c.Assert(err, qt.IsNil)
	openLoans, returnedLoans := 0, 0
	overdueLoans := 0
	for _, l := range loans {
		if l.IsOpen() {
			openLoans++
		} else {
			returnedLoans++
		}
		if l.IsOverdue(now) {
			overdueLoans++
		}
	}
	c.Assert(openLoans >= 3, qt.IsTrue, qt.Commentf("≥3 open loans, got %d", openLoans))
	c.Assert(overdueLoans >= 1, qt.IsTrue, qt.Commentf("≥1 overdue loan, got %d", overdueLoans))
	c.Assert(returnedLoans >= 2, qt.IsTrue, qt.Commentf("≥2 returned loans, got %d", returnedLoans))

	// Services.
	services, _, err := registrySet.CommodityServiceRegistry.ListPaginated(ctx, 0, 1000, registry.ServiceListOptions{})
	c.Assert(err, qt.IsNil)
	openSvc, completedSvc := 0, 0
	for _, s := range services {
		if s.IsOpen() {
			openSvc++
		} else {
			completedSvc++
		}
	}
	c.Assert(openSvc >= 2, qt.IsTrue, qt.Commentf("≥2 active services, got %d", openSvc))
	c.Assert(completedSvc >= 2, qt.IsTrue, qt.Commentf("≥2 completed services, got %d", completedSvc))

	// Group membership: primary group has ≥2 members; the Family group exists.
	groups, err := registrySet.LocationGroupRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(groups) >= 2, qt.IsTrue, qt.Commentf("expected ≥2 groups (Default + Family), got %d", len(groups)))

	// At least one group has ≥2 active members.
	multiMember := false
	for _, g := range groups {
		members, err := registrySet.GroupMembershipRegistry.ListByGroup(ctx, g.ID)
		c.Assert(err, qt.IsNil)
		if len(members) >= 2 {
			multiMember = true
			break
		}
	}
	c.Assert(multiMember, qt.IsTrue, qt.Commentf("expected ≥1 group with ≥2 members"))

	// Regression net for #1658-r1: admin and user2 MUST live in
	// different groups so user-isolation.spec.ts keeps working
	// (admin → CZK Default, user2 → EUR Default). The multi-member
	// fixture is filled by the dedicated `teammate@test-org.com`
	// user, NOT by user2.
	allUsers, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	var admin, user2 *models.User
	for _, u := range allUsers {
		switch u.Email {
		case "admin@test-org.com":
			admin = u
		case "user2@test-org.com":
			user2 = u
		}
	}
	c.Assert(admin, qt.IsNotNil)
	c.Assert(user2, qt.IsNotNil)
	c.Assert(admin.DefaultGroupID, qt.IsNotNil, qt.Commentf("admin must have a default group"))
	c.Assert(user2.DefaultGroupID, qt.IsNotNil, qt.Commentf("user2 must have a default group"))
	c.Assert(*admin.DefaultGroupID, qt.Not(qt.Equals), *user2.DefaultGroupID,
		qt.Commentf("admin and user2 must be in DIFFERENT default groups to keep user-isolation.spec.ts green"))

	user2InAdminGroup, _ := registrySet.GroupMembershipRegistry.GetByGroupAndUser(ctx, *admin.DefaultGroupID, user2.ID)
	c.Assert(user2InAdminGroup, qt.IsNil,
		qt.Commentf("user2 must NOT be a member of admin's default group — user-isolation specs depend on it"))

	// Pending invite.
	invites, err := registrySet.GroupInviteRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(invites) >= 1, qt.IsTrue, qt.Commentf("≥1 pending group invite, got %d", len(invites)))

	// Exports + restores.
	exports, err := registrySet.ExportRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(exports) >= 1, qt.IsTrue, qt.Commentf("≥1 export record, got %d", len(exports)))
	restores, err := registrySet.RestoreOperationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(restores) >= 1, qt.IsTrue, qt.Commentf("≥1 restore record, got %d", len(restores)))

	// Commodity events + audit log.
	events, err := registrySet.CommodityEventRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(events) >= 10, qt.IsTrue, qt.Commentf("≥10 commodity events for #1653 activity tab, got %d", len(events)))

	auditLogs, err := registrySet.AuditLogRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(len(auditLogs) >= 5, qt.IsTrue, qt.Commentf("≥5 audit logs, got %d", len(auditLogs)))
}

func TestSeedDataIdempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()

	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsFalse)

	registrySet := factorySet.CreateServiceRegistrySet()
	locationsAfterFirst, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	areasAfterFirst, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	commoditiesAfterFirst, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	tagsAfterFirst, err := registrySet.TagRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	filesAfterFirst, err := registrySet.FileRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	loansAfterFirst, _, err := registrySet.CommodityLoanRegistry.ListPaginated(ctx, 0, 1000, registry.LoanListOptions{})
	c.Assert(err, qt.IsNil)

	alreadySeeded, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsTrue)

	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.HasLen, 1)

	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 5)

	locations, err := registrySet.LocationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(locations, qt.HasLen, len(locationsAfterFirst))

	areas, err := registrySet.AreaRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(areas, qt.HasLen, len(areasAfterFirst))

	commodities, err := registrySet.CommodityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(commodities, qt.HasLen, len(commoditiesAfterFirst))

	tags, err := registrySet.TagRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(tags, qt.HasLen, len(tagsAfterFirst))

	files, err := registrySet.FileRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(files, qt.HasLen, len(filesAfterFirst))

	loans, _, err := registrySet.CommodityLoanRegistry.ListPaginated(ctx, 0, 1000, registry.LoanListOptions{})
	c.Assert(err, qt.IsNil)
	c.Assert(loans, qt.HasLen, len(loansAfterFirst))
}

// TestSeedDataDoesNotCreateFixturesInNonTestTenant guards the
// security gate on the well-known-password fixture users: orphan,
// user2 (when planted via the email path), and family. They must
// never land outside the test-org tenant.
func TestSeedDataDoesNotCreateFixturesInNonTestTenant(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()

	_, err := registrySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Acme Corp",
		Slug:   "acme",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	_, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{TenantSlug: "acme"})
	c.Assert(err, qt.IsNil)

	users, err := registrySet.UserRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	for _, u := range users {
		c.Assert(u.Email, qt.Not(qt.Equals), "orphan@test-org.com")
		c.Assert(u.Email, qt.Not(qt.Equals), "family@test-org.com")
		c.Assert(u.Email, qt.Not(qt.Equals), "teammate@test-org.com")
	}
}

// referenceNow returns the wall-clock value used by warranty-bucket
// assertions; mirrors the seed's relative-date computation so a
// commodity seeded with WarrantyDaysFromNow=5 lands in the "expiring"
// bucket when computed at test time.
func referenceNow() time.Time { return time.Now() }
