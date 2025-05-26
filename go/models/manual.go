package models

import (
	"context"

	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Manual)(nil)
	_ IDable                 = (*Manual)(nil)
)

type Manual struct {
	EntityID
	CommodityID string `json:"commodity_id" db:"commodity_id"`
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
