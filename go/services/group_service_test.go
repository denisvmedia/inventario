package services_test

import (
	"context"
	"testing"
	"time"

	qt "github.com/frankban/quicktest"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry/memory"
	"github.com/denisvmedia/inventario/services"
)

func newTestGroupService() *services.GroupService {
	return services.NewGroupService(
		memory.NewLocationGroupRegistry(),
		memory.NewGroupMembershipRegistry(),
		memory.NewGroupInviteRegistry(),
	)
}

// newTestGroupServiceWithUsers wires the service with a UserRegistry so the
// #1592 EnsureDefaultGroup path is exercised. Returns the service plus the
// registries so tests can assert on user state without going through the
// public API.
func newTestGroupServiceWithUsers() (*services.GroupService, *memory.UserRegistry, *memory.GroupMembershipRegistry) {
	users := memory.NewUserRegistry()
	memberships := memory.NewGroupMembershipRegistry()
	svc := services.NewGroupService(
		memory.NewLocationGroupRegistry(),
		memberships,
		memory.NewGroupInviteRegistry(),
	)
	svc.SetUserRegistry(users)
	return svc, users, memberships
}

// seedUser creates a user in the in-memory registry and returns the saved row.
func seedUser(c *qt.C, users *memory.UserRegistry, tenantID, email string) *models.User {
	created, err := users.Create(context.Background(), models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{TenantID: tenantID},
		Email:               email,
		Name:                email,
		IsActive:            true,
	})
	c.Assert(err, qt.IsNil)
	c.Assert(created, qt.IsNotNil)
	return created
}

func TestGroupService_CreateGroup(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "My Group", "📦", "")
	c.Assert(err, qt.IsNil)
	c.Assert(group, qt.IsNotNil)
	c.Assert(group.Name, qt.Equals, "My Group")
	c.Assert(group.Icon, qt.Equals, "📦")
	c.Assert(group.Status, qt.Equals, models.LocationGroupStatusActive)
	c.Assert(len(group.Slug) >= 22, qt.IsTrue, qt.Commentf("slug should be >= 22 chars, got %d", len(group.Slug)))

	// Creator should be an admin member
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsTrue)
	c.Assert(svc.IsGroupAdmin(ctx, group.ID, "user-1"), qt.IsTrue)
}

func TestGroupService_ListUserGroups(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	// Create two groups for the same user
	g1, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group 1", "", "")
	c.Assert(err, qt.IsNil)
	g2, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group 2", "", "")
	c.Assert(err, qt.IsNil)

	groups, err := svc.ListUserGroups(ctx, "tenant-1", "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(groups, qt.HasLen, 2)

	ids := []string{groups[0].ID, groups[1].ID}
	c.Assert(ids, qt.Contains, g1.ID)
	c.Assert(ids, qt.Contains, g2.ID)
}

func TestGroupService_UpdateGroup(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Old Name", "🏠", "")
	c.Assert(err, qt.IsNil)

	updated, err := svc.UpdateGroup(ctx, group.ID, "New Name", "🏢")
	c.Assert(err, qt.IsNil)
	c.Assert(updated.Name, qt.Equals, "New Name")
	c.Assert(updated.Icon, qt.Equals, "🏢")
}

func TestGroupService_InitiateGroupDeletion(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "To Delete", "", "")
	c.Assert(err, qt.IsNil)

	// Wrong confirmation word
	err = svc.InitiateGroupDeletion(ctx, group.ID, "wrong", "To Delete")
	c.Assert(err, qt.IsNotNil)

	// Correct confirmation word
	err = svc.InitiateGroupDeletion(ctx, group.ID, "To Delete", "To Delete")
	c.Assert(err, qt.IsNil)

	// Group should now be pending deletion
	deleted, err := svc.GetGroup(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(deleted.Status, qt.Equals, models.LocationGroupStatusPendingDeletion)

	// Cannot update a pending_deletion group
	_, err = svc.UpdateGroup(ctx, group.ID, "New Name", "")
	c.Assert(err, qt.IsNotNil)
}

func TestGroupService_AddMember(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Add a new member
	membership, err := svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	c.Assert(membership.MemberUserID, qt.Equals, "user-2")
	c.Assert(membership.Role, qt.Equals, models.GroupRoleUser)

	// Cannot add the same user again
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNotNil)
}

