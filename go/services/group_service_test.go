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
	// Wire the user registry into the membership registry too — the
	// memory backend's ListByGroupWithUsers does its own user lookup
	// (the SQL JOIN equivalent), and without this the joined User
	// field comes back nil from the memory backend.
	memberships.SetUserRegistry(users)
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

func TestGroupService_RemoveMember_LastOwnerProtection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// The sole owner cannot be removed — would leave the group without
	// anyone able to delete it. ErrorIs guards against the check being
	// skipped in favor of a generic failure (NotFound etc.).
	err = svc.RemoveMember(ctx, group.ID, "user-1")
	c.Assert(err, qt.ErrorIs, services.ErrLastOwner)
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsTrue)

	// Non-owner members can still be removed even when only one owner
	// exists — the owner count stays at 1, invariant preserved.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	err = svc.RemoveMember(ctx, group.ID, "user-2")
	c.Assert(err, qt.IsNil)

	// Once a second owner exists the original owner can be removed.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-3", models.GroupRoleOwner)
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

	// Add a second owner so user-1 can be demoted without tripping
	// the ≥1-owner invariant.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleOwner)
	c.Assert(err, qt.IsNil)

	// Demote user-1 (there's still user-2 as owner).
	membership, err := svc.UpdateMemberRole(ctx, group.ID, "user-1", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	c.Assert(membership.Role, qt.Equals, models.GroupRoleAdmin)

	// Demote a user-2 owner all the way down to viewer fails because
	// it would leave zero owners.
	_, err = svc.UpdateMemberRole(ctx, group.ID, "user-2", models.GroupRoleViewer)
	c.Assert(err, qt.ErrorIs, services.ErrLastOwner)
}

func TestGroupService_UpdateMemberRole_LastOwnerProtection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Cannot demote the last owner.
	_, err = svc.UpdateMemberRole(ctx, group.ID, "user-1", models.GroupRoleAdmin)
	c.Assert(err, qt.ErrorIs, services.ErrLastOwner)
}

func TestGroupService_LeaveGroup(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Add a second owner so the first can leave without violating the
	// ≥1-owner invariant.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleOwner)
	c.Assert(err, qt.IsNil)

	err = svc.LeaveGroup(ctx, group.ID, "user-1")
	c.Assert(err, qt.IsNil)

	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsFalse)
}

func TestGroupService_LeaveGroup_LastOwnerProtection(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// The sole owner cannot leave.
	err = svc.LeaveGroup(ctx, group.ID, "user-1")
	c.Assert(err, qt.ErrorIs, services.ErrLastOwner)
	c.Assert(svc.IsGroupMember(ctx, group.ID, "user-1"), qt.IsTrue)

	// A non-owner can still leave.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	err = svc.LeaveGroup(ctx, group.ID, "user-2")
	c.Assert(err, qt.IsNil)

	// Promoting a second owner unblocks the original owner's leave.
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-3", models.GroupRoleOwner)
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

	// Add a co-owner to g1 so the original owner can leave without
	// tripping the ≥1 owner invariant.
	_, err = svc.AddMember(ctx, "tenant-1", g1.ID, "co-owner", models.GroupRoleOwner)
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
	// Add a co-owner so the leaving owner doesn't trip the ≥1 owner guard.
	_, err = svc.AddMember(ctx, "tenant-1", g1.ID, "co-owner", models.GroupRoleOwner)
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

// --- #1388 MembershipCap ----------------------------------------------------

func TestGroupService_MembershipCap_CreateGroup(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	// Fill the cap with three groups for the same user.
	for i := range services.MaxGroupMembershipsPerUser() {
		_, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "G", "", "")
		c.Assert(err, qt.IsNil, qt.Commentf("group %d should fit under the cap", i+1))
	}

	// The next CreateGroup must be rejected with the typed sentinel —
	// surface code (and the FE) match on it to render the right copy.
	_, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Overflow", "", "")
	c.Assert(err, qt.ErrorIs, services.ErrTooManyGroupMemberships)

	// A different user is unaffected — the cap is per-user, not
	// per-tenant. This guards against accidentally globbing the
	// membership count across users in a future refactor.
	_, err = svc.CreateGroup(ctx, "tenant-1", "user-2", "Other", "", "")
	c.Assert(err, qt.IsNil)
}

