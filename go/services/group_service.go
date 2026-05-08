package services

import (
	"context"
	"errors"
	"log/slog"
	"sort"
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
	// ErrInvalidPassword is distinct from ErrInvalidConfirmation so the
	// frontend can render different copy ("wrong group name" vs. "wrong
	// password"). See spec #1219 §12.
	ErrInvalidPassword  = errx.NewSentinel("invalid password")
	ErrInviteNotInGroup = errx.NewSentinel("invite does not belong to this group")
)

// GroupService handles business logic for location groups, memberships, and invites.
type GroupService struct {
	groupRegistry      registry.LocationGroupRegistry
	membershipRegistry registry.GroupMembershipRegistry
	inviteRegistry     registry.GroupInviteRegistry
	// userRegistry is optional; when nil, EnsureDefaultGroup degrades to a
	// no-op. The auth-aware bootstrap wires it in so CreateGroup, AcceptInvite
	// and RemoveMember can promote a deterministic membership to default for
	// the affected user (#1592). Tests that don't care about the default-group
	// invariant can construct the service without it via NewGroupService.
	userRegistry registry.UserRegistry
}

// NewGroupService creates a new GroupService without default-group auto-promotion.
// Call SetUserRegistry to enable the EnsureDefaultGroup invariant (#1592).
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

// SetUserRegistry enables EnsureDefaultGroup auto-promotion (#1592). When set,
// CreateGroup / AcceptInvite / RemoveMember will keep the user's
// default_group_id pointing at one of their memberships whenever possible.
func (s *GroupService) SetUserRegistry(userRegistry registry.UserRegistry) {
	s.userRegistry = userRegistry
}

// CreateGroup creates a new location group and adds the creator as its admin.
// An empty groupCurrency falls back to USD so memory-backed registries (which
// don't apply DB defaults) still produce a valid group — commodity validation
// would otherwise trip on an empty currency.
func (s *GroupService) CreateGroup(ctx context.Context, tenantID, userID, name, icon string, groupCurrency models.Currency) (*models.LocationGroup, error) {
	slug, err := models.GenerateGroupSlug()
	if err != nil {
		return nil, errxtrace.Wrap("failed to generate group slug", err)
	}

	if groupCurrency == "" {
		groupCurrency = models.Currency("USD")
	}

	group := models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
		},
		Slug:          slug,
		Name:          name,
		Icon:          icon,
		Status:        models.LocationGroupStatusActive,
		CreatedBy:     userID,
		GroupCurrency: groupCurrency,
	}

	created, err := s.groupRegistry.Create(ctx, group)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create group", err)
	}

	// Add creator as admin. The two writes aren't wrapped in a single
	// transaction (the registries hold their own DB handles), so if the
	// membership insert fails, compensate by deleting the just-created
	// group — otherwise we'd leak a group with no admin and violate the
	// "≥1 admin per group" invariant.
	membership := models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
		},
		GroupID:      created.ID,
		MemberUserID: userID,
		Role:         models.GroupRoleAdmin,
		JoinedAt:     time.Now(),
	}

	_, err = s.membershipRegistry.Create(ctx, membership)
	if err != nil {
		if delErr := s.groupRegistry.Delete(ctx, created.ID); delErr != nil {
			return nil, errxtrace.Wrap("failed to create creator membership (and failed to roll back the group)", errors.Join(err, delErr))
		}
		return nil, errxtrace.Wrap("failed to create creator membership", err)
	}

	// Promote the freshly-created membership to the user's default if they
	// don't have a valid one yet (#1592). Failures are logged inside the
	// helper and do not roll back the create — the user can still pick a
	// default later via PATCH /me, and the invariant is restored on the next
	// membership change or by the boot-time backfill.
	s.ensureDefaultGroupBestEffort(ctx, userID)

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
			// Only swallow NotFound — a membership row can outlive its
			// group during an in-progress deletion. Real errors (DB
			// outage, timeout, etc.) must bubble up instead of being
			// reported as a partial-but-successful list.
			if errors.Is(err, registry.ErrNotFound) {
				continue
			}
			return nil, errxtrace.Wrap("failed to load group for membership", err, errx.Attrs("group_id", m.GroupID))
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
	// Check if already a member. Only swallow NotFound — any other error
	// from the registry (DB outage, timeout, etc.) must surface, otherwise
	// we'd silently fall through to create a duplicate membership.
	existing, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err == nil && existing != nil {
		return nil, errxtrace.Classify(ErrAlreadyMember)
	}
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to look up existing membership", err)
	}

	membership := models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenantID,
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
		if errors.Is(err, registry.ErrNotFound) {
			return errxtrace.Classify(ErrNotGroupMember)
		}
		return errxtrace.Wrap("failed to look up membership", err)
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

	if err := s.membershipRegistry.Delete(ctx, membership.ID); err != nil {
		return err
	}

	// Auto-promote a remaining membership to default if the user lost the one
	// they pointed at (#1592). Best-effort — see ensureDefaultGroupBestEffort.
	s.ensureDefaultGroupBestEffort(ctx, userID)
	return nil
}

