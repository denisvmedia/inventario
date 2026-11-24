package memory_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

func newMemoryAPIToken(c *qt.C, r *memory.APITokenRegistry, userID string) (*models.APIToken, string) {
	c.Helper()
	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	created, err := r.Create(context.Background(), models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: "tenant-1", UserID: userID},
		Name:                    "script",
		TokenHash:               gen.Hash,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadOnly,
	})
	c.Assert(err, qt.IsNil)
	return created, gen.Token
}

func TestAPITokenRegistryMemory_CreateAndLookUp(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewAPITokenRegistry()

	created, secret := newMemoryAPIToken(c, r, "user-1")
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)

	got, err := r.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)

	_, err = r.GetByTokenHash(ctx, models.HashAPIToken("inv_pat_unknown"))
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound)

	// The digest is unique.
	_, err = r.Create(ctx, models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: "tenant-1", UserID: "user-2"},
		Name:                    "copy",
		TokenHash:               created.TokenHash,
		TokenPrefix:             created.TokenPrefix,
		Scope:                   models.APITokenScopeReadOnly,
	})
	c.Assert(err, qt.ErrorIs, registry.ErrAlreadyExists)
}

func TestAPITokenRegistryMemory_RevokeIsScopedToTheOwner(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewAPITokenRegistry()
	token, secret := newMemoryAPIToken(c, r, "user-1")

	_, err := r.RevokeByID(ctx, "user-2", token.ID)
	c.Assert(err, qt.ErrorIs, registry.ErrNotFound, qt.Commentf("another user revoked the token"))

	revoked, err := r.RevokeByID(ctx, "user-1", token.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(revoked.RevokedAt, qt.IsNotNil)

	// Revoking twice keeps the first revocation time.
	again, err := r.RevokeByID(ctx, "user-1", token.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(again.RevokedAt.Equal(*revoked.RevokedAt), qt.IsTrue)

	// The row stays findable by digest so authentication can tell a revoked
	// token from an unknown one in its logs; it leaves the owner's list.
	got, err := r.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.IsNil)
	c.Assert(got.IsRevoked(), qt.IsTrue)
	list, err := r.ListByUserID(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 0)
}

func TestAPITokenRegistryMemory_ListAndCount(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewAPITokenRegistry()
	now := time.Now()

	first, _ := newMemoryAPIToken(c, r, "user-1")
	_, _ = newMemoryAPIToken(c, r, "user-1")
	_, _ = newMemoryAPIToken(c, r, "user-2")

	gen, err := models.GenerateAPIToken()
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, models.APIToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: "tenant-1", UserID: "user-1"},
		Name:                    "expired",
		TokenHash:               gen.Hash,
		TokenPrefix:             gen.DisplayPrefix,
		Scope:                   models.APITokenScopeReadOnly,
		ExpiresAt:               new(now.Add(-time.Hour)),
	})
	c.Assert(err, qt.IsNil)

	list, err := r.ListByUserID(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 3, qt.Commentf("expired tokens stay listed"))

	count, err := r.CountActiveByUserID(ctx, "user-1", now)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2, qt.Commentf("expired tokens do not count"))

	_, err = r.RevokeByID(ctx, "user-1", first.ID)
	c.Assert(err, qt.IsNil)
	count, err = r.CountActiveByUserID(ctx, "user-1", now)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 1)
}

func TestAPITokenRegistryMemory_RevokeAllByUserID(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewAPITokenRegistry()

	_, _ = newMemoryAPIToken(c, r, "user-1")
	_, _ = newMemoryAPIToken(c, r, "user-1")
	other, _ := newMemoryAPIToken(c, r, "user-2")

	n, err := r.RevokeAllByUserID(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 2)

	n, err = r.RevokeAllByUserID(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(n, qt.Equals, 0)

	list, err := r.ListByUserID(ctx, "user-2")
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 1)
	c.Assert(list[0].ID, qt.Equals, other.ID)
}

// TouchLastUsed writes only when the stored time is older than staleBefore,
// which is what keeps a busy token from writing on every request.
func TestAPITokenRegistryMemory_TouchLastUsedIsThrottled(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewAPITokenRegistry()
	token, secret := newMemoryAPIToken(c, r, "user-1")

	t0 := time.Now()
	wrote, err := r.TouchLastUsed(ctx, token.ID, t0, t0.Add(-models.APITokenLastUsedInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(wrote, qt.IsTrue, qt.Commentf("first use must be recorded"))

	t1 := t0.Add(time.Minute)
	wrote, err = r.TouchLastUsed(ctx, token.ID, t1, t1.Add(-models.APITokenLastUsedInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(wrote, qt.IsFalse, qt.Commentf("a use inside the interval must not write"))

	t2 := t0.Add(models.APITokenLastUsedInterval + time.Second)
	wrote, err = r.TouchLastUsed(ctx, token.ID, t2, t2.Add(-models.APITokenLastUsedInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(wrote, qt.IsTrue)

	got, err := r.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.IsNil)
	c.Assert(got.LastUsedAt.Equal(t2), qt.IsTrue)

	// A revoked token is never touched.
	_, err = r.RevokeByID(ctx, "user-1", token.ID)
	c.Assert(err, qt.IsNil)
	t3 := t2.Add(time.Hour)
	wrote, err = r.TouchLastUsed(ctx, token.ID, t3, t3.Add(-models.APITokenLastUsedInterval))
	c.Assert(err, qt.IsNil)
	c.Assert(wrote, qt.IsFalse)
}

// A returned row is a copy: changing it does not change the stored token.
func TestAPITokenRegistryMemory_ReturnsCopies(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewAPITokenRegistry()
	token, secret := newMemoryAPIToken(c, r, "user-1")

	token.Scope = models.APITokenScopeReadWrite
	got, err := r.GetByTokenHash(ctx, models.HashAPIToken(secret))
	c.Assert(err, qt.IsNil)
	c.Assert(got.Scope, qt.Equals, models.APITokenScopeReadOnly)
}