func TestGroupService_MembershipCap_AddMember(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	// user-1 owns three groups (== cap).
	groups := make([]string, services.MaxGroupMembershipsPerUser())
	for i := range groups {
		g, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "G", "", "")
		c.Assert(err, qt.IsNil)
		groups[i] = g.ID
	}

	// user-2 can be added to two of them (== 2 memberships).
	_, err := svc.AddMember(ctx, "tenant-1", groups[0], "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	_, err = svc.AddMember(ctx, "tenant-1", groups[1], "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)

	// A third add for user-2 is allowed (3 == cap, the equality boundary).
	_, err = svc.AddMember(ctx, "tenant-1", groups[2], "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)

	// Now the cap is reached — creating a fourth group for user-2 fails.
	_, err = svc.CreateGroup(ctx, "tenant-1", "user-2", "Fourth", "", "")
	c.Assert(err, qt.ErrorIs, services.ErrTooManyGroupMemberships)
}

// --- #1533 additions ------------------------------------------------------

func TestGroupService_GetMembershipRole(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	// Group creator is owner post-#1533.
	role, err := svc.GetMembershipRole(ctx, group.ID, "user-1")
	c.Assert(err, qt.IsNil)
	c.Assert(role, qt.Equals, models.GroupRoleOwner)

	// Non-member surfaces ErrNotGroupMember (rather than a generic 500).
	_, err = svc.GetMembershipRole(ctx, group.ID, "stranger")
	c.Assert(err, qt.ErrorIs, services.ErrNotGroupMember)
}

func TestGroupService_HasRoleAtLeast(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "viewer-2", models.GroupRoleViewer)
	c.Assert(err, qt.IsNil)
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-3", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "admin-4", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)

	// Owner satisfies every threshold.
	ok, role, err := svc.HasRoleAtLeast(ctx, group.ID, "user-1", models.GroupRoleOwner)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
	c.Assert(role, qt.Equals, models.GroupRoleOwner)

	// Admin satisfies admin but NOT owner.
	ok, _, err = svc.HasRoleAtLeast(ctx, group.ID, "admin-4", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
	ok, _, err = svc.HasRoleAtLeast(ctx, group.ID, "admin-4", models.GroupRoleOwner)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsFalse)

	// User satisfies user / viewer but NOT admin.
	ok, _, err = svc.HasRoleAtLeast(ctx, group.ID, "user-3", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
	ok, _, err = svc.HasRoleAtLeast(ctx, group.ID, "user-3", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsFalse)

	// Viewer satisfies viewer but NOT user.
	ok, _, err = svc.HasRoleAtLeast(ctx, group.ID, "viewer-2", models.GroupRoleViewer)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsTrue)
	ok, _, err = svc.HasRoleAtLeast(ctx, group.ID, "viewer-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsFalse)

	// Non-member: (false, "", nil) — the middleware maps this case to
	// 403 (caller is authenticated but not a member of this group),
	// not 500. Registry / infra errors take a separate path.
	ok, _, err = svc.HasRoleAtLeast(ctx, group.ID, "stranger", models.GroupRoleViewer)
	c.Assert(err, qt.IsNil)
	c.Assert(ok, qt.IsFalse)
}

func TestGroupService_IsGroupOwner(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "admin-2", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)

	c.Assert(svc.IsGroupOwner(ctx, group.ID, "user-1"), qt.IsTrue)
	c.Assert(svc.IsGroupOwner(ctx, group.ID, "admin-2"), qt.IsFalse)
	c.Assert(svc.IsGroupOwner(ctx, group.ID, "stranger"), qt.IsFalse)
}

func TestGroupService_CreateInviteWithEmail(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	email := "invitee@example.com"
	invite, err := svc.CreateInviteWithEmail(
		ctx, "tenant-1", group.ID, "user-1",
		models.GroupRoleAdmin, &email, 0,
	)
	c.Assert(err, qt.IsNil)
	c.Assert(invite.Token, qt.Not(qt.Equals), "")
	c.Assert(invite.Role, qt.Equals, models.GroupRoleAdmin)
	c.Assert(invite.InviteeEmail, qt.IsNotNil)
	c.Assert(*invite.InviteeEmail, qt.Equals, email)

	// Empty role defaults to user.
	defInvite, err := svc.CreateInviteWithEmail(
		ctx, "tenant-1", group.ID, "user-1", "", nil, 0,
	)
	c.Assert(err, qt.IsNil)
	c.Assert(defInvite.Role, qt.Equals, models.GroupRoleUser)
	c.Assert(defInvite.InviteeEmail, qt.IsNil)
}

