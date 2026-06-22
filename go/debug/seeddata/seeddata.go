// Package seeddata populates a fresh Inventario install with a realistic
// dataset so first-time alpha users, the screenshot harness, and the e2e
// suite all see lived-in content instead of empty-state placeholders.
//
// Surfaces exercised by the seed:
//
//   - Inventory tree — 3 locations / 10 areas / ~35 commodities spread
//     across white-goods, electronics, furniture, clothes, equipment.
//   - Warranty mix — ~40% active, ~15% expiring (≥2 of those expiring
//     in ≤7 days so the reminder worker has something to send), ~15%
//     expired, ~30% none. Drives the Dashboard health bars + #1367
//     /warranties tabs + WarrantyBadge.
//   - First-class tag catalogue (#1397) — 10 group-scoped tag rows with
//     curated colors, attached to commodities with realistic overlap
//     so the /tags page lights up and tag-pill filtering on /files
//     and /commodities has content.
//   - Files (#1538) — every commodity gets a cover photo (real bundled
//     JPG from _files/), ~half also carry an invoice PDF, a handful
//     carry a manual. Cover photo is pinned on ~half so FileCard's
//     star-overlay logic gets exercised. Location-level files seed a
//     couple of "house deed"-style entries.
//   - Loans (#1452) — 3 active + 2 overdue + 2 returned, lent to
//     plausible first-name borrowers so the Lent tab + per-item Lend
//     history both look alive.
//   - Services (#1508) — 2 active workshop dispatches + 2 completed
//     services with cost recorded.
//   - Status mix — at least one of each {sold, lost, disposed,
//     written_off} so the Inactive toggle has filters to apply.
//   - Group membership (#1533) — admin (owner) + 1 'user'-role
//     teammate + 1 pending viewer invite; a second group exists with
//     the admin as a non-owner member so the group switcher dropdown
//     shows more than one row.
//   - Exports / Restores (#1534) — 1 completed export + 1 completed
//     restore so the Backup & Restore page isn't a blank list.
//   - Commodity events (#1450) + audit log — sample timeline entries
//     and a couple of security-style audit rows so #1653's profile
//     activity tab and the security audit views have content.
//
// Idempotency: the location-count gate on user1's group remains in
// place. After it returns "already seeded" the call short-circuits, so
// re-running the seed against the same DB never doubles rows.
package seeddata

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// testOrgTenantSlug is the well-known sentinel slug for the seed's own
// throwaway test tenant. The seed treats it as the ONLY slug it is allowed to
// (re)seed into when the row already exists — every other pre-existing slug is
// presumed to be real (production) data and is refused (issue #2113, L-2).
const testOrgTenantSlug = "test-org"

// SeedOptions contains optional parameters for seeding.
type SeedOptions struct {
	UserEmail  string // Optional: email of user to seed for
	TenantSlug string // Optional: slug of tenant to seed for

	// SeedSystemAdmin opts into provisioning the `sysadmin@test-org.com`
	// fixture with the platform-wide is_system_admin flag (#1758). It is
	// OFF by default: the /api/v1/seed endpoint is unauthenticated, so
	// minting a cross-tenant admin from it would be a privilege-
	// escalation hole. Only the e2e harness sets it (the seed handler
	// reads INVENTARIO_SEED_SYSTEM_ADMIN_FIXTURE). Has effect only for
	// the well-known `test-org` tenant.
	SeedSystemAdmin bool

	// UploadLocation is the gocloud-style blob URL the seed uses to
	// publish bundled file fixtures (photos, invoices, manuals). When
	// empty, the seed still creates file *rows* so the UI shows them,
	// but no bytes are written — useful for in-memory unit tests that
	// don't have a blob bucket attached. Real callers (apiserver/seed
	// handler, init-data) pass the server's configured upload
	// location through verbatim.
	UploadLocation string

	// CreateTenantIfMissing turns the strict "tenant with slug 'X' not
	// found" error into a side-effecting "create the tenant row, then
	// seed inside it" path. Used by the e2e cross-tenant fixture (#1851)
	// to provision a second tenant via the public seed endpoint without
	// needing a CLI shell-out or a back-office admin route. OFF by
	// default; the seed handler reads INVENTARIO_SEED_ALLOW_CREATE_TENANT
	// to flip it on (the env-var-gated pattern matches SeedSystemAdmin
	// — never sourced from the request body). Has effect only when
	// TenantSlug is non-empty.
	CreateTenantIfMissing bool

	// AllowBlobUploads opts a non-`test-org` tenant into having the
	// bundled fixture *bytes* (photos, invoices, manuals) written to the
	// configured UploadLocation, not just the metadata rows. Without it,
	// blob writes are restricted to the well-known `test-org` tenant
	// because /api/v1/seed is unauthenticated and writing real bytes for
	// an arbitrary tenant_slug is an abuse vector (see the gate in
	// SeedData). OFF by default; the seed handler reads
	// INVENTARIO_SEED_ALLOW_BLOB_UPLOADS to flip it on — same env-var-
	// gated pattern as SeedSystemAdmin / CreateTenantIfMissing, never
	// sourced from the request body. The Helm chart sets it for the demo
	// overlay so the evaluation deployment looks lived-in. Has effect
	// only when UploadLocation is non-empty.
	AllowBlobUploads bool
}

