package postgres

import (
	"context"
	"errors"

	"github.com/go-extras/errx"
	errxtrace "github.com/go-extras/errx/stacktrace"
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/postgres/store"
)

var _ registry.GroupMembershipRegistry = (*GroupMembershipRegistry)(nil)

type GroupMembershipRegistry struct {
	dbx        *sqlx.DB
	tableNames store.TableNames
}

func NewGroupMembershipRegistry(dbx *sqlx.DB) *GroupMembershipRegistry {
	return &GroupMembershipRegistry{
		dbx:        dbx,
		tableNames: store.DefaultTableNames,
	}
}

func (r *GroupMembershipRegistry) newSQLRegistry() *store.NonRLSRepository[models.GroupMembership, *models.GroupMembership] {
	return store.NewSQLRegistry[models.GroupMembership, *models.GroupMembership](r.dbx, r.tableNames.GroupMemberships())
}

func (r *GroupMembershipRegistry) Get(ctx context.Context, id string) (*models.GroupMembership, error) {
	if id == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	var membership models.GroupMembership
	reg := r.newSQLRegistry()

	err := reg.ScanOneByField(ctx, store.Pair("id", id), &membership)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
				"entity_type", "GroupMembership",
				"entity_id", id,
			))
		}
		return nil, errxtrace.Wrap("failed to get group membership", err)
	}

	return &membership, nil
}

func (r *GroupMembershipRegistry) List(ctx context.Context) ([]*models.GroupMembership, error) {
	var memberships []*models.GroupMembership

	reg := r.newSQLRegistry()

	for membership, err := range reg.Scan(ctx) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list group memberships", err)
		}
		memberships = append(memberships, &membership)
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) Count(ctx context.Context) (int, error) {
	reg := r.newSQLRegistry()

	count, err := reg.Count(ctx)
	if err != nil {
		return 0, errxtrace.Wrap("failed to count group memberships", err)
	}

	return count, nil
}

func (r *GroupMembershipRegistry) Create(ctx context.Context, membership models.GroupMembership) (*models.GroupMembership, error) {
	if membership.GroupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	if membership.MemberUserID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "MemberUserID"))
	}

	if membership.TenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	reg := r.newSQLRegistry()

	createdMembership, err := reg.Create(ctx, membership, func(ctx context.Context, tx *sqlx.Tx) error {
		// Check for duplicate membership
		txReg := store.NewTxRegistry[models.GroupMembership](tx, r.tableNames.GroupMemberships())
		for m, scanErr := range txReg.ScanByField(ctx, store.Pair("group_id", membership.GroupID)) {
			if scanErr != nil {
				return errxtrace.Wrap("failed to check for existing membership", scanErr)
			}
			if m.MemberUserID == membership.MemberUserID {
				return errxtrace.Classify(registry.ErrAlreadyExists, errx.Attrs(
					"group_id", membership.GroupID,
					"member_user_id", membership.MemberUserID,
				))
			}
		}
		return nil
	})
	if err != nil {
		return nil, errxtrace.Wrap("failed to create group membership", err)
	}

	return &createdMembership, nil
}

func (r *GroupMembershipRegistry) Update(ctx context.Context, membership models.GroupMembership) (*models.GroupMembership, error) {
	if membership.GetID() == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Update(ctx, membership, nil)
	if err != nil {
		return nil, errxtrace.Wrap("failed to update group membership", err)
	}

	return &membership, nil
}

func (r *GroupMembershipRegistry) Delete(ctx context.Context, id string) error {
	if id == "" {
		return errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "ID"))
	}

	reg := r.newSQLRegistry()

	err := reg.Delete(ctx, id, nil)
	if err != nil {
		return errxtrace.Wrap("failed to delete group membership", err)
	}

	return nil
}

func (r *GroupMembershipRegistry) GetByGroupAndUser(ctx context.Context, groupID, userID string) (*models.GroupMembership, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to get membership by group and user", err)
		}
		if membership.MemberUserID == userID {
			return &membership, nil
		}
	}

	return nil, errxtrace.Classify(registry.ErrNotFound, errx.Attrs(
		"entity_type", "GroupMembership",
		"group_id", groupID,
		"user_id", userID,
	))
}

func (r *GroupMembershipRegistry) ListByGroup(ctx context.Context, groupID string) ([]*models.GroupMembership, error) {
	if groupID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	var memberships []*models.GroupMembership
	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list memberships by group", err)
		}
		memberships = append(memberships, &membership)
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) ListByUser(ctx context.Context, tenantID, userID string) ([]*models.GroupMembership, error) {
	if tenantID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "TenantID"))
	}

	if userID == "" {
		return nil, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "UserID"))
	}

	var memberships []*models.GroupMembership
	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("member_user_id", userID)) {
		if err != nil {
			return nil, errxtrace.Wrap("failed to list memberships by user", err)
		}
		if membership.TenantID == tenantID {
			memberships = append(memberships, &membership)
		}
	}

	return memberships, nil
}

func (r *GroupMembershipRegistry) CountAdminsByGroup(ctx context.Context, groupID string) (int, error) {
	if groupID == "" {
		return 0, errxtrace.Classify(registry.ErrFieldRequired, errx.Attrs("field_name", "GroupID"))
	}

	count := 0
	reg := r.newSQLRegistry()

	for membership, err := range reg.ScanByField(ctx, store.Pair("group_id", groupID)) {
		if err != nil {
			return 0, errxtrace.Wrap("failed to count admins by group", err)
		}
		if membership.Role == models.GroupRoleAdmin {
			count++
		}
	}

	return count, nil
}
