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

//migrator:schema:table name="categories" platform.mysql.engine="InnoDB" platform.mysql.comment="Product categories"
type Location struct {
	//migrator:embedded
	EntityID
	//migrator:schema:field name="name" type="VARCHAR(100)" primary="true"
	Name string `json:"name" db:"name"`
	//migrator:schema:field name="name" type="TEXT"
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
