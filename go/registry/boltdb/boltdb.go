package boltdb

import (
	"path/filepath"

	"github.com/denisvmedia/inventario/internal/errkit"
	"github.com/denisvmedia/inventario/registry"
	"github.com/denisvmedia/inventario/registry/boltdb/dbx"
)

const Name = "boltdb"

func Register() {
	registry.Register("boltdb", NewRegistrySet)
}

func NewRegistrySet(c registry.Config) (*registry.Set, error) {
	parsed, err := c.Parse()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to parse config DSN")
	}

	if parsed.Scheme != Name {
		return nil, errkit.Wrap(errkit.WithFields(registry.ErrInvalidConfig, errkit.Fields{"expected": Name, "got": parsed.Scheme}), "invalid scheme")
	}

	fpath := filepath.Join(parsed.Host, parsed.Path)

	db, err := dbx.NewDB(filepath.Dir(fpath), filepath.Base(fpath)).Open()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to open db")
	}

	s := &registry.Set{}
	s.LocationRegistry = NewLocationRegistry(db)
	s.AreaRegistry = NewAreaRegistry(db, s.LocationRegistry)
	s.SettingsRegistry = NewSettingsRegistry(db)
	s.CommodityRegistry = NewCommodityRegistry(db, s.AreaRegistry)
	s.ImageRegistry = NewImageRegistry(db, s.CommodityRegistry)
	s.InvoiceRegistry = NewInvoiceRegistry(db, s.CommodityRegistry)
	s.ManualRegistry = NewManualRegistry(db, s.CommodityRegistry)

	return s, nil
}
