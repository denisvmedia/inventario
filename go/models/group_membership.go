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

// GroupRole represents a user's role within a location group. Roles are
// ranked viewer < user < admin < owner. Use AtLeast for "at-least-as-
// privileged-as" comparisons; ranks are an implementation detail and
// never appear on the wire.
type GroupRole string

const (
	GroupRoleViewer GroupRole = "viewer"
	GroupRoleUser   GroupRole = "user"
	GroupRoleAdmin  GroupRole = "admin"
	GroupRoleOwner  GroupRole = "owner"
)

// groupRoleRank gives each known role a numeric rank for comparison.
// Unknown roles return -1 via rank() and never satisfy AtLeast.
var groupRoleRank = map[GroupRole]int{
	GroupRoleViewer: 0,
	GroupRoleUser:   1,
	GroupRoleAdmin:  2,
	GroupRoleOwner:  3,
}

func (r GroupRole) rank() int {
	v, ok := groupRoleRank[r]
	if !ok {
		return -1
	}
	return v
}

// AtLeast reports whether r is at least as privileged as min. Unknown
// roles on either side return false — caller-side validation already
// rejects them at the handler boundary, so this is the safe default.
func (r GroupRole) AtLeast(min GroupRole) bool {
	have := r.rank()
	want := min.rank()
	if have < 0 || want < 0 {
		return false
	}
	return have >= want
}

// Validate implements the validation.Validatable interface for GroupRole.
func (r GroupRole) Validate() error {
	switch r {
	case GroupRoleViewer, GroupRoleUser, GroupRoleAdmin, GroupRoleOwner:
		return nil
	default:
		return validation.NewError("validation_invalid_group_role", "must be one of: viewer, user, admin, owner")
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
	// Named MemberUserID (not UserID) to make it explicit that this is the
	// *subject* of the membership row — the user being admitted to the
	// group — and not metadata about who created the membership. Use
	// the same helper `WithCreatedByUserID` pattern that other entities
	// use when you need the creator-of-record; it's a separate concept.
	//migrator:schema:field name="member_user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_membership_user"
	MemberUserID string `json:"member_user_id" db:"member_user_id"`

	// Role is the user's role within this group: viewer, user, admin, or owner.
	// Validation lives in Go (GroupRole.Validate) rather than a DB CHECK so
	// the column stays a free-form TEXT and the enum can evolve without a
	// schema migration each time.
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

// IsAdmin returns true if this membership has admin privileges, which
// now means role >= admin (admin or owner). Renamed semantics — the old
// name is kept because plenty of callers test "is this user an admin?",
// and owners are by definition also admins.
func (gm *GroupMembership) IsAdmin() bool {
	return gm.Role.AtLeast(GroupRoleAdmin)
}

// IsOwner returns true only for the owner role. Use this for ownership-
// specific gates like delete-group.
func (gm *GroupMembership) IsOwner() bool {
	return gm.Role == GroupRoleOwner
}

// MembershipWithUser bundles a GroupMembership with the User it
// belongs to, populated by registry-layer joins. The members list
// endpoint ships a single round-trip with the data the UI needs —
// avatar initials, display name, email — instead of the opaque
// member-id hash the BE used to surface.
type MembershipWithUser struct {
	Membership *GroupMembership
	User       *User
}
