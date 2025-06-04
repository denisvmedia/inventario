package memory

import (
	"github.com/denisvmedia/inventario/models"
	"github.com/denisvmedia/inventario/registry"
)

type ExportRegistry struct {
	*Registry[models.Export, *models.Export]
}

func NewExportRegistry() registry.ExportRegistry {
	return &ExportRegistry{
		Registry: NewRegistry[models.Export, *models.Export](),
	}
}