package services

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var (
	ErrGroupNotActive      = errx.NewSentinel("group is not active")
	ErrLastAdmin           = errx.NewSentinel("cannot remove the last admin from a group")
	ErrInviteExpired       = errx.NewSentinel("invite has expired")
	ErrInviteAlreadyUsed   = errx.NewSentinel("invite has already been used")
	ErrAlreadyMember       = errx.NewSentinel("user is already a member of this group")
	ErrNotGroupMember      = errx.NewSentinel("user is not a member of this group")
	ErrNotGroupAdmin       = errx.NewSentinel("user is not an admin of this group")
	ErrInvalidConfirmation = errx.NewSentinel("invalid deletion confirmation")
)

// GroupService handles business logic for location groups, memberships, and invites.
type GroupService struct {
	groupRegistry      registry.LocationGroupRegistry
	membershipRegistry registry.GroupMembershipRegistry
	inviteRegistry     registry.GroupInviteRegistry
}

// NewGroupService creates a new GroupService.
func NewGroupService(
	groupRegistry registry.LocationGroupRegistry,
	membershipRegistry registry.GroupMembershipRegistry,
	inviteRegistry registry.GroupInviteRegistry,
) *GroupService {
	return &GroupService{
		groupRegistry:      groupRegistry,
		membershipRegistry: membershipRegistry,
		inviteRegistry:     inviteRegistry,
	}
}

// CreateGroup creates a new location group and adds the creator as its admin.
func (s *GroupService) CreateGroup(ctx context.Context, tenantID, userID, name, icon string) (*models.LocationGroup, error) {
	slug, err := models.GenerateGroupSlug()
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate group slug", err)
	}

	group := models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   userID, // Required by TenantAwareEntityID; will be removed in Phase 6
		},
		Slug:      slug,
		Name:      name,
		Icon:      icon,
		Status:    models.LocationGroupStatusActive,
		CreatedBy: userID,
	}

	created, err := s.groupRegistry.Create(ctx, group)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create group", err)
	}

	// Add creator as admin
	membership := models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   userID,
		},
		GroupID:      created.ID,
		MemberUserID: userID,
		Role:         models.GroupRoleAdmin,
		JoinedAt:     time.Now(),
	}

	_, err = s.membershipRegistry.Create(ctx, membership)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create creator membership", err)
	}

	return created, nil
}

// GetGroup returns a group by ID.
func (s *GroupService) GetGroup(ctx context.Context, groupID string) (*models.LocationGroup, error) {
	return s.groupRegistry.Get(ctx, groupID)
}

// GetGroupBySlug returns a group by its slug within a tenant.
func (s *GroupService) GetGroupBySlug(ctx context.Context, tenantID, slug string) (*models.LocationGroup, error) {
	return s.groupRegistry.GetBySlug(ctx, tenantID, slug)
}

// UpdateGroup updates group metadata. Only name and icon can be changed.
func (s *GroupService) UpdateGroup(ctx context.Context, groupID, name, icon string) (*models.LocationGroup, error) {
	group, err := s.groupRegistry.Get(ctx, groupID)
	if err != nil {
		return nil, err
	}

	if !group.IsActive() {
		return nil, errxtrace.Classify(ErrGroupNotActive)
	}

	group.Name = name
	group.Icon = icon
	group.UpdatedAt = time.Now()

	return s.groupRegistry.Update(ctx, *group)
}

// InitiateGroupDeletion marks a group as pending_deletion.
// The actual deletion is handled by a background job.
func (s *GroupService) InitiateGroupDeletion(ctx context.Context, groupID, confirmWord, expectedWord string) error {
	if confirmWord != expectedWord {
		return errxtrace.Classify(ErrInvalidConfirmation)
	}

	group, err := s.groupRegistry.Get(ctx, groupID)
	if err != nil {
		return err
	}

	group.Status = models.LocationGroupStatusPendingDeletion
	group.UpdatedAt = time.Now()

	_, err = s.groupRegistry.Update(ctx, *group)
	return err
}

// ListUserGroups returns all active groups the user belongs to.
func (s *GroupService) ListUserGroups(ctx context.Context, tenantID, userID string) ([]*models.LocationGroup, error) {
	memberships, err := s.membershipRegistry.ListByUser(ctx, tenantID, userID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list user memberships", err)
	}

	var groups []*models.LocationGroup
	for _, m := range memberships {
		group, err := s.groupRegistry.Get(ctx, m.GroupID)
		if err != nil {
			continue // Skip groups that can't be loaded (e.g. pending deletion)
		}
		if group.IsActive() {
			groups = append(groups, group)
		}
	}

	return groups, nil
}

// GetMembership returns a user's membership in a group.
func (s *GroupService) GetMembership(ctx context.Context, groupID, userID string) (*models.GroupMembership, error) {
	return s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
}

// ListMembers returns all members of a group.
func (s *GroupService) ListMembers(ctx context.Context, groupID string) ([]*models.GroupMembership, error) {
	return s.membershipRegistry.ListByGroup(ctx, groupID)
}

// AddMember adds a user to a group with the specified role.
func (s *GroupService) AddMember(ctx context.Context, tenantID, groupID, userID string, role models.GroupRole) (*models.GroupMembership, error) {
	// Check if already a member
	existing, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err == nil && existing != nil {
		return nil, errxtrace.Classify(ErrAlreadyMember)
	}

	membership := models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   userID,
		},
		GroupID:      groupID,
		MemberUserID: userID,
		Role:         role,
		JoinedAt:     time.Now(),
	}

	return s.membershipRegistry.Create(ctx, membership)
}

