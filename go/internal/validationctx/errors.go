package validationctx

import (
	"errors"
)

var (
	ErrMainCurrencyNotSet = errors.New("main currency not set in context")
)
