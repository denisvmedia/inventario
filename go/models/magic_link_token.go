//nolint:dupl // MagicLinkToken is structurally similar to PasswordReset/EmailVerification but a semantically distinct domain model.
package models

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// MagicLinkToken stores a pending passwordless sign-in ("magic link") request
// for a user account. Verifying a single-use, short-lived token grants a full
// session, so the credential is treated like a bearer secret: opaque random
// token (never serialised to JSON), short expiry, and an atomic single-use
// claim via the claimed_at sentinel.
//
//migrator:schema:table name="magic_link_tokens"
type MagicLinkToken struct {
	// ID is the unique identifier for the token record.
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id" db:"id"`
	// UUID is the immutable public identifier, stable across restores.
	//migrator:schema:field name="uuid" type="TEXT" not_null="true" default_expr="(gen_random_uuid())::text"
	UUID string `json:"uuid" db:"uuid" userinput:"false"`

	// UserID is the ID of the user the sign-in link belongs to.
	//migrator:schema:field name="user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_magic_link_token_user"
	UserID string `json:"user_id" db:"user_id"`

	// TenantID is the tenant this sign-in request belongs to.
	//migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_magic_link_token_tenant"
	TenantID string `json:"tenant_id" db:"tenant_id"`

	// Email is the address associated with the sign-in request.
	//migrator:schema:field name="email" type="TEXT" not_null="true"
	Email string `json:"email" db:"email"`

	// Token is the secure random sign-in token (never serialised to JSON).
	//migrator:schema:field name="token" type="TEXT" not_null="true"
	Token string `json:"-" db:"token"`

	// ExpiresAt is the time after which the token is no longer valid (15 minutes).
	//migrator:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`

	// ClaimedAt is set when the token has been successfully consumed. It is the
	// single-use sentinel — analogous to PasswordReset's UsedAt, but flipped
	// atomically by MarkClaimed so a replay or concurrent request can never
	// burn the same link twice.
	//migrator:schema:field name="claimed_at" type="TIMESTAMP"
	ClaimedAt *time.Time `json:"claimed_at,omitempty" db:"claimed_at"`

	// CreatedAt is when the record was created.
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// MagicLinkTokenIndexes defines PostgreSQL indexes for the magic_link_tokens table.
type MagicLinkTokenIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//migrator:schema:index name="idx_magic_link_tokens_uuid" fields="uuid" unique="true" table="magic_link_tokens"
	_ int
	//migrator:schema:index name="magic_link_tokens_user_id_idx" fields="user_id" table="magic_link_tokens"
	_ int
	//migrator:schema:index name="magic_link_tokens_token_idx" fields="token" unique="true" table="magic_link_tokens"
	_ int
	//migrator:schema:index name="magic_link_tokens_email_idx" fields="email" table="magic_link_tokens"
	_ int
}

// GetID returns the record's unique identifier.
func (mlt *MagicLinkToken) GetID() string { return mlt.ID }

// SetID sets the record's unique identifier.
func (mlt *MagicLinkToken) SetID(id string) { mlt.ID = id }

// GetUUID returns the record's immutable UUID.
func (mlt *MagicLinkToken) GetUUID() string { return mlt.UUID }

// SetUUID sets the record's immutable UUID.
func (mlt *MagicLinkToken) SetUUID(uuid string) { mlt.UUID = uuid }

// IsExpired reports whether the sign-in token has passed its expiry time.
func (mlt *MagicLinkToken) IsExpired() bool {
	return time.Now().After(mlt.ExpiresAt)
}

// IsClaimed reports whether the token has already been consumed.
func (mlt *MagicLinkToken) IsClaimed() bool {
	return mlt.ClaimedAt != nil
}

// GenerateMagicLinkToken creates a cryptographically secure URL-safe token.
func GenerateMagicLinkToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
