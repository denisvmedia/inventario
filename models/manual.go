package models

import (
	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Manual)(nil)
	_ IDable                 = (*Manual)(nil)
)

type Manual struct {
	EntityID
	CommodityID string `json:"commodity_id"`
	*File
}

func (m *Manual) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&m.CommodityID, validation.Required),
		validation.Field(&m.File, validation.Required),
	)

	return validation.ValidateStruct(m, fields...)
}