// SeedData seeds the database with example data. Returns alreadySeeded=true
// when canonical seed records (locations under user1's group) already
// exist and the call was a no-op for the data layer — POST /api/v1/seed
// relies on this to stay idempotent so re-running it doesn't double
// counts. Tenants/users/groups are still reconciled (find-or-create) so
// that callers reseeding after a wipe of just the data tables still get
// a valid context.
func SeedData(factorySet *registry.FactorySet, opts SeedOptions) (alreadySeeded bool, err error) { //nolint:gocyclo // orchestrator: many sequential boot steps
	slog.Info("Seeding database",
		"user_email", opts.UserEmail,
		"tenant_slug", opts.TenantSlug,
		"upload_location_set", opts.UploadLocation != "",
	)
	ctx := context.Background()
	registrySet := factorySet.CreateServiceRegistrySet()

	// Find or create tenant. tenantPreExisted reports whether the row was
	// already present before this call — the L-2 fresh-seed guard below uses
	// it to refuse the first seed into an unrelated production tenant.
	tenant, tenantPreExisted, err := findOrCreateTenant(ctx, registrySet, opts)
	if err != nil {
		return false, err
	}

	// Get existing users for the tenant.
	users, err := registrySet.UserRegistry.List(ctx)
	if err != nil {
		return false, err
	}

	// Fail-closed (issue #2113, L-2): refuse the first seed into a
	// pre-existing tenant the seed doesn't own. Runs before any rows are
	// created, so a rejected call is a clean no-op. Only pre-existing tenants
	// can be polluted this way; a tenant the seed just created is always fine.
	if tenantPreExisted {
		if err := guardPreExistingTenant(ctx, registrySet, tenant, opts); err != nil {
			return false, err
		}
	}

	user1, user2, err := findOrCreateUsers(ctx, registrySet, tenant, users, opts.UserEmail)
	if err != nil {
		return false, err
	}

	// Inside the well-known `test-org` test tenant, provision the extra
	// fixture users (orphan / block-target / opt-in sysadmin) the e2e
	// suite depends on. Extracted into seedTestOrgFixtures so SeedData
	// itself stays under the gocognit budget.
	if tenant.Slug == "test-org" {
		if err := seedTestOrgFixtures(ctx, registrySet, tenant, users, opts); err != nil {
			return false, err
		}
	}

	// Ensure user1 has a default group valued in CZK.
	group1, err := findOrCreateDefaultGroup(ctx, registrySet, user1, models.Currency("CZK"))
	if err != nil {
		return false, err
	}
	userCtx := appctx.WithGroup(appctx.WithUser(ctx, user1), group1)

	// Create user-aware registry set for the group-scoped data tables.
	userRegistrySet, err := factorySet.CreateUserRegistrySet(userCtx)
	if err != nil {
		return false, fmt.Errorf("failed to create user registry set for user 1: %w", err)
	}

	// User 2 gets a default group valued in EUR.
	if user2 != nil {
		if _, err := findOrCreateDefaultGroup(ctx, registrySet, user2, models.Currency("EUR")); err != nil {
			return false, err
		}
	}

	// Idempotency gate: if user1's group already has any locations,
	// short-circuit. Everything below this point is additive.
	locCount, err := userRegistrySet.LocationRegistry.Count(userCtx)
	if err != nil {
		return false, fmt.Errorf("failed to count existing locations for user 1: %w", err)
	}
	if locCount > 0 {
		slog.Info("Database already seeded; skipping data creation",
			"user", user1.Email,
			"location_count", locCount,
		)
		// The data tables are populated, but the bundled fixture BLOBS
		// live in a separate store with an independent lifecycle. A DB
		// seeded metadata-only (before blob uploads were enabled for its
		// tenant, #1931) or a bucket wiped independently of the DB leaves
		// fixture file rows pointing at objects that don't exist — the
		// Files page then shows entries whose download/thumbnail 404s.
		// Reconcile so the bucket converges back to the rows on the next
		// seed call instead of staying permanently diverged. Best-effort
		// and conservative — see reconcileSeedBlobs.
		if err := reconcileSeedBlobs(userCtx, userRegistrySet, tenant, opts); err != nil {
			return true, fmt.Errorf("reconcile seed blobs: %w", err)
		}
		return true, nil
	}

	// Sanity-check the embedded fixture bundle once before we start
	// uploading. Catches the "built from incomplete tree" case early
	// rather than failing mid-seed and leaving a half-populated DB.
	if err := ensureFixturesPresent(); err != nil {
		return false, err
	}

	// Blob uploads (the ~50-file bundled photo/invoice/manual set) are
	// gated on the test-org tenant slug. /api/v1/seed is currently a
	// public, unauthenticated endpoint and the upload step writes real
	// bytes to the configured blob bucket — without this gate, anyone
	// who can reach /seed?tenant_slug=acme could spam someone else's
	// bucket on first call (subsequent calls no-op via the
	// locations-count gate above). Restricting blob writes to the
	// well-known test tenant keeps the demo-fidelity story for
	// test-org while making the public endpoint cost-bounded for
	// every other tenant.
	//
	// AllowBlobUploads is the explicit, env-gated opt-out of that
	// restriction (INVENTARIO_SEED_ALLOW_BLOB_UPLOADS, set server-side
	// only — see the seed handler). The Helm chart flips it on for the
	// demo overlay so the evaluation deployment's `default` tenant gets
	// real cover photos and documents like test-org does.
	uploadLocation := opts.UploadLocation
	if tenant.Slug != "test-org" && !opts.AllowBlobUploads {
		uploadLocation = ""
	}
	uploader, err := newBlobUploader(ctx, uploadLocation)
	if err != nil {
		return false, err
	}
	defer uploader.close()

	// Tag catalogue first — commodities reference tag slugs and the
	// tag rows need to exist before the JSONB array on each commodity
	// can hydrate cleanly on the /tags page (the page joins by slug).
	if err := seedTags(userCtx, userRegistrySet, user1, group1); err != nil {
		return false, fmt.Errorf("seed tags: %w", err)
	}

	// Inventory tree + files in one pass. The function returns the
	// list of created commodities so the loans / services / events
	// passes below can pick from them deterministically.
	inv, err := seedInventory(userCtx, userRegistrySet, user1, group1, uploader)
	if err != nil {
		return false, fmt.Errorf("seed inventory: %w", err)
	}

	// Loans + services — strictly count=1 commodities (#1554 invariant).
	if err := seedLoansAndServices(userCtx, userRegistrySet, user1, group1, inv); err != nil {
		return false, fmt.Errorf("seed loans/services: %w", err)
	}

	// Multi-member group setup. Gated on the test-org tenant only —
	// arbitrary external tenants do not get well-known-password
	// fixture users added to their membership tables.
	if tenant.Slug == "test-org" && user2 != nil {
		if err := seedGroupMembers(ctx, registrySet, tenant, user1, user2, group1); err != nil {
			return false, fmt.Errorf("seed members: %w", err)
		}
	}

	// Exports + restores + commodity events + audit log — populate the
	// audit / backup / activity surfaces.
	if err := seedHistory(userCtx, ctx, userRegistrySet, registrySet, user1, group1, inv); err != nil {
		return false, fmt.Errorf("seed history: %w", err)
	}

	return false, nil
}

