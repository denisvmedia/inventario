package models

import (
	"github.com/jellydator/validation"
	"golang.org/x/text/currency"
)

var (
	_ validation.Validatable = (*Currency)(nil)
)

type Currency string

func (c Currency) IsValid() bool {
	_, err := currency.ParseISO(string(c))
	if err != nil {
		return false
	}

	return true
}

func (c Currency) Validate() error {
	_, err := currency.ParseISO(string(c))
	return err
}
