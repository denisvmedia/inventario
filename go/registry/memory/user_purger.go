package memory

import (
	"context"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

var _ registry.UserPurger = (*UserPurger)(nil)

// UserPurger is the in-memory counterpart to postgres.UserPurger (#2116). It
// hard-deletes a single user's auth / identity rows via the existing
// service-mode registries, mirroring the postgres DELETE-by-user_id sequence.
// Since #2147 it also clears the user's group_invites_audit references
// (created_by / used_by), matching postgres.
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
	apiTokens     *APITokenRegistry
	mfaSecrets    registry.UserMFASecretRegistry
	oauth         registry.OAuthIdentityRegistry
	passwordReset registry.PasswordResetRegistry
	emailVerif    registry.EmailVerificationRegistry
	magicLink     registry.MagicLinkTokenRegistry
	memberships   registry.GroupMembershipRegistry
	adminGrants   registry.SystemAdminGrantRegistry
	inviteAudit   registry.GroupInviteAuditRegistry
	reminders     registry.CommodityReminderRegistryFactory
	reminderSends registry.CommodityReminderSendRegistry
	drafts        registry.CommodityDraftRegistryFactory
	intake        registry.IntakeRegistryFactory
	files         registry.FileRegistryFactory
	savedViews    registry.SavedViewRegistryFactory
	itemTypes     *ItemTypeRegistryFactory
	customFields  *CustomFieldRegistryFactory
	// intakeDocuments is set by SetIntakeDocumentFactory.
	intakeDocuments *IntakeDocumentRegistryFactory
}

// SetIntakeDocumentFactory wires the delivered documents (#2809); the ones
// the user's API tokens created go to another member of their group, as on
// postgres.
func (r *UserPurger) SetIntakeDocumentFactory(docs *IntakeDocumentRegistryFactory) {
	r.intakeDocuments = docs
}

// groupConfiguration is a store of group configuration, whose rows a purged
// user created are handed to another member of their group.
type groupConfiguration interface {
	groupsCreatedBy(tenantID, userID string) []string
	reattributeCreator(tenantID, userID string, heirs map[string]string)
}

// SetItemTypeFactory wires the item type store, whose types the user created
// are handed to another member of their group.
func (r *UserPurger) SetItemTypeFactory(f *ItemTypeRegistryFactory) {
	r.itemTypes = f
}

// SetCustomFieldFactory wires the custom field store, whose fields the user
// created, template entries included, are handed to another member of their
// group.
func (r *UserPurger) SetCustomFieldFactory(f *CustomFieldRegistryFactory) {
	r.customFields = f
}

