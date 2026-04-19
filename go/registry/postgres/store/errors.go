package store

import (
	"errors"

	"github.com/denisvmedia/inventario/registry"
)

var (
	ErrNotFound                = registry.ErrNotFound
	ErrUserIDRequired          = errors.New("user ID is required")
	ErrGroupIDRequired         = errors.New("group ID is required")
	ErrTenantIDRequired        = errors.New("tenant ID is required")
	ErrCreatedByUserIDRequired = errors.New("created_by_user_id is required")
)