// RemoveMember removes a user from a group. Enforces the ≥1 admin invariant.
func (s *GroupService) RemoveMember(ctx context.Context, groupID, userID string) error {
	membership, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err != nil {
		return errxtrace.Classify(ErrNotGroupMember)
	}

	// If removing an admin, ensure at least one admin remains
	if membership.Role == models.GroupRoleAdmin {
		adminCount, err := s.membershipRegistry.CountAdminsByGroup(ctx, groupID)
		if err != nil {
			return errxtrace.Wrap("failed to count admins", err)
		}
		if adminCount <= 1 {
			return errxtrace.Classify(ErrLastAdmin)
		}
	}

	return s.membershipRegistry.Delete(ctx, membership.ID)
}

// UpdateMemberRole changes a member's role. Enforces the ≥1 admin invariant.
func (s *GroupService) UpdateMemberRole(ctx context.Context, groupID, userID string, newRole models.GroupRole) (*models.GroupMembership, error) {
	membership, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err != nil {
		return nil, errxtrace.Classify(ErrNotGroupMember)
	}

	// If demoting an admin, ensure at least one admin remains
	if membership.Role == models.GroupRoleAdmin && newRole != models.GroupRoleAdmin {
		adminCount, err := s.membershipRegistry.CountAdminsByGroup(ctx, groupID)
		if err != nil {
			return nil, errxtrace.Wrap("failed to count admins", err)
		}
		if adminCount <= 1 {
			return nil, errxtrace.Classify(ErrLastAdmin)
		}
	}

	membership.Role = newRole
	return s.membershipRegistry.Update(ctx, *membership)
}

// LeaveGroup removes the current user from a group. Enforces the ≥1 admin invariant.
func (s *GroupService) LeaveGroup(ctx context.Context, groupID, userID string) error {
	return s.RemoveMember(ctx, groupID, userID)
}

// CreateInvite generates a single-use invite link for a group.
func (s *GroupService) CreateInvite(ctx context.Context, tenantID, groupID, createdByUserID string, expiresIn time.Duration) (*models.GroupInvite, error) {
	if expiresIn <= 0 {
		expiresIn = models.DefaultInviteExpiry
	}

	token, err := models.GenerateInviteToken()
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate invite token", err)
	}

	invite := models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
			UserID:   createdByUserID,
		},
		GroupID:   groupID,
		Token:     token,
		CreatedBy: createdByUserID,
		ExpiresAt: time.Now().Add(expiresIn),
	}

	return s.inviteRegistry.Create(ctx, invite)
}

// GetInviteInfo returns invite details (for display to the invitee).
func (s *GroupService) GetInviteInfo(ctx context.Context, token string) (*models.GroupInvite, *models.LocationGroup, error) {
	invite, err := s.inviteRegistry.GetByToken(ctx, token)
	if err != nil {
		return nil, nil, err
	}

	group, err := s.groupRegistry.Get(ctx, invite.GroupID)
	if err != nil {
		return nil, nil, errxtrace.Wrap("failed to get group for invite", err)
	}

	return invite, group, nil
}

// AcceptInvite accepts an invite link, creating a membership for the user.
func (s *GroupService) AcceptInvite(ctx context.Context, token, userID string) (*models.GroupMembership, error) {
	invite, err := s.inviteRegistry.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if invite.IsExpired() {
		return nil, errxtrace.Classify(ErrInviteExpired)
	}

	if invite.IsUsed() {
		return nil, errxtrace.Classify(ErrInviteAlreadyUsed)
	}

	// Check if already a member
	existing, err := s.membershipRegistry.GetByGroupAndUser(ctx, invite.GroupID, userID)
	if err == nil && existing != nil {
		return nil, errxtrace.Classify(ErrAlreadyMember)
	}

	// Mark invite as used
	now := time.Now()
	invite.UsedBy = &userID
	invite.UsedAt = &now
	_, err = s.inviteRegistry.Update(ctx, *invite)
	if err != nil {
		return nil, errxtrace.Wrap("failed to mark invite as used", err)
	}

	// Create membership (new members join as "user" role)
	membership := models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: invite.TenantID,
			UserID:   userID,
		},
		GroupID:      invite.GroupID,
		MemberUserID: userID,
		Role:         models.GroupRoleUser,
		JoinedAt:     now,
	}

	return s.membershipRegistry.Create(ctx, membership)
}

// RevokeInvite deletes an unused invite.
func (s *GroupService) RevokeInvite(ctx context.Context, inviteID string) error {
	invite, err := s.inviteRegistry.Get(ctx, inviteID)
	if err != nil {
		return err
	}

	if invite.IsUsed() {
		return errxtrace.Classify(ErrInviteAlreadyUsed, errx.Attrs("detail", "cannot revoke a used invite"))
	}

	return s.inviteRegistry.Delete(ctx, inviteID)
}

// ListActiveInvites returns all non-expired, unused invites for a group.
func (s *GroupService) ListActiveInvites(ctx context.Context, groupID string) ([]*models.GroupInvite, error) {
	return s.inviteRegistry.ListActiveByGroup(ctx, groupID)
}

// IsGroupMember checks if a user is a member of a group.
func (s *GroupService) IsGroupMember(ctx context.Context, groupID, userID string) bool {
	_, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	return err == nil
}

// IsGroupAdmin checks if a user is an admin of a group.
func (s *GroupService) IsGroupAdmin(ctx context.Context, groupID, userID string) bool {
	membership, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err != nil {
		return false
	}
	return membership.IsAdmin()
}