// seedTestOrgFixtures provisions the extra fixture users that only the
// well-known `test-org` tenant gets. The caller gates this on the tenant
// slug; every helper here is independently idempotent.
//
// Both seed entry points (no-opts memory-mode tests AND init-data's
// email-pinned /api/v1/seed call) reach this for test-org — keeping it
// inside the seed means the e2e workflow doesn't need a separate CLI step
// to provision the fixtures, while the tenant.Slug gate keeps these
// well-known-password accounts out of arbitrary external tenants.
//
//   - orphan@test-org.com — a zero-group user so e2e tests can
//     authenticate against the real `/api/v1/groups` empty-collection
//     response (#1277).
//   - blocktarget@test-org.com — a disposable plain user the
//     block/unblock spec (#1758) deactivates then reactivates. No other
//     spec references it, so a parallel run never observes it mid-block.
//     It carries no elevated privileges, so it is in the same (accepted)
//     risk class as the orphan fixture and is provisioned unconditionally
//     for the test-org tenant.
//   - sysadmin@test-org.com — carries is_system_admin so the
//     admin-section e2e suite (#1758) reaches /api/v1/admin/* and
//     /admin/*. Minting a *cross-tenant* admin from the unauthenticated
//     /api/v1/seed endpoint would be a privilege-escalation hole in any
//     deployment where /seed is reachable, so it is gated behind an
//     explicit opt-in (opts.SeedSystemAdmin, set from
//     INVENTARIO_SEED_SYSTEM_ADMIN_FIXTURE by the seed handler — see
//     apiserver/seed.go). It is OFF by default; only the e2e harness
//     turns it on.
func seedTestOrgFixtures(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User, opts SeedOptions) error {
	if err := ensureOrphanUser(ctx, registrySet, tenant, users); err != nil {
		return err
	}
	if err := ensureBlockTargetUser(ctx, registrySet, tenant, users); err != nil {
		return err
	}
	if opts.SeedSystemAdmin {
		if err := ensureSystemAdminUser(ctx, registrySet, tenant, users); err != nil {
			return err
		}
	}
	return nil
}

