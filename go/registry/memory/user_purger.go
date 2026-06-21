package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"github.com/denisvmedia/inventario/registry"
)

var _ registry.UserPurger = (*UserPurger)(nil)

// UserPurger is the in-memory counterpart to postgres.UserPurger (#2116). It
// hard-deletes a single user's auth / identity rows via the existing
// service-mode registries, mirroring the postgres DELETE-by-user_id sequence.
//
// Unlike the postgres variant these deletes are NOT one transaction — memory
// mode is only used in tests where partial failure is acceptable.
//
// PARITY GAPS (memory backend only): several auth tables have no user-scoped
// delete method on their in-memory registry, so they are skipped here. They are
// fully handled by postgres.UserPurger. If a memory test ever needs to assert
// these are gone, the orchestration layer must add the corresponding
// DeleteByUser-style method to the registry (a shared file, hence not touched
// from this purger). The skipped tables are:
//   - login_events           (only DeleteOlderThan exists)
//   - operation_slots        (only CleanupExpiredSlots / per-slot release)
//   - commodity_scan_audits  (only DeleteOlderThan exists)
//   - settings               (no per-user Delete; Get/Save/Patch only)
//   - group_notification_prefs (only DeleteByGroup / ListByUserGroup; no
//     list-by-user across all groups)
//   - thumbnail_generation_jobs + user_concurrency_slots (purged by group in
//     GroupPurger via the file chain; no user-scoped path)
//
// It does NOT touch the users row itself — the orchestration layer issues the
// final user delete after this returns.
type UserPurger struct {
	refreshTokens registry.RefreshTokenRegistry
	mfaSecrets    registry.UserMFASecretRegistry
	oauth         registry.OAuthIdentityRegistry
	passwordReset registry.PasswordResetRegistry
	emailVerif    registry.EmailVerificationRegistry
	magicLink     registry.MagicLinkTokenRegistry
	memberships   registry.GroupMembershipRegistry
	adminGrants   registry.SystemAdminGrantRegistry
}

// NewUserPurger wires a UserPurger to the registries that own the shared
// in-memory data maps. All parameters are required.
func NewUserPurger(
	refreshTokens registry.RefreshTokenRegistry,
	mfaSecrets registry.UserMFASecretRegistry,
	oauth registry.OAuthIdentityRegistry,
	passwordReset registry.PasswordResetRegistry,
	emailVerif registry.EmailVerificationRegistry,
	magicLink registry.MagicLinkTokenRegistry,
	memberships registry.GroupMembershipRegistry,
	adminGrants registry.SystemAdminGrantRegistry,
) *UserPurger {
	return &UserPurger{
		refreshTokens: refreshTokens,
		mfaSecrets:    mfaSecrets,
		oauth:         oauth,
		passwordReset: passwordReset,
		emailVerif:    emailVerif,
		magicLink:     magicLink,
		memberships:   memberships,
		adminGrants:   adminGrants,
	}
}

// PurgeUserDependents hard-deletes the user's auth/identity rows. Idempotent:
// a second call after a clean purge is a no-op (every step matches zero rows).
//
// PRECONDITION (matching postgres): the user must no longer OWN shared content
// — those rows carry NOT NULL user_id + NOT NULL created_by columns that
// cannot be orphaned, so this purger deliberately does not touch them.
func (r *UserPurger) PurgeUserDependents(ctx context.Context, tenantID, userID string) error {
	if tenantID == "" {
		return errxtrace.Wrap("tenantID required", registry.ErrFieldRequired)
	}
	if userID == "" {
		return errxtrace.Wrap("userID required", registry.ErrFieldRequired)
	}

	type step struct {
		name string
		run  func() error
	}
	steps := []step{
		{"refresh_tokens", func() error { return r.purgeRefreshTokens(ctx, userID) }},
		{"email_verifications", func() error { return r.purgeEmailVerifications(ctx, userID) }},
		{"password_resets", func() error { return r.passwordReset.DeleteByUserID(ctx, userID) }},
		{"magic_link_tokens", func() error { return r.magicLink.DeleteByUserID(ctx, userID) }},
		{"user_mfa_secrets", func() error { return r.mfaSecrets.DeleteByUser(ctx, tenantID, userID) }},
		{"user_oauth_identities", func() error { return r.purgeOAuthIdentities(ctx, tenantID, userID) }},
		{"group_memberships", func() error { return r.purgeMemberships(ctx, tenantID, userID) }},
		// System-admin grant the user HOLDS. RevokeAtomic(allowZero=true)
		// removes it idempotently. The granted_by back-ref is nulled only on
		// the postgres side; the memory grant registry has no granted_by index
		// and stores it as an unread value field, so no memory action is needed
		// for the SET NULL case.
		{"system_admin_grants", func() error {
			_, err := r.adminGrants.RevokeAtomic(ctx, userID, true)
			return err
		}},
	}
	for _, s := range steps {
		if err := s.run(); err != nil {
			return errxtrace.Wrap("failed to purge "+s.name, err)
		}
	}
	return nil
}

// purgeRefreshTokens hard-deletes every refresh token the user owns. The
// service interface only exposes RevokeByUserID (a soft revoke), so we list +
// Delete(id) for a true purge mirroring the postgres DELETE FROM refresh_tokens.
func (r *UserPurger) purgeRefreshTokens(ctx context.Context, userID string) error {
	tokens, err := r.refreshTokens.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for _, t := range tokens {
		if t == nil {
			continue
		}
		if err := r.refreshTokens.Delete(ctx, t.GetID()); err != nil {
			return err
		}
	}
	return nil
}

// purgeEmailVerifications lists the user's email-verification rows and deletes
// each by id (no dedicated DeleteByUserID on the memory registry).
func (r *UserPurger) purgeEmailVerifications(ctx context.Context, userID string) error {
	rows, err := r.emailVerif.GetByUserID(ctx, userID)
	if err != nil {
		return err
	}
	for _, ev := range rows {
		if ev == nil {
			continue
		}
		if err := r.emailVerif.Delete(ctx, ev.GetID()); err != nil {
			return err
		}
	}
	return nil
}

// purgeOAuthIdentities deletes each provider link the user holds.
func (r *UserPurger) purgeOAuthIdentities(ctx context.Context, tenantID, userID string) error {
	ids, err := r.oauth.ListByUser(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if id == nil {
			continue
		}
		if err := r.oauth.DeleteByUserAndProvider(ctx, tenantID, userID, id.Provider); err != nil {
			return err
		}
	}
	return nil
}

// purgeMemberships deletes every membership where the user is the MEMBER.
// ListByUser filters by member_user_id; Delete(id) bypasses the last-owner
// invariant on purpose — a hard user purge isn't subject to the interactive
// "can't remove last owner" guard; the orchestration layer transfers ownership
// first.
func (r *UserPurger) purgeMemberships(ctx context.Context, tenantID, userID string) error {
	memberships, err := r.memberships.ListByUser(ctx, tenantID, userID)
	if err != nil {
		return err
	}
	for _, m := range memberships {
		if m == nil {
			continue
		}
		if err := r.memberships.Delete(ctx, m.GetID()); err != nil {
			return err
		}
	}
	return nil
}
