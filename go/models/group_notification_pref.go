package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*GroupNotificationPref)(nil)
	_ validation.ValidatableWithContext = (*GroupNotificationPref)(nil)
	_ IDable                            = (*GroupNotificationPref)(nil)
)

// Enable RLS for multi-tenant isolation. User-level filtering happens
// in application logic (the REST endpoint scopes by the auth'd user;
// the warranty worker bypasses via the background-worker role).
//
//migrator:schema:rls:enable table="group_notification_prefs" comment="Enable RLS for multi-tenant per-group notification prefs isolation"
//migrator:schema:rls:policy name="group_notification_prefs_tenant_isolation" table="group_notification_prefs" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" comment="Ensures per-group notification prefs are isolated by tenant; user-level filtering happens in application logic"
//migrator:schema:rls:policy name="group_notification_prefs_background_worker_access" table="group_notification_prefs" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to read per-group prefs when deciding whether to enqueue a reminder"

// GroupNotificationPref is a user's per-group opt-in / opt-out for a
// single notification category (issue #1648). A row OVERRIDES the
// user-global pref from #1373; absence falls through to the user's
// global setting and then the in-code default. The (user_id, group_id,
// category) triple is the upsert key — see the schema's unique index.
//
//migrator:schema:table name="group_notification_prefs"
type GroupNotificationPref struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// GroupID references the location group this pref applies to.
	//migrator:schema:field name="group_id" type="TEXT" not_null="true" foreign="location_groups(id)" foreign_key_name="fk_group_notif_pref_group"
	GroupID string `json:"group_id" db:"group_id"`

	// UserID references the user the pref belongs to. The opt-in/out
	// is intentionally per-user (not per-group-globally) so two admins
	// in the same group can pick their own delivery preferences.
	//migrator:schema:field name="user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_group_notif_pref_user"
	UserID string `json:"user_id" db:"user_id"`

	// Category is the notification kind this row toggles. Stored as
	// free-form TEXT (no DB CHECK) so the enum can evolve in Go
	// without a schema migration each time — see
	// `notifications.Category` for the live allowlist.
	//migrator:schema:field name="category" type="TEXT" not_null="true"
	Category string `json:"category" db:"category"`

	// Enabled is the explicit on/off. Distinct from "row missing" —
	// the row's presence is what flips the per-group override on;
	// absence falls through to the user-global pref.
	//migrator:schema:field name="enabled" type="BOOLEAN" not_null="true"
	Enabled bool `json:"enabled" db:"enabled"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// GroupNotificationPrefIndexes defines PostgreSQL indexes.
type GroupNotificationPrefIndexes struct {
	// Unique index for the immutable UUID (deduplication key for
	// import/restore).
	//migrator:schema:index name="idx_group_notification_prefs_uuid" fields="uuid" unique="true" table="group_notification_prefs"
	_ int

	// Upsert key: one row per (user, group, category).
	//migrator:schema:index name="idx_group_notification_prefs_unique" fields="tenant_id,group_id,user_id,category" unique="true" table="group_notification_prefs"
	_ int

	// "All prefs for this user inside this group" — the GET endpoint
	// hits this on every settings-page open.
	//migrator:schema:index name="idx_group_notification_prefs_user_group" fields="user_id,group_id" table="group_notification_prefs"
	_ int

	//migrator:schema:index name="idx_group_notification_prefs_tenant_id" fields="tenant_id" table="group_notification_prefs"
	_ int
}

func (*GroupNotificationPref) Validate() error {
	return ErrMustUseValidateWithContext
}

func (p *GroupNotificationPref) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, p,
		validation.Field(&p.TenantID, rules.NotEmpty),
		validation.Field(&p.GroupID, rules.NotEmpty),
		validation.Field(&p.UserID, rules.NotEmpty),
		validation.Field(&p.Category, rules.NotEmpty),
	)
}
