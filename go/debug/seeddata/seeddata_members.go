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
// The function is keyed on user2 being available because the design
// of the existing seed already creates user2 to demonstrate two-
// currency tenants; reusing it as the teammate keeps the user table
// small.
func seedGroupMembers(ctx context.Context, set *registry.Set, tenant *models.Tenant, user1, user2 *models.User, group1 *models.LocationGroup) error {
	now := time.Now()

	// 1) Promote group1 to a multi-member group: add user2 as a
	//    `user`-role teammate. Idempotent — when the membership row
	//    already exists, GetByGroupAndUser returns it and we skip
	//    the insert. ErrNotFound is the only "missing" signal we
	//    treat as "go ahead and create"; any other error is
	//    surfaced so a transient DB failure doesn't get swallowed
	//    and then re-emerge as a duplicate-key Create error.
	if err := ensureUser2Membership(ctx, set, tenant, group1, user2, now); err != nil {
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

	if _, err := set.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		GroupID:      family.ID,
		MemberUserID: user1.ID,
		Role:         models.GroupRoleUser,
		JoinedAt:     now.AddDate(0, 0, -60),
	}); err != nil {
		return fmt.Errorf("add user1 to family group: %w", err)
	}

	return nil
}

// ensureUser2Membership adds user2 to group1 as a `user`-role
// member, or no-ops when the membership already exists. Treats
// registry.ErrNotFound as "row missing → create"; any other lookup
// error is propagated.
func ensureUser2Membership(ctx context.Context, set *registry.Set, tenant *models.Tenant, group1 *models.LocationGroup, user2 *models.User, now time.Time) error {
	existing, err := set.GroupMembershipRegistry.GetByGroupAndUser(ctx, group1.ID, user2.ID)
	switch {
	case err == nil && existing != nil:
		return nil
	case errors.Is(err, registry.ErrNotFound):
		// proceed
	case err != nil:
		return fmt.Errorf("lookup user2 membership: %w", err)
	}
	if _, err := set.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		GroupID:      group1.ID,
		MemberUserID: user2.ID,
		Role:         models.GroupRoleUser,
		JoinedAt:     now.AddDate(0, 0, -30),
	}); err != nil {
		return fmt.Errorf("add user2 to default group: %w", err)
	}
	return nil
}

// ensureFamilyOwner looks up (or creates) the "family@test-org.com"
// user that owns the second seed group.
func ensureFamilyOwner(ctx context.Context, set *registry.Set, tenant *models.Tenant, now time.Time) (*models.User, error) {
	const familyEmail = "family@test-org.com"
	users, err := set.UserRegistry.ListByTenant(ctx, tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("list users for family-owner lookup: %w", err)
	}
	for _, u := range users {
		if u.Email == familyEmail {
			return u, nil
		}
	}

	familyOwner := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    familyEmail,
		Name:     "Test Family",
		IsActive: true,
	}
	if err := familyOwner.SetPassword("TestPassword123"); err != nil {
		return nil, err
	}
	familyOwner.CreatedAt = now.AddDate(0, 0, -150)
	familyOwner.UpdatedAt = now
	created, err := set.UserRegistry.Create(ctx, familyOwner)
	if err != nil {
		return nil, fmt.Errorf("create family owner user: %w", err)
	}
	return created, nil
}
