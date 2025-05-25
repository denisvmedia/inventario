package models

import (
	"errors"
)

var (
	// ErrConvertedPriceNotZero is the error that returns when the original price is in the main currency
	// but the converted original price is not zero.
	ErrConvertedPriceNotZero = errors.New("converted original price must be zero when original price is in the main currency")
)

var (
	ErrMustUseValidateWithContext = errors.New("must use validate with context")
)
