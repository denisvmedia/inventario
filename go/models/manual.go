package models

import (
	"context"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Manual)(nil)
	_ IDable                 = (*Manual)(nil)
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="manuals" comment="Enable RLS for multi-tenant manual isolation"
//migrator:schema:rls:policy name="manual_isolation" table="manuals" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures manuals can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="manual_background_worker_access" table="manuals" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all manuals for processing"

//migrator:schema:table name="manuals"
type Manual struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_manual_commodity"
	CommodityID string `json:"commodity_id" db:"commodity_id"`
	//migrator:embedded mode="inline"
	*File
}

// ManualIndexes defines performance indexes for the manuals table
type ManualIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_manuals_tenant_id" fields="tenant_id" table="manuals"
	_ int

	// Composite index for tenant + commodity queries
	//migrator:schema:index name="idx_manuals_tenant_commodity" fields="tenant_id,commodity_id" table="manuals"
	_ int
}

func (*Manual) Validate() error {
	return ErrMustUseValidateWithContext
}

func (m *Manual) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&m.CommodityID, validation.Required),
		validation.Field(&m.File, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, m, fields...)
}