// guardPreExistingTenant enforces the #2113 L-2 fail-closed rule for a tenant
// that already existed before this seed call: the public, unauthenticated
// /api/v1/seed endpoint must not be coaxed into seeding the demo dataset into a
// tenant it doesn't own — that would let any caller pollute real production
// data via /api/v1/seed?tenant_slug=<theirs>. It returns an error (the caller
// aborts before creating any rows) unless one of the exemptions holds:
//   - the slug is the well-known `test-org` sentinel the seed owns;
//   - the explicit env-gated opt-in (CreateTenantIfMissing, from
//     INVENTARIO_SEED_ALLOW_CREATE_TENANT) is set; or
//   - the tenant already owns users — the idempotent reconcile/reseed path
//     (#1931), which must keep working to backfill blobs on a later run.
//
// The tenant-scoped ListByTenant is used deliberately: the service-registry
// UserRegistry.List is GLOBAL (RLS-bypassed, all tenants) and so cannot tell
// whether THIS tenant has users — using it would defeat the guard in a real
// multi-tenant deployment.
func guardPreExistingTenant(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, opts SeedOptions) error {
	if tenant.Slug == testOrgTenantSlug || opts.CreateTenantIfMissing {
		return nil
	}
	tenantUsers, err := registrySet.UserRegistry.ListByTenant(ctx, tenant.ID)
	if err != nil {
		return fmt.Errorf("failed to check existing users for tenant '%s': %w", tenant.Slug, err)
	}
	if len(tenantUsers) > 0 {
		// Already-seeded tenant: the reconcile/reseed path is allowed.
		return nil
	}
	return fmt.Errorf(
		"refusing to seed into pre-existing tenant '%s': the demo dataset is only seeded into the '%s' sentinel, a freshly created tenant, or one already seeded (set INVENTARIO_SEED_ALLOW_CREATE_TENANT to override for trusted fixtures)",
		tenant.Slug, testOrgTenantSlug,
	)
}

