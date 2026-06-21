package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// UserMFASecret persists per-user TOTP credentials (#1380 / PR-C
// #1645). The base32 TOTP secret is encrypted at rest via the
// secrets package using an HKDF-derived subkey of the application's
// root signing secret; backup codes are bcrypt-hashed single-use
// tokens. EnabledAt is null between Setup and the first successful
// verify; the row is created at Setup time so a partial enrollment
// can be resumed without re-issuing the QR code.
//
// One row per user, enforced by the (tenant_id, user_id) unique index.
// Disable simply deletes the row; re-enrollment is a fresh Setup.
//
// RLS mirrors refresh_tokens: the inventario_app role only sees the
// owning user's row; the inventario_background_worker role bypasses
// the user/tenant filter so the unauthenticated step-1 of /auth/login
// can read the row before any RLS context is set on the connection.

// Enable RLS for multi-tenant + per-user isolation
//migrator:schema:rls:enable table="user_mfa_secrets" comment="Enable RLS for per-user MFA secret isolation"
//migrator:schema:rls:policy name="user_mfa_isolation" table="user_mfa_secrets" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures MFA secrets can only be accessed and modified by the owning user within their tenant"
//migrator:schema:rls:policy name="user_mfa_background_worker_access" table="user_mfa_secrets" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows the login flow + management endpoints to read the row before RLS context is established on the connection"

//migrator:schema:table name="user_mfa_secrets"
type UserMFASecret struct {
	//migrator:embedded mode="inline"
	TenantUserAwareEntityID
	// SecretEncrypted is the base32 TOTP secret wrapped with the
	// MFA subkey (AES-256-GCM, versioned). NEVER returned to the
	// API surface — only the service layer decrypts it for
	// verification.
	//migrator:schema:field name="secret_encrypted" type="TEXT" not_null="true"
	SecretEncrypted string `json:"-" db:"secret_encrypted" userinput:"false"`
	// EnabledAt flips from null → now() the first time the user
	// successfully verifies a code. Login enforces the gate only when
	// EnabledAt is non-null, so a half-completed enrollment never
	// locks the user out of their account.
	//migrator:schema:field name="enabled_at" type="TIMESTAMP"
	EnabledAt *time.Time `json:"enabled_at,omitempty" db:"enabled_at" userinput:"false"`
	// BackupCodesHashed is a JSON array of bcrypt hashes. Single-use:
	// service consumes one by computing a new array minus the matched
	// entry. Length is informational — 10 codes per regenerate cycle,
	// but a partially-consumed list may have fewer.
	//migrator:schema:field name="backup_codes_hashed" type="JSONB" not_null="true" default="'[]'"
	BackupCodesHashed ValuerSlice[string] `json:"-" db:"backup_codes_hashed" userinput:"false"`
	// LastUsedAt is updated on every successful verification (TOTP
	// or backup code). Used by replay-window heuristics; the FE
	// surfaces it as "Last used" on the disable confirmation card.
	//migrator:schema:field name="last_used_at" type="TIMESTAMP"
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at" userinput:"false"`
	// LastUsedStep is the TOTP time-step counter (unix/30) of the most
	// recently accepted TOTP code. A presented code whose computed step is
	// <= this value is a replay within the ±1-step skew window and is
	// rejected (RFC 6238 §5.2). Bumped atomically on each accepted TOTP.
	//migrator:schema:field name="last_used_step" type="BIGINT" not_null="true" default="0"
	LastUsedStep int64 `json:"-" db:"last_used_step" userinput:"false"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// PostgreSQL-specific indexes for user_mfa_secrets.
type UserMFASecretIndexes struct {
	// Immutable UUID index (deduplication key for import/restore).
	//migrator:schema:index name="idx_user_mfa_secrets_uuid" fields="uuid" unique="true" table="user_mfa_secrets"
	_ int

	// One MFA row per user within a tenant.
	//migrator:schema:index name="idx_user_mfa_secrets_user" fields="tenant_id,user_id" unique="true" table="user_mfa_secrets"
	_ int
}

func (*UserMFASecret) Validate() error {
	return ErrMustUseValidateWithContext
}

func (u *UserMFASecret) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, u,
		validation.Field(&u.TenantID, rules.NotEmpty),
		validation.Field(&u.UserID, rules.NotEmpty),
		validation.Field(&u.SecretEncrypted, rules.NotEmpty),
	)
}

// IsEnabled reports whether the user has completed enrollment. False
// while EnabledAt is null (post-Setup, pre-Verify) — the login flow
// checks this to decide whether to challenge for a TOTP code.
func (u *UserMFASecret) IsEnabled() bool {
	return u != nil && u.EnabledAt != nil
}
