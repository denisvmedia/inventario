package models

import (
	"context"

	"github.com/jellydator/validation"
	"golang.org/x/text/currency"
)

var (
	_ validation.Validatable = (*Currency)(nil)
)

type Currency string

func (c Currency) IsValid() bool {
	_, err := currency.ParseISO(string(c))
	return err == nil
}

func (Currency) Validate() error {
	return ErrMustUseValidateWithContext
}

func (c Currency) ValidateWithContext(_ context.Context) error {
	_, err := currency.ParseISO(string(c))
	return err
}
