package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/memory"
)

// newTestEmailVerification builds a valid EmailVerification with the given
// token. The memory backend has no FK constraints, so the tenant/user IDs are
// arbitrary — but realistic values keep the tests readable.
func newTestEmailVerification(token string) models.EmailVerification {
	return models.EmailVerification{
		UserID:    "user-1",
		TenantID:  "tenant-1",
		Email:     "user@example.com",
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
}

func TestEmailVerificationRegistry_Create_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	created, err := r.Create(ctx, newTestEmailVerification("token-happy"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.Token, qt.Equals, "token-happy")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
}

func TestEmailVerificationRegistry_Create_MissingFields(t *testing.T) {
	ctx := context.Background()

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
			r := memory.NewEmailVerificationRegistry()
			ev := newTestEmailVerification("token-missing")
			tc.mut(&ev)
			_, err := r.Create(ctx, ev)
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestEmailVerificationRegistry_Get(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	created, err := r.Create(ctx, newTestEmailVerification("token-get"))
	c.Assert(err, qt.IsNil)

	fetched, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)
	c.Assert(fetched.Token, qt.Equals, "token-get")
}

func TestEmailVerificationRegistry_Get_NotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	_, err := r.Get(ctx, "no-such-id")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestEmailVerificationRegistry_List(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	_, err := r.Create(ctx, newTestEmailVerification("token-list-1"))
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestEmailVerification("token-list-2"))
	c.Assert(err, qt.IsNil)

	all, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 2)
}

func TestEmailVerificationRegistry_GetByToken(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	created, err := r.Create(ctx, newTestEmailVerification("token-by-token"))
	c.Assert(err, qt.IsNil)

	fetched, err := r.GetByToken(ctx, "token-by-token")
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)

	_, err = r.GetByToken(ctx, "missing-token")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestEmailVerificationRegistry_GetByUserID(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	ev := newTestEmailVerification("token-by-user")
	ev.UserID = "user-target"
	_, err := r.Create(ctx, ev)
	c.Assert(err, qt.IsNil)

	found, err := r.GetByUserID(ctx, "user-target")
	c.Assert(err, qt.IsNil)
	c.Assert(found, qt.HasLen, 1)
	c.Assert(found[0].UserID, qt.Equals, "user-target")

	empty, err := r.GetByUserID(ctx, "user-unknown")
	c.Assert(err, qt.IsNil)
	c.Assert(empty, qt.HasLen, 0)
}

func TestEmailVerificationRegistry_Update(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	created, err := r.Create(ctx, newTestEmailVerification("token-update"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.VerifiedAt, qt.IsNil)

	verifiedAt := time.Now()
	created.VerifiedAt = &verifiedAt
	_, err = r.Update(ctx, *created)
	c.Assert(err, qt.IsNil)

	reloaded, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.VerifiedAt, qt.IsNotNil)
	c.Assert(reloaded.IsVerified(), qt.IsTrue)
}

func TestEmailVerificationRegistry_Update_NotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	ev := newTestEmailVerification("token-update-missing")
	ev.ID = "no-such-id"
	_, err := r.Update(ctx, ev)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestEmailVerificationRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	created, err := r.Create(ctx, newTestEmailVerification("token-delete"))
	c.Assert(err, qt.IsNil)

	err = r.Delete(ctx, created.ID)
	c.Assert(err, qt.IsNil)

	_, err = r.Get(ctx, created.ID)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

// TestEmailVerificationRegistry_DeleteExpired pins that DeleteExpired removes
// only records whose ExpiresAt is in the past and keeps future-dated ones.
func TestEmailVerificationRegistry_DeleteExpired(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	expired := newTestEmailVerification("token-expired")
	expired.ExpiresAt = time.Now().Add(-1 * time.Hour)
	_, err := r.Create(ctx, expired)
	c.Assert(err, qt.IsNil)

	future := newTestEmailVerification("token-future")
	future.ExpiresAt = time.Now().Add(1 * time.Hour)
	futureCreated, err := r.Create(ctx, future)
	c.Assert(err, qt.IsNil)

	err = r.DeleteExpired(ctx)
	c.Assert(err, qt.IsNil)

	all, err := r.List(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(all, qt.HasLen, 1)
	c.Assert(all[0].ID, qt.Equals, futureCreated.ID)
}

func TestEmailVerificationRegistry_Count(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewEmailVerificationRegistry()

	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	_, err = r.Create(ctx, newTestEmailVerification("token-count-1"))
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestEmailVerification("token-count-2"))
	c.Assert(err, qt.IsNil)

	count, err = r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}
