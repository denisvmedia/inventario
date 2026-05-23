package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// BackofficeUserMFASecret persists per-back-office-user TOTP credentials
// (issue #1785, Phase 4). The base32 TOTP secret is encrypted at rest via
// the secrets package using an HKDF-derived subkey of the application's
// root signing secret; backup codes are bcrypt-hashed single-use tokens.
// EnabledAt is set at Setup time by the operator CLI, so a row in this
// table always represents an active enrollment — unlike UserMFASecret,
// which has a pending-enrollment state because users self-enroll over
// HTTP. Phase 4 has no self-service enrollment surface for back-office
// users; only the operator CLI mints rows.
//
// One row per back-office user, enforced by the unique index on
// backoffice_user_id.
//
// The table has NO row-level security — same reasoning as backoffice_users
// itself: it lives OUTSIDE the tenant model. Access is gated entirely at
// the application layer by the back-office auth plane (issue #1785,
// Phase 2/4).

//migrator:schema:table name="backoffice_user_mfa_secrets"
type BackofficeUserMFASecret struct {
	//migrator:embedded mode="inline"
	EntityID
	// BackofficeUserID is the FK to backoffice_users.id. Distinct from
	// UserMFASecret.user_id so a stolen tenant secret can never resolve to
	// a back-office identity even if the hash collided.
	//migrator:schema:field name="backoffice_user_id" type="TEXT" not_null="true" foreign="backoffice_users(id)" foreign_key_name="fk_backoffice_mfa_user"
	BackofficeUserID string `json:"-" db:"backoffice_user_id" userinput:"false"`
	// SecretEncrypted is the base32 TOTP secret wrapped with the
	// back-office MFA subkey (AES-256-GCM, versioned). NEVER returned to
	// the API surface — only the service layer decrypts it for verification.
	//migrator:schema:field name="secret_encrypted" type="TEXT" not_null="true"
	SecretEncrypted string `json:"-" db:"secret_encrypted" userinput:"false"`
	// EnabledAt is set at CLI setup time. Unlike UserMFASecret (where
	// the field flips from null → now() at first verification), the
	// back-office flow has no over-HTTP self-enrollment — the operator
	// CLI marks the row enabled atomically with its insert. A null value
	// would block the login challenge, so it MUST be populated.
	//migrator:schema:field name="enabled_at" type="TIMESTAMP"
	EnabledAt *time.Time `json:"enabled_at,omitempty" db:"enabled_at" userinput:"false"`
	// BackupCodesHashed is a JSON array of bcrypt hashes. Single-use:
	// service consumes one by computing a new array minus the matched
	// entry. Length is informational — 10 codes per regenerate cycle,
	// but a partially-consumed list may have fewer.
	//migrator:schema:field name="backup_codes_hashed" type="JSONB" not_null="true" default_expr="'[]'"
	BackupCodesHashed ValuerSlice[string] `json:"-" db:"backup_codes_hashed" userinput:"false"`
	// LastUsedAt is updated on every successful verification (TOTP or
	// backup code). Used by replay-window heuristics; the operator-facing
	// CLI surfaces it as "Last used" on the disable confirmation prompt.
	//migrator:schema:field name="last_used_at" type="TIMESTAMP"
	LastUsedAt *time.Time `json:"last_used_at,omitempty" db:"last_used_at" userinput:"false"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// PostgreSQL-specific indexes for backoffice_user_mfa_secrets.
type BackofficeUserMFASecretIndexes struct {
	// Immutable UUID index (deduplication key for import/restore).
	//migrator:schema:index name="idx_backoffice_user_mfa_secrets_uuid" fields="uuid" unique="true" table="backoffice_user_mfa_secrets"
	_ int

	// One MFA row per back-office user.
	//migrator:schema:index name="idx_backoffice_user_mfa_secrets_user" fields="backoffice_user_id" unique="true" table="backoffice_user_mfa_secrets"
	_ int
}

func (*BackofficeUserMFASecret) Validate() error {
	return ErrMustUseValidateWithContext
}

func (m *BackofficeUserMFASecret) ValidateWithContext(ctx context.Context) error {
	return validation.ValidateStructWithContext(ctx, m,
		validation.Field(&m.BackofficeUserID, rules.NotEmpty),
		validation.Field(&m.SecretEncrypted, rules.NotEmpty),
	)
}

// IsEnabled reports whether the row represents an active enrollment. The
// CLI setup flow always stamps EnabledAt, so a row with a null EnabledAt
// is anomalous and should be treated as not-enabled (fail-closed).
func (m *BackofficeUserMFASecret) IsEnabled() bool {
	return m != nil && m.EnabledAt != nil
}