// findOrCreateTenant finds an existing tenant by slug or creates a new
// test tenant. When opts.TenantSlug is non-empty and the row is
// missing, opts.CreateTenantIfMissing decides whether to fail-closed
// (the default, preserves the strict production contract) or to
// create-then-seed (e2e cross-tenant fixture path, #1851; the seed
// handler binds opts.CreateTenantIfMissing to
// INVENTARIO_SEED_ALLOW_CREATE_TENANT).
//
// The second return value (preExisted) reports whether the resolved tenant
// row already existed before this call — the L-2 fresh-seed guard in SeedData
// uses it to refuse the FIRST seed into an unrelated pre-existing tenant while
// still allowing idempotent reconcile reseeds.
func findOrCreateTenant(ctx context.Context, registrySet *registry.Set, opts SeedOptions) (_ *models.Tenant, preExisted bool, _ error) {
	if opts.TenantSlug != "" {
		tenant, err := registrySet.TenantRegistry.GetBySlug(ctx, opts.TenantSlug)
		if err == nil {
			return tenant, true, nil
		}
		// Only fall through to create-if-missing on the explicit "not
		// found" sentinel. Any other lookup error (DB down, RLS
		// rejection, deserialization failure, etc.) must surface
		// unchanged — masking it behind a create would both hide the
		// root cause AND risk creating a duplicate-named tenant when
		// the row already exists but the read failed for a transient
		// reason.
		if !errors.Is(err, registry.ErrNotFound) {
			return nil, false, fmt.Errorf("tenant lookup for slug '%s' failed: %w", opts.TenantSlug, err)
		}
		if !opts.CreateTenantIfMissing {
			return nil, false, fmt.Errorf("tenant with slug '%s' not found: %w", opts.TenantSlug, err)
		}
		// Create-if-missing: provision a new active tenant with a
		// human-friendly name derived from the slug. Slug stays as
		// the operator supplied it. Status defaults to active so the
		// seed flow can immediately mint users/groups inside it.
		newTenant := models.Tenant{
			Name:   "Test Tenant " + opts.TenantSlug,
			Slug:   opts.TenantSlug,
			Status: models.TenantStatusActive,
		}
		created, createErr := registrySet.TenantRegistry.Create(ctx, newTenant)
		if createErr != nil {
			return nil, false, fmt.Errorf("create tenant '%s': %w", opts.TenantSlug, createErr)
		}
		return created, false, nil
	}

	existingTenants, err := registrySet.TenantRegistry.List(ctx)
	if err == nil && len(existingTenants) > 0 {
		return existingTenants[0], true, nil
	}

	testTenant := models.Tenant{
		Name:   "Test Organization",
		Slug:   testOrgTenantSlug,
		Status: models.TenantStatusActive,
	}
	created, createErr := registrySet.TenantRegistry.Create(ctx, testTenant)
	return created, false, createErr
}

// findOrCreateUsers finds existing users or creates test users based on options.
//
// When the caller pinned a user_email + tenant_slug (the init-data docker
// path), we still want the `user2@test-org.com` fixture present so the
// per-tenant multi-member content lights up. That synthesis only fires
// for the test-org tenant — arbitrary external tenants do NOT get a
// well-known-password fixture user planted by /api/v1/seed.
func findOrCreateUsers(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User, userEmail string) (user1 *models.User, user2 *models.User, err error) {
	if userEmail != "" {
		user1, _ = findUserByEmail(users, tenant.ID, userEmail)
		if user1 == nil {
			return nil, nil, fmt.Errorf("user with email '%s' not found in tenant '%s'", userEmail, tenant.Slug)
		}
		_, user2 = findExistingUsers(users, tenant.ID)
		if user2 != nil && user2.ID == user1.ID {
			user2 = nil
		}
		if user2 == nil && tenant.Slug == "test-org" {
			user2, err = createSecondaryTestUser(ctx, registrySet, tenant)
			if err != nil {
				return nil, nil, err
			}
		}
		return user1, user2, nil
	}

	user1, user2 = findExistingUsers(users, tenant.ID)

	if user1 == nil {
		return createTestUsers(ctx, registrySet, tenant, user2)
	}

	return user1, user2, nil
}

// createSecondaryTestUser provisions the well-known user2 fixture when
// the per-tenant lookup didn't find one. Only callable from the
// test-org-gated path in findOrCreateUsers above.
func createSecondaryTestUser(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant) (*models.User, error) {
	testUser2 := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    "user2@test-org.com",
		Name:     "Test User 2",
		IsActive: true,
	}
	if err := testUser2.SetPassword("TestPassword123"); err != nil {
		return nil, err
	}
	created, err := registrySet.UserRegistry.Create(ctx, testUser2)
	if err != nil {
		return nil, fmt.Errorf("failed to create test user 2: %w", err)
	}
	return created, nil
}

