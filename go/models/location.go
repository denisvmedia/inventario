package models

import (
	"context"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable = (*Location)(nil)
	_ TenantGroupAwareIDable = (*Location)(nil)
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="locations" comment="Enable RLS for multi-tenant location isolation"
//migrator:schema:rls:policy name="location_isolation" table="locations" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND group_id = get_current_group_id() AND get_current_group_id() IS NOT NULL AND get_current_group_id() != ''" comment="Ensures locations can only be accessed and modified by their tenant and group with required contexts"
//migrator:schema:rls:policy name="location_background_worker_access" table="locations" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all locations for processing"

//migrator:schema:table name="locations"
type Location struct {
	//migrator:embedded mode="inline"
	TenantGroupAwareEntityID
	//migrator:schema:field name="name" type="TEXT" not_null="true"
	Name string `json:"name" db:"name"`
	//migrator:schema:field name="address" type="TEXT" not_null="true"
	Address string `json:"address" db:"address"`
}

// LocationIndexes defines performance indexes for the locations table
type LocationIndexes struct {
	// Unique index for the immutable UUID (deduplication key for import/restore)
	//migrator:schema:index name="idx_locations_uuid" fields="uuid" unique="true" table="locations"
	_ int

	// Index for tenant-based queries
	//migrator:schema:index name="idx_locations_tenant_id" fields="tenant_id" table="locations"
	_ int

	// Composite index for tenant+group RLS-filtered queries (e.g. list-by-group)
	//migrator:schema:index name="idx_locations_tenant_group" fields="tenant_id,group_id" table="locations"
	_ int
}

func (*Location) Validate() error {
	return ErrMustUseValidateWithContext
}

func (a *Location) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.Name, rules.NotEmpty),
		validation.Field(&a.Address, rules.NotEmpty),
	)

	return validation.ValidateStructWithContext(ctx, a, fields...)
}
