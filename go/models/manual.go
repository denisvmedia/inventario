package models

import (
	"context"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Manual)(nil)
	_ IDable                 = (*Manual)(nil)
)

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
