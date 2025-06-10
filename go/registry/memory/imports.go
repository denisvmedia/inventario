package memory

import (
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

var _ registry.ImportRegistry = (*ImportRegistry)(nil)

type baseImportRegistry = Registry[models.Import, *models.Import]
type ImportRegistry struct {
	*baseImportRegistry
}

func NewImportRegistry() *ImportRegistry {
	return &ImportRegistry{
		baseImportRegistry: NewRegistry[models.Import, *models.Import](),
	}
}
