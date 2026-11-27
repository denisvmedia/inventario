package memory

import (
	"context"
	"strings"
	"time"

	errxtrace "github.com/go-extras/errx/stacktrace"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// memberDeparture removes the rows that were personal to a member in a group
// when the member leaves it or is removed, as the postgres membership delete
// does in its transaction (#2892). See
// postgres.GroupMembershipRegistry.removeDepartedMemberRows for what goes
// and why.
type memberDeparture struct {
	savedViews    *SavedViewRegistryFactory
	notifications *GroupNotificationPrefRegistry
	reminders     registry.CommodityReminderRegistryFactory
	reminderSends registry.CommodityReminderSendRegistry
	drafts        registry.CommodityDraftRegistryFactory
	intake        registry.IntakeRegistryFactory
	files         registry.FileRegistryFactory
	invites       *GroupInviteRegistry
	apiTokens     *APITokenRegistry
}

// newMemberDeparture reads the stores off a wired factory set.
func newMemberDeparture(fs *registry.FactorySet, apiTokens *APITokenRegistry) *memberDeparture {
	d := &memberDeparture{
		reminders:     fs.CommodityReminderRegistryFactory,
		reminderSends: fs.CommodityReminderSendRegistry,
		drafts:        fs.CommodityDraftRegistryFactory,
		intake:        fs.IntakeRegistryFactory,
		files:         fs.FileRegistryFactory,
		apiTokens:     apiTokens,
	}
	d.savedViews, _ = fs.SavedViewRegistryFactory.(*SavedViewRegistryFactory)
	d.notifications, _ = fs.GroupNotificationPrefRegistry.(*GroupNotificationPrefRegistry)
	d.invites, _ = fs.GroupInviteRegistry.(*GroupInviteRegistry)
	return d
}

func (d *memberDeparture) run(ctx context.Context, tenantID, groupID, userID string) error {
	if d.savedViews != nil {
		d.savedViews.removeDepartedMember(tenantID, groupID, userID)
	}
	if d.notifications != nil {
		d.notifications.deleteWhere(func(p *models.GroupNotificationPref) bool {
			return p.TenantID == tenantID && p.GroupID == groupID && p.UserID == userID
		})
	}
	steps := []struct {
		what string
		run  func() error
	}{
		{"item reminders", func() error { return d.removeReminders(ctx, tenantID, groupID, userID) }},
		{"drafts", func() error { return d.removeDrafts(ctx, tenantID, groupID, userID) }},
		{"intake sessions", func() error { return d.removeIntakeSessions(ctx, tenantID, groupID, userID) }},
	}
	for _, step := range steps {
		if err := step.run(); err != nil {
			return errxtrace.Wrap("failed to remove the departed member's "+step.what, err)
		}
	}
	if d.invites != nil {
		d.invites.deleteWhere(func(i *models.GroupInvite) bool {
			return i.TenantID == tenantID && i.GroupID == groupID && i.CreatedBy == userID && i.UsedBy == nil
		})
	}
	if d.apiTokens != nil {
		d.apiTokens.revokeForGroup(userID, groupID, time.Now())
	}
	return nil
}

func (d *memberDeparture) removeReminders(ctx context.Context, tenantID, groupID, userID string) error {
	if d.reminders == nil {
		return nil
	}
	reg := d.reminders.CreateServiceRegistry()
	all, err := reg.List(ctx)
	if err != nil {
		return err
	}
	for _, reminder := range all {
		if reminder.TenantID != tenantID || reminder.GroupID != groupID || reminder.CreatedByUserID != userID {
			continue
		}
		if d.reminderSends != nil {
			if _, err := d.reminderSends.DeleteByReminder(ctx, reminder.ID); err != nil {
				return err
			}
		}
		if err := reg.Delete(ctx, reminder.ID); err != nil {
			return err
		}
	}
	return nil
}

// removeDrafts deletes the member's drafts in the group and the file links
// to them. A draft made for a delivered document stays with the group.
func (d *memberDeparture) removeDrafts(ctx context.Context, tenantID, groupID, userID string) error {
	if d.drafts == nil {
		return nil
	}
	reg := d.drafts.CreateServiceRegistry()
	all, err := reg.List(ctx)
	if err != nil {
		return err
	}
	for _, draft := range all {
		if draft.TenantID != tenantID || draft.GroupID != groupID || draft.CreatedByUserID != userID ||
			strings.HasPrefix(draft.ClientKey, models.IntakeDeliveredDraftKeyPrefix) {
			continue
		}
		if d.files != nil {
			if _, err := d.files.CreateServiceRegistry().RemoveLinksToEntity(ctx, models.FileLinkDraft, draft.ID); err != nil {
				return err
			}
		}
		if err := reg.Delete(ctx, draft.ID); err != nil {
			return err
		}
	}
	return nil
}

// removeIntakeSessions deletes the member's Add item sessions in the group;
// the delete takes their sources, candidates and file links with them.
func (d *memberDeparture) removeIntakeSessions(ctx context.Context, tenantID, groupID, userID string) error {
	if d.intake == nil {
		return nil
	}
	reg := d.intake.CreateServiceRegistry()
	all, err := reg.List(ctx)
	if err != nil {
		return err
	}
	for _, session := range all {
		if session.TenantID != tenantID || session.GroupID != groupID || session.CreatedByUserID != userID {
			continue
		}
		if err := reg.Delete(ctx, session.ID); err != nil {
			return err
		}
	}
	return nil
}

// deleteWhere removes the overrides match selects.
func (r *GroupNotificationPrefRegistry) deleteWhere(match func(*models.GroupNotificationPref) bool) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var ids []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if match(pair.Value) {
			ids = append(ids, pair.Key)
		}
	}
	for _, id := range ids {
		r.items.Delete(id)
	}
}

// deleteWhere removes the invites match selects.
func (r *GroupInviteRegistry) deleteWhere(match func(*models.GroupInvite) bool) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var ids []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if match(pair.Value) {
			ids = append(ids, pair.Key)
		}
	}
	for _, id := range ids {
		r.items.Delete(id)
	}
}

// revokeForGroup revokes the tokens of userID limited to groupID that are
// not revoked yet.
func (r *APITokenRegistry) revokeForGroup(userID, groupID string, now time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for id, t := range r.tokens {
		if t.UserID == userID && t.GroupID != nil && *t.GroupID == groupID && t.RevokedAt == nil {
			t.RevokedAt = new(now)
			r.tokens[id] = t
		}
	}
}
