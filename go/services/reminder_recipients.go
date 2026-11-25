package services

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"go.5x5.cz/inventario/models"
	"go.5x5.cz/inventario/registry"
)

// lookupReminderUser resolves a reminder recipient. It returns (nil, nil)
// when the user no longer exists, which is a lasting state the reminder
// workers skip. Any other error is returned so the caller fails the item
// for this sweep and retries it on the next one: treating a transient
// lookup failure as "no such user" would write the idempotency row
// without that recipient, and they would never get the reminder.
func lookupReminderUser(ctx context.Context, users registry.UserRegistry, userID string) (*models.User, error) {
	user, err := users.Get(ctx, userID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return user, nil
}

// reminderRecipientUsers returns the users a warranty or maintenance
// reminder for c goes to: the group's admins with an email address, one
// per address, or the commodity's creator when the group has none.
// fromMembership is true for the admins; the workers cache that list for
// the group for the rest of the sweep. A member or creator whose user is
// gone is skipped, and any other lookup error is returned (see
// lookupReminderUser): a list missing a recipient would be cached and
// written into the idempotency rows.
func reminderRecipientUsers(ctx context.Context, fs *registry.FactorySet, c *models.Commodity) (users []*models.User, fromMembership bool, err error) {
	if fs.GroupMembershipRegistry != nil {
		members, err := fs.GroupMembershipRegistry.ListByGroup(ctx, c.GroupID)
		if err != nil {
			return nil, false, err
		}
		admins, err := groupAdminUsers(ctx, fs.UserRegistry, c.GroupID, members)
		if err != nil {
			return nil, false, err
		}
		if len(admins) > 0 {
			return admins, true, nil
		}
	}
	if c.CreatedByUserID == "" {
		return nil, false, nil
	}
	creator, err := lookupReminderUser(ctx, fs.UserRegistry, c.CreatedByUserID)
	if err != nil {
		return nil, false, err
	}
	if creator == nil || strings.TrimSpace(creator.Email) == "" {
		return nil, false, nil
	}
	return []*models.User{creator}, false, nil
}

// groupAdminUsers resolves the admin memberships to users with an email
// address, one per address. Admins get the reminder emails because they
// can act on a warranty or a maintenance task; other members see the date
// in the app.
func groupAdminUsers(ctx context.Context, users registry.UserRegistry, groupID string, members []*models.GroupMembership) ([]*models.User, error) {
	out := make([]*models.User, 0, 4)
	seen := make(map[string]struct{}, 4)
	for _, m := range members {
		if m == nil || m.Role != models.GroupRoleAdmin {
			continue
		}
		user, err := lookupReminderUser(ctx, users, m.MemberUserID)
		if err != nil {
			return nil, err
		}
		if user == nil {
			slog.Warn("reminder: skip member with missing user",
				"user_id", m.MemberUserID,
				"group_id", groupID,
			)
			continue
		}
		if strings.TrimSpace(user.Email) == "" {
			continue
		}
		if _, ok := seen[user.Email]; ok {
			continue
		}
		seen[user.Email] = struct{}{}
		out = append(out, user)
	}
	return out, nil
}
