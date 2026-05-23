package models

import "time"

// SystemAdminGrant records the dedicated row that confers platform-wide
// system-admin privilege on a user (#1784). Splitting the privilege out
// of `users.is_system_admin` into a separate table makes it physically
// impossible to escalate via a mutation on the users row — the JWT/REST
// surfaces have no write path to this table; only the CLI does.
//
// The table is NOT tenant-scoped. System-admin is a platform privilege
// orthogonal to tenants and lives outside the tenant data plane (same
// posture as `audit_logs`). No RLS policy applies.
//
//migrator:schema:table name="system_admin_grants"
type SystemAdminGrant struct {
	//migrator:embedded mode="inline"
	EntityID

	// UserID points at the user who holds the grant. ON DELETE CASCADE
	// makes the grant evaporate when the user is hard-deleted — a stale
	// row with a dangling FK would be a confusing forensic artifact.
	//migrator:schema:field name="user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_system_admin_grants_user" on_delete="CASCADE"
	UserID string `json:"user_id" db:"user_id"`

	// GrantedBy is the operator-of-record who issued the grant. NULL on
	// CLI bootstrap (no authenticated session). ON DELETE SET NULL so
	// hard-deleting an operator doesn't cascade-blow-away other
	// operators' grant rows.
	//migrator:schema:field name="granted_by" type="TEXT" foreign="users(id)" foreign_key_name="fk_system_admin_grants_granted_by" on_delete="SET NULL"
	GrantedBy *string `json:"granted_by,omitempty" db:"granted_by"`

	// GrantedAt is the wall-clock time at which the grant was created.
	// The single timestamp on the row — system-admin grants are
	// effectively immutable once issued (revocation deletes the row),
	// so created_at/updated_at would carry no extra signal.
	//migrator:schema:field name="granted_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	GrantedAt time.Time `json:"granted_at" db:"granted_at"`
}

// SystemAdminGrantIndexes defines the PostgreSQL indexes for the
// system_admin_grants table.
type SystemAdminGrantIndexes struct {
	// Unique index for the immutable UUID (deduplication key for
	// import/restore — mirrors the convention every other entity uses).
	//migrator:schema:index name="idx_system_admin_grants_uuid" fields="uuid" unique="true" table="system_admin_grants"
	_ int

	// Unique index on user_id: each user has at most one grant. Backs
	// the hot-path Exists() lookup that RequireSystemAdmin runs on every
	// /api/v1/admin/* request, and prevents duplicate grants under a
	// race between two concurrent CLI grant calls (the unique violation
	// is what Grant() reads as "row already exists, return idempotent").
	//migrator:schema:index name="system_admin_grants_user_id_idx" fields="user_id" unique="true" table="system_admin_grants"
	_ int
}
