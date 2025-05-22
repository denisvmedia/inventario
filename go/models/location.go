package models

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
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
		validation.Field(&a.Name, rules.NotEmpty),
		validation.Field(&a.Address, rules.NotEmpty),
	)

	return validation.ValidateStruct(a, fields...)
}
