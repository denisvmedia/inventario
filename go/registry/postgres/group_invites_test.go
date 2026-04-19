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

func inviteFor(tenantID, groupID, createdByUserID string, expires time.Time) models.GroupInvite {
	token, _ := models.GenerateInviteToken()
	return models.GroupInvite{
		TenantOnlyEntityID: models.TenantOnlyEntityID{TenantID: tenantID},
		GroupID:            groupID,
		Token:              token,
		CreatedBy:          createdByUserID,
		ExpiresAt:          expires,
	}
}

func TestGroupInviteRegistry_Create_HappyPath(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Invites"))
	c.Assert(err, qt.IsNil)

	inv := inviteFor(user.TenantID, group.ID, user.ID, time.Now().Add(24*time.Hour))
	created, err := registrySet.GroupInviteRegistry.Create(ctx, inv)
	c.Assert(err, qt.IsNil)
	c.Assert(created.ID, qt.Not(qt.Equals), "")
	c.Assert(created.Token, qt.Equals, inv.Token)
	c.Assert(created.UsedBy, qt.IsNil)
	c.Assert(created.UsedAt, qt.IsNil)
}

func TestGroupInviteRegistry_Create_MissingFields(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "MF"))
	c.Assert(err, qt.IsNil)
	valid := inviteFor(user.TenantID, group.ID, user.ID, time.Now().Add(24*time.Hour))

	cases := []struct {
		name string
		mut  func(*models.GroupInvite)
	}{
		{"group_id empty", func(i *models.GroupInvite) { i.GroupID = "" }},
		{"token empty", func(i *models.GroupInvite) { i.Token = "" }},
		{"tenant empty", func(i *models.GroupInvite) { i.TenantID = "" }},
		{"created_by empty", func(i *models.GroupInvite) { i.CreatedBy = "" }},
		{"expires_at zero", func(i *models.GroupInvite) { i.ExpiresAt = time.Time{} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := qt.New(t)
			inv := valid
			inv.Token, _ = models.GenerateInviteToken()
			tc.mut(&inv)
			_, err := registrySet.GroupInviteRegistry.Create(ctx, inv)
			c.Assert(err, qt.IsNotNil)
			c.Assert(errors.Is(err, registry.ErrFieldRequired), qt.IsTrue)
		})
	}
}

func TestGroupInviteRegistry_GetByToken_And_ListActive(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "Active"))
	c.Assert(err, qt.IsNil)

	active, err := registrySet.GroupInviteRegistry.Create(ctx, inviteFor(user.TenantID, group.ID, user.ID, time.Now().Add(24*time.Hour)))
	c.Assert(err, qt.IsNil)

	// An expired invite should not appear in ListActiveByGroup.
	expired, err := registrySet.GroupInviteRegistry.Create(ctx, inviteFor(user.TenantID, group.ID, user.ID, time.Now().Add(-1*time.Hour)))
	c.Assert(err, qt.IsNil)

	fetched, err := registrySet.GroupInviteRegistry.GetByToken(ctx, active.Token)
	c.Assert(err, qt.IsNil)
	c.Assert(fetched.ID, qt.Equals, active.ID)

	list, err := registrySet.GroupInviteRegistry.ListActiveByGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(list, qt.HasLen, 1)
	c.Assert(list[0].ID, qt.Equals, active.ID)

	_, err = registrySet.GroupInviteRegistry.GetByToken(ctx, "no-such-token")
	c.Assert(errors.Is(err, registry.ErrNotFound), qt.IsTrue)

	// And the expired one still exists via Get by ID — ListActiveByGroup
	// merely filters it out.
	gotExpired, err := registrySet.GroupInviteRegistry.Get(ctx, expired.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(gotExpired.IsExpired(), qt.IsTrue)
}

// TestGroupInviteRegistry_MarkUsed_CAS verifies compare-and-swap semantics:
// the first MarkUsed call mutates the row and returns (true, nil); a second
// call for the same invite returns (false, nil) without touching used_by.
func TestGroupInviteRegistry_MarkUsed_CAS(t *testing.T) {
	registrySet, cleanup := setupTestRegistrySet(t)
	defer cleanup()

	c := qt.New(t)
	ctx := context.Background()
	user := getTestUser(c, registrySet)

	group, err := registrySet.LocationGroupRegistry.Create(ctx, newTestGroup(c, user.TenantID, user.ID, "CAS"))
	c.Assert(err, qt.IsNil)

	inv, err := registrySet.GroupInviteRegistry.Create(ctx, inviteFor(user.TenantID, group.ID, user.ID, time.Now().Add(24*time.Hour)))
	c.Assert(err, qt.IsNil)

	now := time.Now()
	won, err := registrySet.GroupInviteRegistry.MarkUsed(ctx, inv.ID, user.ID, now)
	c.Assert(err, qt.IsNil)
	c.Assert(won, qt.IsTrue)

	// A second caller attempting to mark the same invite loses the CAS.
	won2, err := registrySet.GroupInviteRegistry.MarkUsed(ctx, inv.ID, "other-user", now.Add(time.Second))
	c.Assert(err, qt.IsNil)
	c.Assert(won2, qt.IsFalse)

	// used_by must reflect the first (winning) caller.
	reloaded, err := registrySet.GroupInviteRegistry.Get(ctx, inv.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(reloaded.UsedBy, qt.IsNotNil)
	c.Assert(*reloaded.UsedBy, qt.Equals, user.ID)
}