func TestGroupService_RemoveMember(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Add a second member
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)

	// Remove the second member
	err = svc.RemoveMember(ctx, group.ID, "user-2")
	c.Assert(err, qt.IsNil)

	// Verify they're gone
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-2"), qt.IsFalse)
}

func TestGroupService_RemoveMember_LastAdminProtection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// The sole admin cannot be removed — would leave the group without an
	// admin. ErrorIs guards against the check being skipped in favor of a
	// generic failure (e.g. NotFound), which would still satisfy IsNotNil but
	// not the ≥1-admin invariant the endpoint is supposed to defend.
	err = svc.RemoveMember(ctx, group.ID, "user-1")
	c.Assert(err, qt.ErrorIs, services.ErrLastAdmin)
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsTrue)

	// A non-admin member can still be removed even when there's only one
	// admin: the admin count stays at 1, so the invariant is preserved.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	err = svc.RemoveMember(ctx, group.ID, "user-2")
	c.Assert(err, qt.IsNil)

	// Once a second admin exists the original admin can be removed.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-3", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	err = svc.RemoveMember(ctx, group.ID, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsFalse)
}

func TestGroupService_UpdateMemberRole(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)

	// Promote user-2 to admin
	membership, err := svc.UpdateMemberRole(ctx, group.ID, "user-2", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	c.Assert(membership.Role, qt.Equals, models.GroupRoleAdmin)

	// Demote user-1 (there's still user-2 as admin)
	membership, err = svc.UpdateMemberRole(ctx, group.ID, "user-1", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	c.Assert(membership.Role, qt.Equals, models.GroupRoleUser)
}

func TestGroupService_UpdateMemberRole_LastAdminProtection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Cannot demote the last admin
	_, err = svc.UpdateMemberRole(ctx, group.ID, "user-1", models.GroupRoleUser)
	c.Assert(err, qt.IsNotNil)
}

func TestGroupService_LeaveGroup(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Add a second admin so the first can leave
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)

	err = svc.LeaveGroup(ctx, group.ID, "user-1")
	c.Assert(err, qt.IsNil)

	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsFalse)
}

func TestGroupService_LeaveGroup_LastAdminProtection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// The sole admin cannot leave — would leave the group without an admin.
	// ErrorIs guards against the check being skipped in favor of a generic
	// failure (e.g. NotFound), which would still satisfy IsNotNil but not
	// the ≥1-admin invariant the endpoint is supposed to defend.
	err = svc.LeaveGroup(ctx, group.ID, "user-1")
	c.Assert(err, qt.ErrorIs, services.ErrLastAdmin)
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsTrue)

	// A non-admin member can still leave even when there's only one admin:
	// the admin count stays at 1, so the invariant is preserved.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	err = svc.LeaveGroup(ctx, group.ID, "user-2")
	c.Assert(err, qt.IsNil)

	// Promoting a second admin unblocks the original admin's leave.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-3", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	err = svc.LeaveGroup(ctx, group.ID, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsFalse)
}

func TestGroupService_InviteFlow(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Create invite
	invite, err := svc.CreateInvite(ctx, "tenant-1", group.ID, "user-1", 24*time.Hour)
	c.Assert(err, qt.IsNil)
	c.Assert(invite.Token, qt.Not(qt.Equals), "")
	c.Assert(invite.IsValid(), qt.IsTrue)

	// Get invite info
	fetchedInvite, fetchedGroup, err := svc.GetInviteInfo(ctx, invite.Token)
	c.Assert(err, qt.IsNil)
	c.Assert(fetchedInvite.ID, qt.Equals, invite.ID)
	c.Assert(fetchedGroup.ID, qt.Equals, group.ID)

	// Accept invite
	membership, err := svc.AcceptInvite(ctx, invite.Token, "user-2", "tenant-1")
	c.Assert(err, qt.IsNil)
	c.Assert(membership.MemberUserID, qt.Equals, "user-2")
	c.Assert(membership.Role, qt.Equals, models.GroupRoleUser)

	// Cannot accept the same invite again
	_, err = svc.AcceptInvite(ctx, invite.Token, "user-3", "tenant-1")
	c.Assert(err, qt.IsNotNil)

	// Verify user-2 is now a member
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-2"), qt.IsTrue)
}

