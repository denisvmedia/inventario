package typekit

import (
	"errors"
)

var (
	ErrNoFieldWithTag    = errors.New("no field with tag")
	ErrUnsettableField   = errors.New("field is not settable")
	ErrIncompatibleValue = errors.New("value does not fit the field type")
)
