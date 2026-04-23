package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*GroupInviteAudit)(nil)
	_ validation.ValidatableWithContext = (*GroupInviteAudit)(nil)
	_ TenantAwareIDable                 = (*GroupInviteAudit)(nil)
)

// Enable RLS for multi-tenant isolation (tenant-only). The audit table keeps
// a persistent snapshot of used invites after the parent group is hard-
// deleted by the purge worker, so it intentionally has no group_id FK.
//
//migrator:schema:rls:enable table="group_invites_audit" comment="Enable RLS for multi-tenant group invite audit isolation"
//migrator:schema:rls:policy name="group_invite_audit_tenant_isolation" table="group_invites_audit" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" comment="Ensures group invite audit records are isolated by tenant"
//migrator:schema:rls:policy name="group_invite_audit_background_worker_access" table="group_invites_audit" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to insert audit rows during group purge"

//migrator:schema:table name="group_invites_audit"
type GroupInviteAudit struct {
	//migrator:embedded mode="inline"
	TenantOnlyEntityID

	// OriginalInviteID preserves the primary key of the source GroupInvite
	// row. No FK — the source row is hard-deleted during purge.
	//migrator:schema:field name="original_invite_id" type="TEXT" not_null="true"
	OriginalInviteID string `json:"original_invite_id" db:"original_invite_id" userinput:"false"`

	// OriginalInviteUUID preserves the immutable UUID of the source invite
	// for cross-export/restore correlation.
	//migrator:schema:field name="original_invite_uuid" type="TEXT" not_null="true"
	OriginalInviteUUID string `json:"original_invite_uuid" db:"original_invite_uuid" userinput:"false"`

	// OriginalGroupID preserves the ID of the purged group. Plain TEXT (no
	// FK): the group row is deleted before/after this audit row is written.
	//migrator:schema:field name="original_group_id" type="TEXT" not_null="true"
	OriginalGroupID string `json:"original_group_id" db:"original_group_id" userinput:"false"`

	// OriginalGroupSlug is the purged group's slug, captured for human
	// readability in audit queries after the group is gone.
	//migrator:schema:field name="original_group_slug" type="TEXT" not_null="true"
	OriginalGroupSlug string `json:"original_group_slug" db:"original_group_slug" userinput:"false"`

	// OriginalGroupName is the purged group's human-readable display name.
	//migrator:schema:field name="original_group_name" type="TEXT" not_null="true"
	OriginalGroupName string `json:"original_group_name" db:"original_group_name" userinput:"false"`

	// Token is the original invite token. Kept for audit queries that cross-
	// reference login records; not unique in the audit table because
	// long-lived snapshots from different lifecycles may collide in theory.
	//migrator:schema:field name="token" type="TEXT" not_null="true"
	Token string `json:"token" db:"token" userinput:"false"`

	// CreatedBy is the admin user who generated the original invite. Kept as
	// a FK: the user row survives group purge. If the user is later deleted,
	// that concern is handled by the users table's own delete path.
	//migrator:schema:field name="created_by" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_invite_audit_created_by"
	CreatedBy string `json:"created_by" db:"created_by" userinput:"false"`

	// UsedBy is the user who accepted the invite. NOT NULL on the audit
	// table — we only snapshot used invites (unused ones are purged outright
	// by the expiry sweep).
	//migrator:schema:field name="used_by" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_invite_audit_used_by"
	UsedBy string `json:"used_by" db:"used_by" userinput:"false"`

	// OriginalCreatedAt is when the invite was originally generated.
	//migrator:schema:field name="original_created_at" type="TIMESTAMP" not_null="true"
	OriginalCreatedAt time.Time `json:"original_created_at" db:"original_created_at" userinput:"false"`

	// OriginalExpiresAt is the original expiry timestamp of the invite.
	//migrator:schema:field name="original_expires_at" type="TIMESTAMP" not_null="true"
	OriginalExpiresAt time.Time `json:"original_expires_at" db:"original_expires_at" userinput:"false"`

	// UsedAt is when the invite was accepted by UsedBy.
	//migrator:schema:field name="used_at" type="TIMESTAMP" not_null="true"
	UsedAt time.Time `json:"used_at" db:"used_at" userinput:"false"`

	// ArchivedAt is when the audit row was written (i.e. when the parent
	// group was purged). Defaults to now at insert time.
	//migrator:schema:field name="archived_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	ArchivedAt time.Time `json:"archived_at" db:"archived_at" userinput:"false"`
}

// GroupInviteAuditIndexes defines PostgreSQL indexes for the
// group_invites_audit table.
type GroupInviteAuditIndexes struct {
	//migrator:schema:index name="idx_group_invites_audit_uuid" fields="uuid" unique="true" table="group_invites_audit"
	_ int

	//migrator:schema:index name="idx_group_invites_audit_tenant_id" fields="tenant_id" table="group_invites_audit"
	_ int

	//migrator:schema:index name="idx_group_invites_audit_original_group_id" fields="original_group_id" table="group_invites_audit"
	_ int

	//migrator:schema:index name="idx_group_invites_audit_used_by" fields="used_by" table="group_invites_audit"
	_ int

	//migrator:schema:index name="idx_group_invites_audit_archived_at" fields="archived_at" table="group_invites_audit"
	_ int
}

func (*GroupInviteAudit) Validate() error {
	return ErrMustUseValidateWithContext
}

func (gia *GroupInviteAudit) ValidateWithContext(ctx context.Context) error {
	fields := []*validation.FieldRules{
		validation.Field(&gia.TenantID, rules.NotEmpty),
		validation.Field(&gia.OriginalInviteID, rules.NotEmpty),
		validation.Field(&gia.OriginalInviteUUID, rules.NotEmpty),
		validation.Field(&gia.OriginalGroupID, rules.NotEmpty),
		validation.Field(&gia.OriginalGroupSlug, rules.NotEmpty),
		validation.Field(&gia.OriginalGroupName, rules.NotEmpty),
		validation.Field(&gia.Token, rules.NotEmpty),
		validation.Field(&gia.CreatedBy, rules.NotEmpty),
		validation.Field(&gia.UsedBy, rules.NotEmpty),
		validation.Field(&gia.OriginalCreatedAt, validation.Required),
		validation.Field(&gia.OriginalExpiresAt, validation.Required),
		validation.Field(&gia.UsedAt, validation.Required),
	}

	return validation.ValidateStructWithContext(ctx, gia, fields...)
}

// NewGroupInviteAuditFromInvite builds an audit snapshot from a used invite
// and the parent group about to be purged. The caller is responsible for
// providing a GroupInvite whose UsedBy/UsedAt fields are populated (unused
// invites are discarded by the expiry sweep, not archived).
func NewGroupInviteAuditFromInvite(invite *GroupInvite, group *LocationGroup) *GroupInviteAudit {
	audit := &GroupInviteAudit{
		TenantOnlyEntityID: TenantOnlyEntityID{
			TenantID: invite.TenantID,
		},
		OriginalInviteID:   invite.ID,
		OriginalInviteUUID: invite.UUID,
		OriginalGroupID:    group.ID,
		OriginalGroupSlug:  group.Slug,
		OriginalGroupName:  group.Name,
		Token:              invite.Token,
		CreatedBy:          invite.CreatedBy,
		OriginalCreatedAt:  invite.CreatedAt,
		OriginalExpiresAt:  invite.ExpiresAt,
	}
	if invite.UsedBy != nil {
		audit.UsedBy = *invite.UsedBy
	}
	if invite.UsedAt != nil {
		audit.UsedAt = *invite.UsedAt
	}
	return audit
}