// findUserByEmail finds a specific user by email and tenant ID.
func findUserByEmail(users []*models.User, tenantID, email string) (primary *models.User, secondary *models.User) {
	for _, user := range users {
		if user.TenantID == tenantID && user.Email == email {
			return user, nil
		}
	}
	return nil, nil
}

// findExistingUsers finds the seeded test users for a tenant. The lookup
// is keyed by the well-known seed emails (admin@test-org.com and
// user2@test-org.com) so the (primary, secondary) pair is stable
// regardless of the order the registry returns users in.
func findExistingUsers(users []*models.User, tenantID string) (primary *models.User, secondary *models.User) {
	const (
		primaryEmail   = "admin@test-org.com"
		secondaryEmail = "user2@test-org.com"
	)
	for _, user := range users {
		if user.TenantID != tenantID {
			continue
		}
		switch user.Email {
		case primaryEmail:
			primary = user
		case secondaryEmail:
			secondary = user
		}
		if primary != nil && secondary != nil {
			break
		}
	}
	return primary, secondary
}

// ensureOrphanUser idempotently provisions a third active test user
// with zero group memberships (`orphan@test-org.com`). See issue #1277.
func ensureOrphanUser(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User) error {
	const orphanEmail = "orphan@test-org.com"
	for _, user := range users {
		if user.TenantID == tenant.ID && user.Email == orphanEmail {
			// Reconcile drift: force the fixture back to active so the
			// next run starts from a clean state.
			if !user.IsActive {
				user.IsActive = true
				if _, err := registrySet.UserRegistry.Update(ctx, *user); err != nil {
					return errxtrace.Wrap("failed to reconcile orphan test user", err)
				}
			}
			return nil
		}
	}

	orphan := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    orphanEmail,
		Name:     "Test Orphan (no group)",
		IsActive: true,
	}
	if err := orphan.SetPassword("TestPassword123"); err != nil {
		return err
	}
	if _, err := registrySet.UserRegistry.Create(ctx, orphan); err != nil {
		return fmt.Errorf("failed to create orphan test user: %w", err)
	}
	return nil
}

// ensureSystemAdminUser idempotently provisions `sysadmin@test-org.com`,
// a platform system administrator the admin-section e2e suite (#1758)
// authenticates as. It mirrors the production `inventario admin
// grant-system-admin` CLI bootstrap, but runs inside the seed so the
// e2e harness (whose local stack is memory-mode — the admin CLI
// rejects memory:// DSNs) gets the fixture. The caller gates this on
// opts.SeedSystemAdmin so an unauthenticated /api/v1/seed call cannot
// mint a cross-tenant admin in production.
//
// The helper is fully idempotent and self-healing: it runs before the
// location-count gate, and on a re-seed it reconciles a drifted
// fixture (deactivated user, or missing grant row) back to the
// expected state rather than no-op'ing.
//
// As of #1784 the system-admin privilege lives in
// `system_admin_grants`, not on the users row — so the reconciliation
// is split into (a) ensure user is_active and (b) ensure a grant row
// exists.
//
// The system admin also gets a USD-valued default group so login lands
// cleanly (no /no-group race) and so it never collides with the CZK/EUR
// groups the user-isolation specs depend on.
func ensureSystemAdminUser(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User) error {
	const sysadminEmail = "sysadmin@test-org.com"

	var sysadmin *models.User
	for _, user := range users {
		if user.TenantID == tenant.ID && user.Email == sysadminEmail {
			sysadmin = user
			break
		}
	}

	switch {
	case sysadmin == nil:
		newUser := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: tenant.ID,
			},
			Email:    sysadminEmail,
			Name:     "Test System Admin",
			IsActive: true,
		}
		if err := newUser.SetPassword("TestPassword123"); err != nil {
			return err
		}
		created, err := registrySet.UserRegistry.Create(ctx, newUser)
		if err != nil {
			return errxtrace.Wrap("failed to create system-admin test user", err)
		}
		sysadmin = created
	case !sysadmin.IsActive:
		// Reconcile drift: a prior run (or a manual edit) may have
		// left the fixture deactivated.
		sysadmin.IsActive = true
		updated, err := registrySet.UserRegistry.Update(ctx, *sysadmin)
		if err != nil {
			return errxtrace.Wrap("failed to reconcile system-admin test user", err)
		}
		sysadmin = updated
	}

	// Idempotent — already-granted users return hadGrant=true with no
	// row mutation. A nil registry here is a miswired FactorySet that
	// would otherwise produce a non-admin sysadmin fixture and break
	// every admin-section e2e expectation downstream; fail loudly
	// instead of silently no-op'ing.
	if registrySet.SystemAdminGrantRegistry == nil {
		return errxtrace.Wrap(
			"system-admin seed fixture requires SystemAdminGrantRegistry; FactorySet is miswired",
			registry.ErrInvalidConfig,
		)
	}
	if _, err := registrySet.SystemAdminGrantRegistry.Grant(ctx, sysadmin.ID, nil); err != nil {
		return errxtrace.Wrap("failed to grant system-admin to seeded fixture", err)
	}

	if _, err := findOrCreateDefaultGroup(ctx, registrySet, sysadmin, models.Currency("USD")); err != nil {
		return errxtrace.Wrap("failed to create system-admin default group", err)
	}
	return nil
}

