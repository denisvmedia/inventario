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

// newTestBackofficeUser returns a valid BackofficeUser with the given
// email. The PasswordHash isn't a real bcrypt — registry-level tests
// only check storage invariants, not bcrypt correctness (the latter is
// covered by models.User.SetPassword's own tests).
func newTestBackofficeUser(email string) models.BackofficeUser {
	return models.BackofficeUser{
		Email:        email,
		Name:         "Operator",
		PasswordHash: "$2a$10$placeholder",
		Role:         models.BackofficeRolePlatformAdmin,
		IsActive:     true,
		MFAEnforced:  true,
	}
}

func TestBackofficeUserRegistry_Create_HappyPath(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("ops@example.com"))
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.UUID, qt.Not(qt.Equals), "")
	c.Assert(created.Email, qt.Equals, "ops@example.com")
	c.Assert(created.Role, qt.Equals, models.BackofficeRolePlatformAdmin)
	c.Assert(created.CreatedAt.IsZero(), qt.IsFalse)
	c.Assert(created.UpdatedAt.IsZero(), qt.IsFalse)
	c.Assert(created.LastLoginAt, qt.IsNil)
}

// TestBackofficeUserRegistry_Create_LowercasesEmail pins the case-
// insensitivity invariant: a mixed-case email lands in the store
// lowercased so subsequent lookups + uniqueness checks collapse case
// variants.
func TestBackofficeUserRegistry_Create_LowercasesEmail(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("Ops@Example.COM"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.Email, qt.Equals, "ops@example.com")
}

func TestBackofficeUserRegistry_Create_MissingFields(t *testing.T) {
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
			r := memory.NewBackofficeUserRegistry()
			u := newTestBackofficeUser("user@example.com")
			tc.mut(&u)
			_, err := r.Create(ctx, u)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestBackofficeUserRegistry_Create_InvalidRole(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	u := newTestBackofficeUser("user@example.com")
	u.Role = "no-such-role"
	_, err := r.Create(ctx, u)
	c.Assert(errors.Is(err, registry.ErrInvalidBackofficeRole), qt.IsTrue)
}

// TestBackofficeUserRegistry_Create_RejectsMalformedEmail proves the
// defence-in-depth model validation runs at the registry layer — a
// string that passes the bespoke TrimSpace-required check ("not-an-
// email") still gets rejected by BackofficeUser.ValidateWithContext's
// EmailPattern rule. The Service.Bootstrap path also runs model
// validation; this test guards the registry against future callers
// that bypass the service layer.
func TestBackofficeUserRegistry_Create_RejectsMalformedEmail(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	u := newTestBackofficeUser("not-an-email")
	_, err := r.Create(ctx, u)
	c.Assert(err, qt.IsNotNil)
	// Model validation returns a wrapped validation.Errors — the
	// registry doesn't translate it to a typed sentinel because the
	// future HTTP layer maps validation errors generically.
	c.Assert(err.Error(), qt.Contains, "model validation")
}

// TestBackofficeUserRegistry_Create_DuplicateEmail pins the platform-wide
// uniqueness invariant — including the case-insensitive variant, since
// the second Create's email is upper-cased on input.
func TestBackofficeUserRegistry_Create_DuplicateEmail(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	_, err := r.Create(ctx, newTestBackofficeUser("ops@example.com"))
	c.Assert(err, qt.IsNil)

	_, err = r.Create(ctx, newTestBackofficeUser("Ops@Example.com"))
	c.Assert(errors.Is(err, registry.ErrBackofficeEmailAlreadyExists), qt.IsTrue)
}

func TestBackofficeUserRegistry_Get(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("get@example.com"))
	c.Assert(err, qt.IsNil)

	fetched, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, created.ID)
	c.Assert(fetched.Email, qt.Equals, "get@example.com")
}

func TestBackofficeUserRegistry_Get_NotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	_, err := r.Get(ctx, "no-such-id")
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
	// The sentinel wraps ErrNotFound, so generic callers branching on
	// the umbrella sentinel still work.
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)
}

func TestBackofficeUserRegistry_GetByEmail_CaseInsensitive(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	_, err := r.Create(ctx, newTestBackofficeUser("ops@example.com"))
	c.Assert(err, qt.IsNil)

	fetched, err := r.GetByEmail(ctx, "OPS@example.COM")
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.Email, qt.Equals, "ops@example.com")
}

func TestBackofficeUserRegistry_GetByEmail_NotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	_, err := r.GetByEmail(ctx, "missing@example.com")
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
}

// TestBackofficeUserRegistry_GetByEmail_WhitespaceOnly pins the
// whitespace-rejection invariant — a stray "   " from the caller must
// surface as ErrFieldRequired, not as a no-rows lookup.
func TestBackofficeUserRegistry_GetByEmail_WhitespaceOnly(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	_, err := r.GetByEmail(ctx, "   ")
	c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
}

