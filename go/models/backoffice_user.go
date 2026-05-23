package models

import (
	"context"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

// BackofficeUser is a platform-operator identity that lives entirely
// OUTSIDE the tenant model. Where models.User belongs to a specific
// tenant and authenticates against the tenant-scoped login flow, a
// BackofficeUser belongs to the platform itself: there is no
// tenant_id column, no per-tenant RLS, no group membership. The whole
// point of the issue #1785 epic is to keep platform operators (support
// agents, platform admins) separate from regular users so impersonating
// a customer or escalating to system-admin cannot happen by accidentally
// flipping a column on a normal user row.
//
// The table has NO row-level security — same reason `tenants` has none:
// it IS the boundary. Access is gated entirely at the application layer
// by the back-office auth plane (added in Phase 2 / #1785).
//
// Email is unique platform-wide and is lowercased at the registry layer
// before persisting and before comparison. The migrator schema annotations
// don't express functional indexes (`UNIQUE INDEX (lower(email))`), so
// case-insensitivity is enforced by registry-level normalisation paired
// with a regular unique index on the `email` column. Callers MUST go
// through BackofficeUserRegistry.{Create,GetByEmail,Update,SetPasswordHash}
// — direct INSERTs that bypass the lower-casing would let duplicates land
// and the unique index would NOT catch them.
//
// MFAEnforced defaults to false in Phase 2 (the schema default and the
// CLI bootstrap both set false) so the data state matches reality: the
// MFA challenge flow lands in Phase 4. The Phase-2 login handler returns
// 501 when MFAEnforced=true so any row manually flipped to true fails
// closed instead of silently bypassing MFA. Phase 4 will flip both
// defaults back to true at the same commit that wires the enforcement.
//
// PasswordHash is bcrypt at DefaultCost — matching models.User so the
// hash format is identical and the CLI bootstrap can reuse the standard
// bcrypt code paths.

//migrator:schema:table name="backoffice_users"
type BackofficeUser struct {
	//migrator:embedded mode="inline"
	EntityID
	//migrator:schema:field name="email" type="TEXT" not_null="true"
	Email string `json:"email" db:"email"`
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	// PasswordHash is bcrypt(DefaultCost). Marshal-blocked via `json:"-"`
	// so the hash can never accidentally leak to a JSON response — back-
	// office identities are higher-value than regular users, so the
	// guardrail matters even more.
	//migrator:schema:field name="password_hash" type="TEXT" not_null="true"
	PasswordHash string `json:"-" db:"password_hash" userinput:"false"`
	// Role is a typed enum: support_agent | platform_admin. Validated in
	// ValidateWithContext via BackofficeRole.Validate.
	//migrator:schema:field name="role" type="TEXT" not_null="true"
	Role BackofficeRole `json:"role" db:"role"`
	//migrator:schema:field name="is_active" type="BOOLEAN" not_null="true" default="true"
	IsActive bool `json:"is_active" db:"is_active"`
	// MFAEnforced flips back to default-true now that Phase 4 wires the
	// real step-1 challenge + step-2 endpoint (issue #1785). A row with
	// MFAEnforced=true AND a populated backoffice_user_mfa_secrets row
	// triggers the MFA challenge response from POST /backoffice/auth/login;
	// MFAEnforced=true with NO secret row still returns the 501 fail-closed
	// branch so an operator must run `inventario backoffice mfa setup`
	// before they can sign in. The schema default flipped from false to
	// true at the same commit that wired the challenge so the security
	// promise lines up with the data state.
	//migrator:schema:field name="mfa_enforced" type="BOOLEAN" not_null="true" default="true"
	MFAEnforced bool `json:"mfa_enforced" db:"mfa_enforced"`
	//migrator:schema:field name="last_login_at" type="TIMESTAMP"
	LastLoginAt *time.Time `json:"last_login_at" db:"last_login_at" userinput:"false"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// PostgreSQL-specific indexes for backoffice_users.
type BackofficeUserIndexes struct {
	// Immutable UUID index (deduplication key for import/restore, mirroring
	// every other table that embeds EntityID).
	//migrator:schema:index name="idx_backoffice_users_uuid" fields="uuid" unique="true" table="backoffice_users"
	_ int

	// Unique index on email enforces platform-wide uniqueness. The
	// registry layer lowercases email on write + read so case variants
	// collapse to the same row; without that normalisation the unique
	// index alone would let "Admin@x.com" and "admin@x.com" coexist.
	//migrator:schema:index name="idx_backoffice_users_email" fields="email" unique="true" table="backoffice_users"
	_ int

	// Index for active-user lookups (Phase 2's login flow will filter on
	// is_active before checking the password hash).
	//migrator:schema:index name="idx_backoffice_users_active" fields="is_active" table="backoffice_users"
	_ int
}

var (
	_ validation.Validatable            = (*BackofficeUser)(nil)
	_ validation.ValidatableWithContext = (*BackofficeUser)(nil)
	_ IDable                            = (*BackofficeUser)(nil)
)

func (*BackofficeUser) Validate() error {
	return ErrMustUseValidateWithContext
}

func (u *BackofficeUser) ValidateWithContext(ctx context.Context) error {
	fields := []*validation.FieldRules{
		validation.Field(&u.Email, rules.NotEmpty, validation.Length(1, 255), validation.Match(EmailPattern)),
		validation.Field(&u.Name, rules.NotEmpty, validation.Length(1, 100)),
		validation.Field(&u.Role, validation.Required),
	}

	return validation.ValidateStructWithContext(ctx, u, fields...)
}
