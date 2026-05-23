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

// newTestEmailVerification builds a valid EmailVerification for the postgres
// backend. The user/tenant IDs must reference real rows (FK constraints
// fk_email_verification_user / fk_email_verification_tenant), and token must be
// unique per record (UNIQUE index). UUID is left empty so the registry layer
// fills it: store.NonRLSRepository.Create generates one server-side, with the
// DB gen_random_uuid() default as a fallback.
func newTestEmailVerification(user *models.User, token string) models.EmailVerification {
	return models.EmailVerification{
		UserID:    user.ID,
		TenantID:  user.TenantID,
		Email:     "verify@test-org.com",
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func TestEmailVerificationRegistry_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-happy"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.Token, qt.Equals, "token-happy")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
}

func TestEmailVerificationRegistry_Create_MissingFields(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	cases := []struct {
		name string
		mut  func(*models.EmailVerification)
	}{
		{"user_id empty", func(ev *models.EmailVerification) { ev.UserID = "" }},
		{"tenant_id empty", func(ev *models.EmailVerification) { ev.TenantID = "" }},
		{"token empty", func(ev *models.EmailVerification) { ev.Token = "" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			ev := newTestEmailVerification(user, "token-missing")
			tc.mut(&ev)
			_, err := registrySet.EmailVerificationRegistry.Create(ctx, ev)
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestEmailVerificationRegistry_Get(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-get"))
	c.Assert(err, qt.IsNil)

	fetched, err := registrySet.EmailVerificationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)
	c.Assert(fetched.Token, qt.Equals, "token-get")
}

func TestEmailVerificationRegistry_Get_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()

	_, err := registrySet.EmailVerificationRegistry.Get(ctx, "no-such-id")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestEmailVerificationRegistry_List(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	_, err := registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-list-1"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-list-2"))
	c.Assert(err, qt.IsNil)

	all, err := registrySet.EmailVerificationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 2)
}

func TestEmailVerificationRegistry_GetByToken(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-by-token"))
	c.Assert(err, qt.IsNil)

	fetched, err := registrySet.EmailVerificationRegistry.GetByToken(ctx, "token-by-token")
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	_, err = registrySet.EmailVerificationRegistry.GetByToken(ctx, "missing-token")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestEmailVerificationRegistry_GetByUserID(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	_, err := registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-by-user"))
	c.Assert(err, qt.IsNil)

	found, err := registrySet.EmailVerificationRegistry.GetByUserID(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(found, qt.HasLen, 1)
	c.Assert(found[0].UserID, qt.Equals, user.ID)

	empty, err := registrySet.EmailVerificationRegistry.GetByUserID(ctx, "user-unknown")
	c.Assert(err, qt.IsNil)
	c.Assert(empty, qt.HasLen, 0)
}

func TestEmailVerificationRegistry_Update(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-update"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.VerifiedAt, qt.IsNil)

	verifiedAt := time.Now()
	created.VerifiedAt = &verifiedAt
	_, err = registrySet.EmailVerificationRegistry.Update(ctx, *created)
	c.Assert(err, qt.IsNil)

	reloaded, err := registrySet.EmailVerificationRegistry.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.VerifiedAt, qt.IsNotNil)
	c.Assert(reloaded.IsVerified(), qt.IsTrue)
}

// TestEmailVerificationRegistry_Update_NotFound pins parity with the memory
// backend: Update against an unknown ID returns registry.ErrNotFound rather
// than silently succeeding with a zero-row UPDATE. See #1814.
func TestEmailVerificationRegistry_Update_NotFound(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	ev := newTestEmailVerification(user, "token-update-missing")
	ev.ID = "no-such-id"
	_, err := registrySet.EmailVerificationRegistry.Update(ctx, ev)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestEmailVerificationRegistry_Delete(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	created, err := registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-delete"))
	c.Assert(err, qt.IsNil)

	err = registrySet.EmailVerificationRegistry.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	_, err = registrySet.EmailVerificationRegistry.Get(ctx, created.ID)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

// TestEmailVerificationRegistry_DeleteExpired pins that DeleteExpired removes
// only records whose ExpiresAt is in the past and keeps future-dated ones.
func TestEmailVerificationRegistry_DeleteExpired(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	expired := newTestEmailVerification(user, "token-expired")
	expired.ExpiresAt = time.Now().Add(-1 * time.Hour)
	_, err := registrySet.EmailVerificationRegistry.Create(ctx, expired)
	c.Assert(err, qt.IsNil)

	future := newTestEmailVerification(user, "token-future")
	future.ExpiresAt = time.Now().Add(1 * time.Hour)
	futureCreated, err := registrySet.EmailVerificationRegistry.Create(ctx, future)
	c.Assert(err, qt.IsNil)

	err = registrySet.EmailVerificationRegistry.DeleteExpired(ctx)
	c.Assert(err, qt.IsNil)

	all, err := registrySet.EmailVerificationRegistry.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(all[0].ID, qt.Equals, futureCreated.ID)
}

func TestEmailVerificationRegistry_Count(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	count, err := registrySet.EmailVerificationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	_, err = registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-count-1"))
	c.Assert(err, qt.IsNil)
	_, err = registrySet.EmailVerificationRegistry.Create(ctx, newTestEmailVerification(user, "token-count-2"))
	c.Assert(err, qt.IsNil)

	count, err = registrySet.EmailVerificationRegistry.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}
