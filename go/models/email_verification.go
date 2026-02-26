package models

import (
	"crypto/rand"
	"encoding/base64"
	"time"
)

// EmailVerification stores a pending email address verification for a user account.
//
//migrator:schema:table name="email_verifications"
type EmailVerification struct {
	// ID is the unique identifier for the verification record.
	//migrator:schema:field name="id" type="TEXT" primary="true"
	ID string `json:"id" db:"id"`

	// UserID is the ID of the user being verified.
	//migrator:schema:field name="user_id" type="TEXT" not_null="true"
	UserID string `json:"user_id" db:"user_id"`

	// TenantID is the tenant this verification belongs to.
	//migrator:schema:field name="tenant_id" type="TEXT" not_null="true"
	TenantID string `json:"tenant_id" db:"tenant_id"`

	// Email is the address being verified.
	//migrator:schema:field name="email" type="TEXT" not_null="true"
	Email string `json:"email" db:"email"`

	// Token is the secure random verification token (never serialised to JSON).
	//migrator:schema:field name="token" type="TEXT" not_null="true"
	Token string `json:"-" db:"token"`

	// ExpiresAt is the time after which the token is no longer valid.
	//migrator:schema:field name="expires_at" type="TIMESTAMP" not_null="true"
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`

	// VerifiedAt is set when the user successfully verifies their email.
	//migrator:schema:field name="verified_at" type="TIMESTAMP"
	VerifiedAt *time.Time `json:"verified_at,omitempty" db:"verified_at"`

	// CreatedAt is when the record was created.
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}

// EmailVerificationIndexes defines PostgreSQL indexes for the email_verifications table.
type EmailVerificationIndexes struct {
	//migrator:schema:index name="email_verifications_user_id_idx" fields="user_id" table="email_verifications"
	_ int
	//migrator:schema:index name="email_verifications_token_idx" fields="token" table="email_verifications"
	_ int
	//migrator:schema:index name="email_verifications_email_idx" fields="email" table="email_verifications"
	_ int
}

// GetID returns the record's unique identifier.
func (ev *EmailVerification) GetID() string { return ev.ID }

// SetID sets the record's unique identifier.
func (ev *EmailVerification) SetID(id string) { ev.ID = id }

// IsExpired reports whether the verification token has passed its expiry time.
func (ev *EmailVerification) IsExpired() bool {
	return time.Now().After(ev.ExpiresAt)
}

// IsVerified reports whether the email address has already been verified.
func (ev *EmailVerification) IsVerified() bool {
	return ev.VerifiedAt != nil
}

// GenerateVerificationToken creates a cryptographically secure URL-safe token.
func GenerateVerificationToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
