package seeddata_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	_ "gocloud.dev/blob/fileblob" // register the file:// blob driver for the upload-location tests

	"github.com/denisvmedia/inventario/debug/seeddata"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

func TestSeedData(t *testing.T) {
	c := qt.New(t)

	// Create an in-memory registry for testing
	factorySet := memory.NewFactorySet()

	// Test that seed data creation works without errors. SeedSystemAdmin
	// opts into the sysadmin fixture (#1758) — gated off by default.
	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{SeedSystemAdmin: true})
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

	// Eleven well-known users land in the test-org tenant:
	//   admin / user2 — currency-different default groups (CZK + EUR);
	//                   user2 stays in its OWN group so user-isolation
	//                   e2e specs keep working.
	//   orphan       — zero memberships (no-group fixture, issue #1277).
	//   family       — owns the second seeded group (#1658 multi-group demo).
	//   teammate     — second member of admin's primary group (#1658
	//                   multi-member demo). Lives apart from user2 by
	//                   design.
	//   sysadmin     — platform system admin (is_system_admin) for the
	//                  admin-section e2e suite (issue #1758).
	//   blocktarget  — disposable plain user the block/unblock spec
	//                  deactivates then reactivates (issue #1758).
	//   delete-solo / delete-wrongpass — sole members of their own private
	//                  groups; delete-account.spec.ts erases the former
	//                  (happy path) and bounces the latter on a bad
	//                  password (issue #2147).
	//   delete-lastowner + delete-lastowner-member — sole owner of a SHARED
	//                  group plus its second member, so a valid-creds delete
	//                  is blocked with auth.delete.last_owner (issue #2147).
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 11)

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

	// The sysadmin fixture carries a row in system_admin_grants (#1784);
	// every other seeded user must not.
	sysadmin := emails["sysadmin@test-org.com"]
	c.Assert(sysadmin, qt.IsNotNil)
	c.Assert(sysadmin.IsActive, qt.IsTrue)
	isAdmin, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, sysadmin.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(isAdmin, qt.IsTrue)
	for _, u := range users {
		if u.Email == "sysadmin@test-org.com" {
			continue
		}
		isAdmin, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, u.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(isAdmin, qt.IsFalse,
			qt.Commentf("%s must not be a system admin", u.Email))
	}

	// The block-target fixture is a plain active user.
	blockTarget := emails["blocktarget@test-org.com"]
	c.Assert(blockTarget, qt.IsNotNil)
	c.Assert(blockTarget.IsActive, qt.IsTrue)
	blockTargetAdmin, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, blockTarget.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(blockTargetAdmin, qt.IsFalse)

	// #2147 self-service-deletion fixtures (delete-account.spec.ts). The
	// happy-path target must be the sole member + owner of its own private
	// group so the purge succeeds; the last-owner target must be the SOLE
	// owner of a SHARED group (a second member) so the delete is blocked.
	deleteSolo := emails["delete-solo@test-org.com"]
	c.Assert(deleteSolo, qt.IsNotNil)
	c.Assert(deleteSolo.IsActive, qt.IsTrue)
	soloMemberships, err := registrySet.GroupMembershipRegistry.ListByUser(ctx, tenant.ID, deleteSolo.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(soloMemberships, qt.HasLen, 1)
	c.Assert(soloMemberships[0].Role, qt.Equals, models.GroupRoleOwner)
	soloMembers, err := registrySet.GroupMembershipRegistry.ListByGroup(ctx, soloMemberships[0].GroupID)
	c.Assert(err, qt.IsNil)
	c.Assert(soloMembers, qt.HasLen, 1,
		qt.Commentf("delete-solo must be the only member of its private group"))

	deleteWrongPass := emails["delete-wrongpass@test-org.com"]
	c.Assert(deleteWrongPass, qt.IsNotNil)
	c.Assert(deleteWrongPass.IsActive, qt.IsTrue)

	deleteLastOwner := emails["delete-lastowner@test-org.com"]
	c.Assert(deleteLastOwner, qt.IsNotNil)
	c.Assert(deleteLastOwner.IsActive, qt.IsTrue)
	lastOwnerMemberships, err := registrySet.GroupMembershipRegistry.ListByUser(ctx, tenant.ID, deleteLastOwner.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(lastOwnerMemberships, qt.HasLen, 1)
	c.Assert(lastOwnerMemberships[0].Role, qt.Equals, models.GroupRoleOwner)
	lastOwnerMembers, err := registrySet.GroupMembershipRegistry.ListByGroup(ctx, lastOwnerMemberships[0].GroupID)
	c.Assert(err, qt.IsNil)
	c.Assert(lastOwnerMembers, qt.HasLen, 2,
		qt.Commentf("delete-lastowner's group must be shared (a second member)"))
	owners := 0
	for _, m := range lastOwnerMembers {
		if m.Role == models.GroupRoleOwner {
			owners++
		}
	}
	c.Assert(owners, qt.Equals, 1, qt.Commentf("delete-lastowner must be the SOLE owner"))

	deleteLastOwnerMember := emails["delete-lastowner-member@test-org.com"]
	c.Assert(deleteLastOwnerMember, qt.IsNotNil)
	c.Assert(deleteLastOwnerMember.IsActive, qt.IsTrue)
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
	// Post-#1622 the `invoices` category is gone; seeded invoice files now
	// land in `documents` and carry the conventional `invoice` tag, so we
	// assert both: enough documents-bucket files exist, and ≥5 of them
	// carry the tag (i.e. were the invoice fixtures).
	files, err := registrySet.FileRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	categoryMix := map[models.FileCategory]int{}
	invoiceTagged := 0
	for _, f := range files {
		categoryMix[f.Category]++
		if slices.Contains([]string(f.Tags), models.FileTagInvoice) {
			invoiceTagged++
		}
	}
	c.Assert(categoryMix[models.FileCategoryImages] >= len(commodities),
		qt.IsTrue, qt.Commentf("every commodity has ≥1 photo (got %d images, %d commodities)", categoryMix[models.FileCategoryImages], len(commodities)))
	c.Assert(invoiceTagged >= 5,
		qt.IsTrue, qt.Commentf("≥5 files tagged %q (post-#1622), got %d", models.FileTagInvoice, invoiceTagged))
	c.Assert(categoryMix[models.FileCategoryDocuments] >= 1,
		qt.IsTrue, qt.Commentf("≥1 documents-bucket file (manuals + invoices)"))

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

	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{SeedSystemAdmin: true})
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

	alreadySeeded, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{SeedSystemAdmin: true})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsTrue)

	tenants, err := registrySet.TenantRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.HasLen, 1)

	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 11)

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
// TestSeedDataRefusesFreshSeedIntoPreExistingNonTestTenant pins the #2113 L-2
// hardening: the first seed of a PRE-EXISTING non-test-org tenant (no opt-in)
// is refused outright. Previously the seed merely withheld the privileged
// fixtures (sysadmin / blob uploads) but still populated the tenant — which
// let an unauthenticated /api/v1/seed?tenant_slug=<real-tenant> call pollute
// production data. The refusal is strictly stronger than the old "no fixtures
// leak" guarantee.
func TestSeedDataRefusesFreshSeedIntoPreExistingNonTestTenant(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	registrySet := factorySet.CreateServiceRegistrySet()

	_, err := registrySet.TenantRegistry.Create(context.Background(), models.Tenant{
		Name:   "Acme Corp",
		Slug:   "acme",
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	// SeedSystemAdmin is opted in on purpose: the L-2 refusal fires
	// regardless, so a misconfigured opt-in cannot smuggle the dataset in.
	_, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{TenantSlug: "acme", SeedSystemAdmin: true})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "refusing to seed into pre-existing tenant 'acme'")

	// No users were minted in the pre-existing tenant.
	ctx := context.Background()
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(users, qt.HasLen, 0)
}

