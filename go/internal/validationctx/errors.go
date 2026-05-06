package validationctx

import (
	"errors"
)

var (
	ErrGroupCurrencyNotSet = errors.New("group currency not set in context")
)