func TestGroupService_RevokeInviteForGroup(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	invite, err := svc.CreateInvite(ctx, "tenant-1", group.ID, "user-1", 24*time.Hour)
	c.Assert(err, qt.IsNil)

	// Revoke the invite
	err = svc.RevokeInviteForGroup(ctx, group.ID, invite.ID)
	c.Assert(err, qt.IsNil)

	// Invite is gone
	_, _, err = svc.GetInviteInfo(ctx, invite.Token)
	c.Assert(err, qt.IsNotNil)
}

func TestGroupService_RevokeInviteForGroup_WrongGroup(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group A", "", "")
	c.Assert(err, qt.IsNil)

	otherGroup, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group B", "", "")
	c.Assert(err, qt.IsNil)

	invite, err := svc.CreateInvite(ctx, "tenant-1", group.ID, "user-1", 24*time.Hour)
	c.Assert(err, qt.IsNil)

	// Cannot revoke invite from a different group
	err = svc.RevokeInviteForGroup(ctx, otherGroup.ID, invite.ID)
	c.Assert(err, qt.IsNotNil)
	c.Assert(err, qt.ErrorIs, services.ErrInviteNotInGroup)
}

func TestGroupService_RevokeInviteForGroup_CannotRevokeUsed(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	invite, err := svc.CreateInvite(ctx, "tenant-1", group.ID, "user-1", 24*time.Hour)
	c.Assert(err, qt.IsNil)

	// Accept the invite
	_, err = svc.AcceptInvite(ctx, invite.Token, "user-2", "tenant-1")
	c.Assert(err, qt.IsNil)

	// Cannot revoke a used invite
	err = svc.RevokeInviteForGroup(ctx, group.ID, invite.ID)
	c.Assert(err, qt.IsNotNil)
}

func TestGroupService_ListActiveInvites(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Create two invites
	_, err = svc.CreateInvite(ctx, "tenant-1", group.ID, "user-1", 24*time.Hour)
	c.Assert(err, qt.IsNil)
	_, err = svc.CreateInvite(ctx, "tenant-1", group.ID, "user-1", 24*time.Hour)
	c.Assert(err, qt.IsNil)

	invites, err := svc.ListActiveInvites(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(invites, qt.HasLen, 2)
}

func TestGroupService_GetGroupBySlug(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	found, err := svc.GetGroupBySlug(ctx, "tenant-1", group.Slug)
	c.Assert(err, qt.IsNil)
	c.Assert(found.ID, qt.Equals, group.ID)

	// Not found in different tenant
	_, err = svc.GetGroupBySlug(ctx, "tenant-2", group.Slug)
	c.Assert(err, qt.IsNotNil)
}

// --- #1592 EnsureDefaultGroup -----------------------------------------------

func TestGroupService_EnsureDefaultGroup_PromotesFirstMembership(t *testing.T) {
	c := qt.New(t)
	svc, users, _ := newTestGroupServiceWithUsers()
	ctx := context.Background()
	user := seedUser(c, users, "tenant-1", "alice@example.com")

	// Brand-new user creates their first group → that group becomes default.
	group, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "First", "", "")
	c.Assert(err, qt.IsNil)

	stored, err := users.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DefaultGroupID, qt.IsNotNil)
	c.Assert(*stored.DefaultGroupID, qt.Equals, group.ID)
}

func TestGroupService_EnsureDefaultGroup_KeepsExistingPreference(t *testing.T) {
	c := qt.New(t)
	svc, users, _ := newTestGroupServiceWithUsers()
	ctx := context.Background()
	user := seedUser(c, users, "tenant-1", "alice@example.com")

	g1, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "First", "", "")
	c.Assert(err, qt.IsNil)
	// A second group must NOT clobber the user's existing default — the
	// invariant only requires that *some* membership is the default, not
	// that the latest one wins.
	g2, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "Second", "", "")
	c.Assert(err, qt.IsNil)

	stored, err := users.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DefaultGroupID, qt.IsNotNil)
	c.Assert(*stored.DefaultGroupID, qt.Equals, g1.ID)
	c.Assert(*stored.DefaultGroupID, qt.Not(qt.Equals), g2.ID)
}

