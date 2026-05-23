package seeddata

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// seedGroupMembers turns the (admin-only) primary group into a small,
// realistic team and adds a pending email invite so the Members page
// (#1533) has non-trivial content on the demo seed. Also provisions
// an additional "Family" location group with the admin as a non-owner
// member so the group switcher shows more than one row.
//
// The teammate role is filled by a NEW fixture user
// (`teammate@test-org.com`) rather than the pre-existing user2 —
// adding user2 to admin's group would break the user-isolation e2e
// specs (`user-isolation.spec.ts`) which rely on admin and user2
// being in DIFFERENT groups so cross-user reads return 404. user2
// keeps its EUR-valued solo group; teammate joins admin's CZK group.
func seedGroupMembers(ctx context.Context, set *registry.Set, tenant *models.Tenant, user1, user2 *models.User, group1 *models.LocationGroup) error {
	_ = user2 // user2 stays isolated in its own EUR group — see comment above
	now := time.Now()

	// 1) Promote group1 to a multi-member group by minting a dedicated
	//    `teammate@test-org.com` user and granting it the `user` role
	//    on admin's primary group. Both the user and the membership
	//    are find-or-create so re-running the seed is a no-op.
	teammate, err := ensureTeammateUser(ctx, set, tenant, now)
	if err != nil {
		return err
	}
	if err := ensureTeammateMembership(ctx, set, tenant, group1, teammate, now); err != nil {
		return err
	}

	// 2) Pending viewer-role invite. Token uses the regular
	//    GenerateInviteToken (cryptographically random) so the
	//    invite is indistinguishable from one created by the real
	//    /invites endpoint — the goal is a realistic Members page,
	//    not a deterministic fixture URL.
	token, err := models.GenerateInviteToken()
	if err != nil {
		return fmt.Errorf("generate invite token: %w", err)
	}
	inviteeEmail := "invited.viewer@example.org"
	if _, err := set.GroupInviteRegistry.Create(ctx, models.GroupInvite{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		GroupID:      group1.ID,
		Token:        token,
		CreatedBy:    user1.ID,
		ExpiresAt:    now.Add(7 * 24 * time.Hour),
		InviteeEmail: &inviteeEmail,
		Role:         models.GroupRoleViewer,
		CreatedAt:    now,
	}); err != nil {
		return fmt.Errorf("create pending invite: %w", err)
	}

	// 3) Second group: admin (user1) is a non-owner member of a
	//    "Family" group owned by a third seeded user. Lets the
	//    group switcher dropdown show multiple rows.
	familyOwner, err := ensureFamilyOwner(ctx, set, tenant, now)
	if err != nil {
		return err
	}

	familySlug, err := models.GenerateGroupSlug()
	if err != nil {
		return fmt.Errorf("generate family slug: %w", err)
	}
	family, err := set.LocationGroupRegistry.Create(ctx, models.LocationGroup{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Slug:          familySlug,
		Name:          "Family",
		Status:        models.LocationGroupStatusActive,
		CreatedBy:     familyOwner.ID,
		GroupCurrency: models.Currency("CZK"),
	})
	if err != nil {
		return fmt.Errorf("create family group: %w", err)
	}

	if _, err := set.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		GroupID:      family.ID,
		MemberUserID: familyOwner.ID,
		Role:         models.GroupRoleOwner,
		JoinedAt:     now.AddDate(0, 0, -120),
	}); err != nil {
		return fmt.Errorf("create family owner membership: %w", err)
	}

	// Invariant: admin's Family membership must JoinedAt strictly LATER
	// than admin's own owner memberships. The default-group re-election
	// tiebreaker (`pickDefaultMembership` in services/group_service.go)
	// sorts by joined_at ASC and picks the oldest — and admin only carries
	// GroupRoleUser on Family. Letting Family win that tiebreaker
	// re-promotes admin onto a group where they cannot write, breaking the
	// rest of the e2e suite the next time `GroupPurgeService` re-elects a
	// default (#1841).
	if _, err := set.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		GroupID:      family.ID,
		MemberUserID: user1.ID,
		Role:         models.GroupRoleUser,
		JoinedAt:     now.AddDate(0, 0, 1),
	}); err != nil {
		return fmt.Errorf("add user1 to family group: %w", err)
	}

	return nil
}

// ensureTeammateUser looks up (or creates) `teammate@test-org.com`,
// the fixture user that fills the second-member slot on admin's
// primary group. Kept separate from user2 so the user-isolation e2e
// specs continue to find user2 in a disjoint group.
func ensureTeammateUser(ctx context.Context, set *registry.Set, tenant *models.Tenant, now time.Time) (*models.User, error) {
	return findOrCreateFixtureUser(ctx, set, tenant, now, "teammate@test-org.com", "Test Teammate", -45)
}

// ensureTeammateMembership grants the teammate fixture a `user`-role
// membership on admin's primary group, idempotent on re-runs.
func ensureTeammateMembership(ctx context.Context, set *registry.Set, tenant *models.Tenant, group1 *models.LocationGroup, teammate *models.User, now time.Time) error {
	existing, err := set.GroupMembershipRegistry.GetByGroupAndUser(ctx, group1.ID, teammate.ID)
	switch {
	case err == nil && existing != nil:
		return nil
	case errors.Is(err, registry.ErrNotFound):
		// proceed
	case err != nil:
		return fmt.Errorf("lookup teammate membership: %w", err)
	}
	if _, err := set.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		GroupID:      group1.ID,
		MemberUserID: teammate.ID,
		Role:         models.GroupRoleUser,
		JoinedAt:     now.AddDate(0, 0, -30),
	}); err != nil {
		return fmt.Errorf("add teammate to default group: %w", err)
	}
	return nil
}

// ensureFamilyOwner looks up (or creates) the "family@test-org.com"
// user that owns the second seed group.
func ensureFamilyOwner(ctx context.Context, set *registry.Set, tenant *models.Tenant, now time.Time) (*models.User, error) {
	return findOrCreateFixtureUser(ctx, set, tenant, now, "family@test-org.com", "Test Family", -150)
}

// findOrCreateFixtureUser is the find-or-create helper shared by the
// teammate / family-owner fixtures. backdatedDays is the offset
// applied to CreatedAt so the seeded user has a believable
// "joined N days ago" history in the audit views.
func findOrCreateFixtureUser(ctx context.Context, set *registry.Set, tenant *models.Tenant, now time.Time, email, name string, backdatedDays int) (*models.User, error) {
	users, err := set.UserRegistry.ListByTenant(ctx, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("list users for %s lookup: %w", email, err)
	}
	for _, u := range users {
		if u.Email == email {
			return u, nil
		}
	}

	user := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    email,
		Name:     name,
		IsActive: true,
	}
	if err := user.SetPassword("TestPassword123"); err != nil {
		return nil, err
	}
	user.CreatedAt = now.AddDate(0, 0, backdatedDays)
	user.UpdatedAt = now
	created, err := set.UserRegistry.Create(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("create %s user: %w", email, err)
	}
	return created, nil
}
