package dbx

import (
	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
)

var (
	ErrNotFound              = registry.ErrNotFound
	ErrFailedToUnmarshalJSON = errkit.NewEquivalent("failed to unmarshal json", registry.ErrBadDataStructure)
)
