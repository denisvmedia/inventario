package postgres_test

import (
	"context"
	"errors"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// newTestOAuthIdentity builds a valid OAuthIdentity for the postgres backend.
// The user/tenant IDs must reference real rows (FK constraints
// fk_oauth_identity_user / fk_entity_tenant), and the (provider,
// provider_user_id) pair must be unique per record (UNIQUE index).
func newTestOAuthIdentity(user *models.User, provider models.OAuthProvider, providerUserID string) models.OAuthIdentity {
	return models.OAuthIdentity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: user.TenantID,
		},
		UserID:         user.ID,
		Provider:       provider,
		ProviderUserID: providerUserID,
		Email:          "oauth@test-org.com",
	}
}

func TestOAuthIdentityRegistry_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "google-happy"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.Provider, qt.Equals, models.OAuthProviderGoogle)
	c.Assert(created.ProviderUserID, qt.Equals, "google-happy")
	c.Assert(created.LinkedAt.IsZero(), qt.IsFalse)
}

func TestOAuthIdentityRegistry_Create_MissingFields(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	cases := []struct {
		name string
		mut  func(*models.OAuthIdentity)
	}{
		{"user_id empty", func(oi *models.OAuthIdentity) { oi.UserID = "" }},
		{"tenant_id empty", func(oi *models.OAuthIdentity) { oi.TenantID = "" }},
		{"provider empty", func(oi *models.OAuthIdentity) { oi.Provider = "" }},
		{"provider unknown", func(oi *models.OAuthIdentity) { oi.Provider = models.OAuthProvider("twitter") }},
		{"provider_user_id empty", func(oi *models.OAuthIdentity) { oi.ProviderUserID = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			oi := newTestOAuthIdentity(user, models.OAuthProviderGoogle, "google-missing")
			tc.mut(&oi)
			_, err := registrySet.OAuthIdentityRegistry.Create(ctx, oi)
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

// TestOAuthIdentityRegistry_Create_DuplicateProviderSubject_PG pins that the
// postgres unique-constraint on (provider, provider_user_id) trips and is
// mapped onto registry.ErrAlreadyExists (SQLSTATE 23505 → sentinel).
func TestOAuthIdentityRegistry_Create_DuplicateProviderSubject(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	_, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "duplicate-sub"))
	c.Assert(err, qt.IsNil)

	// Same provider + provider_user_id; user/tenant don't matter because
	// the uniqueness constraint is global.
	dup := newTestOAuthIdentity(user, models.OAuthProviderGoogle, "duplicate-sub")
	_, err = registrySet.OAuthIdentityRegistry.Create(ctx, dup)
	c.Assert(errors.Is(err, registry.ErrAlreadyExists), qt.IsTrue)
}

func TestOAuthIdentityRegistry_Get(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "g-get"))
	c.Assert(err, qt.IsNil)

	fetched, err := registrySet.OAuthIdentityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)
	c.Assert(fetched.ProviderUserID, qt.Equals, "g-get")
}

func TestOAuthIdentityRegistry_Get_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	_, err := registrySet.OAuthIdentityRegistry.Get(ctx, "no-such-id")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestOAuthIdentityRegistry_GetByProviderSubject(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGitHub, "gh-42"))
	c.Assert(err, qt.IsNil)

	fetched, err := registrySet.OAuthIdentityRegistry.GetByProviderSubject(ctx, models.OAuthProviderGitHub, "gh-42")
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	_, err = registrySet.OAuthIdentityRegistry.GetByProviderSubject(ctx, models.OAuthProviderGoogle, "gh-42")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	_, err = registrySet.OAuthIdentityRegistry.GetByProviderSubject(ctx, models.OAuthProviderGitHub, "missing")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

// TestOAuthIdentityRegistry_ListByUser_TenantScoping pins the
// defense-in-depth tenant filter at the postgres layer: even though service
// mode bypasses RLS, the explicit `tenant_id = $1` qualifier in
// ListByUser must hide rows from other tenants.
func TestOAuthIdentityRegistry_ListByUser_TenantScoping(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	_, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "mine-google"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGitHub, "mine-github"))
	c.Assert(err, qt.IsNil)

	rows, err := registrySet.OAuthIdentityRegistry.ListByUser(ctx, user.TenantID, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 2)

	// Wrong tenant — defense-in-depth tenant filter must hide the rows.
	rows, err = registrySet.OAuthIdentityRegistry.ListByUser(ctx, "tenant-unknown", user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 0)

	// Empty inputs short-circuit to no results (defense-in-depth for a
	// caller that forgot to populate ctx).
	rows, err = registrySet.OAuthIdentityRegistry.ListByUser(ctx, "", user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 0)
}

func TestOAuthIdentityRegistry_GetByUserAndProvider(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "g-sub"))
	c.Assert(err, qt.IsNil)

	fetched, err := registrySet.OAuthIdentityRegistry.GetByUserAndProvider(ctx, user.TenantID, user.ID, models.OAuthProviderGoogle)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	// Wrong tenant — defense-in-depth tenant filter must hide the row.
	_, err = registrySet.OAuthIdentityRegistry.GetByUserAndProvider(ctx, "tenant-unknown", user.ID, models.OAuthProviderGoogle)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	// Wrong provider.
	_, err = registrySet.OAuthIdentityRegistry.GetByUserAndProvider(ctx, user.TenantID, user.ID, models.OAuthProviderGitHub)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

// TestOAuthIdentityRegistry_DeleteByUserAndProvider_Idempotent pins parity
// with the memory backend: second delete on an already removed row returns
// nil, not ErrNotFound.
func TestOAuthIdentityRegistry_DeleteByUserAndProvider_Idempotent(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	_, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "g-sub"))
	c.Assert(err, qt.IsNil)

	err = registrySet.OAuthIdentityRegistry.DeleteByUserAndProvider(ctx, user.TenantID, user.ID, models.OAuthProviderGoogle)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.OAuthIdentityRegistry.GetByUserAndProvider(ctx, user.TenantID, user.ID, models.OAuthProviderGoogle)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	// Second delete is a no-op.
	err = registrySet.OAuthIdentityRegistry.DeleteByUserAndProvider(ctx, user.TenantID, user.ID, models.OAuthProviderGoogle)
	c.Assert(err, qt.IsNil)
}

func TestOAuthIdentityRegistry_Update(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "g-update"))
	c.Assert(err, qt.IsNil)

	created.Email = "rotated@test-org.com"
	_, err = registrySet.OAuthIdentityRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)

	reloaded, err := registrySet.OAuthIdentityRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.Email, qt.Equals, "rotated@test-org.com")
}

func TestOAuthIdentityRegistry_Delete(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "g-delete"))
	c.Assert(err, qt.IsNil)

	err = registrySet.OAuthIdentityRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.OAuthIdentityRegistry.Get(ctx, created.ID)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestOAuthIdentityRegistry_List(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	_, err := registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "g-list-1"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGitHub, "gh-list-1"))
	c.Assert(err, qt.IsNil)

	all, err := registrySet.OAuthIdentityRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 2)
}

func TestOAuthIdentityRegistry_Count(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	count, err := registrySet.OAuthIdentityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	_, err = registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGoogle, "g-count-1"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.OAuthIdentityRegistry.Create(ctx, newTestOAuthIdentity(user, models.OAuthProviderGitHub, "gh-count-1"))
	c.Assert(err, qt.IsNil)

	count, err = registrySet.OAuthIdentityRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}