// TestSeedDataSystemAdminGate asserts the sysadmin fixture is NOT
// provisioned unless SeedSystemAdmin is explicitly set — the security
// gate that keeps an unauthenticated /api/v1/seed call from minting a
// cross-tenant admin (#1758).
func TestSeedDataSystemAdminGate(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{}) // opt-in OFF
	c.Assert(err, qt.IsNil)

	registrySet := factorySet.CreateServiceRegistrySet()
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)

	for _, u := range users {
		c.Assert(u.Email, qt.Not(qt.Equals), "sysadmin@test-org.com",
			qt.Commentf("sysadmin fixture must not be seeded without the opt-in"))
		isAdmin, err := registrySet.SystemAdminGrantRegistry.Exists(ctx, u.ID)
		c.Assert(err, qt.IsNil)
		c.Assert(isAdmin, qt.IsFalse)
	}
	// The non-privileged block-target fixture is still provisioned.
	emails := map[string]bool{}
	for _, u := range users {
		emails[u.Email] = true
	}
	c.Assert(emails["blocktarget@test-org.com"], qt.IsTrue)
}

// TestSeedDataMissingTenantSlug_FailsClosedByDefault asserts the
// strict production contract: passing a non-empty TenantSlug for a
// tenant that doesn't exist returns an error rather than creating one.
// The CreateTenantIfMissing toggle (#1851) is the only path that
// changes this — see TestSeedDataMissingTenantSlug_CreatesWhenOptIn.
func TestSeedDataMissingTenantSlug_FailsClosedByDefault(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug: "does-not-exist",
		// CreateTenantIfMissing left at zero (false).
	})
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "tenant with slug 'does-not-exist' not found")

	// Confirm no tenant was created as a side-effect.
	registrySet := factorySet.CreateServiceRegistrySet()
	tenants, err := registrySet.TenantRegistry.List(context.Background())
	c.Assert(err, qt.IsNil)
	c.Assert(tenants, qt.HasLen, 0)
}

