package memory

import (
	"context"
	"time"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.GroupInviteRegistry = (*GroupInviteRegistry)(nil)

type baseGroupInviteRegistry = Registry[models.GroupInvite, *models.GroupInvite]

type GroupInviteRegistry struct {
	*baseGroupInviteRegistry
}

func NewGroupInviteRegistry() *GroupInviteRegistry {
	return &GroupInviteRegistry{
		baseGroupInviteRegistry: NewRegistry[models.GroupInvite, *models.GroupInvite](),
	}
}

func (r *GroupInviteRegistry) GetByToken(_ context.Context, token string) (*models.GroupInvite, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		invite := pair.Value
		if invite.Token == token {
			v := *invite
			return &v, nil
		}
	}

	return nil, registry.ErrNotFound
}

func (r *GroupInviteRegistry) ListActiveByGroup(_ context.Context, groupID string) ([]*models.GroupInvite, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	now := time.Now()
	var invites []*models.GroupInvite

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		invite := pair.Value
		if invite.GroupID == groupID && invite.UsedBy == nil && invite.ExpiresAt.After(now) {
			v := *invite
			invites = append(invites, &v)
		}
	}

	return invites, nil
}

// ListUsedByGroup returns every accepted (used) invite for a group. Mirrors
// the service-mode postgres counterpart used by GroupPurgeService to build
// the audit snapshot without scanning the whole table.
func (r *GroupInviteRegistry) ListUsedByGroup(_ context.Context, groupID string) ([]*models.GroupInvite, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var invites []*models.GroupInvite
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		invite := pair.Value
		if invite.GroupID == groupID && invite.UsedBy != nil {
			v := *invite
			invites = append(invites, &v)
		}
	}
	return invites, nil
}

// MarkUsed atomically flips an invite from unused to used-by-userID.
// The underlying Registry lock serializes the read-modify-write so two
// concurrent callers cannot both observe UsedBy == nil and both succeed.
func (r *GroupInviteRegistry) MarkUsed(_ context.Context, inviteID, userID string, usedAt time.Time) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	invite, ok := r.items.Get(inviteID)
	if !ok {
		return false, registry.ErrNotFound
	}
	if invite.UsedBy != nil {
		return false, nil
	}
	invite.UsedBy = &userID
	invite.UsedAt = &usedAt
	r.items.Set(inviteID, invite)
	return true, nil
}

// DeleteByGroup removes every invite belonging to the given group and
// returns the count of removed rows.
func (r *GroupInviteRegistry) DeleteByGroup(_ context.Context, groupID string) (int, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	var victims []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value.GroupID == groupID {
			victims = append(victims, pair.Key)
		}
	}
	for _, id := range victims {
		r.items.Delete(id)
	}
	return len(victims), nil
}

// DeleteExpiredUnused removes unused invites whose ExpiresAt is strictly
// before cutoff. Used invites are never touched here — they are either
// snapshotted into the audit table by the purge worker or retained as-is.
func (r *GroupInviteRegistry) DeleteExpiredUnused(_ context.Context, cutoff time.Time) (int, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	var victims []string
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		invite := pair.Value
		if invite.UsedBy == nil && invite.ExpiresAt.Before(cutoff) {
			victims = append(victims, pair.Key)
		}
	}
	for _, id := range victims {
		r.items.Delete(id)
	}
	return len(victims), nil
}