func TestGroupService_EnsureDefaultGroup_AcceptInviteAsBrandNewUser(t *testing.T) {
	c := qt.New(t)
	svc, users, _ := newTestGroupServiceWithUsers()
	ctx := context.Background()
	owner := seedUser(c, users, "tenant-1", "owner@example.com")
	invitee := seedUser(c, users, "tenant-1", "invitee@example.com")

	group, err := svc.CreateGroup(ctx, "tenant-1", owner.ID, "Shared", "", "")
	c.Assert(err, qt.IsNil)

	invite, err := svc.CreateInvite(ctx, "tenant-1", group.ID, owner.ID, 24*time.Hour)
	c.Assert(err, qt.IsNil)

	_, err = svc.AcceptInvite(ctx, invite.Token, invitee.ID, "tenant-1")
	c.Assert(err, qt.IsNil)

	stored, err := users.Get(ctx, invitee.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DefaultGroupID, qt.IsNotNil)
	c.Assert(*stored.DefaultGroupID, qt.Equals, group.ID)
}

func TestGroupService_EnsureDefaultGroup_RepromotesOnLeave(t *testing.T) {
	c := qt.New(t)
	svc, users, _ := newTestGroupServiceWithUsers()
	ctx := context.Background()
	user := seedUser(c, users, "tenant-1", "alice@example.com")

	// User belongs to two groups they created; default points at g1.
	g1, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "First", "", "")
	c.Assert(err, qt.IsNil)
	g2, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "Second", "", "")
	c.Assert(err, qt.IsNil)
	stored, err := users.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(*stored.DefaultGroupID, qt.Equals, g1.ID)

	// Add a co-admin to g1 so the original admin can leave without tripping
	// the ≥1 admin invariant.
	_, err = svc.AddMember(ctx, "tenant-1", g1.ID, "co-admin", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	c.Assert(svc.LeaveGroup(ctx, g1.ID, user.ID), qt.IsNil)

	// Default must have flipped to g2 — the only remaining membership.
	stored, err = users.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DefaultGroupID, qt.IsNotNil)
	c.Assert(*stored.DefaultGroupID, qt.Equals, g2.ID)
}

func TestGroupService_EnsureDefaultGroup_LastMembershipLeavesNullDefault(t *testing.T) {
	c := qt.New(t)
	svc, users, _ := newTestGroupServiceWithUsers()
	ctx := context.Background()
	user := seedUser(c, users, "tenant-1", "alice@example.com")

	g1, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "Only", "", "")
	c.Assert(err, qt.IsNil)
	// Add a co-admin so the leaving admin doesn't trip the ≥1 admin guard.
	_, err = svc.AddMember(ctx, "tenant-1", g1.ID, "co-admin", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)

	c.Assert(svc.LeaveGroup(ctx, g1.ID, user.ID), qt.IsNil)

	// User now has zero memberships → the invariant permits a NULL default.
	stored, err := users.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DefaultGroupID, qt.IsNil)
}

func TestGroupService_EnsureDefaultGroup_DeterministicByJoinedAt(t *testing.T) {
	c := qt.New(t)
	svc, users, memberships := newTestGroupServiceWithUsers()
	ctx := context.Background()
	user := seedUser(c, users, "tenant-1", "alice@example.com")

	// Two memberships with explicit joined_at so the deterministic earliest-
	// joined-at tiebreak is unambiguous regardless of map iteration order.
	earlyGroup, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "Early", "", "")
	c.Assert(err, qt.IsNil)
	lateGroup, err := svc.CreateGroup(ctx, "tenant-1", user.ID, "Late", "", "")
	c.Assert(err, qt.IsNil)

	rows, err := memberships.ListByUser(ctx, "tenant-1", user.ID)
	c.Assert(err, qt.IsNil)
	for _, m := range rows {
		switch m.GroupID {
		case earlyGroup.ID:
			m.JoinedAt = time.Unix(1_000, 0).UTC()
		case lateGroup.ID:
			m.JoinedAt = time.Unix(2_000, 0).UTC()
		}
		_, err := memberships.Update(ctx, *m)
		c.Assert(err, qt.IsNil)
	}

	// Force a recompute by clearing the saved default; EnsureDefaultGroup
	// must promote the early-joined group regardless of insertion order.
	stored, err := users.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	stored.DefaultGroupID = nil
	_, err = users.Update(ctx, *stored)
	c.Assert(err, qt.IsNil)

	c.Assert(svc.EnsureDefaultGroup(ctx, user.ID), qt.IsNil)

	stored, err = users.Get(ctx, user.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(stored.DefaultGroupID, qt.IsNotNil)
	c.Assert(*stored.DefaultGroupID, qt.Equals, earlyGroup.ID)
}
