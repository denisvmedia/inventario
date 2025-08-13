package models

import (
	"context"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable = (*Area)(nil)
	_ IDable                 = (*Area)(nil)
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="areas" comment="Enable RLS for multi-tenant area isolation"
//migrator:schema:rls:policy name="area_tenant_isolation" table="areas" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id()" comment="Ensures areas can only be accessed by their tenant"
//migrator:schema:rls:policy name="area_user_isolation" table="areas" for="ALL" to="inventario_app" using="user_id = get_current_user_id()" with_check="user_id = get_current_user_id()" comment="Ensures areas can only be accessed and modified by their user"

//migrator:schema:table name="areas"
type Area struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//migrator:schema:field name="location_id" type="TEXT" not_null="true" foreign="locations(id)" foreign_key_name="fk_area_location"
	LocationID string `json:"location_id" db:"location_id"`
}

// AreaIndexes defines performance indexes for the areas table
type AreaIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_areas_tenant_id" fields="tenant_id" table="areas"
	_ int

	// Composite index for tenant + location queries
	//migrator:schema:index name="idx_areas_tenant_location" fields="tenant_id,location_id" table="areas"
	_ int
}

func (*Area) Validate() error {
	return ErrMustUseValidateWithContext
}

func (a *Area) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.LocationID, rules.NotEmpty),
		validation.Field(&a.Name, rules.NotEmpty),
	)

	return validation.ValidateStructWithContext(ctx, a, fields...)
}
