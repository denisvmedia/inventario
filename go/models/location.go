package models

import (
	"context"

	"github.com/jellydator/validation"

	"github.com/denisvmedia/inventario/models/rules"
)

var (
	_ validation.Validatable = (*Location)(nil)
	_ IDable                 = (*Location)(nil)
)

type Location struct {
	EntityID
	Name    string `json:"name" db:"name"`
	Address string `json:"address" db:"address"`
}

func (*Location) Validate() error {
	return ErrMustUseValidateWithContext
}

func (a *Location) ValidateWithContext(ctx context.Context) error {
	fields := make([]*validation.FieldRules, 0)

	fields = append(fields,
		validation.Field(&a.Name, rules.NotEmpty),
		validation.Field(&a.Address, rules.NotEmpty),
	)

	return validation.ValidateStructWithContext(ctx, a, fields...)
}
