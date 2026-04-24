package postgres_test

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jmoiron/sqlx"

	_ "github.com/denisvmedia/inventario/internal/fileblob" // register file:// driver
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres"
	"github.com/denisvmedia/inventario/schema/migrations/migrator"
	"github.com/denisvmedia/inventario/services"
)

// setupCleanPostgresFactorySet migrates the schema and returns a raw factory
// set with no seeded tenants/users/groups. Used by multi-tenant purge tests
// that need to manage their own fixtures.
func setupCleanPostgresFactorySet(t *testing.T) *registry.FactorySet {
	t.Helper()

	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)

	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)

	// Drop and recreate the schema so fixtures from prior tests don't bleed in.
	migr := migrator.NewWithFallback(dsn, "../../models")
	ctx := context.Background()
	c.Assert(migrateUp(t, ctx, migr, dsn), qt.IsNil)

	sqlxDB := sqlx.NewDb(stdlib.OpenDBFromPool(pool), "pgx")
	return postgres.NewFactorySet(sqlxDB)
}

// newPostgresFileUploadLocation returns a file:// DSN for a temp dir so the
// FileService can open its bucket during purge (a no-op when no files are
// seeded, but the code path must not error out).
func newPostgresFileUploadLocation(c *qt.C) string {
	tempDir := c.TempDir()
	if runtime.GOOS == "windows" {
		return "file:///" + tempDir + "?create_dir=1"
	}
	return "file://" + tempDir + "?create_dir=1"
}

// seedTenantWithPendingGroup creates a tenant + admin user + pending_deletion
// location group + one used invite inside that tenant. Returns the tenant ID,
// user ID, group ID, and the invite's token (useful for audit assertions).
func seedTenantWithPendingGroup(c *qt.C, ctx context.Context, fs *registry.FactorySet, slug, email string) (tenantID, userID, groupID, inviteToken string) {
	c.Helper()

	tenant, err := fs.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Tenant " + slug,
		Slug:   slug,
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenant.ID},
		Email:               email,
		Name:                "Admin " + slug,
		IsActive:            true,
	}
	c.Assert(user.SetPassword("password123"), qt.IsNil)
	createdUser, err := fs.UserRegistry.Create(ctx, user)
	c.Assert(err, qt.IsNil)

	groupSlug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	group, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: tenant.ID},
		Name:               "Pending " + slug,
		Slug:               groupSlug,
		Status:             models.LocationGroupStatusPendingDeletion,
		CreatedBy:          createdUser.ID,
		MainCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)

	token, err := models.GenerateInviteToken()
	c.Assert(err, qt.IsNil)
	usedAt := time.Now().Add(-1 * time.Hour)
	usedByPtr := createdUser.ID
	_, err = fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: tenant.ID},
		GroupID:            group.ID,
		Token:              token,
		CreatedBy:          createdUser.ID,
		ExpiresAt:          time.Now().Add(24 * time.Hour),
		UsedBy:             &usedByPtr,
		UsedAt:             &usedAt,
	})
	c.Assert(err, qt.IsNil)

	return tenant.ID, createdUser.ID, group.ID, token
}

