package models

import (
	"context"
	"database/sql/driver"
	"encoding/json"
	"regexp"
	"time"

	"github.com/jellydator/validation"

	"go.5x5.cz/inventario/models/rules"
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

//ptah:schema:table name="tenants"
type Tenant struct {
	//ptah:embedded mode="inline"
	EntityID
	//ptah:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//ptah:schema:field name="slug" type="TEXT" not_null="true" unique="true"
	Slug string `json:"slug" db:"slug"`
	//ptah:schema:field name="domain" type="TEXT"
	Domain *string `json:"domain" db:"domain"`
	//ptah:schema:field name="status" type="TEXT" not_null="true" default="active"
	Status TenantStatus `json:"status" db:"status"`
	//ptah:schema:field name="is_default" type="BOOLEAN" not_null="true" default="false"
	IsDefault bool `json:"is_default" db:"is_default"`
	//ptah:schema:field name="registration_mode" type="TEXT" not_null="true" default="closed"
	RegistrationMode RegistrationMode `json:"registration_mode" db:"registration_mode"`
	//ptah:schema:field name="settings" type="JSONB"
	Settings TenantSettings `json:"settings" db:"settings"`
	// PlanID is the subscription tier this tenant pays for. The actual
	// limits + capability gates live on the corresponding `models.Plan`
	// constant (`models.PlanByID(plan_id)`); the column itself only
	// stores the textual id. Defaults to `unlimited` at the SQL level
	// so a fresh self-hosted install behaves like the pre-#1389 binary
	// (issue #1389 — AC: "Self-hosters get `unlimited` as the default
	// tenant plan").
	//
	// There is intentionally no model-level validation on this field
	// yet — until the enforcement layer + write paths exist, the only
	// writer is the DB default + operator hand-edits. Unknown values
	// degrade to PlanUnlimited at read time via `models.PlanByID`
	// rather than rejecting the request.
	//ptah:schema:field name="plan_id" type="TEXT" not_null="true" default="unlimited"
	PlanID string `json:"plan_id" db:"plan_id"`
	//ptah:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//ptah:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// PostgreSQL-specific indexes for tenants
type TenantIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//ptah:schema:index name="idx_tenants_uuid" fields="uuid" unique="true" table="tenants"
	_ int

	// Index for slug lookups
	//ptah:schema:index name="tenants_slug_idx" fields="slug" unique="true" table="tenants"
	_ int

	// Index for domain lookups
	//ptah:schema:index name="tenants_domain_idx" fields="domain" table="tenants"
	_ int

	// Index for status filtering
	//ptah:schema:index name="tenants_status_idx" fields="status" table="tenants"
	_ int

	// Partial unique index ensuring at most one tenant can be the system default
	//ptah:schema:index name="tenants_single_default_idx" fields="is_default" unique="true" condition="is_default = true" table="tenants"
	_ int

	// Index for plan_id lookups (the Plan & quota card joins tenants→plans
	// on every group settings open; #1389).
	//ptah:schema:index name="idx_tenants_plan_id" fields="plan_id" table="tenants"
	_ int
}

func (*Tenant) Validate() error {
	return ErrMustUseValidateWithContext
}

// IsValidTenantDomain reports whether s is the shape a tenant domain has to be
// in to be usable: a bare lowercase hostname, matched against the normalized
// request host verbatim.
//
// Validation refuses to store anything else, and this exists for the code that
// reads a row back and has to act on it. A value that predates the rule, or one
// written by hand, is still whatever is in the column — and "evil.test/x" put
// into a redirect URL is a redirect to evil.test.
func IsValidTenantDomain(s string) bool {
	return s != "" && len(s) <= 255 && domainPattern.MatchString(s)
}

// domainPattern matches a lowercase hostname: labels of letters, digits and
// inner hyphens, joined by dots. It deliberately rejects a scheme, a port, a
// trailing dot and any uppercase letter, because the resolver compares the
// normalized request host against this column verbatim.
var domainPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$`)

func (t *Tenant) ValidateWithContext(ctx context.Context) error {
	// Compile regex pattern for slug validation
	slugPattern := regexp.MustCompile(`^[a-z0-9-]+$`)

	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&t.Name, rules.NotEmpty, validation.Length(1, 100)),
		validation.Field(&t.Slug, rules.NotEmpty, validation.Length(1, 50), validation.Match(slugPattern)),
		validation.Field(&t.Status, validation.Required),
	)

	// Validate registration mode only when set; registries normalise the empty
	// zero-value to RegistrationModeClosed before persisting.
	if t.RegistrationMode != "" {
		fields = append(fields, validation.Field(&t.RegistrationMode))
	}

	// Only validate the domain when one is set — it is optional.
	//
	// The pattern is what makes host-based resolution work: the Host header a
	// request arrives with is lowercased and stripped of its port before the
	// lookup, and the lookup matches the column exactly, so a domain stored
	// as "Acme.COM" or "acme.com:8080" would never be found. Rejecting those
	// on write gives the operator an error instead of a tenant that silently
	// cannot be reached. Same reasoning as the slug pattern above.
	if t.Domain != nil && *t.Domain != "" {
		fields = append(fields, validation.Field(&t.Domain,
			validation.Length(1, 255), validation.Match(domainPattern)))
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
