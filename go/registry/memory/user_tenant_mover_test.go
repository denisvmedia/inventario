package memory_test

import (
	"context"
	"errors"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
	"go.5x5.cz/inventario/registry/memory"
)

// newMoverFixture returns a memory factory set with two tenants and a user in
// the first one who has a session and a second factor.
func newMoverFixture(c *qt.C) (fs *registry.FactorySet, user *models.User, to string) {
	c.Helper()
	ctx := context.Background()
	fs = memory.NewFactorySet()
	from, err := fs.TenantRegistry.Create(ctx, models.Tenant{Name: "From", Slug: "from", Status: models.TenantStatusActive})
	c.Assert(err, qt.IsNil)
	target, err := fs.TenantRegistry.Create(ctx, models.Tenant{Name: "To", Slug: "to", Status: models.TenantStatusActive})
	c.Assert(err, qt.IsNil)

	user, err = fs.UserRegistry.Create(ctx, models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: from.ID},
		Email:               "mover@example.com",
		Name:                "Mover",
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.RefreshTokenRegistry.Create(ctx, models.RefreshToken{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: from.ID, UserID: user.ID},
		TokenHash:               "session",
		ExpiresAt:               time.Now().Add(time.Hour),
	})
	c.Assert(err, qt.IsNil)
	_, err = fs.UserMFASecretRegistry.Create(ctx, models.UserMFASecret{
		TenantUserAwareEntityID: models.TenantUserAwareEntityID{TenantID: from.ID, UserID: user.ID},
		SecretEncrypted:         "encrypted-secret",
	})
	c.Assert(err, qt.IsNil)
	return fs, user, target.ID
}

func TestUserTenantMover_MovesAnEmptyUser(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs, user, to := newMoverFixture(c)

	moved, err := fs.UserTenantMover.MoveUserToTenant(ctx, registry.UserTenantMove{UserID: user.ID, FromTenantID: user.TenantID, ToTenantID: to})
	c.Assert(err, qt.IsNil)
	c.Assert(moved.TenantID, qt.Equals, to)

	sessions, err := fs.RefreshTokenRegistry.GetByUserID(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(sessions, qt.HasLen, 0)
	_, err = fs.UserMFASecretRegistry.GetByUser(ctx, to, user.ID)
	c.Assert(err, qt.IsNil)
}

func TestUserTenantMover_RefusesAMember(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs, user, to := newMoverFixture(c)
	_, err := fs.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: user.TenantID},
		GroupID:             "some-group",
		MemberUserID:        user.ID,
		Role:                models.GroupRoleUser,
	})
	c.Assert(err, qt.IsNil)

	_, err = fs.UserTenantMover.MoveUserToTenant(ctx, registry.UserTenantMove{UserID: user.ID, FromTenantID: user.TenantID, ToTenantID: to})
	blocked, ok := errors.AsType[*registry.UserTenantMoveBlockedError](err)
	c.Assert(ok, qt.IsTrue, qt.Commentf("err: %v", err))
	c.Assert(blocked.Owned.String(), qt.Equals, "member of 1 group")

	stored, err := fs.UserRegistry.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.TenantID, qt.Equals, user.TenantID)
	sessions, err := fs.RefreshTokenRegistry.GetByUserID(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(sessions, qt.HasLen, 1)
}

func TestUserTenantMover_RefusesAStaleSourceTenant(t *testing.T) {
	c := qt.New(t)
	fs, user, to := newMoverFixture(c)

	_, err := fs.UserTenantMover.MoveUserToTenant(context.Background(), registry.UserTenantMove{UserID: user.ID, FromTenantID: "another-tenant", ToTenantID: to})
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantChanged)
}

// UserRegistry.Update never changes a user's tenant, so a write built from a
// copy read before a move cannot put the old tenant back.
func TestUserRegistry_Update_KeepsTheTenant(t *testing.T) {
	c := qt.New(t)
	ctx := context.Background()
	fs, user, to := newMoverFixture(c)

	changed := *user
	changed.TenantID = to
	_, err := fs.UserRegistry.Update(ctx, changed)
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantChanged)

	_, err = fs.UserTenantMover.MoveUserToTenant(ctx, registry.UserTenantMove{UserID: user.ID, FromTenantID: user.TenantID, ToTenantID: to})
	c.Assert(err, qt.IsNil)
	stale := *user
	stale.Name = "Renamed From A Stale Copy"
	_, err = fs.UserRegistry.Update(ctx, stale)
	c.Assert(err, qt.ErrorIs, registry.ErrUserTenantChanged)
}
