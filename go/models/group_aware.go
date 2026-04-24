package models

import (
	"context"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// GroupAware is implemented by entities that belong to a location group.
type GroupAware interface {
	GetGroupID() string
	SetGroupID(string)
}

// CreatedByUserAware is implemented by entities that track who created them.
// This replaces UserAware for data tables where user_id is renamed to
// created_by_user_id (the user no longer controls RLS isolation — the group does).
type CreatedByUserAware interface {
	GetCreatedByUserID() string
	SetCreatedByUserID(string)
}

// TenantGroupAware combines tenant and group awareness for data entities
// that are isolated by both tenant and group.
type TenantGroupAware interface {
	TenantAware
	GroupAware
	CreatedByUserAware
}

// TenantGroupAwareIDable adds IDable to TenantGroupAware.
type TenantGroupAwareIDable interface {
	IDable
	TenantGroupAware
}

var (
	_ TenantGroupAwareIDable = (*TenantGroupAwareEntityID)(nil)
	_ validation.Validatable = (*TenantGroupAwareEntityID)(nil)
)

// TenantGroupAwareEntityID is the base struct for entities scoped to
// tenant + group (full data isolation). Used by: locations, areas,
// commodities, files, exports.
//
// created_by_user_id tracks who created the record (audit only, not access control).
// Access control is enforced via group_id in RLS policies.
type TenantGroupAwareEntityID struct {
	//migrator:embedded mode="inline"
	EntityID
	//migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_entity_tenant"
	TenantID string `json:"-" db:"tenant_id" userinput:"false"`
	//migrator:schema:field name="group_id" type="TEXT" not_null="true" foreign="location_groups(id)" foreign_key_name="fk_entity_group"
	GroupID string `json:"-" db:"group_id" userinput:"false"`
	//migrator:schema:field name="created_by_user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_entity_created_by"
	CreatedByUserID string `json:"-" db:"created_by_user_id" userinput:"false"`
}

func (i *TenantGroupAwareEntityID) GetTenantID() string {
	return i.TenantID
}

func (i *TenantGroupAwareEntityID) SetTenantID(tenantID string) {
	i.TenantID = tenantID
}

func (i *TenantGroupAwareEntityID) GetGroupID() string {
	return i.GroupID
}

func (i *TenantGroupAwareEntityID) SetGroupID(groupID string) {
	i.GroupID = groupID
}

func (i *TenantGroupAwareEntityID) GetCreatedByUserID() string {
	return i.CreatedByUserID
}

func (i *TenantGroupAwareEntityID) SetCreatedByUserID(userID string) {
	i.CreatedByUserID = userID
}

func (*TenantGroupAwareEntityID) Validate() error {
	return ErrMustUseValidateWithContext
}

func (i *TenantGroupAwareEntityID) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, i,
		validation.Field(&i.TenantID, rules.NotEmpty),
		validation.Field(&i.GroupID, rules.NotEmpty),
		validation.Field(&i.CreatedByUserID, rules.NotEmpty),
	)
}

// WithGroupID sets the group ID on a GroupAware entity and returns it.
func WithGroupID[T GroupAware](groupID string, i T) T {
	i.SetGroupID(groupID)
	return i
}

// WithCreatedByUserID sets the created-by user ID on a CreatedByUserAware entity and returns it.
func WithCreatedByUserID[T CreatedByUserAware](userID string, i T) T {
	i.SetCreatedByUserID(userID)
	return i
}

// WithTenantGroupAwareEntityID creates a TenantGroupAwareEntityID with the given IDs.
func WithTenantGroupAwareEntityID(id, tenantID, groupID, createdByUserID string) TenantGroupAwareEntityID {
	return TenantGroupAwareEntityID{
		EntityID:        EntityID{ID: id},
		TenantID:        tenantID,
		GroupID:         groupID,
		CreatedByUserID: createdByUserID,
	}
}
