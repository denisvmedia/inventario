package boltdb

import (
	_ "go.etcd.io/bbolt"

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
		return nil, errkit.Wrap(registry.ErrInvalidConfig, "invalid scheme")
	}

	db, err := dbx.NewDB(parsed.Path).Open()
	if err != nil {
		return nil, errkit.Wrap(err, "failed to open db")
	}

	s := &registry.Set{}
	s.LocationRegistry = NewLocationRegistry(db)
	// s.AreaRegistry = NewAreaRegistry(s.LocationRegistry)
	// s.CommodityRegistry = NewCommodityRegistry(s.AreaRegistry)
	// s.ImageRegistry = NewImageRegistry(s.CommodityRegistry)
	// s.InvoiceRegistry = NewInvoiceRegistry(s.CommodityRegistry)
	// s.ManualRegistry = NewManualRegistry(s.CommodityRegistry)

	return s, nil
}