// UpdateMemberRole changes a member's role. Enforces the ≥1 admin invariant.
func (s *GroupService) UpdateMemberRole(ctx context.Context, groupID, userID string, newRole models.GroupRole) (*models.GroupMembership, error) {
	membership, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err != nil {
		if errors.Is(err, registry.ErrNotFound) {
			return nil, errxtrace.Classify(ErrNotGroupMember)
		}
		return nil, errxtrace.Wrap("failed to look up membership", err)
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
// expectedTenantID is the tenant of the authenticated caller — it must match
// the invite's tenant, otherwise we'd create a cross-tenant membership (which
// in memory silently violates isolation and on PostgreSQL fails RLS with a
// confusing error).
func (s *GroupService) AcceptInvite(ctx context.Context, token, userID, expectedTenantID string) (*models.GroupMembership, error) {
	invite, err := s.inviteRegistry.GetByToken(ctx, token)
	if err != nil {
		return nil, err
	}

	if invite.TenantID != expectedTenantID {
		// Don't leak the distinction between "token not found" and
		// "token belongs to another tenant".
		return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs("entity_type", "GroupInvite"))
	}

	if invite.IsExpired() {
		return nil, errxtrace.Classify(ErrInviteExpired)
	}

	if invite.IsUsed() {
		return nil, errxtrace.Classify(ErrInviteAlreadyUsed)
	}

	// Check if already a member. Distinguish real failures from NotFound.
	existing, err := s.membershipRegistry.GetByGroupAndUser(ctx, invite.GroupID, userID)
	if err == nil && existing != nil {
		return nil, errxtrace.Classify(ErrAlreadyMember)
	}
	if err != nil && !errors.Is(err, registry.ErrNotFound) {
		return nil, errxtrace.Wrap("failed to look up existing membership", err)
	}

	// Atomically mark the invite as used via compare-and-swap. Two concurrent
	// accept requests both pass the IsUsed check above, but only one wins the
	// CAS here — the other gets (false, nil) and is rejected with
	// ErrInviteAlreadyUsed, preventing double-redemption.
	now := time.Now()
	won, err := s.inviteRegistry.MarkUsed(ctx, invite.ID, userID, now)
	if err != nil {
		return nil, errxtrace.Wrap("failed to mark invite as used", err)
	}
	if !won {
		return nil, errxtrace.Classify(ErrInviteAlreadyUsed)
	}

	// Create membership (new members join as "user" role). Build it with
	// the invite's tenant (== expectedTenantID, verified above).
	membership := models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: invite.TenantID,
		},
		GroupID:      invite.GroupID,
		MemberUserID: userID,
		Role:         models.GroupRoleUser,
		JoinedAt:     now,
	}

	created, err := s.membershipRegistry.Create(ctx, membership)
	if err != nil {
		// Best-effort compensating revert of the invite. We can't fully
		// unwind without transactions across registries; surface the
		// primary failure plus any revert error in errors.Join.
		invite.UsedBy = nil
		invite.UsedAt = nil
		if _, revertErr := s.inviteRegistry.Update(ctx, *invite); revertErr != nil {
			return nil, errxtrace.Wrap("failed to create membership (and failed to revert invite to unused)", errors.Join(err, revertErr))
		}
		return nil, errxtrace.Wrap("failed to create membership", err)
	}

	// Promote the freshly-created membership to the user's default if they
	// don't have a valid one yet (#1592). Best-effort — see
	// ensureDefaultGroupBestEffort.
	s.ensureDefaultGroupBestEffort(ctx, userID)

	return created, nil
}

