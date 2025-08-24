package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*TenantStatus)(nil)
	_ validation.Validatable            = (*Tenant)(nil)
	_ validation.ValidatableWithContext = (*Tenant)(nil)
	_ IDable                            = (*Tenant)(nil)
	_ json.Marshaler                    = (*Tenant)(nil)
	_ json.Unmarshaler                  = (*Tenant)(nil)
)

// TenantStatus represents the status of a tenant
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusInactive  TenantStatus = "inactive"
)

// Validate implements the validation.Validatable interface for TenantStatus
func (ts TenantStatus) Validate() error {
	switch ts {
	case TenantStatusActive, TenantStatusSuspended, TenantStatusInactive:
		return nil
	default:
		return validation.NewError("validation_invalid_tenant_status", "must be one of: active, suspended, inactive")
	}
}

// TenantSettings represents tenant-specific configuration settings
type TenantSettings map[string]any

// Value implements the driver.Valuer interface for database storage
func (ts TenantSettings) Value() (driver.Value, error) {
	if ts == nil {
		return nil, nil
	}
	return json.Marshal(ts)
}

// Scan implements the sql.Scanner interface for database retrieval
func (ts *TenantSettings) Scan(value any) error {
	if value == nil {
		*ts = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, ts)
	case string:
		return json.Unmarshal([]byte(v), ts)
	default:
		return validation.NewError("validation_invalid_tenant_settings", "cannot scan tenant settings")
	}
}

//migrator:schema:table name="tenants"
type Tenant struct {
	//migrator:embedded mode="inline"
	EntityID
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//migrator:schema:field name="slug" type="TEXT" not_null="true" unique="true"
	Slug string `json:"slug" db:"slug"`
	//migrator:schema:field name="domain" type="TEXT"
	Domain *string `json:"domain" db:"domain"`
	//migrator:schema:field name="status" type="TEXT" not_null="true" default="active"
	Status TenantStatus `json:"status" db:"status"`
	//migrator:schema:field name="settings" type="JSONB"
	Settings TenantSettings `json:"settings" db:"settings"`
	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_fn="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// PostgreSQL-specific indexes for tenants
type TenantIndexes struct {
	// Index for slug lookups
	//migrator:schema:index name="tenants_slug_idx" fields="slug" unique="true" table="tenants"
	_ int

	// Index for domain lookups
	//migrator:schema:index name="tenants_domain_idx" fields="domain" table="tenants"
	_ int

	// Index for status filtering
	//migrator:schema:index name="tenants_status_idx" fields="status" table="tenants"
	_ int
}

func (*Tenant) Validate() error {
	return ErrMustUseValidateWithContext
}

func (t *Tenant) ValidateWithContext(ctx context.Context) error {
	// Compile regex pattern for slug validation
	slugPattern := regexp.MustCompile(`^[a-z0-9-]+$`)

	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&t.Name, rules.NotEmpty, validation.Length(1, 100)),
		validation.Field(&t.Slug, rules.NotEmpty, validation.Length(1, 50), validation.Match(slugPattern)),
		validation.Field(&t.Status, validation.Required),
	)

	// Only validate domain length if it's not empty
	if t.Domain != nil && *t.Domain != "" {
		fields = append(fields, validation.Field(&t.Domain, validation.Length(1, 255)))
	}

	return validation.ValidateStructWithContext(ctx, t, fields...)
}

func (t *Tenant) MarshalJSON() ([]byte, error) {
	type Alias Tenant
	tmp := *t
	return json.Marshal(Alias(tmp))
}

func (t *Tenant) UnmarshalJSON(data []byte) error {
	type Alias Tenant
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	}
	return json.Unmarshal(data, &aux)
}
