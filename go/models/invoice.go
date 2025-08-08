package models

import (
	"context"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Invoice)(nil)
	_ IDable                 = (*Invoice)(nil)
)

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
