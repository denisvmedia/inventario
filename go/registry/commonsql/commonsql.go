package commonsql

import (
	"github.com/jmoiron/sqlx"

	"github.com/denisvmedia/inventario/registry"
)

type TableNames struct {
	Locations   func() string
	Areas       func() string
	Commodities func() string
	Settings    func() string
	Images      func() string
	Invoices    func() string
	Manuals     func() string
	Exports     func() string
	Files       func() string
}

var DefaultTableNames = TableNames{
	Locations:   func() string { return "locations" },
	Areas:       func() string { return "areas" },
	Commodities: func() string { return "commodities" },
	Settings:    func() string { return "settings" },
	Images:      func() string { return "images" },
	Invoices:    func() string { return "invoices" },
	Manuals:     func() string { return "manuals" },
	Exports:     func() string { return "exports" },
	Files:       func() string { return "files" },
}

func NewRegistrySet(dbx *sqlx.DB) *registry.Set {
	s := &registry.Set{}
	s.LocationRegistry = NewLocationRegistry(dbx)
	s.AreaRegistry = NewAreaRegistry(dbx)
	s.SettingsRegistry = NewSettingsRegistry(dbx)
	s.CommodityRegistry = NewCommodityRegistry(dbx)
	s.ImageRegistry = NewImageRegistry(dbx)
	s.InvoiceRegistry = NewInvoiceRegistry(dbx)
	s.ManualRegistry = NewManualRegistry(dbx)
	s.ExportRegistry = NewExportRegistry(dbx)
	s.RestoreStepRegistry = NewRestoreStepRegistry(dbx)
	s.RestoreOperationRegistry = NewRestoreOperationRegistry(dbx, s.RestoreStepRegistry)
	s.FileRegistry = NewFileRegistry(dbx)

	// Set up dependencies for recursive deletion
	s.LocationRegistry.(*LocationRegistry).SetAreaRegistry(s.AreaRegistry)
	s.AreaRegistry.(*AreaRegistry).SetCommodityRegistry(s.CommodityRegistry)

	return s
}
