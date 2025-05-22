package registry

import (
	"errors"
)

var (
	ErrNotFound         = errors.New("not found")
	ErrCannotDelete     = errors.New("cannot delete")
	ErrInvalidConfig    = errors.New("invalid config")
	ErrFieldRequired    = errors.New("field required")
	ErrAlreadyExists    = errors.New("already exists")
	ErrBadDataStructure = errors.New("bad data structure")

	ErrMainCurrencyNotSet = errors.New("main currency not set")
)