// ensureBlockTargetUser idempotently provisions `blocktarget@test-org.com`,
// a disposable plain (non-admin) test user the admin-section e2e suite
// (#1758) blocks then unblocks. It is intentionally referenced by no
// other spec so the parallel Playwright run never observes it while it
// is mid-block. It is self-healing: a failed block/unblock run can leave
// the fixture deactivated, so a re-seed reconciles it back to active.
func ensureBlockTargetUser(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User) error {
	const blockTargetEmail = "blocktarget@test-org.com"
	for _, user := range users {
		if user.TenantID == tenant.ID && user.Email == blockTargetEmail {
			return reconcileBlockTargetDrift(ctx, registrySet, user)
		}
	}

	target := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    blockTargetEmail,
		Name:     "Test Block Target",
		IsActive: true,
	}
	if err := target.SetPassword("TestPassword123"); err != nil {
		return err
	}
	if _, err := registrySet.UserRegistry.Create(ctx, target); err != nil {
		return errxtrace.Wrap("failed to create block-target test user", err)
	}
	return nil
}

// reconcileBlockTargetDrift forces an already-seeded block-target
// fixture back to a clean state: active, no system-admin grant. A
// failed block/unblock run can leave is_active=false, and a drifted
// admin tooling run can have granted system-admin — the next seed
// should reset both. Extracted from ensureBlockTargetUser so the
// caller stays under the nestif complexity budget.
func reconcileBlockTargetDrift(ctx context.Context, registrySet *registry.Set, user *models.User) error {
	if !user.IsActive {
		user.IsActive = true
		if _, err := registrySet.UserRegistry.Update(ctx, *user); err != nil {
			return errxtrace.Wrap("failed to reconcile block-target test user", err)
		}
	}
	// A nil registry here would leave a drifted block-target fixture
	// silently holding a system-admin grant (if a prior run granted it)
	// — the e2e block/unblock spec then exercises a system admin rather
	// than the plain user it expects. Fail loud on the miswiring instead.
	if registrySet.SystemAdminGrantRegistry == nil {
		return errxtrace.Wrap(
			"block-target reconcile requires SystemAdminGrantRegistry; FactorySet is miswired",
			registry.ErrInvalidConfig,
		)
	}
	if _, err := registrySet.SystemAdminGrantRegistry.RevokeAtomic(ctx, user.ID, true); err != nil {
		return errxtrace.Wrap("failed to revoke any drifted system-admin grant from block-target", err)
	}
	return nil
}

// createTestUsers creates test users.
func createTestUsers(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, existingUser2 *models.User) (primary *models.User, secondary *models.User, err error) {
	slog.Info("Creating test users", "tenant", tenant.Slug)

	testUser1 := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    "admin@test-org.com",
		Name:     "Test Administrator",
		IsActive: true,
	}
	err = testUser1.SetPassword("TestPassword123")
	if err != nil {
		return nil, nil, err
	}
	primary, err = registrySet.UserRegistry.Create(ctx, testUser1)
	if err != nil {
		return nil, nil, err
	}

	secondary = existingUser2
	if secondary == nil {
		testUser2 := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: tenant.ID,
			},
			Email:    "user2@test-org.com",
			Name:     "Test User 2",
			IsActive: true,
		}
		err = testUser2.SetPassword("TestPassword123")
		if err != nil {
			return nil, nil, err
		}
		secondary, err = registrySet.UserRegistry.Create(ctx, testUser2)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to create test user 2: %w", err)
		}
	}

	return primary, secondary, nil
}

