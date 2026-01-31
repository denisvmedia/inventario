package models

import (
	"github.com/go-extras/errx"
)

var (
	// ErrConvertedPriceNotZero is the error that returns when the original price is in the main currency
	// but the converted original price is not zero.
	ErrConvertedPriceNotZero = errx.NewSentinel("converted original price must be zero when original price is in the main currency")
)

var (
	ErrMustUseValidateWithContext = errx.NewSentinel("must use validate with context")
)
