package registry

import (
	"errors"

	"github.com/denisvmedia/inventario/internal/errkit"
)

var (
	ErrNotFound           = errors.New("not found")
	ErrCannotDelete       = errors.New("cannot delete")
	ErrInvalidConfig      = errors.New("invalid config")
	ErrFieldRequired      = errors.New("field required")
	ErrAlreadyExists      = errors.New("already exists")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrSlugAlreadyExists  = errors.New("slug already exists")
	ErrBadDataStructure   = errors.New("bad data structure")
	ErrDeleted            = errkit.NewEquivalent("deleted", ErrNotFound)

	ErrMainCurrencyNotSet       = errors.New("main currency not set")
	ErrMainCurrencyAlreadySet   = errors.New("main currency already set and cannot be changed")
	ErrUserContextRequired      = errors.New("user context required")
	ErrResourceLimitExceeded    = errors.New("resource limit exceeded")
	ErrConcurrencyLimitExceeded = errors.New("concurrency limit exceeded")
)