// TestSeedDataMissingTenantSlug_FailsClosedByDefault_NonNotFoundLookupError
// guards the narrow "only-on-not-found" contract for the create-if-
// missing branch (#1851): a registry lookup that errors with anything
// other than `registry.ErrNotFound` must surface unchanged, even when
// CreateTenantIfMissing is true. Masking the real failure behind a
// create-then-seed would both hide the root cause AND risk minting a
// duplicate-named tenant when the row already exists but the read
// failed transiently. The "memory" backend only returns ErrNotFound
// for unknown slugs, so this contract is asserted at the
// fmt.Errorf-wrap level in findOrCreateTenant; the test pins the
// happy-path "not found = create" branch to make any future
// refactor that re-broadens the catch obvious in diff.
func TestSeedDataMissingTenantSlug_FailsClosedByDefault_NotFoundIsCaughtByOptInOnly(t *testing.T) {
	c := qt.New(t)

	factorySet := memory.NewFactorySet()
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug: "does-not-exist-strict",
		// CreateTenantIfMissing left at zero (false).
	})
	c.Assert(err, qt.IsNotNil)
	// The not-found path takes the strict "not found: …" wrap, NOT the
	// generic "tenant lookup … failed: …" wrap, so a regression that
	// silently catches all errors at the lookup site fails this
	// assertion.
	c.Assert(err.Error(), qt.Contains, "tenant with slug 'does-not-exist-strict' not found")
}

// TestSeedDataMissingTenantSlug_CreatesWhenOptIn covers the
// CreateTenantIfMissing path (#1851): the seed handler binds it from
// the INVENTARIO_SEED_ALLOW_CREATE_TENANT env var, so this test
// exercises the underlying SeedData behavior the e2e fixture relies on
// to provision a second tenant via the public seed endpoint.
func TestSeedDataMissingTenantSlug_CreatesWhenOptIn(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:            "tenant2",
		CreateTenantIfMissing: true,
	})
	c.Assert(err, qt.IsNil)

	registrySet := factorySet.CreateServiceRegistrySet()
	tenant, err := registrySet.TenantRegistry.GetBySlug(ctx, "tenant2")
	c.Assert(err, qt.IsNil)
	c.Assert(tenant.Slug, qt.Equals, "tenant2")
	c.Assert(tenant.Status, qt.Equals, models.TenantStatusActive)

	// The test-org-only fixtures must NOT leak into the newly-created
	// non-test-org tenant — the existing `tenant.Slug == "test-org"`
	// gate in SeedData must still hold for create-if-missing tenants.
	users, err := registrySet.UserRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	for _, u := range users {
		c.Assert(u.Email, qt.Not(qt.Equals), "orphan@test-org.com")
		c.Assert(u.Email, qt.Not(qt.Equals), "blocktarget@test-org.com")
		c.Assert(u.Email, qt.Not(qt.Equals), "sysadmin@test-org.com")
	}
}