// RevokeInviteForGroup verifies the invite belongs to the given group, then deletes it.
// It returns ErrInviteNotInGroup if the invite exists but belongs to a different group.
func (s *GroupService) RevokeInviteForGroup(ctx context.Context, groupID, inviteID string) error {
	invite, err := s.inviteRegistry.Get(ctx, inviteID)
	if err != nil {
		return err
	}

	if invite.GroupID != groupID {
		return errxtrace.Classify(ErrInviteNotInGroup)
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

// IsGroupMember checks if a user is a member of a group. Any error (including
// transient registry failures) is treated as "not a member" — callers that
// need to distinguish a legitimate non-membership from an infrastructure error
// should use CheckGroupMembership instead.
func (s *GroupService) IsGroupMember(ctx context.Context, groupID, userID string) bool {
	_, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	return err == nil
}

// CheckGroupMembership returns (isMember, err). isMember is true only when a
// membership row exists. err is non-nil only for unexpected/transient failures
// — a missing membership is returned as (false, nil). Use this in HTTP
// middleware so that DB outages surface as 5xx instead of being masked as 403.
func (s *GroupService) CheckGroupMembership(ctx context.Context, groupID, userID string) (bool, error) {
	_, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, registry.ErrNotFound) {
		return false, nil
	}
	return false, err
}

// IsGroupAdmin checks if a user is an admin of a group.
func (s *GroupService) IsGroupAdmin(ctx context.Context, groupID, userID string) bool {
	membership, err := s.membershipRegistry.GetByGroupAndUser(ctx, groupID, userID)
	if err != nil {
		return false
	}
	return membership.IsAdmin()
}

// EnsureDefaultGroup enforces the #1592 invariant on a single user. See
// EnsureUserDefaultGroup for the full description; this method is the
// service-bound shortcut that uses the GroupService's wired registries.
func (s *GroupService) EnsureDefaultGroup(ctx context.Context, userID string) error {
	if s.userRegistry == nil {
		return errxtrace.Wrap("EnsureDefaultGroup called without a UserRegistry", registry.ErrFieldRequired)
	}
	return EnsureUserDefaultGroup(ctx, s.userRegistry, s.membershipRegistry, userID)
}

// EnsureUserDefaultGroup is the package-level helper that enforces the #1592
// invariant:
//
//	default_group_id is NULL only when the user has zero memberships.
//	As soon as the user has ≥1 membership, exactly one of them is the
//	default.
//
// If default_group_id already points at a current membership, this is a
// no-op. Otherwise:
//   - with ≥1 membership, the deterministic earliest joined_at (ties broken
//     by group_id ascending) is promoted;
//   - with zero memberships, default_group_id is cleared.
//
// Exposed as a free function so the group-purge worker can run it without
// constructing a full GroupService.
func EnsureUserDefaultGroup(ctx context.Context, users registry.UserRegistry, memberships registry.GroupMembershipRegistry, userID string) error {
	if users == nil {
		return errxtrace.Wrap("EnsureUserDefaultGroup called without a UserRegistry", registry.ErrFieldRequired)
	}
	if memberships == nil {
		return errxtrace.Wrap("EnsureUserDefaultGroup called without a GroupMembershipRegistry", registry.ErrFieldRequired)
	}
	if userID == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "userID"))
	}

	user, err := users.Get(ctx, userID)
	if err != nil {
		return errxtrace.Wrap("failed to load user", err)
	}

	rows, err := memberships.ListByUser(ctx, user.TenantID, userID)
	if err != nil {
		return errxtrace.Wrap("failed to list memberships", err)
	}

	chosen := pickDefaultMembership(rows)

	switch {
	case chosen == nil:
		if user.DefaultGroupID == nil {
			return nil
		}
		user.DefaultGroupID = nil
	case user.DefaultGroupID != nil && membershipExistsForGroup(rows, *user.DefaultGroupID):
		return nil
	default:
		groupID := chosen.GroupID
		user.DefaultGroupID = &groupID
	}

	user.UpdatedAt = time.Now()
	if _, err := users.Update(ctx, *user); err != nil {
		return errxtrace.Wrap("failed to persist default_group_id", err)
	}
	return nil
}

// ensureDefaultGroupBestEffort runs EnsureDefaultGroup, logs any error, and
// returns to the caller. Used in CreateGroup / AcceptInvite / RemoveMember
// where the primary write has already succeeded — we don't want a transient
// registry blip to surface as a 5xx after the membership is real, because the
// next interaction (or the boot-time backfill) will re-establish the
// invariant. The slog warning makes the silent swallow observable in
// production logs so the operator can spot a hot loop of failed promotions.
func (s *GroupService) ensureDefaultGroupBestEffort(ctx context.Context, userID string) {
	if s.userRegistry == nil {
		return
	}
	if err := s.EnsureDefaultGroup(ctx, userID); err != nil {
		slog.WarnContext(ctx, "failed to reconcile default_group_id (best-effort)",
			"user_id", userID,
			"error", err,
		)
	}
}

// pickDefaultMembership picks the deterministic membership to promote: the
// earliest joined_at, ties broken by ascending group_id. Returns nil if the
// slice is empty.
func pickDefaultMembership(memberships []*models.GroupMembership) *models.GroupMembership {
	if len(memberships) == 0 {
		return nil
	}
	candidates := make([]*models.GroupMembership, 0, len(memberships))
	for _, m := range memberships {
		if m == nil {
			continue
		}
		candidates = append(candidates, m)
	}
	if len(candidates) == 0 {
		return nil
	}
	sort.Slice(candidates, func(i, j int) bool {
		if !candidates[i].JoinedAt.Equal(candidates[j].JoinedAt) {
			return candidates[i].JoinedAt.Before(candidates[j].JoinedAt)
		}
		return candidates[i].GroupID < candidates[j].GroupID
	})
	return candidates[0]
}

// membershipExistsForGroup is true when any membership row points at groupID.
func membershipExistsForGroup(memberships []*models.GroupMembership, groupID string) bool {
	for _, m := range memberships {
		if m != nil && m.GroupID == groupID {
			return true
		}
	}
	return false
}
