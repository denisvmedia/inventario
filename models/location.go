package models

import (
	"github.com/jellydator/validation"
)

var (
	_ validation.Validatable = (*Location)(nil)
	_ IDable                 = (*Location)(nil)
)

type Location struct {
	EntityID
	Name    string `json:"name"`
	Address string `json:"address"`
}

func (a *Location) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.Name, validation.Required),
		validation.Field(&a.Address, validation.Required),
	)

	return validation.ValidateStruct(a, fields...)
}