// NewUserPurger wires a UserPurger to the registries that own the shared
// in-memory data maps. All parameters are required.
func NewUserPurger(
	refreshTokens registry.RefreshTokenRegistry,
	apiTokens *APITokenRegistry,
	mfaSecrets registry.UserMFASecretRegistry,
	oauth registry.OAuthIdentityRegistry,
	passwordReset registry.PasswordResetRegistry,
	emailVerif registry.EmailVerificationRegistry,
	magicLink registry.MagicLinkTokenRegistry,
	memberships registry.GroupMembershipRegistry,
	adminGrants registry.SystemAdminGrantRegistry,
	inviteAudit registry.GroupInviteAuditRegistry,
	reminders registry.CommodityReminderRegistryFactory,
	reminderSends registry.CommodityReminderSendRegistry,
	drafts registry.CommodityDraftRegistryFactory,
	intake registry.IntakeRegistryFactory,
	files registry.FileRegistryFactory,
	savedViews registry.SavedViewRegistryFactory,
) *UserPurger {
	return &UserPurger{
		refreshTokens: refreshTokens,
		apiTokens:     apiTokens,
		mfaSecrets:    mfaSecrets,
		oauth:         oauth,
		passwordReset: passwordReset,
		emailVerif:    emailVerif,
		magicLink:     magicLink,
		memberships:   memberships,
		adminGrants:   adminGrants,
		inviteAudit:   inviteAudit,
		reminders:     reminders,
		reminderSends: reminderSends,
		drafts:        drafts,
		intake:        intake,
		files:         files,
		savedViews:    savedViews,
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
		{"api_tokens", func() error { return r.apiTokens.DeleteByUserID(ctx, userID) }},
		{"email_verifications", func() error { return r.purgeEmailVerifications(ctx, userID) }},
		{"password_resets", func() error { return r.passwordReset.DeleteByUserID(ctx, userID) }},
		{"magic_link_tokens", func() error { return r.magicLink.DeleteByUserID(ctx, userID) }},
		{"user_mfa_secrets", func() error { return r.mfaSecrets.DeleteByUser(ctx, tenantID, userID) }},
		{"user_oauth_identities", func() error { return r.purgeOAuthIdentities(ctx, tenantID, userID) }},
		// Item types and custom fields the user created go to another member
		// of their group, as on postgres; they are group configuration, not
		// the user's content.
		{"item_types", func() error {
			if r.itemTypes == nil {
				return nil
			}
			return r.reattributeGroupConfiguration(ctx, r.itemTypes, tenantID, userID)
		}},
		{"custom_fields", func() error {
			if r.customFields == nil {
				return nil
			}
			return r.reattributeGroupConfiguration(ctx, r.customFields, tenantID, userID)
		}},
		{"intake_documents", func() error {
			if r.intakeDocuments == nil {
				return nil
			}
			return r.reattributeGroupConfiguration(ctx, r.intakeDocuments, tenantID, userID)
		}},
		// The drafts those documents made go with them, so a replay finds
		// the draft the record names.
		{"delivered_drafts", func() error {
			drafts, ok := r.drafts.(*CommodityDraftRegistryFactory)
			if !ok {
				return nil
			}
			return r.reattributeGroupConfiguration(ctx, deliveredDrafts{drafts}, tenantID, userID)
		}},
		{"group_memberships", func() error { return r.purgeMemberships(ctx, tenantID, userID) }},
		// group_invites_audit rows the user created or accepted (both NOT NULL FK
		// -> users(id) NO ACTION on postgres) — #2147. Mirrors the postgres DELETE.
		{"group_invites_audit", func() error { return r.purgeGroupInvitesAudit(ctx, userID) }},
		{"commodity_reminders", func() error { return r.purgeCommodityReminders(ctx, tenantID, userID) }},
		{"commodity_drafts", func() error { return r.purgeCommodityDrafts(ctx, tenantID, userID) }},
		{"intake_sessions", func() error { return r.purgeIntakeSessions(ctx, tenantID, userID) }},
		// Where the user shows the views of others, then the views the user
		// created, shared ones included: both carry a NOT NULL user column,
		// like the postgres DELETEs.
		{"saved_view_placements", func() error { return r.purgeSavedViewPlacements(tenantID, userID) }},
		{"saved_views", func() error { return r.purgeSavedViews(ctx, tenantID, userID) }},
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

// purgeGroupInvitesAudit deletes every group_invites_audit row that names the
// user as the invite creator (CreatedBy) OR the accepter (UsedBy), in any
// tenant: a user moved to another tenant (#2179) leaves this trail in the
// tenant they left. Each match is deleted by id. Mirrors the postgres DELETE so
// a user who ever created or used an invite can be removed without tripping
// the NOT NULL FKs (#2147).
func (r *UserPurger) purgeGroupInvitesAudit(ctx context.Context, userID string) error {
	rows, err := r.inviteAudit.List(ctx)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if row == nil {
			continue
		}
		if row.CreatedBy != userID && row.UsedBy != userID {
			continue
		}
		if err := r.inviteAudit.Delete(ctx, row.GetID()); err != nil {
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

// purgeCommodityDrafts deletes the drafts the user created and the file
// links to them (#2812), the same rows the postgres purger removes.
func (r *UserPurger) purgeCommodityDrafts(ctx context.Context, tenantID, userID string) error {
	if r.drafts == nil {
		return nil
	}
	reg := r.drafts.CreateServiceRegistry()
	all, err := reg.List(ctx)
	if err != nil {
		return err
	}
	for _, draft := range all {
		if draft == nil || draft.TenantID != tenantID || draft.CreatedByUserID != userID {
			continue
		}
		if r.files != nil {
			if _, err := r.files.CreateServiceRegistry().RemoveLinksToEntity(ctx, models.FileLinkDraft, draft.ID); err != nil {
				return err
			}
		}
		if err := reg.Delete(ctx, draft.ID); err != nil {
			return err
		}
	}
	return nil
}

// purgeIntakeSessions deletes the intake sessions the user created (#2810)
// with their sources, candidates and file links, the same rows the postgres
// purger removes.
func (r *UserPurger) purgeIntakeSessions(ctx context.Context, tenantID, userID string) error {
	if r.intake == nil {
		return nil
	}
	reg := r.intake.CreateServiceRegistry()
	all, err := reg.List(ctx)
	if err != nil {
		return err
	}
	for _, session := range all {
		if session == nil || session.TenantID != tenantID || session.CreatedByUserID != userID {
			continue
		}
		if err := reg.Delete(ctx, session.ID); err != nil {
			return err
		}
	}
	return nil
}

// purgeCommodityReminders deletes the item reminders the user created and
// their send records (#2805), the same rows the postgres purger removes: a
// reminder emails only its author, so it leaves with the account.
func (r *UserPurger) purgeCommodityReminders(ctx context.Context, tenantID, userID string) error {
	if r.reminders == nil {
		return nil
	}
	reg := r.reminders.CreateServiceRegistry()
	all, err := reg.List(ctx)
	if err != nil {
		return err
	}
	for _, reminder := range all {
		if reminder == nil || reminder.TenantID != tenantID || reminder.CreatedByUserID != userID {
			continue
		}
		if r.reminderSends != nil {
			if _, err := r.reminderSends.DeleteByReminder(ctx, reminder.ID); err != nil {
				return err
			}
		}
		if err := reg.Delete(ctx, reminder.ID); err != nil {
			return err
		}
	}
	return nil
}

// purgeSavedViewPlacements deletes the placements the user has in the
// tenant. Only the memory factory keeps them, so another implementation of
// the factory has nothing to purge here.
func (r *UserPurger) purgeSavedViewPlacements(tenantID, userID string) error {
	if f, ok := r.savedViews.(*SavedViewRegistryFactory); ok {
		f.purgeUserPlacements(tenantID, userID)
	}
	return nil
}

// purgeSavedViews deletes every saved view the user created in the tenant,
// personal and shared, through the service registry that sees all of them.
func (r *UserPurger) purgeSavedViews(ctx context.Context, tenantID, userID string) error {
	reg := r.savedViews.CreateServiceRegistry()
	views, err := reg.List(ctx)
	if err != nil {
		return err
	}
	for _, v := range views {
		if v.TenantID != tenantID || v.CreatedByUserID != userID {
			continue
		}
		if err := reg.Delete(ctx, v.ID); err != nil {
			return err
		}
	}
	return nil
}

// reattributeGroupConfiguration gives each row of store the user created to
// another member of its group: the owner first, then an admin, then the
// member who joined first. A row whose group has no other member keeps its
// creator.
func (r *UserPurger) reattributeGroupConfiguration(ctx context.Context, store groupConfiguration, tenantID, userID string) error {
	heirs := map[string]string{}
	for _, groupID := range store.groupsCreatedBy(tenantID, userID) {
		members, err := r.memberships.ListByGroup(ctx, groupID)
		if err != nil {
			return err
		}
		if heir := groupConfigurationHeir(members, userID); heir != "" {
			heirs[groupID] = heir
		}
	}
	store.reattributeCreator(tenantID, userID, heirs)
	return nil
}

// groupConfigurationHeir picks the member who takes over the group configuration userID
// created: the owner first, then an admin, then the member who joined first.
func groupConfigurationHeir(members []*models.GroupMembership, userID string) string {
	rank := func(role models.GroupRole) int {
		switch role {
		case models.GroupRoleOwner:
			return 0
		case models.GroupRoleAdmin:
			return 1
		default:
			return 2
		}
	}
	var best *models.GroupMembership
	for _, m := range members {
		if m == nil || m.MemberUserID == userID {
			continue
		}
		if best == nil ||
			rank(m.Role) < rank(best.Role) ||
			(rank(m.Role) == rank(best.Role) && m.JoinedAt.Before(best.JoinedAt)) ||
			(rank(m.Role) == rank(best.Role) && m.JoinedAt.Equal(best.JoinedAt) && m.MemberUserID < best.MemberUserID) {
			best = m
		}
	}
	if best == nil {
		return ""
	}
	return best.MemberUserID
}
