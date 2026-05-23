package models

import (
	"time"
)

// BackofficeRefreshToken is the long-lived refresh-token row used by the
// back-office auth plane (issue #1785, Phase 2). It mirrors the shape of
// models.RefreshToken but is scoped to a BackofficeUser rather than a
// tenant-bound User. The boundary differences matter:
//
//   - The FK column is `backoffice_user_id`, NOT `user_id`. The two
//     identity universes (back-office, tenant) MUST not cross-pollinate
//     because that would let a tenant user authenticate against the
//     back-office plane (or vice versa) by simply colliding ids.
//
//   - There is NO `tenant_id` column. Back-office identities live OUTSIDE
//     the tenant model, exactly like models.BackofficeUser itself — same
//     reasoning as `tenants`: it IS the boundary. No tenant_id => no RLS
//     policy that depends on `get_current_tenant_id()`; access is gated
//     entirely at the application layer by the back-office auth plane.
//
//   - No RLS is enabled on the underlying table at all. The back-office
//     login flow runs before any user/tenant DB session context is set,
//     so any RLS predicate that reads `get_current_*_id()` would block
//     the very call that needs to look up its own refresh token.
//
// Token storage follows the existing tenant-side convention: the raw
// token is base64-url-encoded random bytes (services.GenerateRefreshToken
// equivalent); only the SHA-256 hash is persisted. The cookie value is
// the raw token; lookups always hash first. This means a DB dump of
// backoffice_refresh_tokens cannot be replayed against the API.

//migrator:schema:table name="backoffice_refresh_tokens"
type BackofficeRefreshToken struct {
	//migrator:embedded mode="inline"
	EntityID
	// BackofficeUserID is the FK to backoffice_users.id. Distinct from
	// User.user_id so a stolen tenant refresh token can never resolve to
	// a back-office identity even if the hash collided.
	//migrator:schema:field name="backoffice_user_id" type="TEXT" not_null="true" foreign="backoffice_users(id)" foreign_key_name="fk_backoffice_refresh_token_user"
	BackofficeUserID string `json:"-" db:"backoffice_user_id" userinput:"false"`
	//migrator:schema:field name="token_hash" type="VARCHAR(128)" not_null="true"
	TokenHash string `json:"-" db:"token_hash"`
	//migrator:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	//migrator:schema:field name="last_used_at" type="TIMESTAMP"
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at"`
	//migrator:schema:field name="ip_address" type="VARCHAR(45)"
	IPAddress string `json:"ip_address,omitempty" db:"ip_address"`
	//migrator:schema:field name="user_agent" type="TEXT"
	UserAgent string `json:"user_agent,omitempty" db:"user_agent"`
	//migrator:schema:field name="revoked_at" type="TIMESTAMP"
	RevokedAt *time.Time `json:"revoked_at,omitempty" db:"revoked_at"`
}

// PostgreSQL-specific indexes for backoffice_refresh_tokens.
type BackofficeRefreshTokenIndexes struct {
	// Immutable UUID index (deduplication key, mirroring every other
	// table that embeds EntityID).
	//migrator:schema:index name="idx_backoffice_refresh_tokens_uuid" fields="uuid" unique="true" table="backoffice_refresh_tokens"
	_ int

	// Index for per-user lookups (list active sessions for a back-office
	// user, revoke-all on password change, etc.).
	//migrator:schema:index name="idx_backoffice_refresh_tokens_user_id" fields="backoffice_user_id" table="backoffice_refresh_tokens"
	_ int

	// Unique index on token_hash powers the cookie -> row lookup in the
	// refresh flow. Same constraint shape as the tenant-side refresh
	// token table.
	//migrator:schema:index name="idx_backoffice_refresh_tokens_token_hash" fields="token_hash" unique="true" table="backoffice_refresh_tokens"
	_ int

	// Index for expiry-based cleanup; the retention sweep (future worker)
	// will scan by expires_at.
	//migrator:schema:index name="idx_backoffice_refresh_tokens_expires_at" fields="expires_at" table="backoffice_refresh_tokens"
	_ int
}

// IsValid reports whether the refresh token is not expired and not revoked.
// Mirrors RefreshToken.IsValid so the back-office refresh handler reads
// identical to the tenant-side equivalent.
func (rt *BackofficeRefreshToken) IsValid() bool {
	if rt.RevokedAt != nil {
		return false
	}
	return time.Now().Before(rt.ExpiresAt)
}
