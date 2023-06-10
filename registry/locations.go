package registry

import (
	"github.com/denisvmedia/inventario/models"
)

type LocationRegistry = Registry[models.Location]

type MemoryLocationRegistry = MemoryRegistry[models.Location]

func NewMemoryLocationRegistry() *MemoryRegistry[models.Location] {
	return NewMemoryRegistry[models.Location]()
}
