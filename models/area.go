package models

import (
	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable = (*Area)(nil)
	_ IDable                 = (*Area)(nil)
)

type Area struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	LocationID string `json:"location_id"`
}

func (a *Area) GetID() string {
	return a.ID
}

func (a *Area) SetID(id string) {
	a.ID = id
}

func (a *Area) Validate() error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.LocationID, rules.NotEmpty),
		validation.Field(&a.Name, rules.NotEmpty),
	)

	return validation.ValidateStruct(a, fields...)
}
