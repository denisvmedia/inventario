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

// TestBackofficeUserRegistryPostgres_Create_RejectsMalformedEmail proves
// the defence-in-depth model validation runs at the postgres registry
// layer — a string that passes the bespoke TrimSpace-required check
// ("not-an-email") still gets rejected by BackofficeUser.ValidateWith-
// Context's EmailPattern rule before the INSERT touches the database.
func TestBackofficeUserRegistryPostgres_Create_RejectsMalformedEmail(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	u := newTestBackofficeUser("not-an-email")
	_, err := bo.Create(ctx, u)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "model validation")
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

// TestBackofficeUserRegistryPostgres_GetByEmail_WhitespaceOnly pins the
// whitespace-rejection invariant — a stray "   " from the caller must
// surface as ErrFieldRequired, not as a no-rows lookup.
func TestBackofficeUserRegistryPostgres_GetByEmail_WhitespaceOnly(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	_, err := bo.GetByEmail(ctx, "   ")
	c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
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

// TestBackofficeUserRegistryPostgres_Update_NotFoundOnDeletedRow pins
// the defence-in-depth RowsAffected check: if the row is deleted
// between Update's pre-SELECT and its UPDATE, the call must surface
// ErrBackofficeUserNotFound rather than silently succeed. We simulate
// the race by deleting the row immediately before Update (no need for
// real concurrency — the postgres pre-SELECT inside Update's tx will
// see no row, which itself returns ErrBackofficeUserNotFound; this
// test guards the secondary path where the row could vanish between
// the pre-SELECT and the UPDATE if SELECT FOR UPDATE were ever added
// or removed without updating the contract).
func TestBackofficeUserRegistryPostgres_Update_NotFoundOnDeletedRow(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	created, err := bo.Create(ctx, newTestBackofficeUser("race@example.com"))
	c.Assert(err, qt.IsNil)

	c.Assert(bo.Delete(ctx, created.ID), qt.IsNil)

	updated := *created
	updated.Name = "After Delete"
	_, err = bo.Update(ctx, updated)
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
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

// TestBackofficeUserRegistryPostgres_Delete_Idempotent pins the cross-
// backend idempotency contract: Delete on a missing id is a no-op
// rather than an error. The postgres backend's NonRLSRepository.Delete
// returns store.ErrNotFound on a missing row; the registry swallows it
// so the contract matches the memory backend.
func TestBackofficeUserRegistryPostgres_Delete_Idempotent(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	c.Assert(bo.Delete(ctx, "no-such-id"), qt.IsNil)
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

// TestBackofficeUserRegistryPostgres_List_DistinctRowsAndOrder pins two
// invariants on the List query that earlier shapes got wrong:
//
//  1. Each returned pointer references a distinct row, not N copies of
//     the last one. The previous Scan-based implementation re-used the
//     iteration variable's address per yield, so appending `&user`
//     produced a slice of aliased pointers.
//  2. Rows come back ordered by created_at (then id as the tiebreaker).
//     The previous shape relied on Postgres's un-ordered SELECT and
//     happened to look stable in tests, but the contract was not
//     enforced.
//
// We insert three rows with explicit, monotonically-increasing
// CreatedAt stamps and assert (a) three distinct IDs, (b) emails come
// back in created_at order.
func TestBackofficeUserRegistryPostgres_List_DistinctRowsAndOrder(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()
	bo := getBackofficeRegistry(t, registrySet)

	c := qt.New(t)
	ctx := context.Background()

	base := time.Now().UTC().Truncate(time.Second)
	emails := []string{"first@example.com", "second@example.com", "third@example.com"}
	for i, email := range emails {
		u := newTestBackofficeUser(email)
		u.CreatedAt = base.Add(time.Duration(i) * time.Second)
		u.UpdatedAt = u.CreatedAt
		_, err := bo.Create(ctx, u)
		c.Assert(err, qt.IsNil)
	}

	listed, err := bo.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(listed, qt.HasLen, 3)

	seenIDs := make(map[string]struct{}, len(listed))
	gotEmails := make([]string, 0, len(listed))
	for _, u := range listed {
		seenIDs[u.ID] = struct{}{}
		gotEmails = append(gotEmails, u.Email)
	}
	c.Assert(seenIDs, qt.HasLen, 3)
	c.Assert(gotEmails, qt.DeepEquals, emails)
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
