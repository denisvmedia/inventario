package memory

import (
	"context"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.GroupMembershipRegistry = (*GroupMembershipRegistry)(nil)

type baseGroupMembershipRegistry = Registry[models.GroupMembership, *models.GroupMembership]

type GroupMembershipRegistry struct {
	*baseGroupMembershipRegistry
}

func NewGroupMembershipRegistry() *GroupMembershipRegistry {
	return &GroupMembershipRegistry{
		baseGroupMembershipRegistry: NewRegistry[models.GroupMembership, *models.GroupMembership](),
	}
}

func (r *GroupMembershipRegistry) GetByGroupAndUser(_ context.Context, groupID, userID string) (*models.GroupMembership, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID && m.MemberUserID == userID {
			v := *m
			return &v, nil
		}
	}

	return nil, registry.ErrNotFound
}

func (r *GroupMembershipRegistry) ListByGroup(_ context.Context, groupID string) ([]*models.GroupMembership, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var memberships []*models.GroupMembership
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID {
			v := *m
			memberships = append(memberships, &v)
		}
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) ListByUser(_ context.Context, tenantID, userID string) ([]*models.GroupMembership, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	var memberships []*models.GroupMembership
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.MemberUserID == userID && m.TenantID == tenantID {
			v := *m
			memberships = append(memberships, &v)
		}
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) CountAdminsByGroup(_ context.Context, groupID string) (int, error) {
	r.lock.RLock()
	defer r.lock.RUnlock()

	count := 0
	for pair := r.items.Oldest(); pair != nil; pair = pair.Next() {
		m := pair.Value
		if m.GroupID == groupID && m.Role == models.GroupRoleAdmin {
			count++
		}
	}

	return count, nil
}
