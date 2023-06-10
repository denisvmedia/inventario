package registry

import (
	"errors"
)

var (
	ErrNotFound  = errors.New("not found")
	ErrNotIDable = errors.New("not idable")
)