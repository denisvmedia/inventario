package models

import (
	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Invoice)(nil)
	_ IDable                 = (*Invoice)(nil)
)

type Invoice struct {
	EntityID
	CommodityID string `json:"commodity_id"`
	*File
}

func (i *Invoice) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&i.CommodityID, validation.Required),
		validation.Field(&i.File, validation.Required),
	)

	return validation.ValidateStruct(i, fields...)
}
