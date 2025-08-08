package models

import (
	"context"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Image)(nil)
	_ IDable                 = (*Image)(nil)
)

//migrator:schema:table name="images"
type Image struct {
	//migrator:embedded mode="inline"
	TenantAwareEntityID
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_image_commodity"
	CommodityID string `json:"commodity_id" db:"commodity_id"`
	//migrator:embedded mode="inline"
	*File
}

// ImageIndexes defines performance indexes for the images table
type ImageIndexes struct {
	// Index for tenant-based queries
	//migrator:schema:index name="idx_images_tenant_id" fields="tenant_id" table="images"
	_ int

	// Composite index for tenant + commodity queries
	//migrator:schema:index name="idx_images_tenant_commodity" fields="tenant_id,commodity_id" table="images"
	_ int
}

func (*Image) Validate() error {
	return ErrMustUseValidateWithContext
}

func (i *Image) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&i.CommodityID, validation.Required),
		validation.Field(&i.File, validation.Required),
	)

	return validation.ValidateStructWithContext(ctx, i, fields...)
}
