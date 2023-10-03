package registry

import (
	"errors"
)

var (
	ErrNotFound     = errors.New("not found")
	ErrCannotDelete = errors.New("cannot delete")
)
