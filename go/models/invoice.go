package models

import (
	"context"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Invoice)(nil)
	_ IDable                 = (*Invoice)(nil)
)

// Enable RLS for multi-tenant isolation
//migrator:schema:rls:enable table="invoices" comment="Enable RLS for multi-tenant invoice isolation"
//migrator:schema:rls:policy name="invoice_isolation" table="invoices" for="ALL" to="inventario_app" using="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" with_check="tenant_id = get_current_tenant_id() AND get_current_tenant_id() IS NOT NULL AND get_current_tenant_id() != '' AND user_id = get_current_user_id() AND get_current_user_id() IS NOT NULL AND get_current_user_id() != ''" comment="Ensures invoices can only be accessed and modified by their tenant and user with required contexts"
//migrator:schema:rls:policy name="invoice_background_worker_access" table="invoices" for="ALL" to="inventario_background_worker" using="true" with_check="true" comment="Allows background workers to access all invoices for processing"

//migrator:schema:table name="invoices"
type Invoice struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_invoice_commodity"
	CommodityID string `json:"commodity_id" db:"commodity_id"`
	//migrator:embedded mode="inline"
	*File
}

// InvoiceIndexes defines performance indexes for the invoices table
type InvoiceIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_invoices_tenant_id" fields="tenant_id" table="invoices"
	_ int

	// Composite index for tenant + commodity queries
	//migrator:schema:index name="idx_invoices_tenant_commodity" fields="tenant_id,commodity_id" table="invoices"
	_ int
}

func (*Invoice) Validate() error {
	return ErrMustUseValidateWithContext
}

func (i *Invoice) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&i.CommodityID, validation.Required),
		validation.Field(&i.File, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, i, fields...)
}
