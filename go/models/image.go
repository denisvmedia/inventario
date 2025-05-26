package models

import (
	"context"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Image)(nil)
	_ IDable                 = (*Image)(nil)
)

type Image struct {
	EntityID
	CommodityID string `json:"commodity_id" db:"commodity_id"`
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
