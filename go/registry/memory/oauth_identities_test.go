package memory_test

import (
	"context"
	"testing"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// newTestOAuthIdentity builds a valid OAuthIdentity with the given provider
// and provider_user_id. The memory backend has no FK constraints, so the
// tenant/user IDs are arbitrary — but realistic values keep the tests
// readable.
func newTestOAuthIdentity(provider models.OAuthProvider, providerUserID string) models.OAuthIdentity {
	return models.OAuthIdentity{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: "tenant-1",
		},
		UserID:         "user-1",
		Provider:       provider,
		ProviderUserID: providerUserID,
		Email:          "user@example.com",
	}
}

func TestOAuthIdentityRegistry_Create_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	created, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "google-sub-1"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.Provider, qt.Equals, models.OAuthProviderGoogle)
	c.Assert(created.ProviderUserID, qt.Equals, "google-sub-1")
	c.Assert(created.LinkedAt.IsZero(), qt.IsFalse)
}

func TestOAuthIdentityRegistry_Create_MissingFields(t *testing.T) {
	ctx := context.Background()

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
			r := memory.NewOAuthIdentityRegistry()
			oi := newTestOAuthIdentity(models.OAuthProviderGoogle, "google-sub-missing")
			tc.mut(&oi)
			_, err := r.Create(ctx, oi)
			c.Assert(err, qt.IsNotNil)
			c.Assert(err, qt.ErrorIs, registry.ErrFieldRequired)
		})
	}
}

// TestOAuthIdentityRegistry_Create_DuplicateProviderSubject pins that the
// (provider, provider_user_id) pair is globally unique — a duplicate Create
// surfaces ErrAlreadyExists so the callback can disambiguate "first link"
// from "already attached" without an extra round-trip.
func TestOAuthIdentityRegistry_Create_DuplicateProviderSubject(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	_, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "duplicate-sub"))
	c.Assert(err, qt.IsNil)

	// Same provider + provider_user_id but a different user — the
	// uniqueness key is independent of (tenant_id, user_id).
	dup := newTestOAuthIdentity(models.OAuthProviderGoogle, "duplicate-sub")
	dup.UserID = "user-other"
	_, err = r.Create(ctx, dup)
	c.Assert(err, qt.ErrorIs, registry.ErrAlreadyExists)
}

func TestOAuthIdentityRegistry_GetByProviderSubject(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	created, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGitHub, "gh-42"))
	c.Assert(err, qt.IsNil)

	fetched, err := r.GetByProviderSubject(ctx, models.OAuthProviderGitHub, "gh-42")
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	_, err = r.GetByProviderSubject(ctx, models.OAuthProviderGoogle, "gh-42")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	_, err = r.GetByProviderSubject(ctx, models.OAuthProviderGitHub, "missing")
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// TestOAuthIdentityRegistry_ListByUser_TenantScoping pins the
// defense-in-depth tenant filter: a row owned by the same userID but in a
// different tenant must NOT leak through ListByUser.
func TestOAuthIdentityRegistry_ListByUser_TenantScoping(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	mine := newTestOAuthIdentity(models.OAuthProviderGoogle, "mine-google")
	_, err := r.Create(ctx, mine)
	c.Assert(err, qt.IsNil)

	mineGH := newTestOAuthIdentity(models.OAuthProviderGitHub, "mine-github")
	_, err = r.Create(ctx, mineGH)
	c.Assert(err, qt.IsNil)

	otherTenant := newTestOAuthIdentity(models.OAuthProviderGoogle, "other-tenant-google")
	otherTenant.TenantID = "tenant-other"
	_, err = r.Create(ctx, otherTenant)
	c.Assert(err, qt.IsNil)

	otherUser := newTestOAuthIdentity(models.OAuthProviderGoogle, "other-user-google")
	otherUser.UserID = "user-other"
	_, err = r.Create(ctx, otherUser)
	c.Assert(err, qt.IsNil)

	rows, err := r.ListByUser(ctx, "tenant-1", "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 2)
	// Provider asc ordering.
	c.Assert(rows[0].Provider, qt.Equals, models.OAuthProviderGitHub)
	c.Assert(rows[1].Provider, qt.Equals, models.OAuthProviderGoogle)

	// Empty tenant/user IDs short-circuit to no results (defense-in-depth
	// for a caller that forgot to populate ctx).
	rows, err = r.ListByUser(ctx, "", "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 0)

	rows, err = r.ListByUser(ctx, "tenant-1", "")
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 0)
}

func TestOAuthIdentityRegistry_GetByUserAndProvider(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	created, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "g-sub"))
	c.Assert(err, qt.IsNil)

	fetched, err := r.GetByUserAndProvider(ctx, "tenant-1", "user-1", models.OAuthProviderGoogle)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	// Wrong tenant — defense-in-depth tenant filter must hide the row.
	_, err = r.GetByUserAndProvider(ctx, "tenant-other", "user-1", models.OAuthProviderGoogle)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Wrong user.
	_, err = r.GetByUserAndProvider(ctx, "tenant-1", "user-other", models.OAuthProviderGoogle)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Wrong provider (user has no GitHub identity).
	_, err = r.GetByUserAndProvider(ctx, "tenant-1", "user-1", models.OAuthProviderGitHub)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

// TestOAuthIdentityRegistry_DeleteByUserAndProvider_Idempotent pins the
// contract documented on the interface: the second call against an already
// removed row returns nil, not ErrNotFound, so handler code doesn't need to
// branch.
func TestOAuthIdentityRegistry_DeleteByUserAndProvider_Idempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	_, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "g-sub"))
	c.Assert(err, qt.IsNil)

	err = r.DeleteByUserAndProvider(ctx, "tenant-1", "user-1", models.OAuthProviderGoogle)
	c.Assert(err, qt.IsNil)

	// The row must be gone.
	_, err = r.GetByUserAndProvider(ctx, "tenant-1", "user-1", models.OAuthProviderGoogle)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// Second delete is a no-op.
	err = r.DeleteByUserAndProvider(ctx, "tenant-1", "user-1", models.OAuthProviderGoogle)
	c.Assert(err, qt.IsNil)
}

func TestOAuthIdentityRegistry_Update(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	created, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "g-sub"))
	c.Assert(err, qt.IsNil)

	created.Email = "rotated@example.com"
	_, err = r.Update(ctx, *created)
	c.Assert(err, qt.IsNil)

	reloaded, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.Email, qt.Equals, "rotated@example.com")
}

func TestOAuthIdentityRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	created, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "g-sub"))
	c.Assert(err, qt.IsNil)

	err = r.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	_, err = r.Get(ctx, created.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)
}

func TestOAuthIdentityRegistry_List(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	_, err := r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "g-1"))
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGitHub, "gh-1"))
	c.Assert(err, qt.IsNil)

	all, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 2)
}

func TestOAuthIdentityRegistry_Count(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewOAuthIdentityRegistry()

	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	_, err = r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGoogle, "g-1"))
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestOAuthIdentity(models.OAuthProviderGitHub, "gh-1"))
	c.Assert(err, qt.IsNil)

	count, err = r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}