// TestGroupPurgeService_Postgres_CrossTenantRLSBypass verifies that the
// purge worker can see and delete pending_deletion LocationGroups belonging
// to multiple tenants in a single sweep, AND that the resulting audit rows
// are written across tenant boundaries. Under the inventario_app role the
// tenant-isolation RLS policies on location_groups, group_invites, and
// group_invites_audit would hide rows from foreign tenants; the
// inventario_background_worker role has a bypass policy on each of those
// tables and that is what makes cross-tenant maintenance work. This test
// asserts the bypass end-to-end against a real PostgreSQL schema.
func TestGroupPurgeService_Postgres_CrossTenantRLSBypass(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()

	tenantA, _, groupA, _ := seedTenantWithPendingGroup(c, ctx, fs, "tenant-a", "admin@a.example")
	tenantB, _, groupB, _ := seedTenantWithPendingGroup(c, ctx, fs, "tenant-b", "admin@b.example")

	fileSvc := services.NewFileService(fs, newPostgresFileUploadLocation(c))
	svc := services.NewGroupPurgeService(fs, fileSvc)

	purged, failed, err := svc.PurgeOnce(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(purged, qt.Equals, 2)
	c.Assert(failed, qt.Equals, 0)

	// Both pending groups are gone — the service-mode registry ran as the
	// background worker role, which has a USING(true) bypass policy on
	// location_groups, and was therefore able to DELETE rows belonging to
	// foreign tenants in a single transaction sequence.
	_, err = fs.LocationGroupRegistry.Get(ctx, groupA)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue, qt.Commentf("group A should be purged"))
	_, err = fs.LocationGroupRegistry.Get(ctx, groupB)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue, qt.Commentf("group B should be purged"))

	// Audit rows were INSERTed for both tenants — group_invites_audit's
	// bypass policy on inventario_background_worker is what permits a
	// single transaction to write rows tagged with different tenant_ids.
	auditA, err := fs.GroupInviteAuditRegistry.ListByTenant(ctx, tenantA)
	c.Assert(err, qt.IsNil)
	c.Assert(auditA, qt.HasLen, 1)
	c.Assert(auditA[0].OriginalGroupID, qt.Equals, groupA)

	auditB, err := fs.GroupInviteAuditRegistry.ListByTenant(ctx, tenantB)
	c.Assert(err, qt.IsNil)
	c.Assert(auditB, qt.HasLen, 1)
	c.Assert(auditB[0].OriginalGroupID, qt.Equals, groupB)

	// ListByOriginalGroup scoped to tenant A's original group must not leak
	// tenant B's audit row (even though the worker role can see both, the
	// field filter is by original_group_id).
	auditByGroupA, err := fs.GroupInviteAuditRegistry.ListByOriginalGroup(ctx, groupA)
	c.Assert(err, qt.IsNil)
	c.Assert(auditByGroupA, qt.HasLen, 1)
	c.Assert(auditByGroupA[0].TenantID, qt.Equals, tenantA)

	// Invites for both purged groups must be gone (DeleteByGroup ran in
	// service mode, so it reached across both tenants in the same sweep).
	invitesA, err := fs.GroupInviteRegistry.ListActiveByGroup(ctx, groupA)
	c.Assert(err, qt.IsNil)
	c.Assert(invitesA, qt.HasLen, 0)
	invitesB, err := fs.GroupInviteRegistry.ListActiveByGroup(ctx, groupB)
	c.Assert(err, qt.IsNil)
	c.Assert(invitesB, qt.HasLen, 0)
}

