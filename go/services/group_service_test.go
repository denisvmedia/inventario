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

	// Cannot remove the last admin
	err = svc.RemoveMember(ctx, group.ID, "user-1")
	c.Assert(err, qt.IsNotNil)

	// Add a second admin
	_, err = svc.AddMember(ctx, "tenant-1", group.ID, "user-2", models.GroupRoleAdmin)
	c.Assert(err, qt.IsNil)

	// Now we can remove the first admin
	err = svc.RemoveMember(ctx, group.ID, "user-1")
	c.Assert(err, qt.IsNil)
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
