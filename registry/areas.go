package registry

import (
	"github.com/denisvmedia/inventario/models"
)

type AreaRegistry = Registry[models.Area]

type MemoryAreaRegistry = MemoryRegistry[models.Area]

func NewMemoryAreaRegistry() *MemoryRegistry[models.Area] {
	return NewMemoryRegistry[models.Area]()
}
