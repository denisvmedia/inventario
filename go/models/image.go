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
	EntityID
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_image_commodity"
	CommodityID string `json:"commodity_id" db:"commodity_id"`
	//migrator:embedded mode="inline"
	*File
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
