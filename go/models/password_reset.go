//nolint:dupl // PasswordReset and EmailVerification are structurally similar but semantically distinct domain models.
package models

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// PasswordReset stores a pending password-reset request for a user account.
//
//migrator:schema:table name="password_resets"
type PasswordReset struct {
	// ID is the unique identifier for the reset record.
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id" db:"id"`

	// UserID is the ID of the user requesting the reset.
	//migrator:schema:field name="user_id" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_password_reset_user"
	UserID string `json:"user_id" db:"user_id"`

	// TenantID is the tenant this reset request belongs to.
	//migrator:schema:field name="tenant_id" type="TEXT" not_null="true" foreign="tenants(id)" foreign_key_name="fk_password_reset_tenant"
	TenantID string `json:"tenant_id" db:"tenant_id"`

	// Email is the address associated with the reset request.
	//migrator:schema:field name="email" type="TEXT" not_null="true"
	Email string `json:"email" db:"email"`

	// Token is the secure random reset token (never serialised to JSON).
	//migrator:schema:field name="token" type="TEXT" not_null="true"
	Token string `json:"-" db:"token"`

	// ExpiresAt is the time after which the token is no longer valid (1 hour).
	//migrator:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`

	// UsedAt is set when the token has been successfully consumed.
	//migrator:schema:field name="used_at" type="TIMESTAMP"
	UsedAt *time.Time `json:"used_at,omitempty" db:"used_at"`

	// CreatedAt is when the record was created.
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// PasswordResetIndexes defines PostgreSQL indexes for the password_resets table.
type PasswordResetIndexes struct {
	//migrator:schema:index name="password_resets_user_id_idx" fields="user_id" table="password_resets"
	_ int
	//migrator:schema:index name="password_resets_token_idx" fields="token" unique="true" table="password_resets"
	_ int
	//migrator:schema:index name="password_resets_email_idx" fields="email" table="password_resets"
	_ int
}

// GetID returns the record's unique identifier.
func (pr *PasswordReset) GetID() string { return pr.ID }

// SetID sets the record's unique identifier.
func (pr *PasswordReset) SetID(id string) { pr.ID = id }

// IsExpired reports whether the reset token has passed its expiry time.
func (pr *PasswordReset) IsExpired() bool {
	return time.Now().After(pr.ExpiresAt)
}

// IsUsed reports whether the token has already been consumed.
func (pr *PasswordReset) IsUsed() bool {
	return pr.UsedAt != nil
}

// GeneratePasswordResetToken creates a cryptographically secure URL-safe token.
func GeneratePasswordResetToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
