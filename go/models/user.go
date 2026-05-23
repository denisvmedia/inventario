package models

import (
	"context"
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/jellydator/validation"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models/rules"
)

// bcryptCost is the cost factor used by SetPassword. In production it is
// bcrypt.DefaultCost (10, ~80ms per hash). Tests lower it to bcrypt.MinCost
// (4, ~1ms per hash) via SetBcryptCostForTesting; under `go test -race` the
// difference scales by ~10x, which is the only reason the apiserver test
// package stays under the 10-minute per-binary panic timeout.
var bcryptCost = bcrypt.DefaultCost

// SetBcryptCostForTesting overrides the package-level bcrypt cost used by
// SetPassword for the duration of the test (or TestMain). Restores the
// previous value via t.Cleanup so a single test that opts in to MinCost
// doesn't leak the override into other tests in the same package. Pass
// nil for `t` from a TestMain that wants the override to persist for the
// whole binary run.
func SetBcryptCostForTesting(t *testing.T, cost int) {
	orig := bcryptCost
	bcryptCost = cost
	if t != nil {
		t.Cleanup(func() { bcryptCost = orig })
	}
}

var (
	_ validation.Validatable            = (*User)(nil)
	_ validation.ValidatableWithContext = (*User)(nil)
	_ TenantAwareIDable                 = (*User)(nil)
	_ json.Marshaler                    = (*User)(nil)
	_ json.Unmarshaler                  = (*User)(nil)
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="users" comment="Enable RLS for multi-tenant user isolation"
//migrator:schema:rls:policy name="user_isolation" table="users" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures users can only access and modify their own data within their tenant with required contexts"
//migrator:schema:rls:policy name="user_background_worker_access" table="users" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all users for processing"

//migrator:schema:table name="users"
type User struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="email" type="TEXT" not_null="true"
	Email string `json:"email" db:"email"`
	//migrator:schema:field name="password_hash" type="TEXT" not_null="true"
	PasswordHash string `json:"-" db:"password_hash" userinput:"false"`
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//migrator:schema:field name="is_active" type="BOOLEAN" not_null="true" default="true"
	IsActive bool `json:"is_active" db:"is_active"`
	//migrator:schema:field name="last_login_at" type="TIMESTAMP"
	LastLoginAt *time.Time `json:"last_login_at" db:"last_login_at" userinput:"false"`
	// DefaultGroupID is the user's preferred landing group after login.
	// Nullable: when unset, the login flow falls back to "first group the user
	// created, else first group they were invited to" (#1263). ON DELETE SET NULL
	// so removing a group silently clears the preference instead of blocking the delete.
	//migrator:schema:field name="default_group_id" type="TEXT" foreign="location_groups(id)" foreign_key_name="fk_user_default_group" on_delete="SET NULL"
	DefaultGroupID *string `json:"default_group_id" db:"default_group_id"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`

	// IsSystemAdmin is a transient wire-only field reflecting the user's
	// entry in `system_admin_grants` (#1784). It is NEVER persisted on the
	// users row — the privilege lives in the grant table. Handlers that
	// emit a /auth/me-style payload (the authenticated user reading their
	// own identity) populate it from `SystemAdminGrantRegistry.Exists`
	// before encoding; the FE uses it as an advisory hint to gate sidebar
	// and route visibility (the backend re-checks via `RequireSystemAdmin`
	// on every /admin/* request).
	//
	// Persistence safety: there is no `//migrator:schema:field` annotation
	// so the migration generator never re-adds the column, and the `db:"-"`
	// tag tells sqlx to skip the field in both SELECT and INSERT/UPDATE,
	// so this stays purely in-memory. A caller that smuggles `true` here
	// gains nothing — Go authorization paths never read the field; they
	// all consult the grant registry directly.
	IsSystemAdmin bool `json:"is_system_admin" db:"-" userinput:"false"`
}

// PostgreSQL-specific indexes for users
type UserIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//migrator:schema:index name="idx_users_uuid" fields="uuid" unique="true" table="users"
	_ int

	// Unique index for email within tenant
	//migrator:schema:index name="users_tenant_email_idx" fields="tenant_id,email" unique="true" table="users"
	_ int

	// Index for tenant lookups
	//migrator:schema:index name="users_tenant_idx" fields="tenant_id" table="users"
	_ int

	// Index for active users
	//migrator:schema:index name="users_active_idx" fields="is_active" table="users"
	_ int
}

func (*User) Validate() error {
	return ErrMustUseValidateWithContext
}

func (u *User) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&u.Email, rules.NotEmpty, validation.Length(1, 255), validation.Match(EmailPattern)),
		validation.Field(&u.Name, rules.NotEmpty, validation.Length(1, 100)),
		validation.Field(&u.TenantID, rules.NotEmpty),
	)

	return validation.ValidateStructWithContext(ctx, u, fields...)
}

// SetPassword hashes and sets the user's password.
//
// The password is run through ValidatePassword first so the same complexity
// rules (length, upper/lower/digit) are enforced everywhere a password is
// persisted — including admin tooling and seed data, which previously bypassed
// the rules and could store weak credentials.
func (u *User) SetPassword(password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return err
	}

	u.PasswordHash = string(hashedPassword)
	return nil
}

// CheckPassword verifies if the provided password matches the user's password
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

// ValidatePassword validates a password without setting it
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return validation.NewError("validation_password_too_short", "password must be at least 8 characters long")
	}

	// Check for at least one uppercase letter
	if matched, _ := regexp.MatchString(`[A-Z]`, password); !matched {
		return validation.NewError("validation_password_no_uppercase", "password must contain at least one uppercase letter")
	}

	// Check for at least one lowercase letter
	if matched, _ := regexp.MatchString(`[a-z]`, password); !matched {
		return validation.NewError("validation_password_no_lowercase", "password must contain at least one lowercase letter")
	}

	// Check for at least one digit
	if matched, _ := regexp.MatchString(`[0-9]`, password); !matched {
		return validation.NewError("validation_password_no_digit", "password must contain at least one digit")
	}

	return nil
}

// UpdateLastLogin updates the user's last login timestamp
func (u *User) UpdateLastLogin() {
	now := time.Now()
	u.LastLoginAt = &now
}

func (u *User) MarshalJSON() ([]byte, error) {
	type Alias User
	tmp := *u
	return json.Marshal(Alias(tmp))
}

func (u *User) UnmarshalJSON(data []byte) error {
	type Alias User
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(u),
	}
	return json.Unmarshal(data, &aux)
}
