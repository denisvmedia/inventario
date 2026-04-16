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
