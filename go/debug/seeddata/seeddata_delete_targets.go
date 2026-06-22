package seeddata

import (
	"context"
	"errors"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

// Disposable fixture emails consumed exclusively by delete-account.spec.ts
// (#2147). Each is provisioned for the well-known `test-org` tenant only,
// carries the shared TestPassword123, and is referenced by no other spec so
// a parallel Playwright run never observes one mid-deletion.
const (
	// deleteSoloTargetEmail is the sole member of its own PRIVATE group, so
	// the happy-path purge succeeds and the account is erased (204). After
	// the happy-path spec runs the row is gone; a re-seed re-creates it.
	deleteSoloTargetEmail = "delete-solo@test-org.com"

	// deleteWrongPassEmail is also the sole member of its own group, but the
	// wrong-password spec submits a bad password — so the BE rejects with
	// auth.delete.invalid_password before any purge runs and the account
	// survives untouched. Kept distinct from the solo target so neither spec
	// can clobber the other under parallel execution.
	deleteWrongPassEmail = "delete-wrongpass@test-org.com"

	// deleteLastOwnerEmail is the SOLE owner of a shared group that has a
	// second (non-owner) member, so a valid-credentials delete is blocked
	// with auth.delete.last_owner. The second member is a dedicated fixture
	// user so the group genuinely stays shared after any membership churn.
	deleteLastOwnerEmail = "delete-lastowner@test-org.com"

	// deleteLastOwnerMemberEmail fills the second-member slot on the
	// last-owner fixture's group so that group is genuinely shared.
	deleteLastOwnerMemberEmail = "delete-lastowner-member@test-org.com"
)

// ensureDeleteTargetFixtures idempotently provisions the disposable
// self-service-account-deletion fixtures (#2147). All three target users own
// their own group (via findOrCreateDefaultGroup, which mints a "Default"
// group with the user as GroupRoleOwner) so they exercise the real purge /
// last-owner classification paths in services.AccountDeletionService rather
// than the orphan/no-group edge case. Gated on the test-org tenant by the
// caller, same as the other test-org fixtures.
func ensureDeleteTargetFixtures(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User) error {
	now := time.Now()

	// Scenario 1 — happy path: sole member of own private group → purge OK.
	if _, err := ensureDeleteTargetUser(ctx, registrySet, tenant, users, deleteSoloTargetEmail, "Delete Solo Target"); err != nil {
		return errxtrace.Wrap("failed to provision delete-solo target fixture", err)
	}

	// Scenario 2 — wrong password: sole member of own private group; the bad
	// password fails the re-auth before any purge, so this account survives.
	if _, err := ensureDeleteTargetUser(ctx, registrySet, tenant, users, deleteWrongPassEmail, "Delete Wrong-Password Target"); err != nil {
		return errxtrace.Wrap("failed to provision delete-wrongpass target fixture", err)
	}

	// Scenario 3 — last-owner block: sole owner of a SHARED group (a second
	// member exists), so a valid-creds delete is rejected with
	// auth.delete.last_owner.
	lastOwner, err := ensureDeleteTargetUser(ctx, registrySet, tenant, users, deleteLastOwnerEmail, "Delete Last-Owner Target")
	if err != nil {
		return errxtrace.Wrap("failed to provision delete-lastowner target fixture", err)
	}
	// findOrCreateDefaultGroup (called inside ensureDeleteTargetUser) returns
	// the user's owned group, but we re-derive it here straight from the
	// last-owner's group so we never rely on the in-memory user pointer being
	// stamped — on a re-seed the early-return-when-already-default branch
	// leaves the pointer's DefaultGroupID untouched.
	lastOwnerGroup, err := findOrCreateDefaultGroup(ctx, registrySet, lastOwner, models.Currency("CZK"))
	if err != nil {
		return errxtrace.Wrap("failed to resolve last-owner shared group", err)
	}
	if err := ensureLastOwnerSecondMember(ctx, registrySet, tenant, lastOwnerGroup.ID, now); err != nil {
		return errxtrace.Wrap("failed to provision delete-lastowner second member", err)
	}

	return nil
}

// ensureDeleteTargetUser find-or-creates an active fixture user and guarantees
// it owns its own group (default group, GroupRoleOwner). Returns the user so
// the caller can reference its group for the shared-member wiring.
//
// The happy-path spec deletes its user outright, so on a re-seed the user is
// missing and gets re-created here; the wrong-password / last-owner specs
// leave their users intact, so they hit the find-and-reconcile branch.
func ensureDeleteTargetUser(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, users []*models.User, email, name string) (*models.User, error) {
	var target *models.User
	for _, user := range users {
		if user.TenantID == tenant.ID && user.Email == email {
			target = user
			break
		}
	}

	if target == nil {
		newUser := models.User{
			TenantAwareEntityID: models.TenantAwareEntityID{
				TenantID: tenant.ID,
			},
			Email:    email,
			Name:     name,
			IsActive: true,
		}
		if err := newUser.SetPassword("TestPassword123"); err != nil {
			return nil, err
		}
		created, err := registrySet.UserRegistry.Create(ctx, newUser)
		if err != nil {
			return nil, errxtrace.Wrap("failed to create delete-target test user", err)
		}
		target = created
	} else if !target.IsActive {
		// Reconcile drift: a prior failed run could have left the fixture
		// deactivated. The happy-path purge deletes the row entirely rather
		// than deactivating, so this only guards against unexpected states.
		target.IsActive = true
		updated, err := registrySet.UserRegistry.Update(ctx, *target)
		if err != nil {
			return nil, errxtrace.Wrap("failed to reconcile delete-target test user", err)
		}
		target = updated
	}

	// Guarantee the user owns its own group. CZK keeps it disjoint from the
	// USD sysadmin group and consistent with the rest of the test-org seed.
	if _, err := findOrCreateDefaultGroup(ctx, registrySet, target, models.Currency("CZK")); err != nil {
		return nil, errxtrace.Wrap("failed to ensure delete-target default group", err)
	}
	return target, nil
}

// ensureLastOwnerSecondMember adds a dedicated second member (GroupRoleUser,
// not an owner) to the last-owner fixture's group so the group is genuinely
// shared and the sole-owner classification in
// services.AccountDeletionService fires. Idempotent on re-runs.
func ensureLastOwnerSecondMember(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant, groupID string, now time.Time) error {
	member, err := ensureDeleteTargetSecondMemberUser(ctx, registrySet, tenant)
	if err != nil {
		return err
	}

	existing, err := registrySet.GroupMembershipRegistry.GetByGroupAndUser(ctx, groupID, member.ID)
	switch {
	case err == nil && existing != nil:
		return nil
	case errors.Is(err, registry.ErrNotFound):
		// proceed to create the membership below
	case err != nil:
		return errxtrace.Wrap("failed to look up last-owner second membership", err)
	}

	if _, err := registrySet.GroupMembershipRegistry.Create(ctx, models.GroupMembership{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		GroupID:      groupID,
		MemberUserID: member.ID,
		Role:         models.GroupRoleUser,
		JoinedAt:     now.AddDate(0, 0, -10),
	}); err != nil {
		return errxtrace.Wrap("failed to add second member to last-owner group", err)
	}
	return nil
}

// ensureDeleteTargetSecondMemberUser find-or-creates the plain user that
// occupies the second-member slot on the last-owner fixture's group. It is
// intentionally never used as a deletion target itself.
func ensureDeleteTargetSecondMemberUser(ctx context.Context, registrySet *registry.Set, tenant *models.Tenant) (*models.User, error) {
	users, err := registrySet.UserRegistry.ListByTenant(ctx, tenant.ID)
	if err != nil {
		return nil, errxtrace.Wrap("failed to list users for last-owner member lookup", err)
	}
	for _, user := range users {
		if user.Email == deleteLastOwnerMemberEmail {
			return user, nil
		}
	}

	member := models.User{
		TenantAwareEntityID: models.TenantAwareEntityID{
			TenantID: tenant.ID,
		},
		Email:    deleteLastOwnerMemberEmail,
		Name:     "Delete Last-Owner Second Member",
		IsActive: true,
	}
	if err := member.SetPassword("TestPassword123"); err != nil {
		return nil, err
	}
	created, err := registrySet.UserRegistry.Create(ctx, member)
	if err != nil {
		return nil, errxtrace.Wrap("failed to create last-owner second member user", err)
	}
	return created, nil
}
