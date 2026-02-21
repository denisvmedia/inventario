package models

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="refresh_tokens" comment="Enable RLS for multi-tenant refresh token isolation"
//migrator:schema:rls:policy name="refresh_token_isolation" table="refresh_tokens" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures refresh tokens can only be accessed and modified by the owning user within their tenant"
//migrator:schema:rls:policy name="refresh_token_background_worker_access" table="refresh_tokens" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all refresh tokens for cleanup"

//migrator:schema:table name="refresh_tokens"
type RefreshToken struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="token_hash" type="VARCHAR(128)" not_null="true"
	TokenHash string `json:"-" db:"token_hash"`
	//migrator:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
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

// PostgreSQL-specific indexes for refresh_tokens
type RefreshTokenIndexes struct {
	// Index for user-based queries
	//migrator:schema:index name="idx_refresh_tokens_user_id" fields="user_id" table="refresh_tokens"
	_ int

	// Unique index for token hash lookups
	//migrator:schema:index name="idx_refresh_tokens_token_hash" fields="token_hash" unique="true" table="refresh_tokens"
	_ int

	// Index for expiry-based cleanup
	//migrator:schema:index name="idx_refresh_tokens_expires_at" fields="expires_at" table="refresh_tokens"
	_ int
}

// GenerateRefreshToken creates a cryptographically secure random token and its SHA-256 hash.
// Returns the raw token (to send to client), the hash (to store in DB), and any error.
func GenerateRefreshToken() (token string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	hash = base64.RawURLEncoding.EncodeToString(h[:])
	return token, hash, nil
}

// HashRefreshToken computes the SHA-256 hash of a raw token string.
func HashRefreshToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return base64.RawURLEncoding.EncodeToString(h[:])
}

// IsValid reports whether the refresh token is not expired and not revoked.
func (rt *RefreshToken) IsValid() bool {
	if rt.RevokedAt != nil {
		return false
	}
	return time.Now().Before(rt.ExpiresAt)
}
