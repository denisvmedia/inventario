package models

import (
	"context"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable = (*Area)(nil)
	_ IDable                 = (*Area)(nil)
)

type Area struct {
	EntityID
	Name       string `json:"name" db:"name"`
	LocationID string `json:"location_id" db:"location_id"`
}

func (*Area) Validate() error {
	return ErrMustUseValidateWithContext
}

func (a *Area) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.LocationID, rules.NotEmpty),
		validation.Field(&a.Name, rules.NotEmpty),
	)

	return validation.ValidateStructWithContext(ctx, a, fields...)
}
