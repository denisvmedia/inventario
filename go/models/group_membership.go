package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*GroupRole)(nil)
	_ validation.Validatable            = (*GroupMembership)(nil)
	_ validation.ValidatableWithContext = (*GroupMembership)(nil)
	_ TenantAwareIDable                 = (*GroupMembership)(nil)
)

// GroupRole represents a user's role within a location group.
type GroupRole string

const (
	GroupRoleAdmin GroupRole = "admin"
	GroupRoleUser  GroupRole = "user"
)

// Validate implements the validation.Validatable interface for GroupRole.
func (r GroupRole) Validate() error {
	switch r {
	case GroupRoleAdmin, GroupRoleUser:
		return nil
	default:
		return validation.NewError("validation_invalid_group_role", "must be one of: admin, user")
	}
}

// Enable RLS for multi-tenant isolation (tenant-only; membership queries are filtered in application logic)
//
//migrator:schema:rls:enable table="group_memberships" comment="Enable RLS for multi-tenant group membership isolation"
//migrator:schema:rls:policy name="group_membership_tenant_isolation" table="group_memberships" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" comment="Ensures group memberships are isolated by tenant; user-level filtering happens in application logic"
//migrator:schema:rls:policy name="group_membership_background_worker_access" table="group_memberships" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all group memberships for processing"

//migrator:schema:table name="group_memberships"
type GroupMembership struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// GroupID references the location group this membership belongs to.
	//migrator:schema:field name="group_id" type="TEXT" not_null="true" foreign="location_groups(id)" foreign_key_name="fk_membership_group"
	GroupID string `json:"group_id" db:"group_id"`

	// MemberUserID references the user who is a member of the group.
	// Named MemberUserID (not UserID) to avoid collision with TenantAwareEntityID.UserID.
	//migrator:schema:field name="member_user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_membership_user"
	MemberUserID string `json:"member_user_id" db:"member_user_id"`

	// Role is the user's role within this group (admin or user).
	//migrator:schema:field name="role" type="TEXT" not_null="true" default="user"
	Role GroupRole `json:"role" db:"role"`

	// JoinedAt is when the user joined the group.
	//migrator:schema:field name="joined_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	JoinedAt time.Time `json:"joined_at" db:"joined_at" userinput:"false"`
}

// GroupMembershipIndexes defines PostgreSQL indexes for the group_memberships table.
type GroupMembershipIndexes struct {
	// Unique index for the immutable UUID
	//migrator:schema:index name="idx_group_memberships_uuid" fields="uuid" unique="true" table="group_memberships"
	_ int

	// Unique index ensuring a user can only have one membership per group within a tenant
	//migrator:schema:index name="idx_group_memberships_unique" fields="tenant_id,group_id,member_user_id" unique="true" table="group_memberships"
	_ int

	// Index for listing members of a group
	//migrator:schema:index name="idx_group_memberships_group_id" fields="group_id" table="group_memberships"
	_ int

	// Index for listing groups a user belongs to
	//migrator:schema:index name="idx_group_memberships_member_user_id" fields="member_user_id" table="group_memberships"
	_ int

	// Index for tenant-based queries
	//migrator:schema:index name="idx_group_memberships_tenant_id" fields="tenant_id" table="group_memberships"
	_ int
}

func (*GroupMembership) Validate() error {
	return ErrMustUseValidateWithContext
}

func (gm *GroupMembership) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&gm.TenantID, rules.NotEmpty),
		validation.Field(&gm.GroupID, rules.NotEmpty),
		validation.Field(&gm.MemberUserID, rules.NotEmpty),
		validation.Field(&gm.Role, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, gm, fields...)
}

// IsAdmin returns true if this membership has the admin role.
func (gm *GroupMembership) IsAdmin() bool {
	return gm.Role == GroupRoleAdmin
}
