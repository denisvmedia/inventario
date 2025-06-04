package memory

import (
	"github.com/denisvmedia/inventario/registry"
)

const Name = "memory"

func Register() {
	registry.Register(Name, NewRegistrySet)
}

func NewRegistrySet(_ registry.Config) (*registry.Set, error) {
	s := &registry.Set{}
	s.LocationRegistry = NewLocationRegistry()
	s.AreaRegistry = NewAreaRegistry(s.LocationRegistry)
	s.SettingsRegistry = NewSettingsRegistry()
	s.CommodityRegistry = NewCommodityRegistry(s.AreaRegistry)
	s.ImageRegistry = NewImageRegistry(s.CommodityRegistry)
	s.InvoiceRegistry = NewInvoiceRegistry(s.CommodityRegistry)
	s.ManualRegistry = NewManualRegistry(s.CommodityRegistry)
	s.ExportRegistry = NewExportRegistry()

	return s, nil
}
