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

// newTestBackofficeUser builds a valid BackofficeUser for the postgres
// backend. backoffice_users has NO foreign keys — back-office identities
// live OUTSIDE the tenant model — so the row stands alone without any
// preceding tenant / user setup. The PasswordHash is a placeholder
// string; bcrypt correctness lives in models.User's own tests.
func newTestBackofficeUser(email string) models.BackofficeUser {
	return models.BackofficeUser{
		Email:        email,
		Name:         "Operator",
		PasswordHash: "$2a$10$placeholderhashvalue",
		Role:         models.BackofficeRolePlatformAdmin,
		IsActive:     true,
		MFAEnforced:  true,
	}
}

func TestBackofficeUserRegistryPostgres_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("ops@example.com"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.Email, qt.Equals, "ops@example.com")
	c.Assert(created.Role, qt.Equals, models.BackofficeRolePlatformAdmin)
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
}

func TestBackofficeUserRegistryPostgres_Create_LowercasesEmail(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("Ops@Example.COM"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.Email, qt.Equals, "ops@example.com")
}

func TestBackofficeUserRegistryPostgres_Create_MissingFields(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)
	ctx := context.Background()

	cases := []struct {
		name string
		mut  func(*models.BackofficeUser)
	}{
		{"email empty", func(u *models.BackofficeUser) { u.Email = "" }},
		{"name empty", func(u *models.BackofficeUser) { u.Name = "" }},
		{"password_hash empty", func(u *models.BackofficeUser) { u.PasswordHash = "" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			u := newTestBackofficeUser("missing@example.com")
			tc.mut(&u)
			_, err := bo.Create(ctx, u)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestBackofficeUserRegistryPostgres_Create_InvalidRole(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	u := newTestBackofficeUser("role@example.com")
	u.Role = "no-such-role"
	_, err := bo.Create(ctx, u)
	c.Assert(errors.Is(err, registry.ErrInvalidBackofficeRole), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_Create_DuplicateEmail(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	_, err := bo.Create(ctx, newTestBackofficeUser("dup@example.com"))
	c.Assert(err, qt.IsNil)

	_, err = bo.Create(ctx, newTestBackofficeUser("DUP@example.com"))
	c.Assert(errors.Is(err, registry.ErrBackofficeEmailAlreadyExists), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_Get(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("get@example.com"))
	c.Assert(err, qt.IsNil)

	fetched, err := bo.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)
}

func TestBackofficeUserRegistryPostgres_Get_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	_, err := bo.Get(ctx, "no-such-id")
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_GetByEmail_CaseInsensitive(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	_, err := bo.Create(ctx, newTestBackofficeUser("case@example.com"))
	c.Assert(err, qt.IsNil)

	fetched, err := bo.GetByEmail(ctx, "CASE@Example.COM")
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.Email, qt.Equals, "case@example.com")
}

func TestBackofficeUserRegistryPostgres_GetByEmail_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	_, err := bo.GetByEmail(ctx, "missing@example.com")
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_Update_PreservesPasswordHash(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("upd@example.com"))
	c.Assert(err, qt.IsNil)
	originalHash := created.PasswordHash

	updated := *created
	updated.PasswordHash = ""
	updated.Name = "Renamed"

	got, err := bo.Update(ctx, updated)
	c.Assert(err, qt.IsNil)
	c.Assert(got.PasswordHash, qt.Equals, originalHash)
	c.Assert(got.Name, qt.Equals, "Renamed")

	reloaded, err := bo.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.PasswordHash, qt.Equals, originalHash)
	c.Assert(reloaded.Name, qt.Equals, "Renamed")
}

func TestBackofficeUserRegistryPostgres_Update_RejectsEmailCollision(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	_, err := bo.Create(ctx, newTestBackofficeUser("first@example.com"))
	c.Assert(err, qt.IsNil)
	second, err := bo.Create(ctx, newTestBackofficeUser("second@example.com"))
	c.Assert(err, qt.IsNil)

	second.Email = "first@example.com"
	_, err = bo.Update(ctx, *second)
	c.Assert(errors.Is(err, registry.ErrBackofficeEmailAlreadyExists), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_Delete(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("del@example.com"))
	c.Assert(err, qt.IsNil)

	c.Assert(bo.Delete(ctx, created.ID), qt.IsNil)
	_, err = bo.Get(ctx, created.ID)
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_Count(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	count, err := bo.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	_, err = bo.Create(ctx, newTestBackofficeUser("a@example.com"))
	c.Assert(err, qt.IsNil)
	_, err = bo.Create(ctx, newTestBackofficeUser("b@example.com"))
	c.Assert(err, qt.IsNil)

	count, err = bo.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

func TestBackofficeUserRegistryPostgres_SetPasswordHash(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("hash@example.com"))
	c.Assert(err, qt.IsNil)

	c.Assert(bo.SetPasswordHash(ctx, created.ID, "$2a$10$newvalue"), qt.IsNil)
	fetched, err := bo.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.PasswordHash, qt.Equals, "$2a$10$newvalue")
}

func TestBackofficeUserRegistryPostgres_SetPasswordHash_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	err := bo.SetPasswordHash(ctx, "no-such-id", "$2a$10$x")
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_UpdateLastLogin(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("login@example.com"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.LastLoginAt, qt.IsNil)

	at := time.Now().UTC().Truncate(time.Second)
	c.Assert(bo.UpdateLastLogin(ctx, created.ID, at), qt.IsNil)

	fetched, err := bo.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.LastLoginAt, qt.IsNotNil)
	c.Assert(fetched.LastLoginAt.UTC().Truncate(time.Second).Equal(at), qt.IsTrue)
}

func TestBackofficeUserRegistryPostgres_SetActive(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("act@example.com"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.IsActive, qt.IsTrue)

	c.Assert(bo.SetActive(ctx, created.ID, false), qt.IsNil)
	fetched, err := bo.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.IsActive, qt.IsFalse)
}

// getBackofficeRegistry resolves the BackofficeUserRegistry via the
// underlying factory set. The per-request *Set this package's helpers
// produce only carries user-aware data, while back-office identities
// live on FactorySet (they're cross-cutting infra, not user-aware) —
// so we rebuild a factory set from the same pool the setupTest helper
// used and grab the registry off it. The rows we write here share the
// database with the helper's test tenant rows but never overlap them
// (no FK, separate table).
func getBackofficeRegistry(t *testing.T, _ *registry.Set) registry.BackofficeUserRegistry {
	t.Helper()
	dsn := skipIfNoPostgreSQL(t)
	c := qt.New(t)
	pool, err := getOrCreatePool(dsn)
	c.Assert(err, qt.IsNil)
	fs := createRegistrySetFromPool(pool)
	return fs.BackofficeUserRegistry
}