// referenceNow returns the wall-clock value used by warranty-bucket
// assertions; mirrors the seed's relative-date computation so a
// commodity seeded with WarrantyDaysFromNow=5 lands in the "expiring"
// bucket when computed at test time.
func referenceNow() time.Time { return time.Now() }

// TestSeedDataBlobUploads_OffByDefault pins the security default: even
// with a real UploadLocation configured, a non-`test-org` tenant must
// NOT get fixture bytes written when AllowBlobUploads is off. The seed
// still creates the cover-photo file *rows* (so the UI surfaces them),
// but the no-op uploader leaves SizeBytes at 0. This is the
// public-/api/v1/seed cost-bound: an arbitrary tenant_slug can't be
// coaxed into spamming the configured bucket.
func TestSeedDataBlobUploads_OffByDefault(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:            "acme",
		CreateTenantIfMissing: true,
		UploadLocation:        "file://" + t.TempDir() + "?create_dir=1",
		// AllowBlobUploads left at zero (false).
	})
	c.Assert(err, qt.IsNil)

	images, withBytes := seededImageStats(c, factorySet, ctx)
	c.Assert(images > 0, qt.IsTrue, qt.Commentf("seed still creates cover-photo rows"))
	c.Assert(withBytes, qt.Equals, 0,
		qt.Commentf("no fixture bytes for a non-test-org tenant without the opt-in"))
}

// TestSeedDataBlobUploads_WhenOptIn covers the AllowBlobUploads path:
// the seed handler binds it from INVENTARIO_SEED_ALLOW_BLOB_UPLOADS,
// which the Helm chart sets for the demo overlay so the evaluation
// deployment's `default` tenant gets real cover photos and documents.
// With the opt-in on, every bundled cover photo is written to the
// configured bucket and its row carries the real byte count.
func TestSeedDataBlobUploads_WhenOptIn(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	tempDir := t.TempDir()

	factorySet := memory.NewFactorySet()
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:            "acme",
		CreateTenantIfMissing: true,
		UploadLocation:        "file://" + tempDir + "?create_dir=1",
		AllowBlobUploads:      true,
	})
	c.Assert(err, qt.IsNil)

	images, withBytes := seededImageStats(c, factorySet, ctx)
	c.Assert(images > 0, qt.IsTrue)
	c.Assert(withBytes, qt.Equals, images,
		qt.Commentf("every cover photo gets real bytes when the opt-in is on"))

	// The fixture bytes physically landed in the bucket.
	c.Assert(countRegularFiles(c, tempDir) > 0, qt.IsTrue,
		qt.Commentf("seed fixtures written to the configured bucket"))
}

