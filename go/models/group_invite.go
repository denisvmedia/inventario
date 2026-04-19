package models

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*GroupInvite)(nil)
	_ validation.ValidatableWithContext = (*GroupInvite)(nil)
	_ TenantAwareIDable                 = (*GroupInvite)(nil)
)

// DefaultInviteExpiry is the default duration before an invite link expires.
const DefaultInviteExpiry = 24 * time.Hour

// Enable RLS for multi-tenant isolation (tenant-only)
//
//migrator:schema:rls:enable table="group_invites" comment="Enable RLS for multi-tenant group invite isolation"
//migrator:schema:rls:policy name="group_invite_tenant_isolation" table="group_invites" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" comment="Ensures group invites are isolated by tenant"
//migrator:schema:rls:policy name="group_invite_background_worker_access" table="group_invites" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all group invites for cleanup"

//migrator:schema:table name="group_invites"
type GroupInvite struct {
	//migrator:embedded mode="inline"
	TenantOnlyEntityID

	// GroupID references the group this invite is for.
	//migrator:schema:field name="group_id" type="TEXT" not_null="true" foreign="location_groups(id)" foreign_key_name="fk_invite_group"
	GroupID string `json:"group_id" db:"group_id"`

	// Token is a cryptographically random, URL-safe string used in the invite link.
	//migrator:schema:field name="token" type="TEXT" not_null="true"
	Token string `json:"token" db:"token" userinput:"false"`

	// CreatedBy is the user ID of the admin who generated the invite.
	//migrator:schema:field name="created_by" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_invite_created_by"
	CreatedBy string `json:"created_by" db:"created_by" userinput:"false"`

	// ExpiresAt is when the invite link becomes invalid.
	//migrator:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`

	// UsedBy is the user ID of the person who accepted the invite (null if unused).
	//migrator:schema:field name="used_by" type="TEXT" foreign="users(id)" foreign_key_name="fk_invite_used_by"
	UsedBy *string `json:"used_by" db:"used_by"`

	// UsedAt is when the invite was accepted (null if unused).
	//migrator:schema:field name="used_at" type="TIMESTAMP"
	UsedAt *time.Time `json:"used_at" db:"used_at"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
}

// GroupInviteIndexes defines PostgreSQL indexes for the group_invites table.
type GroupInviteIndexes struct {
	// Unique index for the immutable UUID
	//migrator:schema:index name="idx_group_invites_uuid" fields="uuid" unique="true" table="group_invites"
	_ int

	// Unique index for token lookups
	//migrator:schema:index name="idx_group_invites_token" fields="token" unique="true" table="group_invites"
	_ int

	// Index for listing invites by group
	//migrator:schema:index name="idx_group_invites_group_id" fields="group_id" table="group_invites"
	_ int

	// Index for tenant-based queries
	//migrator:schema:index name="idx_group_invites_tenant_id" fields="tenant_id" table="group_invites"
	_ int

	// Index for expiry-based cleanup
	//migrator:schema:index name="idx_group_invites_expires_at" fields="expires_at" table="group_invites"
	_ int
}

func (*GroupInvite) Validate() error {
	return ErrMustUseValidateWithContext
}

func (gi *GroupInvite) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&gi.TenantID, rules.NotEmpty),
		validation.Field(&gi.GroupID, rules.NotEmpty),
		validation.Field(&gi.Token, rules.NotEmpty),
		validation.Field(&gi.CreatedBy, rules.NotEmpty),
		validation.Field(&gi.ExpiresAt,
			validation.Required,
			// Reject invites that would be born already expired — both a
			// zero time and a past time fail here. Use Min with the current
			// time rather than a bespoke rule so the error matches the rest
			// of the validation surface.
			validation.Min(time.Now()).Error("expires_at must be in the future"),
		),
	)

	return validation.ValidateStructWithContext(ctx, gi, fields...)
}

// IsExpired returns true if the invite has passed its expiration time.
func (gi *GroupInvite) IsExpired() bool {
	return time.Now().After(gi.ExpiresAt)
}

// IsUsed returns true if the invite has been accepted by someone.
func (gi *GroupInvite) IsUsed() bool {
	return gi.UsedBy != nil
}

// IsValid returns true if the invite can still be accepted
// (not expired and not already used).
func (gi *GroupInvite) IsValid() bool {
	return !gi.IsExpired() && !gi.IsUsed()
}

// GenerateInviteToken creates a cryptographically random, URL-safe token
// (base64url-encoded 32 random bytes = 43 chars).
func GenerateInviteToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
