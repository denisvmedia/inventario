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

// newTestBackofficeRefreshToken builds a valid refresh-token row for the
// in-memory backend. Mirrors the postgres helper of the same name —
// the only required FK target is a backoffice_user_id, and the memory
// backend doesn't enforce referential integrity, so the caller can pass
// any non-empty string.
func newTestBackofficeRefreshToken(backofficeUserID, tokenHash string) models.BackofficeRefreshToken {
	return models.BackofficeRefreshToken{
		BackofficeUserID: backofficeUserID,
		TokenHash:        tokenHash,
		ExpiresAt:        time.Now().Add(time.Hour),
		IPAddress:        "10.0.0.1",
		UserAgent:        "go-test",
	}
}

func TestBackofficeRefreshTokenRegistry_Create_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "hash-1"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.TokenHash, qt.Equals, "hash-1")
	c.Assert(created.BackofficeUserID, qt.Equals, "user-1")
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
	c.Assert(created.RevokedAt, qt.IsNil)
	c.Assert(created.IsValid(), qt.IsTrue)
}

func TestBackofficeRefreshTokenRegistry_Create_MissingFields(t *testing.T) {
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
			r := memory.NewBackofficeRefreshTokenRegistry()
			tok := newTestBackofficeRefreshToken("user-1", "hash-1")
			tc.mut(&tok)
			_, err := r.Create(ctx, tok)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestBackofficeRefreshTokenRegistry_GetByHash_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "hash-target"))
	c.Assert(err, qt.IsNil)

	got, err := r.GetByHash(ctx, "hash-target")
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, created.ID)
	c.Assert(got.BackofficeUserID, qt.Equals, "user-1")
}

func TestBackofficeRefreshTokenRegistry_GetByHash_NotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	_, err := r.GetByHash(ctx, "nope")
	c.Assert(errors.Is(err, registry.ErrBackofficeRefreshTokenNotFound), qt.IsTrue)
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestBackofficeRefreshTokenRegistry_Revoke_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "hash-1"))
	c.Assert(err, qt.IsNil)

	err = r.Revoke(ctx, "user-1", created.ID)
	c.Assert(err, qt.IsNil)

	got, err := r.GetByHash(ctx, "hash-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got.RevokedAt, qt.IsNotNil)
	c.Assert(got.IsValid(), qt.IsFalse)
}

// TestBackofficeRefreshTokenRegistry_Revoke_Idempotent pins the
// already-revoked-is-success branch — re-revoking a row must not error.
func TestBackofficeRefreshTokenRegistry_Revoke_Idempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "hash-1"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.Revoke(ctx, "user-1", created.ID), qt.IsNil)
	c.Assert(r.Revoke(ctx, "user-1", created.ID), qt.IsNil)
}

// TestBackofficeRefreshTokenRegistry_Revoke_WrongUser pins the cross-user
// gating invariant — a guessed id matched to the wrong user must 404
// rather than revoke the row.
func TestBackofficeRefreshTokenRegistry_Revoke_WrongUser(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	created, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "hash-1"))
	c.Assert(err, qt.IsNil)

	err = r.Revoke(ctx, "other-user", created.ID)
	c.Assert(errors.Is(err, registry.ErrBackofficeRefreshTokenNotFound), qt.IsTrue)

	got, err := r.GetByHash(ctx, "hash-1")
	c.Assert(err, qt.IsNil)
	c.Assert(got.RevokedAt, qt.IsNil)
}

func TestBackofficeRefreshTokenRegistry_ListActive(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	// Three rows for user-1: one active, one revoked, one expired.
	active, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "active"))
	c.Assert(err, qt.IsNil)
	revoked, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "revoked"))
	c.Assert(err, qt.IsNil)
	expired := newTestBackofficeRefreshToken("user-1", "expired")
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	_, err = r.Create(ctx, expired)
	c.Assert(err, qt.IsNil)
	// One unrelated row for user-2 to verify scoping.
	_, err = r.Create(ctx, newTestBackofficeRefreshToken("user-2", "other"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.Revoke(ctx, "user-1", revoked.ID), qt.IsNil)

	out, err := r.ListActiveByBackofficeUserID(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.HasLen, 1)
	c.Assert(out[0].ID, qt.Equals, active.ID)
}

func TestBackofficeRefreshTokenRegistry_RevokeByBackofficeUserID(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	_, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "h1"))
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestBackofficeRefreshToken("user-1", "h2"))
	c.Assert(err, qt.IsNil)
	other, err := r.Create(ctx, newTestBackofficeRefreshToken("user-2", "other"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.RevokeByBackofficeUserID(ctx, "user-1"), qt.IsNil)

	out, err := r.ListActiveByBackofficeUserID(ctx, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(out, qt.HasLen, 0)

	// Other user's row is untouched.
	got, err := r.GetByHash(ctx, "other")
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, other.ID)
	c.Assert(got.RevokedAt, qt.IsNil)
}

func TestBackofficeRefreshTokenRegistry_DeleteExpired(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeRefreshTokenRegistry()

	live, err := r.Create(ctx, newTestBackofficeRefreshToken("user-1", "live"))
	c.Assert(err, qt.IsNil)
	expired := newTestBackofficeRefreshToken("user-1", "expired")
	expired.ExpiresAt = time.Now().Add(-time.Minute)
	_, err = r.Create(ctx, expired)
	c.Assert(err, qt.IsNil)

	c.Assert(r.DeleteExpired(ctx), qt.IsNil)

	_, err = r.GetByHash(ctx, "expired")
	c.Assert(errors.Is(err, registry.ErrBackofficeRefreshTokenNotFound), qt.IsTrue)

	got, err := r.GetByHash(ctx, "live")
	c.Assert(err, qt.IsNil)
	c.Assert(got.ID, qt.Equals, live.ID)
}
