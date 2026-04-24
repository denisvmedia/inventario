package models

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable            = (*LocationGroupStatus)(nil)
	_ validation.Validatable            = (*LocationGroup)(nil)
	_ validation.ValidatableWithContext = (*LocationGroup)(nil)
	_ TenantAwareIDable                 = (*LocationGroup)(nil)
)

// LocationGroupStatus represents the status of a location group.
type LocationGroupStatus string

const (
	LocationGroupStatusActive          LocationGroupStatus = "active"
	LocationGroupStatusPendingDeletion LocationGroupStatus = "pending_deletion"
)

// Validate implements the validation.Validatable interface for LocationGroupStatus.
func (s LocationGroupStatus) Validate() error {
	switch s {
	case LocationGroupStatusActive, LocationGroupStatusPendingDeletion:
		return nil
	default:
		return validation.NewError("validation_invalid_location_group_status", "must be one of: active, pending_deletion")
	}
}

// Enable RLS for multi-tenant isolation (tenant-only; group access is controlled at application level)
//
//migrator:schema:rls:enable table="location_groups" comment="Enable RLS for multi-tenant location group isolation"
//migrator:schema:rls:policy name="location_group_tenant_isolation" table="location_groups" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != ''" comment="Ensures location groups are isolated by tenant; group-level access is enforced in application logic via memberships"
//migrator:schema:rls:policy name="location_group_background_worker_access" table="location_groups" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all location groups for processing"

//migrator:schema:table name="location_groups"
type LocationGroup struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID

	// Slug is a non-human-readable, randomly generated identifier used in URLs.
	// It must be infeasible to guess or brute-force (22+ chars, base64url).
	// Immutable after creation (this decision may be revisited — see issue #1219 §2).
	//migrator:schema:field name="slug" type="TEXT" not_null="true"
	Slug string `json:"slug" db:"slug" userinput:"false"`

	// Name is a human-readable display name visible only to group members.
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`

	// Icon is an optional emoji from the curated set in
	// models.ValidGroupIcons. Empty string means "no icon". Issue #1255.
	//migrator:schema:field name="icon" type="TEXT"
	Icon string `json:"icon" db:"icon"`

	// Status indicates whether the group is active or pending deletion.
	//migrator:schema:field name="status" type="TEXT" not_null="true" default="active"
	Status LocationGroupStatus `json:"status" db:"status"`

	// CreatedBy is the user ID of the group creator.
	//migrator:schema:field name="created_by" type="TEXT" not_null="true" foreign="users(id)" foreign_key_name="fk_location_group_created_by"
	CreatedBy string `json:"created_by" db:"created_by" userinput:"false"`

	// MainCurrency is the ISO-4217 code the group values its inventory in. It is
	// a property of the group (not the user) because a user can belong to
	// groups valued in different currencies. Admins change it via the group's
	// update endpoint; changing it triggers a reprice of the group's commodities.
	//migrator:schema:field name="main_currency" type="TEXT" not_null="true" default="USD"
	MainCurrency Currency `json:"main_currency" db:"main_currency"`

	//migrator:schema:field name="created_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	CreatedAt time.Time `json:"created_at" db:"created_at" userinput:"false"`
	//migrator:schema:field name="updated_at" type="TIMESTAMP" not_null="true" default_expr="CURRENT_TIMESTAMP"
	UpdatedAt time.Time `json:"updated_at" db:"updated_at" userinput:"false"`
}

// LocationGroupIndexes defines PostgreSQL indexes for the location_groups table.
type LocationGroupIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//migrator:schema:index name="idx_location_groups_uuid" fields="uuid" unique="true" table="location_groups"
	_ int

	// Unique index for slug within tenant
	//migrator:schema:index name="idx_location_groups_tenant_slug" fields="tenant_id,slug" unique="true" table="location_groups"
	_ int

	// Index for tenant-based queries
	//migrator:schema:index name="idx_location_groups_tenant_id" fields="tenant_id" table="location_groups"
	_ int

	// Index for status filtering
	//migrator:schema:index name="idx_location_groups_status" fields="status" table="location_groups"
	_ int
}

func (*LocationGroup) Validate() error {
	return ErrMustUseValidateWithContext
}

func (lg *LocationGroup) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&lg.Name, rules.NotEmpty, validation.Length(1, 100)),
		validation.Field(&lg.Slug, rules.NotEmpty, validation.Length(22, 64)),
		validation.Field(&lg.Status, validation.Required),
		validation.Field(&lg.TenantID, rules.NotEmpty),
		validation.Field(&lg.CreatedBy, rules.NotEmpty),
		validation.Field(&lg.MainCurrency, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, lg, fields...)
}

// IsActive returns true if the group is in the active state.
func (lg *LocationGroup) IsActive() bool {
	return lg.Status == LocationGroupStatusActive
}

// GenerateGroupSlug creates a cryptographically random, URL-safe slug
// of at least 22 characters (base64url-encoded 16 random bytes = 22 chars).
func GenerateGroupSlug() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
