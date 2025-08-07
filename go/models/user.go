package models

import (
	"context"
	"encoding/json"
	"regexp"
	"time"

	"github.com/jellydator/validation"
	"golang.org/x/crypto/bcrypt"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*UserRole)(nil)
	_ validation.Validatable            = (*User)(nil)
	_ validation.ValidatableWithContext = (*User)(nil)
	_ TenantAwareIDable                 = (*User)(nil)
	_ json.Marshaler                    = (*User)(nil)
	_ json.Unmarshaler                  = (*User)(nil)
)

// UserRole represents the role of a user within a tenant
type UserRole string

const (
	UserRoleAdmin UserRole = "admin"
	UserRoleUser  UserRole = "user"
)

// Validate implements the validation.Validatable interface for UserRole
func (ur UserRole) Validate() error {
	switch ur {
	case UserRoleAdmin, UserRoleUser:
		return nil
	default:
		return validation.NewError("validation_invalid_user_role", "must be one of: admin, user")
	}
}

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
	//migrator:schema:field name="role" type="TEXT" not_null="true" default="user"
	Role UserRole `json:"role" db:"role"`
	//migrator:schema:field name="is_active" type="BOOLEAN" not_null="true" default="true"
	IsActive bool `json:"is_active" db:"is_active"`
	//migrator:schema:field name="last_login_at" type="TIMESTAMP"
	LastLoginAt *time.Time `json:"last_login_at" db:"last_login_at" userinput:"false"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// PostgreSQL-specific indexes for users
type UserIndexes struct {
	// Unique index for email within tenant
	//migrator:schema:index name="users_tenant_email_idx" fields="tenant_id,email" unique="true" table="users"
	_ int

	// Index for tenant lookups
	//migrator:schema:index name="users_tenant_idx" fields="tenant_id" table="users"
	_ int

	// Index for role filtering
	//migrator:schema:index name="users_role_idx" fields="role" table="users"
	_ int

	// Index for active users
	//migrator:schema:index name="users_active_idx" fields="is_active" table="users"
	_ int
}

func (*User) Validate() error {
	return ErrMustUseValidateWithContext
}

func (u *User) ValidateWithContext(ctx context.Context) error {
	// Email regex pattern for basic validation
	emailPattern := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&u.Email, rules.NotEmpty, validation.Length(1, 255), validation.Match(emailPattern)),
		validation.Field(&u.Name, rules.NotEmpty, validation.Length(1, 100)),
		validation.Field(&u.Role, validation.Required),
		validation.Field(&u.TenantID, rules.NotEmpty),
	)

	return validation.ValidateStructWithContext(ctx, u, fields...)
}

// SetPassword hashes and sets the user's password
func (u *User) SetPassword(password string) error {
	if len(password) < 8 {
		return validation.NewError("validation_password_too_short", "password must be at least 8 characters long")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
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