// findOrCreateDefaultGroup returns the user's first existing group (via
// membership) or creates a new active group with the user as owner.
func findOrCreateDefaultGroup(ctx context.Context, registrySet *registry.Set, user *models.User, groupCurrency models.Currency) (*models.LocationGroup, error) {
	memberships, err := registrySet.GroupMembershipRegistry.ListByUser(ctx, user.TenantID, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to list memberships for user %s: %w", user.ID, err)
	}

	var group *models.LocationGroup
	if len(memberships) > 0 {
		group, err = reconcileExistingDefaultGroup(ctx, registrySet, memberships[0].GroupID, groupCurrency)
	} else {
		group, err = createDefaultGroupForUser(ctx, registrySet, user, groupCurrency)
	}
	if err != nil {
		return nil, err
	}

	if err := ensureUserDefaultGroup(ctx, registrySet, user, group.ID); err != nil {
		return nil, fmt.Errorf("failed to reconcile default_group_id for user %s: %w", user.ID, err)
	}
	return group, nil
}

// reconcileExistingDefaultGroup loads the user's already-known group
// and, if the stored group_currency drifted from what the seed wants,
// updates the row.
func reconcileExistingDefaultGroup(ctx context.Context, registrySet *registry.Set, groupID string, groupCurrency models.Currency) (*models.LocationGroup, error) {
	group, err := registrySet.LocationGroupRegistry.Get(ctx, groupID)
	if err != nil {
		return nil, fmt.Errorf("failed to load existing group %s: %w", groupID, err)
	}
	if group.GroupCurrency == groupCurrency {
		return group, nil
	}
	group.GroupCurrency = groupCurrency
	updated, err := registrySet.LocationGroupRegistry.Update(ctx, *group)
	if err != nil {
		return nil, fmt.Errorf("failed to update group currency on existing group %s: %w", group.ID, err)
	}
	return updated, nil
}

// createDefaultGroupForUser provisions a fresh "Default" group for the
// user and inserts the owner membership row.
func createDefaultGroupForUser(ctx context.Context, registrySet *registry.Set, user *models.User, groupCurrency models.Currency) (*models.LocationGroup, error) {
	slug, err := models.GenerateGroupSlug()
	if err != nil {
		return nil, fmt.Errorf("failed to generate group slug: %w", err)
	}
	created, err := registrySet.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
		},
		Slug:          slug,
		Name:          "Default",
		Status:        models.LocationGroupStatusActive,
		CreatedBy:     user.ID,
		GroupCurrency: groupCurrency,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create default group: %w", err)
	}
	if created == nil {
		return nil, errors.New("group registry returned nil group")
	}

	if _, err := registrySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
		},
		GroupID:      created.ID,
		MemberUserID: user.ID,
		Role:         models.GroupRoleOwner,
		JoinedAt:     time.Now(),
	}); err != nil {
		return nil, fmt.Errorf("failed to create owner membership: %w", err)
	}
	return created, nil
}

// ensureUserDefaultGroup mirrors services.EnsureUserDefaultGroup for the
// seed path.
func ensureUserDefaultGroup(ctx context.Context, registrySet *registry.Set, user *models.User, fallbackGroupID string) error {
	current, err := registrySet.UserRegistry.Get(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("failed to load user %s: %w", user.ID, err)
	}
	if current.DefaultGroupID != nil && *current.DefaultGroupID != "" {
		return nil
	}
	current.DefaultGroupID = &fallbackGroupID
	current.UpdatedAt = time.Now()
	if _, err := registrySet.UserRegistry.Update(ctx, *current); err != nil {
		return fmt.Errorf("failed to persist default_group_id on user %s: %w", user.ID, err)
	}
	user.DefaultGroupID = current.DefaultGroupID
	return nil
}
