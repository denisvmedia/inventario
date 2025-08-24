package store

import (
	"errors"

	"github.com/denisvmedia/inventario/registry"
)

var (
	ErrNotFound       = registry.ErrNotFound
	ErrUserIDRequired = errors.New("user ID is required")
)
