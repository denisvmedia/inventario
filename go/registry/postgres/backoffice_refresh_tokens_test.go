package postgres_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// newTestBackofficeRefreshToken builds a valid refresh-token row for the
// postgres backend, FK'd to the supplied backoffice_user_id. Mirrors the
// memory-side helper.
func newTestBackofficeRefreshToken(backofficeUserID, tokenHash string) models.BackofficeRefreshToken {
	return models.BackofficeRefreshToken{
		BackofficeUserID: backofficeUserID,
		TokenHash:        tokenHash,
		ExpiresAt:        time.Now().Add(time.Hour),
		IPAddress:        "10.0.0.1",
		UserAgent:        "go-test",
	}
}

// createBackofficeUserForRefreshTest provisions a back-office user the
// refresh-token rows can FK to. Returns the row id. Each call uses a
// fresh email so concurrent table-driven cases never collide on the
// platform-wide unique index.
func createBackofficeUserForRefreshTest(t *testing.T, registrySet *registry.Set, email string) string {
	t.Helper()
	c := qt.New(t)
	bo := getBackofficeRegistry(t, registrySet)
	user, err := bo.Create(context.Background(), newTestBackofficeUser(email))
	c.Assert(err, qt.IsNil)
	return user.ID
}

// getBackofficeRefreshTokenRegistry mirrors getBackofficeRegistry — the
// per-request *Set only carries user-aware data, and back-office refresh
// tokens are cross-cutting infra, so we resolve via a fresh factory set.
func getBackofficeRefreshTokenRegistry(t *testing.T, _ *registry.Set) registry.BackofficeRefreshTokenRegistry {
	t.Helper()
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	fs := createRegistrySetFromPool(pool)
	return fs.BackofficeRefreshTokenRegistry
}

func TestBackofficeRefreshTokenRegistryPostgres_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-create@example.com")

	c := qt.New(t)
	ctx := context.Background()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "hash-create"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.TokenHash, qt.Equals, "hash-create")
	c.Assert(created.BackofficeUserID, qt.Equals, userID)
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
	c.Assert(created.IsValid(), qt.IsTrue)
}

func TestBackofficeRefreshTokenRegistryPostgres_Create_MissingFields(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	ctx := context.Background()

	cases := []struct {
		name string
		mut  func(*models.BackofficeRefreshToken)
	}{
		{"token_hash empty", func(t *models.BackofficeRefreshToken) { t.TokenHash = "" }},
		{"backoffice_user_id empty", func(t *models.BackofficeRefreshToken) { t.BackofficeUserID = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			tok := newTestBackofficeRefreshToken("placeholder", "hash")
			tc.mut(&tok)
			_, err := r.Create(ctx, tok)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestBackofficeRefreshTokenRegistryPostgres_GetByHash_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-byhash@example.com")

	c := qt.New(t)
	ctx := context.Background()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "hash-by-hash"))
	c.Assert(err, qt.IsNil)

	got, err := r.GetByHash(ctx, "hash-by-hash")
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.BackofficeUserID, qt.Equals, userID)
}

func TestBackofficeRefreshTokenRegistryPostgres_GetByHash_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	_, err := r.GetByHash(ctx, "no-such-hash")
	c.Assert(errors.Is(err, registry.ErrBackofficeRefreshTokenNotFound), qt.IsTrue)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestBackofficeRefreshTokenRegistryPostgres_Revoke_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-revoke@example.com")

	c := qt.New(t)
	ctx := context.Background()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "hash-revoke"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.Revoke(ctx, userID, created.ID), qt.IsNil)

	got, err := r.GetByHash(ctx, "hash-revoke")
	c.Assert(err, qt.IsNil)
	c.Assert(got.RevokedAt, qt.IsNotNil)
	c.Assert(got.IsValid(), qt.IsFalse)
}

func TestBackofficeRefreshTokenRegistryPostgres_Revoke_Idempotent(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-revoke-idem@example.com")

	c := qt.New(t)
	ctx := context.Background()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "hash-idem"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.Revoke(ctx, userID, created.ID), qt.IsNil)
	c.Assert(r.Revoke(ctx, userID, created.ID), qt.IsNil)
}

func TestBackofficeRefreshTokenRegistryPostgres_Revoke_WrongUser(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-revoke-wrong@example.com")
	otherUserID := createBackofficeUserForRefreshTest(t, registrySet, "rt-revoke-wrong-other@example.com")

	c := qt.New(t)
	ctx := context.Background()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "hash-wrong"))
	c.Assert(err, qt.IsNil)

	err = r.Revoke(ctx, otherUserID, created.ID)
	c.Assert(errors.Is(err, registry.ErrBackofficeRefreshTokenNotFound), qt.IsTrue)

	got, err := r.GetByHash(ctx, "hash-wrong")
	c.Assert(err, qt.IsNil)
	c.Assert(got.RevokedAt, qt.IsNil)
}

func TestBackofficeRefreshTokenRegistryPostgres_ListActive(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-list@example.com")
	otherUserID := createBackofficeUserForRefreshTest(t, registrySet, "rt-list-other@example.com")

	c := qt.New(t)
	ctx := context.Background()

	active, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "active-list"))
	c.Assert(err, qt.IsNil)
	revoked, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "revoked-list"))
	c.Assert(err, qt.IsNil)
	expired := newTestBackofficeRefreshToken(userID, "expired-list")
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	_, err = r.Create(ctx, expired)
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestBackofficeRefreshToken(otherUserID, "other-list"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.Revoke(ctx, userID, revoked.ID), qt.IsNil)

	out, err := r.ListActiveByBackofficeUserID(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.HasLen, 1)
	c.Assert(out[0].ID, qt.Equals, active.ID)
}

func TestBackofficeRefreshTokenRegistryPostgres_RevokeByBackofficeUserID(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-revoke-by-user@example.com")
	otherUserID := createBackofficeUserForRefreshTest(t, registrySet, "rt-revoke-by-user-other@example.com")

	c := qt.New(t)
	ctx := context.Background()

	_, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "by-user-h1"))
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestBackofficeRefreshToken(userID, "by-user-h2"))
	c.Assert(err, qt.IsNil)
	other, err := r.Create(ctx, newTestBackofficeRefreshToken(otherUserID, "by-user-other"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.RevokeByBackofficeUserID(ctx, userID), qt.IsNil)

	out, err := r.ListActiveByBackofficeUserID(ctx, userID)
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.HasLen, 0)

	got, err := r.GetByHash(ctx, "by-user-other")
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, other.ID)
	c.Assert(got.RevokedAt, qt.IsNil)
}

func TestBackofficeRefreshTokenRegistryPostgres_DeleteExpired(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	r := getBackofficeRefreshTokenRegistry(t, registrySet)
	userID := createBackofficeUserForRefreshTest(t, registrySet, "rt-delete-expired@example.com")

	c := qt.New(t)
	ctx := context.Background()

	live, err := r.Create(ctx, newTestBackofficeRefreshToken(userID, "delete-live"))
	c.Assert(err, qt.IsNil)
	expired := newTestBackofficeRefreshToken(userID, "delete-expired")
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	_, err = r.Create(ctx, expired)
	c.Assert(err, qt.IsNil)

	c.Assert(r.DeleteExpired(ctx), qt.IsNil)

	_, err = r.GetByHash(ctx, "delete-expired")
	c.Assert(errors.Is(err, registry.ErrBackofficeRefreshTokenNotFound), qt.IsTrue)

	got, err := r.GetByHash(ctx, "delete-live")
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, live.ID)
}
