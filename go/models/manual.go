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
	EntityID
	//migrator:schema:field name="commodity_id" type="TEXT" not_null="true" foreign="commodities(id)" foreign_key_name="fk_manual_commodity"
	CommodityID string `json:"commodity_id" db:"commodity_id"`
	//migrator:embedded mode="inline"
	*File
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
