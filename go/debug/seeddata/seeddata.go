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

	"github.com/denisvmedia/inventario/appctx"
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// SeedOptions contains optional parameters for seeding.
type SeedOptions struct {
	UserEmail  string // Optional: email of user to seed for
	TenantSlug string // Optional: slug of tenant to seed for

	// UploadLocation is the gocloud-style blob URL the seed uses to
	// publish bundled file fixtures (photos, invoices, manuals). When
	// empty, the seed still creates file *rows* so the UI shows them,
	// but no bytes are written — useful for in-memory unit tests that
	// don't have a blob bucket attached. Real callers (apiserver/seed
	// handler, init-data) pass the server's configured upload
	// location through verbatim.
	UploadLocation string
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

	// Find or create tenant.
	tenant, err := findOrCreateTenant(ctx, registrySet, opts.TenantSlug)
	if err != nil {
		return false, err
	}

	// Get existing users for the tenant.
	users, err := registrySet.UserRegistry.List(ctx)
	if err != nil {
		return false, err
	}

	user1, user2, err := findOrCreateUsers(ctx, registrySet, tenant, users, opts.UserEmail)
	if err != nil {
		return false, err
	}

	// Inside the well-known `test-org` test tenant, provision a third
	// zero-group test user so e2e tests can authenticate against the
	// real `/api/v1/groups` empty-collection response (#1277). Both
	// seed entry points (no-opts memory-mode tests AND init-data's
	// email-pinned /api/v1/seed call) take this branch — keeping it
	// inside the seed means the e2e workflow doesn't need a separate
	// CLI step to provision the fixture. The tenant.Slug gate keeps
	// the well-known-password orphan account out of arbitrary
	// external tenants.
	if tenant.Slug == "test-org" {
		if err := ensureOrphanUser(ctx, registrySet, tenant, users); err != nil {
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
		return true, nil
	}

	// Sanity-check the embedded fixture bundle once before we start
	// uploading. Catches the "built from incomplete tree" case early
	// rather than failing mid-seed and leaving a half-populated DB.
	if err := ensureFixturesPresent(); err != nil {
		return false, err
	}

	uploader, err := newBlobUploader(ctx, opts.UploadLocation)
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

// findOrCreateTenant finds an existing tenant by slug or creates a new
// test tenant.
func findOrCreateTenant(ctx context.Context, registrySet *registry.Set, tenantSlug string) (*models.Tenant, error) {
	if tenantSlug != "" {
		tenant, err := registrySet.TenantRegistry.GetBySlug(ctx, tenantSlug)
		if err != nil {
			return nil, fmt.Errorf("tenant with slug '%s' not found: %w", tenantSlug, err)
		}
		return tenant, nil
	}

	existingTenants, err := registrySet.TenantRegistry.List(ctx)
	if err == nil && len(existingTenants) > 0 {
		return existingTenants[0], nil
	}

	testTenant := models.Tenant{
		Name:   "Test Organization",
		Slug:   "test-org",
		Status: models.TenantStatusActive,
	}
	return registrySet.TenantRegistry.Create(ctx, testTenant)
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