func TestGroupService_AcceptInvite_UsesInviteRole(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	email := "invitee@example.com"
	invite, err := svc.CreateInviteWithEmail(
		ctx, "tenant-1", group.ID, "user-1",
		models.GroupRoleViewer, &email, 0,
	)
	c.Assert(err, qt.IsNil)

	mem, err := svc.AcceptInvite(ctx, invite.Token, "user-2", "tenant-1")
	c.Assert(err, qt.IsNil)
	c.Assert(mem.Role, qt.Equals, models.GroupRoleViewer)
}

func TestGroupService_ResendInvite(t *testing.T) {
	c := qt.New(t)
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)

	email := "invitee@example.com"
	invite, err := svc.CreateInviteWithEmail(
		ctx, "tenant-1", group.ID, "user-1",
		models.GroupRoleUser, &email, 1*time.Hour,
	)
	c.Assert(err, qt.IsNil)
	oldToken := invite.Token
	oldExpiry := invite.ExpiresAt

	// Resend mints a fresh token and bumps expiry forward.
	resent, err := svc.ResendInvite(ctx, group.ID, invite.ID, 24*time.Hour)
	c.Assert(err, qt.IsNil)
	c.Assert(resent.Token, qt.Not(qt.Equals), oldToken)
	c.Assert(resent.ExpiresAt.After(oldExpiry), qt.IsTrue)

	// Legacy token-only invite has no email → resend is rejected.
	tokenOnly, err := svc.CreateInvite(ctx, "tenant-1", group.ID, "user-1", 0)
	c.Assert(err, qt.IsNil)
	_, err = svc.ResendInvite(ctx, group.ID, tokenOnly.ID, 0)
	c.Assert(err, qt.ErrorIs, services.ErrInviteNotByEmail)

	// Wrong-group ownership is rejected.
	otherGroup, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Other", "", "")
	c.Assert(err, qt.IsNil)
	_, err = svc.ResendInvite(ctx, otherGroup.ID, invite.ID, 0)
	c.Assert(err, qt.ErrorIs, services.ErrInviteNotInGroup)
}

func TestGroupService_ListMembersWithUsers_NoUserRegistry(t *testing.T) {
	c := qt.New(t)
	// The bare service (no UserRegistry wired) still returns memberships;
	// the joined User field is nil — callers render fallbacks.
	svc := newTestGroupService()
	ctx := context.Background()

	group, err := svc.CreateGroup(ctx, "tenant-1", "user-1", "Group", "", "")
	c.Assert(err, qt.IsNil)
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleUser)
	c.Assert(err, qt.IsNil)

	rows, err := svc.ListMembersWithUsers(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 2)
	for _, row := range rows {
		c.Assert(row.Membership, qt.IsNotNil)
		// User is nil because the bare service didn't wire UserRegistry.
		c.Assert(row.User, qt.IsNil)
	}
}

func TestGroupService_ListMembersWithUsers_JoinedFields(t *testing.T) {
	c := qt.New(t)
	svc, users, _ := newTestGroupServiceWithUsers()
	ctx := context.Background()

	owner := seedUser(c, users, "tenant-1", "owner@example.com")
	other := seedUser(c, users, "tenant-1", "other@example.com")

	group, err := svc.CreateGroup(ctx, "tenant-1", owner.ID, "Group", "", "")
	c.Assert(err, qt.IsNil)
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, other.ID, models.GroupRoleUser)
	c.Assert(err, qt.IsNil)

	rows, err := svc.ListMembersWithUsers(ctx, group.ID)
	c.Assert(err, qt.IsNil)
	c.Assert(rows, qt.HasLen, 2)

	seenEmails := map[string]bool{}
	for _, row := range rows {
		c.Assert(row.User, qt.IsNotNil)
		seenEmails[row.User.Email] = true
	}
	c.Assert(seenEmails["owner@example.com"], qt.IsTrue)
	c.Assert(seenEmails["other@example.com"], qt.IsTrue)
}
