package memory

import (
	"context"
	"time"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var _ registry.UserTenantMover = (*UserTenantMover)(nil)

// UserTenantMover is the in-memory counterpart to postgres.UserTenantMover
// (#2179). It holds the membership store's write lock while it counts the
// user's memberships and rewrites the users row, so a membership cannot be
// added between the two. The content count runs before that lock and the
// per-user rows are moved after it: the memory stores have no shared
// transaction, and the backend is for development and tests.
//
// Parity gaps: group notification preferences, operation slots and thumbnail
// jobs have no per-user delete in memory and are left alone; settings are
// keyed by user alone and need no move.
type UserTenantMover struct {
	fs          *registry.FactorySet
	users       *UserRegistry
	memberships *GroupMembershipRegistry
	purger      *UserPurger
}

// NewUserTenantMover wires a mover to the populated FactorySet and the stores
// it changes directly.
func NewUserTenantMover(fs *registry.FactorySet, users *UserRegistry, memberships *GroupMembershipRegistry, purger *UserPurger) *UserTenantMover {
	return &UserTenantMover{fs: fs, users: users, memberships: memberships, purger: purger}
}

// CountOwnedContent implements registry.UserTenantMover.
func (m *UserTenantMover) CountOwnedContent(ctx context.Context, userID string) (registry.UserOwnedContent, error) {
	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}
	counts, err := countOwnedContent(ctx, ownedSources(m.fs), userID, func(ownedSource) bool { return true })
	if err != nil {
		return nil, errxtrace.Wrap("failed to count the user's content", err)
	}
	return registry.NewUserOwnedContent(counts), nil
}

// MoveUserToTenant implements registry.UserTenantMover.
func (m *UserTenantMover) MoveUserToTenant(ctx context.Context, move registry.UserTenantMove) (*models.User, error) {
	if err := validateMove(move); err != nil {
		return nil, err
	}
	current, err := m.users.Get(ctx, move.UserID)
	if err != nil {
		return nil, err
	}
	if current.TenantID != move.FromTenantID {
		return nil, errxtrace.Classify(registry.ErrUserTenantChanged, errx.Attrs(
			"user_id", move.UserID, "tenant_id", current.TenantID, "expected_tenant_id", move.FromTenantID))
	}
	if _, err := m.fs.TenantRegistry.Get(ctx, move.ToTenantID); err != nil {
		return nil, errxtrace.Wrap("failed to find the target tenant", err)
	}

	counts, err := countOwnedContent(ctx, ownedSources(m.fs), move.UserID, func(src ownedSource) bool {
		return src.kind != registry.OwnedGroupMemberships
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to count the user's content", err)
	}

	moved, err := m.moveUserUnderMembershipLock(move, counts)
	if err != nil {
		return nil, err
	}
	if err := m.movePerUserRows(ctx, move); err != nil {
		return nil, errxtrace.Wrap("the user moved, but their per-user rows did not", err)
	}
	return moved, nil
}

func validateMove(move registry.UserTenantMove) error {
	switch {
	case move.UserID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	case move.FromTenantID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "FromTenantID"))
	case move.ToTenantID == "":
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ToTenantID"))
	case move.FromTenantID == move.ToTenantID:
		return errxtrace.Classify(registry.ErrInvalidInput, errx.Attrs("reason", "the user is already in the target tenant"))
	}
	return nil
}

// moveUserUnderMembershipLock counts the user's memberships and rewrites the
// users row while no membership can be added.
func (m *UserTenantMover) moveUserUnderMembershipLock(move registry.UserTenantMove, counts map[registry.OwnedContentKind]int) (*models.User, error) {
	m.memberships.lock.Lock()
	defer m.memberships.lock.Unlock()

	for pair := m.memberships.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.MemberUserID == move.UserID {
			counts[registry.OwnedGroupMemberships]++
		}
	}
	if owned := registry.NewUserOwnedContent(counts); !owned.IsEmpty() {
		return nil, &registry.UserTenantMoveBlockedError{Owned: owned}
	}
	return m.users.moveToTenant(move, time.Now().UTC())
}

// movePerUserRows deletes the user's sessions, tokens and per-group rows and
// moves the rows that follow the person, as postgres does.
func (m *UserTenantMover) movePerUserRows(ctx context.Context, move registry.UserTenantMove) error {
	userID := move.UserID
	steps := []func() error{
		func() error { return m.purger.purgeRefreshTokens(ctx, userID) },
		func() error { return m.purger.apiTokens.DeleteByUserID(ctx, userID) },
		func() error { return m.purger.purgeEmailVerifications(ctx, userID) },
		func() error { return m.purger.passwordReset.DeleteByUserID(ctx, userID) },
		func() error { return m.purger.magicLink.DeleteByUserID(ctx, userID) },
		func() error { return m.purger.purgeSavedViewPlacements(move.FromTenantID, userID) },
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return err
		}
	}

	if mfa, ok := m.fs.UserMFASecretRegistry.(*UserMFASecretRegistry); ok {
		retenantRows(mfa.baseUserMFASecretRegistry, move.ToTenantID, func(s *models.UserMFASecret) bool { return s.UserID == userID })
	}
	if oauth, ok := m.fs.OAuthIdentityRegistry.(*OAuthIdentityRegistry); ok {
		retenantRows(oauth.baseOAuthIdentityRegistry, move.ToTenantID, func(oi *models.OAuthIdentity) bool { return oi.UserID == userID })
	}
	if events, ok := m.fs.LoginEventRegistry.(*LoginEventRegistry); ok {
		retenantRows(events.baseLoginEventRegistry, move.ToTenantID, func(e *models.LoginEvent) bool { return e.UserID != nil && *e.UserID == userID })
	}
	if digests, ok := m.fs.WeeklyDigestSendRegistry.(*WeeklyDigestSendRegistry); ok {
		retenantRows(digests.baseWeeklyDigestSendRegistry, move.ToTenantID, func(d *models.WeeklyDigestSend) bool { return d.UserID == userID })
	}
	if audits, ok := m.fs.CommodityScanAuditRegistry.(*CommodityScanAuditRegistry); ok {
		audits.mu.Lock()
		for _, a := range audits.items {
			if a.UserID == userID {
				a.TenantID = move.ToTenantID
			}
		}
		audits.mu.Unlock()
	}
	return nil
}

// retenantRows sets the tenant of every row of a store that match picks.
func retenantRows[T any, P registry.PIDable[T]](base *Registry[T, P], tenantID string, match func(P) bool) {
	base.lock.Lock()
	defer base.lock.Unlock()
	for pair := base.items.Oldest(); pair != nil; pair = pair.Next() {
		if !match(pair.Value) {
			continue
		}
		if ta, ok := any(pair.Value).(interface{ SetTenantID(string) }); ok {
			ta.SetTenantID(tenantID)
		}
	}
}