// TestGroupPurgeService_Postgres_CleanExpiredInvitesCrossTenant verifies
// the Option 2i sweep reaches across tenants: unused expired invites owned
// by different tenants are all removed in one service-mode DELETE, while
// used invites (which are the domain of the per-group purge audit path)
// and unused non-expired invites are preserved regardless of tenant.
func TestGroupPurgeService_Postgres_CleanExpiredInvitesCrossTenant(t *testing.T) {
	fs := setupCleanPostgresFactorySet(t)

	c := qt.New(t)
	ctx := context.Background()

	// Two separate tenants, each with an ACTIVE group (so the per-group
	// purge path will not touch them) and a mix of invites.
	tenantA := mustCreateTenant(c, ctx, fs, "tenant-a")
	userA := mustCreateUser(c, ctx, fs, tenantA, "admin@a.example")
	groupA := mustCreateActiveGroup(c, ctx, fs, tenantA, userA.ID)

	tenantB := mustCreateTenant(c, ctx, fs, "tenant-b")
	userB := mustCreateUser(c, ctx, fs, tenantB, "admin@b.example")
	groupB := mustCreateActiveGroup(c, ctx, fs, tenantB, userB.ID)

	// Invites to sweep (unused, expired) — one per tenant.
	expiredA := mustCreateInvite(c, ctx, fs, tenantA, groupA, userA.ID, time.Now().Add(-time.Hour), nil, nil)
	expiredB := mustCreateInvite(c, ctx, fs, tenantB, groupB, userB.ID, time.Now().Add(-30*time.Minute), nil, nil)

	// Invite that must survive: unused, non-expired.
	activeInvite := mustCreateInvite(c, ctx, fs, tenantA, groupA, userA.ID, time.Now().Add(24*time.Hour), nil, nil)

	// Invite that must survive: used, expired (audit domain, not sweep).
	usedAt := time.Now().Add(-2 * time.Hour)
	usedBy := userB.ID
	usedInvite := mustCreateInvite(c, ctx, fs, tenantB, groupB, userB.ID, time.Now().Add(-1*time.Hour), &usedBy, &usedAt)

	fileSvc := services.NewFileService(fs, newPostgresFileUploadLocation(c))
	svc := services.NewGroupPurgeService(fs, fileSvc)

	deleted, err := svc.CleanExpiredInvites(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(deleted, qt.Equals, 2)

	_, err = fs.GroupInviteRegistry.Get(ctx, expiredA)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue, qt.Commentf("tenant A expired unused invite should be gone"))
	_, err = fs.GroupInviteRegistry.Get(ctx, expiredB)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue, qt.Commentf("tenant B expired unused invite should be gone"))

	// Survivors.
	_, err = fs.GroupInviteRegistry.Get(ctx, activeInvite)
	c.Assert(err, qt.IsNil)
	_, err = fs.GroupInviteRegistry.Get(ctx, usedInvite)
	c.Assert(err, qt.IsNil)
}

// -- small, test-local fixture helpers -------------------------------------

func mustCreateTenant(c *qt.C, ctx context.Context, fs *registry.FactorySet, slug string) string {
	c.Helper()
	t, err := fs.TenantRegistry.Create(ctx, models.Tenant{
		Name:   "Tenant " + slug,
		Slug:   slug,
		Status: models.TenantStatusActive,
	})
	c.Assert(err, qt.IsNil)
	return t.ID
}

func mustCreateUser(c *qt.C, ctx context.Context, fs *registry.FactorySet, tenantID, email string) *models.User {
	c.Helper()
	u := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               email,
		Name:                "User " + email,
		IsActive:            true,
	}
	c.Assert(u.SetPassword("password123"), qt.IsNil)
	created, err := fs.UserRegistry.Create(ctx, u)
	c.Assert(err, qt.IsNil)
	return created
}

func mustCreateActiveGroup(c *qt.C, ctx context.Context, fs *registry.FactorySet, tenantID, userID string) string {
	c.Helper()
	slug, err := models.GenerateGroupSlug()
	c.Assert(err, qt.IsNil)
	g, err := fs.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: tenantID},
		Name:               "Active " + slug,
		Slug:               slug,
		Status:             models.LocationGroupStatusActive,
		CreatedBy:          userID,
		MainCurrency:       models.Currency("USD"),
	})
	c.Assert(err, qt.IsNil)
	return g.ID
}

func mustCreateInvite(c *qt.C, ctx context.Context, fs *registry.FactorySet, tenantID, groupID, createdBy string, expiresAt time.Time, usedBy *string, usedAt *time.Time) string {
	c.Helper()
	token, err := models.GenerateInviteToken()
	c.Assert(err, qt.IsNil)
	inv, err := fs.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: tenantID},
		GroupID:            groupID,
		Token:              token,
		CreatedBy:          createdBy,
		ExpiresAt:          expiresAt,
		UsedBy:             usedBy,
		UsedAt:             usedAt,
	})
	c.Assert(err, qt.IsNil)
	return inv.ID
}