func TestBackofficeUserRegistry_Update_PreservesPasswordHash(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("upd@example.com"))
	c.Assert(err, qt.IsNil)
	originalHash := created.PasswordHash

	// Simulate a caller that nulls out the hash on the way through
	// (a partial-struct update from the future HTTP layer). The
	// registry must keep the persisted hash intact.
	updated := *created
	updated.PasswordHash = ""
	updated.Name = "Renamed Operator"

	got, err := r.Update(ctx, updated)
	c.Assert(err, qt.IsNil)
	c.Assert(got.PasswordHash, qt.Equals, originalHash)
	c.Assert(got.Name, qt.Equals, "Renamed Operator")
}

// TestBackofficeUserRegistry_Update_RejectsMalformedEmail confirms the
// model-level EmailPattern check runs on Update too — a payload that
// passes the bespoke required-field check ("not-an-email") still fails
// model validation. Symmetric with the Create variant.
func TestBackofficeUserRegistry_Update_RejectsMalformedEmail(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("ok@example.com"))
	c.Assert(err, qt.IsNil)

	updated := *created
	updated.Email = "not-an-email"
	_, err = r.Update(ctx, updated)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err.Error(), qt.Contains, "model validation")
}

func TestBackofficeUserRegistry_Update_RejectsEmailCollision(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	_, err := r.Create(ctx, newTestBackofficeUser("first@example.com"))
	c.Assert(err, qt.IsNil)
	second, err := r.Create(ctx, newTestBackofficeUser("second@example.com"))
	c.Assert(err, qt.IsNil)

	second.Email = "first@example.com"
	_, err = r.Update(ctx, *second)
	c.Assert(errors.Is(err, registry.ErrBackofficeEmailAlreadyExists), qt.IsTrue)
}

func TestBackofficeUserRegistry_Delete(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("del@example.com"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.Delete(ctx, created.ID), qt.IsNil)
	_, err = r.Get(ctx, created.ID)
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
}

// TestBackofficeUserRegistry_Delete_Idempotent pins the cross-backend
// idempotency contract: a Delete on a missing id is a no-op rather than
// an error, so callers (provisioning scripts, the future admin surface)
// can re-run Delete safely. Mirrored by the postgres backend.
func TestBackofficeUserRegistry_Delete_Idempotent(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	c.Assert(r.Delete(ctx, "no-such-id"), qt.IsNil)
}

func TestBackofficeUserRegistry_Count(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	count, err := r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 0)

	_, err = r.Create(ctx, newTestBackofficeUser("a@example.com"))
	c.Assert(err, qt.IsNil)
	_, err = r.Create(ctx, newTestBackofficeUser("b@example.com"))
	c.Assert(err, qt.IsNil)

	count, err = r.Count(ctx)
	c.Assert(err, qt.IsNil)
	c.Assert(count, qt.Equals, 2)
}

func TestBackofficeUserRegistry_SetPasswordHash(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("hash@example.com"))
	c.Assert(err, qt.IsNil)

	c.Assert(r.SetPasswordHash(ctx, created.ID, "$2a$10$newhash"), qt.IsNil)
	fetched, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.PasswordHash, qt.Equals, "$2a$10$newhash")
}

func TestBackofficeUserRegistry_SetPasswordHash_NotFound(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	err := r.SetPasswordHash(ctx, "no-such-id", "$2a$10$x")
	c.Assert(errors.Is(err, registry.ErrBackofficeUserNotFound), qt.IsTrue)
}

func TestBackofficeUserRegistry_UpdateLastLogin(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("login@example.com"))
	c.Assert(err, qt.IsNil)

	at := time.Now().UTC().Truncate(time.Second)
	c.Assert(r.UpdateLastLogin(ctx, created.ID, at), qt.IsNil)

	fetched, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.LastLoginAt, qt.IsNotNil)
	c.Assert(fetched.LastLoginAt.Equal(at), qt.IsTrue)
}

func TestBackofficeUserRegistry_SetActive(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	r := memory.NewBackofficeUserRegistry()

	created, err := r.Create(ctx, newTestBackofficeUser("act@example.com"))
	c.Assert(err, qt.IsNil)
	c.Assert(created.IsActive, qt.IsTrue)

	c.Assert(r.SetActive(ctx, created.ID, false), qt.IsNil)
	fetched, err := r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.IsActive, qt.IsFalse)

	c.Assert(r.SetActive(ctx, created.ID, true), qt.IsNil)
	fetched, err = r.Get(ctx, created.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.IsActive, qt.IsTrue)
}