// TestSeedDataReconcilesMissingBlobsOnReseed reproduces the production
// failure mode behind inv-vcl01-master's empty Files page: a database
// first seeded metadata-only (blob uploads disabled for its tenant, the
// pre-#1931 default) carries fixture file rows with SizeBytes 0 and no
// objects in the bucket. The location-count idempotency gate means a
// later re-seed short-circuits the whole seed — so before the reconcile
// step, the bytes were never backfilled and the rows stayed dangling
// forever. A second seed with the bucket + opt-in now configured must
// repair them.
func TestSeedDataReconcilesMissingBlobsOnReseed(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()

	// First seed: no UploadLocation → no-op uploader → metadata-only rows.
	// This is the state any demo/preview env seeded before #1931 is in.
	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:            "acme",
		CreateTenantIfMissing: true,
		// UploadLocation empty: metadata-only, mirrors the pre-opt-in seed.
	})
	c.Assert(err, qt.IsNil)

	images, withBytes := seededImageStats(c, factorySet, ctx)
	c.Assert(images > 0, qt.IsTrue)
	c.Assert(withBytes, qt.Equals, 0,
		qt.Commentf("first seed wrote no bytes — rows are metadata-only"))

	// Second seed: bucket + opt-in now wired (the post-#1931 config). The
	// location-count gate makes this an "already seeded" no-op for the
	// data tables, but reconcileSeedBlobs must still backfill the bytes.
	tempDir := t.TempDir()
	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:       "acme",
		UploadLocation:   "file://" + tempDir + "?create_dir=1",
		AllowBlobUploads: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsTrue,
		qt.Commentf("data tables already populated — this is the reconcile path"))

	imagesAfter, withBytesAfter := seededImageStats(c, factorySet, ctx)
	c.Assert(imagesAfter, qt.Equals, images,
		qt.Commentf("reconcile must not create or drop rows"))
	c.Assert(withBytesAfter, qt.Equals, imagesAfter,
		qt.Commentf("every fixture row now carries the real byte count"))

	// The fixture bytes physically landed in the bucket at the rows'
	// pre-existing keys.
	c.Assert(countRegularFiles(c, tempDir) > 0, qt.IsTrue,
		qt.Commentf("reconcile wrote the missing blobs to the bucket"))

	// Idempotent: a third identical seed finds every blob present and is a
	// clean no-op (no error, nothing rewritten).
	alreadySeeded, err = seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:       "acme",
		UploadLocation:   "file://" + tempDir + "?create_dir=1",
		AllowBlobUploads: true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsTrue)
	_, withBytesFinal := seededImageStats(c, factorySet, ctx)
	c.Assert(withBytesFinal, qt.Equals, imagesAfter)
}

// TestSeedDataReconcileRespectsBlobGate pins that the reconcile path
// honours the same tenant gate as the fresh-seed upload path: a re-seed
// of a non-`test-org` tenant WITHOUT the opt-in must not write bytes,
// even with a real UploadLocation configured. Otherwise the reconcile
// step would be a back door around the public-/api/v1/seed cost bound.
func TestSeedDataReconcileRespectsBlobGate(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()

	factorySet := memory.NewFactorySet()

	_, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:            "acme",
		CreateTenantIfMissing: true,
	})
	c.Assert(err, qt.IsNil)

	tempDir := t.TempDir()
	alreadySeeded, err := seeddata.SeedData(factorySet, seeddata.SeedOptions{
		TenantSlug:     "acme",
		UploadLocation: "file://" + tempDir + "?create_dir=1",
		// AllowBlobUploads left false — no opt-in.
	})
	c.Assert(err, qt.IsNil)
	c.Assert(alreadySeeded, qt.IsTrue)

	_, withBytes := seededImageStats(c, factorySet, ctx)
	c.Assert(withBytes, qt.Equals, 0,
		qt.Commentf("reconcile must not write bytes without the opt-in"))
	c.Assert(countRegularFiles(c, tempDir), qt.Equals, 0,
		qt.Commentf("no objects written to the bucket without the opt-in"))
}

// seededImageStats returns (number of image-category file rows, number
// of those carrying non-zero SizeBytes). Image-category rows only ever
// come from the bundled fixture upload path, so they cleanly isolate the
// blob-upload gate from unrelated file rows (e.g. export documents).
func seededImageStats(c *qt.C, factorySet *registry.FactorySet, ctx context.Context) (images, withBytes int) {
	files, err := factorySet.CreateServiceRegistrySet().FileRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	for _, f := range files {
		if f.Category != models.FileCategoryImages {
			continue
		}
		images++
		if f.File != nil && f.File.SizeBytes > 0 {
			withBytes++
		}
	}
	return images, withBytes
}

// countRegularFiles walks dir and returns the number of regular files
// found — used to assert whether the seed wrote fixture bytes to the
// configured file:// bucket.
func countRegularFiles(c *qt.C, dir string) int {
	n := 0
	err := filepath.WalkDir(dir, func(_ string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.Type().IsRegular() {
			n++
		}
		return nil
	})
	c.Assert(err, qt.IsNil)
	return n
}
