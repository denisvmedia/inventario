package models

import (
	"github.com/jellydator/validation"
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
		validation.Field(&a.LocationID, validation.Required),
		validation.Field(&a.Name, validation.Required),
	)

	return validation.ValidateStruct(a, fields...)
}
